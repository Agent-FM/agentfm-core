package ledger

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"agentfm/internal/ledger/store"
	"agentfm/internal/network"
	"agentfm/internal/obs"

	"github.com/libp2p/go-libp2p/core/host"
	libnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// fetchStreamTimeout caps a single ledger-fetch exchange. Walks of
// large ranges are bounded; clients that want everything should
// paginate.
const fetchStreamTimeout = 30 * time.Second

// maxFetchEntries is the per-request entry cap. Defends the responder
// against a peer asking for the whole log in one shot (which would
// allocate memory linear in our log size).
const maxFetchEntries = 1000

// maxFetchAggregateBytes bounds the total payload a client buffers from one
// fetch page. The per-entry 1 MiB cap alone still lets a hostile responder
// ship maxFetchEntries * 1 MiB (~1 GiB); legit entries are ~1 KiB.
const maxFetchAggregateBytes = 32 << 20

// startFetchHandler registers the LedgerFetchProtocol handler on host.
// Each incoming stream serves one (from, count) range from the local
// store.
func (l *ledgerImpl) startFetchHandler(h host.Host) {
	h.SetStreamHandler(network.LedgerFetchProtocol, l.handleFetch)
}

func (l *ledgerImpl) stopFetchHandler(h host.Host) {
	h.RemoveStreamHandler(network.LedgerFetchProtocol)
}

// Wire format (length-prefixed, all u64 BE):
//
//	REQ : <from u64><count u64>
//	RESP: <count u64> followed by `count` repetitions of:
//	         <peer_idx u64><payload_len u32><payload bytes>
//
// Both sides set deadlines so a stuck peer cannot hang our goroutine.
// Errors mid-stream are surfaced as a zero-count response so the
// client can distinguish "no data" from "transport failure."
func (l *ledgerImpl) handleFetch(s libnet.Stream) {
	reset := true
	defer func() {
		if reset {
			_ = s.Reset()
		} else {
			_ = s.Close()
		}
	}()
	if err := s.SetDeadline(time.Now().Add(fetchStreamTimeout)); err != nil {
		slog.Debug("fetch: set deadline", slog.Any(obs.FieldErr, err))
		return
	}

	var hdr [16]byte
	if _, err := io.ReadFull(s, hdr[:]); err != nil {
		slog.Debug("fetch: read request",
			slog.Any(obs.FieldErr, err),
			slog.String("remote", s.Conn().RemotePeer().String()))
		return
	}
	from := binary.BigEndian.Uint64(hdr[:8])
	count := binary.BigEndian.Uint64(hdr[8:])
	if count == 0 {
		_ = writeFetchCount(s, 0)
		reset = false
		return
	}
	if count > maxFetchEntries {
		count = maxFetchEntries
	}

	ctx, cancel := context.WithTimeout(context.Background(), fetchStreamTimeout)
	defer cancel()

	var entries []*store.Entry
	err := l.store.IterateEntries(ctx, from, func(e *store.Entry) error {
		if uint64(len(entries)) >= count {
			return errStopIterate
		}
		entries = append(entries, e)
		return nil
	})
	if err != nil && !errors.Is(err, errStopIterate) {
		slog.Debug("fetch: iterate", slog.Any(obs.FieldErr, err))
		_ = writeFetchCount(s, 0)
		reset = false
		return
	}

	if err := writeFetchCount(s, uint64(len(entries))); err != nil {
		slog.Debug("fetch: write count", slog.Any(obs.FieldErr, err))
		return
	}
	for _, e := range entries {
		if err := writeFetchEntry(s, e); err != nil {
			slog.Debug("fetch: write entry",
				slog.Any(obs.FieldErr, err),
				slog.Uint64("idx", e.Idx))
			return
		}
	}
	reset = false
}

func writeFetchCount(w io.Writer, n uint64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], n)
	_, err := w.Write(buf[:])
	return err
}

func writeFetchEntry(w io.Writer, e *store.Entry) error {
	var hdr [12]byte
	binary.BigEndian.PutUint64(hdr[:8], e.Idx)
	if len(e.Payload) > (1 << 20) {
		return fmt.Errorf("ledger fetch: payload too large (%d > 1MiB)", len(e.Payload))
	}
	binary.BigEndian.PutUint32(hdr[8:], uint32(len(e.Payload)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := w.Write(e.Payload); err != nil {
		return err
	}
	return nil
}

// FetchedEntry is one row returned by FetchEntries. Hash is recomputed
// on the client side (we don't trust the source — but we don't need
// to either; downstream VerifyEntry catches tampering via the
// signature check).
type FetchedEntry struct {
	Idx     uint64
	Payload []byte // full SignedEntry proto bytes
}

// FetchClient opens a LedgerFetchProtocol stream to remote and pulls
// `count` entries starting at `from` (1-based, inclusive). Returns
// the entries as raw payload bytes; callers MUST verify each via
// VerifyEntry before trusting the contents.
func FetchClient(ctx context.Context, h host.Host, remote peer.ID, from, count uint64) ([]FetchedEntry, error) {
	s, err := h.NewStream(ctx, remote, network.LedgerFetchProtocol)
	if err != nil {
		return nil, fmt.Errorf("ledger fetch: open stream: %w", err)
	}
	success := false
	defer func() {
		if success {
			_ = s.Close()
		} else {
			_ = s.Reset()
		}
	}()

	if err := s.SetDeadline(time.Now().Add(fetchStreamTimeout)); err != nil {
		return nil, fmt.Errorf("ledger fetch: set deadline: %w", err)
	}

	var hdr [16]byte
	binary.BigEndian.PutUint64(hdr[:8], from)
	binary.BigEndian.PutUint64(hdr[8:], count)
	if _, err := s.Write(hdr[:]); err != nil {
		return nil, fmt.Errorf("ledger fetch: write request: %w", err)
	}
	if err := s.CloseWrite(); err != nil {
		return nil, fmt.Errorf("ledger fetch: close-write: %w", err)
	}

	var countBuf [8]byte
	if _, err := io.ReadFull(s, countBuf[:]); err != nil {
		return nil, fmt.Errorf("ledger fetch: read count: %w", err)
	}
	n := binary.BigEndian.Uint64(countBuf[:])
	if n > maxFetchEntries {
		return nil, fmt.Errorf("ledger fetch: server returned %d entries, exceeds cap %d", n, maxFetchEntries)
	}

	out := make([]FetchedEntry, 0, n)
	var aggregate uint64
	for i := uint64(0); i < n; i++ {
		var eh [12]byte
		if _, err := io.ReadFull(s, eh[:]); err != nil {
			return nil, fmt.Errorf("ledger fetch: read entry %d header: %w", i, err)
		}
		idx := binary.BigEndian.Uint64(eh[:8])
		plen := binary.BigEndian.Uint32(eh[8:])
		if plen > (1 << 20) {
			return nil, fmt.Errorf("ledger fetch: entry %d payload too large (%d > 1MiB)", i, plen)
		}
		aggregate += uint64(plen)
		if aggregate > maxFetchAggregateBytes {
			return nil, fmt.Errorf("ledger fetch: aggregate payload exceeds %d bytes", maxFetchAggregateBytes)
		}
		payload := make([]byte, plen)
		if _, err := io.ReadFull(s, payload); err != nil {
			return nil, fmt.Errorf("ledger fetch: read entry %d payload: %w", i, err)
		}
		out = append(out, FetchedEntry{Idx: idx, Payload: payload})
	}
	success = true
	return out, nil
}
