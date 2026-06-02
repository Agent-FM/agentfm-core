package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"agentfm/internal/ledger/store"
)

// TestIterateInboxFrom_PaginatesByRowid covers the contract:
// * inserting N entries yields N visits in insertion order.
// * passing sinceRowid filters out earlier rows.
// * the limit argument bounds the page size.
// * the callback receives the rowid so the caller can resume.
func TestIterateInboxFrom_PaginatesByRowid(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "iter.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	for i := byte(1); i <= 3; i++ {
		peer := []byte{0x0A, i}
		var hash, prev [32]byte
		hash[0] = i
		payload := []byte{0xCA, 0xFE, i}
		if err := s.InsertInboxEntry(ctx, peer, hash, prev, payload); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	var rowids []uint64
	var payloads [][]byte
	if err := s.IterateInboxFrom(ctx, 0, 100, func(rowid uint64, e *store.InboxEntry) error {
		rowids = append(rowids, rowid)
		payloads = append(payloads, e.Payload)
		return nil
	}); err != nil {
		t.Fatalf("iterate from 0: %v", err)
	}
	if len(rowids) != 3 {
		t.Fatalf("page 1 expected 3 rows, got %d", len(rowids))
	}
	if rowids[0] >= rowids[1] || rowids[1] >= rowids[2] {
		t.Fatalf("rowids not strictly increasing: %v", rowids)
	}
	if payloads[2][2] != 3 {
		t.Fatalf("expected last payload byte to be 3 (insert order), got %d", payloads[2][2])
	}

	cursor := rowids[1]
	var p2 []uint64
	if err := s.IterateInboxFrom(ctx, cursor, 100, func(rowid uint64, _ *store.InboxEntry) error {
		p2 = append(p2, rowid)
		return nil
	}); err != nil {
		t.Fatalf("iterate from %d: %v", cursor, err)
	}
	if len(p2) != 1 {
		t.Fatalf("page 2 expected 1 row, got %d (%v)", len(p2), p2)
	}
	if p2[0] != rowids[2] {
		t.Fatalf("page 2 row mismatch: got %d, want %d", p2[0], rowids[2])
	}

	var clamped []uint64
	if err := s.IterateInboxFrom(ctx, 0, 2, func(rowid uint64, _ *store.InboxEntry) error {
		clamped = append(clamped, rowid)
		return nil
	}); err != nil {
		t.Fatalf("iterate with limit=2: %v", err)
	}
	if len(clamped) != 2 {
		t.Fatalf("limit=2 expected 2 rows, got %d", len(clamped))
	}
}
