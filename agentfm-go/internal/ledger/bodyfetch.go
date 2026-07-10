package ledger

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"agentfm/internal/ledger/comments"
	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/ledger/store"
	"agentfm/internal/obs"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	"google.golang.org/protobuf/proto"
)

const (
	bodyFetchQueueCap = 64
	bodyFetchTimeout  = 30 * time.Second

	bodyBackfillInitialDelay = time.Minute
	bodyBackfillInterval     = 10 * time.Minute
	bodyBackfillMaxPerSweep  = 256
)

// errStopBackfillScan is the sentinel the backfill scan callback returns
// to stop iterating once the per-sweep cap is reached.
var errStopBackfillScan = errors.New("stop backfill scan")

type bodyFetchJob struct {
	author peer.ID
	cid    []byte
}

type bodyFetcher struct {
	host         host.Host
	store        *comments.Store
	jobs         chan bodyFetchJob
	done         chan struct{}
	backfillDone chan struct{}
}

func newBodyFetcher(h host.Host, s *comments.Store) *bodyFetcher {
	return &bodyFetcher{
		host:         h,
		store:        s,
		jobs:         make(chan bodyFetchJob, bodyFetchQueueCap),
		done:         make(chan struct{}),
		backfillDone: make(chan struct{}),
	}
}

func (f *bodyFetcher) enqueue(entry *pb.SignedEntry) {
	c := commentOf(entry)
	if c == nil || len(c.TextCid) == 0 || f.store.Has(c.TextCid) {
		return
	}
	select {
	case f.jobs <- bodyFetchJob{author: peer.ID(c.RaterPeerId), cid: c.TextCid}:
	default:
		slog.Debug("ledger: comment body fetch queue full; deferring to backfill sweep",
			slog.String("author", peer.ID(c.RaterPeerId).String()))
	}
}

func (f *bodyFetcher) run(ctx context.Context) {
	defer close(f.done)
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-f.jobs:
			f.fetchOne(ctx, job)
		}
	}
}

// runBackfill periodically re-scans the inbox for Comment entries whose
// bodies are still missing from the store — bodies dropped by a full
// live queue, or whose author was unreachable at gossip time — and
// re-fetches them. The first sweep runs shortly after startup so a
// restarted node heals gaps without waiting a full interval.
func (f *bodyFetcher) runBackfill(ctx context.Context, s *store.Store) {
	defer close(f.backfillDone)
	timer := time.NewTimer(bodyBackfillInitialDelay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		f.backfillOnce(ctx, s)
		timer.Reset(bodyBackfillInterval)
	}
}

// backfillOnce collects up to bodyBackfillMaxPerSweep missing (author,
// cid) pairs from the inbox, then fetches them serially. Collect and
// fetch are separate phases so no network I/O happens inside the SQLite
// row iteration.
func (f *bodyFetcher) backfillOnce(ctx context.Context, s *store.Store) {
	missing := make([]bodyFetchJob, 0, 16)
	truncated := false
	err := s.IterateAllInboxEntries(ctx, func(e *store.InboxEntry) error {
		var signed pb.SignedEntry
		if uerr := proto.Unmarshal(e.Payload, &signed); uerr != nil {
			return nil
		}
		c := commentOf(&signed)
		if c == nil || len(c.TextCid) == 0 || f.store.Has(c.TextCid) {
			return nil
		}
		if len(missing) >= bodyBackfillMaxPerSweep {
			truncated = true
			return errStopBackfillScan
		}
		missing = append(missing, bodyFetchJob{author: peer.ID(c.RaterPeerId), cid: c.TextCid})
		return nil
	})
	if err != nil && !errors.Is(err, errStopBackfillScan) {
		slog.Debug("ledger: comment body backfill scan failed",
			slog.Any(obs.FieldErr, err))
		return
	}
	if truncated {
		slog.Debug("ledger: comment body backfill hit per-sweep cap; remainder picked up next sweep",
			slog.Int("cap", bodyBackfillMaxPerSweep))
	}
	for _, job := range missing {
		if ctx.Err() != nil {
			return
		}
		f.fetchOne(ctx, job)
	}
}

func (f *bodyFetcher) fetchOne(ctx context.Context, job bodyFetchJob) {
	if f.store.Has(job.cid) {
		return
	}
	fetchCtx, cancel := context.WithTimeout(ctx, bodyFetchTimeout)
	body, err := comments.Fetch(fetchCtx, f.host, job.author, job.cid)
	cancel()
	if err != nil {
		slog.Debug("ledger: comment body fetch failed",
			slog.String("author", job.author.String()),
			slog.Any(obs.FieldErr, err))
		return
	}
	if _, err := f.store.Put(body); err != nil {
		slog.Warn("ledger: comment body persist failed",
			slog.Any(obs.FieldErr, err))
	}
}

func commentOf(entry *pb.SignedEntry) *pb.Comment {
	if body, ok := entry.GetBody().(*pb.SignedEntry_Comment); ok {
		return body.Comment
	}
	return nil
}
