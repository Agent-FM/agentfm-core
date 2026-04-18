package worker

import (
	"testing"

	"agentfm/internal/network"
	"agentfm/test/testutil"

	"github.com/libp2p/go-libp2p/core/host"
)

// newTestWorker builds a Worker wired to a fresh libp2p host without
// calling Start (so no Podman image is built). Kept in-package because it
// constructs the unexported Worker struct directly — the whole reason
// Go's convention for unit tests is "same package as the code".
func newTestWorker(t *testing.T, cfg Config) (*Worker, host.Host) {
	t.Helper()
	h := testutil.NewHost(t)
	w := &Worker{
		node:   &network.MeshNode{Host: h},
		config: cfg,
	}
	return w, h
}
