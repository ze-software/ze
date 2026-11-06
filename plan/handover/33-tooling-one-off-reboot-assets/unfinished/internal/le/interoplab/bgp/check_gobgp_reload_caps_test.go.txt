// Design: docs/architecture/testing/interop.md -- native capability-set session fences.
package bgp

import (
	"strings"
	"testing"
)

// TestGoBGPReloadNativeCapabilityOrderAndMultiplicity feeds the actual CLI
// decoder and session fence with pinned api.Capability JSON. GoBGP toConfig
// iterates a map when emitting remote capabilities: ordering is not identity,
// while capability values and repeated-element multiplicity remain identity.
// This is an observer regression, never a native export-reload proof.
func TestGoBGPReloadNativeCapabilityOrderAndMultiplicity(t *testing.T) {
	const document = `{"state":{"neighbor_address":"172.30.0.2","peer_asn":65001,"router_id":"172.30.0.2","session_state":6,"messages":{"received":{"open":1,"update":8},"sent":{"open":1,"update":1}},"remote_cap":REMOTE,"local_cap":LOCAL},"timers":{"state":{"uptime":{"seconds":1}}},"afi_safis":[{"state":{}}]}`
	const multi = `{"Cap":{"MultiProtocol":{"family":{"afi":1,"safi":1}}}}`
	const refresh = `{"Cap":{"RouteRefresh":{}}}`
	const asn = `{"Cap":{"FourOctetAsn":{"asn":65001}}}`
	original := "[" + multi + "," + refresh + "," + asn + "]"
	permuted := "[" + asn + "," + multi + "," + refresh + "]"
	repeated := "[" + multi + "," + refresh + "," + refresh + "," + asn + "]"
	repeatedPermuted := "[" + refresh + "," + asn + "," + refresh + "," + multi + "]"
	parse := func(t *testing.T, remote, local string) gobgpReloadPeer {
		t.Helper()
		input := strings.NewReplacer("REMOTE", remote, "LOCAL", local).Replace(document)
		peer, err := parseGoBGPReloadPeer(input, "172.30.0.2", 65001)
		if err != nil {
			t.Fatal(err)
		}
		return peer
	}
	for _, test := range []struct{ name, beforeRemote, afterRemote, beforeLocal, afterLocal string }{
		{"original", original, original, original, original},
		{"remote-map-order", original, permuted, original, original},
		{"local-order", original, original, original, permuted},
		{"preserved-multiplicity", repeated, repeatedPermuted, original, original},
	} {
		t.Run(test.name, func(t *testing.T) {
			before, after := parse(t, test.beforeRemote, test.beforeLocal), parse(t, test.afterRemote, test.afterLocal)
			if err := sameGoBGPReloadSession(before, after, true); err != nil {
				t.Fatalf("unchanged original native capability contents were rejected: %v", err)
			}
		})
	}
	for _, test := range []struct{ name, beforeRemote, afterRemote string }{
		{"removed-capability", original, "[" + multi + "," + asn + "]"},
		{"added-capability", original, repeated},
		{"removed-duplicate", repeated, original},
		{"changed-AS-capability", original, strings.Replace(original, `"asn":65001`, `"asn":65004`, 1)},
		{"changed-family-capability", original, strings.Replace(original, `"safi":1`, `"safi":2`, 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			before, after := parse(t, test.beforeRemote, original), parse(t, test.afterRemote, original)
			if sameGoBGPReloadSession(before, after, true) == nil {
				t.Fatal("changed native capability contents/multiplicity passed")
			}
		})
	}
}
