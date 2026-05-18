package boss

import (
	"context"
	"io"
	"net/http"

	"agentfm/internal/network"
	"agentfm/internal/types"

	"github.com/libp2p/go-libp2p/core/host"
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

// HostForTest returns the boss's underlying libp2p host identity so test
// helpers can write ledger entries attributed to the boss's own peer ID.
// Must only be used in test code.
func (b *Boss) HostForTest() host.Host {
	return b.node.Host
}

// HandleGetWorkersForTest exposes handleGetWorkers to test packages. Used
// to exercise the ?include_offline query parameter without starting the full
// API server (auth, bind, CORS, etc.).
func (b *Boss) HandleGetWorkersForTest(w http.ResponseWriter, r *http.Request) {
	b.handleGetWorkers(w, r)
}

// SetMenuPickerForTest installs a deterministic menu-choice function that
// replaces the pterm interactive-select in showPeerMenu. Nil resets to the
// real pterm picker. Only for use in tests.
func (b *Boss) SetMenuPickerForTest(f func([]string) (string, error)) {
	b.menuPickerForTest = f
}

// SetPeerViewHookForTest installs a hook that replaces the real
// viewPeerHistory call inside executeFlow. Nil resets to the real
// viewPeerHistory. Only for use in tests.
func (b *Boss) SetPeerViewHookForTest(f func(ctx context.Context, peerIDStr string)) {
	b.peerViewHookForTest = f
}

// RenderRadarForTest writes the radar output (ONLINE + OFFLINE sections) to w
// without starting the interactive keyboard listener. Used by unit tests to
// assert section headers and row content.
func (b *Boss) RenderRadarForTest(w io.Writer) {
	b.renderRadar(context.Background(), w)
}
