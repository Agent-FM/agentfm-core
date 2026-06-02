package ledger

import (
	"context"
	"encoding/binary"
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

const (
	inboxFetchStreamTimeout = 30 * time.Second
	maxInboxFetchEntries    = 1000
	maxInboxPayloadBytes    = 1 << 20
)

// startInboxFetchHandler registers the InboxFetchProtocol handler.
// Each incoming stream serves one (sinceRowid, count) page from the
// local inbox_entries table.
func (l *ledgerImpl) startInboxFetchHandler(h host.Host) {
	h.SetStreamHandler(network.InboxFetchProtocol, l.handleInboxFetch)
}

func (l *ledgerImpl) stopInboxFetchHandler(h host.Host) {
	h.RemoveStreamHandler(network.InboxFetchProtocol)
}

// Wire format (length-prefixed, big-endian):
//
//	REQ : <sinceRowid u64><count u64>
//	RESP: <count u64> followed by `count` repetitions of:
//	         <rowid u64><payloadLen u32><payload bytes>
//
// Both sides set deadlines. Mid-stream errors collapse to a zero-count
// response so the client can distinguish "no data" from "transport
// failure" (the latter shows up as a stream error).
func (l *ledgerImpl) handleInboxFetch(s libnet.Stream) {
	defer func() { _ = s.Close() }()
	if err := s.SetDeadline(time.Now().Add(inboxFetchStreamTimeout)); err != nil {
		slog.Debug("inbox-fetch: set deadline", slog.Any(obs.FieldErr, err))
		return
	}

	var hdr [16]byte
	if _, err := io.ReadFull(s, hdr[:]); err != nil {
		slog.Debug("inbox-fetch: read request",
			slog.Any(obs.FieldErr, err),
			slog.String("remote", s.Conn().RemotePeer().String()))
		return
	}
	sinceRowid := binary.BigEndian.Uint64(hdr[:8])
	count := binary.BigEndian.Uint64(hdr[8:])
	if count == 0 {
		_ = writeInboxFetchCount(s, 0)
		return
	}
	if count > maxInboxFetchEntries {
		count = maxInboxFetchEntries
	}

	ctx, cancel := context.WithTimeout(context.Background(), inboxFetchStreamTimeout)
	defer cancel()

	type page struct {
		rowid   uint64
		payload []byte
	}
	pages := make([]page, 0, count)
	err := l.store.IterateInboxFrom(ctx, sinceRowid, int(count), func(rowid uint64, e *store.InboxEntry) error {
		pages = append(pages, page{rowid: rowid, payload: e.Payload})
		return nil
	})
	if err != nil {
		slog.Debug("inbox-fetch: iterate", slog.Any(obs.FieldErr, err))
		_ = writeInboxFetchCount(s, 0)
		return
	}

	if err := writeInboxFetchCount(s, uint64(len(pages))); err != nil {
		slog.Debug("inbox-fetch: write count", slog.Any(obs.FieldErr, err))
		return
	}
	for _, p := range pages {
		if err := writeInboxFetchEntry(s, p.rowid, p.payload); err != nil {
			slog.Debug("inbox-fetch: write entry",
				slog.Any(obs.FieldErr, err),
				slog.Uint64("rowid", p.rowid))
			return
		}
	}
}

func writeInboxFetchCount(w io.Writer, n uint64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], n)
	_, err := w.Write(buf[:])
	return err
}

func writeInboxFetchEntry(w io.Writer, rowid uint64, payload []byte) error {
	if len(payload) > maxInboxPayloadBytes {
		return fmt.Errorf("inbox-fetch: payload too large (%d > %d)", len(payload), maxInboxPayloadBytes)
	}
	var hdr [12]byte
	binary.BigEndian.PutUint64(hdr[:8], rowid)
	binary.BigEndian.PutUint32(hdr[8:], uint32(len(payload)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	return nil
}

// FetchedInboxEntry is one row returned by FetchInboxFrom. The caller
// MUST route Payload through Ledger.AcceptEntry — the local node does
// not verify signatures or chain extension here.
type FetchedInboxEntry struct {
	Rowid   uint64
	Payload []byte
}

// FetchInboxFrom opens an InboxFetchProtocol stream to remote and
// returns up to count entries with rowid > sinceRowid. Callers MUST
// pass each returned Payload through Ledger.AcceptEntry to verify the
// signature and chain-extend the inbox.
//
// Pagination: if len(result) == count, more entries may exist past
// the last returned rowid — call again with sinceRowid set to the
// last rowid received. If len(result) < count, the remote has no
// more entries after sinceRowid.
func FetchInboxFrom(ctx context.Context, h host.Host, remote peer.ID, sinceRowid, count uint64) ([]FetchedInboxEntry, error) {
	s, err := h.NewStream(ctx, remote, network.InboxFetchProtocol)
	if err != nil {
		return nil, fmt.Errorf("inbox-fetch: open stream: %w", err)
	}
	defer func() { _ = s.Close() }()

	if err := s.SetDeadline(time.Now().Add(inboxFetchStreamTimeout)); err != nil {
		return nil, fmt.Errorf("inbox-fetch: set deadline: %w", err)
	}

	var hdr [16]byte
	binary.BigEndian.PutUint64(hdr[:8], sinceRowid)
	binary.BigEndian.PutUint64(hdr[8:], count)
	if _, err := s.Write(hdr[:]); err != nil {
		return nil, fmt.Errorf("inbox-fetch: write request: %w", err)
	}
	if err := s.CloseWrite(); err != nil {
		return nil, fmt.Errorf("inbox-fetch: close-write: %w", err)
	}

	var countBuf [8]byte
	if _, err := io.ReadFull(s, countBuf[:]); err != nil {
		return nil, fmt.Errorf("inbox-fetch: read count: %w", err)
	}
	n := binary.BigEndian.Uint64(countBuf[:])
	if n > maxInboxFetchEntries {
		return nil, fmt.Errorf("inbox-fetch: server returned %d entries, exceeds cap %d", n, maxInboxFetchEntries)
	}

	out := make([]FetchedInboxEntry, 0, n)
	for i := uint64(0); i < n; i++ {
		var eh [12]byte
		if _, err := io.ReadFull(s, eh[:]); err != nil {
			return nil, fmt.Errorf("inbox-fetch: read entry %d header: %w", i, err)
		}
		rowid := binary.BigEndian.Uint64(eh[:8])
		plen := binary.BigEndian.Uint32(eh[8:])
		if uint64(plen) > maxInboxPayloadBytes {
			return nil, fmt.Errorf("inbox-fetch: entry %d payload too large (%d > %d)", i, plen, maxInboxPayloadBytes)
		}
		payload := make([]byte, plen)
		if _, err := io.ReadFull(s, payload); err != nil {
			return nil, fmt.Errorf("inbox-fetch: read entry %d payload: %w", i, err)
		}
		out = append(out, FetchedInboxEntry{Rowid: rowid, Payload: payload})
	}
	return out, nil
}
