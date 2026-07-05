package boss

import (
	"time"

	"agentfm/internal/network"

	"github.com/libp2p/go-libp2p/core/peer"
)

// artifactExpectTTL bounds how long a dispatched task may deliver its
// artifact zip. Tasks stream stdout for at most TaskExecutionTimeout and
// the artifact transfer itself is bounded by ArtifactStreamTimeout, so a
// legitimate delivery always lands within the sum.
const artifactExpectTTL = network.TaskExecutionTimeout + network.ArtifactStreamTimeout

type artifactExpectation struct {
	worker  peer.ID
	expires time.Time
}

// expectArtifact records that taskID was dispatched to worker, authorizing
// exactly one artifact delivery for that pair until the TTL elapses. Call
// before sending the task payload so the worker can never race the
// registration.
func (b *Boss) expectArtifact(taskID string, worker peer.ID) {
	b.artifactMu.Lock()
	defer b.artifactMu.Unlock()
	if b.artifactExpect == nil {
		b.artifactExpect = make(map[string]artifactExpectation)
	}
	for id, e := range b.artifactExpect {
		if time.Now().After(e.expires) {
			delete(b.artifactExpect, id)
		}
	}
	b.artifactExpect[taskID] = artifactExpectation{
		worker:  worker,
		expires: time.Now().Add(artifactExpectTTL),
	}
}

// authorizeArtifact reports whether an inbound artifact stream for taskID
// from peer `from` corresponds to a live dispatch. A successful match
// consumes the expectation so a second stream cannot overwrite the zip.
func (b *Boss) authorizeArtifact(taskID string, from peer.ID) bool {
	b.artifactMu.Lock()
	defer b.artifactMu.Unlock()
	e, ok := b.artifactExpect[taskID]
	if !ok || e.worker != from || time.Now().After(e.expires) {
		return false
	}
	delete(b.artifactExpect, taskID)
	return true
}
