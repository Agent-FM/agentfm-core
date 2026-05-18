package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	pb "agentfm/internal/ledger/pb"

	"google.golang.org/protobuf/proto"
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

// IterateAllOwnEntries streams every entry from THIS peer's own log
// (the `entries` table — populated by Ledger.Append). Distinct from
// IterateAllInboxEntries which yields entries received from OTHER
// peers via gossip.
//
// Used by the reputation engine on the boss-side so the engine can
// score subjects using BOTH the boss's machine-issued attestation
// ratings (own log) AND what other peers say (inbox).
func (s *Store) IterateAllOwnEntries(ctx context.Context, fn func(*Entry) error) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT idx, hash, prev_hash, kind, payload, sig, inserted_at
		 FROM entries
		 ORDER BY idx ASC`,
	)
	if err != nil {
		return fmt.Errorf("query own entries: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		e := &Entry{}
		var h, p []byte
		var kindInt int
		if err := rows.Scan(&e.Idx, &h, &p, &kindInt, &e.Payload, &e.Sig, &e.InsertedAt); err != nil {
			return fmt.Errorf("scan own entry: %w", err)
		}
		if len(h) != 32 || len(p) != 32 {
			return fmt.Errorf("malformed own row: hash=%d prev=%d", len(h), len(p))
		}
		copy(e.Hash[:], h)
		copy(e.PrevHash[:], p)
		e.Kind = Kind(kindInt)
		if err := fn(e); err != nil {
			return err
		}
	}
	return rows.Err()
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

// -----------------------------------------------------------------------------
// equivocators — permanent marker per peer caught showing two
// non-extending heads (P2-3). The marker is set ON CONFLICT DO NOTHING
// so additional alerts about the same peer don't overwrite the
// earlier evidence blob.
// -----------------------------------------------------------------------------

// MarkEquivocator records peerID as a permanent equivocator. The
// alert that justified the mark is stored alongside for audit /
// display in the reputation UI. If peerID is already an equivocator,
// returns nil without touching the existing row (preserves the
// original evidence).
func (s *Store) MarkEquivocator(ctx context.Context, peerID, alertBlob []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO equivocators(peer_id, alert_blob, marked_at)
		 VALUES(?, ?, ?)
		 ON CONFLICT(peer_id) DO NOTHING`,
		peerID, alertBlob, time.Now().UnixNano(),
	)
	if err != nil {
		return fmt.Errorf("mark equivocator: %w", err)
	}
	return nil
}

// IsEquivocator reports whether peerID has been marked as such by
// some witness alert this node has seen.
func (s *Store) IsEquivocator(ctx context.Context, peerID []byte) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM equivocators WHERE peer_id = ?`, peerID,
	).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query equivocators: %w", err)
	}
	return true, nil
}

// EquivocatorAlert returns the alert_blob that originally marked
// peerID as an equivocator, or (nil, nil) if no such row exists.
func (s *Store) EquivocatorAlert(ctx context.Context, peerID []byte) ([]byte, error) {
	var blob []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT alert_blob FROM equivocators WHERE peer_id = ?`, peerID,
	).Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query equivocator alert: %w", err)
	}
	return blob, nil
}

// -----------------------------------------------------------------------------
// witness_state — per-peer "last head this WITNESS has co-signed" memory.
// Used by P2-2 to detect equivocation: a witness refuses to co-sign a
// head whose tree_size is <= the head it last signed for that peer.
// -----------------------------------------------------------------------------

// WitnessState is one row from witness_state. Contains the most recent
// LogHead bytes this witness has co-signed for the given peer.
type WitnessState struct {
	PeerID    []byte
	TreeSize  uint64
	LastHead  []byte // serialised pb.LogHead bytes (full envelope)
}

// GetWitnessState returns the row for peerID, or (nil, nil) if the
// witness has never signed a head for this peer.
func (s *Store) GetWitnessState(ctx context.Context, peerID []byte) (*WitnessState, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT peer_id, tree_size, last_head FROM witness_state WHERE peer_id = ?`,
		peerID,
	)
	out := &WitnessState{}
	err := row.Scan(&out.PeerID, &out.TreeSize, &out.LastHead)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query witness_state: %w", err)
	}
	return out, nil
}

// UpsertWitnessState records peerID's latest head — overwriting any
// previous entry. Caller is responsible for ensuring this is only
// invoked AFTER it has verified the head extends the previous state.
func (s *Store) UpsertWitnessState(ctx context.Context, peerID []byte, treeSize uint64, headBlob []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO witness_state(peer_id, tree_size, last_head)
		 VALUES(?, ?, ?)
		 ON CONFLICT(peer_id) DO UPDATE SET
		     tree_size = excluded.tree_size,
		     last_head = excluded.last_head`,
		peerID, treeSize, headBlob,
	)
	if err != nil {
		return fmt.Errorf("upsert witness_state: %w", err)
	}
	return nil
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

// DistinctSubjects returns every distinct peer ID that appears as a
// SubjectPeerId in either entries (own log) or inbox_entries (gossipped),
// sorted. Used to surface offline peers in the operator-facing /api/workers
// and TUI radar.
//
// Implementation note: neither `entries` nor `inbox_entries` has a dedicated
// subject_peer_id column (the subject is embedded in the protobuf payload).
// At hobbyist scale (~100s of entries) a full scan + decode is cheaper than
// adding a migration + backfill. If the ledger grows to millions of entries
// a future migration can add an indexed column and this function can be
// rewritten as a simple SQL query.
func (s *Store) DistinctSubjects(ctx context.Context) ([][]byte, error) {
	seen := map[string][]byte{} // key = string(subjectBytes) for dedup

	extractSubject := func(payload []byte) []byte {
		var signed pb.SignedEntry
		if err := proto.Unmarshal(payload, &signed); err != nil {
			return nil
		}
		switch body := signed.GetBody().(type) {
		case *pb.SignedEntry_Rating:
			if body.Rating != nil && len(body.Rating.SubjectPeerId) > 0 {
				return body.Rating.SubjectPeerId
			}
		case *pb.SignedEntry_Comment:
			if body.Comment != nil && len(body.Comment.SubjectPeerId) > 0 {
				return body.Comment.SubjectPeerId
			}
		}
		return nil
	}

	if err := s.IterateAllOwnEntries(ctx, func(e *Entry) error {
		subj := extractSubject(e.Payload)
		if subj != nil {
			seen[string(subj)] = subj
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("DistinctSubjects own entries: %w", err)
	}

	if err := s.IterateAllInboxEntries(ctx, func(e *InboxEntry) error {
		subj := extractSubject(e.Payload)
		if subj != nil {
			seen[string(subj)] = subj
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("DistinctSubjects inbox entries: %w", err)
	}

	out := make([][]byte, 0, len(seen))
	for _, v := range seen {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		return bytes.Compare(out[i], out[j]) < 0
	})
	return out, nil
}
