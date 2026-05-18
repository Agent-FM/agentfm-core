package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"agentfm/internal/boss"
	"agentfm/internal/network"
	"agentfm/internal/types"
	"agentfm/test/testutil"

	netcore "github.com/libp2p/go-libp2p/core/network"
)

// TestDispatch_InboundConnectionNoPeerstoreAddrs pins the regression for
// the symptom "worker boots but boss cannot dispatch". When the worker
// dialed the boss first (the steady-state shape for NAT'd workers
// punching out to a lighthouse), the boss's peerstore knows only the
// worker's ephemeral source-port from the inbound connection — not its
// listening multiaddr. dialWorkerStream MUST short-circuit on
// Connectedness == Connected and reuse the live tunnel rather than
// fail with "peer not in cache and DHT unavailable".
//
// Before the fix, this test failed deterministically with a 500
// "Failed to connect to worker via DHT or Relay" because the peerstore
// path errored before NewStream got a chance.
func TestDispatch_InboundConnectionNoPeerstoreAddrs(t *testing.T) {
	// Two raw hosts with no DHT — mirrors NewForTest's stripped MeshNode.
	hosts := testutil.NewConnectedMesh(t, 2)
	workerHost, bossHost := hosts[0], hosts[1]

	workerHost.SetStreamHandler(network.TaskProtocol, func(s netcore.Stream) {
		_ = s.SetDeadline(time.Now().Add(5 * time.Second))
		var p types.TaskPayload
		_ = json.NewDecoder(io.LimitReader(s, 1024*1024)).Decode(&p)
		_, _ = s.Write([]byte("ok\n"))
		_ = s.Close()
	})

	// Sanity: the boss is Connected to the worker but its peerstore
	// does NOT have the worker's listen addr — this is the precise
	// libp2p quirk the production-side bug fell over.
	if got := bossHost.Network().Connectedness(workerHost.ID()); got != netcore.Connected {
		t.Fatalf("boss connectedness=%v, want Connected", got)
	}
	if addrs := bossHost.Peerstore().PeerInfo(workerHost.ID()).Addrs; len(addrs) != 0 {
		t.Logf("note: peerstore has %d addrs; bug repros most reliably with 0", len(addrs))
	}

	b := boss.NewForTest(&network.MeshNode{Host: bossHost})
	b.SeedWorker(types.WorkerProfile{
		PeerID:    workerHost.ID().String(),
		AgentName: "echo",
		CPUCores:  1,
		MaxTasks:  1,
		Status:    "AVAILABLE",
	})

	body, _ := json.Marshal(map[string]string{
		"worker_id": workerHost.ID().String(),
		"prompt":    "hi",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/execute", bytes.NewReader(body))
	b.ServeHTTPExecute(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status=%d body=%q (regression: dispatch to inbound-connected peer failed)",
			rec.Code, rec.Body.String())
	}
}
