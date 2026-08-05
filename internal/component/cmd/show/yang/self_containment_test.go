package yang

import (
	"strings"
	"testing"
)

// TestShowSchemaHasNoBGPPluginCommands enforces ai/rules/plugins.md
// for the central `show` verb schema.
//
// VALIDATES: the central show schema declares no part of the `show bgp ...`
// subtree. BGP peer state and RIB queries are owned by the BGP plugin schemas
// (internal/component/bgp/plugins/cmd/{peer,rib}/schema); the offline
// decode/encode diagnostics are owned by internal/component/bgp/cli/yang next to their
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
		"ze-rib-api:":          "BGP RIB queries -> internal/component/bgp/plugins/cmd/rib/yang",
		`"ze-bgp:peer-`:        "BGP peer state -> internal/component/bgp/plugins/cmd/peer/yang",
		`"ze-show:bgp-decode"`: "offline BGP decode -> internal/component/bgp/cli/yang",
		`"ze-show:bgp-encode"`: "offline BGP encode -> internal/component/bgp/cli/yang",
		`"ze-show:bmp-`:        "BMP monitoring -> internal/component/bgp/plugins/bmp/yang",
		`"ze-show:rr-`:         "BGP route reflector -> internal/component/bgp/plugins/rr/yang",
		`"ze-show:bgp-health"`: "BGP health overview -> internal/component/bgp/plugins/cmd/peer/yang",
	}
	for token, owner := range banned {
		if strings.Contains(ZeCliShowCmdYANG, token) {
			t.Errorf("central show schema declares BGP-owned command %q; move it to %s (see ai/rules/plugins.md)", token, owner)
		}
	}
}

// TestShowSchemaHasNoMigratedOwnerCommands enforces ai/rules/plugins.md
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
		`"ze-show:flow-export"`:           "flow export -> internal/plugins/flowexport-cmd/yang",
		`"ze-show:rsvp-te-`:               "RSVP-TE -> internal/plugins/rsvpte/yang",
		`"ze-show:ldp-`:                   "LDP -> internal/plugins/ldp/yang",
		`"ze-show:isis-`:                  "IS-IS -> internal/plugins/isis/yang",
		`"ze-show:ospf`:                   "OSPF -> internal/plugins/ospf/yang",
		`"ze-show:policy-routes"`:         "policy routing -> internal/plugins/policyroute/yang",
		`"ze-show:static"`:                "static routes -> internal/plugins/static/yang",
		`"ze-show:vpn-ipsec-`:             "IPsec -> internal/component/ike/yang",
		`"ze-show:vpp-`:                   "VPP dataplane -> internal/plugins/iface/vpp/yang",
		`"ze-show:arp"`:                   "kernel IPv4 ARP read -> internal/component/iface/yang",
		`"ze-show:neighbor"`:              "kernel neighbor read -> internal/component/iface/yang",
		`"ze-show:route"`:                 "kernel FIB read -> internal/component/iface/yang",
		`"ze-show:route-lookup"`:          "kernel FIB lookup -> internal/component/iface/yang",
		`"ze-show:ping"`:                  "ICMP ping -> internal/plugins/ping-cmd/yang",
		`"ze-show:traceroute"`:            "ICMP traceroute -> internal/plugins/traceroute-cmd/yang",
		`"ze-show:probe-round"`:           "parallel probe round -> internal/plugins/traceroute-cmd/yang",
		`"ze-show:interface"`:             "interface family -> internal/component/iface/yang",
		`"ze-show:interface-brief"`:       "interface brief -> internal/component/iface/yang",
		`"ze-show:interface-type"`:        "interface type filter -> internal/component/iface/yang",
		`"ze-show:interface-errors"`:      "interface error counters -> internal/component/iface/yang",
		`"ze-show:interface-rate"`:        "interface rate -> internal/component/iface/yang",
		`"ze-show:interface-detail"`:      "interface detail -> internal/component/iface/yang",
		`"ze-show:interface-counters"`:    "interface counters -> internal/component/iface/yang",
		`"ze-show:interface-scan"`:        "interface scan -> internal/component/iface/yang",
		`"ze-show:traffic"`:               "traffic control (QoS) -> internal/plugins/traffic-cmd/yang",
		`"ze-show:dns-lookup"`:            "DNS lookup -> internal/plugins/resolve-cmd/yang",
		`"ze-show:dns-cache"`:             "DNS cache inspection -> internal/plugins/resolve-cmd/yang",
		`"ze-show:geodns"`:                "GeoDNS status -> internal/plugins/geodns/yang",
		`"ze-show:firewall-ruleset"`:      "firewall ruleset -> internal/component/firewall/yang",
		`"ze-show:firewall-group"`:        "firewall group -> internal/component/firewall/yang",
		`"ze-show:pki-certificates"`:      "PKI certificate list -> internal/plugins/pki-cmd/yang",
		`"ze-show:pki-certificate"`:       "PKI certificate detail -> internal/plugins/pki-cmd/yang",
		`"ze-show:l2tp-health"`:           "L2TP health -> internal/component/l2tp/cmd/yang",
		`"ze-show:storage-smart"`:         "storage SMART -> internal/plugins/storage-cmd/yang",
		`"ze-show:gnmi"`:                  "gNMI server status -> internal/plugins/gnmi-cmd/yang",
		`"ze-show:aaa-accounting"`:        "AAA accounting -> internal/plugins/aaa-cmd/yang",
		`"ze-show:mpls-forwarding"`:       "MPLS forwarding -> internal/plugins/mpls-cmd/yang",
		`"ze-show:system-ntp"`:            "NTP status -> internal/plugins/ntp/yang",
		`"ze-show:system-ntp-peers"`:      "NTP peers -> internal/plugins/ntp/yang",
		`"ze-show:system-conntrack"`:      "conntrack -> internal/component/firewall/yang",
		`"ze-show:system-update"`:         "system update -> internal/plugins/update-cmd/yang",
		`"ze-show:system-update-history"`: "update history -> internal/plugins/update-cmd/yang",
		`"ze-show:system-kernel-log"`:     "kernel log -> internal/plugins/host-cmd/yang",
		`"ze-show:host-`:                  "host inventory -> internal/plugins/host-cmd/yang",
		`"ze-show:crashes"`:               "crash reports -> internal/plugins/crashes/yang",
		`"ze-show:doctor"`:                "doctor checks -> internal/component/doctor/yang",
		`"ze-show:capture"`:               "packet capture -> internal/plugins/diag/yang",
		`"ze-show:vrrp`:                   "VRRP virtual-router state -> internal/plugins/vrrp/yang",
		`"ze-show:capture-raw"`:           "raw capture -> internal/plugins/diag/yang",
		`"ze-show:capture-interface"`:     "interface capture -> internal/plugins/diag/yang",
		`"ze-show:tcp-check"`:             "TCP check -> internal/plugins/diag/yang",
		`"ze-show:policy-chain"`:          "policy chain -> internal/component/bgp/plugins/cmd/policy/yang",
		`"ze-show:policy-test"`:           "policy test -> internal/component/bgp/plugins/cmd/policy/yang",
		`"ze-show:config-`:                "config inspection -> internal/plugins/config-cli/yang",
		`"ze-show:data-`:                  "data store -> internal/plugins/config-storage/yang",
		`"ze-show:env-`:                   "env vars -> internal/plugins/env/yang",
		`"ze-show:schema-`:                "schema introspection -> internal/plugins/config-schema/yang",
		`"ze-show:yang-`:                  "YANG tools -> internal/plugins/config-yang/yang",
	}
	for token, owner := range banned {
		if strings.Contains(ZeCliShowCmdYANG, token) {
			t.Errorf("central show schema declares owner command %q; move it to %s (see ai/rules/plugins.md)", token, owner)
		}
	}
}
