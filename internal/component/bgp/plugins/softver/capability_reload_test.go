package softver

import (
	"slices"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// softverReloadConfig is a bgp root with peer edge carrying software-version
// in the given encoding, and peer core carrying it only when coreEnabled.
func softverReloadConfig(encoding string, coreEnabled bool) []sdk.ConfigSection {
	core := `"core":{}`
	if coreEnabled {
		core = `"core":{"session":{"capability":{"software-version":{"encoding":"` + encoding + `"}}}}`
	}
	return []sdk.ConfigSection{{Root: "bgp", Data: `{"bgp":{"peer":{
		"edge":{"session":{"capability":{"software-version":{"encoding":"` + encoding + `"}}}},
		` + core + `
	}}}`}}
}

// softverPayload answers the payload the candidate declares for selector under
// the software-version code, and whether it declares one.
func softverPayload(t *testing.T, sections []sdk.ConfigSection, selector string) (string, bool) {
	t.Helper()
	declared, err := sdk.CandidateCapabilities(softverCapabilities, sections)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range declared.Capabilities {
		if decl.Code == softverCapCode && slices.Contains(decl.Peers, selector) {
			return decl.Payload, true
		}
	}
	return "", false
}

// TestSoftverCapabilityExtractorRegistered checks that the extractor
// RunSoftverPlugin registers gives a peer a reload enables software-version on
// the capability, and a live peer whose encoding a reload changes the new
// payload, in the declaration the config-verify answer carries (AC-2).
// VALIDATES: softverCapabilities through sdk.CandidateCapabilities, the
// declaration the composed config-verify answers.
// PREVENTS: a reload-enabled software-version that never reaches the OPEN.
func TestSoftverCapabilityExtractorRegistered(t *testing.T) {
	startup := softverReloadConfig("legacy", false)
	if _, ok := softverPayload(t, startup, "core"); ok {
		t.Fatal("the startup config declares software-version for peer core")
	}
	before, ok := softverPayload(t, startup, "edge")
	if !ok {
		t.Fatal("the startup config declares no software-version for peer edge")
	}

	added, ok := softverPayload(t, softverReloadConfig("legacy", true), "core")
	if !ok {
		t.Fatal("the peer the reload enables carries no software-version capability")
	}
	if added != before {
		t.Errorf("core's payload %q, want edge's %q for the same encoding", added, before)
	}

	changed, ok := softverPayload(t, softverReloadConfig("draft", false), "edge")
	if !ok {
		t.Fatal("a changed encoding declares no software-version for peer edge")
	}
	if changed == before {
		t.Errorf("an encoding changed from legacy to draft still declares %q", changed)
	}
}
