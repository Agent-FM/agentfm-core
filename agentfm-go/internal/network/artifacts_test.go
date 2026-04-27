package network

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentfm/test/testutil"

	"github.com/libp2p/go-libp2p/core/host"
	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// --- helpers ---------------------------------------------------------------

// registerSignallingHandler wraps HandleArtifactStream so tests can wait for
// it to finish without sleeping. Returns a channel that closes once the real
// handler returns.
func registerSignallingHandler(h host.Host) <-chan struct{} {
	done := make(chan struct{})
	h.SetStreamHandler(ArtifactProtocol, func(s netcore.Stream) {
		HandleArtifactStream(s)
		close(done)
	})
	return done
}

// --- happy path ------------------------------------------------------------

// TestSendAndHandleArtifactStream_RoundTrip is the end-to-end contract test
// for the artifact wire protocol. We send a known zip from one mocknet peer
// to another and byte-compare what lands on disk against what was sent.
// This guards against regressions in the size/taskID/body framing AND in the
// path-traversal sanitization for the output filename.
func TestSendAndHandleArtifactStream_RoundTrip(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	sender, receiver := hosts[0], hosts[1]

	// HandleArtifactStream hardcodes "./agentfm_artifacts" relative to cwd,
	// so isolate the test with t.Chdir. Incompatible with t.Parallel.
	t.Chdir(t.TempDir())

	zipPath := testutil.MakeZip(t, t.TempDir(), nil)
	wantBytes := testutil.ReadFile(t, zipPath)

	done := registerSignallingHandler(receiver)

	ctx := testutil.WithTimeout(t, 5*time.Second)
	const taskID = "task_1234567890"
	if err := SendArtifacts(ctx, sender, receiver.ID(), zipPath, taskID); err != nil {
		t.Fatalf("SendArtifacts: %v", err)
	}
	testutil.WaitFor(t, done, 5*time.Second, "artifact handler to finish")

	// Verify the receiver wrote exactly what the sender streamed, under the
	// sanitized task id, into the fixed ./agentfm_artifacts dir.
	gotPath := filepath.Join("agentfm_artifacts", taskID+".zip")
	gotBytes := testutil.ReadFile(t, gotPath)
	testutil.AssertBytesEqual(t, gotBytes, wantBytes, "received zip")
}

// --- handler edge cases ----------------------------------------------------

// TestHandleArtifactStream_TruncatedPayloads exercises the three points in
// the stream framing where a malicious or crashed peer could send partial
// data. Each row expects the handler to log + Reset and produce no file.
func TestHandleArtifactStream_TruncatedPayloads(t *testing.T) {
	cases := []struct {
		name  string
		write func(io.Writer) // what the fake sender writes before closing
	}{
		{
			name: "nothing written (empty stream)",
			// Peer opens stream and closes it immediately. Handler's binary.Read
			// for fileSize hits io.EOF.
			write: func(io.Writer) {},
		},
		{
			name: "size header truncated",
			// Only 4 of 8 size bytes. Handler bails at size read.
			write: func(w io.Writer) { _, _ = w.Write([]byte{1, 2, 3, 4}) },
		},
		{
			name: "taskID length byte missing",
			// Full 8-byte size header, then nothing. Handler bails at idLen.
			write: func(w io.Writer) {
				_ = binary.Write(w, binary.LittleEndian, int64(10))
			},
		},
		{
			name: "taskID body truncated",
			// idLen = 10 but only 3 bytes of taskID sent.
			write: func(w io.Writer) {
				_ = binary.Write(w, binary.LittleEndian, int64(10))
				_ = binary.Write(w, binary.LittleEndian, uint8(10))
				_, _ = w.Write([]byte("abc"))
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			hosts := testutil.NewConnectedMesh(t, 2)
			sender, receiver := hosts[0], hosts[1]

			t.Chdir(t.TempDir())
			done := registerSignallingHandler(receiver)

			ctx := testutil.WithTimeout(t, 3*time.Second)
			s, err := sender.NewStream(ctx, receiver.ID(), ArtifactProtocol)
			if err != nil {
				t.Fatalf("NewStream: %v", err)
			}
			tc.write(s)
			_ = s.Close()

			testutil.WaitFor(t, done, 3*time.Second, "handler to abort")

			// Verify no zip was persisted — the whole point of truncation detection.
			entries, err := os.ReadDir("agentfm_artifacts")
			if err != nil && !os.IsNotExist(err) {
				t.Fatalf("readdir: %v", err)
			}
			if len(entries) != 0 {
				names := make([]string, len(entries))
				for i, e := range entries {
					names[i] = e.Name()
				}
				t.Errorf("expected no artifacts, got %v", names)
			}
		})
	}
}

// TestHandleArtifactStream_TaskIDSanitization verifies the path-traversal
// defence. A malicious worker cannot escape ./agentfm_artifacts/ via ".."
// components in the task ID, and cannot crash us with unusually short/empty
// task IDs (regression guard for the `safeTaskID[:8]` panic we fixed).
func TestHandleArtifactStream_TaskIDSanitization(t *testing.T) {
	cases := []struct {
		name           string
		taskID         string
		wantFilePrefix string // expected filename (sans ".zip") or prefix for fallback ids
	}{
		{
			name:           "path traversal dots",
			taskID:         "../../etc/passwd",
			wantFilePrefix: "passwd",
		},
		{
			name:           "absolute path",
			taskID:         "/absolute/dangerous/path",
			wantFilePrefix: "path",
		},
		{
			name:           "current dir",
			taskID:         ".",
			wantFilePrefix: "fallback_",
		},
		{
			name:           "leading dot rejected",
			taskID:         ".hidden",
			wantFilePrefix: "fallback_",
		},
		{
			name:           "control bytes rejected",
			taskID:         "ok\x00bad",
			wantFilePrefix: "fallback_",
		},
		{
			name:           "short id (regression: safeTaskID[:8] panic)",
			taskID:         "abc",
			wantFilePrefix: "abc",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			hosts := testutil.NewConnectedMesh(t, 2)
			sender, receiver := hosts[0], hosts[1]

			t.Chdir(t.TempDir())
			zipPath := testutil.MakeZip(t, t.TempDir(), nil)
			done := registerSignallingHandler(receiver)

			ctx := testutil.WithTimeout(t, 5*time.Second)
			if err := SendArtifacts(ctx, sender, receiver.ID(), zipPath, tc.taskID); err != nil {
				t.Fatalf("SendArtifacts: %v", err)
			}
			testutil.WaitFor(t, done, 5*time.Second, "handler to finish")

			entries, err := os.ReadDir("agentfm_artifacts")
			if err != nil {
				t.Fatalf("readdir: %v", err)
			}
			if len(entries) != 1 {
				t.Fatalf("expected 1 file, got %d", len(entries))
			}
			got := entries[0].Name()
			if !strings.HasPrefix(got, tc.wantFilePrefix) {
				t.Errorf("filename %q does not start with %q", got, tc.wantFilePrefix)
			}
			// Defence-in-depth: assert the file lives inside the artifact dir.
			absDir, _ := filepath.Abs("agentfm_artifacts")
			absFile, _ := filepath.Abs(filepath.Join("agentfm_artifacts", got))
			if !strings.HasPrefix(absFile, absDir) {
				t.Errorf("file escaped artifact dir: %s not under %s", absFile, absDir)
			}
		})
	}
}

func TestHandleArtifactStream_RejectsZeroLengthTaskID(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	sender, receiver := hosts[0], hosts[1]
	t.Chdir(t.TempDir())
	done := registerSignallingHandler(receiver)

	ctx := testutil.WithTimeout(t, 3*time.Second)
	s, err := sender.NewStream(ctx, receiver.ID(), ArtifactProtocol)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	_ = binary.Write(s, binary.LittleEndian, int64(285))
	_ = binary.Write(s, binary.LittleEndian, uint8(0))
	_, _ = s.Write(bytes.Repeat([]byte{0xAA}, 285))
	_ = s.Close()

	testutil.WaitFor(t, done, 3*time.Second, "handler to finish")

	if _, err := os.Stat("agentfm_artifacts"); !os.IsNotExist(err) {
		entries, _ := os.ReadDir("agentfm_artifacts")
		t.Errorf("zero-length task-id should produce no file; got %d entries", len(entries))
	}
}

// TestHandleArtifactStream_BodyShorterThanHeader simulates a peer advertising
// more bytes than it actually sends. io.Copy on the handler side returns
// without error when the peer closes cleanly, so the file will be short — we
// assert the handler does not claim success beyond what was received.
func TestHandleArtifactStream_BodyShorterThanHeader(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	sender, receiver := hosts[0], hosts[1]
	t.Chdir(t.TempDir())
	done := registerSignallingHandler(receiver)

	const (
		advertised = int64(10_000)
		actual     = 500
	)
	ctx := testutil.WithTimeout(t, 3*time.Second)
	s, err := sender.NewStream(ctx, receiver.ID(), ArtifactProtocol)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	_ = binary.Write(s, binary.LittleEndian, advertised)
	taskID := []byte("shorty")
	_ = binary.Write(s, binary.LittleEndian, uint8(len(taskID)))
	_, _ = s.Write(taskID)
	_, _ = s.Write(bytes.Repeat([]byte{0xAA}, actual))
	_ = s.Close()

	testutil.WaitFor(t, done, 3*time.Second, "handler to finish")

	gotPath := filepath.Join("agentfm_artifacts", "shorty.zip")
	data, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	// Body is shorter than header; file on disk must reflect what was actually
	// received, never padded or magically completed.
	if int64(len(data)) >= advertised {
		t.Errorf("unexpectedly got %d bytes, wanted <= %d", len(data), advertised)
	}
	if len(data) != actual {
		t.Errorf("got %d bytes, want exactly %d (actual stream length)", len(data), actual)
	}
}

// --- SendArtifacts error paths ---------------------------------------------

// TestSendArtifacts_Errors verifies that each failure mode returns a wrapped,
// observable error rather than silently succeeding.
func TestSendArtifacts_Errors(t *testing.T) {
	t.Run("missing zip file", func(t *testing.T) {
		hosts := testutil.NewConnectedMesh(t, 2)
		receiver := hosts[1]
		_ = registerSignallingHandler(receiver)

		ctx := testutil.WithTimeout(t, 3*time.Second)
		err := SendArtifacts(ctx, hosts[0], receiver.ID(), "/does/not/exist.zip", "t1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to open zip file") {
			t.Errorf("want wrapped open-zip error, got: %v", err)
		}
	})

	t.Run("dial context already cancelled", func(t *testing.T) {
		hosts := testutil.NewConnectedMesh(t, 2)
		_ = registerSignallingHandler(hosts[1])

		// Cancelled ctx short-circuits NewStream without waiting for StreamDialTimeout.
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		zipPath := testutil.MakeZip(t, t.TempDir(), nil)
		err := SendArtifacts(ctx, hosts[0], hosts[1].ID(), zipPath, "t1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to open artifact stream") {
			t.Errorf("want wrapped open-stream error, got: %v", err)
		}
	})

	t.Run("receiver resets mid-transfer", func(t *testing.T) {
		hosts := testutil.NewConnectedMesh(t, 2)
		sender, receiver := hosts[0], hosts[1]

		// Custom handler: read just the size header, then Reset. The sender's
		// io.Copy should observe the reset and return an error.
		receiver.SetStreamHandler(ArtifactProtocol, func(s netcore.Stream) {
			var size int64
			_ = binary.Read(s, binary.LittleEndian, &size)
			_ = s.Reset()
		})

		// Use a zip large enough that Copy is in-flight when the reset lands.
		tmp := t.TempDir()
		zipPath := filepath.Join(tmp, "big.zip")
		if err := os.WriteFile(zipPath, bytes.Repeat([]byte("x"), 256*1024), 0o600); err != nil {
			t.Fatalf("write big zip: %v", err)
		}

		ctx := testutil.WithTimeout(t, 5*time.Second)
		err := SendArtifacts(ctx, sender, receiver.ID(), zipPath, "mid_transfer")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "artifact stream") {
			t.Errorf("want wrapped stream error, got: %v", err)
		}
	})

	t.Run("unreachable peer id", func(t *testing.T) {
		hosts := testutil.NewConnectedMesh(t, 1)
		sender := hosts[0]

		// A peer ID that isn't in the mocknet — dial must fail.
		// We manufacture a bogus peer.ID by decoding a valid-format string
		// that simply doesn't correspond to any linked peer.
		bogusID, err := peer.Decode("12D3KooWGRUacXc4oieAeoKvk3zQvkgRadLmuVf4SVy23bY2gXxT")
		if err != nil {
			t.Fatalf("peer.Decode: %v", err)
		}

		zipPath := testutil.MakeZip(t, t.TempDir(), nil)
		ctx := testutil.WithTimeout(t, 3*time.Second)
		err = SendArtifacts(ctx, sender, bogusID, zipPath, "unreachable")
		if err == nil {
			t.Fatal("expected error dialing unknown peer, got nil")
		}
	})
}

// --- progressWriter --------------------------------------------------------

// TestProgressWriter_PassesThroughBytes verifies the decorator correctness:
// bytes land in the underlying writer unchanged, and the returned (n, err)
// mirrors the wrapped writer's behaviour. With a nil progress bar it must
// not panic — that's the branch HandleArtifactStream exercises on short
// writes before pterm has had a chance to render.
func TestProgressWriter_PassesThroughBytes(t *testing.T) {
	t.Parallel()

	var underlying bytes.Buffer
	pw := &progressWriter{Writer: &underlying, pb: nil}

	payload := []byte("hello world, some artifact bytes")
	n, err := pw.Write(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(payload) {
		t.Errorf("n = %d, want %d", n, len(payload))
	}
	if got := underlying.Bytes(); !bytes.Equal(got, payload) {
		t.Errorf("underlying bytes mismatch: got %q", got)
	}
}
