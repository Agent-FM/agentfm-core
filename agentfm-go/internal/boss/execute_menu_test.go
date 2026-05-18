package boss

import (
	"context"
	"sync/atomic"
	"testing"

	"agentfm/internal/types"
)

// TestExecuteFlow_BackToRadar_ReturnsImmediately verifies that choosing
// "Back to radar" exits executeFlow without calling viewPeerHistory.
func TestExecuteFlow_BackToRadar_ReturnsImmediately(t *testing.T) {
	b, _ := newTestBossWithLedger(t)
	worker := types.WorkerProfile{PeerID: "12D3KooWtest", AgentName: "test"}

	var viewCalls atomic.Int32
	b.SetPeerViewHookForTest(func(_ context.Context, _ string) { viewCalls.Add(1) })
	b.SetMenuPickerForTest(func(_ []string) (string, error) {
		return "Back to radar", nil
	})

	b.executeFlow(context.Background(), worker)

	if viewCalls.Load() != 0 {
		t.Fatalf("expected viewPeerHistory not called; got %d calls", viewCalls.Load())
	}
}

// TestExecuteFlow_OffersThreeChoiceMenu_ChoosingViewCallsPeerView verifies
// that:
//  1. Choosing "View ratings & feedback" calls viewPeerHistory exactly once.
//  2. executeFlow loops back to the menu after viewPeerHistory returns.
//  3. On the second iteration "Back to radar" exits cleanly.
func TestExecuteFlow_OffersThreeChoiceMenu_ChoosingViewCallsPeerView(t *testing.T) {
	b, _ := newTestBossWithLedger(t)
	worker := types.WorkerProfile{PeerID: "12D3KooWtest", AgentName: "test"}

	var viewCalls atomic.Int32
	b.SetPeerViewHookForTest(func(_ context.Context, _ string) { viewCalls.Add(1) })

	callCount := 0
	b.SetMenuPickerForTest(func(opts []string) (string, error) {
		// Verify the three expected options are present.
		if len(opts) != 3 {
			t.Errorf("expected 3 menu options; got %d: %v", len(opts), opts)
		}
		callCount++
		if callCount == 1 {
			return "View ratings & feedback", nil
		}
		return "Back to radar", nil
	})

	b.executeFlow(context.Background(), worker)

	if viewCalls.Load() != 1 {
		t.Fatalf("expected viewPeerHistory called once; got %d", viewCalls.Load())
	}
	if callCount != 2 {
		t.Fatalf("expected menu shown twice (view + back); got %d", callCount)
	}
}

// TestExecuteFlow_MenuOptions_ContainsExpectedStrings verifies the three
// canonical option strings are offered to the picker.
func TestExecuteFlow_MenuOptions_ContainsExpectedStrings(t *testing.T) {
	b, _ := newTestBossWithLedger(t)
	worker := types.WorkerProfile{PeerID: "12D3KooWtest", AgentName: "test"}

	var got []string
	b.SetMenuPickerForTest(func(opts []string) (string, error) {
		got = opts
		return "Back to radar", nil
	})
	b.executeFlow(context.Background(), worker)

	want := map[string]bool{
		"Execute task":            false,
		"View ratings & feedback": false,
		"Back to radar":           false,
	}
	for _, o := range got {
		want[o] = true
	}
	for k, found := range want {
		if !found {
			t.Errorf("expected menu option %q not found in %v", k, got)
		}
	}
}
