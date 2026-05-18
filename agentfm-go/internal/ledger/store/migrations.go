package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// migration is one numbered SQL file to be applied in order.
type migration struct {
	version  int    // parsed from "NNN_*.sql" filename prefix
	filename string // for diagnostics + tracking
	sql      string // full file body
}

// loadMigrations reads every *.sql under migrations/ in numeric order.
// Returns the in-memory list — runMigrations decides which to apply.
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read embed migrations dir: %w", err)
	}
	var out []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		// Filename convention: "NNN_short_name.sql" (e.g. 001_init.sql).
		// Reject any file we cannot parse rather than silently apply in
		// a surprising order.
		parts := strings.SplitN(e.Name(), "_", 2)
		if len(parts) < 1 {
			return nil, fmt.Errorf("migration filename has no version prefix: %s", e.Name())
		}
		var version int
		if _, err := fmt.Sscanf(parts[0], "%d", &version); err != nil {
			return nil, fmt.Errorf("migration %q: cannot parse version: %w", e.Name(), err)
		}
		body, err := fs.ReadFile(migrationFiles, "migrations/"+e.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", e.Name(), err)
		}
		out = append(out, migration{version: version, filename: e.Name(), sql: string(body)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

// runMigrations applies any not-yet-applied migrations, in order. The
// schema_migrations table tracks which versions have run so re-opens
// of an existing DB are no-ops.
func runMigrations(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			filename   TEXT NOT NULL,
			applied_at INTEGER NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("bootstrap schema_migrations: %w", err)
	}

	applied, err := loadAppliedVersions(ctx, db)
	if err != nil {
		return err
	}

	pending, err := loadMigrations()
	if err != nil {
		return err
	}

	for _, m := range pending {
		if applied[m.version] {
			continue
		}
		if err := applyOne(ctx, db, m); err != nil {
			return fmt.Errorf("apply %s: %w", m.filename, err)
		}
	}
	return nil
}

// loadAppliedVersions returns the set of migration versions already
// recorded as applied.
func loadAppliedVersions(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()
	out := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan migration version: %w", err)
		}
		out[v] = true
	}
	return out, rows.Err()
}

// applyOne runs a single migration's SQL inside a transaction and
// records it in schema_migrations on success.
func applyOne(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return fmt.Errorf("exec sql: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations(version, filename, applied_at) VALUES(?, ?, ?)`,
		m.version, m.filename, time.Now().UnixNano(),
	); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}
	return tx.Commit()
}
