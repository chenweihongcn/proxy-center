package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"proxy-center/internal/auth"
	"proxy-center/internal/config"
	"proxy-center/internal/policy"
	"proxy-center/internal/proxy"
	"proxy-center/internal/session"
	"proxy-center/internal/store"
	"proxy-center/internal/traffic"
	"proxy-center/internal/upstream"
	"proxy-center/internal/web"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(version)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		log.Fatalf("create db dir: %v", err)
	}

	st, err := store.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := st.Migrate(ctx); err != nil {
		log.Fatalf("migrate db: %v", err)
	}
	if err := st.EnsureAdmin(ctx, cfg.AdminUser, cfg.AdminPass); err != nil {
		log.Fatalf("ensure admin: %v", err)
	}

	authSvc := auth.NewService(st)
	sessions := session.NewManager()
	trafficRecorder := traffic.NewRecorder(st, 2048)
	defer trafficRecorder.Close()
	router, err := upstream.NewRouter(cfg)
	if err != nil {
		log.Fatalf("init upstream router: %v", err)
	}
	enforcer := policy.NewEnforcer(authSvc, st, sessions, 2*time.Second)

	httpProxy := proxy.NewHTTPProxy(cfg.HTTPListen, authSvc, sessions, st, trafficRecorder, router, cfg.LogDomains)
	socksProxy := proxy.NewSOCKS5Proxy(cfg.SOCKSListen, authSvc, sessions, st, trafficRecorder, router, cfg.LogDomains)
	webServer := web.NewServer(cfg.WebListen, authSvc, sessions, st, router)

	log.Printf("proxy-center starting: http=%s socks5=%s web=%s egress=%s", cfg.HTTPListen, cfg.SOCKSListen, cfg.WebListen, cfg.EgressMode)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go enforcer.Start(runCtx)
	go router.Start(runCtx)

	errCh := make(chan error, 3)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := httpProxy.Start(runCtx); err != nil {
			errCh <- err
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := socksProxy.Start(runCtx); err != nil {
			errCh <- err
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := webServer.Start(runCtx); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Printf("shutdown signal received")
	case err := <-errCh:
		log.Printf("service error: %v", err)
	}

	cancel()
	wg.Wait()
	log.Printf("proxy-center stopped")
}
