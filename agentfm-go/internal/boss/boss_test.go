package boss

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"agentfm/test/testutil"

	"github.com/libp2p/go-libp2p/core/network"
)

const testEchoProto = "/agentfm-test/echo/1.0.0"

// TestTimeoutReader_ReadsNormally: the decorator must be transparent on the
// happy path. Bytes flow through, errors are forwarded, deadline refreshes
// don't interfere with a quick writer.
func TestTimeoutReader_ReadsNormally(t *testing.T) {
	server := testutil.NewHost(t)
	client := testutil.NewHost(t)
	testutil.ConnectHosts(t, client, server)

	server.SetStreamHandler(testEchoProto, func(s network.Stream) {
		_, _ = s.Write([]byte("hello"))
		_ = s.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := client.NewStream(ctx, server.ID(), testEchoProto)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	tr := &timeoutReader{stream: s, timeout: 2 * time.Second}
	data, err := io.ReadAll(tr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(data, []byte("hello")) {
		t.Errorf("got %q, want %q", data, "hello")
	}
}

// TestTimeoutReader_DeadlineFires: the whole point of the reader. A server
// that opens the stream but never writes must cause Read to return within
// roughly the configured timeout — proving that SetReadDeadline is actually
// being applied per-call.
func TestTimeoutReader_DeadlineFires(t *testing.T) {
	server := testutil.NewHost(t)
	client := testutil.NewHost(t)
	testutil.ConnectHosts(t, client, server)

	// Server opens the stream and blocks forever (until test cleanup).
	serverCtx, serverCancel := context.WithCancel(context.Background())
	t.Cleanup(serverCancel)
	server.SetStreamHandler(testEchoProto, func(s network.Stream) {
		<-serverCtx.Done()
		_ = s.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := client.NewStream(ctx, server.ID(), testEchoProto)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	const timeout = 150 * time.Millisecond
	tr := &timeoutReader{stream: s, timeout: timeout}

	start := time.Now()
	_, err = tr.Read(make([]byte, 10))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout/deadline error, got nil")
	}
	// Reasonable tolerance: real libp2p scheduling can add ~100ms.
	if elapsed < timeout {
		t.Errorf("Read returned before deadline: %v (want >= %v)", elapsed, timeout)
	}
	if elapsed > timeout+500*time.Millisecond {
		t.Errorf("Read took suspiciously long: %v (want ~%v)", elapsed, timeout)
	}
}

// TestTimeoutReader_DeadlineRefreshesPerRead asserts that a slow writer
// sending one byte every 80ms under a 200ms timeout succeeds indefinitely —
// each Read resets the deadline clock rather than draining a shared budget.
// This is the critical invariant for AgentFM's 10-minute task-stream idle
// timeout: active streams never spuriously time out.
func TestTimeoutReader_DeadlineRefreshesPerRead(t *testing.T) {
	server := testutil.NewHost(t)
	client := testutil.NewHost(t)
	testutil.ConnectHosts(t, client, server)

	server.SetStreamHandler(testEchoProto, func(s network.Stream) {
		defer s.Close()
		// Five writes, 80ms apart. Total 400ms — well past a 200ms
		// cumulative budget, but each individual gap is under 200ms.
		for i := 0; i < 5; i++ {
			_, _ = s.Write([]byte{byte('a' + i)})
			time.Sleep(80 * time.Millisecond)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := client.NewStream(ctx, server.ID(), testEchoProto)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	tr := &timeoutReader{stream: s, timeout: 200 * time.Millisecond}

	got, err := io.ReadAll(tr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	want := []byte("abcde")
	if !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
