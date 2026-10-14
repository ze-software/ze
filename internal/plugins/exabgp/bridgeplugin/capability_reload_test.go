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

// TestBridgeCapabilityExtractorRegistered checks that the extractor and the
// leaves the bridge registers make the SDK refuse a changed route-refresh and
// a family edit while add-path is set, each naming every peer and its leaf
// (AC-2f), and accept an unchanged bridge (AC-2e).
// VALIDATES: bridgeCapabilities and bridgeCapabilityLeaves through
// sdk.CheckCapabilityReload, the comparison the composed config-verify runs.
// PREVENTS: a family edit that changes the add-path payload accepted in
// silence, or a refusal naming a peer when the declaration is for every peer.
func TestBridgeCapabilityExtractorRegistered(t *testing.T) {
	check := func(candidate []sdk.ConfigSection) error {
		startup := bridgeReloadConfig("true", "both", `"ipv4/unicast"`)
		return sdk.CheckCapabilityReload("exabgp-bridge", bridgeCapabilities, bridgeCapabilityLeaves, startup, candidate)
	}

	if err := check(bridgeReloadConfig("true", "both", `"ipv4/unicast"`)); err != nil {
		t.Fatalf("an unchanged bridge was refused: %v", err)
	}

	cases := []struct {
		name      string
		candidate []sdk.ConfigSection
		want      string
	}{
		{
			name:      "route-refresh removed",
			candidate: bridgeReloadConfig("false", "both", `"ipv4/unicast"`),
			want:      "every peer: capability route-refresh cannot change while ze runs (plugin exabgp-bridge).",
		},
		{
			name:      "add-path mode changed",
			candidate: bridgeReloadConfig("true", "send", `"ipv4/unicast"`),
			want:      "every peer: capability add-path cannot change while ze runs (plugin exabgp-bridge).",
		},
		{
			name:      "family added while add-path is set",
			candidate: bridgeReloadConfig("true", "both", `"ipv4/unicast","ipv6/unicast"`),
			want:      "every peer: capability add-path cannot change while ze runs (plugin exabgp-bridge).",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := check(tc.candidate)
			if err == nil {
				t.Fatal("accepted, want a refusal")
			}
			if first, _, _ := strings.Cut(err.Error(), "\n"); first != tc.want {
				t.Errorf("refusal opens with %q, want %q", first, tc.want)
			}
		})
	}
}
