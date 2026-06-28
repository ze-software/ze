package local

import (
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
