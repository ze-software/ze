package bridgeplugin

import (
	"strings"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// bridgeReloadConfig is an exabgp root whose bridge sets route-refresh and
// add-path, over the given families.
func bridgeReloadConfig(routeRefresh, addPath, families string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: configRoot, Data: `{"exabgp":{"bridge":{
		"process":{"main":{"run":"./p.py"}},
		"route-refresh":"` + routeRefresh + `",
		"add-path":"` + addPath + `",
		"family":[` + families + `]
	}}}`}}
}

// bridgeDeclared answers the payload the candidate declares for every peer
// under code, and whether it declares one.
func bridgeDeclared(t *testing.T, sections []sdk.ConfigSection, code uint8) (string, bool) {
	t.Helper()
	declared, err := sdk.CandidateCapabilities(bridgeCapabilities, sections)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range declared.Capabilities {
		if decl.Code != code {
			continue
		}
		if len(decl.Peers) != 0 {
			t.Errorf("code %d is declared for %v, want every peer", code, decl.Peers)
		}
		return decl.Payload, true
	}
	return "", false
}

// TestBridgeCapabilityExtractorRegistered checks that the extractor the bridge
// registers answers, in the declaration the config-verify answer carries, a
// route-refresh a reload removes as gone and an add-path mode a reload changes
// as a new payload, each for every peer (AC-2f), and that the bridge's own
// verify still refuses a family edit, which its running scripts cannot follow.
// VALIDATES: bridgeCapabilities through sdk.CandidateCapabilities, and
// bridgeRunner.verifyReload.
// PREVENTS: a removed route-refresh still sent, a changed add-path mode never
// sent, or a family edit accepted under scripts that started with the old one.
func TestBridgeCapabilityExtractorRegistered(t *testing.T) {
	startup := bridgeReloadConfig("true", "both", `"ipv4/unicast"`)
	if _, ok := bridgeDeclared(t, startup, routeRefreshCapCode); !ok {
		t.Fatal("the startup bridge declares no route-refresh")
	}
	before, ok := bridgeDeclared(t, startup, addPathCapCode)
	if !ok {
		t.Fatal("the startup bridge declares no add-path")
	}

	if _, ok := bridgeDeclared(t, bridgeReloadConfig("false", "both", `"ipv4/unicast"`), routeRefreshCapCode); ok {
		t.Error("a reload that removes route-refresh still declares it")
	}
	after, ok := bridgeDeclared(t, bridgeReloadConfig("true", "send", `"ipv4/unicast"`), addPathCapCode)
	if !ok {
		t.Fatal("a changed add-path mode declares no add-path")
	}
	if after == before {
		t.Errorf("an add-path mode changed from both to send still declares %q", after)
	}

	running, err := parseConfig(startup[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	r := &bridgeRunner{config: running}
	if err := r.verifyReload(bridgeReloadConfig("false", "send", `"ipv4/unicast"`)); err != nil {
		t.Errorf("the bridge refused a change of its capability leaves alone: %v", err)
	}
	refusal := r.verifyReload(bridgeReloadConfig("true", "both", `"ipv4/unicast","ipv6/unicast"`))
	if refusal == nil {
		t.Fatal("the bridge accepted a family edit")
	}
	if !strings.Contains(refusal.Error(), "exabgp bridge family") {
		t.Errorf("the refusal %q does not name the family leaf", refusal)
	}
}
