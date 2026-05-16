package ledger_test

import "testing"

// Placeholder for the end-to-end "append on A, gossip to B, restart B,
// still has it" test that lands in P1-7. Kept here today so CI
// discovers the file and the test ID is reserved.
func TestLedger_IntegrationDissemination(t *testing.T) {
	t.Skip("waiting on P1-7 (2-peer dissemination + restart)")
}
