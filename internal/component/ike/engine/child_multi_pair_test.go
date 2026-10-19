// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one policy per negotiated pair
// Related: child.go -- childPolicies and inboundStateSelector, the producers under test
// Related: rfc4301_sad_selector_test.go -- the single-pair inbound SAD entry

package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// multiPairPrefix parses a CIDR for a fixture and fails the test on a typo.
func multiPairPrefix(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("fixture prefix %q: %v", cidr, err)
	}
	return n
}

// multiPairFixture creates a Child SA as the IKE_AUTH initiator over the given
// negotiated pairs (TSi/TSr orientation, TSi local) and returns the child and what the
// mock dataplane received.
func multiPairFixture(t *testing.T, transportMode bool, pairs []tsPair) (*ChildSA, *mockDP) {
	t.Helper()
	sa := testSA()
	sa.IsInitiator = true
	sa.UseTransportMode = transportMode
	sa.NegotiatedPairs = pairs
	sa.NegotiatedTSi, sa.NegotiatedTSr = pairs[0].I.Net, pairs[0].R.Net
	dp := &mockDP{}

	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 7, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	t.Cleanup(child.Clear)
	if len(dp.sas) != 2 {
		t.Fatalf("installed SAs = %d, want 2 (one inbound, one outbound): several pairs share ONE SA", len(dp.sas))
	}
	return child, dp
}

// wantPolicy is the selector one installed policy must carry.
type wantPolicy struct {
	dir      dataplane.SADir
	src, dst string
	proto    uint8
	srcPort  dataplane.PortMatch
	dstPort  dataplane.PortMatch
}

// VALIDATES: one inbound and one outbound policy per negotiated pair, bound to the one SA.
// TestRFC4301EveryNegotiatedPairGetsAPolicyPerDirection proves that a Child SA whose
// peer answered two TSi/TSr pairs reaches the dataplane with both values, in both
// directions, all bound to the one SA.
//
// Goal: RFC 4301 Section 4.4.2, the inbound SAD entry populated with "the value or
// values negotiated". Method: two disjoint pairs, the second restricted to TCP from
// local port 22, installed in tunnel mode through the mock dataplane. Four policies
// must arrive, inbound first, each with the negotiated prefixes, protocol and ports
// of its own pair, and each naming the SA of its direction (SAID, ReqID). Before the
// fix only the first pair was installed, and the second pair's traffic had no policy.
func TestRFC4301EveryNegotiatedPairGetsAPolicyPerDirection(t *testing.T) {
	// Claim toward RFC4301-4.4.2-2, untagged while that row holds its {gap} for the transport-mode residual: with two negotiated pairs, Ze installs one inbound and one outbound policy per pair, each carrying that pair's prefixes, protocol and ports, and every one referencing the Child SA's inbound or outbound SA by SPI and ReqID.
	pairs := []tsPair{
		{I: tsSelector{Net: multiPairPrefix(t, "10.1.0.0/16")}, R: tsSelector{Net: multiPairPrefix(t, "10.2.0.0/16")}},
		{
			I: tsSelector{Net: multiPairPrefix(t, "10.3.0.0/24"), Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: 22}, Proto: 6},
			R: tsSelector{Net: multiPairPrefix(t, "10.4.0.0/24"), Proto: 6},
		},
	}
	child, dp := multiPairFixture(t, false, pairs)

	anyPort, ssh := dataplane.AnyPortMatch(), dataplane.ExactPortMatch(22)
	want := []wantPolicy{
		{dir: dataplane.SADirIn, src: "10.2.0.0/16", dst: "10.1.0.0/16", srcPort: anyPort, dstPort: anyPort},
		{dir: dataplane.SADirIn, src: "10.4.0.0/24", dst: "10.3.0.0/24", proto: 6, srcPort: anyPort, dstPort: ssh},
		{dir: dataplane.SADirOut, src: "10.1.0.0/16", dst: "10.2.0.0/16", srcPort: anyPort, dstPort: anyPort},
		{dir: dataplane.SADirOut, src: "10.3.0.0/24", dst: "10.4.0.0/24", proto: 6, srcPort: ssh, dstPort: anyPort},
	}
	if len(dp.policies) != len(want) {
		t.Fatalf("installed policies = %d, want %d (one per negotiated pair per direction)", len(dp.policies), len(want))
	}
	for i, w := range want {
		got := dp.policies[i]
		if got.Dir != w.dir {
			t.Errorf("policy %d direction = %d, want %d", i, got.Dir, w.dir)
		}
		if got.Src.String() != w.src || got.Dst.String() != w.dst {
			t.Errorf("policy %d selector %v -> %v, want %s -> %s", i, got.Src, got.Dst, w.src, w.dst)
		}
		if got.UpperProto != w.proto {
			t.Errorf("policy %d protocol = %d, want %d", i, got.UpperProto, w.proto)
		}
		if got.SrcPort != w.srcPort || got.DstPort != w.dstPort {
			t.Errorf("policy %d ports %+v -> %+v, want %+v -> %+v", i, got.SrcPort, got.DstPort, w.srcPort, w.dstPort)
		}
		wantSPI := child.OutboundSPI
		if w.dir == dataplane.SADirIn {
			wantSPI = child.InboundSPI
		}
		if got.SAID != wantSPI {
			t.Errorf("policy %d protects with SPI %#x, want the %d-direction SA %#x", i, got.SAID, w.dir, wantSPI)
		}
		if got.ReqID != child.ReqID {
			t.Errorf("policy %d ReqID = %d, want the Child SA's %d, which binds its template to the SA", i, got.ReqID, child.ReqID)
		}
		if got.Action != dataplane.SPActionProtect {
			t.Errorf("policy %d action = %d, want protect", i, got.Action)
		}
	}
}

// VALIDATES: the transport-mode inbound state selector admits every negotiated pair.
// TestRFC4301TransportStateSelectorCoversEveryPair proves the transport-mode inbound
// state selector holds every negotiated value, and no wider than one selector must.
//
// Goal: RFC 4301 Section 4.4.2 on the SAD entry itself. One XFRM state carries one
// selector, so two pairs between the same two hosts, TCP to local port 179 and UDP to
// local port 3784, must leave a selector that admits both: the hosts exactly, every
// protocol and every port. The first pair alone, the state before the fix, admits
// TCP/179 only and drops the second pair's traffic as XfrmInStateMismatch. The single
// pair case keeps that pair exactly.
func TestRFC4301TransportStateSelectorCoversEveryPair(t *testing.T) {
	// Claim toward RFC4301-4.4.2-2, untagged while that row holds its {gap} for the transport-mode residual: with two negotiated transport-mode pairs, the inbound state selector Ze installs holds both pairs' values: the two hosts exactly and, as the pairs differ in protocol and port, any protocol and any port.
	local, remote := multiPairPrefix(t, "10.0.0.1/32"), multiPairPrefix(t, "10.0.0.2/32")
	bgp := tsPair{
		I: tsSelector{Net: local, Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: 179}, Proto: 6},
		R: tsSelector{Net: remote, Proto: 6},
	}
	bfd := tsPair{
		I: tsSelector{Net: local, Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: 3784}, Proto: 17},
		R: tsSelector{Net: remote, Proto: 17},
	}

	t.Run("two pairs", func(t *testing.T) {
		_, dp := multiPairFixture(t, true, []tsPair{bgp, bfd})
		sel := dp.sas[0].Sel
		if sel == nil {
			t.Fatal("transport-mode inbound state has no selector")
		}
		if sel.Src.String() != "10.0.0.2/32" || sel.Dst.String() != "10.0.0.1/32" {
			t.Errorf("state selector %v -> %v, want the two hosts 10.0.0.2/32 -> 10.0.0.1/32", sel.Src, sel.Dst)
		}
		if sel.UpperProto != 0 {
			t.Errorf("state selector protocol = %d, want any: the pairs negotiated 6 and 17, and a single protocol drops the other pair", sel.UpperProto)
		}
		if sel.SrcPort != dataplane.AnyPortMatch() || sel.DstPort != dataplane.AnyPortMatch() {
			t.Errorf("state selector ports %+v -> %+v, want any: a single port drops the other pair", sel.SrcPort, sel.DstPort)
		}
		if len(dp.policies) != 4 {
			t.Errorf("installed policies = %d, want 4: each pair keeps its exact selector in a policy", len(dp.policies))
		}
	})
	t.Run("one pair stays exact", func(t *testing.T) {
		_, dp := multiPairFixture(t, true, []tsPair{bgp})
		sel := dp.sas[0].Sel
		if sel == nil {
			t.Fatal("transport-mode inbound state has no selector")
		}
		if sel.UpperProto != 6 || sel.DstPort != dataplane.ExactPortMatch(179) {
			t.Errorf("state selector protocol %d port %+v, want exactly TCP to 179", sel.UpperProto, sel.DstPort)
		}
	})
}

// VALIDATES: coverPrefix widens to the longest common prefix and refuses two families.
// TestCoverPrefixIsTheLongestCommonPrefix pins the address widening the transport
// state selector uses: the longest prefix holding both, nil (any) absorbing, and two
// families refused rather than widened to every address.
func TestCoverPrefixIsTheLongestCommonPrefix(t *testing.T) {
	cases := []struct{ a, b, want string }{
		{"10.0.0.1/32", "10.0.0.1/32", "10.0.0.1/32"},
		{"10.0.0.1/32", "10.0.0.3/32", "10.0.0.0/30"},
		{"10.1.0.0/16", "10.3.0.0/24", "10.0.0.0/14"},
		{"2001:db8::1/128", "2001:db8::2/128", "2001:db8::/126"},
	}
	for _, c := range cases {
		got, err := coverPrefix(multiPairPrefix(t, c.a), multiPairPrefix(t, c.b))
		if err != nil {
			t.Fatalf("coverPrefix(%s, %s): %v", c.a, c.b, err)
		}
		if got.String() != c.want {
			t.Errorf("coverPrefix(%s, %s) = %v, want %s", c.a, c.b, got, c.want)
		}
	}
	if got, err := coverPrefix(nil, multiPairPrefix(t, "10.0.0.1/32")); err != nil || got != nil {
		t.Errorf("coverPrefix(any, host) = %v, %v; want any (nil) and no error", got, err)
	}
	if _, err := coverPrefix(multiPairPrefix(t, "10.0.0.1/32"), multiPairPrefix(t, "2001:db8::1/128")); err == nil {
		t.Error("coverPrefix across two families returned no error; one selector holds one family")
	}
}
