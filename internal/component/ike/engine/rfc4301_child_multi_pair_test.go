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

// VALIDATES: a transport-mode inbound state selector holds the one negotiated pair
// exactly, and a transport-mode Child SA of several pairs is refused before anything is
// installed.
// TestRFC4301TransportStateSelectorIsTheNegotiatedPair proves the transport-mode SAD
// entry carries the negotiated values and nothing wider.
//
// Goal: RFC 4301 Section 4.4.2 and Section 5.2 on the SAD entry itself. One XFRM state
// carries one selector, so the only exact state is the one pair. Method: install one
// TCP/179 pair and compare every selector field with the inbound policy of that pair;
// then hand installChildSA two pairs between the same hosts (TCP/179 and UDP/3784) and
// require a refusal with no state installed. Before the fix the two pairs left a cover
// of every protocol and port, which admitted a decrypted packet in no pair.
func TestRFC4301TransportStateSelectorIsTheNegotiatedPair(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2-2 positive -- a transport-mode Child SA of one negotiated pair installs an inbound state whose selector is that pair exactly: both hosts, the protocol and both ports.
	// RFC requirement: RFC4301-4.4.2-2 negative -- a transport-mode Child SA of two negotiated pairs, which one state selector cannot hold exactly, is refused and installs no state.
	// RFC requirement: RFC4301-5.2-2 positive -- the transport-mode inbound state selector the kernel matches a decrypted packet against equals the single negotiated pair, field by field.
	local, remote := multiPairPrefix(t, "10.0.0.1/32"), multiPairPrefix(t, "10.0.0.2/32")
	bgp := tsPair{
		I: tsSelector{Net: local, Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: 179}, Proto: 6},
		R: tsSelector{Net: remote, Proto: 6},
	}
	bfd := tsPair{
		I: tsSelector{Net: local, Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: 3784}, Proto: 17},
		R: tsSelector{Net: remote, Proto: 17},
	}

	t.Run("one pair is exact", func(t *testing.T) {
		_, dp := multiPairFixture(t, true, []tsPair{bgp})
		sel := dp.sas[0].Sel
		if sel == nil {
			t.Fatal("transport-mode inbound state has no selector")
		}
		if sel.Src.String() != "10.0.0.2/32" || sel.Dst.String() != "10.0.0.1/32" {
			t.Errorf("state selector %v -> %v, want the two hosts 10.0.0.2/32 -> 10.0.0.1/32", sel.Src, sel.Dst)
		}
		if sel.UpperProto != 6 {
			t.Errorf("state selector protocol = %d, want 6 (TCP), the negotiated protocol", sel.UpperProto)
		}
		if sel.DstPort != dataplane.ExactPortMatch(179) || sel.SrcPort != dataplane.AnyPortMatch() {
			t.Errorf("state selector ports %+v -> %+v, want any -> exactly 179", sel.SrcPort, sel.DstPort)
		}
		in := policyForDir(t, dp.policies, dataplane.SADirIn)
		if sel.Src.String() != in.Src.String() || sel.Dst.String() != in.Dst.String() ||
			sel.UpperProto != in.UpperProto || sel.SrcPort != in.SrcPort || sel.DstPort != in.DstPort {
			t.Errorf("state selector %+v differs from the inbound policy %+v of the one pair", *sel, in)
		}
	})
	t.Run("two pairs are refused", func(t *testing.T) {
		sa := testSA()
		sa.IsInitiator = true
		sa.UseTransportMode = true
		sa.NegotiatedPairs = []tsPair{bgp, bfd}
		sa.NegotiatedTSi, sa.NegotiatedTSr = bgp.I.Net, bgp.R.Net
		dp := &mockDP{}
		child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 7, dp, slogutil.DiscardLogger())
		if err == nil {
			child.Clear()
			t.Fatal("a transport-mode Child SA of two pairs was installed; one state selector cannot hold both exactly")
		}
		if len(dp.sas) != 0 {
			t.Errorf("installed SAs = %d after the refusal, want 0", len(dp.sas))
		}
	})
}
