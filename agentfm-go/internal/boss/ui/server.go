// Package ui serves the P4-5 peer-reputation viewer at
// /ui/peer/{peer_id}. The HTML is embedded into the binary at
// compile time so operators don't need to deploy static assets
// separately.
package ui

import (
	_ "embed"
	"net/http"
	"strings"
)

//go:embed peer.html
var peerHTML []byte

// Handler returns an http.HandlerFunc that serves the embedded
// peer.html page on any GET /ui/peer/* path. The page itself
// parses the peer ID out of window.location.pathname and fetches
// data from the existing /v1/peers/{id}/{reputation,log,proof}
// endpoints.
func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "only GET", http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/ui/peer/") {
			http.NotFound(w, r)
			return
		}
		// Defensive: short paths with no peer id slug still serve
		// the page; the JS renders a "missing peer id in URL" error.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(peerHTML)
	}
}
