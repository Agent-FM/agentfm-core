// Package comments implements the content-addressed body store for
// P4-1 comments. Comments are split across two layers:
//
//   - The Merkle ledger holds a SignedEntry/Comment envelope with a
//     text_cid pointer (a multihash of the body). This keeps the
//     Merkle leaves small and bandwidth-light during gossip.
//   - The body itself lives as a file at
//     ~/.agentfm/comments/<cid-prefix>/<cid>, fetched on demand by
//     anyone holding a Comment envelope via the
//     /agentfm/comment-fetch/1.0.0 stream protocol.
//
// CID format: a 2-byte multihash header + 32 raw SHA-256 bytes. The
// header is the standard IPFS multihash convention so future
// integrations don't have to fight a custom encoding. The on-disk +
// URL form is hex (lower-case) for human readability.
package comments

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// MaxBodyBytes is the cap on a single comment body (P4-1 §4). Larger
// submissions are rejected at the API boundary before any disk write.
// 10 KiB is plenty for a written review and keeps the on-disk
// footprint bounded without operator tuning.
const MaxBodyBytes = 10 * 1024

// MultihashPrefix is the 2-byte multihash header for "sha2-256 / 32
// bytes": 0x12 0x20. Prepended to the 32-byte digest to form a CID.
var MultihashPrefix = [2]byte{0x12, 0x20}

// ErrBodyTooLarge is returned by Put when len(body) > MaxBodyBytes.
var ErrBodyTooLarge = errors.New("comments: body exceeds 10 KiB cap")

// ErrCIDMismatch is returned by Get when the file at the expected
// path doesn't hash back to the requested CID. Indicates tampering
// or filesystem corruption.
var ErrCIDMismatch = errors.New("comments: stored body does not match CID")

// ErrNotFound is returned by Get when no body is stored for the
// requested CID. Surfaced by the fetch protocol as a typed wire
// error so the requester can retry against a different peer.
var ErrNotFound = errors.New("comments: body not found")

// CIDOf returns the canonical CID for body. The same body always
// produces the same CID — the SHA-256 of the body bytes, wrapped
// with the multihash header.
func CIDOf(body []byte) []byte {
	digest := sha256.Sum256(body)
	out := make([]byte, 0, 2+32)
	out = append(out, MultihashPrefix[:]...)
	out = append(out, digest[:]...)
	return out
}

// CIDString renders a CID as a lowercase hex string suitable for
// URLs and filenames. Inverse of ParseCIDString.
func CIDString(cid []byte) string {
	return hex.EncodeToString(cid)
}

// ParseCIDString returns the raw CID bytes for a hex-encoded CID
// string (the output of CIDString). Returns an error on malformed
// input or wrong length.
func ParseCIDString(s string) ([]byte, error) {
	bs, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("comments: parse cid: %w", err)
	}
	if len(bs) != 2+32 {
		return nil, fmt.Errorf("comments: parse cid: unexpected length %d (want %d)", len(bs), 2+32)
	}
	if bs[0] != MultihashPrefix[0] || bs[1] != MultihashPrefix[1] {
		return nil, fmt.Errorf("comments: parse cid: bad multihash prefix")
	}
	return bs, nil
}

// Store is the per-peer body store. Concurrent access is safe; the
// underlying filesystem operations are atomic per file.
type Store struct {
	root string // e.g. ~/.agentfm/comments

	// mu guards the in-flight set so two concurrent Put calls for
	// the same body don't race on the rename step.
	mu sync.Mutex
}

// Open returns a Store rooted at the given directory. The directory
// is created if it doesn't exist (with 0700 perms — comments may
// contain user reviews).
func Open(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("comments: empty root")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("comments: mkdir %s: %w", root, err)
	}
	return &Store{root: root}, nil
}

// Put writes body to the content-addressed slot for its CID. Returns
// the CID. Idempotent — writing the same body twice is a no-op on
// the second call.
//
// Returns ErrBodyTooLarge if len(body) > MaxBodyBytes.
func (s *Store) Put(body []byte) ([]byte, error) {
	if len(body) > MaxBodyBytes {
		return nil, fmt.Errorf("%w: %d bytes", ErrBodyTooLarge, len(body))
	}
	cid := CIDOf(body)
	path := s.pathFor(cid)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Idempotent: if the file already exists, we're done.
	if _, err := os.Stat(path); err == nil {
		return cid, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("comments: mkdir bucket: %w", err)
	}

	// Write to a temp file in the same directory then rename — atomic
	// on POSIX, so a concurrent reader either sees the old (absent)
	// state or the new full file.
	tmp, err := os.CreateTemp(filepath.Dir(path), "tmp-*")
	if err != nil {
		return nil, fmt.Errorf("comments: open tmp: %w", err)
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("comments: write tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("comments: close tmp: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("comments: rename tmp: %w", err)
	}
	return cid, nil
}

// Get returns the body stored at cid. Returns ErrNotFound when the
// CID is unknown. Returns ErrCIDMismatch when the on-disk bytes
// don't hash back to cid — only happens on filesystem corruption
// or a malicious operator that modified the file directly.
func (s *Store) Get(cid []byte) ([]byte, error) {
	path := s.pathFor(cid)
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("comments: read: %w", err)
	}
	want := CIDOf(body)
	if !equalBytes(want, cid) {
		return nil, ErrCIDMismatch
	}
	return body, nil
}

// Has reports whether a body is stored for cid. Cheap — uses
// os.Stat rather than reading the file.
func (s *Store) Has(cid []byte) bool {
	_, err := os.Stat(s.pathFor(cid))
	return err == nil
}

// Delete removes a body. Used by the GDPR-style redaction path
// (P4-1 hooks; not exposed to v1.3 HTTP API). Returns nil even if
// the body was already absent — idempotent.
func (s *Store) Delete(cid []byte) error {
	path := s.pathFor(cid)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("comments: delete: %w", err)
	}
	return nil
}

// pathFor returns the on-disk path for a CID. Sharded by the first
// two hex characters of the CID so directories don't grow unbounded.
func (s *Store) pathFor(cid []byte) string {
	str := CIDString(cid)
	if len(str) < 2 {
		// Defensive — caller should not pass empty CIDs.
		return filepath.Join(s.root, "_bad", str)
	}
	return filepath.Join(s.root, str[:2], str)
}

// WriteAtomic is exported for the fetch handler to stream a received
// body to disk while validating against an expected CID before
// commit. Used to avoid a write-then-validate-then-delete cycle on
// the receive path.
func (s *Store) WriteAtomic(expectedCID []byte, r io.Reader, maxBytes int64) error {
	limited := &io.LimitedReader{R: r, N: maxBytes + 1}
	body, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("comments: read incoming: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return ErrBodyTooLarge
	}
	got := CIDOf(body)
	if !equalBytes(got, expectedCID) {
		return ErrCIDMismatch
	}
	if _, err := s.Put(body); err != nil {
		return err
	}
	return nil
}

func equalBytes(a, b []byte) bool {
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

// DefaultRoot returns ~/.agentfm/comments — the convention all
// agentfm binaries follow.
func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("comments: user home dir: %w", err)
	}
	return strings.TrimRight(home, string(os.PathSeparator)) + "/.agentfm/comments", nil
}
