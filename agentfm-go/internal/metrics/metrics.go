// Package metrics owns the AgentFM Prometheus registry and the shared
// counter/histogram/gauge families published by every role (boss, worker,
// relay).
//
// All collectors are package-level singletons. Roles increment them from
// anywhere via metrics.TasksTotal.WithLabelValues("ok").Inc() and similar.
//
// Cardinality discipline: labels on these metrics are deliberately bounded
// (e.g. status ∈ {ok, error, rejected, timeout}). Never add a peer_id, task_id,
// or unbounded user-supplied string as a label — Prometheus' time-series
// cardinality is the operator's bill.
package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry is the AgentFM-specific Prometheus registry. Kept distinct from
// prometheus.DefaultRegisterer so tests can construct fresh registries
// without polluting global state, and so the standard process_/go_ collectors
// are explicitly opted in (rather than implicitly attached).
var Registry = prometheus.NewRegistry()

// Status label values for TasksTotal.
const (
	StatusOK       = "ok"
	StatusError    = "error"
	StatusRejected = "rejected"
	StatusTimeout  = "timeout"
)

// Protocol label values for StreamErrorsTotal.
const (
	ProtocolTask      = "task"
	ProtocolArtifacts = "artifacts"
	ProtocolFeedback  = "feedback"
)

// Reason label values for StreamErrorsTotal.
const (
	ReasonDecode           = "decode"
	ReasonDeadline         = "deadline"
	ReasonReset            = "reset"
	ReasonPeerEOF          = "peer_eof"
	ReasonCapacityRejected = "capacity_rejected"
	ReasonVersionMismatch  = "version_mismatch"
	ReasonUnknown          = "unknown"
)

// DHT op label values for DHTQueriesTotal (relay-only).
const (
	DHTOpFindPeer = "find_peer"
	DHTOpProvide  = "provide"
	DHTOpGetValue = "get_value"
)

// TasksTotal counts task executions seen by this node, partitioned by
// terminal status.
var TasksTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "agentfm_tasks_total",
		Help: "Number of task executions, partitioned by terminal status.",
	},
	[]string{"status"},
)

// TaskDurationSeconds records wall-clock task durations. Buckets are tuned
// for AI-agent workloads (1s to 30min); the Prometheus default buckets
// (5ms-10s) would all overflow for typical AgentFM tasks.
var TaskDurationSeconds = prometheus.NewHistogram(
	prometheus.HistogramOpts{
		Name:    "agentfm_task_duration_seconds",
		Help:    "Wall-clock task duration in seconds.",
		Buckets: []float64{1, 5, 15, 60, 180, 300, 600, 1800},
	},
)

// WorkersOnline is the count of workers currently advertised in the boss's
// telemetry view. Set, not incremented.
var WorkersOnline = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "agentfm_workers_online",
		Help: "Number of workers currently visible in this node's telemetry.",
	},
)

// ArtifactBytesSentTotal is the total bytes shipped over the artifact
// protocol stream by this node.
var ArtifactBytesSentTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "agentfm_artifact_bytes_sent_total",
		Help: "Cumulative bytes shipped over the artifact protocol.",
	},
)

// StreamErrorsTotal counts failures on the custom libp2p protocols. The
// reason label values are an enum (see Reason* constants); never pass a
// raw error string here — it would explode cardinality.
var StreamErrorsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "agentfm_stream_errors_total",
		Help: "Stream-level failures on AgentFM libp2p protocols.",
	},
	[]string{"protocol", "reason"},
)

// DHTQueriesTotal is incremented by the relay role for every DHT operation
// it serves. The op label is bounded (find_peer / provide / get_value).
var DHTQueriesTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "agentfm_dht_queries_total",
		Help: "DHT operations served by this relay, partitioned by op.",
	},
	[]string{"op"},
)

func init() {
	Registry.MustRegister(
		TasksTotal,
		TaskDurationSeconds,
		WorkersOnline,
		ArtifactBytesSentTotal,
		StreamErrorsTotal,
		DHTQueriesTotal,
		// Standard runtime collectors. Operators expect process_* and go_*
		// in any Go service's /metrics output; omitting them would surprise
		// dashboard authors.
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
	)
	// Pre-create zero-valued samples for every known label tuple so the
	// metric families appear in scrape output even before the first event.
	// Without this, dashboards alert on "no data" when a fresh node starts
	// up rather than on actual zero traffic.
	for _, status := range []string{StatusOK, StatusError, StatusRejected, StatusTimeout} {
		TasksTotal.WithLabelValues(status)
	}
	for _, proto := range []string{ProtocolTask, ProtocolArtifacts, ProtocolFeedback} {
		for _, reason := range []string{
			ReasonDecode, ReasonDeadline, ReasonReset, ReasonPeerEOF,
			ReasonCapacityRejected, ReasonVersionMismatch, ReasonUnknown,
		} {
			StreamErrorsTotal.WithLabelValues(proto, reason)
		}
	}
	for _, op := range []string{DHTOpFindPeer, DHTOpProvide, DHTOpGetValue} {
		DHTQueriesTotal.WithLabelValues(op)
	}
}

// Handler returns an http.Handler that scrapes this package's registry.
// Mount it on /metrics in each role's HTTP server.
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{
		// Don't suppress runtime errors — surface scrape failures rather
		// than silently shipping partial data.
		ErrorHandling: promhttp.HTTPErrorOnError,
	})
}

// Serve starts a tiny HTTP server that exposes /metrics on listen. Empty
// listen disables the server (returns nil immediately). When listen is
// non-empty the call blocks until ctx is cancelled, then performs a
// graceful shutdown bounded by 5 seconds.
//
// Use "127.0.0.1:9090" for the safe-by-default loopback bind. Operators
// who want off-host scrapers pass "0.0.0.0:9090" via --prom-listen.
func Serve(ctx context.Context, listen string) error {
	if listen == "" {
		return nil
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", Handler())
	srv := &http.Server{
		Addr:              listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		err := srv.ListenAndServe()
		if err == http.ErrServerClosed {
			err = nil
		}
		errCh <- err
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return <-errCh
	case err := <-errCh:
		return err
	}
}
