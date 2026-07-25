// VALIDATES: AC-11 -- BGP plugin declares debug flags via YANG registration.
// PREVENTS: BGP debug flags missing from debug YANG registry.

package yang

import (
	"testing"

	debugyang "github.com/ze-software/ze/internal/component/debug/yang"
)

func TestBGPDebugFlagsRegistered(t *testing.T) {
	if !debugyang.HasModule("bgp") {
		t.Fatal("expected bgp debug module to be registered")
	}

	if !debugyang.ValidateFlag("bgp.reactor", "update") {
		t.Error("expected 'update' to be a valid bgp debug flag")
	}
	if !debugyang.ValidateFlag("bgp.reactor", "open") {
		t.Error("expected 'open' to be a valid bgp debug flag")
	}
	if debugyang.ValidateFlag("bgp.reactor", "nonexistent") {
		t.Error("expected 'nonexistent' to be invalid for bgp")
	}
}

func TestBGPDebugScopesRegistered(t *testing.T) {
	if !debugyang.ValidateScope("bgp.reactor", "neighbor") {
		t.Error("expected 'neighbor' to be a valid bgp debug scope")
	}
	if !debugyang.ValidateScope("bgp.reactor", "group") {
		t.Error("expected 'group' to be a valid bgp debug scope")
	}
	if debugyang.ValidateScope("bgp.reactor", "interface") {
		t.Error("expected 'interface' to be invalid for bgp")
	}
}
