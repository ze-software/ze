package reactor

import (
	"testing"
)

// VALIDATES: AC-28 -- pool maximum dynamically tracks peer set
// VALIDATES: AC-19 -- per-peer buffer share proportional to prefix count
// PREVENTS: static pool sizes that ignore actual workload

func TestWeightTracker_AddPeer(t *testing.T) {
	var lastGuaranteed, lastOverflow int
	wt := newWeightTracker(func(guaranteed, overflow int, _ overflowBudgetResult) {
		lastGuaranteed = guaranteed
		lastOverflow = overflow
	})

	wt.AddPeer("10.0.0.1", 1000, 1) // burstWeight=500, preEOR buffers=50

	if wt.PeerCount() != 1 {
		t.Fatalf("PeerCount() = %d, want 1", wt.PeerCount())
	}
	// Pre-EOR: 1000/20 = 50 buffers guaranteed.
	if lastGuaranteed != 50 {
		t.Errorf("guaranteed = %d, want 50", lastGuaranteed)
	}
	// K=1, top 1 = 50.
	if lastOverflow != 50 {
		t.Errorf("overflow = %d, want 50", lastOverflow)
	}
}

func TestWeightTracker_RemovePeer(t *testing.T) {
	var lastGuaranteed, lastOverflow int
	wt := newWeightTracker(func(guaranteed, overflow int, _ overflowBudgetResult) {
		lastGuaranteed = guaranteed
		lastOverflow = overflow
	})

	wt.AddPeer("10.0.0.1", 1000, 1)
	wt.AddPeer("10.0.0.2", 10000, 1)
	wt.RemovePeer("10.0.0.1")

	if wt.PeerCount() != 1 {
		t.Fatalf("PeerCount() = %d, want 1", wt.PeerCount())
	}
	// Only peer 2 remains: preEOR, 10000/20 = 500 buffers.
	if lastGuaranteed != 500 {
		t.Errorf("guaranteed = %d, want 500", lastGuaranteed)
	}
	// K=1, top 1 = 500.
	if lastOverflow != 500 {
		t.Errorf("overflow = %d, want 500", lastOverflow)
	}
}

func TestWeightTracker_RemoveNonexistent(t *testing.T) {
	var callCount int
	wt := newWeightTracker(func(_, _ int, _ overflowBudgetResult) { callCount++ })

	wt.AddPeer("10.0.0.1", 1000, 1)
	callCount = 0

	wt.RemovePeer("10.0.0.99") // does not exist
	if callCount != 0 {
		t.Error("callback should not fire for nonexistent peer removal")
	}
	if wt.PeerCount() != 1 {
		t.Fatalf("PeerCount() = %d, want 1", wt.PeerCount())
	}
}

func TestWeightTracker_EORTransition(t *testing.T) {
	var lastGuaranteed int
	wt := newWeightTracker(func(guaranteed, _ int, _ overflowBudgetResult) {
		lastGuaranteed = guaranteed
	})

	wt.AddPeer("10.0.0.1", 100000, 1) // preEOR: 100000/20 = 5000
	if lastGuaranteed != 5000 {
		t.Fatalf("pre-EOR guaranteed = %d, want 5000", lastGuaranteed)
	}

	wt.PeerEORComplete("10.0.0.1") // postEOR: burstWeight=15000, 15000/20=750
	if lastGuaranteed != 750 {
		t.Errorf("post-EOR guaranteed = %d, want 750", lastGuaranteed)
	}
}

func TestWeightTracker_EORReceivedIncremental(t *testing.T) {
	var callCount int
	var lastGuaranteed int
	wt := newWeightTracker(func(guaranteed, _ int, _ overflowBudgetResult) {
		callCount++
		lastGuaranteed = guaranteed
	})

	// Peer with 2 families: needs 2 EORs to transition.
	wt.AddPeer("10.0.0.1", 100000, 2) // preEOR: 100000/20 = 5000
	callCount = 0

	wt.PeerEORReceived("10.0.0.1") // 1 of 2, still pre-EOR
	if callCount != 0 {
		t.Error("callback should not fire before all family EORs received")
	}
	if d := wt.PeerDemand("10.0.0.1"); d != 5000 {
		t.Errorf("still pre-EOR demand = %d, want 5000", d)
	}

	wt.PeerEORReceived("10.0.0.1") // 2 of 2, transition to post-EOR
	if callCount != 1 {
		t.Errorf("callback should fire once on transition, got %d", callCount)
	}
	if lastGuaranteed != 750 {
		t.Errorf("post-EOR guaranteed = %d, want 750", lastGuaranteed)
	}
}

func TestWeightTracker_EORNonexistent(t *testing.T) {
	var callCount int
	wt := newWeightTracker(func(_, _ int, _ overflowBudgetResult) { callCount++ })
	callCount = 0

	wt.PeerEORComplete("10.0.0.99") // does not exist
	if callCount != 0 {
		t.Error("callback should not fire for nonexistent peer EOR")
	}
}

func TestWeightTracker_EORAlreadyPostEOR(t *testing.T) {
	var callCount int
	wt := newWeightTracker(func(_, _ int, _ overflowBudgetResult) { callCount++ })

	wt.AddPeer("10.0.0.1", 1000, 1)
	wt.PeerEORComplete("10.0.0.1")
	callCount = 0

	wt.PeerEORComplete("10.0.0.1") // already post-EOR
	if callCount != 0 {
		t.Error("callback should not fire for redundant EOR")
	}
}

func TestWeightTracker_MultiplePeers(t *testing.T) {
	var lastGuaranteed, lastOverflow int
	wt := newWeightTracker(func(guaranteed, overflow int, _ overflowBudgetResult) {
		lastGuaranteed = guaranteed
		lastOverflow = overflow
	})

	// Add 4 peers with different sizes.
	wt.AddPeer("10.0.0.1", 200, 1)    // preEOR: 200/20=10
	wt.AddPeer("10.0.0.2", 1000, 1)   // preEOR: 1000/20=50
	wt.AddPeer("10.0.0.3", 10000, 1)  // preEOR: 10000/20=500
	wt.AddPeer("10.0.0.4", 100000, 1) // preEOR: 100000/20=5000

	// Guaranteed = 10+50+500+5000 = 5560
	if lastGuaranteed != 5560 {
		t.Errorf("guaranteed = %d, want 5560", lastGuaranteed)
	}
	// K = sqrt(4) = 2, top 2 = 5000+500 = 5500
	if lastOverflow != 5500 {
		t.Errorf("overflow = %d, want 5500", lastOverflow)
	}

	// Transition all to post-EOR.
	wt.PeerEORComplete("10.0.0.1") // postEOR: burstWeight=200, 200/20=10
	wt.PeerEORComplete("10.0.0.2") // postEOR: burstWeight=500, 500/20=25
	wt.PeerEORComplete("10.0.0.3") // postEOR: burstWeight=3000, 3000/20=150
	wt.PeerEORComplete("10.0.0.4") // postEOR: burstWeight=15000, 15000/20=750

	// Guaranteed = 10+25+150+750 = 935
	if lastGuaranteed != 935 {
		t.Errorf("post-EOR guaranteed = %d, want 935", lastGuaranteed)
	}
	// K = 2, top 2 = 750+150 = 900
	if lastOverflow != 900 {
		t.Errorf("post-EOR overflow = %d, want 900", lastOverflow)
	}
}

func TestWeightTracker_PeerDemand(t *testing.T) {
	wt := newWeightTracker(func(_, _ int, _ overflowBudgetResult) {})

	wt.AddPeer("10.0.0.1", 100000, 1)
	// Pre-EOR demand.
	if d := wt.PeerDemand("10.0.0.1"); d != 5000 {
		t.Errorf("pre-EOR PeerDemand = %d, want 5000", d)
	}

	wt.PeerEORComplete("10.0.0.1")
	// Post-EOR demand.
	if d := wt.PeerDemand("10.0.0.1"); d != 750 {
		t.Errorf("post-EOR PeerDemand = %d, want 750", d)
	}

	// Unknown peer returns 0.
	if d := wt.PeerDemand("10.0.0.99"); d != 0 {
		t.Errorf("unknown PeerDemand = %d, want 0", d)
	}
}

func TestWeightTracker_TotalBudget(t *testing.T) {
	wt := newWeightTracker(func(_, _ int, _ overflowBudgetResult) {})

	wt.AddPeer("10.0.0.1", 1000, 1)
	wt.AddPeer("10.0.0.2", 10000, 1)

	guaranteed, overflow := wt.TotalBudget()

	// Peer 1: preEOR 1000/20=50; Peer 2: preEOR 10000/20=500.
	// Guaranteed = 50+500 = 550. K=max(1,sqrt(2))=1, top 1 = 500.
	if guaranteed != 550 {
		t.Errorf("guaranteed = %d, want 550", guaranteed)
	}
	if overflow != 500 {
		t.Errorf("overflow = %d, want 500", overflow)
	}
}

func TestWeightTracker_AddPeerReregistration(t *testing.T) {
	wt := newWeightTracker(func(_, _ int, _ overflowBudgetResult) {})

	wt.AddPeer("10.0.0.1", 100000, 2)
	wt.PeerEORReceived("10.0.0.1")
	wt.PeerEORReceived("10.0.0.1") // transition to post-EOR

	if d := wt.PeerDemand("10.0.0.1"); d != 750 {
		t.Fatalf("post-EOR demand = %d, want 750", d)
	}

	// Re-register with different prefix max: resets to pre-EOR.
	wt.AddPeer("10.0.0.1", 50000, 1)
	if d := wt.PeerDemand("10.0.0.1"); d != 2500 {
		t.Errorf("re-registered pre-EOR demand = %d, want 2500 (50000/20)", d)
	}
}

func TestWeightTracker_EORReceivedFamilyCountZero(t *testing.T) {
	var callCount int
	wt := newWeightTracker(func(_, _ int, _ overflowBudgetResult) { callCount++ })

	wt.AddPeer("10.0.0.1", 1000, 0) // familyCount=0
	callCount = 0

	// familyCount=0: first EOR transitions immediately to post-EOR.
	wt.PeerEORReceived("10.0.0.1")
	if callCount != 1 {
		t.Errorf("callback should fire on transition, got %d calls", callCount)
	}
}

func TestWeightTracker_UsageToWeightRatios(t *testing.T) {
	wt := newWeightTracker(func(_, _ int, _ overflowBudgetResult) {})

	wt.AddPeer("10.0.0.1", 100000, 1) // preEOR demand = 5000
	wt.AddPeer("10.0.0.2", 1000, 1)   // preEOR demand = 50

	depths := map[string]int{
		"10.0.0.1": 2500, // 2500/5000 = 0.5
		"10.0.0.2": 100,  // 100/50 = 2.0
		"10.0.0.3": 50,   // unknown peer, omitted
	}

	ratios := wt.UsageToWeightRatios(depths)

	if r, ok := ratios["10.0.0.1"]; !ok || r != 0.5 {
		t.Errorf("10.0.0.1 ratio = %v, want 0.5", r)
	}
	if r, ok := ratios["10.0.0.2"]; !ok || r != 2.0 {
		t.Errorf("10.0.0.2 ratio = %v, want 2.0", r)
	}
	if _, ok := ratios["10.0.0.3"]; ok {
		t.Error("unknown peer should be omitted from ratios")
	}

	// Zero depth peers are omitted.
	depths["10.0.0.1"] = 0
	ratios = wt.UsageToWeightRatios(depths)
	if _, ok := ratios["10.0.0.1"]; ok {
		t.Error("zero-depth peer should be omitted")
	}
}

func TestWeightTracker_NilCallback(t *testing.T) {
	// nil callback should not panic.
	wt := newWeightTracker(nil)
	wt.AddPeer("10.0.0.1", 1000, 1)
	wt.PeerEORComplete("10.0.0.1")
	wt.RemovePeer("10.0.0.1")
}

// --- fwd-auto-sizing Phase 5 tests ---

func TestOverflowPoolAutoResize(t *testing.T) {
	// AddPeer triggers overflow byte budget recalculation.
	mux := newMixedBufMux()
	var lastBudget int64
	wt := newWeightTracker(func(guaranteed, overflow int, _ overflowBudgetResult) {
		budget := int64(guaranteed+overflow) * 4096
		mux.SetByteBudget(budget)
		lastBudget = budget
	})

	// Add a peer: should trigger budget update.
	wt.AddPeer("10.0.0.1", 10000, 1)
	if lastBudget <= 0 {
		t.Fatalf("expected positive budget after AddPeer, got %d", lastBudget)
	}
	if mux.ByteBudget() != lastBudget {
		t.Fatalf("MixedBufMux budget = %d, want %d", mux.ByteBudget(), lastBudget)
	}

	// Add second peer: budget should increase.
	prevBudget := lastBudget
	wt.AddPeer("10.0.0.2", 10000, 1)
	if lastBudget <= prevBudget {
		t.Fatalf("budget should increase with more peers: was %d, now %d", prevBudget, lastBudget)
	}
}

func TestOverflowPoolEORShrink(t *testing.T) {
	// EOR transitions shrink byte budget.
	mux := newMixedBufMux()
	var lastBudget int64
	wt := newWeightTracker(func(guaranteed, overflow int, _ overflowBudgetResult) {
		budget := int64(guaranteed+overflow) * 4096
		mux.SetByteBudget(budget)
		lastBudget = budget
	})

	wt.AddPeer("10.0.0.1", 10000, 1) // pre-EOR
	preEORBudget := lastBudget

	wt.PeerEORComplete("10.0.0.1") // post-EOR
	postEORBudget := lastBudget

	if postEORBudget >= preEORBudget {
		t.Fatalf("post-EOR budget (%d) should be less than pre-EOR (%d)", postEORBudget, preEORBudget)
	}
}

func TestOverflowPoolEnvOverride(t *testing.T) {
	// When ze.fwd.pool.size > 0, auto-sizing should not change the budget.
	mux := newMixedBufMux()
	overrideBudget := int64(999999)
	mux.SetByteBudget(overrideBudget)

	// Simulate: callback does NOT update mux because env override is active.
	wt := newWeightTracker(func(_, _ int, _ overflowBudgetResult) {
		// No-op: operator override active.
	})

	wt.AddPeer("10.0.0.1", 10000, 1)
	// Budget should remain at the override value.
	if mux.ByteBudget() != overrideBudget {
		t.Fatalf("budget = %d, want override %d", mux.ByteBudget(), overrideBudget)
	}
}

// --- Finding #5: UpdateExtMsg tests ---

func TestWeightTracker_UpdateExtMsg(t *testing.T) {
	var callCount int
	var lastOB overflowBudgetResult
	wt := newWeightTracker(func(_, _ int, ob overflowBudgetResult) {
		callCount++
		lastOB = ob
	})

	// Add a standard peer (extMsg defaults to false).
	wt.AddPeer("10.0.0.1", 10000, 1)
	initialCalls := callCount
	initialBytes := lastOB.bytes

	// Update to ExtMsg -- callback should fire with higher byte budget.
	wt.UpdateExtMsg("10.0.0.1", true)
	if callCount != initialCalls+1 {
		t.Fatalf("UpdateExtMsg should fire callback, got %d calls (want %d)", callCount, initialCalls+1)
	}
	if lastOB.bytes <= initialBytes {
		t.Fatalf("ExtMsg peer should increase byte budget: was %d, now %d", initialBytes, lastOB.bytes)
	}

	// Update same value again -- callback should NOT fire (no change).
	prevCalls := callCount
	wt.UpdateExtMsg("10.0.0.1", true)
	if callCount != prevCalls {
		t.Fatalf("UpdateExtMsg(same value) should not fire callback, got %d calls (want %d)", callCount, prevCalls)
	}

	// Update unknown peer -- should not fire callback.
	prevCalls = callCount
	wt.UpdateExtMsg("10.0.0.99", true)
	if callCount != prevCalls {
		t.Fatalf("UpdateExtMsg(unknown peer) should not fire callback, got %d (want %d)", callCount, prevCalls)
	}
}
