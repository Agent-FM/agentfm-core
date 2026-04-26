package integration

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"agentfm/internal/network"
	"agentfm/test/testutil"

	netcore "github.com/libp2p/go-libp2p/core/network"
)

// TestFeedbackPayload_PinsTaskField asserts the wire schema the boss sends
// over FeedbackProtocol contains task / feedback / timestamp. Pre-fix the
// boss only sent {feedback, timestamp}, so the worker decoded payload.Task
// as the empty string and the feedback log silently corrupted.
//
// We don't import the boss package (it would drag pterm + the whole TUI
// surface into the test). Instead we manually mimic the boss send and
// assert the worker-side struct decodes all three fields.
func TestFeedbackPayload_PinsTaskField(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	worker, boss := hosts[0], hosts[1]

	type wirePayload struct {
		Task      string `json:"task"`
		Feedback  string `json:"feedback"`
		Timestamp string `json:"timestamp"`
	}

	got := make(chan wirePayload, 1)

	worker.SetStreamHandler(network.FeedbackProtocol, func(s netcore.Stream) {
		defer s.Close()
		_ = s.SetDeadline(time.Now().Add(5 * time.Second))
		var p wirePayload
		if err := json.NewDecoder(io.LimitReader(s, 1024)).Decode(&p); err != nil {
			t.Errorf("decode: %v", err)
			return
		}
		got <- p
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := boss.NewStream(ctx, worker.ID(), network.FeedbackProtocol)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	_ = stream.SetWriteDeadline(time.Now().Add(5 * time.Second))

	send := wirePayload{
		Task:      "task_abc123",
		Feedback:  "thanks!",
		Timestamp: time.Now().Format(time.RFC3339),
	}
	if err := json.NewEncoder(stream).Encode(send); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	_ = stream.CloseWrite()

	select {
	case p := <-got:
		if p.Task != "task_abc123" {
			t.Errorf("Task=%q, want task_abc123", p.Task)
		}
		if p.Feedback != "thanks!" {
			t.Errorf("Feedback=%q, want thanks!", p.Feedback)
		}
		if p.Timestamp == "" {
			t.Errorf("Timestamp empty")
		}
	case <-ctx.Done():
		t.Fatalf("worker never received payload")
	}
}
