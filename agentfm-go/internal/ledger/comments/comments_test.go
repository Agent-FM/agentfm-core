package comments_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentfm/internal/ledger/comments"
	"agentfm/internal/network"
	"agentfm/test/testutil"

	libnet "github.com/libp2p/go-libp2p/core/network"
)

func TestCIDOf_Deterministic(t *testing.T) {
	a := comments.CIDOf([]byte("hello"))
	b := comments.CIDOf([]byte("hello"))
	if !bytes.Equal(a, b) {
		t.Fatalf("CIDOf non-deterministic: %x vs %x", a, b)
	}
}

func TestCIDOf_DifferentBytes_DifferentCIDs(t *testing.T) {
	a := comments.CIDOf([]byte("a"))
	b := comments.CIDOf([]byte("b"))
	if bytes.Equal(a, b) {
		t.Fatal("collision on trivial inputs")
	}
}

func TestCIDString_RoundTrip(t *testing.T) {
	cid := comments.CIDOf([]byte("round-trip"))
	s := comments.CIDString(cid)
	parsed, err := comments.ParseCIDString(s)
	if err != nil {
		t.Fatalf("ParseCIDString: %v", err)
	}
	if !bytes.Equal(parsed, cid) {
		t.Fatalf("round-trip mismatch")
	}
}

func TestParseCIDString_Malformed(t *testing.T) {
	cases := []string{"", "not-hex", strings.Repeat("aa", 50)}
	for _, c := range cases {
		_, err := comments.ParseCIDString(c)
		if err == nil {
			t.Errorf("ParseCIDString(%q) should fail", c)
		}
	}
}

func TestStore_PutGet(t *testing.T) {
	s, err := comments.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	cid, err := s.Put([]byte("a thoughtful review"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := s.Get(cid)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "a thoughtful review" {
		t.Fatalf("body mismatch: %q", got)
	}
}

func TestStore_Put_Idempotent(t *testing.T) {
	s, _ := comments.Open(t.TempDir())
	c1, _ := s.Put([]byte("same body"))
	c2, _ := s.Put([]byte("same body"))
	if !bytes.Equal(c1, c2) {
		t.Fatal("Put returned different CIDs for same body")
	}
}

func TestStore_Put_TooLarge_Rejected(t *testing.T) {
	s, _ := comments.Open(t.TempDir())
	big := make([]byte, comments.MaxBodyBytes+1)
	_, err := s.Put(big)
	if !errors.Is(err, comments.ErrBodyTooLarge) {
		t.Fatalf("want ErrBodyTooLarge, got %v", err)
	}
}

func TestStore_Get_NotFound(t *testing.T) {
	s, _ := comments.Open(t.TempDir())
	_, err := s.Get(comments.CIDOf([]byte("not-stored")))
	if !errors.Is(err, comments.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestStore_Get_TamperedFileFails(t *testing.T) {
	root := t.TempDir()
	s, _ := comments.Open(root)
	cid, _ := s.Put([]byte("original"))

	// Walk to find the stored file and rewrite it with different bytes.
	var found string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			found = path
		}
		return nil
	})
	if found == "" {
		t.Fatal("could not locate stored file for tamper test")
	}
	if err := os.WriteFile(found, []byte("tampered"), 0o600); err != nil {
		t.Fatalf("tamper write: %v", err)
	}
	_, err := s.Get(cid)
	if !errors.Is(err, comments.ErrCIDMismatch) {
		t.Fatalf("want ErrCIDMismatch, got %v", err)
	}
}

func TestStore_Delete(t *testing.T) {
	s, _ := comments.Open(t.TempDir())
	cid, _ := s.Put([]byte("ephemeral"))
	if !s.Has(cid) {
		t.Fatal("Has should be true before Delete")
	}
	if err := s.Delete(cid); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if s.Has(cid) {
		t.Fatal("Has should be false after Delete")
	}
	// Idempotent.
	if err := s.Delete(cid); err != nil {
		t.Fatalf("Delete (second time) should be no-op, got: %v", err)
	}
}

// End-to-end fetch over real libp2p: server stores a body, client
// pulls it via CommentFetchProtocol.
func TestFetch_RoundTrip(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	srvHost, cliHost := hosts[0], hosts[1]

	s, _ := comments.Open(t.TempDir())
	srv := comments.NewServer(srvHost, s)
	srv.Start()
	t.Cleanup(srv.Stop)

	cid, err := s.Put([]byte("hello from the other peer"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	ctx := context.Background()
	body, err := comments.Fetch(ctx, cliHost, srvHost.ID(), cid)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(body) != "hello from the other peer" {
		t.Fatalf("body mismatch: %q", body)
	}
}

func TestFetch_CtxBoundsStalledServer(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	srvHost, cliHost := hosts[0], hosts[1]

	stall := make(chan struct{})
	t.Cleanup(func() { close(stall) })
	srvHost.SetStreamHandler(network.CommentFetchProtocol, func(s libnet.Stream) {
		<-stall
		_ = s.Reset()
	})

	cid := comments.CIDOf([]byte("body the server will never send"))
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	_, err := comments.Fetch(ctx, cliHost, srvHost.ID(), cid)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Fetch against a stalled server should fail")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Fetch ignored ctx deadline: took %v against a stalled server (want ~1s)", elapsed)
	}
}

func TestFetch_CancelUnblocksImmediately(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	srvHost, cliHost := hosts[0], hosts[1]

	stall := make(chan struct{})
	t.Cleanup(func() { close(stall) })
	srvHost.SetStreamHandler(network.CommentFetchProtocol, func(s libnet.Stream) {
		<-stall
		_ = s.Reset()
	})

	cid := comments.CIDOf([]byte("never delivered"))
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := comments.Fetch(ctx, cliHost, srvHost.ID(), cid)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Fetch should fail when ctx is cancelled mid-flight")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Fetch did not unblock on ctx cancel: took %v (want ~300ms)", elapsed)
	}
}

func TestFetch_NotFound(t *testing.T) {
	hosts := testutil.NewConnectedMesh(t, 2)
	srvHost, cliHost := hosts[0], hosts[1]

	s, _ := comments.Open(t.TempDir())
	srv := comments.NewServer(srvHost, s)
	srv.Start()
	t.Cleanup(srv.Stop)

	missing := comments.CIDOf([]byte("not stored on server"))
	_, err := comments.Fetch(context.Background(), cliHost, srvHost.ID(), missing)
	if !errors.Is(err, comments.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
