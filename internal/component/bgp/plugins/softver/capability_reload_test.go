package softver

import (
	"strings"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// softverReloadConfig is a bgp root with peer edge carrying software-version
// in the given encoding, and peer core carrying none.
func softverReloadConfig(encoding string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: "bgp", Data: `{"bgp":{"peer":{
		"edge":{"session":{"capability":{"software-version":{"encoding":"` + encoding + `"}}}},
		"core":{}
	}}}`}}
}

// TestSoftverCapabilityExtractorRegistered checks that the extractor and the
// leaves RunSoftverPlugin registers make the SDK refuse a reload that changes
// software-version on a live peer (AC-2) and accept one that keeps it (AC-2e).
// VALIDATES: softverCapabilities and softverCapabilityLeaves through
// sdk.CheckCapabilityReload, the comparison the composed config-verify runs.
// PREVENTS: a softver declaration the SDK cannot compare, or a refusal that
// names a code number instead of the software-version leaf.
func TestSoftverCapabilityExtractorRegistered(t *testing.T) {
	startup := softverReloadConfig("legacy")

	if err := sdk.CheckCapabilityReload("bgp-softver", softverCapabilities, softverCapabilityLeaves,
		startup, softverReloadConfig("legacy")); err != nil {
		t.Fatalf("an unchanged software-version was refused: %v", err)
	}

	err := sdk.CheckCapabilityReload("bgp-softver", softverCapabilities, softverCapabilityLeaves,
		startup, softverReloadConfig("draft"))
	if err == nil {
		t.Fatal("a changed software-version encoding was accepted")
	}
	want := "peer edge: capability software-version cannot change while ze runs (plugin bgp-softver)."
	if first, _, _ := strings.Cut(err.Error(), "\n"); first != want {
		t.Errorf("refusal opens with %q, want %q", first, want)
	}
}
