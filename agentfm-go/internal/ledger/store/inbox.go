package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// InboxEntry is one row from inbox_entries or inbox_orphans. The same
// shape serves both since the schemas are identical — only the
// semantics differ (accepted vs. waiting for parent).
type InboxEntry struct {
	PeerID     []byte // libp2p PeerID bytes of the rater
	Hash       [32]byte
	PrevHash   [32]byte
	Payload    []byte // full pb.SignedEntry proto with signature
	ReceivedAt int64  // unix nanoseconds
}

// ErrInboxEntryNotFound is returned by GetInboxEntry / GetInboxOrphan
// when the requested (peer_id, hash) row does not exist.
var ErrInboxEntryNotFound = errors.New("store: inbox entry not found")

// HasInboxEntry reports whether (peerID, hash) is already accepted into
// the inbox. Used by Inbox.AcceptOrQueue for dedup before doing any
// further work.
func (s *Store) HasInboxEntry(ctx context.Context, peerID []byte, hash [32]byte) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM inbox_entries WHERE peer_id = ? AND hash = ?`,
		peerID, hash[:],
	).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query inbox_entries: %w", err)
	}
	return true, nil
}

// HasInboxOrphan reports whether (peerID, hash) is currently sitting in
// the orphan queue.
func (s *Store) HasInboxOrphan(ctx context.Context, peerID []byte, hash [32]byte) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM inbox_orphans WHERE peer_id = ? AND hash = ?`,
		peerID, hash[:],
	).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query inbox_orphans: %w", err)
	}
	return true, nil
}

// InsertInboxEntry adds an accepted entry to inbox_entries and updates
// the per-peer chain head to point at this entry's hash. Both writes
// happen in one transaction so a concurrent reader never sees the
// "entry inserted but head not yet updated" intermediate state.
//
// If (peerID, hash) already exists, returns nil (idempotent insert).
func (s *Store) InsertInboxEntry(
	ctx context.Context,
	peerID []byte,
	hash, prevHash [32]byte,
	payload []byte,
) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UnixNano()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO inbox_entries(peer_id, hash, prev_hash, payload, received_at)
		 VALUES(?, ?, ?, ?, ?)
		 ON CONFLICT(peer_id, hash) DO NOTHING`,
		peerID, hash[:], prevHash[:], payload, now,
	); err != nil {
		return fmt.Errorf("insert inbox entry: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO inbox_known_chain_head(peer_id, last_hash, updated_at)
		 VALUES(?, ?, ?)
		 ON CONFLICT(peer_id) DO UPDATE SET
		     last_hash  = excluded.last_hash,
		     updated_at = excluded.updated_at`,
		peerID, hash[:], now,
	); err != nil {
		return fmt.Errorf("upsert chain head: %w", err)
	}

	return tx.Commit()
}

// InsertInboxOrphan adds an entry to the orphan queue. If the entry is
// already an orphan, this is a no-op.
func (s *Store) InsertInboxOrphan(
	ctx context.Context,
	peerID []byte,
	hash, prevHash [32]byte,
	payload []byte,
) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO inbox_orphans(peer_id, hash, prev_hash, payload, received_at)
		 VALUES(?, ?, ?, ?, ?)
		 ON CONFLICT(peer_id, hash) DO NOTHING`,
		peerID, hash[:], prevHash[:], payload, time.Now().UnixNano(),
	)
	if err != nil {
		return fmt.Errorf("insert inbox orphan: %w", err)
	}
	return nil
}

// DeleteInboxOrphan removes an entry from the orphan queue — called
// after promoting it to inbox_entries.
func (s *Store) DeleteInboxOrphan(ctx context.Context, peerID []byte, hash [32]byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	_, err := s.db.ExecContext(ctx,
		`DELETE FROM inbox_orphans WHERE peer_id = ? AND hash = ?`,
		peerID, hash[:],
	)
	if err != nil {
		return fmt.Errorf("delete inbox orphan: %w", err)
	}
	return nil
}

// FindInboxOrphansAwaiting returns every orphan whose prev_hash equals
// parentHash for the given peer. Used to promote children when their
// parent arrives.
func (s *Store) FindInboxOrphansAwaiting(
	ctx context.Context,
	peerID []byte,
	parentHash [32]byte,
) ([]*InboxEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT peer_id, hash, prev_hash, payload, received_at
		 FROM inbox_orphans
		 WHERE peer_id = ? AND prev_hash = ?
		 ORDER BY received_at ASC`,
		peerID, parentHash[:],
	)
	if err != nil {
		return nil, fmt.Errorf("query orphans: %w", err)
	}
	defer rows.Close()

	var out []*InboxEntry
	for rows.Next() {
		e := &InboxEntry{}
		var h, p []byte
		if err := rows.Scan(&e.PeerID, &h, &p, &e.Payload, &e.ReceivedAt); err != nil {
			return nil, fmt.Errorf("scan orphan: %w", err)
		}
		if len(h) != 32 || len(p) != 32 {
			return nil, fmt.Errorf("malformed orphan row: hash=%d prev=%d", len(h), len(p))
		}
		copy(e.Hash[:], h)
		copy(e.PrevHash[:], p)
		out = append(out, e)
	}
	return out, rows.Err()
}

// CountInboxOrphans returns the total number of orphans across all
// peers. Used by Inbox to enforce the global cap.
func (s *Store) CountInboxOrphans(ctx context.Context) (uint64, error) {
	var n uint64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM inbox_orphans`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count orphans: %w", err)
	}
	return n, nil
}

// GetInboxChainHead returns the latest known hash for peerID, or
// (zero, false) if this peer has not been seen yet.
func (s *Store) GetInboxChainHead(ctx context.Context, peerID []byte) ([32]byte, bool, error) {
	var out [32]byte
	var raw []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT last_hash FROM inbox_known_chain_head WHERE peer_id = ?`, peerID,
	).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return out, false, nil
	}
	if err != nil {
		return out, false, fmt.Errorf("query chain head: %w", err)
	}
	if len(raw) != 32 {
		return out, false, fmt.Errorf("malformed chain head: len=%d", len(raw))
	}
	copy(out[:], raw)
	return out, true, nil
}

// IterateAllInboxEntries streams every accepted inbox entry across
// all rater peers in (peer_id, received_at) order, invoking fn for
// each. If fn returns a non-nil error, iteration stops and that error
// is propagated.
//
// Used today by the P1-6 CLI to gather entries about a specific
// subject by filtering in Go (we do not yet have a subject index on
// inbox_entries — keyed by (peer_id, hash) — because filling that
// index would mean an ALTER TABLE migration on a column we don't yet
// need for any hot path. The index will land alongside P3-7 when
// reputation scoring needs efficient subject lookups).
func (s *Store) IterateAllInboxEntries(ctx context.Context, fn func(*InboxEntry) error) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT peer_id, hash, prev_hash, payload, received_at
		 FROM inbox_entries
		 ORDER BY peer_id ASC, received_at ASC`,
	)
	if err != nil {
		return fmt.Errorf("query inbox_entries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		e := &InboxEntry{}
		var h, p []byte
		if err := rows.Scan(&e.PeerID, &h, &p, &e.Payload, &e.ReceivedAt); err != nil {
			return fmt.Errorf("scan inbox row: %w", err)
		}
		if len(h) != 32 || len(p) != 32 {
			return fmt.Errorf("malformed inbox row: hash=%d prev=%d", len(h), len(p))
		}
		copy(e.Hash[:], h)
		copy(e.PrevHash[:], p)
		if err := fn(e); err != nil {
			return err
		}
	}
	return rows.Err()
}

// GetInboxEntry returns the inbox row for (peerID, hash) or
// ErrInboxEntryNotFound. Used by Inbox.HasEntry-style tests and by
// downstream pull-on-demand fetch (P4-2 surface).
func (s *Store) GetInboxEntry(ctx context.Context, peerID []byte, hash [32]byte) (*InboxEntry, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT peer_id, hash, prev_hash, payload, received_at
		 FROM inbox_entries WHERE peer_id = ? AND hash = ?`,
		peerID, hash[:],
	)
	e := &InboxEntry{}
	var h, p []byte
	if err := row.Scan(&e.PeerID, &h, &p, &e.Payload, &e.ReceivedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: peer=%x hash=%x", ErrInboxEntryNotFound, peerID, hash)
		}
		return nil, fmt.Errorf("scan inbox entry: %w", err)
	}
	if len(h) != 32 || len(p) != 32 {
		return nil, fmt.Errorf("malformed inbox row: hash=%d prev=%d", len(h), len(p))
	}
	copy(e.Hash[:], h)
	copy(e.PrevHash[:], p)
	return e, nil
}
