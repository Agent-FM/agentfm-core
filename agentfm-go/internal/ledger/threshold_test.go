package ledger_test

import (
	"testing"

	"agentfm/internal/ledger"
	pb "agentfm/internal/ledger/pb"
)

func TestIsHeadValid_NilHead(t *testing.T) {
	if ledger.IsHeadValid(nil, 1) {
		t.Fatal("nil head should never be valid")
	}
}

func TestIsHeadValid_ThresholdZero_AlwaysValid(t *testing.T) {
	h := &pb.LogHead{TreeSize: 1}
	if !ledger.IsHeadValid(h, 0) {
		t.Fatal("threshold 0 should always be valid (no quorum required)")
	}
}

func TestIsHeadValid_AboveThreshold(t *testing.T) {
	h := &pb.LogHead{
		WitnessSigs: []*pb.WitnessSig{{}, {}, {}, {}},
	}
	if !ledger.IsHeadValid(h, 3) {
		t.Fatal("4 sigs should satisfy threshold 3")
	}
	if !ledger.IsHeadValid(h, 4) {
		t.Fatal("4 sigs should satisfy threshold 4 (exact)")
	}
}

func TestIsHeadValid_BelowThreshold(t *testing.T) {
	h := &pb.LogHead{
		WitnessSigs: []*pb.WitnessSig{{}, {}},
	}
	if ledger.IsHeadValid(h, 3) {
		t.Fatal("2 sigs should NOT satisfy threshold 3 — head must be flagged advisory")
	}
}

func TestIsHeadValid_NoSigs_BelowAnyPositiveThreshold(t *testing.T) {
	h := &pb.LogHead{}
	if ledger.IsHeadValid(h, 1) {
		t.Fatal("no sigs should fail threshold 1")
	}
}
