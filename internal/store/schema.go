package store

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		enabled INTEGER NOT NULL DEFAULT 1,
		expires_at INTEGER NOT NULL DEFAULT 0,
		max_conns INTEGER NOT NULL DEFAULT 1,
		quota_day_mb INTEGER NOT NULL DEFAULT 0,
		quota_month_mb INTEGER NOT NULL DEFAULT 0,
		quota_total_mb INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL
	);`,
	`CREATE TABLE IF NOT EXISTS usage_counters (
		user_id INTEGER PRIMARY KEY,
		day_key TEXT NOT NULL,
		day_bytes INTEGER NOT NULL DEFAULT 0,
		month_key TEXT NOT NULL,
		month_bytes INTEGER NOT NULL DEFAULT 0,
		total_bytes INTEGER NOT NULL DEFAULT 0,
		updated_at INTEGER NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);`,
	`CREATE TABLE IF NOT EXISTS domain_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		domain TEXT NOT NULL,
		ts INTEGER NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);`,
	`CREATE INDEX IF NOT EXISTS idx_domain_logs_user_ts ON domain_logs(user_id, ts);`,
	`CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL,
		proto TEXT NOT NULL,
		remote_addr TEXT NOT NULL,
		target_addr TEXT NOT NULL,
		started_at INTEGER NOT NULL,
		ended_at INTEGER NOT NULL DEFAULT 0,
		bytes_up INTEGER NOT NULL DEFAULT 0,
		bytes_down INTEGER NOT NULL DEFAULT 0
	);`,
	`CREATE INDEX IF NOT EXISTS idx_sessions_username_started ON sessions(username, started_at);`,
	`CREATE TABLE IF NOT EXISTS audit_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		actor TEXT NOT NULL,
		action TEXT NOT NULL,
		detail TEXT NOT NULL,
		ts INTEGER NOT NULL
	);`,
	`CREATE INDEX IF NOT EXISTS idx_audit_logs_ts ON audit_logs(ts);`,
}
