// Package store is the SQLite-backed persistence layer for the v1.3
// ledger (P1-2).
//
// Design constraints (from the plan):
//   - Pure Go (modernc.org/sqlite) — no CGO, no platform-specific build.
//   - Append-only enforced both in the Go API and in the database via
//     triggers, so an operator with a sqlite3 shell cannot silently
//     rewrite history.
//   - Single writer, many readers — Append serialises behind a mutex;
//     reads use the shared *sql.DB connection pool.
//   - Crash-safe — every Append runs in a transaction; WAL mode plus
//     synchronous=NORMAL gives durable commits at txn boundaries.
//
// Public types
//
//   - Store         — the handle returned by Open
//   - Kind          — entry kind tag (Rating=1, Comment=2)
//   - Entry         — a row read from `entries`
//   - HeadRow       — a row read from `heads`
//
// Sentinel errors: ErrEntryNotFound.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// Kind tags an entry as either a Rating or a Comment. Values match the
// proto3 enum implied by the SignedEntry oneof (P0-2).
type Kind int

const (
	KindRating  Kind = 1
	KindComment Kind = 2
)

// Entry is one row read out of the entries table.
type Entry struct {
	Idx        uint64
	Hash       [32]byte
	PrevHash   [32]byte
	Kind       Kind
	Payload    []byte
	Sig        []byte
	InsertedAt int64 // unix nanoseconds
}

// HeadRow is one row read out of the heads table.
type HeadRow struct {
	TreeSize uint64
	RootHash [32]byte
	SignedAt int64
	HeadBlob []byte
}

// ErrEntryNotFound is returned by GetEntry when the requested idx is
// not present. Use errors.Is for matching.
var ErrEntryNotFound = errors.New("store: entry not found")

// Store is the per-peer ledger persistence handle. One instance per
// process; goroutine-safe under the documented single-writer pattern.
type Store struct {
	db *sql.DB

	// writeMu serialises Append + WriteHead so concurrent writers
	// cannot interleave transactions that depend on shared autoincrement
	// state. Reads bypass this lock and rely on SQLite's own MVCC under
	// WAL mode.
	writeMu sync.Mutex

	// closed is set on first Close call; further calls become no-ops.
	closeOnce sync.Once
	closeErr  error
}

// Open returns a Store backed by a SQLite database at path. The
// containing directory must already exist. Migrations are applied
// automatically on first open and on every subsequent open (no-op
// after the first).
func Open(path string) (*Store, error) {
	// WAL gives durable commits + concurrent readers. synchronous=NORMAL
	// is the standard trade-off: a crash within the OS page-cache
	// window can lose the very last committed transaction but never
	// corrupt the database. busy_timeout absorbs short lock waits so
	// readers don't fail spuriously under write contention.
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(on)",
		url.PathEscape(path),
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}

	// Cap the connection pool. SQLite serialises writes; many
	// connections waste memory and contend on the file lock. A small
	// fixed pool is plenty.
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)

	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	if err := runMigrations(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrations: %w", err)
	}

	return &Store{db: db}, nil
}

// Close releases the underlying SQLite handle. Idempotent — calling
// Close more than once returns the same (possibly nil) error.
func (s *Store) Close() error {
	s.closeOnce.Do(func() {
		s.closeErr = s.db.Close()
	})
	return s.closeErr
}

// AppendEntry inserts a new entry and returns its assigned idx
// (1-based). All append-only validation lives in the schema; this
// function just packs the values into the prepared statement.
func (s *Store) AppendEntry(
	ctx context.Context,
	hash, prevHash [32]byte,
	kind Kind,
	payload, sig []byte,
) (uint64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO entries(hash, prev_hash, kind, payload, sig, inserted_at)
		 VALUES(?, ?, ?, ?, ?, ?)`,
		hash[:], prevHash[:], int(kind), payload, sig, time.Now().UnixNano(),
	)
	if err != nil {
		return 0, fmt.Errorf("insert entry: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return uint64(id), nil
}

// GetEntryByHash returns the entry whose RFC 6962 leaf hash matches.
// Backed by the `entries_hash_idx` SQLite index — O(log n) lookup.
// Returns ErrEntryNotFound when the hash is not present.
func (s *Store) GetEntryByHash(ctx context.Context, hash [32]byte) (*Entry, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT idx, hash, prev_hash, kind, payload, sig, inserted_at
		 FROM entries WHERE hash = ?`, hash[:],
	)
	out := &Entry{}
	var (
		hashBytes, prev []byte
		kindInt         int
	)
	err := row.Scan(&out.Idx, &hashBytes, &prev, &kindInt, &out.Payload, &out.Sig, &out.InsertedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: hash=%x", ErrEntryNotFound, hash)
	}
	if err != nil {
		return nil, fmt.Errorf("scan entry by hash: %w", err)
	}
	if len(hashBytes) != 32 || len(prev) != 32 {
		return nil, fmt.Errorf("malformed row: hash=%d prev=%d", len(hashBytes), len(prev))
	}
	copy(out.Hash[:], hashBytes)
	copy(out.PrevHash[:], prev)
	out.Kind = Kind(kindInt)
	return out, nil
}

// GetEntry returns the entry at idx. Returns ErrEntryNotFound (wrapped)
// when the row does not exist.
func (s *Store) GetEntry(ctx context.Context, idx uint64) (*Entry, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT idx, hash, prev_hash, kind, payload, sig, inserted_at
		 FROM entries WHERE idx = ?`, idx,
	)
	out := &Entry{}
	var (
		hash, prev []byte
		kindInt    int
	)
	err := row.Scan(&out.Idx, &hash, &prev, &kindInt, &out.Payload, &out.Sig, &out.InsertedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: idx=%d", ErrEntryNotFound, idx)
	}
	if err != nil {
		return nil, fmt.Errorf("scan entry: %w", err)
	}
	if len(hash) != 32 || len(prev) != 32 {
		return nil, fmt.Errorf("malformed row at idx=%d: hash=%d prev=%d", idx, len(hash), len(prev))
	}
	copy(out.Hash[:], hash)
	copy(out.PrevHash[:], prev)
	out.Kind = Kind(kindInt)
	return out, nil
}

// EntryCount returns the total number of entries.
func (s *Store) EntryCount(ctx context.Context) (uint64, error) {
	var n uint64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM entries`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count entries: %w", err)
	}
	return n, nil
}

// WriteHead inserts or overwrites the head at treeSize. Overwrites are
// expected — additional witness signatures accumulate on the same
// (tree_size, root_hash) pair over time.
func (s *Store) WriteHead(
	ctx context.Context,
	treeSize uint64,
	rootHash [32]byte,
	signedAt int64,
	headBlob []byte,
) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO heads(tree_size, root_hash, signed_at, head_blob)
		 VALUES(?, ?, ?, ?)
		 ON CONFLICT(tree_size) DO UPDATE SET
		     root_hash = excluded.root_hash,
		     signed_at = excluded.signed_at,
		     head_blob = excluded.head_blob`,
		treeSize, rootHash[:], signedAt, headBlob,
	)
	if err != nil {
		return fmt.Errorf("upsert head: %w", err)
	}
	return nil
}

// LatestHead returns the head with the largest tree_size, or nil, nil
// if no heads have been written yet.
func (s *Store) LatestHead(ctx context.Context) (*HeadRow, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT tree_size, root_hash, signed_at, head_blob
		 FROM heads
		 ORDER BY tree_size DESC
		 LIMIT 1`,
	)
	out := &HeadRow{}
	var rootBytes []byte
	err := row.Scan(&out.TreeSize, &rootBytes, &out.SignedAt, &out.HeadBlob)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan head: %w", err)
	}
	if len(rootBytes) != 32 {
		return nil, fmt.Errorf("malformed head row: root len=%d", len(rootBytes))
	}
	copy(out.RootHash[:], rootBytes)
	return out, nil
}

// IterateEntries streams entries with idx >= startIdx in ascending
// order, invoking fn for each. If fn returns a non-nil error,
// iteration stops and that error is propagated. The visitor MUST NOT
// retain *Entry past the callback — the underlying buffers are reused
// (currently they aren't, but the contract reserves the right).
func (s *Store) IterateEntries(ctx context.Context, startIdx uint64, fn func(*Entry) error) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT idx, hash, prev_hash, kind, payload, sig, inserted_at
		 FROM entries
		 WHERE idx >= ?
		 ORDER BY idx ASC`, startIdx,
	)
	if err != nil {
		return fmt.Errorf("query entries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		out := &Entry{}
		var (
			hash, prev []byte
			kindInt    int
		)
		if err := rows.Scan(&out.Idx, &hash, &prev, &kindInt, &out.Payload, &out.Sig, &out.InsertedAt); err != nil {
			return fmt.Errorf("scan entry: %w", err)
		}
		if len(hash) != 32 || len(prev) != 32 {
			return fmt.Errorf("malformed row at idx=%d", out.Idx)
		}
		copy(out.Hash[:], hash)
		copy(out.PrevHash[:], prev)
		out.Kind = Kind(kindInt)
		if err := fn(out); err != nil {
			return err
		}
	}
	return rows.Err()
}

// RawExecForTest runs arbitrary SQL against the database. EXPORTED FOR
// TESTS ONLY — production code MUST go through the typed APIs above so
// the append-only invariant is preserved. We expose this so tests can
// directly exercise UPDATE/DELETE-refusal triggers without bypassing
// the package boundary.
func (s *Store) RawExecForTest(ctx context.Context, query string, args ...any) error {
	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}
