// Design: docs/architecture/testing/interop.md -- typed scenario registry.
// Related: check_engine.go -- operation semantics and fail-closed assertions.
package bgp

import (
	"context"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// checkers returns the complete typed checker registry for test/interop/scenarios.
func checkers() map[string]interoplab.Checker {
	checkers := make(map[string]interoplab.Checker, len(scenarioOperations))
	for name := range scenarioOperations {
		scenarioName := name
		checkers[name] = func(ctx context.Context, check *interoplab.CheckContext) error {
			return checkScenario(ctx, check, scenarioName)
		}
	}
	for name, checker := range specialCheckers {
		scenarioName := name
		scenarioChecker := checker
		checkers[name] = func(ctx context.Context, check *interoplab.CheckContext) error {
			if err := scenarioChecker(ctx, check); err != nil {
				return checkerFailure(ctx, check.Lab, scenarioName, 0, err)
			}
			return nil
		}
	}
	return checkers
}

var scenarioOperations = map[string][]operation{
	"as112-community-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "192.175.48.0/24"},
		{kind: opFRRRoute, argument: "192.31.196.0/24"},
		{kind: opFRRCommunity, argument: "192.175.48.0/24", contains: []string{"no-export"}},
		{kind: opFRRCommunity, argument: "192.31.196.0/24", contains: []string{"no-peer"}},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"as112-origin-as-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opBIRDSession, argument: "ze_peer"},
		{kind: opFRRRoute, argument: "192.175.48.0/24"},
		{kind: opBIRDRoute, argument: "192.175.48.0/24"},
		{kind: opFRRRoute, argument: "192.31.196.0/24"},
		{kind: opBIRDRoute, argument: "192.31.196.0/24"},
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opBIRDSession, argument: "ze_peer"},
	},
	"as112-redistribute-community-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "192.175.48.0/24"},
		{kind: opFRRRoute, argument: "192.31.196.0/24"},
		{kind: opFRRCommunity, argument: "192.175.48.0/24", contains: []string{"no-peer"}},
		{kind: opFRRCommunity, argument: "192.31.196.0/24", contains: []string{"no-peer"}},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"as112-redistribute-lab": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "192.175.48.0/24"},
		{kind: opFRRRoute, argument: "192.31.196.0/24"},
		{kind: opFRRRouteAbsent, argument: "192.175.48.1/32"},
		{kind: opFRRRouteAbsent, argument: "192.31.196.1/32"},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"as112-redistribute-origin-custom-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "192.175.48.0/24"},
		{kind: opFRRRoute, argument: "192.31.196.0/24"},
		{kind: opFRRRouteAbsent, argument: "192.175.48.1/32"},
		{kind: opFRRRouteAbsent, argument: "192.31.196.1/32"},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"as112-redistribute-origin-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opBIRDSession, argument: "ze_peer"},
		{kind: opFRRRoute, argument: "192.175.48.0/24"},
		{kind: opBIRDRoute, argument: "192.175.48.0/24"},
		{kind: opFRRRoute, argument: "192.31.196.0/24"},
		{kind: opBIRDRoute, argument: "192.31.196.0/24"},
		{kind: opFRRRouteAbsent, argument: "192.175.48.1/32"},
		{kind: opFRRRouteAbsent, argument: "192.31.196.1/32"},
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opBIRDSession, argument: "ze_peer"},
	},
	"bgp-4byte-asn-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-addpath-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "10.10.0.0/24"},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-community-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "10.10.0.0/24"},
		{kind: opFRRRoute, argument: "10.10.1.0/24"},
		{kind: opFRRCommunity, argument: "10.10.0.0/24", contains: []string{"65001:100"}},
		{kind: opFRRCommunity, argument: "10.10.0.0/24", contains: []string{"65001:200"}},
		{kind: opFRRCommunity, argument: "10.10.1.0/24", contains: []string{"65001:0:1"}},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-ebgp-gobgp": {
		{kind: opGoBGPSession, argument: "172.30.0.2"},
	},
	"bgp-ebgp-ipv4-bird": {
		{kind: opBIRDSession, argument: "ze_peer"},
	},
	"bgp-ebgp-ipv4-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-ecmp-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRSession, argument: "172.30.0.5"},
		{kind: opExec, peer: "gobgp", command: []string{"gobgp", "global", "rib", "add", "10.100.0.0/24", "-a", "ipv4", "nexthop", "172.30.0.5"}},
		{kind: opFRRRoute, argument: "10.100.0.0/24", timeout: 30 * time.Second},
	},
	"bgp-evpn-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-evpn-gobgp": {
		{kind: opGoBGPSession, argument: "172.30.0.2"},
	},
	"bgp-extended-community-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "10.10.0.0/24"},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-flowspec-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-flowspec-gobgp": {
		{kind: opGoBGPSession, argument: "172.30.0.2"},
	},
	"bgp-flowspec-sctp-gobgp": {
		{kind: opGoBGPSession, argument: "172.30.0.2"},
		{kind: opExec, peer: "gobgp", command: []string{"gobgp", "global", "rib", "-a", "ipv4-flowspec", "add", "match", "destination", "10.99.5.0/24", "protocol", "==sctp", "then", "discard"}},
		{kind: opWaitContains, peer: "ze", command: []string{"nft", "list", "ruleset"}, contains: []string{"sctp", "10.99.5.0/24"}, timeout: 30 * time.Second},
		{kind: opExec, peer: "gobgp", command: []string{"gobgp", "global", "rib", "-a", "ipv4-flowspec", "del", "match", "destination", "10.99.5.0/24", "protocol", "==sctp", "then", "discard"}},
		{kind: opWaitAbsent, peer: "ze", command: []string{"nft", "list", "ruleset"}, absent: []string{"10.99.5.0/24"}, proof: []string{"table"}, timeout: 30 * time.Second},
		{kind: opGoBGPSession, argument: "172.30.0.2"},
	},
	"bgp-graceful-restart-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "10.10.0.0/24"},
		{kind: opRequireJSONFields, peer: "ze", command: []string{"ze", "cli", "-c", "show bgp rib status", "--user", "interop", "--format", "json"}, minimum: map[string]int{"routes-in": 1}},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-gtsm-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-ibgp-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-ipv6-ebgp-bird": {
		{kind: opBIRDSession, argument: "ze_peer"},
		{kind: opBIRDRoute, argument: "2001:db8:1::/48"},
		{kind: opBIRDRoute, argument: "2001:db8:2::/48"},
		{kind: opBIRDRoute, argument: "2001:db8:3::/48"},
		{kind: opBIRDSession, argument: "ze_peer"},
	},
	"bgp-ipv6-ebgp-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "2001:db8:1::/48", family: "ipv6 unicast"},
		{kind: opFRRRoute, argument: "2001:db8:2::/48", family: "ipv6 unicast"},
		{kind: opFRRRoute, argument: "2001:db8:3::/48", family: "ipv6 unicast"},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-ipv6-ebgp-gobgp": {
		{kind: opGoBGPSession, argument: "172.30.0.2"},
		{kind: opGoBGPRoute, argument: "2001:db8:1::/48", family: "ipv6 unicast"},
		{kind: opGoBGPRoute, argument: "2001:db8:2::/48", family: "ipv6 unicast"},
		{kind: opGoBGPRoute, argument: "2001:db8:3::/48", family: "ipv6 unicast"},
		{kind: opGoBGPSession, argument: "172.30.0.2"},
	},
	"bgp-max-prefix-cease-frr": {
		{kind: opWaitLogContains, peer: "ze", contains: []string{"prefix count exceeded maximum"}, timeout: 90 * time.Second},
		{kind: opWaitAbsent, peer: "frr", command: []string{"vtysh", "-c", "show bgp neighbor 172.30.0.2"}, absent: []string{"BGP state = Established"}, proof: []string{"BGP neighbor is"}, timeout: 30 * time.Second},
		{kind: opExec, peer: "frr", command: []string{"vtysh", "-c", "configure terminal", "-c", "no ip route 10.45.1.0/24 Null0"}},
		{kind: opFRRSession, argument: "172.30.0.2", timeout: 60 * time.Second},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-md5-auth-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-med-ibgp-post-selection-removal-gobgp": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opGoBGPSession, argument: "172.30.0.2"},
		{kind: opGoBGPRoute, argument: "10.62.0.0/24"},
	},
	"bgp-multihop-ebgp-bird": {
		{kind: opBIRDSession, argument: "ze_peer"},
		{kind: opBIRDRoute, argument: "10.10.0.0/24"},
		{kind: opBIRDRoute, argument: "10.10.1.0/24"},
		{kind: opBIRDSession, argument: "ze_peer"},
	},
	"bgp-multihop-ebgp-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "10.10.0.0/24"},
		{kind: opFRRRoute, argument: "10.10.1.0/24"},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-multihop-ebgp-gobgp": {
		{kind: opGoBGPSession, argument: "172.30.0.2"},
		{kind: opGoBGPRoute, argument: "10.10.0.0/24"},
		{kind: opGoBGPRoute, argument: "10.10.1.0/24"},
		{kind: opGoBGPSession, argument: "172.30.0.2"},
	},
	"bgp-paths-limit-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "10.10.0.0/24"},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-policy-import-export-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opBIRDSession, argument: "ze_policy"},
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opBIRDSession, argument: "ze_policy"},
	},
	"bgp-redist-late-join-dynamic-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "10.99.0.0/24"},
	},
	"bgp-relay-withdraw-nexthop-self-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "10.10.0.0/24", timeout: 60 * time.Second},
		{kind: opFRRRouteAbsent, argument: "10.10.0.0/24", timeout: 90 * time.Second},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-remove-private-as-as4path-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opBIRDSession, argument: "ze_peer"},
		{kind: opExec, peer: "bird", command: []string{"birdc", "enable static_routes"}},
		{kind: opFRRRoute, argument: "10.99.1.0/24"},
		{kind: opFRRNoAS, argument: "10.99.1.0/24", absent: []string{"4200000000"}},
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opBIRDSession, argument: "ze_peer"},
	},
	"bgp-remove-private-as-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opGoBGPSession, argument: "172.30.0.2"},
		{kind: opExec, peer: "gobgp", command: []string{"gobgp", "global", "rib", "add", "10.99.0.0/24", "-a", "ipv4", "nexthop", "172.30.0.5"}},
		{kind: opFRRRoute, argument: "10.99.0.0/24"},
		{kind: opFRRNoAS, argument: "10.99.0.0/24", absent: []string{"64512"}},
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opGoBGPSession, argument: "172.30.0.2"},
	},
	"bgp-rfc7606-speaker-dup-attr": {
		{kind: opWaitLogFields, peer: "speaker", timeout: 120 * time.Second, fields: map[string]string{"established": "yes", "result": "PASS"}, minimum: map[string]int{"route-bearing-updates": 1}},
	},
	"bgp-role-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "10.99.0.0/24", timeout: 30 * time.Second},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-role-gobgp": {
		{kind: opGoBGPSession, argument: "172.30.0.2"},
		{kind: opDelayRequireContains, peer: "gobgp", command: []string{"gobgp", "neighbor", "172.30.0.2"}, contains: []string{"Established"}, delay: 5 * time.Second},
	},
	"bgp-route-reflection-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opBIRDSession, argument: "ze_rr"},
		{kind: opBIRDRoute, argument: "10.38.0.0/24"},
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opBIRDSession, argument: "ze_rr"},
	},
	"bgp-route-refresh-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "10.10.0.0/24"},
		{kind: opFRRRoute, argument: "10.10.1.0/24"},
		{kind: opFRRRoute, argument: "10.10.2.0/24"},
		{kind: opFRRRoute, argument: "10.10.0.0/24"},
		{kind: opFRRRoute, argument: "10.10.1.0/24"},
		{kind: opFRRRoute, argument: "10.10.2.0/24"},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-route-withdrawal-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "10.10.0.0/24"},
		{kind: opFRRRoute, argument: "10.10.1.0/24"},
		{kind: opFRRRoute, argument: "10.10.2.0/24"},
		{kind: opFRRRoute, argument: "10.10.0.0/24"},
		{kind: opFRRRoute, argument: "10.10.1.0/24"},
		{kind: opFRRRoute, argument: "10.10.2.0/24"},
		{kind: opFRRRouteAbsent, argument: "10.10.1.0/24", timeout: 30 * time.Second},
		{kind: opFRRRouteAbsent, argument: "10.10.1.0/24"},
		{kind: opFRRRoute, argument: "10.10.0.0/24"},
		{kind: opFRRRoute, argument: "10.10.2.0/24"},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-routes-from-bird": {
		{kind: opBIRDSession, argument: "ze_peer"},
		{kind: opBIRDRoute, argument: "10.0.0.0/24"},
		{kind: opBIRDRoute, argument: "10.0.1.0/24"},
		{kind: opBIRDRoute, argument: "10.0.2.0/24"},
		{kind: opRequireJSONFields, peer: "ze", command: []string{"ze", "cli", "-c", "show bgp rib status", "--user", "interop", "--format", "json"}, minimum: map[string]int{"routes-in": 3}},
		{kind: opBIRDSession, argument: "ze_peer"},
	},
	"bgp-routes-from-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "10.0.0.0/24"},
		{kind: opFRRRoute, argument: "10.0.1.0/24"},
		{kind: opFRRRoute, argument: "10.0.2.0/24"},
		{kind: opRequireJSONFields, peer: "ze", command: []string{"ze", "cli", "-c", "show bgp rib status", "--user", "interop", "--format", "json"}, minimum: map[string]int{"routes-in": 3}},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-routes-gobgp": {
		{kind: opGoBGPSession, argument: "172.30.0.2"},
		{kind: opGoBGPRoute, argument: "10.10.0.0/24"},
		{kind: opGoBGPRoute, argument: "10.10.1.0/24"},
		{kind: opExec, peer: "gobgp", command: []string{"gobgp", "global", "rib", "add", "10.20.0.0/24", "-a", "ipv4", "nexthop", "172.30.0.5"}},
		{kind: opExec, peer: "gobgp", command: []string{"gobgp", "global", "rib", "add", "10.20.1.0/24", "-a", "ipv4", "nexthop", "172.30.0.5"}},
		{kind: opRequireJSONFields, peer: "ze", command: []string{"ze", "cli", "-c", "show bgp rib status", "--user", "interop", "--format", "json"}, minimum: map[string]int{"routes-in": 2}},
		{kind: opGoBGPSession, argument: "172.30.0.2"},
	},
	"bgp-routes-to-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "10.10.0.0/24"},
		{kind: opFRRRoute, argument: "10.10.1.0/24"},
		{kind: opFRRRoute, argument: "10.10.2.0/24"},
	},
	"bgp-send-community-suppress-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opBIRDSession, argument: "ze_peer"},
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opBIRDSession, argument: "ze_peer"},
	},
	"bgp-speaker-two-instance": {
		{kind: opWaitLogFields, peer: "speaker", timeout: 90 * time.Second, fields: map[string]string{"established": "yes"}},
		{kind: opWaitLogFields, peer: "speaker2", timeout: 90 * time.Second, fields: map[string]string{"established": "yes"}},
	},
	"bgp-srv6-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-triangle": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRSession, argument: "172.30.0.4"},
		{kind: opBIRDSession, argument: "ze_peer"},
		{kind: opBIRDSession, argument: "frr_peer"},
		{kind: opBIRDRoute, argument: "10.99.0.0/24"},
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRSession, argument: "172.30.0.4"},
		{kind: opBIRDSession, argument: "ze_peer"},
	},
	"bgp-vpn-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "10.99.0.0/24", family: "ipv4 vpn"},
		{kind: opFRRRoute, argument: "10.99.1.0/24", family: "ipv4 vpn"},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-vpn-gobgp": {
		{kind: opGoBGPSession, argument: "172.30.0.2"},
	},
	"bmp-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "10.44.0.0/24"},
	},
	"isis-auth-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show isis neighbor"}, contains: []string{"Up"}, timeout: 90 * time.Second},
	},
	"isis-convergence-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show isis neighbor"}, contains: []string{"Up"}, timeout: 90 * time.Second},
		{kind: opExec, peer: "frr", command: []string{"ip", "link", "set", "eth0", "down"}},
		{kind: opWaitAbsent, peer: "frr", command: []string{"vtysh", "-c", "show isis neighbor"}, absent: []string{"Up"}, proof: []string{"System"}, timeout: 30 * time.Second},
		{kind: opExec, peer: "frr", command: []string{"ip", "link", "set", "eth0", "up"}},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show isis neighbor"}, contains: []string{"Up"}, timeout: 90 * time.Second},
	},
	"isis-dualstack-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show isis neighbor"}, contains: []string{"Up"}, timeout: 90 * time.Second},
	},
	"isis-lan-dis-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show isis neighbor"}, contains: []string{"Up"}, timeout: 90 * time.Second},
	},
	"isis-redist-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show isis neighbor"}, contains: []string{"Up"}, timeout: 60 * time.Second},
	},
	"ospf-auth-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospf-bfd-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show bfd peers"}, contains: []string{"172.30.0.2", "Status: up"}, timeout: 60 * time.Second},
		{kind: opExec, peer: "frr", command: []string{"ip", "link", "set", "eth0", "down"}},
		{kind: opWaitAbsent, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, absent: []string{"Full"}, proof: []string{"Neighbor"}, timeout: 15 * time.Second},
		{kind: opExec, peer: "frr", command: []string{"ip", "link", "set", "eth0", "up"}},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show bfd peers"}, contains: []string{"172.30.0.2", "Status: up"}, timeout: 60 * time.Second},
	},
	"ospf-broadcast-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospf-convergence-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
		{kind: opExec, peer: "frr", command: []string{"ip", "link", "set", "eth0", "down"}},
		{kind: opWaitAbsent, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, absent: []string{"Full"}, proof: []string{"Neighbor"}, timeout: 30 * time.Second},
		{kind: opExec, peer: "frr", command: []string{"ip", "link", "set", "eth0", "up"}},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospf-debug-inject-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospf-debug-te-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospf-ext-prefix-link-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospf-gr-fib-retention": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospf-gr-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospf-ipsec-ah-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
		{kind: opRequireContains, peer: "ze", command: []string{"ip", "-6", "xfrm", "state"}, contains: []string{"proto ah", "mode transport", "auth-trunc hmac(sha256)"}},
		{kind: opRequireContains, peer: "frr", command: []string{"ip", "-6", "xfrm", "state"}, contains: []string{"proto ah", "mode transport", "auth-trunc hmac(sha256)"}},
		{kind: opRequireAbsent, peer: "ze", command: []string{"ip", "-6", "xfrm", "state"}, absent: []string{" enc "}, proof: []string{"proto ah", "mode transport"}},
		{kind: opRequireContains, peer: "ze", command: []string{"ip", "-6", "xfrm", "policy"}, contains: []string{"dir out", "proto ospf", "tmpl", "proto ah", "mode transport"}},
	},
	"ospf-ipsec-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
		{kind: opRequireContains, peer: "ze", command: []string{"ip", "-6", "xfrm", "state"}, contains: []string{"proto esp", "mode transport", "auth-trunc hmac(sha256)"}},
		{kind: opRequireContains, peer: "frr", command: []string{"ip", "-6", "xfrm", "state"}, contains: []string{"proto esp", "mode transport", "auth-trunc hmac(sha256)"}},
		{kind: opRequireContains, peer: "ze", command: []string{"ip", "-6", "xfrm", "policy"}, contains: []string{"dir out", "proto ospf", "tmpl", "proto esp", "mode transport"}},
	},
	"ospf-ldp-sync-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospf-multiaf-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospf-multiaf-v4-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}},
	},
	"ospf-multiarea-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospf-multiinstance-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 120 * time.Second},
	},
	"ospf-nbma-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 120 * time.Second},
	},
	"ospf-opaque-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospf-p2p-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospf-ptmp-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospf-ri-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospf-sr-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospf-te-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospf-te-interas-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospf-virtual-link-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip route ospf"}, contains: []string{"192.0.2.0/24"}, timeout: 60 * time.Second},
	},
	"ospfv3-bfd-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show bfd peers"}, contains: []string{"Status: up"}, timeout: 60 * time.Second},
		{kind: opExec, peer: "frr", command: []string{"ip", "link", "set", "eth0", "down"}},
		{kind: opWaitAbsent, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, absent: []string{"Full"}, proof: []string{"Neighbor"}, timeout: 15 * time.Second},
		{kind: opExec, peer: "frr", command: []string{"ip", "link", "set", "eth0", "up"}},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show bfd peers"}, contains: []string{"Status: up"}, timeout: 60 * time.Second},
	},
	"ospfv3-broadcast-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospfv3-debug-decode-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospfv3-debug-inject-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospfv3-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospfv3-gr-fib-retention": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospfv3-gr-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospfv3-multiarea-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 route ospf6"}, contains: []string{"2001:db8:a1::/64"}, timeout: 90 * time.Second},
	},
	"ospfv3-nbma-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}},
	},
	"ospfv3-nssa-redist-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
		{kind: opGoBGPSession, argument: "172.30.0.2"},
		{kind: opExec, peer: "gobgp", command: []string{"gobgp", "global", "rib", "add", "2001:db8:7e5::/48", "-a", "ipv6", "nexthop", "fd00:1e:0::5"}},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 route ospf6"}, contains: []string{"2001:db8:7e5::/48"}, timeout: 90 * time.Second},
	},
	"ospfv3-ptmp-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 120 * time.Second},
	},
	"ospfv3-redist-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
		{kind: opGoBGPSession, argument: "172.30.0.2"},
		{kind: opExec, peer: "gobgp", command: []string{"gobgp", "global", "rib", "add", "2001:db8:5e5::/48", "-a", "ipv6", "nexthop", "fd00:1e:0::5"}},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 route ospf6"}, contains: []string{"2001:db8:5e5::/48"}, timeout: 90 * time.Second},
	},
	"ospfv3-ri-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
	},
	"ospfv3-sr-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 120 * time.Second},
	},
	"ospfv3-stub-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 90 * time.Second},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 route ospf6"}, contains: []string{"::/0"}, timeout: 90 * time.Second},
	},
	"ospfv3-vlink-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}},
	},
	"rpki-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"rtr-stayrtr": {
		{kind: opWaitContains, peer: "stayrtr", command: []string{"wget", "-q", "-O", "-", "http://127.0.0.1:9847/rpki.json"}, contains: []string{"prefix"}},
	},
	"shutdown-cease-frr": {
		{kind: opFRRSession, argument: "172.30.0.2"},
		{kind: opFRRRoute, argument: "10.10.0.0/24", timeout: 60 * time.Second},
		{kind: opFRRRoute, argument: "10.10.0.0/24"},
		{kind: opSignal, peer: "ze", argument: "TERM"},
		{kind: opFRRRouteAbsent, argument: "10.10.0.0/24", timeout: 30 * time.Second},
	},
	"vrrp-mastership-keepalived": {
		{kind: opWaitContains, peer: "ze", command: []string{"ip", "-o", "-f", "inet", "addr"}, contains: []string{"172.30.0.100"}, timeout: 40 * time.Second},
		{kind: opRequireAbsent, peer: "keepalived", command: []string{"ip", "-o", "-f", "inet", "addr"}, absent: []string{"172.30.0.100"}, proof: []string{"eth0"}},
		{kind: opSignal, peer: "ze", argument: "TERM"},
		{kind: opWaitContains, peer: "keepalived", command: []string{"ip", "-o", "-f", "inet", "addr"}, contains: []string{"172.30.0.100"}, timeout: 12 * time.Second},
		{kind: opStart, peer: "ze"},
		{kind: opWaitContains, peer: "ze", command: []string{"ip", "-o", "-f", "inet", "addr"}, contains: []string{"172.30.0.100"}, timeout: 40 * time.Second},
		{kind: opRequireAbsent, peer: "keepalived", command: []string{"ip", "-o", "-f", "inet", "addr"}, absent: []string{"172.30.0.100"}, proof: []string{"eth0"}},
	},
}
