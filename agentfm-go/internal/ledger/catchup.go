package ledger

// CatchUp + HeadFetch protocol (P5-1).
//
// On restart a boss may have missed entries that were gossiped while it
// was offline. CatchUp pulls the gap from the relay via the existing
// LedgerFetchProtocol, bounds the walk against the relay's signed head
// (HeadFetchProtocol), and routes every fetched entry through
// local.AcceptEntry (which signature-verifies and chain-extends the
// inbox). Entries whose idx exceeds the relay's signed head.TreeSize are
// rejected as suspect to prevent a malicious relay from serving forged
// entries beyond what it has committed.
//
// HeadFetchProtocol wire format (simple, single-round-trip):
//
//	REQ : zero bytes (the server sends as soon as the stream is opened)
//	RESP: <len u32 BE> <proto bytes>
//
// The server serialises its current LogHead (pb.LogHead proto), prefixes
// a 4-byte big-endian length, and closes the write half. The client
// reads the length then reads that many bytes and unmarshals.

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"agentfm/internal/obs"
	pb "agentfm/internal/ledger/pb"

	libnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

// headFetchProtocol is the P5-1 libp2p protocol identifier for the
// single-round-trip head exchange. Kept package-private — callers
// outside this package use FetchRemoteHead / the handler registration
// path. If we later promote it to a top-level constant we can move it
// to internal/network/constants.go.
const headFetchProtocol = "/agentfm/head-fetch/1.0.0"

// headFetchTimeout caps a single head-fetch exchange.
const headFetchTimeout = 15 * time.Second

// maxHeadBytes is a sanity cap on the LogHead proto size. A LogHead
// with many witness sigs might be a few hundred bytes; 64 KiB is a
// generous ceiling that prevents a rogue server from allocating large
// slabs in the client.
const maxHeadBytes = 64 * 1024

// startHeadFetchHandler registers the HeadFetchProtocol handler on h.
// When a remote peer opens a stream the handler immediately marshals
// and sends the current signed LogHead, then closes the write-half.
// If no head is available yet (fresh ledger), it sends a zero-length
// response (len=0, no bytes) so the client can distinguish "not
// available" from a transport error.
func (l *ledgerImpl) startHeadFetchHandler(h host.Host) {
	h.SetStreamHandler(headFetchProtocol, l.handleHeadFetch)
}

func (l *ledgerImpl) stopHeadFetchHandler(h host.Host) {
	h.RemoveStreamHandler(headFetchProtocol)
}

func (l *ledgerImpl) handleHeadFetch(s libnet.Stream) {
	defer func() { _ = s.Close() }()
	if err := s.SetDeadline(time.Now().Add(headFetchTimeout)); err != nil {
		slog.Debug("head-fetch: set deadline", slog.Any(obs.FieldErr, err))
		return
	}

	l.mu.Lock()
	head := l.lastHead
	l.mu.Unlock()

	var payload []byte
	if head != nil {
		var err error
		payload, err = proto.Marshal(head)
		if err != nil {
			slog.Debug("head-fetch: marshal head", slog.Any(obs.FieldErr, err))
			return
		}
	}

	// Write <len u32 BE> <payload bytes>
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(payload)))
	if _, err := s.Write(lenBuf[:]); err != nil {
		slog.Debug("head-fetch: write len", slog.Any(obs.FieldErr, err))
		return
	}
	if len(payload) > 0 {
		if _, err := s.Write(payload); err != nil {
			slog.Debug("head-fetch: write payload", slog.Any(obs.FieldErr, err))
		}
	}
}

// FetchRemoteHead opens a HeadFetchProtocol stream to remote and reads
// back its current signed LogHead. Returns (nil, nil) if the remote
// reports no head yet (fresh ledger). Returns an error on transport
// failure or if the payload exceeds maxHeadBytes.
func FetchRemoteHead(ctx context.Context, h host.Host, remote peer.ID) (*pb.LogHead, error) {
	s, err := h.NewStream(ctx, remote, headFetchProtocol)
	if err != nil {
		return nil, fmt.Errorf("head-fetch: open stream: %w", err)
	}
	defer func() { _ = s.Close() }()

	if err := s.SetDeadline(time.Now().Add(headFetchTimeout)); err != nil {
		return nil, fmt.Errorf("head-fetch: set deadline: %w", err)
	}

	// Close write-half immediately — the server sends without waiting
	// for any request bytes.
	if err := s.CloseWrite(); err != nil {
		return nil, fmt.Errorf("head-fetch: close-write: %w", err)
	}

	var lenBuf [4]byte
	if _, err := io.ReadFull(s, lenBuf[:]); err != nil {
		return nil, fmt.Errorf("head-fetch: read length: %w", err)
	}
	n := binary.BigEndian.Uint32(lenBuf[:])
	if n == 0 {
		return nil, nil // remote has no head yet
	}
	if n > maxHeadBytes {
		return nil, fmt.Errorf("head-fetch: response too large (%d > %d)", n, maxHeadBytes)
	}

	buf := make([]byte, n)
	if _, err := io.ReadFull(s, buf); err != nil {
		return nil, fmt.Errorf("head-fetch: read payload: %w", err)
	}

	var head pb.LogHead
	if err := proto.Unmarshal(buf, &head); err != nil {
		return nil, fmt.Errorf("head-fetch: unmarshal LogHead: %w", err)
	}
	return &head, nil
}

// VerifyHeadSignature returns true iff head.Signature is a valid
// Ed25519 signature by the key embedded in head.PeerId over the
// canonical head bytes. Uses the same verifyHeadSig path as the alert
// verification so the two remain in sync.
func VerifyHeadSignature(head *pb.LogHead) bool {
	if head == nil {
		return false
	}
	pid, err := peer.IDFromBytes(head.PeerId)
	if err != nil {
		return false
	}
	pub, err := pid.ExtractPublicKey()
	if err != nil {
		return false
	}
	return verifyHeadSig(pub, head) == nil
}

// CatchUp pulls all entries from relayPID that local does not yet have,
// then routes them through local.AcceptEntry (which signature-verifies
// and chain-extends via the inbox). Entries past the relay's signed
// head are rejected as suspect.
//
// On success returns nil. On any unrecoverable error (relay unreachable,
// head signature invalid, fetch protocol error) returns the underlying
// error so the caller can log and continue without catch-up.
func CatchUp(ctx context.Context, local Ledger, h host.Host, relayPID peer.ID) error {
	lastIdx, err := local.LastInboxIdx(ctx)
	if err != nil {
		return fmt.Errorf("last inbox idx: %w", err)
	}

	relayHead, err := FetchRemoteHead(ctx, h, relayPID)
	if err != nil {
		return fmt.Errorf("fetch relay head: %w", err)
	}
	if relayHead == nil {
		// Relay has an empty ledger — nothing to pull.
		slog.Debug("catchup: relay has no head; nothing to pull")
		return nil
	}
	if !VerifyHeadSignature(relayHead) {
		return errors.New("relay head signature invalid")
	}
	if relayHead.TreeSize == 0 {
		return nil // nothing to pull
	}

	const pageSize = uint64(1000)
	for {
		entries, err := FetchClient(ctx, h, relayPID, lastIdx+1, pageSize)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			break
		}
		for _, e := range entries {
			if e.Idx > relayHead.TreeSize {
				return fmt.Errorf("relay served entry idx=%d past head size=%d", e.Idx, relayHead.TreeSize)
			}
			if err := local.AcceptEntry(ctx, e.Payload); err != nil {
				slog.Debug("catchup: drop entry",
					slog.Uint64("idx", e.Idx),
					slog.Any(obs.FieldErr, err))
				// Continue — a bad entry should not abort the whole
				// catch-up; the inbox deduplicates and we don't want
				// a single malformed/self-authored entry to stall
				// ingestion of valid subsequent entries.
			}
			lastIdx = e.Idx
		}
		if lastIdx >= relayHead.TreeSize {
			break
		}
	}
	slog.Debug("catchup: complete",
		slog.Uint64("up_to_idx", lastIdx),
		slog.Uint64("relay_head_size", relayHead.TreeSize))
	return nil
}
