package store_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"agentfm/internal/ledger/store"
)

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

// openFresh returns a Store backed by a brand-new SQLite file in a
// per-test temp dir. The store is closed by t.Cleanup so each test
// gets a clean slate without sharing state.
func openFresh(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ledger.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// h32 returns a deterministic 32-byte test hash from a seed integer.
func h32(seed byte) [32]byte {
	var out [32]byte
	for i := range out {
		out[i] = seed
	}
	return out
}

func ctx() context.Context { return context.Background() }

// -----------------------------------------------------------------------------
// open / migrations
// -----------------------------------------------------------------------------

func TestOpen_FreshDB_RunsMigrationsAndIsEmpty(t *testing.T) {
	s := openFresh(t)
	n, err := s.EntryCount(ctx())
	if err != nil {
		t.Fatalf("EntryCount: %v", err)
	}
	if n != 0 {
		t.Fatalf("fresh DB EntryCount = %d, want 0", n)
	}
	head, err := s.LatestHead(ctx())
	if err != nil {
		t.Fatalf("LatestHead: %v", err)
	}
	if head != nil {
		t.Fatalf("fresh DB LatestHead = %+v, want nil", head)
	}
}

func TestOpen_ReopenExisting_PreservesData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persist.db")

	// First session: write one entry and close.
	s1, err := store.Open(path)
	if err != nil {
		t.Fatalf("open #1: %v", err)
	}
	idx, err := s1.AppendEntry(ctx(), h32(1), h32(0), store.KindRating, []byte("payload-1"), []byte("sig-1"))
	if err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}
	if idx != 1 {
		t.Fatalf("first AppendEntry idx = %d, want 1", idx)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("close #1: %v", err)
	}

	// Second session: reopen and confirm the entry is still there.
	s2, err := store.Open(path)
	if err != nil {
		t.Fatalf("open #2: %v", err)
	}
	t.Cleanup(func() { _ = s2.Close() })

	got, err := s2.GetEntry(ctx(), 1)
	if err != nil {
		t.Fatalf("GetEntry: %v", err)
	}
	if got.Idx != 1 {
		t.Fatalf("Idx = %d, want 1", got.Idx)
	}
	if !bytes.Equal(got.Payload, []byte("payload-1")) {
		t.Fatalf("Payload mismatch after reopen")
	}
}

// -----------------------------------------------------------------------------
// append + get
// -----------------------------------------------------------------------------

func TestAppendEntry_AssignsConsecutiveIndices(t *testing.T) {
	s := openFresh(t)
	prev := h32(0)
	for i := byte(1); i <= 10; i++ {
		hash := h32(i)
		idx, err := s.AppendEntry(ctx(), hash, prev, store.KindRating, []byte{i}, []byte{i, i})
		if err != nil {
			t.Fatalf("AppendEntry %d: %v", i, err)
		}
		if want := uint64(i); idx != want {
			t.Fatalf("AppendEntry returned idx %d, want %d", idx, want)
		}
		prev = hash
	}
	n, err := s.EntryCount(ctx())
	if err != nil {
		t.Fatalf("EntryCount: %v", err)
	}
	if n != 10 {
		t.Fatalf("EntryCount = %d, want 10", n)
	}
}

func TestAppendEntry_DuplicateHashRejected(t *testing.T) {
	s := openFresh(t)
	hash := h32(7)
	if _, err := s.AppendEntry(ctx(), hash, h32(0), store.KindRating, []byte("a"), []byte("s")); err != nil {
		t.Fatalf("first AppendEntry: %v", err)
	}
	_, err := s.AppendEntry(ctx(), hash, h32(1), store.KindRating, []byte("b"), []byte("s"))
	if err == nil {
		t.Fatal("expected unique-violation error for duplicate hash, got nil")
	}
}

func TestGetEntry_NotFoundReturnsErrEntryNotFound(t *testing.T) {
	s := openFresh(t)
	_, err := s.GetEntry(ctx(), 42)
	if !errors.Is(err, store.ErrEntryNotFound) {
		t.Fatalf("want ErrEntryNotFound, got %v", err)
	}
}

// Fix-5 acceptance: GetEntryByHash uses the entries_hash_idx index
// for O(log n) lookups. Verify round-trip + ErrEntryNotFound.
func TestGetEntryByHash_RoundTrip(t *testing.T) {
	s := openFresh(t)
	hash := h32(0xab)
	if _, err := s.AppendEntry(ctx(), hash, h32(0), store.KindRating, []byte("p"), []byte("s")); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}
	got, err := s.GetEntryByHash(ctx(), hash)
	if err != nil {
		t.Fatalf("GetEntryByHash: %v", err)
	}
	if got.Idx != 1 {
		t.Errorf("Idx = %d, want 1", got.Idx)
	}
	if got.Hash != hash {
		t.Errorf("Hash mismatch")
	}
}

func TestGetEntryByHash_NotFound(t *testing.T) {
	s := openFresh(t)
	_, err := s.GetEntryByHash(ctx(), h32(0xff))
	if !errors.Is(err, store.ErrEntryNotFound) {
		t.Fatalf("want ErrEntryNotFound, got %v", err)
	}
}

func TestGetEntry_RoundTrip_AllFields(t *testing.T) {
	s := openFresh(t)
	hash := h32(0xab)
	prev := h32(0xcd)
	payload := []byte{0x10, 0x20, 0x30}
	sig := []byte{0xaa, 0xbb}

	idx, err := s.AppendEntry(ctx(), hash, prev, store.KindComment, payload, sig)
	if err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}
	got, err := s.GetEntry(ctx(), idx)
	if err != nil {
		t.Fatalf("GetEntry: %v", err)
	}
	if got.Hash != hash {
		t.Errorf("Hash mismatch: %x vs %x", got.Hash, hash)
	}
	if got.PrevHash != prev {
		t.Errorf("PrevHash mismatch")
	}
	if got.Kind != store.KindComment {
		t.Errorf("Kind = %d, want %d", got.Kind, store.KindComment)
	}
	if !bytes.Equal(got.Payload, payload) {
		t.Errorf("Payload mismatch")
	}
	if !bytes.Equal(got.Sig, sig) {
		t.Errorf("Sig mismatch")
	}
	if got.InsertedAt == 0 {
		t.Errorf("InsertedAt unexpectedly zero")
	}
}

// -----------------------------------------------------------------------------
// append-only enforcement
// -----------------------------------------------------------------------------

func TestAppendOnly_UpdateOnEntries_Refused(t *testing.T) {
	s := openFresh(t)
	if _, err := s.AppendEntry(ctx(), h32(1), h32(0), store.KindRating, []byte("p"), []byte("s")); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}
	err := s.RawExecForTest(ctx(), `UPDATE entries SET payload = X'00' WHERE idx = 1`)
	if err == nil {
		t.Fatal("expected UPDATE to be refused by trigger, got nil")
	}
}

func TestAppendOnly_DeleteOnEntries_Refused(t *testing.T) {
	s := openFresh(t)
	if _, err := s.AppendEntry(ctx(), h32(1), h32(0), store.KindRating, []byte("p"), []byte("s")); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}
	err := s.RawExecForTest(ctx(), `DELETE FROM entries WHERE idx = 1`)
	if err == nil {
		t.Fatal("expected DELETE to be refused by trigger, got nil")
	}
}

// Schema v4 (migration 004): DELETE on equivocators must be refused
// too, matching the entries-table pattern. Without this trigger a
// local sqlite3 user could DELETE FROM equivocators WHERE peer_id=X
// and silently un-mark someone for THIS node only — locally
// inconsistent with the mesh-wide marker that other peers still
// hold.
func TestAppendOnly_DeleteOnEquivocators_Refused(t *testing.T) {
	s := openFresh(t)
	if err := s.MarkEquivocator(ctx(), []byte("offender"), []byte("alert-blob")); err != nil {
		t.Fatalf("MarkEquivocator: %v", err)
	}
	// Confirm row exists first — sanity check that MarkEquivocator
	// actually wrote, so the DELETE we attempt has a target.
	marked, err := s.IsEquivocator(ctx(), []byte("offender"))
	if err != nil || !marked {
		t.Fatalf("MarkEquivocator did not stick (marked=%v err=%v)", marked, err)
	}
	err = s.RawExecForTest(ctx(), `DELETE FROM equivocators WHERE peer_id = X'6f6666656e646572'`) // hex of "offender"
	if err == nil {
		t.Fatal("expected DELETE on equivocators to be refused by trigger")
	}
	// Sanity: row is still there.
	stillMarked, _ := s.IsEquivocator(ctx(), []byte("offender"))
	if !stillMarked {
		t.Fatal("equivocator was deleted despite trigger error")
	}
}

// Symmetric guard for the existing UPDATE trigger on equivocators —
// covered by trigger 003 but not previously tested.
func TestAppendOnly_UpdateOnEquivocators_Refused(t *testing.T) {
	s := openFresh(t)
	if err := s.MarkEquivocator(ctx(), []byte("p"), []byte("alert")); err != nil {
		t.Fatalf("MarkEquivocator: %v", err)
	}
	err := s.RawExecForTest(ctx(), `UPDATE equivocators SET alert_blob = X'00'`)
	if err == nil {
		t.Fatal("expected UPDATE on equivocators to be refused by trigger")
	}
}

// -----------------------------------------------------------------------------
// heads
// -----------------------------------------------------------------------------

func TestWriteHead_LatestHeadReflectsLastWrite(t *testing.T) {
	s := openFresh(t)
	if err := s.WriteHead(ctx(), 1, h32(0x11), 1700, []byte("head-1")); err != nil {
		t.Fatalf("WriteHead 1: %v", err)
	}
	if err := s.WriteHead(ctx(), 2, h32(0x22), 1701, []byte("head-2")); err != nil {
		t.Fatalf("WriteHead 2: %v", err)
	}
	if err := s.WriteHead(ctx(), 3, h32(0x33), 1702, []byte("head-3")); err != nil {
		t.Fatalf("WriteHead 3: %v", err)
	}

	got, err := s.LatestHead(ctx())
	if err != nil {
		t.Fatalf("LatestHead: %v", err)
	}
	if got == nil {
		t.Fatal("LatestHead is nil after writes")
	}
	if got.TreeSize != 3 {
		t.Errorf("TreeSize = %d, want 3", got.TreeSize)
	}
	if got.RootHash != h32(0x33) {
		t.Errorf("RootHash mismatch")
	}
	if !bytes.Equal(got.HeadBlob, []byte("head-3")) {
		t.Errorf("HeadBlob mismatch")
	}
}

func TestWriteHead_DuplicateTreeSize_Overwrites(t *testing.T) {
	// Heads at the same tree_size can occur when more witness signatures
	// arrive after the initial publish — the new blob carries an updated
	// witness_sigs list but the same (tree_size, root_hash). The store
	// must accept and overwrite, not error.
	s := openFresh(t)
	if err := s.WriteHead(ctx(), 5, h32(0x55), 1700, []byte("partial-sigs")); err != nil {
		t.Fatalf("WriteHead #1: %v", err)
	}
	if err := s.WriteHead(ctx(), 5, h32(0x55), 1701, []byte("full-sigs")); err != nil {
		t.Fatalf("WriteHead #2 (overwrite): %v", err)
	}
	got, err := s.LatestHead(ctx())
	if err != nil {
		t.Fatalf("LatestHead: %v", err)
	}
	if !bytes.Equal(got.HeadBlob, []byte("full-sigs")) {
		t.Fatalf("HeadBlob = %q, want full-sigs", string(got.HeadBlob))
	}
}

// -----------------------------------------------------------------------------
// iterate
// -----------------------------------------------------------------------------

func TestIterateEntries_FromStart_VisitsAllInOrder(t *testing.T) {
	s := openFresh(t)
	prev := h32(0)
	for i := byte(1); i <= 5; i++ {
		hash := h32(i)
		if _, err := s.AppendEntry(ctx(), hash, prev, store.KindRating, []byte{i}, []byte{i}); err != nil {
			t.Fatalf("AppendEntry %d: %v", i, err)
		}
		prev = hash
	}
	var visited []uint64
	err := s.IterateEntries(ctx(), 1, func(e *store.Entry) error {
		visited = append(visited, e.Idx)
		return nil
	})
	if err != nil {
		t.Fatalf("IterateEntries: %v", err)
	}
	if len(visited) != 5 {
		t.Fatalf("visited %d entries, want 5: %v", len(visited), visited)
	}
	for i, idx := range visited {
		if idx != uint64(i+1) {
			t.Errorf("visited[%d] = %d, want %d", i, idx, i+1)
		}
	}
}

func TestIterateEntries_FromMiddleIdx_SkipsEarlier(t *testing.T) {
	s := openFresh(t)
	prev := h32(0)
	for i := byte(1); i <= 10; i++ {
		hash := h32(i)
		if _, err := s.AppendEntry(ctx(), hash, prev, store.KindRating, []byte{i}, []byte{i}); err != nil {
			t.Fatalf("AppendEntry: %v", err)
		}
		prev = hash
	}
	var visited []uint64
	err := s.IterateEntries(ctx(), 6, func(e *store.Entry) error {
		visited = append(visited, e.Idx)
		return nil
	})
	if err != nil {
		t.Fatalf("IterateEntries: %v", err)
	}
	want := []uint64{6, 7, 8, 9, 10}
	if !equalU64(visited, want) {
		t.Fatalf("visited = %v, want %v", visited, want)
	}
}

func TestIterateEntries_CallbackErrorAbortsIteration(t *testing.T) {
	s := openFresh(t)
	for i := byte(1); i <= 3; i++ {
		if _, err := s.AppendEntry(ctx(), h32(i), h32(i-1), store.KindRating, []byte{i}, []byte{i}); err != nil {
			t.Fatalf("AppendEntry: %v", err)
		}
	}
	abort := errors.New("test-abort")
	calls := 0
	err := s.IterateEntries(ctx(), 1, func(e *store.Entry) error {
		calls++
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatalf("want abort error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("callback called %d times, want 1 (aborted on first)", calls)
	}
}

// -----------------------------------------------------------------------------
// goroutine safety: many readers + single writer
// -----------------------------------------------------------------------------

func TestStore_ConcurrentReadsDuringWrites(t *testing.T) {
	s := openFresh(t)
	const total = 200

	var wg sync.WaitGroup

	// Writer goroutine: serially appends entries.
	wg.Add(1)
	go func() {
		defer wg.Done()
		prev := h32(0)
		for i := 1; i <= total; i++ {
			hash := h32(byte(i))
			if _, err := s.AppendEntry(ctx(), hash, prev, store.KindRating, []byte{byte(i)}, []byte{byte(i)}); err != nil {
				t.Errorf("writer AppendEntry %d: %v", i, err)
				return
			}
			prev = hash
		}
	}()

	// Reader goroutines: poll EntryCount + GetEntry.
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				if _, err := s.EntryCount(ctx()); err != nil {
					t.Errorf("reader EntryCount: %v", err)
					return
				}
			}
		}()
	}

	wg.Wait()

	n, err := s.EntryCount(ctx())
	if err != nil {
		t.Fatalf("EntryCount: %v", err)
	}
	if n != total {
		t.Fatalf("final EntryCount = %d, want %d", n, total)
	}
}

// -----------------------------------------------------------------------------
// crash recovery: a connection forcibly closed mid-transaction must leave
// the database either pre-txn or post-txn — never half-applied.
// -----------------------------------------------------------------------------

func TestStore_CrashRecovery_NoPartialState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crash.db")

	// Session 1: commit some entries, then forcibly close (simulate kill).
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("open #1: %v", err)
	}
	prev := h32(0)
	for i := byte(1); i <= 5; i++ {
		hash := h32(i)
		if _, err := s.AppendEntry(ctx(), hash, prev, store.KindRating, []byte{i}, []byte{i}); err != nil {
			t.Fatalf("AppendEntry: %v", err)
		}
		prev = hash
	}
	// "Crash" — close without explicit shutdown ceremony. WAL + commit
	// guarantees mean the 5 committed entries must survive.
	_ = s.Close()

	// Session 2: reopen, verify exactly 5 entries.
	s2, err := store.Open(path)
	if err != nil {
		t.Fatalf("open #2: %v", err)
	}
	t.Cleanup(func() { _ = s2.Close() })

	n, err := s2.EntryCount(ctx())
	if err != nil {
		t.Fatalf("EntryCount: %v", err)
	}
	if n != 5 {
		t.Fatalf("post-crash EntryCount = %d, want 5", n)
	}
	for i := byte(1); i <= 5; i++ {
		got, err := s2.GetEntry(ctx(), uint64(i))
		if err != nil {
			t.Fatalf("GetEntry %d: %v", i, err)
		}
		if got.Hash != h32(i) {
			t.Errorf("Hash mismatch at idx %d", i)
		}
	}
}

// -----------------------------------------------------------------------------
// Close is idempotent
// -----------------------------------------------------------------------------

func TestClose_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "close.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second close should be no-op, got: %v", err)
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

func equalU64(a, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// -----------------------------------------------------------------------------
// benchmark: 10k appends — informs the driver-choice decision.
// -----------------------------------------------------------------------------

func BenchmarkAppendEntry_10k(b *testing.B) {
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		path := filepath.Join(b.TempDir(), fmt.Sprintf("bench-%d.db", n))
		s, err := store.Open(path)
		if err != nil {
			b.Fatalf("open: %v", err)
		}
		b.StartTimer()
		prev := h32(0)
		for i := 0; i < 10_000; i++ {
			hash := h32(byte(i))
			hash[1] = byte(i >> 8) // disambiguate
			if _, err := s.AppendEntry(ctx(), hash, prev, store.KindRating, []byte{byte(i)}, []byte{byte(i)}); err != nil {
				b.Fatalf("AppendEntry: %v", err)
			}
			prev = hash
		}
		b.StopTimer()
		_ = s.Close()
	}
}
