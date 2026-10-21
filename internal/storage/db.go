// Package storage implements the PostgreSQL persistence layer of the Iota
// Coder bot: schema migrations, users, settings, the Telegram file cache,
// analytics events, download history and group tracking.
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Iotatg/iota-downloader/internal/logger"
	_ "github.com/lib/pq"
)

// DB wraps the database connection pool.
type DB struct {
	sql *sql.DB
	log *logger.Logger
}

// Options configures the database connection.
type Options struct {
	DSN         string
	PoolSize    int
	MaxOverflow int
	PoolTimeout time.Duration
	Logger      *logger.Logger
}

// Open connects to PostgreSQL and prepares the connection pool.
func Open(ctx context.Context, opts Options) (*DB, error) {
	db, err := sql.Open("postgres", opts.DSN)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	poolSize := opts.PoolSize
	if poolSize <= 0 {
		poolSize = 5
	}
	maxOverflow := opts.MaxOverflow
	if maxOverflow < 0 {
		maxOverflow = 0
	}
	db.SetMaxOpenConns(poolSize + maxOverflow)
	db.SetMaxIdleConns(poolSize)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	timeout := opts.PoolTimeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{sql: db, log: opts.Logger}, nil
}

// SQL exposes the underlying connection pool.
func (d *DB) SQL() *sql.DB { return d.sql }

// Close releases the connection pool.
func (d *DB) Close() error {
	if d == nil || d.sql == nil {
		return nil
	}
	return d.sql.Close()
}

// migration is a single forward-only schema change.
type migration struct {
	version string
	stmts   []string
}

var migrations = []migration{
	{
		version: "0001_initial",
		stmts: []string{
			`CREATE TABLE IF NOT EXISTS downloaded_files (
				id BIGSERIAL PRIMARY KEY,
				url TEXT NOT NULL UNIQUE,
				file_id TEXT NOT NULL,
				date_added TIMESTAMPTZ DEFAULT now(),
				file_type TEXT
			)`,
			`CREATE INDEX IF NOT EXISTS ix_downloaded_files_date_added ON downloaded_files (date_added)`,

			`CREATE TABLE IF NOT EXISTS users (
				user_id BIGINT PRIMARY KEY,
				user_name TEXT,
				user_username TEXT,
				chat_type TEXT DEFAULT 'private',
				language TEXT,
				has_dm BOOLEAN NOT NULL DEFAULT false,
				status TEXT DEFAULT 'active',
				referred_by BIGINT,
				source TEXT
			)`,
			`CREATE INDEX IF NOT EXISTS ix_users_user_username ON users (user_username)`,
			`CREATE INDEX IF NOT EXISTS ix_users_chat_type ON users (chat_type)`,
			`CREATE INDEX IF NOT EXISTS ix_users_status ON users (status)`,
			`CREATE INDEX IF NOT EXISTS ix_users_has_dm ON users (has_dm)`,

			`CREATE TABLE IF NOT EXISTS groups (
				id BIGINT PRIMARY KEY,
				title TEXT,
				username TEXT,
				chat_type TEXT,
				status TEXT NOT NULL DEFAULT 'active',
				member_count BIGINT NOT NULL DEFAULT 0,
				last_thread_id BIGINT,
				created_at TIMESTAMPTZ DEFAULT now(),
				updated_at TIMESTAMPTZ DEFAULT now()
			)`,
			`CREATE INDEX IF NOT EXISTS ix_groups_status ON groups (status)`,

			`CREATE TABLE IF NOT EXISTS group_members (
				group_id BIGINT NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
				user_id BIGINT NOT NULL REFERENCES users (user_id) ON DELETE CASCADE,
				last_seen_at TIMESTAMPTZ DEFAULT now(),
				PRIMARY KEY (group_id, user_id)
			)`,
			`CREATE INDEX IF NOT EXISTS ix_group_members_user_id ON group_members (user_id)`,
			`CREATE INDEX IF NOT EXISTS ix_group_members_group_id ON group_members (group_id)`,

			`CREATE TABLE IF NOT EXISTS analytics_events (
				id BIGSERIAL PRIMARY KEY,
				user_id BIGINT NOT NULL,
				chat_type TEXT,
				action_name TEXT NOT NULL,
				created_at TIMESTAMPTZ DEFAULT now()
			)`,
			`CREATE INDEX IF NOT EXISTS ix_analytics_events_action_name_created_at ON analytics_events (action_name, created_at)`,
			`CREATE INDEX IF NOT EXISTS ix_analytics_events_created_action ON analytics_events (created_at, action_name)`,

			`CREATE TABLE IF NOT EXISTS settings (
				id BIGSERIAL PRIMARY KEY,
				user_id BIGINT,
				captions TEXT NOT NULL DEFAULT 'off',
				delete_message TEXT NOT NULL DEFAULT 'off',
				info_buttons TEXT NOT NULL DEFAULT 'off',
				url_button TEXT NOT NULL DEFAULT 'off',
				audio_button TEXT NOT NULL DEFAULT 'off',
				file_button TEXT NOT NULL DEFAULT 'off',
				video_quality TEXT NOT NULL DEFAULT 'best',
				as_document TEXT NOT NULL DEFAULT 'off',
				audio_format TEXT NOT NULL DEFAULT 'mp3',
				CONSTRAINT uq_settings_user_id UNIQUE (user_id)
			)`,
			`CREATE INDEX IF NOT EXISTS ix_settings_user_id ON settings (user_id)`,

			`CREATE TABLE IF NOT EXISTS download_history (
				id BIGSERIAL PRIMARY KEY,
				user_id BIGINT NOT NULL REFERENCES users (user_id) ON DELETE CASCADE,
				chat_id BIGINT,
				chat_type TEXT,
				service TEXT NOT NULL,
				url TEXT NOT NULL,
				title TEXT,
				file_type TEXT,
				file_id TEXT,
				file_size_bytes BIGINT,
				duration_seconds DOUBLE PRECISION,
				status TEXT NOT NULL DEFAULT 'success',
				error_message TEXT,
				created_at TIMESTAMPTZ DEFAULT now()
			)`,
			`CREATE INDEX IF NOT EXISTS ix_download_history_user_id ON download_history (user_id)`,
			`CREATE INDEX IF NOT EXISTS ix_download_history_created_at ON download_history (created_at)`,
			`CREATE INDEX IF NOT EXISTS ix_download_history_service ON download_history (service)`,
			`CREATE INDEX IF NOT EXISTS ix_download_history_status ON download_history (status)`,
		},
	},
}

// Init applies pending migrations and synchronises the sequences.
func (d *DB) Init(ctx context.Context) error {
	started := time.Now()
	if _, err := d.sql.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied, err := d.appliedVersions(ctx)
	if err != nil {
		return err
	}

	for _, item := range migrations {
		if applied[item.version] {
			continue
		}
		tx, err := d.sql.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", item.version, err)
		}
		for _, stmt := range item.stmts {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migration %s failed: %w", item.version, err)
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT DO NOTHING`, item.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", item.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", item.version, err)
		}
		if d.log != nil {
			d.log.Info("applied schema migration %s", item.version)
		}
	}

	if err := d.syncSequences(ctx); err != nil {
		return err
	}

	if d.log != nil {
		d.log.Perf("db_init", float64(time.Since(started).Milliseconds()), logger.Fields{})
	}
	return nil
}

func (d *DB) appliedVersions(ctx context.Context) (map[string]bool, error) {
	rows, err := d.sql.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan schema_migrations: %w", err)
		}
		applied[version] = true
	}
	return applied, rows.Err()
}

func (d *DB) syncSequences(ctx context.Context) error {
	tables := []string{"downloaded_files", "analytics_events", "settings", "download_history"}
	for _, table := range tables {
		stmt := fmt.Sprintf(`SELECT setval(pg_get_serial_sequence('%s','id'), COALESCE(MAX(id),0)+1, false) FROM %s`, table, table)
		if _, err := d.sql.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("sync sequence %s: %w", table, err)
		}
	}
	return nil
}
