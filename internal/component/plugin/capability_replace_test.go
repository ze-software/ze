package plugin

import (
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// roleCaps declares code 9 for each selector named, under plugin name.
func roleCaps(name, payload string, peers ...string) *PluginCapabilities {
	return &PluginCapabilities{PluginName: name, Capabilities: []PluginCapability{
		{Code: 9, Encoding: rpc.CapEncodingHex, Payload: payload, Peers: peers},
	}}
}

// VALIDATES: spec-runtime-peer-plugin-capabilities AC-3: a reload's
// declaration replaces the plugin's own entries, so a peer the reload adds is
// answered with its capability, and a peer the reload dropped is not.
// PREVENTS: a replace that keeps the Stage 3 entries beside the new ones, or
// that refuses the plugin's own previous declaration as a conflict.
func TestReplacePluginCapabilities(t *testing.T) {
	ci := NewCapabilityInjector()
	if err := ci.AddPluginCapabilities(roleCaps("p", "03", "192.0.2.1")); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := ci.ReplacePluginCapabilities(map[string]*PluginCapabilities{"p": roleCaps("p", "00", "192.0.2.2")}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := ci.GetCapabilitiesForSelectors("192.0.2.1"); len(got) != 0 {
		t.Fatalf("dropped selector still answered: %+v", got)
	}
	got := ci.GetCapabilitiesForSelectors("192.0.2.2")
	if len(got) != 1 || got[0].Code != 9 || len(got[0].Value) != 1 || got[0].Value[0] != 0x00 || got[0].Plugin != "p" {
		t.Fatalf("added selector answered %+v, want code 9 value 00 from p", got)
	}
}

// VALIDATES: AC-3: a replace that conflicts with ANOTHER plugin is refused and
// leaves the store as it was; the check answers the same without mutating.
// PREVENTS: a half-applied replace, or a verify that accepts what apply refuses.
func TestReplacePluginCapabilitiesConflict(t *testing.T) {
	ci := NewCapabilityInjector()
	if err := ci.AddPluginCapabilities(roleCaps("p", "03", "192.0.2.1")); err != nil {
		t.Fatalf("add p: %v", err)
	}
	if err := ci.AddPluginCapabilities(roleCaps("q", "01", "192.0.2.9")); err != nil {
		t.Fatalf("add q: %v", err)
	}
	clash := roleCaps("p", "00", "192.0.2.9")
	if err := ci.CheckReplacePluginCapabilities(map[string]*PluginCapabilities{"p": clash}); err == nil {
		t.Fatal("check accepted a code another plugin holds for the selector")
	}
	if err := ci.ReplacePluginCapabilities(map[string]*PluginCapabilities{"p": clash}); err == nil {
		t.Fatal("replace accepted a code another plugin holds for the selector")
	}
	got := ci.GetCapabilitiesForSelectors("192.0.2.1")
	if len(got) != 1 || got[0].Plugin != "p" || got[0].Value[0] != 0x03 {
		t.Fatalf("refused replace changed the store: %+v", got)
	}
}
