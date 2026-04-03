package policy

import (
	"context"
	"errors"
	"log"
	"time"

	"proxy-center/internal/auth"
	"proxy-center/internal/session"
	"proxy-center/internal/store"
)

type Enforcer struct {
	authSvc   *auth.Service
	store     *store.Store
	sessions  *session.Manager
	interval  time.Duration
	lastKicks map[string]time.Time
}

func NewEnforcer(authSvc *auth.Service, st *store.Store, sessions *session.Manager, interval time.Duration) *Enforcer {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	return &Enforcer{
		authSvc:   authSvc,
		store:     st,
		sessions:  sessions,
		interval:  interval,
		lastKicks: make(map[string]time.Time),
	}
}

func (e *Enforcer) Start(ctx context.Context) {
	e.runOnce(ctx)
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.runOnce(ctx)
		}
	}
}

func (e *Enforcer) runOnce(ctx context.Context) {
	now := time.Now()
	users := e.sessions.ActiveUsernames()
	for _, username := range users {
		user, err := e.store.GetUserByUsername(ctx, username)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				kicked := e.sessions.KickUser(username)
				if kicked > 0 {
					log.Printf("policy enforcer kicked %d sessions for unknown user=%s", kicked, username)
				}
			}
			continue
		}
		if err := e.authSvc.Authorize(ctx, user, now); err != nil {
			if !e.shouldLogKick(username, now) {
				_ = e.sessions.KickUser(username)
				continue
			}
			kicked := e.sessions.KickUser(username)
			if kicked > 0 {
				log.Printf("policy enforcer kicked %d sessions for user=%s reason=%v", kicked, username, err)
			}
		}
	}
}

func (e *Enforcer) shouldLogKick(username string, now time.Time) bool {
	last := e.lastKicks[username]
	if now.Sub(last) < 10*time.Second {
		e.lastKicks[username] = now
		return false
	}
	e.lastKicks[username] = now
	return true
}
