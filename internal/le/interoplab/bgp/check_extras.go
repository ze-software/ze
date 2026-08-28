// Design: docs/architecture/testing/interop.md -- scenario-specific peer evidence.
// Related: checkers.go -- common session, route, and adjacency assertions.
package bgp

import (
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var scenarioExtras = map[string][]operation{
	"as112-origin-as-frr": {
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show bgp ipv4 unicast 192.175.48.0/24 json"}, contains: []string{"192.175.48.0/24", "112"}},
		{kind: opRequireContains, peer: "bird", command: []string{"birdc", "show route for 192.175.48.0/24 all"}, contains: []string{"192.175.48.0/24", "65001", "112"}},
	},
	"as112-redistribute-lab": {
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show bgp ipv4 unicast 192.175.48.0/24 json"}, contains: []string{"192.175.48.0/24", "65001", "112"}},
	},
	"as112-redistribute-origin-custom-frr": {
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show bgp ipv4 unicast 192.175.48.0/24 json"}, contains: []string{"192.175.48.0/24", "65001", "112"}},
	},
	"as112-redistribute-origin-frr": {
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show bgp ipv4 unicast 192.175.48.0/24 json"}, contains: []string{"192.175.48.0/24", "112"}},
		{kind: opRequireContains, peer: "bird", command: []string{"birdc", "show route for 192.175.48.0/24 all"}, contains: []string{"192.175.48.0/24", "65001", "112"}},
	},
	"bgp-4byte-asn-frr": {
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show bgp neighbor 172.30.0.2 json"}, contains: []string{"4200000000", "Established"}},
	},
	"bgp-addpath-frr": {
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show bgp ipv4 unicast 10.10.0.0/24"}, contains: []string{"10.10.0.0/24", "2 paths"}},
	},
	"bgp-ecmp-frr": {
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show bgp ipv4 unicast 10.100.0.0/24 json"}, contains: []string{"10.100.0.0/24", "172.30.0.2", "172.30.0.5"}},
	},
	"bgp-evpn-frr": {
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show bgp neighbor 172.30.0.2 json"}, contains: []string{"l2VpnEvpn", "advertisedAndReceived"}},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show bgp l2vpn evpn"}, contains: []string{"00:11:22:33:44:55", "00:11:22:33:44:66"}, timeout: 30 * time.Second},
	},
	"bgp-evpn-gobgp": {
		{kind: opWaitContains, peer: "gobgp", command: []string{"gobgp", "global", "rib", "-a", "evpn", "-j"}, contains: []string{"00:11:22:33:44:55", "00:11:22:33:44:66"}, timeout: 30 * time.Second},
	},
	"bgp-flowspec-frr": {
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show bgp neighbor 172.30.0.2 json"}, contains: []string{"ipv4Flowspec", "advertisedAndReceived"}},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show bgp ipv4 flowspec json"}, contains: []string{"10.99.0.0/24", "10.99.1.0/24"}, timeout: 30 * time.Second},
	},
	"bgp-flowspec-gobgp": {
		{kind: opWaitContains, peer: "gobgp", command: []string{"gobgp", "global", "rib", "-a", "ipv4-flowspec", "-j"}, contains: []string{"10.99.0.0/24", "10.99.1.0/24"}, timeout: 30 * time.Second},
	},
	"bgp-graceful-restart-frr": {
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show bgp neighbor 172.30.0.2 json"}, contains: []string{"gracefulRestart", "advertisedAndReceived"}},
	},
	"bgp-med-ibgp-post-selection-removal-gobgp": {
		{kind: opWaitLogContains, peer: "ze", contains: []string{"RAW-MED-DROP: removed MULTI_EXIT_DISC"}, timeout: 120 * time.Second},
		{kind: opRequireContains, peer: "gobgp", command: []string{"gobgp", "global", "rib", "-a", "ipv4", "10.62.0.0/24"}, contains: []string{"10.62.0.0/24", "65005", "172.30.0.2", "Med: 100"}},
	},
	"bgp-policy-import-export-frr": {
		{kind: opRequireContains, peer: "bird", command: []string{"birdc", "show route for 10.39.1.0/24 all"}, contains: []string{"10.39.1.0/24", "BGP.local_pref: 250", "BGP.med: 77"}},
		{kind: opBIRDRouteAbsent, argument: "10.39.2.0/24", proof: []string{"BIRD"}},
	},
	"bgp-relay-withdraw-nexthop-self-frr": relayWithdrawalExtras("10.10.0.0/24"),
	"bgp-send-community-suppress-frr": {
		{kind: opRequireAbsent, peer: "frr", command: []string{"vtysh", "-c", "show bgp ipv4 unicast 10.52.0.0/24"}, absent: []string{"65004:100", "65004:200", "65004:300", "65004:1:2"}, proof: []string{"10.52.0.0/24"}},
		{kind: opRequireAbsent, peer: "frr", command: []string{"vtysh", "-c", "show bgp ipv4 unicast 10.52.1.0/24"}, absent: []string{"65004:100", "65004:200", "65004:300", "65004:1:2"}, proof: []string{"10.52.1.0/24"}},
		{kind: opRequireAbsent, peer: "frr", command: []string{"vtysh", "-c", "show bgp ipv4 unicast 10.52.2.0/24"}, absent: []string{"65004:100", "65004:200", "65004:300", "65004:1:2"}, proof: []string{"10.52.2.0/24"}},
		{kind: opRequireAbsent, peer: "bird", command: []string{"birdc", "show route for 10.52.0.0/24 all"}, absent: []string{"ext_community", "large_community"}, proof: []string{"10.52.0.0/24", "(65004,100)", "(65004,200)"}},
		{kind: opRequireAbsent, peer: "bird", command: []string{"birdc", "show route for 10.52.1.0/24 all"}, absent: []string{"ext_community", "large_community"}, proof: []string{"10.52.1.0/24", "(65004,100)", "(65004,200)"}},
		{kind: opRequireAbsent, peer: "bird", command: []string{"birdc", "show route for 10.52.2.0/24 all"}, absent: []string{"ext_community", "large_community"}, proof: []string{"10.52.2.0/24", "(65004,100)", "(65004,200)"}},
	},
	"bgp-srv6-frr": {
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show bgp neighbor 172.30.0.2 json"}, contains: []string{"ipv6Vpn", "advertisedAndReceived"}},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show bgp ipv6 vpn"}, contains: []string{"2001:db8:customer::/48"}, timeout: 30 * time.Second},
		{kind: opWaitJSONFields, peer: "ze", command: zeCommand("show bgp rib count"), minimum: map[string]int{"count": 1}, timeout: 30 * time.Second},
		{kind: opFRRSession, argument: "172.30.0.2"},
	},
	"bgp-vpn-frr": {
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show bgp neighbor 172.30.0.2 json"}, contains: []string{"ipv4Vpn", "advertisedAndReceived"}},
	},
	"bgp-vpn-gobgp": {
		{kind: opWaitContains, peer: "gobgp", command: []string{"gobgp", "global", "rib", "-a", "vpnv4", "-j"}, contains: []string{"10.99.0.0/24", "10.99.1.0/24"}, timeout: 30 * time.Second},
	},
	"bmp-frr": {
		{kind: opWaitContains, peer: "bmp", command: []string{"cat", "/tmp/bmp-status.json"}, contains: []string{"initiation", "peer-up", "route-monitoring"}, timeout: 30 * time.Second},
	},
	"isis-auth-frr": {
		{kind: opDelayRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show isis neighbor"}, contains: []string{"Up"}, delay: 5 * time.Second},
	},
	"isis-dualstack-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show isis database"}, contains: []string{"ze-ds"}, timeout: 60 * time.Second},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 route isis"}, contains: []string{"I"}, timeout: 60 * time.Second},
	},
	"isis-redist-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip route isis"}, contains: []string{"10.99.0.0/24"}, timeout: 60 * time.Second},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show bgp ipv4 unicast"}, contains: []string{"10.99.0.0/24"}, timeout: 60 * time.Second},
	},
	"ospf-ldp-sync-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf database router 172.30.0.2"}, contains: []string{"172.30.0.3", "TOS 0 Metric: 65535"}, timeout: 45 * time.Second},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf database router 172.30.0.2"}, contains: []string{"172.30.0.3", "TOS 0 Metric: 10"}, timeout: 120 * time.Second},
	},
	"ospf-multiinstance-frr": {
		{kind: opWaitContains, peer: "bird", command: []string{"birdc", "show ospf neighbors ospf5"}, contains: []string{"Full"}, timeout: 120 * time.Second},
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"172.30.0.2", "Full"}},
		{kind: opDelayRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, delay: 5 * time.Second},
	},
	"ospf-nbma-frr": {
		{kind: opWaitContainsAny, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf database network"}, contains: []string{"172.30.0.2", "172.30.0.3"}, timeout: 60 * time.Second},
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf database router"}, contains: []string{"172.30.0.2"}},
		{kind: opDelayRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, delay: 5 * time.Second},
	},
	"ospf-ptmp-frr": {
		{kind: opRequireAbsent, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf database network"}, absent: []string{"172.30.0.2"}, proof: []string{"Network Link States"}},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf database router"}, contains: []string{"172.30.0.2"}, timeout: 60 * time.Second},
		{kind: opDelayRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, delay: 5 * time.Second},
	},
	"isis-lan-dis-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show isis database"}, contains: []string{".00-01"}, timeout: 60 * time.Second},
		{kind: opDelayRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show isis neighbor"}, contains: []string{"Up"}, delay: 5 * time.Second},
	},
	"ospf-broadcast-frr": {
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full/", "DR"}},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf database network"}, contains: []string{"Network Link States"}, timeout: 60 * time.Second},
	},
	"ospf-gr-frr": {
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf database router"}, contains: []string{"172.30.0.2"}},
		{kind: opExec, peer: "frr", command: []string{"vtysh", "-c", "clear ip ospf process graceful-restart"}},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 30 * time.Second},
		{kind: opDelayRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, delay: 5 * time.Second},
	},
	"ospf-multiaf-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 database router"}, contains: []string{"172.30.0.2"}, timeout: 60 * time.Second},
		{kind: opDelayRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, delay: 5 * time.Second},
	},
	"ospf-p2p-frr": {
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf interface"}, contains: []string{"POINTOPOINT"}},
		{kind: opDelayRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, delay: 5 * time.Second},
	},
	"ospf-debug-inject-frr": {
		{kind: opExec, peer: "ze", command: zeCommand("debug ospf inject enable")},
		{kind: opExec, peer: "ze", command: zeCommand("debug ip ospf inject opaque scope area id 1 hex 0001000401020304")},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf database opaque-area"}, contains: []string{"1"}, timeout: 60 * time.Second},
		{kind: opExec, peer: "ze", command: zeCommand("debug ip ospf inject opaque scope area id 1 withdraw")},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, timeout: 30 * time.Second},
	},
	"ospfv3-debug-inject-frr": {
		{kind: opExec, peer: "ze", command: zeCommand("debug ospf inject enable")},
		{kind: opExec, peer: "ze", command: zeCommand("debug ipv6 ospf inject lsa scope area type 0x2009 id 1 hex 00000000")},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 database"}, contains: []string{"0x2009"}, timeout: 60 * time.Second},
		{kind: opExec, peer: "ze", command: zeCommand("debug ipv6 ospf inject lsa scope area type 0x2009 id 1 withdraw")},
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, timeout: 30 * time.Second},
	},
	"ospfv3-debug-decode-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf ipv6 database detail"), contains: []string{"\"router\"", "\"scope\"", "\"decoded\""}},
	},
	"ospf-ext-prefix-link-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf database opaque-area"), contains: []string{"extended-prefix", "extended-link"}},
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf database opaque-area"}, contains: []string{"172.30.0.2"}},
		{kind: opDelayRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, delay: 5 * time.Second},
	},
	"ospf-opaque-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf database opaque-area"), contains: []string{"172.30.0.3"}},
		{kind: opDelayRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, delay: 5 * time.Second},
	},
	"ospf-ri-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf database router-information"), contains: []string{"172.30.0.2"}},
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf database opaque-area"}, contains: []string{"172.30.0.2"}},
		{kind: opDelayRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, delay: 5 * time.Second},
	},
	"ospf-sr-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf segment-routing"), contains: []string{"16000", "172.30.0.2"}},
		{kind: opWaitContains, peer: "ze", command: []string{"mpls", "-ls"}, contains: []string{"16100"}, timeout: 60 * time.Second},
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf database segment-routing"}, contains: []string{"172.30.0.2"}},
		{kind: opDelayRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf neighbor"}, contains: []string{"Full"}, delay: 5 * time.Second},
	},
	"ospf-te-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf te-database"), contains: []string{"172.30.0.3"}},
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf database opaque-area"}, contains: []string{"172.30.0.2"}},
	},
	"ospf-te-interas-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf database opaque-as"), contains: []string{"inter-as"}},
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ip ospf database opaque-as"}, contains: []string{"172.30.0.2"}},
	},
	"ospfv3-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 database router"}, contains: []string{"172.30.0.2"}, timeout: 60 * time.Second},
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}},
		{kind: opDelayRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, delay: 5 * time.Second},
	},
	"ospfv3-gr-frr": {
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}},
		{kind: opDelayRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, delay: 5 * time.Second},
	},
	"ospfv3-ptmp-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 database router"}, contains: []string{"172.30.0.2"}, timeout: 60 * time.Second},
		{kind: opDelayRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, delay: 5 * time.Second},
	},
	"ospfv3-vlink-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 interface"}, contains: []string{"VLINK"}, timeout: 90 * time.Second},
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}},
	},
	"ospfv3-broadcast-frr": {
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 database network"}, contains: []string{"172.30.0.2"}},
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 database link"}, contains: []string{"172.30.0.2"}},
	},
	"ospfv3-nssa-redist-frr": {
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 route"}, contains: []string{"2001:db8:7e5::/48", "NSSA"}},
		{kind: opRequireAbsent, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 database external"}, absent: []string{"2001:db8:7e5::/48"}, proof: []string{"AS-External"}},
	},
	"ospfv3-ri-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf database router-information"), contains: []string{"172.30.0.2"}},
		{kind: opRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 database router-information"}, contains: []string{"172.30.0.2"}},
		{kind: opDelayRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, delay: 5 * time.Second},
	},
	"ospfv3-sr-frr": {
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf ipv6 segment-routing"), contains: []string{"16000"}},
		{kind: opWaitContains, peer: "ze", command: []string{"mpls", "-ls"}, contains: []string{"16100"}, timeout: 60 * time.Second},
		{kind: opDelayRequireContains, peer: "frr", command: []string{"vtysh", "-c", "show ipv6 ospf6 neighbor"}, contains: []string{"Full"}, delay: 5 * time.Second},
	},
	"rpki-frr": {
		{kind: opWaitJSONFields, peer: "ze", command: []string{"cat", "/tmp/rpki-check.json"}, fields: map[string]string{"status": "ok", "9.43.0.0/24": "1", "11.43.0.0/24": "2"}, timeout: 60 * time.Second},
		{kind: opRequireAbsent, peer: "ze", command: []string{"cat", "/tmp/rpki-check.json"}, absent: []string{"\"10.43.0.0/24\""}, proof: []string{"\"9.43.0.0/24\"", "\"11.43.0.0/24\""}},
	},
	"rtr-stayrtr": {
		{kind: opWaitJSONFields, peer: "ze", command: zeCommand("show bgp rpki status"), minimum: map[string]int{"vrp-count-ipv4": 2, "vrp-count-ipv6": 2}, timeout: 90 * time.Second},
		{kind: opRequireContains, peer: "ze", command: zeCommand("show bgp rpki roa"), contains: []string{"9.58.0.0/16", "10.58.0.0/16", "2001:db8:58::/48", "2001:db8:59::/48", "4200000001", "65001"}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 9.58.0.0/16 4200000001"), fields: map[string]string{"state": "valid"}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 9.58.0.0/24 4200000001"), fields: map[string]string{"state": "valid"}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 9.58.0.0/25 4200000001"), fields: map[string]string{"state": "invalid"}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 9.58.0.0/16 65001"), fields: map[string]string{"state": "invalid"}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 10.58.0.0/16 65001"), fields: map[string]string{"state": "valid"}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 10.58.0.0/24 65001"), fields: map[string]string{"state": "invalid"}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 2001:db8:58::/48 4200000001"), fields: map[string]string{"state": "valid"}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 2001:db8:58::/64 4200000001"), fields: map[string]string{"state": "invalid"}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 11.58.0.0/16 65001"), fields: map[string]string{"state": "not-found"}},
		{kind: opRequireJSONFields, peer: "ze", command: zeCommand("request bgp rpki validate 2001:db8:5a::/48 65001"), fields: map[string]string{"state": "not-found"}},
	},
	"shutdown-cease-frr": {
		{kind: opWaitContains, peer: "frr", command: []string{"cat", "/tmp/frr.log"}, contains: []string{"Cease", "Administrative Shutdown"}, timeout: 30 * time.Second},
	},
}

func relayWithdrawalExtras(prefix string) []operation {
	var evidence textbuf.Buffer
	update := evidence.Str("rcvd UPDATE about ").Str(prefix).String()
	return []operation{
		{kind: opWaitContains, peer: "frr", command: []string{"cat", "/tmp/frr.log"}, contains: []string{update, "withdrawn"}, timeout: 90 * time.Second},
		{kind: opRequireAbsent, peer: "frr", command: []string{"cat", "/tmp/frr.log"}, absent: []string{"Missing well-known attribute", "rcvd UPDATE with errors in attr"}, proof: []string{update, "withdrawn"}},
	}
}

func zeCommand(command string) []string {
	return []string{"ze", "cli", "-c", command, "--user", "interop", "--format", "json"}
}
