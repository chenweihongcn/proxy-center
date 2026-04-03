package proxy

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"proxy-center/internal/auth"
	"proxy-center/internal/session"
	"proxy-center/internal/store"
	"proxy-center/internal/traffic"
	"proxy-center/internal/upstream"
)

type SOCKS5Proxy struct {
	addr       string
	authSvc    *auth.Service
	sessions   *session.Manager
	store      *store.Store
	traffic    *traffic.Recorder
	router     *upstream.Router
	logDomains bool
	ln         net.Listener
}

func NewSOCKS5Proxy(
	addr string,
	authSvc *auth.Service,
	sessions *session.Manager,
	st *store.Store,
	tr *traffic.Recorder,
	router *upstream.Router,
	logDomains bool,
) *SOCKS5Proxy {
	return &SOCKS5Proxy{
		addr:       addr,
		authSvc:    authSvc,
		sessions:   sessions,
		store:      st,
		traffic:    tr,
		router:     router,
		logDomains: logDomains,
	}
}

func (p *SOCKS5Proxy) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", p.addr)
	if err != nil {
		return err
	}
	p.ln = ln

	go func() {
		<-ctx.Done()
		_ = p.ln.Close()
	}()

	for {
		conn, err := p.ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			continue
		}
		go p.handleConn(conn)
	}
}

func (p *SOCKS5Proxy) handleConn(client net.Conn) {
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(20 * time.Second))

	user, err := p.authenticate(client)
	if err != nil {
		return
	}

	cmd, target, domain, err := readSocksRequest(client)
	if err != nil {
		_ = writeSocksReply(client, 0x01, nil)
		return
	}
	if cmd != 0x01 {
		_ = writeSocksReply(client, 0x07, nil)
		return
	}

	sessionToken, acquired, _ := p.sessions.Acquire(user.Username, user.MaxConns, func() {
		_ = client.Close()
	})
	if !acquired {
		_ = writeSocksReply(client, 0x01, nil)
		return
	}
	defer p.sessions.Release(sessionToken)

	now := time.Now()
	sessionID, _ := p.store.InsertSessionStart(context.Background(), user.Username, "socks5", client.RemoteAddr().String(), target, now)
	sessionEnded := false
	defer func() {
		if !sessionEnded {
			_ = p.store.EndSession(context.Background(), sessionID, 0, 0, time.Now())
		}
	}()

	_ = client.SetDeadline(time.Time{})
	remote, err := p.router.DialContext(context.Background(), target)
	if err != nil {
		_ = writeSocksReply(client, 0x05, nil)
		return
	}
	defer remote.Close()
	p.sessions.SetCloser(sessionToken, func() {
		_ = client.Close()
		_ = remote.Close()
	})

	if err := writeSocksReply(client, 0x00, remote.LocalAddr()); err != nil {
		return
	}
	if p.logDomains && domain != "" {
		p.traffic.RecordDomain(user.ID, domain, now)
	}

	up, down := relayBidirectional(client, remote,
		func(n int64) { p.traffic.RecordUsage(user.ID, n, time.Now()) },
		func(n int64) { p.traffic.RecordUsage(user.ID, n, time.Now()) },
	)
	_ = p.store.EndSession(context.Background(), sessionID, up, down, time.Now())
	sessionEnded = true
}

func (p *SOCKS5Proxy) authenticate(client net.Conn) (store.User, error) {
	head := make([]byte, 2)
	if _, err := io.ReadFull(client, head); err != nil {
		return store.User{}, err
	}
	if head[0] != 0x05 {
		return store.User{}, fmt.Errorf("unsupported socks version")
	}
	nMethods := int(head[1])
	methods := make([]byte, nMethods)
	if _, err := io.ReadFull(client, methods); err != nil {
		return store.User{}, err
	}

	hasUserPass := false
	for _, m := range methods {
		if m == 0x02 {
			hasUserPass = true
			break
		}
	}
	if !hasUserPass {
		_, _ = client.Write([]byte{0x05, 0xFF})
		return store.User{}, fmt.Errorf("username/password auth not offered")
	}
	if _, err := client.Write([]byte{0x05, 0x02}); err != nil {
		return store.User{}, err
	}

	u, psswd, err := readSocksUserPass(client)
	if err != nil {
		_, _ = client.Write([]byte{0x01, 0x01})
		return store.User{}, err
	}

	user, err := p.authSvc.AuthenticateAndAuthorize(context.Background(), u, psswd, time.Now())
	if err != nil {
		_, _ = client.Write([]byte{0x01, 0x01})
		return store.User{}, err
	}
	_, _ = client.Write([]byte{0x01, 0x00})
	return user, nil
}

func readSocksUserPass(r io.Reader) (username, password string, err error) {
	head := make([]byte, 2)
	if _, err = io.ReadFull(r, head); err != nil {
		return "", "", err
	}
	if head[0] != 0x01 {
		return "", "", fmt.Errorf("invalid auth version")
	}
	ulen := int(head[1])
	if ulen == 0 {
		return "", "", fmt.Errorf("empty username")
	}
	ub := make([]byte, ulen)
	if _, err = io.ReadFull(r, ub); err != nil {
		return "", "", err
	}
	plenBuf := make([]byte, 1)
	if _, err = io.ReadFull(r, plenBuf); err != nil {
		return "", "", err
	}
	plen := int(plenBuf[0])
	pb := make([]byte, plen)
	if _, err = io.ReadFull(r, pb); err != nil {
		return "", "", err
	}
	return string(ub), string(pb), nil
}

func readSocksRequest(r io.Reader) (cmd byte, target string, domain string, err error) {
	h := make([]byte, 4)
	if _, err = io.ReadFull(r, h); err != nil {
		return 0, "", "", err
	}
	if h[0] != 0x05 {
		return 0, "", "", fmt.Errorf("invalid request version")
	}
	cmd = h[1]
	atyp := h[3]

	var host string
	switch atyp {
	case 0x01:
		addr := make([]byte, 4)
		if _, err = io.ReadFull(r, addr); err != nil {
			return 0, "", "", err
		}
		host = net.IP(addr).String()
	case 0x04:
		addr := make([]byte, 16)
		if _, err = io.ReadFull(r, addr); err != nil {
			return 0, "", "", err
		}
		host = net.IP(addr).String()
	case 0x03:
		l := make([]byte, 1)
		if _, err = io.ReadFull(r, l); err != nil {
			return 0, "", "", err
		}
		db := make([]byte, int(l[0]))
		if _, err = io.ReadFull(r, db); err != nil {
			return 0, "", "", err
		}
		host = string(db)
		domain = host
	default:
		return 0, "", "", fmt.Errorf("unsupported atyp")
	}

	pb := make([]byte, 2)
	if _, err = io.ReadFull(r, pb); err != nil {
		return 0, "", "", err
	}
	port := int(binary.BigEndian.Uint16(pb))
	target = net.JoinHostPort(host, strconv.Itoa(port))
	return cmd, target, domain, nil
}

func writeSocksReply(w io.Writer, rep byte, localAddr net.Addr) error {
	resp := []byte{0x05, rep, 0x00}
	if localAddr == nil {
		resp = append(resp, 0x01, 0, 0, 0, 0, 0, 0)
		_, err := w.Write(resp)
		return err
	}

	tcp, ok := localAddr.(*net.TCPAddr)
	if !ok {
		resp = append(resp, 0x01, 0, 0, 0, 0, 0, 0)
		_, err := w.Write(resp)
		return err
	}
	ip4 := tcp.IP.To4()
	if ip4 != nil {
		resp = append(resp, 0x01)
		resp = append(resp, ip4...)
	} else {
		resp = append(resp, 0x04)
		resp = append(resp, tcp.IP.To16()...)
	}
	p := make([]byte, 2)
	binary.BigEndian.PutUint16(p, uint16(tcp.Port))
	resp = append(resp, p...)
	_, err := w.Write(resp)
	return err
}
