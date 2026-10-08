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
		`"ze-bgp:rib-`:         "BGP RIB queries -> internal/component/bgp/plugins/cmd/rib/yang",
		`"ze-rib-api:`:         "BGP RIB rpc pointers -> internal/component/bgp/plugins/cmd/rib/yang",
		`"ze-bgp:peer-`:        "BGP peer state -> internal/component/bgp/plugins/cmd/peer/yang",
		`"ze-bgp:show-decode"`: "offline BGP decode -> internal/component/bgp/cli/yang",
		`"ze-bgp:show-encode"`: "offline BGP encode -> internal/component/bgp/cli/yang",
		`"ze-bgp:show-bmp-`:    "BMP monitoring -> internal/component/bgp/plugins/bmp/yang",
		`"ze-bgp:show-rr-`:     "BGP route reflector -> internal/component/bgp/plugins/rr/yang",
		`"ze-bgp:show-health"`: "BGP health overview -> internal/component/bgp/plugins/cmd/peer/yang",
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
		`"ze-flowexport:show-flow-export"`:       "flow export -> internal/plugins/flowexport-cmd/yang",
		`"ze-rsvpte:`:                            "RSVP-TE -> internal/plugins/rsvpte/yang",
		`"ze-ldp:`:                               "LDP -> internal/plugins/ldp/yang",
		`"ze-isis:`:                              "IS-IS -> internal/plugins/isis/yang",
		`"ze-ospf:show-ospf`:                     "OSPF -> internal/plugins/ospf/yang",
		`"ze-policyroute:show-policy-routes"`:    "policy routing -> internal/plugins/policyroute/yang",
		`"ze-static:show-static"`:                "static routes -> internal/plugins/static/yang",
		`"ze-ike:show-vpn-ipsec-`:                "IPsec -> internal/component/ike/yang",
		`"ze-iface:show-vpp-`:                    "VPP dataplane -> internal/plugins/iface/vpp/yang",
		`"ze-iface:show-arp"`:                    "kernel IPv4 ARP read -> internal/component/iface/yang",
		`"ze-iface:show-neighbor"`:               "kernel neighbor read -> internal/component/iface/yang",
		`"ze-iface:show-route"`:                  "kernel FIB read -> internal/component/iface/yang",
		`"ze-iface:show-route-lookup"`:           "kernel FIB lookup -> internal/component/iface/yang",
		`"ze-ping:show-ping"`:                    "ICMP ping -> internal/plugins/ping-cmd/yang",
		`"ze-traceroute:show-traceroute"`:        "ICMP traceroute -> internal/plugins/traceroute-cmd/yang",
		`"ze-traceroute:show-probe-round"`:       "parallel probe round -> internal/plugins/traceroute-cmd/yang",
		`"ze-iface:show-interface"`:              "interface family -> internal/component/iface/yang",
		`"ze-iface:show-interface-brief"`:        "interface brief -> internal/component/iface/yang",
		`"ze-iface:show-interface-type"`:         "interface type filter -> internal/component/iface/yang",
		`"ze-iface:show-interface-errors"`:       "interface error counters -> internal/component/iface/yang",
		`"ze-iface:show-interface-rate"`:         "interface rate -> internal/component/iface/yang",
		`"ze-iface:show-interface-detail"`:       "interface detail -> internal/component/iface/yang",
		`"ze-iface:show-interface-counters"`:     "interface counters -> internal/component/iface/yang",
		`"ze-iface:show-interface-scan"`:         "interface scan -> internal/component/iface/yang",
		`"ze-traffic:show-traffic"`:              "traffic control (QoS) -> internal/plugins/traffic-cmd/yang",
		`"ze-resolve:show-dns-lookup"`:           "DNS lookup -> internal/plugins/resolve-cmd/yang",
		`"ze-resolve:show-dns-cache-`:            "DNS cache inspection -> internal/plugins/resolve-cmd/yang",
		`"ze-geodns:show-geodns"`:                "GeoDNS status -> internal/plugins/geodns/yang",
		`"ze-firewall:show-ruleset"`:             "firewall ruleset -> internal/component/firewall/yang",
		`"ze-firewall:show-group"`:               "firewall group -> internal/component/firewall/yang",
		`"ze-pki:show-certificates"`:             "PKI certificate list -> internal/plugins/pki-cmd/yang",
		`"ze-pki:show-certificate"`:              "PKI certificate detail -> internal/plugins/pki-cmd/yang",
		`"ze-l2tp:show-health"`:                  "L2TP health -> internal/component/l2tp/cmd/yang",
		`"ze-storage:show-smart"`:                "storage SMART -> internal/plugins/storage-cmd/yang",
		`"ze-gnmi:show-gnmi"`:                    "gNMI server status -> internal/plugins/gnmi-cmd/yang",
		`"ze-aaa:show-accounting"`:               "AAA accounting -> internal/plugins/aaa-cmd/yang",
		`"ze-mpls:show-forwarding"`:              "MPLS forwarding -> internal/plugins/mpls-cmd/yang",
		`"ze-ntp:show-system-ntp"`:               "NTP status -> internal/plugins/ntp/yang",
		`"ze-ntp:show-system-ntp-peers"`:         "NTP peers -> internal/plugins/ntp/yang",
		`"ze-firewall:show-system-conntrack"`:    "conntrack -> internal/component/firewall/yang",
		`"ze-update:show-system-update"`:         "system update -> internal/plugins/update-cmd/yang",
		`"ze-update:show-system-update-history"`: "update history -> internal/plugins/update-cmd/yang",
		`"ze-host:show-system-kernel-log"`:       "kernel log -> internal/plugins/host-cmd/yang",
		`"ze-host:`:                              "host inventory -> internal/plugins/host-cmd/yang",
		`"ze-crashes:show-crashes"`:              "crash reports -> internal/plugins/crashes/yang",
		`"ze-doctor:show-doctor"`:                "doctor checks -> internal/component/doctor/yang",
		`"ze-diag:show-capture"`:                 "packet capture -> internal/plugins/diag/yang",
		`"ze-vrrp:show-vrrp`:                     "VRRP virtual-router state -> internal/plugins/vrrp/yang",
		`"ze-diag:show-capture-raw"`:             "raw capture -> internal/plugins/diag/yang",
		`"ze-diag:show-capture-interface"`:       "interface capture -> internal/plugins/diag/yang",
		`"ze-diag:show-tcp-check"`:               "TCP check -> internal/plugins/diag/yang",
		`"ze-bgp:show-policy-chain"`:             "policy chain -> internal/component/bgp/plugins/cmd/policy/yang",
		`"ze-bgp:show-policy-test"`:              "policy test -> internal/component/bgp/plugins/cmd/policy/yang",
		`"ze-config-cli:`:                        "config inspection -> internal/plugins/config-cli/yang",
		`"ze-config-storage:`:                    "data store -> internal/plugins/config-storage/yang",
		`"ze-env:`:                               "env vars -> internal/plugins/env/yang",
		`"ze-config-schema:`:                     "schema introspection -> internal/plugins/config-schema/yang",
		`"ze-config-yang:`:                       "YANG tools -> internal/plugins/config-yang/yang",
	}
	for token, owner := range banned {
		if strings.Contains(ZeCliShowCmdYANG, token) {
			t.Errorf("central show schema declares owner command %q; move it to %s (see ai/rules/plugins.md)", token, owner)
		}
	}
}

// TestShowSchemaNamesNoOwnerCommand derives the owner check rather than
// listing owners.
//
// VALIDATES: every ze:command in the central show schemas carries the
// ze-cmd: prefix and every ze:rpc names this package's own API module, so a
// node an owner (BGP, OSPF, L2TP, ...) declares under its own prefix cannot
// sit here, whichever owner it is.
//
// PREVENTS: a show command drifting back into the central schema under an
// owner prefix that the hand-listed tokens above do not name.
func TestShowSchemaNamesNoOwnerCommand(t *testing.T) {
	commands, rpcs := 0, 0
	for name, text := range map[string]string{"ze-cli-show-cmd": ZeCliShowCmdYANG, "ze-cli-show-api": ZeCliShowAPIYANG} {
		bad, judged := foreignPrefixes(t, name, text, "ze:command", "ze-cmd")
		commands += judged
		for _, value := range bad {
			t.Errorf("%s declares owner command %q; it belongs in the owner's schema (see ai/rules/plugins.md)", name, value)
		}
		bad, judged = foreignPrefixes(t, name, text, "ze:rpc", "ze-cli-show-api")
		rpcs += judged
		for _, value := range bad {
			t.Errorf("%s points at owner rpc %q; it belongs in the owner's schema (see ai/rules/plugins.md)", name, value)
		}
	}
	// The central show schema holds the ze-cmd: commands, some carrying an
	// rpc pointer, so a run that judged none of either judged nothing.
	if commands == 0 {
		t.Error("the central show schemas carry no ze:command statement, so the check judged nothing")
	}
	if rpcs == 0 {
		t.Error("the central show schemas carry no ze:rpc statement, so the check judged nothing")
	}
}

// foreignPrefixes returns each `<extension> "<prefix>:..."` argument whose
// prefix is not own, and how many arguments it judged. It fails the test when
// the text carries an extension statement it could not read, so a spelling
// the scan does not know cannot pass unjudged.
func foreignPrefixes(t *testing.T, name, text, extension, own string) (foreign []string, judged int) {
	t.Helper()
	for line := range strings.Lines(text) {
		_, arg, found := strings.Cut(line, extension+` "`)
		if !found {
			continue
		}
		judged++
		value, _, _ := strings.Cut(arg, `"`)
		prefix, _, _ := strings.Cut(value, ":")
		if prefix != own {
			foreign = append(foreign, value)
		}
	}
	if present := statementCount(text, extension); present != judged {
		t.Errorf("%s carries %d %s statements but the scan judged %d", name, present, extension, judged)
	}
	return foreign, judged
}

// statementCount counts the keyword statements in text: each keyword followed
// by a YANG separator (RFC 7950 Section 14, sep = 1*(WSP / line-break)), so a
// statement whose argument follows a tab or a line break is counted as well
// as one whose argument follows a space.
func statementCount(text, keyword string) int {
	count := 0
	for rest := text; ; {
		at := strings.Index(rest, keyword)
		if at < 0 {
			return count
		}
		rest = rest[at+len(keyword):]
		if rest != "" && strings.ContainsRune(" \t\r\n", rune(rest[0])) {
			count++
		}
	}
}
