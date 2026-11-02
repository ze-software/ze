package hostname

import (
	"slices"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// hostnameReloadConfig is a bgp root with peer edge advertising host, and
// peer core advertising core-host only when coreHost is not empty.
func hostnameReloadConfig(host, coreHost string) []sdk.ConfigSection {
	core := `"core":{}`
	if coreHost != "" {
		core = `"core":{"session":{"capability":{"hostname":{"host":"` + coreHost + `","domain":"example.net"}}}}`
	}
	return []sdk.ConfigSection{{Root: "bgp", Data: `{"bgp":{"peer":{
		"edge":{"session":{"capability":{"hostname":{"host":"` + host + `","domain":"example.net"}}}},
		` + core + `
	}}}`}}
}

// hostnamePayload answers the payload the candidate declares for selector
// under the FQDN code, and whether it declares one.
func hostnamePayload(t *testing.T, sections []sdk.ConfigSection, selector string) (string, bool) {
	t.Helper()
	declared, err := sdk.CandidateCapabilities(hostnameCapabilities, sections)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range declared.Capabilities {
		if decl.Code == fqdnCapCode && slices.Contains(decl.Peers, selector) {
			return decl.Payload, true
		}
	}
	return "", false
}

// TestHostnameCapabilityExtractorRegistered checks that the extractor
// runHostnamePlugin registers gives a peer a reload configures a hostname on
// the capability, and a live peer whose hostname a reload changes the new
// payload, in the declaration the config-verify answer carries (AC-2). The
// declaration reads only the config (A-6), so the same config answers the
// same payload.
// VALIDATES: hostnameCapabilities through sdk.CandidateCapabilities, the
// declaration the composed config-verify answers.
// PREVENTS: a reload-configured hostname that never reaches the OPEN.
func TestHostnameCapabilityExtractorRegistered(t *testing.T) {
	startup := hostnameReloadConfig("router1", "")
	if _, ok := hostnamePayload(t, startup, "core"); ok {
		t.Fatal("the startup config declares a hostname for peer core")
	}
	before, ok := hostnamePayload(t, startup, "edge")
	if !ok {
		t.Fatal("the startup config declares no hostname for peer edge")
	}
	if same, _ := hostnamePayload(t, hostnameReloadConfig("router1", ""), "edge"); same != before {
		t.Errorf("the same config declares %q, then %q", before, same)
	}

	added, ok := hostnamePayload(t, hostnameReloadConfig("router1", "router1"), "core")
	if !ok {
		t.Fatal("the peer the reload configures carries no hostname capability")
	}
	if added != before {
		t.Errorf("core's payload %q, want edge's %q for the same hostname", added, before)
	}

	changed, ok := hostnamePayload(t, hostnameReloadConfig("router2", ""), "edge")
	if !ok {
		t.Fatal("a changed hostname declares nothing for peer edge")
	}
	if changed == before {
		t.Errorf("a hostname changed from router1 to router2 still declares %q", changed)
	}
}
