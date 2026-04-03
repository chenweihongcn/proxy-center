package proxy

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"proxy-center/internal/auth"
	"proxy-center/internal/session"
	"proxy-center/internal/store"
	"proxy-center/internal/traffic"
	"proxy-center/internal/upstream"
)

type HTTPProxy struct {
	addr       string
	authSvc    *auth.Service
	sessions   *session.Manager
	store      *store.Store
	traffic    *traffic.Recorder
	router     *upstream.Router
	logDomains bool
	srv        *http.Server
}

func NewHTTPProxy(
	addr string,
	authSvc *auth.Service,
	sessions *session.Manager,
	st *store.Store,
	tr *traffic.Recorder,
	router *upstream.Router,
	logDomains bool,
) *HTTPProxy {
	return &HTTPProxy{
		addr:       addr,
		authSvc:    authSvc,
		sessions:   sessions,
		store:      st,
		traffic:    tr,
		router:     router,
		logDomains: logDomains,
	}
}

func (p *HTTPProxy) Start(ctx context.Context) error {
	p.srv = &http.Server{
		Addr:    p.addr,
		Handler: p,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = p.srv.Shutdown(shutdownCtx)
	}()

	err := p.srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (p *HTTPProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	now := time.Now()
	username, password, ok := parseProxyBasicAuth(req.Header.Get("Proxy-Authorization"))
	if !ok {
		writeProxyAuthRequired(w)
		return
	}

	user, err := p.authSvc.AuthenticateAndAuthorize(req.Context(), username, password, now)
	if err != nil {
		writeProxyDenied(w, err)
		return
	}

	sessionToken, acquired, _ := p.sessions.Acquire(user.Username, user.MaxConns, nil)
	if !acquired {
		http.Error(w, "too many active connections", http.StatusTooManyRequests)
		return
	}
	defer p.sessions.Release(sessionToken)

	target := resolveHTTPTarget(req)
	sessionID, _ := p.store.InsertSessionStart(req.Context(), user.Username, "http", req.RemoteAddr, target, now)
	sessionEnded := false
	defer func() {
		if !sessionEnded {
			_ = p.store.EndSession(context.Background(), sessionID, 0, 0, time.Now())
		}
	}()

	domain := extractDomainFromRequest(req)
	if p.logDomains && domain != "" {
		p.traffic.RecordDomain(user.ID, domain, now)
	}

	if strings.EqualFold(req.Method, http.MethodConnect) {
		p.handleConnect(w, req, user, sessionID, sessionToken, &sessionEnded)
		return
	}
	p.handlePlainHTTP(w, req, user, sessionID, sessionToken, &sessionEnded)
}

func (p *HTTPProxy) handleConnect(w http.ResponseWriter, req *http.Request, user store.User, sessionID int64, sessionToken uint64, sessionEnded *bool) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking unsupported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, "hijack failed", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	target := req.Host
	if !strings.Contains(target, ":") {
		target += ":443"
	}

	upstreamConn, err := p.router.DialContext(req.Context(), target)
	if err != nil {
		_, _ = io.WriteString(clientConn, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		return
	}
	defer upstreamConn.Close()

	if _, err := io.WriteString(clientConn, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	p.sessions.SetCloser(sessionToken, func() {
		_ = clientConn.Close()
		_ = upstreamConn.Close()
	})

	up, down := relayBidirectional(clientConn, upstreamConn,
		func(n int64) { p.traffic.RecordUsage(user.ID, n, time.Now()) },
		func(n int64) { p.traffic.RecordUsage(user.ID, n, time.Now()) },
	)
	bytesTotal := up + down
	_ = bytesTotal
	_ = p.store.EndSession(context.Background(), sessionID, up, down, time.Now())
	*sessionEnded = true
}

func (p *HTTPProxy) handlePlainHTTP(w http.ResponseWriter, req *http.Request, user store.User, sessionID int64, _ uint64, sessionEnded *bool) {
	transport := &http.Transport{
		Proxy:             nil,
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			_ = network
			return p.router.DialContext(ctx, addr)
		},
	}

	outReq := req.Clone(req.Context())
	outReq.RequestURI = ""
	outReq.Header.Del("Proxy-Authorization")
	if outReq.URL == nil {
		outReq.URL = &url.URL{}
	}
	if outReq.URL.Scheme == "" {
		outReq.URL.Scheme = "http"
	}
	if outReq.URL.Host == "" {
		outReq.URL.Host = outReq.Host
	}

	bodyCounter := &countingReadCloser{ReadCloser: outReq.Body}
	if outReq.Body != nil {
		outReq.Body = bodyCounter
	}

	resp, err := transport.RoundTrip(outReq)
	if err != nil {
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	downBytes, _ := io.Copy(w, resp.Body)
	upBytes := bodyCounter.n
	p.traffic.RecordUsage(user.ID, upBytes+downBytes, time.Now())
	_ = p.store.EndSession(context.Background(), sessionID, upBytes, downBytes, time.Now())
	*sessionEnded = true
}

func writeProxyAuthRequired(w http.ResponseWriter) {
	w.Header().Set("Proxy-Authenticate", `Basic realm="proxy-center"`)
	http.Error(w, "proxy authentication required", http.StatusProxyAuthRequired)
}

func writeProxyDenied(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		http.Error(w, "invalid credentials", http.StatusProxyAuthRequired)
	case errors.Is(err, auth.ErrAccountDisabled):
		http.Error(w, "account disabled", http.StatusForbidden)
	case errors.Is(err, auth.ErrAccountExpired):
		http.Error(w, "account expired", http.StatusForbidden)
	case errors.Is(err, auth.ErrQuotaExceeded):
		http.Error(w, "traffic quota exceeded", http.StatusForbidden)
	default:
		http.Error(w, "authentication failed", http.StatusUnauthorized)
	}
}

func parseProxyBasicAuth(value string) (string, string, bool) {
	if value == "" {
		return "", "", false
	}
	parts := strings.SplitN(value, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Basic") {
		return "", "", false
	}
	raw, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", false
	}
	pair := strings.SplitN(string(raw), ":", 2)
	if len(pair) != 2 {
		return "", "", false
	}
	return pair[0], pair[1], true
}

func resolveHTTPTarget(req *http.Request) string {
	if req == nil {
		return ""
	}
	if req.Method == http.MethodConnect {
		return req.Host
	}
	if req.URL != nil && req.URL.Host != "" {
		return req.URL.Host
	}
	return req.Host
}

func extractDomainFromRequest(req *http.Request) string {
	if req == nil {
		return ""
	}
	if req.URL != nil {
		if host := req.URL.Hostname(); host != "" {
			return host
		}
	}
	host := req.Host
	if i := strings.Index(host, ":"); i > 0 {
		host = host[:i]
	}
	return host
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

type countingReadCloser struct {
	io.ReadCloser
	n int64
}

func (c *countingReadCloser) Read(p []byte) (int, error) {
	if c.ReadCloser == nil {
		return 0, io.EOF
	}
	n, err := c.ReadCloser.Read(p)
	c.n += int64(n)
	return n, err
}

func (c *countingReadCloser) Close() error {
	if c.ReadCloser == nil {
		return nil
	}
	return c.ReadCloser.Close()
}


