package bgp

import (
	"encoding/json"
	"testing"
)

// pathsLimitCheckerBranches is the named subtest TestBespokeCheckerBranches owes
// this bespoke checker: every predicate driven in both polarities, no container.
func pathsLimitCheckerBranches(t *testing.T) {
	t.Run("foreign capability", pathsLimitBranchForeignCapability)
	t.Run("unrestricted sender", pathsLimitBranchUnrestrictedSender)
	t.Run("replacement withdrawal retry", pathsLimitBranchReplacementWithdrawalRetry)
	t.Run("received remote limit", pathsLimitBranchReceivedRemoteLimit)
}

// FRR 10.3.1 bgp_vty.c emits these fields only for negotiated capabilities.
// ADD-PATH alone must never qualify as a PATHS-LIMIT interop success.
func pathsLimitBranchForeignCapability(t *testing.T) {
	const negotiated = `{"172.30.0.2":{"bgpState":"Established","neighborCapabilities":{"addPath":{"ipv4Unicast":{"rxAdvertisedAndReceived":true}},"pathsLimit":{"ipv4Unicast":{"advertisedAndReceived":true,"advertisedPathsLimit":2,"receivedPathsLimit":10}}}}}`
	if err := requireFRRPathsLimit(negotiated, zeLabAddress); err != nil {
		t.Fatal(err)
	}
	for name, output := range map[string]string{
		"ADD-PATH only":               `{"172.30.0.2":{"bgpState":"Established","neighborCapabilities":{"addPath":{"ipv4Unicast":{"rxAdvertisedAndReceived":true}}}}}`,
		"advertised but not received": `{"172.30.0.2":{"bgpState":"Established","neighborCapabilities":{"addPath":{"ipv4Unicast":{"rxAdvertisedAndReceived":true}},"pathsLimit":{"ipv4Unicast":{"advertised":true,"advertisedPathsLimit":2}}}}}`,
		"reversed limits":             `{"172.30.0.2":{"bgpState":"Established","neighborCapabilities":{"addPath":{"ipv4Unicast":{"rxAdvertisedAndReceived":true}},"pathsLimit":{"ipv4Unicast":{"advertisedAndReceived":true,"advertisedPathsLimit":10,"receivedPathsLimit":2}}}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := requireFRRPathsLimit(output, zeLabAddress); err == nil {
				t.Fatal("missing or wrong code76 negotiation passed")
			}
		})
	}
}

func pathsLimitBranchUnrestrictedSender(t *testing.T) {
	// This is the remote RIB after three distinct offers across separate writes
	// with admission removed: the prefix and session still exist, but the third
	// path is decisive even though the first two remain exactly right.
	output := pathsLimitReceiverJSON(t, []uint32{1, 2, 3}, []uint32{100, 200, 300})
	if err := requireFRRPathsLimitState(output, zeLabAddress, map[uint32]uint32{1: 100, 2: 200}); err == nil {
		t.Fatal("sender without admission passed at-cap path count")
	}
}

func pathsLimitBranchReplacementWithdrawalRetry(t *testing.T) {
	for _, state := range []struct {
		name    string
		ids     []uint32
		metrics []uint32
		want    map[uint32]uint32
	}{
		{"at cap", []uint32{1, 2}, []uint32{100, 200}, map[uint32]uint32{1: 100, 2: 200}},
		{"replacement", []uint32{1, 2}, []uint32{101, 200}, map[uint32]uint32{1: 101, 2: 200}},
		{"withdrawal", []uint32{1}, []uint32{101}, map[uint32]uint32{1: 101}},
		{"retry", []uint32{1, 3}, []uint32{101, 300}, map[uint32]uint32{1: 101, 3: 300}},
	} {
		t.Run(state.name, func(t *testing.T) {
			if err := requireFRRPathsLimitState(pathsLimitReceiverJSON(t, state.ids, state.metrics), zeLabAddress, state.want); err != nil {
				t.Fatal(err)
			}
		})
	}
	for name, output := range map[string]string{
		"blocked replacement":     pathsLimitReceiverJSON(t, []uint32{1, 2}, []uint32{100, 200}),
		"duplicate identifier":    pathsLimitReceiverJSON(t, []uint32{1, 1}, []uint32{101, 101}),
		"wrong withdrawal":        pathsLimitReceiverJSON(t, []uint32{2}, []uint32{200}),
		"cached retry suppressed": pathsLimitReceiverJSON(t, []uint32{1}, []uint32{101}),
	} {
		t.Run(name, func(t *testing.T) {
			want := map[uint32]uint32{1: 101, 2: 200}
			if name == "wrong withdrawal" {
				want = map[uint32]uint32{1: 101}
			}
			if name == "cached retry suppressed" {
				want = map[uint32]uint32{1: 101, 3: 300}
			}
			if err := requireFRRPathsLimitState(output, zeLabAddress, want); err == nil {
				t.Fatal("incorrect remote path transition passed")
			}
		})
	}
}

func pathsLimitBranchReceivedRemoteLimit(t *testing.T) {
	const correct = `[{"peer":"172.30.0.3","negotiated":{"paths-limit":{"send":{"ipv4/unicast":2},"receive":{"ipv4/unicast":10}}}}]`
	if err := requireZePathsLimit(correct, "172.30.0.3"); err != nil {
		t.Fatal(err)
	}
	const localOnly = `[{"peer":"172.30.0.3","negotiated":{"paths-limit":{"receive":{"ipv4/unicast":10}}}}]`
	if err := requireZePathsLimit(localOnly, "172.30.0.3"); err == nil {
		t.Fatal("local receive request passed as received remote limit")
	}
}

func pathsLimitReceiverJSON(t *testing.T, ids, metrics []uint32) string {
	t.Helper()
	paths := make([]map[string]any, len(ids))
	for index, id := range ids {
		paths[index] = map[string]any{
			"addpathRxId": id, "metric": metrics[index], "valid": true,
			"peer": map[string]string{"peerId": zeLabAddress},
		}
	}
	output, err := json.Marshal(map[string]any{"prefix": injectPrefixFirst, "paths": paths})
	if err != nil {
		t.Fatal(err)
	}
	return string(output)
}
