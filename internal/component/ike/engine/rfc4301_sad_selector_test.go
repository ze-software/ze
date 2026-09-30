// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- Child SA install into the dataplane
// Related: child_test.go -- mockDP, testSA and the inbound SPD policy assertions
// Related: rfc4301_sad_selector_linux_test.go -- the kernel dropping packets outside these selectors

package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// sadSelectorFixture creates a Child SA over one negotiated pair (10.1.0.0/16 TCP from
// local port 179 to 10.2.0.0/16) and returns what the mock dataplane received.
func sadSelectorFixture(t *testing.T, transportMode bool) (*SA, *mockDP) {
	t.Helper()
	sa := testSA()
	sa.IsInitiator = true
	sa.UseTransportMode = transportMode
	tsi := &net.IPNet{IP: net.ParseIP("10.1.0.0").To4(), Mask: net.CIDRMask(16, 32)}
	tsr := &net.IPNet{IP: net.ParseIP("10.2.0.0").To4(), Mask: net.CIDRMask(16, 32)}
	sa.NegotiatedTSi, sa.NegotiatedTSr = tsi, tsr
	sa.NegotiatedPairs = []tsPair{{
		I: tsSelector{Net: tsi, Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: 179}, Proto: 6},
		R: tsSelector{Net: tsr, Proto: 6},
	}}
	dp := &mockDP{}

	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 7, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	t.Cleanup(child.Clear)

	if len(dp.sas) != 2 {
		t.Fatalf("installed SAs = %d, want 2 (in + out)", len(dp.sas))
	}
	if dp.sas[0].SPI != child.InboundSPI {
		t.Fatalf("sas[0] SPI = %#x, want the inbound SPI %#x", dp.sas[0].SPI, child.InboundSPI)
	}
	return sa, dp
}

// TestRFC4301InboundSADEntryCarriesTheNegotiatedSelectors proves what Ze hands the
// dataplane for the inbound SAD entry, per mode, under the whole-stack reading of RFC 4301
// Section 4.4.2.
//
// Goal: every Section 4.4.1.1 selector Ze negotiates (remote address, local address, next
// layer protocol, local port, remote port) reaches the entry that decides which decrypted
// packets the SA accepts. Method: one negotiated pair, installed in each mode through the
// mock dataplane. In TRANSPORT mode the inbound state itself carries the selector
// (SAParams.Sel), because a transport-only secpath passes the kernel's inbound policy
// check. In TUNNEL mode the inbound require-policy carries it, because the kernel drops a
// decapsulated packet that matches no inbound policy; its state selector is not asserted.
// rfc4301_sad_selector_linux_test.go proves the kernel then drops what lies outside.
func TestRFC4301InboundSADEntryCarriesTheNegotiatedSelectors(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2-1 positive -- the inbound SAD entry Ze installs is
	// populated with the negotiated selectors: remote TS as source, local TS as destination,
	// the negotiated protocol, the local port and the any remote port, on the transport-mode
	// state and on the tunnel-mode inbound require-policy.
	t.Run("transport mode: the inbound state carries the selector", func(t *testing.T) {
		sa, dp := sadSelectorFixture(t, true)
		sel := dp.sas[0].Sel
		if sel == nil {
			t.Fatal("transport-mode inbound SAD entry has no selector: the negotiated TSr/TSi/protocol/ports are not populated")
		}
		assertInboundSelector(t, "transport inbound state", sel.Src, sel.Dst, sel.UpperProto, sel.SrcPort, sel.DstPort, sa)
	})
	t.Run("tunnel mode: the inbound require-policy carries the selector", func(t *testing.T) {
		sa, dp := sadSelectorFixture(t, false)
		if len(dp.policies) == 0 {
			t.Fatal("no policy installed")
		}
		pol := dp.policies[0]
		if pol.Dir != dataplane.SADirIn {
			t.Fatalf("policies[0] Dir = %d, want inbound (%d)", pol.Dir, dataplane.SADirIn)
		}
		if pol.Action != dataplane.SPActionProtect || pol.Mode != dataplane.ModeTunnel {
			t.Fatalf("inbound policy action %d mode %d, want a tunnel-mode protect (require) policy", pol.Action, pol.Mode)
		}
		assertInboundSelector(t, "tunnel inbound policy", pol.Src, pol.Dst, pol.UpperProto, pol.SrcPort, pol.DstPort, sa)
	})
}

func assertInboundSelector(t *testing.T, what string, src, dst *net.IPNet, proto uint8, srcPort, dstPort dataplane.PortMatch, sa *SA) {
	t.Helper()
	if src.String() != sa.NegotiatedTSr.String() {
		t.Errorf("%s source = %v, want the negotiated TSr %v", what, src, sa.NegotiatedTSr)
	}
	if dst.String() != sa.NegotiatedTSi.String() {
		t.Errorf("%s destination = %v, want the negotiated TSi %v", what, dst, sa.NegotiatedTSi)
	}
	if proto != 6 {
		t.Errorf("%s protocol = %d, want the negotiated 6", what, proto)
	}
	if dstPort != dataplane.ExactPortMatch(179) {
		t.Errorf("%s destination port = %+v, want exactly the negotiated local port 179", what, dstPort)
	}
	if srcPort != dataplane.AnyPortMatch() {
		t.Errorf("%s source port = %+v, want any, as negotiated for the remote side", what, srcPort)
	}
}
