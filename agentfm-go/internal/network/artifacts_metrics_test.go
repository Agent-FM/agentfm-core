package network_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agentfm/internal/metrics"
	"agentfm/internal/network"
	"agentfm/test/testutil"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestArtifactsBuilt_IncrementsOnSuccess(t *testing.T) {
	t.Chdir(t.TempDir())

	beforeCount := counterValue(t, metrics.ArtifactsBuiltTotal)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	hosts := testutil.NewConnectedMesh(t, 2)
	server, client := hosts[0], hosts[1]
	server.SetStreamHandler(network.ArtifactProtocol, network.HandleArtifactStream)

	payload := []byte("PK\x03\x04 fake-but-positive-bytes")
	taskID := "task_abc123"

	s, err := client.NewStream(ctx, server.ID(), network.ArtifactProtocol)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	if err := s.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("deadline: %v", err)
	}
	if err := binary.Write(s, binary.LittleEndian, int64(len(payload))); err != nil {
		t.Fatalf("write size: %v", err)
	}
	if err := binary.Write(s, binary.LittleEndian, uint8(len(taskID))); err != nil {
		t.Fatalf("write id len: %v", err)
	}
	if _, err := s.Write([]byte(taskID)); err != nil {
		t.Fatalf("write id: %v", err)
	}
	if _, err := s.Write(payload); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	if err := s.CloseWrite(); err != nil {
		t.Fatalf("close-write: %v", err)
	}
	_, _ = bytes.NewBuffer(nil).ReadFrom(s)
	_ = s.Close()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if counterValue(t, metrics.ArtifactsBuiltTotal) > beforeCount {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	got := counterValue(t, metrics.ArtifactsBuiltTotal) - beforeCount
	if got != 1 {
		entries, _ := os.ReadDir(filepath.Join(".", "agentfm_artifacts"))
		t.Fatalf("expected ArtifactsBuiltTotal to grow by 1, grew by %v (artifacts dir: %v); server peer %s",
			got, entries, peer.ID(server.ID()).String())
	}
}

func TestArtifactsBuilt_DoesNotFireOnTruncated(t *testing.T) {
	t.Chdir(t.TempDir())
	beforeCount := counterValue(t, metrics.ArtifactsBuiltTotal)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	hosts := testutil.NewConnectedMesh(t, 2)
	server, client := hosts[0], hosts[1]
	server.SetStreamHandler(network.ArtifactProtocol, network.HandleArtifactStream)

	declared := int64(200)
	short := []byte("PK\x03\x04 way-shorter-than-declared")
	taskID := "task_trunc"

	s, err := client.NewStream(ctx, server.ID(), network.ArtifactProtocol)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	if err := s.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("deadline: %v", err)
	}
	_ = binary.Write(s, binary.LittleEndian, declared)
	_ = binary.Write(s, binary.LittleEndian, uint8(len(taskID)))
	_, _ = s.Write([]byte(taskID))
	_, _ = s.Write(short)
	_ = s.CloseWrite()
	_, _ = bytes.NewBuffer(nil).ReadFrom(s)
	_ = s.Close()

	time.Sleep(500 * time.Millisecond)
	got := counterValue(t, metrics.ArtifactsBuiltTotal) - beforeCount
	if got != 0 {
		t.Fatalf("expected no increment on truncated artifact, got %v", got)
	}
}

func counterValue(t testing.TB, c prometheus.Counter) float64 {
	t.Helper()
	m := &dto.Metric{}
	if err := c.Write(m); err != nil {
		t.Fatalf("counter write: %v", err)
	}
	return m.GetCounter().GetValue()
}
