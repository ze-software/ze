// Design: docs/architecture/ike/ipsec-14-responder.md -- the Configuration payload in IKE_AUTH
// Related: cp.go -- proposeLeasedChildTSPayloads and answerNamesOnlyLeases, under test
// VALIDATES: a Child SA request Ze sends on a pool-bound peer's IKE SA proposes the lease
// as the peer's side, and an answer naming anything wider is refused.
// PREVENTS: a Ze-initiated rekey installing the pool prefix or the wildcard as the peer's
// inner side, which routes to one peer the addresses leased to others (RFC 3948 Section 5.1).

package engine

import (
	"errors"
	"net"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// leasedRekeyFixture brings a pool-bound responder SA up through the real IKE_AUTH with a
// CFG_REQUEST of asks, installs its first Child SA, and starts the Child SA rekey Ze
// initiates. selectors, when set, is the responder peer's configured traffic selector
// policy, in place before IKE_AUTH. It returns the SA, its lease host prefix of each
// family as "4" and "6", the pending rekey and the dataplane.
func leasedRekeyFixture(t *testing.T, selectors []ipsec.TrafficSelectorPolicy, asks ...wire.ConfigAttr) (*SA, map[string]string, *pendingRekey, *mockDP) {
	t.Helper()
	vip := cpVIP(t, cpPool())
	ini, resp, ps := cpResponder(t, vip, "ze", true)
	resp.PeerCfg.TrafficSelectors = selectors
	ps.peerCfg.TrafficSelectors = selectors
	cpAuth(t, ini, resp, ps, cfgRequest(asks...))
	leases := resp.VirtualIPs()
	if len(leases) != len(asks) {
		t.Fatalf("the responder holds %v, want one lease per requested family (%d)", leases, len(asks))
	}
	hosts := make(map[string]string, len(leases))
	for _, l := range leases {
		if l.Is4() {
			hosts["4"] = netip.PrefixFrom(l, 32).String()
			continue
		}
		hosts["6"] = netip.PrefixFrom(l, 128).String()
	}
	dp := &mockDP{}
	old, err := createFirstChildSA(resp, testESPGroup(), resp.PeerCfg.LocalAddress, resp.PeerCfg.RemoteAddress, 1, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	_, pending, err := initiateChildRekey(resp, old)
	if err != nil {
		t.Fatalf("initiateChildRekey: %v", err)
	}
	return resp, hosts, pending, dp
}

// leasedRekeySelector is a configured policy valid at commit on a pool-bound peer: the
// remote prefixes 10.10.0.0/24 and fd00:10::/120 cover the pool ranges 10.10.0.0/29 and
// fd00:10::/120 (ValidateVirtualIPPoolBindings, selectorsCoverPool).
func leasedRekeySelector(t *testing.T) []ipsec.TrafficSelectorPolicy {
	t.Helper()
	_, local, err := net.ParseCIDR("192.168.0.0/16")
	if err != nil {
		t.Fatalf("local prefix: %v", err)
	}
	_, remote, err := net.ParseCIDR("10.10.0.0/24")
	if err != nil {
		t.Fatalf("remote prefix: %v", err)
	}
	_, local6, err := net.ParseCIDR("::/0")
	if err != nil {
		t.Fatalf("local IPv6 prefix: %v", err)
	}
	_, remote6, err := net.ParseCIDR("fd00:10::/120")
	if err != nil {
		t.Fatalf("remote IPv6 prefix: %v", err)
	}
	return []ipsec.TrafficSelectorPolicy{{
		Number: "1", LocalPrefix: local, LocalPort: ipsec.AnyPort(),
		RemotePrefix: remote, RemotePort: ipsec.AnyPort(),
	}, {
		Number: "2", LocalPrefix: local6, LocalPort: ipsec.AnyPort(),
		RemotePrefix: remote6, RemotePort: ipsec.AnyPort(),
	}}
}

// leasedRekeyAnswer builds the peer's CREATE_CHILD_SA answer naming tsi as this node's
// side and tsr as its own, where "lease4" and "lease6" stand for the lease host.
func leasedRekeyAnswer(t *testing.T, hosts map[string]string, tsi, tsr []string) []wire.PayloadEntry {
	t.Helper()
	answered := make([]string, 0, len(tsr))
	for _, r := range tsr {
		switch r {
		case "lease4":
			answered = append(answered, hosts["4"])
		case "lease6":
			answered = append(answered, hosts["6"])
		default:
			answered = append(answered, r)
		}
	}
	return []wire.PayloadEntry{
		{Payload: espSAPayload(0x0A0B0C0D)},
		{Payload: &wire.PayloadNonce{NonceData: testNonce(4)}},
		{Payload: tsPayload(t, wire.PayloadTypeTSi, tsi...)},
		{Payload: tsPayload(t, wire.PayloadTypeTSr, answered...)},
	}
}

// TestRFC3948ZeInitiatedRekeyNamesOnlyTheLease drives initiateChildRekey and then
// applyChildRekeyResponse on a pool-bound responder SA, with the peer's answer naming
// its side as the lease host, as the pool prefix, as the configured remote prefix, or as
// the wildcard. Three peers are covered: one with no configured selector and an IPv4
// lease, one with the configured selectors 192.168.0.0/16 <-> 10.10.0.0/24 and
// ::/0 <-> fd00:10::/120 and an IPv4 lease, and one with no configured selector and an
// IPv4 and an IPv6 lease. The cases that clear the recorded
// proposal judge the configured-policy fallback ceiling too. An accepted answer is read
// back from the outbound policies the dataplane received for the replacement Child SA.
//
// RFC requirement: RFC3948-5.1-1 positive -- on a pool-bound peer's IKE SA, the Child SA rekey ze starts records its proposal as this node's side (0.0.0.0/0 and ::/0 with no configured selector, the configured local prefix 192.168.0.0/16 with one) against each of the peer's lease hosts, /32 and /128, and the peer's answer naming exactly those lease hosts is installed as the replacement Child SA, whose outbound policies the dataplane receives name the lease hosts.
// RFC requirement: RFC3948-5.1-1 negative -- an answer to that rekey whose peer side is the pool prefix 10.10.0.0/29, the configured remote prefix 10.10.0.0/24, 0.0.0.0/0, or the IPv6 pool prefix fd00:10::/120 beside the IPv4 lease host is refused as wider than the peer's lease host (errTSWidened) and no replacement Child SA is returned, including when no proposal was recorded and the configured policy, which covers the pool, is the ceiling.
func TestRFC3948ZeInitiatedRekeyNamesOnlyTheLease(t *testing.T) {
	v4 := []wire.ConfigAttr{ip4Ask(netip.Addr{})}
	dual := []wire.ConfigAttr{ip4Ask(netip.Addr{}), {Type: wire.CPAttrInternalIP6Address}}
	for _, tc := range []struct {
		name       string
		selector   bool              // the peer configures 192.168.0.0/16 <-> 10.10.0.0/24 and ::/0 <-> fd00:10::/120
		asks       []wire.ConfigAttr // the CFG_REQUEST attributes
		proposed   []string          // the recorded proposal, "lease4"/"lease6" for the lease host
		tsi, tsr   []string          // the answer, "lease4"/"lease6" for the lease host
		forgetCeil bool              // clear sa.ProposedChildPairs, so the configured policy is the ceiling
		accepted   bool
	}{
		{"the lease host is installed", false, v4, []string{"0.0.0.0/0 <-> lease4"},
			[]string{"0.0.0.0/0"}, []string{"lease4"}, false, true},
		{"the pool prefix is refused", false, v4, []string{"0.0.0.0/0 <-> lease4"},
			[]string{"0.0.0.0/0"}, []string{"10.10.0.0/29"}, false, false},
		{"the wildcard is refused", false, v4, []string{"0.0.0.0/0 <-> lease4"},
			[]string{"0.0.0.0/0"}, []string{"0.0.0.0/0"}, false, false},
		{"the pool prefix is refused with no recorded proposal", false, v4, []string{"0.0.0.0/0 <-> lease4"},
			[]string{"0.0.0.0/0"}, []string{"10.10.0.0/29"}, true, false},
		{"the wildcard is refused with no recorded proposal", false, v4, []string{"0.0.0.0/0 <-> lease4"},
			[]string{"0.0.0.0/0"}, []string{"0.0.0.0/0"}, true, false},
		{"a configured selector installs the lease host", true, v4, []string{"192.168.0.0/16 <-> lease4"},
			[]string{"192.168.0.0/16"}, []string{"lease4"}, false, true},
		{"a configured selector refuses the pool prefix", true, v4, []string{"192.168.0.0/16 <-> lease4"},
			[]string{"192.168.0.0/16"}, []string{"10.10.0.0/29"}, false, false},
		{"a configured selector refuses its remote prefix", true, v4, []string{"192.168.0.0/16 <-> lease4"},
			[]string{"192.168.0.0/16"}, []string{"10.10.0.0/24"}, false, false},
		{"a configured selector refuses its remote prefix with no recorded proposal", true, v4,
			[]string{"192.168.0.0/16 <-> lease4"}, []string{"192.168.0.0/16"}, []string{"10.10.0.0/24"}, true, false},
		{"a dual-stack grant installs both lease hosts", false, dual, []string{"0.0.0.0/0 <-> lease4", "::/0 <-> lease6"},
			[]string{"0.0.0.0/0", "::/0"}, []string{"lease4", "lease6"}, false, true},
		{"a dual-stack grant refuses the IPv6 pool prefix", false, dual, []string{"0.0.0.0/0 <-> lease4", "::/0 <-> lease6"},
			[]string{"0.0.0.0/0", "::/0"}, []string{"lease4", "fd00:10::/120"}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var selectors []ipsec.TrafficSelectorPolicy
			if tc.selector {
				selectors = leasedRekeySelector(t)
			}
			sa, hosts, pending, dp := leasedRekeyFixture(t, selectors, tc.asks...)
			named := strings.NewReplacer("lease4", hosts["4"], "lease6", hosts["6"])
			want := make([]string, 0, len(tc.proposed))
			for _, p := range tc.proposed {
				want = append(want, named.Replace(p))
			}
			slices.Sort(want)
			if got := scopeText(sa.ProposedChildPairs); !slices.Equal(got, want) {
				t.Fatalf("ze proposed %v, want %v: the peer's side of a Child SA request on a pool-bound "+
					"IKE SA is its lease host", got, want)
			}
			if tc.forgetCeil {
				sa.ProposedChildPairs = nil
			}
			answer := leasedRekeyAnswer(t, hosts, tc.tsi, tc.tsr)
			child, err := applyChildRekeyResponse(sa, pending, answer, dp, slogutil.DiscardLogger())
			if tc.accepted {
				if err != nil {
					t.Fatalf("the answer naming the leases %v was refused: %v", hosts, err)
				}
				if child == nil {
					t.Fatal("the answer naming the leases returned no replacement Child SA")
				}
				installed := strings.Split(installedPolicyScope(dp, child), ", ")
				slices.Sort(installed)
				if !slices.Equal(installed, want) {
					t.Fatalf("the replacement Child SA's outbound policies are %v, want %v: the peer's "+
						"side installed is its lease host", installed, want)
				}
				return
			}
			if !errors.Is(err, errTSWidened) {
				t.Fatalf("the answer naming the peer's side as %v returned %v, want a refusal as wider "+
					"than the leases %v", tc.tsr, err, hosts)
			}
			if child != nil {
				t.Fatalf("the refused answer %v still returned a replacement Child SA", tc.tsr)
			}
		})
	}
}
