package hostname

import (
	"strings"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// hostnameReloadConfig is a bgp root with peer edge advertising host, and
// peer core advertising no hostname.
func hostnameReloadConfig(host string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: "bgp", Data: `{"bgp":{"peer":{
		"edge":{"session":{"capability":{"hostname":{"host":"` + host + `","domain":"example.net"}}}},
		"core":{}
	}}}`}}
}

// TestHostnameCapabilityExtractorRegistered checks that the extractor and the
// leaves runHostnamePlugin registers make the SDK refuse a reload that changes
// a live peer's hostname and accept one that keeps it (AC-2d, AC-2e). The
// declaration reads only the config (A-6), so the same config is accepted.
// VALIDATES: hostnameCapabilities and hostnameCapabilityLeaves through
// sdk.CheckCapabilityReload, the comparison the composed config-verify runs.
// PREVENTS: a hostname declaration the SDK cannot compare, or a refusal that
// names a code number instead of the hostname leaf.
func TestHostnameCapabilityExtractorRegistered(t *testing.T) {
	startup := hostnameReloadConfig("router1")

	if err := sdk.CheckCapabilityReload("bgp-hostname", hostnameCapabilities, hostnameCapabilityLeaves,
		startup, hostnameReloadConfig("router1")); err != nil {
		t.Fatalf("an unchanged hostname was refused: %v", err)
	}

	err := sdk.CheckCapabilityReload("bgp-hostname", hostnameCapabilities, hostnameCapabilityLeaves,
		startup, hostnameReloadConfig("router2"))
	if err == nil {
		t.Fatal("a changed hostname was accepted")
	}
	want := "peer edge: capability hostname cannot change while ze runs (plugin bgp-hostname)."
	if first, _, _ := strings.Cut(err.Error(), "\n"); first != want {
		t.Errorf("refusal opens with %q, want %q", first, want)
	}
}
