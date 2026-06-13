// VALIDATES: AC-11 -- debug YANG modules register separately from config YANG.
// VALIDATES: AC-14 -- ValidateFlag rejects unknown flags using registered YANG metadata.
// PREVENTS: Debug YANG registration mixing with config YANG registry.

package yang

import "testing"

func TestRegisterModule(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	RegisterModule(Module{
		Name:   "ze-bgp-debug.yang",
		Prefix: "bgp",
		Flags:  []string{"open", "update", "keepalive"},
		Scopes: []string{"neighbor", "group"},
	})

	if !HasModule("bgp") {
		t.Fatal("expected bgp module to be registered")
	}
	if !ValidateFlag("bgp", "open") {
		t.Error("expected open flag registered")
	}
	if !ValidateFlag("bgp", "update") {
		t.Error("expected update flag registered")
	}
	if !ValidateFlag("bgp", "keepalive") {
		t.Error("expected keepalive flag registered")
	}
}

func TestModulesEmpty(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	if HasModule("bgp") {
		t.Fatal("expected no modules registered")
	}
}

func TestMultipleModules(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	RegisterModule(Module{Name: "ze-bgp-debug.yang", Prefix: "bgp", Flags: []string{"open"}})
	RegisterModule(Module{Name: "ze-iface-debug.yang", Prefix: "iface", Flags: []string{"netlink"}})

	if !HasModule("bgp") {
		t.Error("expected bgp module")
	}
	if !HasModule("iface") {
		t.Error("expected iface module")
	}
}

func TestValidateFlag(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	RegisterModule(Module{
		Name:   "ze-bgp-debug.yang",
		Prefix: "bgp",
		Flags:  []string{"open", "update", "keepalive", "notification"},
	})

	tests := []struct {
		module string
		flag   string
		valid  bool
	}{
		{"bgp.reactor", "update", true},
		{"bgp.reactor", "open", true},
		{"bgp.reactor", "nonexistent", false},
		{"bgp", "update", true},
		{"plugin.manager", "update", false},
	}

	for _, tt := range tests {
		got := ValidateFlag(tt.module, tt.flag)
		if got != tt.valid {
			t.Errorf("ValidateFlag(%q, %q) = %v, want %v", tt.module, tt.flag, got, tt.valid)
		}
	}
}

func TestValidateFlagNoModules(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	if ValidateFlag("bgp.reactor", "update") {
		t.Error("expected false when no modules registered")
	}
}

func TestValidateScope(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	RegisterModule(Module{
		Name:   "ze-bgp-debug.yang",
		Prefix: "bgp",
		Scopes: []string{"neighbor", "group"},
	})

	if !ValidateScope("bgp.reactor", "neighbor") {
		t.Error("expected neighbor to be valid for bgp.reactor")
	}
	if ValidateScope("bgp.reactor", "interface") {
		t.Error("expected interface to be invalid for bgp.reactor")
	}
}
