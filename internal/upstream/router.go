package upstream

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"proxy-center/internal/config"
)

type node struct {
	ID     string
	Mode   string
	Addr   string
	User   string
	Pass   string
	Weight int
	health atomic.Bool
}

type NodeStatus struct {
	ID      string `json:"id"`
	Mode    string `json:"mode"`
	Addr    string `json:"addr"`
	Weight  int    `json:"weight"`
	Healthy bool   `json:"healthy"`
}

type Router struct {
	cfg    config.Config
	dialer net.Dialer
	pool   []*node
	rr     uint64
}

func NewRouter(cfg config.Config) (*Router, error) {
	r := &Router{
		cfg: cfg,
		dialer: net.Dialer{
			Timeout: cfg.DialTimeout,
		},
	}
	if strings.EqualFold(cfg.EgressMode, "pool") {
		pool, err := parsePool(cfg.EgressPool)
		if err != nil {
			return nil, err
		}
		r.pool = pool
	}
	return r, nil
}

func (r *Router) Start(ctx context.Context) {
	if len(r.pool) == 0 {
		return
	}
	r.checkAll()
	tick := r.cfg.HealthTicker
	if tick <= 0 {
		tick = 10 * time.Second
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.checkAll()
		}
	}
}

func (r *Router) Status() []NodeStatus {
	out := make([]NodeStatus, 0, len(r.pool))
	for _, n := range r.pool {
		out = append(out, NodeStatus{
			ID:      n.ID,
			Mode:    n.Mode,
			Addr:    n.Addr,
			Weight:  n.Weight,
			Healthy: n.health.Load(),
		})
	}
	return out
}

func (r *Router) DialContext(ctx context.Context, targetAddr string) (net.Conn, error) {
	switch r.cfg.EgressMode {
	case "direct":
		return r.dialer.DialContext(ctx, "tcp", targetAddr)
	case "http-upstream":
		return r.dialViaHTTPUpstream(ctx, targetAddr, r.cfg.EgressAddr, r.cfg.EgressUser, r.cfg.EgressPass)
	case "socks5-upstream":
		return r.dialViaSOCKS5Upstream(ctx, targetAddr, r.cfg.EgressAddr, r.cfg.EgressUser, r.cfg.EgressPass)
	case "pool":
		n, err := r.pickNode()
		if err != nil {
			return nil, err
		}
		switch n.Mode {
		case "http-upstream":
			return r.dialViaHTTPUpstream(ctx, targetAddr, n.Addr, n.User, n.Pass)
		case "socks5-upstream":
			return r.dialViaSOCKS5Upstream(ctx, targetAddr, n.Addr, n.User, n.Pass)
		default:
			return nil, fmt.Errorf("unsupported pool node mode: %s", n.Mode)
		}
	default:
		return nil, fmt.Errorf("unknown egress mode: %s", r.cfg.EgressMode)
	}
}

func (r *Router) dialViaHTTPUpstream(ctx context.Context, targetAddr, upstreamAddr, upstreamUser, upstreamPass string) (net.Conn, error) {
	conn, err := r.dialer.DialContext(ctx, "tcp", upstreamAddr)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(r.cfg.DialTimeout)
	_ = conn.SetDeadline(deadline)

	var b strings.Builder
	fmt.Fprintf(&b, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n", targetAddr, targetAddr)
	if upstreamUser != "" {
		token := base64.StdEncoding.EncodeToString([]byte(upstreamUser + ":" + upstreamPass))
		fmt.Fprintf(&b, "Proxy-Authorization: Basic %s\r\n", token)
	}
	b.WriteString("\r\n")

	if _, err := io.WriteString(conn, b.String()); err != nil {
		_ = conn.Close()
		return nil, err
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = conn.Close()
		return nil, fmt.Errorf("http upstream CONNECT failed: %s", resp.Status)
	}

	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func (r *Router) dialViaSOCKS5Upstream(ctx context.Context, targetAddr, upstreamAddr, upstreamUser, upstreamPass string) (net.Conn, error) {
	conn, err := r.dialer.DialContext(ctx, "tcp", upstreamAddr)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(r.cfg.DialTimeout))

	methods := []byte{0x00}
	if upstreamUser != "" {
		methods = append(methods, 0x02)
	}
	greeting := append([]byte{0x05, byte(len(methods))}, methods...)
	if _, err := conn.Write(greeting); err != nil {
		_ = conn.Close()
		return nil, err
	}

	selected := make([]byte, 2)
	if _, err := io.ReadFull(conn, selected); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if selected[0] != 0x05 || selected[1] == 0xFF {
		_ = conn.Close()
		return nil, fmt.Errorf("socks5 upstream rejected auth methods")
	}

	if selected[1] == 0x02 {
		if err := socks5UserPassAuth(conn, upstreamUser, upstreamPass); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}

	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		_ = conn.Close()
		return nil, fmt.Errorf("invalid target port: %s", portStr)
	}

	req := []byte{0x05, 0x01, 0x00}
	ip := net.ParseIP(host)
	switch {
	case ip != nil && ip.To4() != nil:
		req = append(req, 0x01)
		req = append(req, ip.To4()...)
	case ip != nil && ip.To16() != nil:
		req = append(req, 0x04)
		req = append(req, ip.To16()...)
	default:
		hostBytes := []byte(host)
		if len(hostBytes) > 255 {
			_ = conn.Close()
			return nil, fmt.Errorf("target host too long")
		}
		req = append(req, 0x03, byte(len(hostBytes)))
		req = append(req, hostBytes...)
	}
	pbuf := make([]byte, 2)
	binary.BigEndian.PutUint16(pbuf, uint16(port))
	req = append(req, pbuf...)

	if _, err := conn.Write(req); err != nil {
		_ = conn.Close()
		return nil, err
	}

	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if head[1] != 0x00 {
		_ = conn.Close()
		return nil, fmt.Errorf("socks5 upstream connect failed with code %d", head[1])
	}

	if err := discardSocks5Address(conn, head[3]); err != nil {
		_ = conn.Close()
		return nil, err
	}

	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func socks5UserPassAuth(conn net.Conn, user, pass string) error {
	if len(user) > 255 || len(pass) > 255 {
		return fmt.Errorf("upstream credentials too long")
	}
	pkt := []byte{0x01, byte(len(user))}
	pkt = append(pkt, []byte(user)...)
	pkt = append(pkt, byte(len(pass)))
	pkt = append(pkt, []byte(pass)...)
	if _, err := conn.Write(pkt); err != nil {
		return err
	}
	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return err
	}
	if resp[1] != 0x00 {
		return fmt.Errorf("upstream socks5 auth failed")
	}
	return nil
}

func (r *Router) pickNode() (*node, error) {
	if len(r.pool) == 0 {
		return nil, errors.New("pool mode has no nodes")
	}
	healthy := make([]*node, 0, len(r.pool))
	for _, n := range r.pool {
		if n.health.Load() {
			healthy = append(healthy, n)
		}
	}
	candidates := healthy
	if len(candidates) == 0 {
		candidates = r.pool
	}

	sumWeight := 0
	for _, n := range candidates {
		w := n.Weight
		if w <= 0 {
			w = 1
		}
		sumWeight += w
	}
	if sumWeight <= 0 {
		return nil, errors.New("pool has invalid weight setup")
	}

	pos := int(atomic.AddUint64(&r.rr, 1) % uint64(sumWeight))
	acc := 0
	for _, n := range candidates {
		w := n.Weight
		if w <= 0 {
			w = 1
		}
		acc += w
		if pos < acc {
			return n, nil
		}
	}
	return candidates[len(candidates)-1], nil
}

func (r *Router) checkAll() {
	for _, n := range r.pool {
		n.health.Store(r.probeNode(n.Addr))
	}
}

func (r *Router) probeNode(addr string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.DialTimeout)
	defer cancel()
	conn, err := r.dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func parsePool(raw string) ([]*node, error) {
	parts := strings.Split(raw, ",")
	out := make([]*node, 0, len(parts))
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		u, err := url.Parse(part)
		if err != nil {
			return nil, fmt.Errorf("parse pool item %q: %w", part, err)
		}
		mode := ""
		switch strings.ToLower(u.Scheme) {
		case "http", "https":
			mode = "http-upstream"
		case "socks", "socks5":
			mode = "socks5-upstream"
		default:
			return nil, fmt.Errorf("unsupported pool scheme: %s", u.Scheme)
		}
		if u.Host == "" {
			return nil, fmt.Errorf("pool item host is empty: %q", part)
		}
		weight := 1
		if w := strings.TrimSpace(u.Query().Get("weight")); w != "" {
			parsed, err := strconv.Atoi(w)
			if err != nil || parsed <= 0 {
				return nil, fmt.Errorf("invalid pool weight in %q", part)
			}
			weight = parsed
		}
		user := ""
		pass := ""
		if u.User != nil {
			user = u.User.Username()
			pass, _ = u.User.Password()
		}
		n := &node{
			ID:     fmt.Sprintf("pool-%d", i+1),
			Mode:   mode,
			Addr:   u.Host,
			User:   user,
			Pass:   pass,
			Weight: weight,
		}
		n.health.Store(true)
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, errors.New("egress pool is empty")
	}
	return out, nil
}

func discardSocks5Address(conn net.Conn, atyp byte) error {
	var n int
	switch atyp {
	case 0x01:
		n = 4
	case 0x04:
		n = 16
	case 0x03:
		l := make([]byte, 1)
		if _, err := io.ReadFull(conn, l); err != nil {
			return err
		}
		n = int(l[0])
	default:
		return fmt.Errorf("unsupported atyp: %d", atyp)
	}
	if n > 0 {
		buf := make([]byte, n)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return err
		}
	}
	port := make([]byte, 2)
	if _, err := io.ReadFull(conn, port); err != nil {
		return err
	}
	return nil
}
