package boss

import (
	"net/http"

	"agentfm/internal/network"
	"agentfm/internal/types"
)

// NewForTest is a thin alias for New, named explicitly so test packages
// don't accidentally import the production constructor (which would carry
// surprising defaults if it ever grows).
func NewForTest(node *network.MeshNode) *Boss {
	return New(node)
}

// SeedWorker preloads the activeWorkers map with a profile. Production
// code populates this from telemetry (see listenTelemetry); tests use
// this hook to skip the GossipSub roundtrip and exercise dispatch
// directly. Take the lock so calls from a test goroutine remain race-safe
// under -race.
func (b *Boss) SeedWorker(p types.WorkerProfile) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.activeWorkers[p.PeerID] = p
}

// ServeHTTPExecute exposes handleExecuteTask to test packages without
// publishing the full handler set. Keeps the test surface narrow so
// future internal renames stay free.
func (b *Boss) ServeHTTPExecute(w http.ResponseWriter, r *http.Request) {
	b.handleExecuteTask(w, r)
}
