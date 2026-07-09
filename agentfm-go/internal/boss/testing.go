package boss

import (
	"context"
	"io"
	"net/http"
	"sync"

	"agentfm/internal/ledger"
	"agentfm/internal/ledger/store"
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

// SetLedger injects a ledger into the boss. Used by integration tests that
// build a Boss via NewForTest and then supply a real ledger to exercise the
// equivocator check and ListKnownPeers paths.
func (b *Boss) SetLedger(l ledger.Ledger) {
	b.ledger = l
}

// SetReadStoreForTest injects a store handle that ListKnownPeers uses to
// enumerate DistinctSubjects from ledger entries. In production this is
// wired via Options.ReadStore; in integration tests inject via this helper.
func (b *Boss) SetReadStoreForTest(s *store.Store) {
	b.readStore = s
}

// mockReputationEngine is a minimal engine for test use. Scores are injected
// via SetReputationScoreForTest; Score() returns the injected value or 0.
type mockReputationEngine struct {
	mu     sync.RWMutex
	scores map[string]float64
}

func newMockReputationEngine() *mockReputationEngine {
	return &mockReputationEngine{scores: make(map[string]float64)}
}

func (m *mockReputationEngine) Score(peerID string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.scores[peerID]
}

// Recompute satisfies reputationEngineIface. The mock never reads a store.
func (m *mockReputationEngine) Recompute(_ context.Context, _ *store.Store) (float64, error) {
	return 0, nil
}

func (m *mockReputationEngine) set(peerID string, score float64) {
	m.mu.Lock()
	m.scores[peerID] = score
	m.mu.Unlock()
}

// handleAboutForTest exposes handleAbout without the full auth/CORS
// middleware. Only for use in tests.
func (b *Boss) handleAboutForTest(w http.ResponseWriter, r *http.Request) {
	b.handleAbout(w, r)
}

// handleEventsForTest exposes handleEvents without the full auth/CORS
// middleware. Only for use in tests.
func (b *Boss) handleEventsForTest(w http.ResponseWriter, r *http.Request) {
	b.handleEvents(w, r)
}

// handlePeersForTest exposes the umbrella /v1/peers/* handler without
// the full middleware stack. Only for use in tests.
func (b *Boss) handlePeersForTest(w http.ResponseWriter, r *http.Request) {
	b.handlePeers(w, r)
}

// EventBus returns the boss's event bus so telemetry callbacks and
// ledger callbacks can publish events into the SSE stream.
func (b *Boss) EventBus() *EventBus {
	return b.eventBus
}

// SetReputationScoreForTest injects a score for a given peer ID into a
// mock reputation engine attached to the boss. If no mock engine is
// present yet, one is created and installed. Only for use in tests.
func (b *Boss) SetReputationScoreForTest(pid string, score float64) {
	if mock, ok := b.reputationEngine.(*mockReputationEngine); ok {
		mock.set(pid, score)
		return
	}
	// Install a fresh mock engine and set the score.
	eng := newMockReputationEngine()
	eng.set(pid, score)
	b.reputationEngine = eng
}
