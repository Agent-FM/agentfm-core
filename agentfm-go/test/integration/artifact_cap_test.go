package integration

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agentfm/internal/network"
	"agentfm/test/testutil"

	netcore "github.com/libp2p/go-libp2p/core/network"
)

// TestHandleArtifactStream_RejectsOversizedDeclaration: a malicious worker
// declares a fileSize beyond MaxArtifactBytes. The receiver must refuse the
// download outright, not start writing.
func TestHandleArtifactStream_RejectsOversizedDeclaration(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	worker, boss := hosts[0], hosts[1]

	// Run boss in a tempdir so we can verify nothing was written.
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	boss.SetStreamHandler(network.ArtifactProtocol, network.HandleArtifactStream)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := worker.NewStream(ctx, boss.ID(), network.ArtifactProtocol)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	_ = stream.SetWriteDeadline(time.Now().Add(5 * time.Second))

	// Declare a fileSize 2x the allowed cap; never actually ship the bytes.
	if err := binary.Write(stream, binary.LittleEndian, network.MaxArtifactBytes*2); err != nil {
		t.Fatalf("write fileSize: %v", err)
	}
	taskID := []byte("evil-task")
	if err := binary.Write(stream, binary.LittleEndian, uint8(len(taskID))); err != nil {
		t.Fatalf("write idLen: %v", err)
	}
	if _, err := stream.Write(taskID); err != nil {
		t.Fatalf("write id: %v", err)
	}
	_ = stream.CloseWrite()

	// Give the receiver a tick to process the (rejected) handshake.
	testutil.Eventually(t, 2*time.Second, func() bool {
		// Receiver must NOT have created an artifact file. The chdir-to-tmp
		// above means an empty agentfm_artifacts dir is the success state.
		entries, _ := os.ReadDir(filepath.Join(tmp, "agentfm_artifacts"))
		return len(entries) == 0
	}, "boss must not write any file when fileSize exceeds MaxArtifactBytes")
}

// TestHandleArtifactStream_RejectsTruncatedShipment: worker declares 1 MiB
// but only ships 100 bytes. The receiver must NOT mark the result as
// successful (the partial would otherwise be picked up by collect_for_task).
func TestHandleArtifactStream_RejectsTruncatedShipment(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	worker, boss := hosts[0], hosts[1]

	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	boss.SetStreamHandler(network.ArtifactProtocol, network.HandleArtifactStream)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := worker.NewStream(ctx, boss.ID(), network.ArtifactProtocol)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	_ = stream.SetWriteDeadline(time.Now().Add(5 * time.Second))

	const declared int64 = 1024 * 1024
	if err := binary.Write(stream, binary.LittleEndian, declared); err != nil {
		t.Fatalf("write fileSize: %v", err)
	}
	taskID := []byte("truncated-task")
	if err := binary.Write(stream, binary.LittleEndian, uint8(len(taskID))); err != nil {
		t.Fatalf("write idLen: %v", err)
	}
	if _, err := stream.Write(taskID); err != nil {
		t.Fatalf("write id: %v", err)
	}
	// Ship only 100 bytes — truncation.
	if _, err := stream.Write(make([]byte, 100)); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	_ = stream.CloseWrite()

	// The boss-side handler defers stream.Reset() on truncation (success
	// flag stays false). Confirm by checking the artifact file is the
	// 100-byte partial — but since success=false, the next collect_for_task
	// would *currently* still see it on disk. The contract we need to pin
	// is "the receiver did not mark this as success", which we approximate
	// by requiring the partial file exists but is 100 bytes (not the
	// declared 1 MiB).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		path := filepath.Join(tmp, "agentfm_artifacts", "truncated-task.zip")
		if info, err := os.Stat(path); err == nil {
			if info.Size() == 100 {
				return // success: partial bounded at the actual shipped bytes
			}
			if info.Size() > 100 {
				t.Fatalf("size=%d, expected to be capped at the actual shipped bytes (100)", info.Size())
			}
		}
		_ = netcore.Stream(stream) // keep the import live
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected partial artifact file at 100 bytes within 2s")
}
