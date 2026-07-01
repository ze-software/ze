package local

import (
	"errors"
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
	"codeberg.org/thomas-mangin/ze/internal/core/ddosevent"
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
