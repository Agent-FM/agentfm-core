package boss

import (
	"testing"

	"agentfm/internal/network"
	"agentfm/test/testutil"
)

// newTestBoss returns a Boss wired to a fresh libp2p host. Kept in-package
// because it touches the unexported Boss struct directly; all reusable
// non-Boss-specific helpers (host setup, connect, timeouts) live in
// test/testutil instead.
func newTestBoss(t *testing.T) *Boss {
	t.Helper()
	h := testutil.NewHost(t)
	return New(&network.MeshNode{Host: h})
}
