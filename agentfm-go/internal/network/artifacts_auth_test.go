package network

import (
	"os"
	"sync"
	"testing"
	"time"

	"agentfm/test/testutil"

	"github.com/libp2p/go-libp2p/core/host"
	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

func registerAuthHandler(h host.Host, authorize func(taskID string, from peer.ID) bool) <-chan struct{} {
	done := make(chan struct{})
	handler := NewArtifactStreamHandler(authorize)
	h.SetStreamHandler(ArtifactProtocol, func(s netcore.Stream) {
		handler(s)
		close(done)
	})
	return done
}

func TestNewArtifactStreamHandler_UnauthorizedRefused(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	sender, receiver := hosts[0], hosts[1]
	t.Chdir(t.TempDir())
	zipPath := testutil.MakeZip(t, t.TempDir(), nil)

	done := registerAuthHandler(receiver, func(string, peer.ID) bool { return false })

	ctx := testutil.WithTimeout(t, 5*time.Second)
	_ = SendArtifacts(ctx, sender, receiver.ID(), zipPath, "task_evil")
	testutil.WaitFor(t, done, 5*time.Second, "handler to refuse")

	entries, err := os.ReadDir("agentfm_artifacts")
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("unauthorized artifact was persisted: %v", entries)
	}
}

func TestNewArtifactStreamHandler_AuthorizedAcceptedWithIdentity(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	sender, receiver := hosts[0], hosts[1]
	t.Chdir(t.TempDir())
	zipPath := testutil.MakeZip(t, t.TempDir(), nil)
	wantBytes := testutil.ReadFile(t, zipPath)

	var mu sync.Mutex
	var gotTaskID string
	var gotFrom peer.ID
	done := registerAuthHandler(receiver, func(taskID string, from peer.ID) bool {
		mu.Lock()
		defer mu.Unlock()
		gotTaskID = taskID
		gotFrom = from
		return true
	})

	ctx := testutil.WithTimeout(t, 5*time.Second)
	const taskID = "task_authorized1"
	if err := SendArtifacts(ctx, sender, receiver.ID(), zipPath, taskID); err != nil {
		t.Fatalf("SendArtifacts: %v", err)
	}
	testutil.WaitFor(t, done, 5*time.Second, "handler to finish")

	mu.Lock()
	defer mu.Unlock()
	if gotTaskID != taskID {
		t.Errorf("authorizer saw taskID %q, want %q", gotTaskID, taskID)
	}
	if gotFrom != sender.ID() {
		t.Errorf("authorizer saw peer %s, want %s", gotFrom, sender.ID())
	}
	gotBytes := testutil.ReadFile(t, "agentfm_artifacts/"+taskID+".zip")
	testutil.AssertBytesEqual(t, gotBytes, wantBytes, "received zip")
}

func TestNewArtifactStreamHandler_NilAuthorizerAllowsAll(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	sender, receiver := hosts[0], hosts[1]
	t.Chdir(t.TempDir())
	zipPath := testutil.MakeZip(t, t.TempDir(), nil)

	done := registerAuthHandler(receiver, nil)

	ctx := testutil.WithTimeout(t, 5*time.Second)
	const taskID = "task_nilauth"
	if err := SendArtifacts(ctx, sender, receiver.ID(), zipPath, taskID); err != nil {
		t.Fatalf("SendArtifacts: %v", err)
	}
	testutil.WaitFor(t, done, 5*time.Second, "handler to finish")

	if _, err := os.Stat("agentfm_artifacts/" + taskID + ".zip"); err != nil {
		t.Fatalf("expected artifact with nil authorizer: %v", err)
	}
}
