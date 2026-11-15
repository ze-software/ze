// Design: docs/architecture/bgp/structural-forwarding.md -- foreign acceptance of default next hops.
// Related: check_nexthop_auto_ebgp.go -- the pure foreign-output predicate.
package bgp

import (
	"net/netip"
	"strings"
	"testing"
)

// TestNextHopAutoFRRAcceptance requires the selected, accessible route rather
// than a matching address elsewhere in FRR's JSON or an invalid received path.
func TestNextHopAutoFRRAcceptance(t *testing.T) {
	const accepted = `{"prefix":"10.10.0.0/24","paths":[{"valid":true,"bestpath":{"overall":true},"aspath":{"string":"65001 65004"},"peer":{"peerId":"172.30.0.2"},"nexthops":[{"ip":"172.30.0.2","accessible":true}]}]}`
	prefix := netip.MustParsePrefix("10.10.0.0/24")
	local := netip.MustParseAddr("172.30.0.2")
	for _, test := range []struct {
		name, output string
		accepted     bool
	}{
		{"selected route", accepted, true},
		{"source next hop", strings.Replace(accepted, `"ip":"172.30.0.2"`, `"ip":"172.30.0.9"`, 1), false},
		{"not valid", strings.Replace(accepted, `"valid":true`, `"valid":false`, 1), false},
		{"not selected", strings.Replace(accepted, `"overall":true`, `"overall":false`, 1), false},
		{"inaccessible", strings.Replace(accepted, `"accessible":true`, `"accessible":false`, 1), false},
		{"wrong peer", strings.Replace(accepted, `"peerId":"172.30.0.2"`, `"peerId":"172.30.0.9"`, 1), false},
		{"wrong AS path", strings.Replace(accepted, "65001 65004", "65004", 1), false},
		{"wrong prefix", strings.Replace(accepted, "10.10.0.0/24", "10.10.1.0/24", 1), false},
		{"no path", `{"prefix":"10.10.0.0/24","paths":[]}`, false},
		{"missing fields", `{"prefix":"10.10.0.0/24","paths":[{}]}`, false},
		{"unreadable", "not JSON", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := requireNextHopAutoFRR(test.output, prefix, local, local, zeInjectorASPath)
			if (err == nil) != test.accepted {
				t.Fatalf("acceptance=%t, want %t: %v", err == nil, test.accepted, err)
			}
		})
	}
}
