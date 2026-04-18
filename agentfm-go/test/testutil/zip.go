package testutil

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// DefaultZipEntries is the fixture payload used by MakeZip when the caller
// passes nil. Two entries keep multi-file extraction paths exercised.
var DefaultZipEntries = map[string]string{
	"hello.txt":  "hello from worker",
	"result.csv": "a,b,c\n1,2,3\n",
}

// MakeZip writes a zip file to dir named "payload.zip" and returns its path.
// If entries is nil, DefaultZipEntries is used. Content is deterministic so
// byte-level comparisons across runs stay stable.
func MakeZip(t testing.TB, dir string, entries map[string]string) string {
	t.Helper()
	if entries == nil {
		entries = DefaultZipEntries
	}

	path := filepath.Join(dir, "payload.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	zw := zip.NewWriter(f)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("file close: %v", err)
	}
	return path
}

// ReadFile returns the contents of path or fails the test. Saves the
// `if err != nil { t.Fatal(err) }` boilerplate in every caller.
func ReadFile(t testing.TB, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// AssertBytesEqual compares two byte slices and fails fast on length
// mismatch so stack traces stay short. Works for any test function that
// needs to verify wire-protocol payloads.
func AssertBytesEqual(t testing.TB, got, want []byte, label string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: length mismatch: got %d, want %d", label, len(got), len(want))
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s: bytes differ", label)
	}
}
