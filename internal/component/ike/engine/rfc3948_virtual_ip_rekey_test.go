// Design: docs/architecture/ike/ipsec-14-responder.md -- the Configuration payload in IKE_AUTH
// Related: cp.go -- proposeLeasedChildTSPayloads and answerNamesOnlyLeases, under test
// VALIDATES: a Child SA request Ze sends on a pool-bound peer's IKE SA proposes the lease
// as the peer's side, and an answer naming anything wider is refused.
// PREVENTS: a Ze-initiated rekey installing the pool prefix or the wildcard as the peer's
// inner side, which routes to one peer the addresses leased to others (RFC 3948 Section 5.1).

package engine

import (
	"errors"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// leasedRekeyFixture brings a pool-bound responder SA up through the real IKE_AUTH with a
// CFG_REQUEST, installs its first Child SA, and starts the Child SA rekey Ze initiates. It
// returns the SA, its lease, the pending rekey and the dataplane.
func leasedRekeyFixture(t *testing.T) (*SA, netip.Addr, *pendingRekey, *mockDP) {
	t.Helper()
	vip := cpVIP(t, cpPool())
	ini, resp, ps := cpResponder(t, vip, "ze", true)
	cpAuth(t, ini, resp, ps, cfgRequest(ip4Ask(netip.Addr{})))
	leases := resp.VirtualIPs()
	if len(leases) != 1 {
		t.Fatalf("the responder holds %v, want one IPv4 lease", leases)
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
	return resp, leases[0], pending, dp
}

// TestRFC3948ZeInitiatedRekeyNamesOnlyTheLease drives initiateChildRekey and then
// applyChildRekeyResponse on a pool-bound responder SA, with the peer's answer naming
// its side as the lease host, as the pool prefix, or as 0.0.0.0/0. The last case clears
// the recorded proposal, so the configured-policy fallback ceiling is judged too.
//
// RFC requirement: RFC3948-5.1-1 positive -- on a pool-bound peer's IKE SA, the Child SA rekey ze starts records its proposal as this node's 0.0.0.0/0 against the peer's lease host /32, and the peer's answer naming exactly that lease host is installed as the replacement Child SA.
// RFC requirement: RFC3948-5.1-1 negative -- an answer to that rekey whose peer side is the pool prefix 10.10.0.0/29 or 0.0.0.0/0 is refused as wider than the proposal and no replacement Child SA is returned, including when no proposal was recorded and the configured policy is the ceiling.
func TestRFC3948ZeInitiatedRekeyNamesOnlyTheLease(t *testing.T) {
	for _, tc := range []struct {
		name        string
		answeredTSr string // "" answers the lease host
		forgetCeil  bool   // clear sa.ProposedChildPairs, so the configured policy is the ceiling
		accepted    bool
	}{
		{"the lease host is installed", "", false, true},
		{"the pool prefix is refused", "10.10.0.0/29", false, false},
		{"the wildcard is refused", "0.0.0.0/0", false, false},
		{"the pool prefix is refused with no recorded proposal", "10.10.0.0/29", true, false},
		{"the wildcard is refused with no recorded proposal", "0.0.0.0/0", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sa, lease, pending, dp := leasedRekeyFixture(t)
			host := netip.PrefixFrom(lease, 32).String()
			if want := []string{"0.0.0.0/0 <-> " + host}; !slices.Equal(scopeText(sa.ProposedChildPairs), want) {
				t.Fatalf("ze proposed %v, want %v: the peer's side of a Child SA request on a pool-bound "+
					"IKE SA is its lease host", scopeText(sa.ProposedChildPairs), want)
			}
			if tc.forgetCeil {
				sa.ProposedChildPairs = nil
			}
			answered := tc.answeredTSr
			if answered == "" {
				answered = host
			}
			child, err := applyChildRekeyResponse(sa, pending, peerRekeyAnswer(t, "0.0.0.0/0", answered), dp, slogutil.DiscardLogger())
			if tc.accepted {
				if err != nil {
					t.Fatalf("the answer naming the lease %s was refused: %v", host, err)
				}
				if child == nil {
					t.Fatal("the answer naming the lease returned no replacement Child SA")
				}
				return
			}
			if !errors.Is(err, errTSWidened) {
				t.Fatalf("the answer naming the peer's side as %s returned %v, want a refusal as wider "+
					"than the lease %s", answered, err, host)
			}
			if child != nil {
				t.Fatalf("the refused answer %s still returned a replacement Child SA", answered)
			}
		})
	}
}
