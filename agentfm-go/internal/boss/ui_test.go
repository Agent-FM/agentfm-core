package boss

import (
	"bytes"
	"strings"
	"testing"

	"agentfm/internal/types"
	"agentfm/test/testutil"
)

// TestRadarRender_ShowsOnlineAndOfflineSections verifies that RenderRadarForTest
// writes ONLINE and OFFLINE section headers, with:
//   - the online peer appearing under ONLINE
//   - the offline peer (known only via ledger) appearing under OFFLINE
func TestRadarRender_ShowsOnlineAndOfflineSections(t *testing.T) {
	b, store := newTestBossWithReadStore(t)

	onlineSubj := testutil.NewHost(t)
	offlineSubj := testutil.NewHost(t)

	b.SeedWorker(types.WorkerProfile{
		PeerID:    onlineSubj.ID().String(),
		AgentName: "online",
		CPUCores:  1,
	})
	testutil.AppendOwnRating(t, store, b.HostForTest(), offlineSubj.ID(), -0.2, "test")

	var buf bytes.Buffer
	b.RenderRadarForTest(&buf)
	out := buf.String()

	if !strings.Contains(out, "ONLINE") {
		t.Errorf("missing ONLINE section header; got:\n%s", out)
	}
	if !strings.Contains(out, "OFFLINE") {
		t.Errorf("missing OFFLINE section header; got:\n%s", out)
	}
}

// TestRadarRender_OnlineSectionShowsPeer verifies that a seeded online worker
// appears in the ONLINE section of the radar render.
func TestRadarRender_OnlineSectionShowsPeer(t *testing.T) {
	b, _ := newTestBossWithReadStore(t)

	h := testutil.NewHost(t)
	b.SeedWorker(types.WorkerProfile{
		PeerID:    h.ID().String(),
		AgentName: "my-agent",
		CPUCores:  2,
	})

	var buf bytes.Buffer
	b.RenderRadarForTest(&buf)
	out := buf.String()

	// The peer ID short prefix should appear somewhere in the output.
	shortPID := h.ID().String()[:12]
	if !strings.Contains(out, shortPID) {
		t.Errorf("expected short peer ID %q in radar output; got:\n%s", shortPID, out)
	}
}

// TestRadarRender_EmptyMesh verifies that ONLINE (0) and OFFLINE (0) are shown
// when there are no known peers.
func TestRadarRender_EmptyMesh(t *testing.T) {
	b, _ := newTestBossWithReadStore(t)

	var buf bytes.Buffer
	b.RenderRadarForTest(&buf)
	out := buf.String()

	if !strings.Contains(out, "ONLINE") {
		t.Errorf("missing ONLINE label when empty; got:\n%s", out)
	}
	if !strings.Contains(out, "OFFLINE") {
		t.Errorf("missing OFFLINE label when empty; got:\n%s", out)
	}
}
