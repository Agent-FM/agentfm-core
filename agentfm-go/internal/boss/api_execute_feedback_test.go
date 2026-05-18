package boss

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agentfm/internal/ledger/comments"
	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/network"
	"agentfm/internal/types"
	"agentfm/test/testutil"

	netcore "github.com/libp2p/go-libp2p/core/network"
)

// recordingLedger captures Append calls for inspection.
type recordingLedger struct {
	stubLedger
	appended []*pb.SignedEntry
}

func (l *recordingLedger) Append(_ context.Context, payload *pb.SignedEntry) ([32]byte, error) {
	l.appended = append(l.appended, payload)
	return [32]byte{}, nil
}

// TestHandleExecuteTask_FeedbackPersistsComment exercises the happy path
// where /api/execute receives a feedback field and the ledger records a Comment.
func TestHandleExecuteTask_FeedbackPersistsComment(t *testing.T) {
	// Spin up a minimal task-protocol worker.
	workerHost := testutil.NewHost(t)
	workerHost.SetStreamHandler(network.TaskProtocol, func(s netcore.Stream) {
		defer s.Close()
		_, _ = io.ReadAll(io.LimitReader(s, 2*1024*1024))
		_, _ = s.Write([]byte("done\n"))
	})

	// Build a Boss wired with a recording ledger + real comments store.
	bossHost := testutil.NewHost(t)
	testutil.ConnectHosts(t, bossHost, workerHost)

	dir := t.TempDir()
	cs, err := comments.Open(dir)
	if err != nil {
		t.Fatalf("comments.Open: %v", err)
	}

	recLedger := &recordingLedger{}
	// completionRater must be non-nil for the feedback guard to trigger.
	crw := NewCompletionRatingWriter(recLedger, bossHost)

	b := &Boss{
		node:          &network.MeshNode{Host: bossHost},
		activeWorkers: make(map[string]types.WorkerProfile),
		lastSeen:      make(map[string]time.Time),
		ledger:        recLedger,
		commentsStore: cs,
		completionRater: crw,
	}
	workerPID := workerHost.ID()
	b.activeWorkers[workerPID.String()] = types.WorkerProfile{
		PeerID:   workerPID.String(),
		CPUCores: 4,
	}

	body, _ := json.Marshal(map[string]interface{}{
		"worker_id": workerPID.String(),
		"prompt":    "do something",
		"task_id":   "task_test1",
		"feedback":  "great work",
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/execute", bytes.NewReader(body))
	b.handleExecuteTask(rec, req)

	// The task should succeed (200-ish: text/plain streaming, no status code written
	// before streaming starts, so the recorder will show 200).
	if rec.Code != http.StatusOK {
		t.Logf("response body: %s", rec.Body.String())
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Count Comment entries in the recording ledger.
	var commentCount int
	for _, e := range recLedger.appended {
		if e.GetComment() != nil {
			commentCount++
		}
	}
	if commentCount != 1 {
		t.Fatalf("expected 1 Comment in ledger; got %d (total appended=%d)", commentCount, len(recLedger.appended))
	}
}

// TestHandleExecuteTask_FeedbackWithRating verifies that feedback_rating also
// results in a Rating entry in the ledger.
func TestHandleExecuteTask_FeedbackWithRating(t *testing.T) {
	workerHost := testutil.NewHost(t)
	workerHost.SetStreamHandler(network.TaskProtocol, func(s netcore.Stream) {
		defer s.Close()
		_, _ = io.ReadAll(io.LimitReader(s, 2*1024*1024))
		_, _ = s.Write([]byte("done\n"))
	})

	bossHost := testutil.NewHost(t)
	testutil.ConnectHosts(t, bossHost, workerHost)

	dir := t.TempDir()
	cs, err := comments.Open(dir)
	if err != nil {
		t.Fatalf("comments.Open: %v", err)
	}

	recLedger := &recordingLedger{}
	crw := NewCompletionRatingWriter(recLedger, bossHost)

	b := &Boss{
		node:            &network.MeshNode{Host: bossHost},
		activeWorkers:   make(map[string]types.WorkerProfile),
		lastSeen:        make(map[string]time.Time),
		ledger:          recLedger,
		commentsStore:   cs,
		completionRater: crw,
	}
	workerPID := workerHost.ID()
	b.activeWorkers[workerPID.String()] = types.WorkerProfile{
		PeerID:   workerPID.String(),
		CPUCores: 4,
	}

	ratingVal := 0.8
	body, _ := json.Marshal(map[string]interface{}{
		"worker_id":       workerPID.String(),
		"prompt":          "do something",
		"task_id":         "task_test2",
		"feedback":        "excellent",
		"feedback_rating": ratingVal,
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/execute", bytes.NewReader(body))
	b.handleExecuteTask(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var commentCount, ratingCount int
	for _, e := range recLedger.appended {
		if e.GetComment() != nil {
			commentCount++
		}
		if e.GetRating() != nil {
			// Only count interactive ratings (not completion-rater ones).
			if e.GetRating().GetContext() == "interactive" {
				ratingCount++
			}
		}
	}
	if commentCount != 1 || ratingCount != 1 {
		t.Fatalf("want 1 comment + 1 interactive rating; got %d + %d", commentCount, ratingCount)
	}
}

// TestHandleExecuteTask_NoFeedbackSkipsPersist verifies that omitting feedback
// does not append any Comment entries to the ledger.
func TestHandleExecuteTask_NoFeedbackSkipsPersist(t *testing.T) {
	workerHost := testutil.NewHost(t)
	workerHost.SetStreamHandler(network.TaskProtocol, func(s netcore.Stream) {
		defer s.Close()
		_, _ = io.ReadAll(io.LimitReader(s, 2*1024*1024))
		_, _ = s.Write([]byte("done\n"))
	})

	bossHost := testutil.NewHost(t)
	testutil.ConnectHosts(t, bossHost, workerHost)

	dir := t.TempDir()
	cs, err := comments.Open(dir)
	if err != nil {
		t.Fatalf("comments.Open: %v", err)
	}

	recLedger := &recordingLedger{}
	crw := NewCompletionRatingWriter(recLedger, bossHost)

	b := &Boss{
		node:            &network.MeshNode{Host: bossHost},
		activeWorkers:   make(map[string]types.WorkerProfile),
		lastSeen:        make(map[string]time.Time),
		ledger:          recLedger,
		commentsStore:   cs,
		completionRater: crw,
	}
	workerPID := workerHost.ID()
	b.activeWorkers[workerPID.String()] = types.WorkerProfile{
		PeerID:   workerPID.String(),
		CPUCores: 4,
	}

	body, _ := json.Marshal(map[string]interface{}{
		"worker_id": workerPID.String(),
		"prompt":    "do something",
		"task_id":   "task_test3",
		// No "feedback" field.
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/execute", bytes.NewReader(body))
	b.handleExecuteTask(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	for _, e := range recLedger.appended {
		if e.GetComment() != nil {
			t.Fatal("expected no Comment in ledger when feedback field is absent")
		}
	}
}

