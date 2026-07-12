package comments

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"agentfm/internal/network"
	"agentfm/internal/obs"

	"github.com/libp2p/go-libp2p/core/host"
	libnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// fetchTimeout caps a single comment-fetch round-trip. 30s per the
// plan — bodies are at most 10 KiB so any longer means the peer is
// stalled and we should give up.
const fetchTimeout = 30 * time.Second

// Wire format (P4-1):
//
//	REQ : <cid_len u32><cid bytes>
//	RESP: <body_len u32><body bytes>   on success
//	      <0xffffffff><error_len u32><error message>   on not-found / error
//
// 0xffffffff as the body_len sentinel doubles as a "not found"
// marker without needing a separate response code.

const notFoundSentinel uint32 = 0xffffffff

// Server registers the CommentFetchProtocol handler on h.
type Server struct {
	host  host.Host
	store *Store
}

// NewServer wires up the handler.
func NewServer(h host.Host, s *Store) *Server {
	return &Server{host: h, store: s}
}

// Start registers the handler. Safe to call multiple times.
func (srv *Server) Start() {
	srv.host.SetStreamHandler(network.CommentFetchProtocol, srv.handle)
}

// Stop unregisters the handler.
func (srv *Server) Stop() {
	srv.host.RemoveStreamHandler(network.CommentFetchProtocol)
}

func (srv *Server) handle(s libnet.Stream) {
	defer func() { _ = s.Close() }()
	if err := s.SetDeadline(time.Now().Add(fetchTimeout)); err != nil {
		slog.Debug("comments fetch: set deadline", slog.Any(obs.FieldErr, err))
		return
	}

	var hdr [4]byte
	if _, err := io.ReadFull(s, hdr[:]); err != nil {
		slog.Debug("comments fetch: read cid_len",
			slog.Any(obs.FieldErr, err),
			slog.String("remote", s.Conn().RemotePeer().String()))
		return
	}
	cidLen := binary.BigEndian.Uint32(hdr[:])
	if cidLen > 256 {
		// CIDs are 34 bytes today; allow some room for future
		// multihash variants but reject obviously-malicious payloads.
		writeNotFound(s, "cid too large")
		return
	}
	cid := make([]byte, cidLen)
	if _, err := io.ReadFull(s, cid); err != nil {
		slog.Debug("comments fetch: read cid", slog.Any(obs.FieldErr, err))
		return
	}
	body, err := srv.store.Get(cid)
	if err != nil {
		// Log the real error (which may embed local filesystem paths)
		// locally; send only a fixed string to the remote peer.
		slog.Debug("comments fetch: get body", slog.Any(obs.FieldErr, err))
		writeNotFound(s, "not found")
		return
	}
	writeBody(s, body)
}

func writeBody(w io.Writer, body []byte) {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(body)))
	if _, err := w.Write(hdr[:]); err != nil {
		return
	}
	_, _ = w.Write(body)
}

func writeNotFound(w io.Writer, reason string) {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], notFoundSentinel)
	if _, err := w.Write(hdr[:]); err != nil {
		return
	}
	bs := []byte(reason)
	binary.BigEndian.PutUint32(hdr[:], uint32(len(bs)))
	_, _ = w.Write(hdr[:])
	_, _ = w.Write(bs)
}

// Fetch opens a CommentFetchProtocol stream to remote and pulls the
// body for cid. Returns (body, nil) on success; ErrNotFound when
// the remote doesn't have the body; or a transport / decode error.
//
// The stream deadline is the sooner of fetchTimeout and ctx's own
// deadline, and ctx cancellation resets the stream immediately — a
// stalled remote can never pin the caller past its context bound.
//
// Validates the returned body against cid before returning —
// callers can trust the bytes match what they asked for without
// re-hashing.
func Fetch(ctx context.Context, h host.Host, remote peer.ID, cid []byte) ([]byte, error) {
	s, err := h.NewStream(ctx, remote, network.CommentFetchProtocol)
	if err != nil {
		return nil, fmt.Errorf("comments fetch: open stream: %w", err)
	}
	defer func() { _ = s.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = s.Reset() })
	defer stop()
	deadline := time.Now().Add(fetchTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := s.SetDeadline(deadline); err != nil {
		return nil, fmt.Errorf("comments fetch: set deadline: %w", err)
	}

	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(cid)))
	if _, err := s.Write(hdr[:]); err != nil {
		return nil, fmt.Errorf("comments fetch: write cid_len: %w", err)
	}
	if _, err := s.Write(cid); err != nil {
		return nil, fmt.Errorf("comments fetch: write cid: %w", err)
	}
	if err := s.CloseWrite(); err != nil {
		return nil, fmt.Errorf("comments fetch: close-write: %w", err)
	}

	if _, err := io.ReadFull(s, hdr[:]); err != nil {
		return nil, fmt.Errorf("comments fetch: read body_len: %w", err)
	}
	bodyLen := binary.BigEndian.Uint32(hdr[:])
	if bodyLen == notFoundSentinel {
		// Read + discard the error message; surface ErrNotFound.
		if _, err := io.ReadFull(s, hdr[:]); err == nil {
			errLen := binary.BigEndian.Uint32(hdr[:])
			if errLen > 0 && errLen < 1024 {
				_, _ = io.CopyN(io.Discard, s, int64(errLen))
			}
		}
		return nil, ErrNotFound
	}
	if int64(bodyLen) > int64(MaxBodyBytes) {
		return nil, fmt.Errorf("comments fetch: body too large: %d > %d", bodyLen, MaxBodyBytes)
	}
	body := make([]byte, bodyLen)
	if _, err := io.ReadFull(s, body); err != nil {
		return nil, fmt.Errorf("comments fetch: read body: %w", err)
	}
	// Validate the body hashes back to the requested cid.
	got := CIDOf(body)
	if !equalBytes(got, cid) {
		return nil, errors.New("comments fetch: returned body does not match requested cid")
	}
	return body, nil
}
