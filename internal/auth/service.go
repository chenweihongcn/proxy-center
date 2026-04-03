package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"proxy-center/internal/store"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountDisabled    = errors.New("account is disabled")
	ErrAccountExpired     = errors.New("account has expired")
	ErrQuotaExceeded      = errors.New("traffic quota exceeded")
)

type Service struct {
	store *store.Store
}

func NewService(st *store.Store) *Service {
	return &Service{store: st}
}

func (s *Service) AuthenticateAndAuthorize(ctx context.Context, username, password string, now time.Time) (store.User, error) {
	u, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.User{}, ErrInvalidCredentials
		}
		return store.User{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return store.User{}, ErrInvalidCredentials
	}
	if err := s.Authorize(ctx, u, now); err != nil {
		return store.User{}, err
	}
	return u, nil
}

func (s *Service) Authorize(ctx context.Context, user store.User, now time.Time) error {
	if !user.Enabled {
		return ErrAccountDisabled
	}
	if user.ExpiresAt > 0 && now.Unix() >= user.ExpiresAt {
		return ErrAccountExpired
	}

	usage, err := s.store.GetUsage(ctx, user.ID, now)
	if err != nil {
		return fmt.Errorf("read usage: %w", err)
	}

	if overLimit(usage.DayBytes, user.QuotaDayMB) {
		return ErrQuotaExceeded
	}
	if overLimit(usage.MonthBytes, user.QuotaMonthMB) {
		return ErrQuotaExceeded
	}
	if overLimit(usage.TotalBytes, user.QuotaTotalMB) {
		return ErrQuotaExceeded
	}
	return nil
}

func overLimit(usedBytes, limitMB int64) bool {
	if limitMB <= 0 {
		return false
	}
	limitBytes := limitMB * 1024 * 1024
	return usedBytes >= limitBytes
}
