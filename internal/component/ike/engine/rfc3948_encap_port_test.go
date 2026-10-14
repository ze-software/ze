// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- NAT traversal, encapsulation ports
// Related: create_child.go -- childEncapPorts, takeEncapPorts
// Related: child.go -- installChildSA

package engine

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// encapTestPeerPort is the port a NAT mapped the peer's IKE to in these fixtures. It is
// not 4500, so an assertion on it cannot be met by the NAT-T constant.
const encapTestPeerPort uint16 = 14500

// encapPath gives sa the IKE path a Child SA takes its UDP encapsulation ports from: a
// bound NAT-T socket on loopback, and the peer's authenticated endpoint (10.0.0.2,
// peerPort). It answers this node's NAT-T socket port, which the kernel picks, so it is
// never 4500.
func encapPath(t *testing.T, sa *SA, peerPort uint16) uint16 {
	t.Helper()
	natt := mbTransport(t, "127.0.0.1:0", true)
	sa.bindSockets(nil, natt)
	sa.peerEndpoint = &net.UDPAddr{IP: net.IPv4(10, 0, 0, 2), Port: int(peerPort)}
	return uint16(nttPeerAddr(t, natt).Port)
}

// encapPorts splits installed states into the inbound and the outbound one by address,
// as bfmDirections does, and returns their (sport, dport) pairs.
func encapPorts(t *testing.T, sas []dataplane.SAParams) (in, out [2]uint16) {
	t.Helper()
	var haveIn, haveOut bool
	for i := range sas {
		pair := [2]uint16{sas[i].UDPEncapSPort, sas[i].UDPEncapDPort}
		if sas[i].Dst.String() == "10.0.0.1" {
			in, haveIn = pair, true
			continue
		}
		out, haveOut = pair, true
	}
	if !haveIn || !haveOut {
		t.Fatalf("inbound found %v, outbound found %v among %d states", haveIn, haveOut, len(sas))
	}
	return in, out
}

// natChildSA builds and installs the first Child SA of a NAT-detected, floated SA whose
// peer authenticated from peerPort.
func natChildSA(t *testing.T, peerPort uint16) (sa *SA, localPort uint16, child *ChildSA, dp *mockDP) {
	t.Helper()
	sa = testSA()
	sa.NATDetected = true
	sa.floatToNATTPort()
	localPort = encapPath(t, sa, peerPort)
	dp = &mockDP{}
	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA(peer port %d): %v", peerPort, err)
	}
	return sa, localPort, child, dp
}

// VALIDATES: a peer whose IKE arrives from port 4500 (no port translation) keeps 4500 as
// the peer side of both states, the no-translation polarity of AC-1.
// PREVENTS: a port derivation that only works for a translated peer.
func TestRFC3948PeerOn4500KeepsPort4500(t *testing.T) {
	_, localPort, _, dp := natChildSA(t, transport.NATTPort)
	in, out := encapPorts(t, dp.sas)
	if in != [2]uint16{transport.NATTPort, localPort} {
		t.Errorf("inbound encap (sport, dport) = %v, want (%d, %d)", in, transport.NATTPort, localPort)
	}
	if out != [2]uint16{localPort, transport.NATTPort} {
		t.Errorf("outbound encap (sport, dport) = %v, want (%d, %d)", out, localPort, transport.NATTPort)
	}
}

// VALIDATES: this node's encapsulation port is the port its NAT-T socket is bound to,
// not the constant 4500 (D1d). The test socket is bound to an ephemeral port, the shape
// the ze.test.ike.port hook produces with p+1.
// PREVENTS: ESP encapsulated from a port this node's IKE does not use.
func TestRFC3948LocalEncapPortIsTheNATTSocketPort(t *testing.T) {
	_, localPort, _, dp := natChildSA(t, encapTestPeerPort)
	if localPort == transport.NATTPort {
		t.Fatalf("the test NAT-T socket is bound to %d; the test cannot tell the socket from the constant", localPort)
	}
	in, out := encapPorts(t, dp.sas)
	if in[1] != localPort {
		t.Errorf("inbound encap dport = %d, want the NAT-T socket port %d", in[1], localPort)
	}
	if out[0] != localPort {
		t.Errorf("outbound encap sport = %d, want the NAT-T socket port %d", out[0], localPort)
	}
}

// VALIDATES: a peer a NAT mapped to port 500 is followed like any other port: inbound
// encap (500, L), outbound (L, 500), and the Child SA is not refused (Q-2 = A').
// PREVENTS: a refusal that would reject a connection strongSwan accepts.
//
// RFC requirement: RFC3948-2.1-2 positive -- at Child SA creation the encapsulation ports
// are the ports IKE uses, including a NAT-mapped peer port 500; this node's own port stays
// its NAT-T socket port.
func TestRFC3948ChildSAFollowsANATMappedPeerPort500(t *testing.T) {
	_, localPort, _, dp := natChildSA(t, transport.IKEPort)
	in, out := encapPorts(t, dp.sas)
	if in != [2]uint16{transport.IKEPort, localPort} {
		t.Errorf("inbound encap (sport, dport) = %v, want (%d, %d)", in, transport.IKEPort, localPort)
	}
	if out != [2]uint16{localPort, transport.IKEPort} {
		t.Errorf("outbound encap (sport, dport) = %v, want (%d, %d)", out, localPort, transport.IKEPort)
	}
}

// VALIDATES: a CREATE_CHILD_SA Child SA (newChildSA, the builder both CREATE_CHILD_SA
// directions call) and a ze-initiated Child SA rekey take the IKE SA's CURRENT ports,
// after the IKE SA's endpoint moved from Y1 to Y2, rather than the retired pair's (D1c).
// PREVENTS: a rekey that reinstalls ESP on a port IKE no longer uses.
func TestRFC3948CreatedAndRekeyedChildSATakeTheIKESAPorts(t *testing.T) {
	const movedPort uint16 = 14501
	sa, localPort, old, dp := natChildSA(t, encapTestPeerPort)
	sa.peerEndpoint = &net.UDPAddr{IP: net.IPv4(10, 0, 0, 2), Port: int(movedPort)}

	created, err := newChildSA(sa, "10.0.0.1", "10.0.0.2", &childSpec{mode: modeTunnel})
	if err != nil {
		t.Fatalf("newChildSA: %v", err)
	}
	if created.udpLocalPort != localPort || created.udpRemotePort != movedPort {
		t.Errorf("CREATE_CHILD_SA child ports (local, remote) = (%d, %d), want (%d, %d)",
			created.udpLocalPort, created.udpRemotePort, localPort, movedPort)
	}

	pending := &pendingRekey{
		kind:          rekeyChild,
		localNonce:    make([]byte, nonceLen),
		newInboundSPI: 0x55667788,
		oldChild:      old,
	}
	inner := append([]wire.PayloadEntry{
		{Payload: espSAPayload(0x99AABBCC)},
		{Payload: &wire.PayloadNonce{NonceData: make([]byte, nonceLen)}},
	}, childRekeyAnswerTS(t, old)...)
	if _, err := applyChildRekeyResponse(sa, pending, inner, dp, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("applyChildRekeyResponse: %v", err)
	}
	if len(dp.sas) != 4 {
		t.Fatalf("installed %d states, want the original pair and the rekeyed pair", len(dp.sas))
	}
	in, out := encapPorts(t, dp.sas[2:])
	if in != [2]uint16{movedPort, localPort} {
		t.Errorf("rekeyed inbound encap (sport, dport) = %v, want (%d, %d)", in, movedPort, localPort)
	}
	if out != [2]uint16{localPort, movedPort} {
		t.Errorf("rekeyed outbound encap (sport, dport) = %v, want (%d, %d)", out, localPort, movedPort)
	}
}

// VALIDATES: a Child SA that needs UDP encapsulation on an IKE SA with no authenticated
// peer endpoint is refused with errChildEncapPort naming the peer, and nothing is
// installed: the first Child SA with and without MOBIKE, and a ze-initiated rekey, which
// leaves the old pair as the only one installed (Q-2 = A').
// PREVENTS: the 4500 default that hid D1b coming back.
//
// RFC requirement: RFC3948-2.1-2 negative -- with no port IKE uses with the peer, no
// encapsulated Child SA is installed on any other port.
func TestRFC3948ChildSARefusesEncapWithoutAnIKEPort(t *testing.T) {
	for _, mobike := range []bool{false, true} {
		sa := testSA()
		sa.NATDetected = true
		sa.floatToNATTPort()
		encapPath(t, sa, encapTestPeerPort)
		sa.peerEndpoint = nil
		if mobike {
			sa.mobike.enabled = true
			sa.mobike.local = &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: int(transport.NATTPort)}
		}
		dp := &mockDP{}
		_, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, slogutil.DiscardLogger())
		if !errors.Is(err, errChildEncapPort) {
			t.Errorf("mobike=%v: createFirstChildSA error = %v, want errChildEncapPort", mobike, err)
		}
		if err != nil && !strings.Contains(err.Error(), sa.PeerName) {
			t.Errorf("mobike=%v: error %q does not name the peer %q", mobike, err, sa.PeerName)
		}
		if len(dp.sas) != 0 {
			t.Errorf("mobike=%v: %d states installed, want none", mobike, len(dp.sas))
		}
	}

	sa, _, old, dp := natChildSA(t, encapTestPeerPort)
	sa.peerEndpoint = nil
	pending := &pendingRekey{
		kind:          rekeyChild,
		localNonce:    make([]byte, nonceLen),
		newInboundSPI: 0x55667788,
		oldChild:      old,
	}
	inner := append([]wire.PayloadEntry{
		{Payload: espSAPayload(0x99AABBCC)},
		{Payload: &wire.PayloadNonce{NonceData: make([]byte, nonceLen)}},
	}, childRekeyAnswerTS(t, old)...)
	if _, err := applyChildRekeyResponse(sa, pending, inner, dp, slogutil.DiscardLogger()); !errors.Is(err, errChildEncapPort) {
		t.Errorf("rekey: applyChildRekeyResponse error = %v, want errChildEncapPort", err)
	}
	if len(dp.sas) != 2 {
		t.Errorf("rekey: %d states installed, want only the original pair", len(dp.sas))
	}
}
