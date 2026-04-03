package web

import (
	"context"
	"encoding/csv"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"proxy-center/internal/auth"
	"proxy-center/internal/session"
	"proxy-center/internal/store"
	"proxy-center/internal/upstream"
)

type Server struct {
	addr     string
	authSvc  *auth.Service
	sessions *session.Manager
	store    *store.Store
	router   *upstream.Router
	srv      *http.Server
}

type ctxKey string

const adminUserKey ctxKey = "admin_user"

func NewServer(addr string, authSvc *auth.Service, sessions *session.Manager, st *store.Store, router *upstream.Router) *Server {
	return &Server{
		addr:     addr,
		authSvc:  authSvc,
		sessions: sessions,
		store:    st,
		router:   router,
	}
}

func (s *Server) Start(ctx context.Context) error {
	r := chi.NewRouter()
	healthHandler := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}
	r.Get("/healthz", healthHandler)
	r.Get("/health", healthHandler)
	r.Get("/api/health", healthHandler)

	r.Group(func(r chi.Router) {
		r.Use(s.adminAuth)
		r.Get("/", s.handleDashboardPage)
		r.Get("/api/dashboard", s.handleDashboard)
		r.Get("/api/users", s.handleListUsers)
		r.Post("/api/users", s.handleCreateUser)
		r.Post("/api/users/import-csv", s.handleImportUsersCSV)
		r.Patch("/api/users/{username}", s.handleUpdateUser)
		r.Post("/api/users/{username}/kick", s.handleKickUser)
		r.Post("/api/users/{username}/reset-usage", s.handleResetUsage)
		r.Delete("/api/users/{username}", s.handleDeleteUser)
		r.Get("/api/active-connections", s.handleActiveConnections)
		r.Get("/api/upstreams", s.handleUpstreams)
	})

	s.srv = &http.Server{
		Addr:              s.addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.srv.Shutdown(shutdownCtx)
	}()

	err := s.srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) adminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := parseBasicAuth(r.Header.Get("Authorization"))
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="proxy-center-admin"`)
			http.Error(w, "admin auth required", http.StatusUnauthorized)
			return
		}

		admin, err := s.authSvc.AuthenticateAndAuthorize(r.Context(), u, p, time.Now())
		if err != nil || admin.Role != "admin" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), adminUserKey, admin.Username)))
	})
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	out := sanitizeUsers(users)
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Username     string `json:"username"`
		Password     string `json:"password"`
		Role         string `json:"role"`
		Enabled      *bool  `json:"enabled"`
		ExpiresAt    int64  `json:"expires_at"`
		MaxConns     int    `json:"max_conns"`
		QuotaDayMB   int64  `json:"quota_day_mb"`
		QuotaMonthMB int64  `json:"quota_month_mb"`
		QuotaTotalMB int64  `json:"quota_total_mb"`
	}

	var in payload
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(in.Username) == "" || strings.TrimSpace(in.Password) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "username/password required"})
		return
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}

	u, err := s.store.CreateUser(r.Context(), store.UserCreateInput{
		Username:     strings.TrimSpace(in.Username),
		Password:     in.Password,
		Role:         strings.TrimSpace(in.Role),
		Enabled:      enabled,
		ExpiresAt:    in.ExpiresAt,
		MaxConns:     in.MaxConns,
		QuotaDayMB:   in.QuotaDayMB,
		QuotaMonthMB: in.QuotaMonthMB,
		QuotaTotalMB: in.QuotaTotalMB,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":       u.ID,
		"username": u.Username,
	})
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(chi.URLParam(r, "username"))
	if username == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "username required"})
		return
	}

	type payload struct {
		Password     *string `json:"password"`
		Role         *string `json:"role"`
		Enabled      *bool   `json:"enabled"`
		ExpiresAt    *int64  `json:"expires_at"`
		MaxConns     *int    `json:"max_conns"`
		QuotaDayMB   *int64  `json:"quota_day_mb"`
		QuotaMonthMB *int64  `json:"quota_month_mb"`
		QuotaTotalMB *int64  `json:"quota_total_mb"`
	}

	var in payload
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}

	actor := ""
	if v := r.Context().Value(adminUserKey); v != nil {
		if s, ok := v.(string); ok {
			actor = s
		}
	}
	if actor == username {
		if in.Enabled != nil && !*in.Enabled {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "cannot disable current admin"})
			return
		}
		if in.Role != nil && strings.TrimSpace(*in.Role) != "admin" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "cannot demote current admin"})
			return
		}
	}

	updated, err := s.store.UpdateUserByUsername(r.Context(), username, store.UserUpdateInput{
		Password:     in.Password,
		Role:         in.Role,
		Enabled:      in.Enabled,
		ExpiresAt:    in.ExpiresAt,
		MaxConns:     in.MaxConns,
		QuotaDayMB:   in.QuotaDayMB,
		QuotaMonthMB: in.QuotaMonthMB,
		QuotaTotalMB: in.QuotaTotalMB,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	shouldKick := false
	now := time.Now()
	if !updated.Enabled || (updated.ExpiresAt > 0 && now.Unix() >= updated.ExpiresAt) {
		shouldKick = true
	} else {
		if err := s.authSvc.Authorize(r.Context(), updated, now); err != nil {
			shouldKick = true
		}
	}

	if in.MaxConns != nil {
		shouldKick = true
	}

	kicked := 0
	if shouldKick {
		kicked = s.sessions.KickUser(updated.Username)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"item":   sanitizeUser(updated),
		"kicked": kicked,
	})
}

func (s *Server) handleKickUser(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(chi.URLParam(r, "username"))
	if username == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "username required"})
		return
	}
	kicked := s.sessions.KickUser(username)
	writeJSON(w, http.StatusOK, map[string]any{"username": username, "kicked": kicked})
}

func (s *Server) handleResetUsage(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(chi.URLParam(r, "username"))
	if username == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "username required"})
		return
	}

	if err := s.store.ResetUsageByUsername(r.Context(), username, time.Now()); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"username": username, "reset": true})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(chi.URLParam(r, "username"))
	if username == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "username required"})
		return
	}

	actor := ""
	if v := r.Context().Value(adminUserKey); v != nil {
		if s, ok := v.(string); ok {
			actor = s
		}
	}
	if actor == username {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "cannot delete current admin"})
		return
	}

	u, err := s.store.GetUserByUsername(r.Context(), username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if u.Role == "admin" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "deleting admin user is not allowed"})
		return
	}

	kicked := s.sessions.KickUser(username)
	if err := s.store.DeleteUserByUsername(r.Context(), username); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"username": username, "deleted": true, "kicked": kicked})
}

func (s *Server) handleImportUsersCSV(w http.ResponseWriter, r *http.Request) {
	csvText, err := readCSVTextFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	reader := csv.NewReader(strings.NewReader(csvText))
	reader.TrimLeadingSpace = true
	rows, err := reader.ReadAll()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid csv"})
		return
	}
	if len(rows) < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "empty csv"})
		return
	}

	headers := make(map[string]int)
	for i, h := range rows[0] {
		headers[strings.ToLower(strings.TrimSpace(h))] = i
	}
	if _, ok := headers["username"]; !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "csv requires username column"})
		return
	}

	created := 0
	updated := 0
	failed := 0
	errorsOut := make([]string, 0)

	for idx := 1; idx < len(rows); idx++ {
		lineNo := idx + 1
		row := rows[idx]
		username := readCSVCell(row, headers, "username")
		if username == "" {
			continue
		}

		password := readCSVCell(row, headers, "password")
		role := readCSVCell(row, headers, "role")
		enabledVal, hasEnabled, err := parseOptionalBool(readCSVCell(row, headers, "enabled"))
		if err != nil {
			failed++
			errorsOut = append(errorsOut, "line "+strconv.Itoa(lineNo)+" ("+username+"): enabled parse error")
			continue
		}
		expiresAt, hasExpires, err := parseOptionalInt64(readCSVCell(row, headers, "expires_at"))
		if err != nil {
			failed++
			errorsOut = append(errorsOut, "line "+strconv.Itoa(lineNo)+" ("+username+"): expires_at parse error")
			continue
		}
		maxConns, hasMaxConns, err := parseOptionalInt(readCSVCell(row, headers, "max_conns"))
		if err != nil {
			failed++
			errorsOut = append(errorsOut, "line "+strconv.Itoa(lineNo)+" ("+username+"): max_conns parse error")
			continue
		}
		quotaDay, hasQuotaDay, err := parseOptionalInt64(readCSVCell(row, headers, "quota_day_mb"))
		if err != nil {
			failed++
			errorsOut = append(errorsOut, "line "+strconv.Itoa(lineNo)+" ("+username+"): quota_day_mb parse error")
			continue
		}
		quotaMonth, hasQuotaMonth, err := parseOptionalInt64(readCSVCell(row, headers, "quota_month_mb"))
		if err != nil {
			failed++
			errorsOut = append(errorsOut, "line "+strconv.Itoa(lineNo)+" ("+username+"): quota_month_mb parse error")
			continue
		}
		quotaTotal, hasQuotaTotal, err := parseOptionalInt64(readCSVCell(row, headers, "quota_total_mb"))
		if err != nil {
			failed++
			errorsOut = append(errorsOut, "line "+strconv.Itoa(lineNo)+" ("+username+"): quota_total_mb parse error")
			continue
		}

		_, err = s.store.GetUserByUsername(r.Context(), username)
		if errors.Is(err, store.ErrNotFound) {
			if strings.TrimSpace(password) == "" {
				failed++
				errorsOut = append(errorsOut, "line "+strconv.Itoa(lineNo)+" ("+username+"): new user requires password")
				continue
			}
			enabled := true
			if hasEnabled {
				enabled = enabledVal
			}
			in := store.UserCreateInput{
				Username: username,
				Password: password,
				Role:     role,
				Enabled:  enabled,
			}
			if hasExpires {
				in.ExpiresAt = expiresAt
			}
			if hasMaxConns {
				in.MaxConns = maxConns
			}
			if hasQuotaDay {
				in.QuotaDayMB = quotaDay
			}
			if hasQuotaMonth {
				in.QuotaMonthMB = quotaMonth
			}
			if hasQuotaTotal {
				in.QuotaTotalMB = quotaTotal
			}
			if _, err := s.store.CreateUser(r.Context(), in); err != nil {
				failed++
				errorsOut = append(errorsOut, "line "+strconv.Itoa(lineNo)+" ("+username+"): "+err.Error())
				continue
			}
			created++
			continue
		}
		if err != nil {
			failed++
			errorsOut = append(errorsOut, "line "+strconv.Itoa(lineNo)+" ("+username+"): "+err.Error())
			continue
		}

		update := store.UserUpdateInput{}
		if strings.TrimSpace(password) != "" {
			pwd := password
			update.Password = &pwd
		}
		if strings.TrimSpace(role) != "" {
			r := role
			update.Role = &r
		}
		if hasEnabled {
			v := enabledVal
			update.Enabled = &v
		}
		if hasExpires {
			v := expiresAt
			update.ExpiresAt = &v
		}
		if hasMaxConns {
			v := maxConns
			update.MaxConns = &v
		}
		if hasQuotaDay {
			v := quotaDay
			update.QuotaDayMB = &v
		}
		if hasQuotaMonth {
			v := quotaMonth
			update.QuotaMonthMB = &v
		}
		if hasQuotaTotal {
			v := quotaTotal
			update.QuotaTotalMB = &v
		}

		if _, err := s.store.UpdateUserByUsername(r.Context(), username, update); err != nil {
			failed++
			errorsOut = append(errorsOut, "line "+strconv.Itoa(lineNo)+" ("+username+"): "+err.Error())
			continue
		}
		updated++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"created": created,
		"updated": updated,
		"failed":  failed,
		"errors":  errorsOut,
	})
}

func readCSVTextFromRequest(r *http.Request) (string, error) {
	ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			return "", err
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			return "", err
		}
		defer file.Close()
		b, err := io.ReadAll(io.LimitReader(file, 8<<20))
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		return "", err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return "", errors.New("empty request body")
	}
	return string(b), nil
}

func readCSVCell(row []string, headers map[string]int, name string) string {
	idx, ok := headers[name]
	if !ok {
		return ""
	}
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func parseOptionalBool(raw string) (bool, bool, error) {
	if strings.TrimSpace(raw) == "" {
		return false, false, nil
	}
	v, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, false, err
	}
	return v, true, nil
}

func parseOptionalInt64(raw string) (int64, bool, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, false, nil
	}
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, false, err
	}
	return v, true, nil
}

func parseOptionalInt(raw string) (int, bool, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, false, nil
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, false, err
	}
	return v, true, nil
}

func (s *Server) handleActiveConnections(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.sessions.Snapshot()})
}

func (s *Server) handleUpstreams(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.router.Status()})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	active := s.sessions.Snapshot()
	sort.Slice(active, func(i, j int) bool {
		return active[i].Username < active[j].Username
	})

	enabledCount := 0
	for _, u := range users {
		if u.Enabled {
			enabledCount++
		}
	}

	topDomains, err := s.store.TopDomainsSince(r.Context(), time.Now().Add(-24*time.Hour).Unix(), 8)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	recent, err := s.store.RecentDomainLogs(r.Context(), 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	activeTotal := 0
	for _, item := range active {
		activeTotal += item.Count
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"generated_at": time.Now().Unix(),
		"overview": map[string]any{
			"users_total":     len(users),
			"users_enabled":   enabledCount,
			"active_sessions": activeTotal,
			"top_domains_24h": len(topDomains),
		},
		"users":              sanitizeUsers(users),
		"active_connections": active,
		"top_domains":        topDomains,
		"recent_domains":     recent,
		"upstreams":          s.router.Status(),
	})
}

func sanitizeUsers(users []store.User) []map[string]any {
	out := make([]map[string]any, 0, len(users))
	for _, u := range users {
		out = append(out, sanitizeUser(u))
	}
	return out
}

func sanitizeUser(u store.User) map[string]any {
	return map[string]any{
		"id":             u.ID,
		"username":       u.Username,
		"role":           u.Role,
		"enabled":        u.Enabled,
		"expires_at":     u.ExpiresAt,
		"max_conns":      u.MaxConns,
		"quota_day_mb":   u.QuotaDayMB,
		"quota_month_mb": u.QuotaMonthMB,
		"quota_total_mb": u.QuotaTotalMB,
		"created_at":     u.CreatedAt,
	}
}

func (s *Server) handleDashboardPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(dashboardHTML))
}

func parseBasicAuth(value string) (string, string, bool) {
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

var dashboardHTML = `<!doctype html>
<html lang="zh-CN">
<head>
	<meta charset="utf-8" />
	<meta name="viewport" content="width=device-width, initial-scale=1" />
	<title>Proxy Center Console</title>
	<style>
		@import url('https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@400;600;700&family=IBM+Plex+Mono:wght@400;500&display=swap');
		:root {
			--bg: #f6f6ef;
			--ink: #10221c;
			--card: rgba(255,255,255,0.86);
			--accent: #e45f2b;
			--accent-2: #1f7a8c;
			--line: #d8d4c6;
		}
		* { box-sizing: border-box; }
		body {
			margin: 0;
			font-family: 'Space Grotesk', sans-serif;
			color: var(--ink);
			background:
				radial-gradient(circle at 92% 10%, #ffe6be 0%, rgba(255,230,190,0) 32%),
				radial-gradient(circle at 8% 90%, #c8ecf3 0%, rgba(200,236,243,0) 28%),
				linear-gradient(165deg, #f3efe2 0%, #f9f7f0 45%, #f5f4ee 100%);
			min-height: 100vh;
		}
		.wrap {
			max-width: 1180px;
			margin: 0 auto;
			padding: 24px;
			animation: rise .45s ease;
		}
		@keyframes rise {
			from { opacity: 0; transform: translateY(14px); }
			to { opacity: 1; transform: translateY(0); }
		}
		.head {
			display: flex;
			align-items: end;
			justify-content: space-between;
			gap: 12px;
			margin-bottom: 18px;
		}
		.title {
			font-size: 34px;
			margin: 0;
			letter-spacing: .2px;
		}
		.sub {
			margin: 4px 0 0;
			color: #32564d;
			font-size: 14px;
		}
		.meta {
			font-family: 'IBM Plex Mono', monospace;
			font-size: 12px;
			opacity: .8;
		}
		.cards {
			display: grid;
			grid-template-columns: repeat(4, minmax(120px, 1fr));
			gap: 10px;
			margin-bottom: 16px;
		}
		.card {
			background: var(--card);
			border: 1px solid var(--line);
			border-radius: 16px;
			padding: 12px;
			backdrop-filter: blur(6px);
		}
		.card .k { font-size: 12px; opacity: .75; margin-bottom: 8px; }
		.card .v { font-size: 30px; font-weight: 700; line-height: 1; }
		.grid {
			display: grid;
			grid-template-columns: 1.1fr 1fr;
			gap: 12px;
		}
		.panel {
			background: var(--card);
			border: 1px solid var(--line);
			border-radius: 16px;
			padding: 14px;
		}
		.panel h3 {
			margin: 0 0 10px;
			font-size: 15px;
			letter-spacing: .2px;
		}
		table { width: 100%; border-collapse: collapse; font-size: 13px; }
		th, td { text-align: left; padding: 8px 6px; border-bottom: 1px dashed #d9d5c6; }
		th { font-size: 12px; opacity: .8; }
		.pill {
			display: inline-flex;
			align-items: center;
			padding: 3px 8px;
			border-radius: 999px;
			font-size: 11px;
			border: 1px solid;
		}
		.ok { color: #145f4d; border-color: #145f4d55; background: #ebfff9; }
		.bad { color: #8b2c1c; border-color: #8b2c1c55; background: #fff1ee; }
		.row {
			display: grid;
			grid-template-columns: 1fr 1fr;
			gap: 12px;
			margin-top: 12px;
		}
		.form {
			display: grid;
			grid-template-columns: repeat(2, minmax(0,1fr));
			gap: 8px;
			margin-top: 8px;
		}
		.form input {
			width: 100%;
			border: 1px solid #cfc9b9;
			border-radius: 10px;
			padding: 9px 10px;
			background: #fffdf8;
			font-family: 'IBM Plex Mono', monospace;
			font-size: 12px;
		}
		.btn {
			border: 0;
			border-radius: 10px;
			background: linear-gradient(120deg, var(--accent), #f18f01);
			color: white;
			padding: 10px 12px;
			font-weight: 700;
			cursor: pointer;
			transition: transform .12s ease;
		}
		.btn:hover { transform: translateY(-1px); }
		.btn.secondary {
			background: linear-gradient(120deg, var(--accent-2), #3ea5bc);
		}
		.hint {
			font-size: 12px;
			color: #33544c;
			margin-top: 8px;
			min-height: 16px;
		}
		@media (max-width: 960px) {
			.cards { grid-template-columns: repeat(2, minmax(0, 1fr)); }
			.grid { grid-template-columns: 1fr; }
			.row { grid-template-columns: 1fr; }
			.title { font-size: 28px; }
		}
	</style>
</head>
<body>
	<div class="wrap">
		<div class="head">
			<div>
				<h1 class="title">Proxy Center Console</h1>
				<p class="sub">SOCKS5 + HTTP 代理用户与会话控制台</p>
			</div>
			<div class="meta" id="ts">loading...</div>
		</div>

		<div class="cards">
			<div class="card"><div class="k">用户总数</div><div class="v" id="mUsers">0</div></div>
			<div class="card"><div class="k">启用用户</div><div class="v" id="mEnabled">0</div></div>
			<div class="card"><div class="k">活跃会话</div><div class="v" id="mActive">0</div></div>
			<div class="card"><div class="k">24h Top域名数</div><div class="v" id="mDomains">0</div></div>
		</div>

		<div class="grid">
			<section class="panel">
				<h3>活跃连接</h3>
				<table>
					<thead><tr><th>账号</th><th>连接数</th><th>操作</th></tr></thead>
					<tbody id="activeBody"></tbody>
				</table>
			</section>
			<section class="panel">
				<h3>上游池状态</h3>
				<table>
					<thead><tr><th>节点</th><th>模式</th><th>权重</th><th>健康</th></tr></thead>
					<tbody id="upBody"></tbody>
				</table>
			</section>
		</div>

		<div class="row">
			<section class="panel">
				<h3>24小时热门域名</h3>
				<table>
					<thead><tr><th>域名</th><th>次数</th></tr></thead>
					<tbody id="topBody"></tbody>
				</table>
			</section>
			<section class="panel">
				<h3>最近域名日志</h3>
				<table>
					<thead><tr><th>用户</th><th>域名</th><th>时间</th></tr></thead>
					<tbody id="recentBody"></tbody>
				</table>
			</section>
		</div>

		<section class="panel" style="margin-top:12px;">
			<h3>用户列表</h3>
			<table>
				<thead><tr><th>用户名</th><th>角色</th><th>状态</th><th>到期</th><th>并发</th><th>操作</th></tr></thead>
				<tbody id="usersBody"></tbody>
			</table>
		</section>

		<div class="row">
			<section class="panel" style="margin-top:12px;">
				<h3>新建用户</h3>
				<div class="form">
					<input id="uName" placeholder="username" />
					<input id="uPass" placeholder="password" />
					<input id="uMax" placeholder="max_conns" value="2" />
					<input id="uExp" placeholder="expires_at unix(秒), 0=不限" value="0" />
					<input id="uDay" placeholder="quota_day_mb" value="0" />
					<input id="uMonth" placeholder="quota_month_mb" value="0" />
					<input id="uTotal" placeholder="quota_total_mb" value="0" />
					<button class="btn" id="btnCreate">创建用户</button>
				</div>
			</section>

			<section class="panel" style="margin-top:12px;">
				<h3>更新用户策略</h3>
				<div class="form">
					<input id="eName" placeholder="username(必填)" />
					<input id="ePass" placeholder="new password(留空不改)" />
					<select id="eRole" style="border:1px solid #cfc9b9;border-radius:10px;padding:9px 10px;background:#fffdf8;font-family:'IBM Plex Mono', monospace;font-size:12px;">
						<option value="">role(不改)</option>
						<option value="user">user</option>
						<option value="admin">admin</option>
					</select>
					<select id="eEnabled" style="border:1px solid #cfc9b9;border-radius:10px;padding:9px 10px;background:#fffdf8;font-family:'IBM Plex Mono', monospace;font-size:12px;">
						<option value="">enabled(不改)</option>
						<option value="true">true</option>
						<option value="false">false</option>
					</select>
					<input id="eMax" placeholder="max_conns(留空不改)" />
					<input id="eExp" placeholder="expires_at unix(留空不改)" />
					<input id="eDay" placeholder="quota_day_mb(留空不改)" />
					<input id="eMonth" placeholder="quota_month_mb(留空不改)" />
					<input id="eTotal" placeholder="quota_total_mb(留空不改)" />
					<button class="btn secondary" id="btnUpdate">更新策略</button>
				</div>
			</section>
		</div>

		<section class="panel" style="margin-top:12px;">
			<h3>CSV 批量导入</h3>
			<textarea id="csvInput" style="width:100%;min-height:130px;border:1px solid #cfc9b9;border-radius:10px;padding:10px;background:#fffdf8;font-family:'IBM Plex Mono', monospace;font-size:12px;" placeholder="username,password,role,enabled,expires_at,max_conns,quota_day_mb,quota_month_mb,quota_total_mb\nu100,p100,user,true,0,2,1024,20480,0"></textarea>
			<div style="display:flex;gap:8px;margin-top:8px;">
				<button class="btn" id="btnImportCsv">导入 CSV</button>
			</div>
		</section>
		<p class="hint" id="hint"></p>
	</div>

	<script>
		const qs = (id) => document.getElementById(id);
		const esc = (v) => String(v ?? '').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;');

		async function load() {
			const res = await fetch('/api/dashboard', { cache: 'no-store' });
			if (!res.ok) throw new Error('load dashboard failed');
			const data = await res.json();

			qs('ts').textContent = 'updated ' + new Date(data.generated_at * 1000).toLocaleString();
			qs('mUsers').textContent = data.overview.users_total;
			qs('mEnabled').textContent = data.overview.users_enabled;
			qs('mActive').textContent = data.overview.active_sessions;
			qs('mDomains').textContent = data.overview.top_domains_24h;

			qs('activeBody').innerHTML = (data.active_connections || []).map(function(x) {
				return '<tr>' +
					'<td>' + esc(x.username) + '</td>' +
					'<td>' + x.count + '</td>' +
					'<td><button class="btn secondary" onclick="kickUser(\'' + encodeURIComponent(x.username) + '\')">踢线</button></td>' +
					'</tr>';
			}).join('') || '<tr><td colspan="3">暂无</td></tr>';

			qs('upBody').innerHTML = (data.upstreams || []).map(function(x) {
				const stateClass = x.healthy ? 'ok' : 'bad';
				const stateText = x.healthy ? 'healthy' : 'down';
				return '<tr>' +
					'<td>' + esc(x.addr) + '</td>' +
					'<td>' + esc(x.mode) + '</td>' +
					'<td>' + x.weight + '</td>' +
					'<td><span class="pill ' + stateClass + '">' + stateText + '</span></td>' +
					'</tr>';
			}).join('') || '<tr><td colspan="4">直连模式或无节点</td></tr>';

			qs('topBody').innerHTML = (data.top_domains || []).map(function(x) {
				return '<tr><td>' + esc(x.domain) + '</td><td>' + x.count + '</td></tr>';
			}).join('') || '<tr><td colspan="2">暂无</td></tr>';

			qs('recentBody').innerHTML = (data.recent_domains || []).map(function(x) {
				return '<tr><td>' + esc(x.username) + '</td><td>' + esc(x.domain) + '</td><td>' + new Date(x.ts * 1000).toLocaleString() + '</td></tr>';
			}).join('') || '<tr><td colspan="3">暂无</td></tr>';

			qs('usersBody').innerHTML = (data.users || []).map(function(x) {
				const status = x.enabled ? '启用' : '禁用';
				const exp = x.expires_at > 0 ? new Date(x.expires_at * 1000).toLocaleString() : '不限';
				return '<tr>' +
					'<td>' + esc(x.username) + '</td>' +
					'<td>' + esc(x.role) + '</td>' +
					'<td>' + status + '</td>' +
					'<td>' + exp + '</td>' +
					'<td>' + x.max_conns + '</td>' +
					'<td>' +
						'<button class="btn secondary" onclick="editUser(\'' + encodeURIComponent(x.username) + '\')">编辑</button> ' +
						'<button class="btn secondary" onclick="resetUsage(\'' + encodeURIComponent(x.username) + '\')">重置流量</button> ' +
						'<button class="btn secondary" onclick="deleteUser(\'' + encodeURIComponent(x.username) + '\')">删除</button>' +
					'</td>' +
					'</tr>';
			}).join('') || '<tr><td colspan="6">暂无用户</td></tr>';
		}

		function parseOptionalInt(v) {
			const t = (v || '').trim();
			if (t === '') return null;
			const n = parseInt(t, 10);
			if (Number.isNaN(n)) return null;
			return n;
		}

		function editUser(encodedName) {
			const username = decodeURIComponent(encodedName);
			qs('eName').value = username;
			qs('hint').textContent = '已填入编辑用户: ' + username;
		}
		window.editUser = editUser;

		async function resetUsage(encodedName) {
			const username = decodeURIComponent(encodedName);
			const ok = window.confirm('确认重置用户流量计数: ' + username + ' ?');
			if (!ok) return;
			const res = await fetch('/api/users/' + encodeURIComponent(username) + '/reset-usage', { method: 'POST' });
			const data = await res.json();
			if (!res.ok) {
				qs('hint').textContent = '重置失败: ' + (data.error || res.statusText);
				return;
			}
			qs('hint').textContent = '已重置流量: ' + username;
			await load();
		}
		window.resetUsage = resetUsage;

		async function deleteUser(encodedName) {
			const username = decodeURIComponent(encodedName);
			const ok = window.confirm('确认删除用户: ' + username + ' ?');
			if (!ok) return;
			const res = await fetch('/api/users/' + encodeURIComponent(username), { method: 'DELETE' });
			const data = await res.json();
			if (!res.ok) {
				qs('hint').textContent = '删除失败: ' + (data.error || res.statusText);
				return;
			}
			qs('hint').textContent = '已删除用户: ' + username + '，踢线 ' + (data.kicked || 0) + ' 条';
			await load();
		}
		window.deleteUser = deleteUser;

		async function kickUser(encodedName) {
			const username = decodeURIComponent(encodedName);
			const res = await fetch('/api/users/' + encodeURIComponent(username) + '/kick', { method: 'POST' });
			const data = await res.json();
			qs('hint').textContent = '踢线: ' + username + ' -> ' + (data.kicked ?? 0) + ' 条连接';
			await load();
		}
		window.kickUser = kickUser;

		qs('btnCreate').addEventListener('click', async () => {
			const payload = {
				username: qs('uName').value.trim(),
				password: qs('uPass').value,
				enabled: true,
				max_conns: parseInt(qs('uMax').value || '1', 10),
				expires_at: parseInt(qs('uExp').value || '0', 10),
				quota_day_mb: parseInt(qs('uDay').value || '0', 10),
				quota_month_mb: parseInt(qs('uMonth').value || '0', 10),
				quota_total_mb: parseInt(qs('uTotal').value || '0', 10)
			};
			const res = await fetch('/api/users', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(payload)
			});
			const data = await res.json();
			if (!res.ok) {
				qs('hint').textContent = '创建失败: ' + (data.error || res.statusText);
				return;
			}
			qs('hint').textContent = '已创建用户: ' + data.username;
			qs('uName').value = '';
			qs('uPass').value = '';
			await load();
		});

		qs('btnUpdate').addEventListener('click', async () => {
			const username = qs('eName').value.trim();
			if (!username) {
				qs('hint').textContent = '更新失败: username 必填';
				return;
			}

			const payload = {};
			const pwd = qs('ePass').value;
			if (pwd.trim() !== '') payload.password = pwd;

			const role = qs('eRole').value;
			if (role !== '') payload.role = role;

			const enabled = qs('eEnabled').value;
			if (enabled !== '') payload.enabled = (enabled === 'true');

			const maxConns = parseOptionalInt(qs('eMax').value);
			if (maxConns !== null) payload.max_conns = maxConns;
			const expires = parseOptionalInt(qs('eExp').value);
			if (expires !== null) payload.expires_at = expires;
			const qd = parseOptionalInt(qs('eDay').value);
			if (qd !== null) payload.quota_day_mb = qd;
			const qm = parseOptionalInt(qs('eMonth').value);
			if (qm !== null) payload.quota_month_mb = qm;
			const qt = parseOptionalInt(qs('eTotal').value);
			if (qt !== null) payload.quota_total_mb = qt;

			if (Object.keys(payload).length === 0) {
				qs('hint').textContent = '没有可更新字段';
				return;
			}

			const res = await fetch('/api/users/' + encodeURIComponent(username), {
				method: 'PATCH',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(payload)
			});
			const data = await res.json();
			if (!res.ok) {
				qs('hint').textContent = '更新失败: ' + (data.error || res.statusText);
				return;
			}
			qs('hint').textContent = '更新成功: ' + username + '，踢线 ' + (data.kicked || 0) + ' 条';
			qs('ePass').value = '';
			await load();
		});

		qs('btnImportCsv').addEventListener('click', async () => {
			const raw = qs('csvInput').value;
			if (!raw || raw.trim() === '') {
				qs('hint').textContent = '请先填写 CSV 内容';
				return;
			}
			const res = await fetch('/api/users/import-csv', {
				method: 'POST',
				headers: { 'Content-Type': 'text/csv' },
				body: raw
			});
			const data = await res.json();
			if (!res.ok) {
				qs('hint').textContent = '导入失败: ' + (data.error || res.statusText);
				return;
			}
			const errCount = (data.errors || []).length;
			qs('hint').textContent = '导入完成: 创建 ' + data.created + '，更新 ' + data.updated + '，失败 ' + data.failed + (errCount > 0 ? '（详情看返回 errors）' : '');
			await load();
		});

		load().catch(err => { qs('hint').textContent = '加载失败: ' + err.message; });
		setInterval(() => load().catch(() => {}), 5000);
	</script>
</body>
</html>`
