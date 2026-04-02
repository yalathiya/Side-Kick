package configdb

import (
	"database/sql"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// RateLimitRule defines a per-path rate limit override.
type RateLimitRule struct {
	ID            int    `db:"id" json:"id"`
	Path          string `db:"path" json:"path"`
	Limit         int    `db:"limit" json:"limit"`
	WindowSeconds int    `db:"window_seconds" json:"window_seconds"`
}

// Snapshot holds the current in-memory config loaded from SQLite.
type Snapshot struct {
	Config     map[string]string `json:"config"`
	RateLimits []RateLimitRule   `json:"rate_limits"`
	LoadedAt   time.Time         `json:"loaded_at"`
}

// DB manages the SQLite config database with hot-reload support.
type DB struct {
	db       *sqlx.DB
	current  atomic.Value // stores *Snapshot
	interval time.Duration
	stopCh   chan struct{}
}

// Open connects to the SQLite database, creates tables if needed, and loads initial config.
func Open(path string, reloadInterval int) (*DB, error) {
	db, err := sqlx.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Enable WAL mode for better concurrent read performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if err := createTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}

	cdb := &DB{
		db:       db,
		interval: time.Duration(reloadInterval) * time.Second,
		stopCh:   make(chan struct{}),
	}

	// Load initial config
	if err := cdb.reload(); err != nil {
		db.Close()
		return nil, fmt.Errorf("initial load: %w", err)
	}

	// Start background polling for hot reload
	go cdb.poll()

	return cdb, nil
}

// Current returns the latest in-memory config snapshot (lock-free read).
func (cdb *DB) Current() *Snapshot {
	if v := cdb.current.Load(); v != nil {
		return v.(*Snapshot)
	}
	return &Snapshot{Config: map[string]string{}}
}

// GetConfig returns a single config value, or empty string if not found.
func (cdb *DB) GetConfig(key string) string {
	return cdb.Current().Config[key]
}

// SetConfig upserts a config key-value pair and triggers a reload.
func (cdb *DB) SetConfig(key, value string) error {
	_, err := cdb.db.Exec(
		`INSERT INTO config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?`,
		key, value, value,
	)
	if err != nil {
		return err
	}
	return cdb.reload()
}

// DeleteConfig removes a config key and triggers a reload.
func (cdb *DB) DeleteConfig(key string) error {
	_, err := cdb.db.Exec(`DELETE FROM config WHERE key = ?`, key)
	if err != nil {
		return err
	}
	return cdb.reload()
}

// GetRateLimits returns all rate limit rules from the current snapshot.
func (cdb *DB) GetRateLimits() []RateLimitRule {
	return cdb.Current().RateLimits
}

// RateLimitForPath returns the rate limit rule for a specific path, or nil if none.
func (cdb *DB) RateLimitForPath(path string) *RateLimitRule {
	for _, rule := range cdb.Current().RateLimits {
		if rule.Path == path {
			return &rule
		}
	}
	return nil
}

// AddRateLimit inserts a new rate limit rule and triggers a reload.
func (cdb *DB) AddRateLimit(path string, limit, windowSeconds int) error {
	_, err := cdb.db.Exec(
		`INSERT INTO rate_limits (path, "limit", window_seconds) VALUES (?, ?, ?)`,
		path, limit, windowSeconds,
	)
	if err != nil {
		return err
	}
	return cdb.reload()
}

// RemoveRateLimit deletes a rate limit rule by ID and triggers a reload.
func (cdb *DB) RemoveRateLimit(id int) error {
	_, err := cdb.db.Exec(`DELETE FROM rate_limits WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return cdb.reload()
}

// Close stops the polling goroutine and closes the database.
func (cdb *DB) Close() error {
	close(cdb.stopCh)
	return cdb.db.Close()
}

// Reload forces a reload of config from SQLite into memory.
func (cdb *DB) Reload() error {
	return cdb.reload()
}

func createTables(db *sqlx.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS rate_limits (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT NOT NULL,
		"limit" INTEGER NOT NULL,
		window_seconds INTEGER NOT NULL
	);`
	_, err := db.Exec(schema)
	return err
}

func (cdb *DB) reload() error {
	snap := &Snapshot{
		Config:   make(map[string]string),
		LoadedAt: time.Now(),
	}

	// Load key-value config
	rows, err := cdb.db.Query(`SELECT key, value FROM config`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		snap.Config[k] = v
	}

	// Load rate limit rules
	var rules []RateLimitRule
	err = cdb.db.Select(&rules, `SELECT id, path, "limit", window_seconds FROM rate_limits`)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	snap.RateLimits = rules

	cdb.current.Store(snap)
	return nil
}

func (cdb *DB) poll() {
	ticker := time.NewTicker(cdb.interval)
	defer ticker.Stop()

	for {
		select {
		case <-cdb.stopCh:
			return
		case <-ticker.C:
			cdb.reload()
		}
	}
}
