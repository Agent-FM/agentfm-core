package store_test

import (
	"context"
	"testing"

	"agentfm/test/testutil"
)

// TestDistinctSubjects_ReturnsAllAcrossOwnAndInbox verifies that
// DistinctSubjects collects subject peer IDs from BOTH the own log
// (entries table) and the inbox (inbox_entries table), deduplicating
// across both sources.
func TestDistinctSubjects_ReturnsAllAcrossOwnAndInbox(t *testing.T) {
	s := openFresh(t)

	subj1Host := testutil.NewHost(t)
	subj2Host := testutil.NewHost(t)
	rater1 := testutil.NewHost(t)
	rater2 := testutil.NewHost(t)

	subj1 := subj1Host.ID()
	subj2 := subj2Host.ID()

	// One own-log entry about subj1, one inbox entry about subj2.
	testutil.AppendOwnRating(t, s, rater1, subj1, 0.5, "x")
	testutil.AppendInboxRating(t, s, rater2, subj2, 0.3, "y")

	got, err := s.DistinctSubjects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 distinct subjects; got %d", len(got))
	}
}

// TestDistinctSubjects_DeduplicatesAcrossTables verifies that a subject
// appearing in both own and inbox is returned only once.
func TestDistinctSubjects_DeduplicatesAcrossTables(t *testing.T) {
	s := openFresh(t)

	rater1 := testutil.NewHost(t)
	rater2 := testutil.NewHost(t)
	subjHost := testutil.NewHost(t)
	subj := subjHost.ID()

	// Same subject in both tables.
	testutil.AppendOwnRating(t, s, rater1, subj, 0.5, "a")
	testutil.AppendInboxRating(t, s, rater2, subj, 0.3, "b")

	got, err := s.DistinctSubjects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 distinct subject (deduplicated); got %d", len(got))
	}
}

// TestDistinctSubjects_EmptyStore verifies that DistinctSubjects returns
// an empty slice (not an error) when no entries exist.
func TestDistinctSubjects_EmptyStore(t *testing.T) {
	s := openFresh(t)

	got, err := s.DistinctSubjects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 subjects from empty store; got %d", len(got))
	}
}
