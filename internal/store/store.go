package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	db *sql.DB
}

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string
	Enabled      bool
	ExpiresAt    int64
	MaxConns     int
	QuotaDayMB   int64
	QuotaMonthMB int64
	QuotaTotalMB int64
	CreatedAt    int64
}

type Usage struct {
	DayKey     string
	DayBytes   int64
	MonthKey   string
	MonthBytes int64
	TotalBytes int64
	UpdatedAt  int64
}

type DomainStat struct {
	Domain string `json:"domain"`
	Count  int64  `json:"count"`
}

type RecentDomainLog struct {
	Username string `json:"username"`
	Domain   string `json:"domain"`
	TS       int64  `json:"ts"`
}

type UserCreateInput struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	Role         string `json:"role"`
	Enabled      bool   `json:"enabled"`
	ExpiresAt    int64  `json:"expires_at"`
	MaxConns     int    `json:"max_conns"`
	QuotaDayMB   int64  `json:"quota_day_mb"`
	QuotaMonthMB int64  `json:"quota_month_mb"`
	QuotaTotalMB int64  `json:"quota_total_mb"`
}

type UserUpdateInput struct {
	Password     *string `json:"password"`
	Role         *string `json:"role"`
	Enabled      *bool   `json:"enabled"`
	ExpiresAt    *int64  `json:"expires_at"`
	MaxConns     *int    `json:"max_conns"`
	QuotaDayMB   *int64  `json:"quota_day_mb"`
	QuotaMonthMB *int64  `json:"quota_month_mb"`
	QuotaTotalMB *int64  `json:"quota_total_mb"`
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)

	s := &Store{db: db}
	if err := s.setPragmas(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) setPragmas() error {
	stmts := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA foreign_keys=ON;",
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("apply pragma: %w", err)
		}
	}
	return nil
}

func (s *Store) Migrate(ctx context.Context) error {
	for _, stmt := range schemaStatements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate schema: %w", err)
		}
	}
	return nil
}

func (s *Store) EnsureAdmin(ctx context.Context, username, password string) error {
	u, err := s.GetUserByUsername(ctx, username)
	if err == nil {
		if u.Role != "admin" {
			_, err = s.db.ExecContext(ctx, `UPDATE users SET role='admin', enabled=1 WHERE id=?`, u.ID)
			if err != nil {
				return fmt.Errorf("promote admin: %w", err)
			}
		} else if !u.Enabled {
			_, err = s.db.ExecContext(ctx, `UPDATE users SET enabled=1 WHERE id=?`, u.ID)
			if err != nil {
				return fmt.Errorf("enable admin: %w", err)
			}
		}
		return nil
	}
	if !errors.Is(err, ErrNotFound) {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	now := time.Now().Unix()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO users (
			username, password_hash, role, enabled, expires_at, max_conns,
			quota_day_mb, quota_month_mb, quota_total_mb, created_at
		) VALUES (?, ?, 'admin', 1, 0, 16, 0, 0, 0, ?)
	`, username, string(hash), now)
	if err != nil {
		return fmt.Errorf("insert admin user: %w", err)
	}
	return nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (User, error) {
	const q = `
		SELECT id, username, password_hash, role, enabled, expires_at, max_conns,
		       quota_day_mb, quota_month_mb, quota_total_mb, created_at
		FROM users WHERE username = ?
	`
	var u User
	var enabledInt int
	err := s.db.QueryRowContext(ctx, q, username).Scan(
		&u.ID,
		&u.Username,
		&u.PasswordHash,
		&u.Role,
		&enabledInt,
		&u.ExpiresAt,
		&u.MaxConns,
		&u.QuotaDayMB,
		&u.QuotaMonthMB,
		&u.QuotaTotalMB,
		&u.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("get user: %w", err)
	}
	u.Enabled = enabledInt == 1
	return u, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	const q = `
		SELECT id, username, password_hash, role, enabled, expires_at, max_conns,
		       quota_day_mb, quota_month_mb, quota_total_mb, created_at
		FROM users
		ORDER BY id ASC
	`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	out := make([]User, 0)
	for rows.Next() {
		var u User
		var enabledInt int
		if err := rows.Scan(
			&u.ID,
			&u.Username,
			&u.PasswordHash,
			&u.Role,
			&enabledInt,
			&u.ExpiresAt,
			&u.MaxConns,
			&u.QuotaDayMB,
			&u.QuotaMonthMB,
			&u.QuotaTotalMB,
			&u.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan users: %w", err)
		}
		u.Enabled = enabledInt == 1
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) CreateUser(ctx context.Context, in UserCreateInput) (User, error) {
	if in.Role == "" {
		in.Role = "user"
	}
	if in.MaxConns <= 0 {
		in.MaxConns = 1
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}
	now := time.Now().Unix()
	enabled := 0
	if in.Enabled {
		enabled = 1
	}

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO users (
			username, password_hash, role, enabled, expires_at, max_conns,
			quota_day_mb, quota_month_mb, quota_total_mb, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		in.Username,
		string(hash),
		in.Role,
		enabled,
		in.ExpiresAt,
		in.MaxConns,
		in.QuotaDayMB,
		in.QuotaMonthMB,
		in.QuotaTotalMB,
		now,
	)
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("get user id: %w", err)
	}

	u, err := s.GetUserByUsername(ctx, in.Username)
	if err != nil {
		return User{}, err
	}
	u.ID = id
	return u, nil
}

func (s *Store) UpdateUserByUsername(ctx context.Context, username string, in UserUpdateInput) (User, error) {
	u, err := s.GetUserByUsername(ctx, username)
	if err != nil {
		return User{}, err
	}

	passwordHash := u.PasswordHash
	if in.Password != nil {
		pwd := strings.TrimSpace(*in.Password)
		if pwd == "" {
			return User{}, fmt.Errorf("password cannot be empty")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
		if err != nil {
			return User{}, fmt.Errorf("hash password: %w", err)
		}
		passwordHash = string(hash)
	}

	role := u.Role
	if in.Role != nil {
		role = strings.TrimSpace(*in.Role)
		if role != "admin" && role != "user" {
			return User{}, fmt.Errorf("role must be admin or user")
		}
	}

	enabled := u.Enabled
	if in.Enabled != nil {
		enabled = *in.Enabled
	}

	expiresAt := u.ExpiresAt
	if in.ExpiresAt != nil {
		expiresAt = *in.ExpiresAt
	}

	maxConns := u.MaxConns
	if in.MaxConns != nil {
		if *in.MaxConns <= 0 {
			return User{}, fmt.Errorf("max_conns must be > 0")
		}
		maxConns = *in.MaxConns
	}

	quotaDayMB := u.QuotaDayMB
	if in.QuotaDayMB != nil {
		if *in.QuotaDayMB < 0 {
			return User{}, fmt.Errorf("quota_day_mb must be >= 0")
		}
		quotaDayMB = *in.QuotaDayMB
	}

	quotaMonthMB := u.QuotaMonthMB
	if in.QuotaMonthMB != nil {
		if *in.QuotaMonthMB < 0 {
			return User{}, fmt.Errorf("quota_month_mb must be >= 0")
		}
		quotaMonthMB = *in.QuotaMonthMB
	}

	quotaTotalMB := u.QuotaTotalMB
	if in.QuotaTotalMB != nil {
		if *in.QuotaTotalMB < 0 {
			return User{}, fmt.Errorf("quota_total_mb must be >= 0")
		}
		quotaTotalMB = *in.QuotaTotalMB
	}

	enabledInt := 0
	if enabled {
		enabledInt = 1
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE users
		SET password_hash=?, role=?, enabled=?, expires_at=?, max_conns=?,
		    quota_day_mb=?, quota_month_mb=?, quota_total_mb=?
		WHERE id=?
	`,
		passwordHash,
		role,
		enabledInt,
		expiresAt,
		maxConns,
		quotaDayMB,
		quotaMonthMB,
		quotaTotalMB,
		u.ID,
	)
	if err != nil {
		return User{}, fmt.Errorf("update user: %w", err)
	}

	return s.GetUserByUsername(ctx, username)
}

func (s *Store) DeleteUserByUsername(ctx context.Context, username string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE username = ?`, username)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete user rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ResetUsageByUsername(ctx context.Context, username string, now time.Time) error {
	u, err := s.GetUserByUsername(ctx, username)
	if err != nil {
		return err
	}

	dayKey := now.Format("2006-01-02")
	monthKey := now.Format("2006-01")
	nowUnix := now.Unix()

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO usage_counters (user_id, day_key, day_bytes, month_key, month_bytes, total_bytes, updated_at)
		VALUES (?, ?, 0, ?, 0, 0, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			day_key=excluded.day_key,
			day_bytes=0,
			month_key=excluded.month_key,
			month_bytes=0,
			total_bytes=0,
			updated_at=excluded.updated_at
	`, u.ID, dayKey, monthKey, nowUnix)
	if err != nil {
		return fmt.Errorf("reset usage: %w", err)
	}
	return nil
}

func (s *Store) GetUsage(ctx context.Context, userID int64, now time.Time) (Usage, error) {
	const q = `SELECT day_key, day_bytes, month_key, month_bytes, total_bytes, updated_at FROM usage_counters WHERE user_id = ?`
	var u Usage
	err := s.db.QueryRowContext(ctx, q, userID).Scan(
		&u.DayKey,
		&u.DayBytes,
		&u.MonthKey,
		&u.MonthBytes,
		&u.TotalBytes,
		&u.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		dayKey := now.Format("2006-01-02")
		monthKey := now.Format("2006-01")
		return Usage{DayKey: dayKey, MonthKey: monthKey}, nil
	}
	if err != nil {
		return Usage{}, fmt.Errorf("get usage: %w", err)
	}

	dayKey := now.Format("2006-01-02")
	monthKey := now.Format("2006-01")
	if u.DayKey != dayKey {
		u.DayKey = dayKey
		u.DayBytes = 0
	}
	if u.MonthKey != monthKey {
		u.MonthKey = monthKey
		u.MonthBytes = 0
	}
	return u, nil
}

func (s *Store) AddUsage(ctx context.Context, userID int64, bytes int64, now time.Time) error {
	if bytes <= 0 {
		return nil
	}

	dayKey := now.Format("2006-01-02")
	monthKey := now.Format("2006-01")
	nowUnix := now.Unix()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin usage tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO usage_counters (user_id, day_key, day_bytes, month_key, month_bytes, total_bytes, updated_at)
		VALUES (?, ?, 0, ?, 0, 0, ?)
		ON CONFLICT(user_id) DO NOTHING
	`, userID, dayKey, monthKey, nowUnix)
	if err != nil {
		return fmt.Errorf("ensure usage row: %w", err)
	}

	const q = `SELECT day_key, day_bytes, month_key, month_bytes, total_bytes FROM usage_counters WHERE user_id = ?`
	var currDayKey, currMonthKey string
	var dayBytes, monthBytes, totalBytes int64
	if err := tx.QueryRowContext(ctx, q, userID).Scan(&currDayKey, &dayBytes, &currMonthKey, &monthBytes, &totalBytes); err != nil {
		return fmt.Errorf("load usage row: %w", err)
	}
	if currDayKey != dayKey {
		dayBytes = 0
	}
	if currMonthKey != monthKey {
		monthBytes = 0
	}
	dayBytes += bytes
	monthBytes += bytes
	totalBytes += bytes

	_, err = tx.ExecContext(ctx, `
		UPDATE usage_counters
		SET day_key=?, day_bytes=?, month_key=?, month_bytes=?, total_bytes=?, updated_at=?
		WHERE user_id=?
	`, dayKey, dayBytes, monthKey, monthBytes, totalBytes, nowUnix, userID)
	if err != nil {
		return fmt.Errorf("update usage row: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit usage tx: %w", err)
	}
	return nil
}

func (s *Store) LogDomain(ctx context.Context, userID int64, domain string, now time.Time) error {
	if domain == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO domain_logs (user_id, domain, ts) VALUES (?, ?, ?)`, userID, domain, now.Unix())
	if err != nil {
		return fmt.Errorf("insert domain log: %w", err)
	}
	return nil
}

func (s *Store) InsertSessionStart(ctx context.Context, username, proto, remoteAddr, targetAddr string, startedAt time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (username, proto, remote_addr, target_addr, started_at, ended_at, bytes_up, bytes_down)
		VALUES (?, ?, ?, ?, ?, 0, 0, 0)
	`, username, proto, remoteAddr, targetAddr, startedAt.Unix())
	if err != nil {
		return 0, fmt.Errorf("insert session start: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get session id: %w", err)
	}
	return id, nil
}

func (s *Store) EndSession(ctx context.Context, sessionID int64, bytesUp, bytesDown int64, endedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE sessions
		SET ended_at=?, bytes_up=?, bytes_down=?
		WHERE id=?
	`, endedAt.Unix(), bytesUp, bytesDown, sessionID)
	if err != nil {
		return fmt.Errorf("end session: %w", err)
	}
	return nil
}

func (s *Store) TopDomainsSince(ctx context.Context, sinceUnix int64, limit int) ([]DomainStat, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT domain, COUNT(*) AS c
		FROM domain_logs
		WHERE ts >= ?
		GROUP BY domain
		ORDER BY c DESC
		LIMIT ?
	`, sinceUnix, limit)
	if err != nil {
		return nil, fmt.Errorf("top domains query: %w", err)
	}
	defer rows.Close()

	out := make([]DomainStat, 0, limit)
	for rows.Next() {
		var item DomainStat
		if err := rows.Scan(&item.Domain, &item.Count); err != nil {
			return nil, fmt.Errorf("scan top domains: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) RecentDomainLogs(ctx context.Context, limit int) ([]RecentDomainLog, error) {
	if limit <= 0 {
		limit = 30
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.username, d.domain, d.ts
		FROM domain_logs d
		JOIN users u ON u.id = d.user_id
		ORDER BY d.id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("recent domain logs query: %w", err)
	}
	defer rows.Close()

	out := make([]RecentDomainLog, 0, limit)
	for rows.Next() {
		var item RecentDomainLog
		if err := rows.Scan(&item.Username, &item.Domain, &item.TS); err != nil {
			return nil, fmt.Errorf("scan recent domain logs: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
