package schema

import (
	"strings"
	"testing"
)

// TestShowSchemaHasNoBGPPluginCommands enforces ai/rules/plugin-self-containment.md
// for the central `show` verb schema.
//
// VALIDATES: the central show schema declares no part of the `show bgp ...`
// subtree. BGP peer state and RIB queries are owned by the BGP plugin schemas
// (internal/component/bgp/plugins/cmd/{peer,rib}/schema); the offline
// decode/encode diagnostics are owned by internal/component/bgp/cli/schema next to their
// handlers. Removing the BGP surface must remove the whole `show bgp ...`
// branch with no dangling YANG node.
//
// PREVENTS: regression where any BGP command's schema drifts back into the
// central show package, which would leave a handler-less command node after
// the BGP surface is removed.
func TestShowSchemaHasNoBGPPluginCommands(t *testing.T) {
	// Command tokens owned by removable BGP packages; none may appear in the
	// central show schema.
	banned := map[string]string{
		"ze-rib-api:":          "BGP RIB queries -> internal/component/bgp/plugins/cmd/rib/schema",
		`"ze-bgp:peer-`:        "BGP peer state -> internal/component/bgp/plugins/cmd/peer/schema",
		`"ze-show:bgp-decode"`: "offline BGP decode -> internal/component/bgp/cli/schema",
		`"ze-show:bgp-encode"`: "offline BGP encode -> internal/component/bgp/cli/schema",
		`"ze-show:bmp-`:        "BMP monitoring -> internal/component/bgp/plugins/bmp/schema",
		`"ze-show:rr-`:         "BGP route reflector -> internal/component/bgp/plugins/rr/schema",
	}
	for token, owner := range banned {
		if strings.Contains(ZeCliShowCmdYANG, token) {
			t.Errorf("central show schema declares BGP-owned command %q; move it to %s (see ai/rules/plugin-self-containment.md)", token, owner)
		}
	}
}

// TestShowSchemaHasNoMigratedOwnerCommands enforces ai/rules/plugin-self-containment.md
// for non-BGP owners whose `show ...` subtree has been relocated to the owning
// component or plugin schema package.
//
// VALIDATES: the central show schema declares none of these owner-specific
// command nodes. Each owner's schema package re-declares the node via container
// merge and asserts its own presence (the owner half of the invariant).
//
// PREVENTS: regression where an owner command's schema drifts back into the
// central show package, which would leave a handler-less command node after the
// owner is removed.
func TestShowSchemaHasNoMigratedOwnerCommands(t *testing.T) {
	banned := map[string]string{
		`"ze-show:flow-export"`:        "flow export -> internal/component/flowexport/schema",
		`"ze-show:rsvp-te-`:            "RSVP-TE -> internal/component/rsvpte/schema",
		`"ze-show:ldp-`:                "LDP -> internal/component/ldp/schema",
		`"ze-show:policy-routes"`:      "policy routing -> internal/plugins/policyroute/schema",
		`"ze-show:static"`:             "static routes -> internal/plugins/static/schema",
		`"ze-show:vpn-ipsec-`:          "IPsec -> internal/component/ike/schema",
		`"ze-show:vpp-`:                "VPP dataplane -> internal/plugins/iface/vpp/schema",
		`"ze-show:ip-arp"`:             "kernel ARP/ND read -> internal/component/iface/schema",
		`"ze-show:ip-route"`:           "kernel FIB read -> internal/component/iface/schema",
		`"ze-show:route-lookup"`:       "kernel FIB lookup -> internal/component/iface/schema",
		`"ze-show:neighbors"`:          "kernel neighbor read -> internal/component/iface/schema",
		`"ze-show:kernel-routes"`:      "kernel FIB read -> internal/component/iface/schema",
		`"ze-show:ping"`:               "ICMP ping -> internal/component/ping/schema",
		`"ze-show:traceroute"`:         "ICMP traceroute -> internal/component/traceroute/schema",
		`"ze-show:probe-round"`:        "parallel probe round -> internal/component/traceroute/schema",
		`"ze-show:interface"`:          "interface family -> internal/component/iface/schema",
		`"ze-show:interface-detail"`:   "interface detail -> internal/component/iface/schema",
		`"ze-show:interface-counters"`: "interface counters -> internal/component/iface/schema",
		`"ze-show:interface-scan"`:     "interface scan -> internal/component/iface/schema",
		`"ze-show:traffic"`:            "traffic control (QoS) -> internal/component/traffic/schema",
		`"ze-show:dns-lookup"`:         "DNS lookup -> internal/component/resolve/schema",
		`"ze-show:dns-cache"`:          "DNS cache inspection -> internal/component/resolve/schema",
	}
	for token, owner := range banned {
		if strings.Contains(ZeCliShowCmdYANG, token) {
			t.Errorf("central show schema declares owner command %q; move it to %s (see ai/rules/plugin-self-containment.md)", token, owner)
		}
	}
}
