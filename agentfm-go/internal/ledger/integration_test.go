package ledger_test

// The 2-peer dissemination + restart scenario this file used to stub
// (TestLedger_IntegrationDissemination t.Skip) is now implemented at
//
//   test/integration/ledger_dissemination_test.go::TestLedger_TwoPeerDisseminationAndRestart
//
// It runs as part of the integration suite (`make test-integration`)
// rather than the unit suite (`make test`), matching the convention
// for tests that spin up real libp2p hosts. Two unit-scoped slices of
// that scenario live alongside the impl in:
//
//   internal/ledger/impl_test.go::TestAppend_SurvivesRestart_ChainContinues
//   internal/ledger/impl_test.go::TestTwoPeer_BInboxIngestsAEntry
//
// Kept this file present (empty package-level only) so Git history of
// the stub's lineage stays grep-able.
