package ledger

import (
	"context"
	"log/slog"
	"time"

	"agentfm/internal/ledger/comments"
	pb "agentfm/internal/ledger/pb"
	"agentfm/internal/obs"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	bodyFetchQueueCap = 64
	bodyFetchTimeout  = 30 * time.Second
)

type bodyFetchJob struct {
	author peer.ID
	cid    []byte
}

type bodyFetcher struct {
	host  host.Host
	store *comments.Store
	jobs  chan bodyFetchJob
	done  chan struct{}
}

func newBodyFetcher(h host.Host, s *comments.Store) *bodyFetcher {
	return &bodyFetcher{
		host:  h,
		store: s,
		jobs:  make(chan bodyFetchJob, bodyFetchQueueCap),
		done:  make(chan struct{}),
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
		slog.Debug("ledger: comment body fetch queue full; dropping job",
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
			if f.store.Has(job.cid) {
				continue
			}
			fetchCtx, cancel := context.WithTimeout(ctx, bodyFetchTimeout)
			body, err := comments.Fetch(fetchCtx, f.host, job.author, job.cid)
			cancel()
			if err != nil {
				slog.Debug("ledger: comment body fetch failed",
					slog.String("author", job.author.String()),
					slog.Any(obs.FieldErr, err))
				continue
			}
			if _, err := f.store.Put(body); err != nil {
				slog.Warn("ledger: comment body persist failed",
					slog.Any(obs.FieldErr, err))
			}
		}
	}
}

func commentOf(entry *pb.SignedEntry) *pb.Comment {
	if body, ok := entry.GetBody().(*pb.SignedEntry_Comment); ok {
		return body.Comment
	}
	return nil
}
