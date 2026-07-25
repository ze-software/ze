package local

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/ddosevent"
)

func withNoopFirewall() func() {
	origReg := registerTables
	origApply := applyAll
	registerTables = func(_ string, _ []firewall.Table) {}
	applyAll = func() error { return nil }
	return func() {
		registerTables = origReg
		applyAll = origApply
	}
}

func TestAlertModeInstallsNothing(t *testing.T) {
	// VALIDATES: AC-4 -- alert mode logs, installs nothing
	r := newResponder(&Config{ResponseLevel: "alert"}, nil)
	event := &ddosevent.AttackDetected{
		Interface: "xe0",
		Target: ddosevent.VectorTuple{
			DstPrefix: netip.MustParsePrefix("10.0.0.1/32"),
			Proto:     17,
			DstPort:   53,
		},
		Family:    ddosevent.FamilyUDPFlood,
		PeakRxPps: 500000,
	}
	r.onDetected(event)
	if r.active {
		t.Error("alert mode should not activate mitigation")
	}
}

func TestEnforceModeActivates(t *testing.T) {
	// VALIDATES: AC-1 -- enforce mode installs a mitigation
	defer withNoopFirewall()()
	r := newResponder(&Config{ResponseLevel: "enforce"}, nil)
	event := &ddosevent.AttackDetected{
		Interface: "xe0",
		Target: ddosevent.VectorTuple{
			DstPrefix: netip.MustParsePrefix("10.0.0.1/32"),
			Proto:     17,
			DstPort:   53,
		},
		Family:    ddosevent.FamilyUDPFlood,
		PeakRxPps: 500000,
	}
	r.onDetected(event)
	if !r.active {
		t.Error("enforce mode should activate mitigation")
	}
}

// VALIDATES: AC-11/AC-12 -- confidence-min gates the characterized mitigation path;
// the default of 0 leaves behavior unchanged.
func TestLocalConfidenceGate(t *testing.T) {
	defer withNoopFirewall()()
	victim := netip.MustParsePrefix("10.0.0.1/32")
	ev := func(conf int) *ddosevent.AttackCharacterized {
		return &ddosevent.AttackCharacterized{
			Target:     ddosevent.VectorTuple{DstPrefix: victim, Proto: 17},
			Family:     ddosevent.FamilyUDPFlood,
			Confidence: conf,
		}
	}

	r := newResponder(&Config{ResponseLevel: "enforce", ConfidenceMin: 80}, nil)
	r.onCharacterized(ev(50))
	if r.active {
		t.Error("confidence 50 below min 80 must not mitigate")
	}
	r.onCharacterized(ev(80))
	if !r.active {
		t.Error("confidence 80 >= min 80 must mitigate")
	}

	// Default confidence-min 0 does not gate.
	r2 := newResponder(&Config{ResponseLevel: "enforce"}, nil)
	r2.onCharacterized(ev(1))
	if !r2.active {
		t.Error("confidence-min 0 must not gate (behavior unchanged)")
	}
}

func tablesHaveTCPFlags(tables []firewall.Table) bool {
	for _, tbl := range tables {
		for _, ch := range tbl.Chains {
			for _, term := range ch.Terms {
				for _, m := range term.Matches {
					if _, ok := m.(firewall.MatchTCPFlags); ok {
						return true
					}
				}
			}
		}
	}
	return false
}

func TestLocalNarrowsInPlace(t *testing.T) {
	// VALIDATES: AC-7 -- on AttackCharacterized the local rule is re-registered in
	// place with the narrowed vector (proto + TCP flags), starting from a coarse
	// AttackDetected drop.
	origReg := registerTables
	origApply := applyAll
	var lastTables []firewall.Table
	registerTables = func(_ string, tables []firewall.Table) { lastTables = tables }
	applyAll = func() error { return nil }
	defer func() { registerTables = origReg; applyAll = origApply }()

	r := newResponder(&Config{ResponseLevel: "enforce"}, nil)
	victim := netip.MustParsePrefix("10.0.0.1/32")

	r.onDetected(&ddosevent.AttackDetected{
		Target: ddosevent.VectorTuple{DstPrefix: victim},
		Family: ddosevent.FamilyGenericFlood,
	})
	if !r.active {
		t.Fatal("coarse drop should be active after detect")
	}
	if tablesHaveTCPFlags(lastTables) {
		t.Error("coarse rule should not carry TCP flags")
	}

	r.onCharacterized(&ddosevent.AttackCharacterized{
		Target: ddosevent.VectorTuple{DstPrefix: victim, Proto: 6, TCPFlags: 0x02, DstPort: 80},
		Family: ddosevent.FamilySYNFlood,
	})
	if !r.active {
		t.Fatal("should stay active after narrowing")
	}
	if r.target.Proto != 6 || r.target.TCPFlags != 0x02 {
		t.Errorf("responder target not narrowed in place: %+v", r.target)
	}
	if !tablesHaveTCPFlags(lastTables) {
		t.Error("narrowed rule should carry the SYN TCP-flags match")
	}
}

func TestLocalApplyFailureRollsBack(t *testing.T) {
	// VALIDATES: on a failed nft apply the responder rolls the registry back to
	// nil and clears active, rather than leaving a phantom active mitigation with
	// the registry empty while the kernel keeps the last rule (review Run-4 NOTE).
	origReg := registerTables
	origApply := applyAll
	var lastTables []firewall.Table
	registerTables = func(_ string, tables []firewall.Table) { lastTables = tables }
	applyAll = func() error { return errors.New("nft apply failed") }
	defer func() { registerTables = origReg; applyAll = origApply }()

	r := newResponder(&Config{ResponseLevel: "enforce"}, nil)
	r.onDetected(&ddosevent.AttackDetected{
		Target: ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32")},
	})

	if r.active {
		t.Error("a failed apply must not leave the responder active")
	}
	if lastTables != nil {
		t.Errorf("registry must be rolled back to nil on apply failure, got %v", lastTables)
	}
}

func firstChainHook(tables []firewall.Table) firewall.ChainHook {
	if len(tables) > 0 && len(tables[0].Chains) > 0 {
		return tables[0].Chains[0].Hook
	}
	return 0
}

func TestLocalHookByDirection(t *testing.T) {
	// VALIDATES: AC-9/AC-10/AC-11 -- direction selects the netfilter hook: INPUT for a
	// local victim, FORWARD for a remote victim when forward-mitigation is on, and no
	// drop at all for a remote victim when it is off (flowspec owns that case).
	origReg := registerTables
	origApply := applyAll
	var lastTables []firewall.Table
	registerTables = func(_ string, tables []firewall.Table) { lastTables = tables }
	applyAll = func() error { return nil }
	defer func() { registerTables = origReg; applyAll = origApply }()

	victim := netip.MustParsePrefix("10.0.0.1/32")

	rLocal := newResponder(&Config{ResponseLevel: "enforce"}, nil)
	rLocal.onDetected(&ddosevent.AttackDetected{
		Target: ddosevent.VectorTuple{DstPrefix: victim}, Direction: ddosevent.DirectionLocal,
	})
	if !rLocal.active || firstChainHook(lastTables) != firewall.HookInput {
		t.Errorf("local victim: want active INPUT drop, active=%v hook=%v", rLocal.active, firstChainHook(lastTables))
	}

	lastTables = nil
	rFwd := newResponder(&Config{ResponseLevel: "enforce", ForwardMitigation: true}, nil)
	rFwd.onDetected(&ddosevent.AttackDetected{
		Target: ddosevent.VectorTuple{DstPrefix: victim}, Direction: ddosevent.DirectionRemote,
	})
	if !rFwd.active || firstChainHook(lastTables) != firewall.HookForward {
		t.Errorf("remote victim + forward-mitigation: want active FORWARD drop, active=%v hook=%v", rFwd.active, firstChainHook(lastTables))
	}

	rOff := newResponder(&Config{ResponseLevel: "enforce"}, nil)
	rOff.onDetected(&ddosevent.AttackDetected{
		Target: ddosevent.VectorTuple{DstPrefix: victim}, Direction: ddosevent.DirectionRemote,
	})
	if rOff.active {
		t.Error("remote victim with forward-mitigation off must not install a drop")
	}
}

func TestLocalHonorsSuppressMitigation(t *testing.T) {
	// VALIDATES: AC-2/AC-4 -- the policy's SuppressMitigation flag stops a drop, and a
	// characterized flip to exempt withdraws a drop the fast path already installed.
	defer withNoopFirewall()()
	victim := netip.MustParsePrefix("10.0.0.1/32")

	r := newResponder(&Config{ResponseLevel: "enforce"}, nil)
	r.onDetected(&ddosevent.AttackDetected{
		Target: ddosevent.VectorTuple{DstPrefix: victim}, Direction: ddosevent.DirectionLocal, SuppressMitigation: true,
	})
	if r.active {
		t.Error("SuppressMitigation must prevent a drop")
	}

	r2 := newResponder(&Config{ResponseLevel: "enforce"}, nil)
	r2.onDetected(&ddosevent.AttackDetected{
		Target: ddosevent.VectorTuple{DstPrefix: victim}, Direction: ddosevent.DirectionLocal,
	})
	if !r2.active {
		t.Fatal("fast path should install a drop")
	}
	r2.onCharacterized(&ddosevent.AttackCharacterized{
		Target: ddosevent.VectorTuple{DstPrefix: victim}, Direction: ddosevent.DirectionLocal, SuppressMitigation: true,
	})
	if r2.active {
		t.Error("characterized SuppressMitigation must withdraw the fast-path drop")
	}
}

func TestClearedDeactivates(t *testing.T) {
	// VALIDATES: AC-3 -- AttackCleared removes the mitigation
	defer withNoopFirewall()()
	r := newResponder(&Config{ResponseLevel: "enforce"}, nil)
	r.onDetected(&ddosevent.AttackDetected{
		Target: ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32"), Proto: 17},
	})
	if !r.active {
		t.Fatal("should be active")
	}
	r.onCleared(&ddosevent.AttackCleared{
		Target: ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("10.0.0.1/32")},
	})
	if r.active {
		t.Error("should be inactive after clear")
	}
}
