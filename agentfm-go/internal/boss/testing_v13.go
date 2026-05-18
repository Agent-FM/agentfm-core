package boss

import "net/http"

// TestExportHandlePeers exposes the umbrella /v1/peers/* handler
// for integration tests that need to drive the v1.3 reputation /
// log / proof / comments endpoints without bringing up the full
// API server (auth, bind, CORS, etc.).
//
// Production code MUST NOT use this; the production wiring goes
// through StartAPIServer which installs the full middleware chain.
func TestExportHandlePeers(b *Boss) http.HandlerFunc {
	return b.handlePeers
}
