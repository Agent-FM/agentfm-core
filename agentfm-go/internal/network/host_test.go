package network

import (
	"context"
	"testing"
	"time"

	"agentfm/test/testutil"
)

// kad-dht v0.43 dropped the context argument from dht.New, so the DHT now
// runs until Close. initRoutingAndPubSub ties Close to the node's context to
// keep the old lifetime: cancelling the context must stop the DHT, or a
// cancelled node leaves its routing goroutines running.
func TestInitRoutingAndPubSub_CancelStopsDHT(t *testing.T) {
	for _, isWorker := range []bool{true, false} {
		ctx, cancel := context.WithCancel(context.Background())
		h := testutil.NewHost(t)

		_, kademliaDHT, err := initRoutingAndPubSub(ctx, h, isWorker)
		if err != nil {
			cancel()
			t.Fatalf("isWorker=%v: initRoutingAndPubSub: %v", isWorker, err)
		}
		if err := kademliaDHT.Context().Err(); err != nil {
			cancel()
			t.Fatalf("isWorker=%v: DHT stopped before its context was cancelled: %v", isWorker, err)
		}

		cancel()

		select {
		case <-kademliaDHT.Context().Done():
		case <-time.After(5 * time.Second):
			t.Fatalf("isWorker=%v: DHT still running 5s after its context was cancelled", isWorker)
		}
	}
}
