// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- NAT traversal, encapsulation ports
// Related: create_child.go -- childEncapPorts, takeEncapPorts
// Related: child.go -- installChildSA

package engine

import (
	"encoding/binary"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
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

// bindNATT gives sa a bound NAT-T socket on loopback and answers its port, which the
// kernel picks, so it is never 4500. Unlike encapPath it records no peer endpoint: the
// handler under test adopts one.
func bindNATT(t *testing.T, sa *SA) (*transport.UDPTransport, uint16) {
	t.Helper()
	natt := mbTransport(t, "127.0.0.1:0", true)
	sa.bindSockets(nil, natt)
	return natt, uint16(nttPeerAddr(t, natt).Port)
}

// encapPortsByDir is encapPorts keyed on the state's direction rather than its address,
// for fixtures whose two ends use different address plans.
func encapPortsByDir(t *testing.T, sas []dataplane.SAParams) (in, out [2]uint16) {
	t.Helper()
	var haveIn, haveOut bool
	for i := range sas {
		pair := [2]uint16{sas[i].UDPEncapSPort, sas[i].UDPEncapDPort}
		switch sas[i].Dir {
		case dataplane.SADirIn:
			in, haveIn = pair, true
		case dataplane.SADirOut:
			out, haveOut = pair, true
		case dataplane.SADirFwd:
			// A Child SA installs one inbound and one outbound state (child.go); forward is
			// a policy direction only, so a forward state here is an engine defect.
			t.Fatalf("state %d has direction fwd: a Child SA state is inbound or outbound", i)
		}
	}
	if !haveIn || !haveOut {
		t.Fatalf("inbound found %v, outbound found %v among %d states", haveIn, haveOut, len(sas))
	}
	return in, out
}

// wantEncap checks the installed pair is (peer, local) inbound and (local, peer) outbound.
func wantEncap(t *testing.T, sas []dataplane.SAParams, local, peer uint16) {
	t.Helper()
	in, out := encapPortsByDir(t, sas)
	if in != [2]uint16{peer, local} {
		t.Errorf("inbound encap (sport, dport) = %v, want (%d, %d)", in, peer, local)
	}
	if out != [2]uint16{local, peer} {
		t.Errorf("outbound encap (sport, dport) = %v, want (%d, %d)", out, local, peer)
	}
}

// encapSAInit runs IKE_SA_INIT between two in-process SAs and marks both NAT-detected,
// as a NAT_DETECTION mismatch would, so the Child SA the test drives needs UDP
// encapsulation.
func encapSAInit(t *testing.T, iniPeer, respPeer ipsec.SiteToSitePeer) (ini, resp *SA, table *SATable) {
	t.Helper()
	quiet := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	table = NewSATable()
	ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, testESPGroup())
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	saInitReq := buildSAInitRequest(ini, ikeGroup)
	ini.InitiatorSAInitMsg = saInitReq
	ini.State = StateSAInitSent
	resp, err = newResponderSA("ze", respPeer, ikeGroup, testESPGroup(), ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, quiet)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, quiet)
	ini.NATDetected, resp.NATDetected = true, true
	return ini, resp, table
}

// encapPeerAt is the source a NAT gave the peer's IKE in these fixtures.
func encapPeerAt(port uint16) *net.UDPAddr {
	return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: int(port)}
}

// VALIDATES: AC-1 through the responder's entry point. A PSK IKE_AUTH that arrives on the
// NAT-T socket from (A, 14500) installs a Child SA encapsulated to 14500 from the NAT-T
// socket's own port: handleAuthRequest adopts the endpoint, then buildAuthResponse builds
// the Child SA through newChildSA and installChildSA.
// PREVENTS: the responder sending ESP to 4500 while the peer's NAT maps another port.
//
// RFC requirement: RFC3948-2.1-2 positive -- the responder's first Child SA, built from an
// IKE_AUTH that arrived from a translated source port, is encapsulated with the ports that
// IKE_AUTH used.
func TestRFC3948ResponderChildSAEncapsulatesToTheIKEPeerPort(t *testing.T) {
	dp := &mockDP{}
	loadFailClosedBackend(t, dp)
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "encap-port-psk")
	ini, resp, _ := encapSAInit(t, iniPeer, respPeer)
	natt, localPort := bindNATT(t, resp)

	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
	ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, natt, encapPeerAt(encapTestPeerPort), slogutil.DiscardLogger())
	if resp.State != StateEstablished {
		t.Fatalf("the responder did not establish (state %v)", resp.State)
	}
	wantEncap(t, dp.sas, localPort, encapTestPeerPort)
}

// VALIDATES: AC-6 through the EAP responder's entry point. The final IKE_AUTH
// (AUTH-from-MSK) from (A, 14500) installs the Child SA encapsulated to 14500:
// handleResponderEAP adopts the endpoint BEFORE buildAuthResponse (D1b).
// PREVENTS: the EAP path building its Child SA before the endpoint is known, which the
// old 4500 default hid and which childEncapPorts now refuses.
//
// RFC requirement: RFC3948-2.1-2 positive -- the EAP responder's first Child SA is
// encapsulated with the ports the final IKE_AUTH used.
func TestRFC3948EAPResponderChildSAEncapsulatesToTheIKEPeerPort(t *testing.T) {
	quiet := slogutil.DiscardLogger()
	dp := &mockDP{}
	loadFailClosedBackend(t, dp)
	autLoadPKI(t)
	iniPeer, respPeer := autPeers(ipsec.AuthConfig{
		Mode:          ipsec.AuthEAPMSCHAPv2,
		PSK:           "eap-pass",
		Certificate:   autCertName,
		CACertificate: autCAName,
	})
	ini, resp, table := encapSAInit(t, iniPeer, respPeer)
	natt, localPort := bindNATT(t, resp)
	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
	ps.setSA(resp)
	setActivePeers(map[string]*PeerSession{"ze": ps})
	t.Cleanup(func() { setActivePeers(nil) })

	cur := ini.LastSentMsg
	toResponder := true
	for range 24 {
		if toResponder {
			pkt := transport.Packet{Data: cur, RemoteAddr: encapPeerAt(encapTestPeerPort), NATT: true}
			handleInbound(resp, pkt, table, natt, quiet)
			cur = resp.LastSentMsg
		} else {
			handleInbound(ini, transport.Packet{Data: cur}, table, nil, quiet)
			cur = ini.LastSentMsg
		}
		toResponder = !toResponder
		if resp.State == StateDead || ini.State == StateDead {
			t.Fatalf("the EAP handshake died (ini=%v resp=%v)", ini.State, resp.State)
		}
		if ini.State == StateEstablished && resp.State == StateEstablished {
			break
		}
	}
	if resp.State != StateEstablished {
		t.Fatalf("the EAP handshake did not establish (ini=%v resp=%v)", ini.State, resp.State)
	}
	wantEncap(t, dp.sas, localPort, encapTestPeerPort)
}

// encapRunEstablished runs ps.runEstablished over sa with the session already stopped,
// so it installs the first Child SA and returns.
func encapRunEstablished(t *testing.T, ps *PeerSession, sa *SA, table *SATable, tr *transport.UDPTransport) {
	t.Helper()
	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)
	close(ps.stopCh)
	done := make(chan error, 1)
	go func() {
		done <- ps.runEstablished(sa, ps.peerCfg, testIKEGroup(), table, tr, nil, slogutil.DiscardLogger())
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runEstablished: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runEstablished did not return after the session was stopped")
	}
}

// VALIDATES: AC-7 through the initiator's entry point. The authenticated IKE_AUTH
// response arrives on the NAT-T socket from (B, 14501); handleInbound adopts that
// endpoint, and runEstablished's first Child SA (initiatorFirstChildSA) is encapsulated
// to 14501 from the NAT-T socket's own port.
// PREVENTS: the initiator encapsulating to the configured port instead of the one the
// responder's NAT answered from.
func TestRFC3948InitiatorChildSAEncapsulatesToTheIKEPeerPort(t *testing.T) {
	const respondedFrom uint16 = 14501
	quiet := slogutil.DiscardLogger()
	dp := &mockDP{}
	loadFailClosedBackend(t, dp)
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "encap-port-initiator")
	ini, resp, table := encapSAInit(t, iniPeer, respPeer)
	respNATT, _ := bindNATT(t, resp)
	_, localPort := bindNATT(t, ini)

	respPS := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
	respPS.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, respNATT, encapPeerAt(encapTestPeerPort), quiet)
	if resp.State != StateEstablished {
		t.Fatalf("the responder did not establish (state %v)", resp.State)
	}
	dp.sas = nil

	pkt := transport.Packet{Data: resp.LastSentMsg, RemoteAddr: encapPeerAt(respondedFrom), NATT: true}
	handleInbound(ini, pkt, table, nil, quiet)
	if ini.State != StateEstablished || ini.IKEAuthChildless {
		t.Fatalf("the initiator reached state %v childless=%v, want established with a Child SA", ini.State, ini.IKEAuthChildless)
	}
	ps := &PeerSession{peerName: "ze", peerCfg: iniPeer, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
	encapRunEstablished(t, ps, ini, table, nil)
	wantEncap(t, dp.sas, localPort, respondedFrom)
}

// deleteListsSPI reads up to three datagrams, skipping NAT keepalives, at the stand-in peer and reports whether one
// is an INFORMATIONAL Delete whose ESP SPI list holds spi.
func deleteListsSPI(t *testing.T, peer *SA, peerTr *transport.UDPTransport, spi uint32) bool {
	t.Helper()
	for range 3 {
		raw := rtxRecv(t, peerTr)
		if raw == nil {
			return false
		}
		if peerTr.IsNATT() {
			// IKE on the NAT-T port carries the non-ESP marker; a NAT keepalive
			// (RFC 3948 Section 2.3) does not, and is not an IKE message.
			stripped, isIKE := transport.StripNonESPMarker(raw)
			if !isIKE {
				continue
			}
			raw = stripped
		}
		inner, err := decryptAndParse(peer, parseMsg(t, raw), raw)
		if err != nil {
			t.Fatalf("the peer could not decrypt what ze sent: %v", err)
		}
		for i := range inner {
			del, ok := inner[i].Payload.(*wire.PayloadDelete)
			if !ok || del.ProtocolID != wire.ProtocolESP {
				continue
			}
			for off := 0; off+4 <= len(del.SPIs); off += 4 {
				if binary.BigEndian.Uint32(del.SPIs[off:]) == spi {
					return true
				}
			}
		}
	}
	return false
}

// encapNATTPeerLink is rtxPeerLink for an SA that floated to NAT-T: ze sends from its
// NAT-T socket to nattAddr, which the ze.test.ike.port seam puts at the seam's port + 1,
// so the stand-in peer is a NAT-T socket bound there. myTr is the IKE socket ze is
// handed; a floated SA does not send from it.
func encapNATTPeerLink(t *testing.T) (peerTr, myTr *transport.UDPTransport) {
	t.Helper()
	peerTr = mbTransport(t, "127.0.0.1:0", true)
	myTr = mbTransport(t, "127.0.0.1:0", false)
	port := nttPeerAddr(t, peerTr).Port
	oldPortFn := ikeTestPortFn
	ikeTestPortFn = func() string { return strconv.Itoa(port - 1) }
	t.Cleanup(func() { ikeTestPortFn = oldPortFn })
	return peerTr, myTr
}

// encapMOBIKE turns MOBIKE on for sa, with a recorded local path, so the refusal is
// proven not to come from a missing MOBIKE local port.
func encapMOBIKE(sa *SA, on bool) {
	if !on {
		return
	}
	sa.mobike.enabled = true
	sa.mobike.local = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: int(transport.NATTPort)}
}

// VALIDATES: AC-10 at the session level, each path with and without MOBIKE. Where ze
// INITIATED the exchange (IKE_AUTH, CREATE_CHILD_SA, Child SA rekey) the peer has already
// installed its half when the refusal is found, so ze sends an INFORMATIONAL Delete
// listing its own inbound SPI of the refused pair, the SPI "as they would be expected in
// the headers of inbound packets" (RFC 7296 Section 1.4.1), installs nothing and keeps
// the IKE SA. Where ze ANSWERS (IKE_AUTH, CREATE_CHILD_SA) it refuses before anything is
// installed: a childless IKE_AUTH response, or an error notify, and the IKE SA stays up.
// PREVENTS: a refused Child SA that leaves the peer encrypting to a pair ze never
// installed, or that costs the IKE SA (RFC 7296 Section 2.21.2: "This failure does not
// automatically cause the IKE SA to be deleted.").
func TestRFC3948ChildSARefusalDeletesThePeersHalf(t *testing.T) {
	for _, mobike := range []bool{false, true} {
		name := map[bool]string{false: "no mobike", true: "mobike"}[mobike]

		t.Run("initiator IKE_AUTH/"+name, func(t *testing.T) {
			quiet := slogutil.DiscardLogger()
			dp := &mockDP{}
			loadFailClosedBackend(t, dp)
			peerTr, myTr := encapNATTPeerLink(t)
			iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "encap-refusal-initiator")
			iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
			ini, resp, table := encapSAInit(t, iniPeer, respPeer)
			respNATT, _ := bindNATT(t, resp)
			bindNATT(t, ini)
			respPS := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
			respPS.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, respNATT, encapPeerAt(encapTestPeerPort), quiet)
			dp.sas = nil

			// Delivered with no source address: the response authenticates, frees the
			// request window as on the wire, and the initiator records no endpoint.
			handleInbound(ini, transport.Packet{Data: resp.LastSentMsg}, table, myTr, quiet)
			if ini.State != StateEstablished || ini.peerEndpoint != nil {
				t.Fatalf("setup: initiator state %v endpoint %v, want established with no endpoint", ini.State, ini.peerEndpoint)
			}
			encapMOBIKE(ini, mobike)
			ps := &PeerSession{peerName: "ze", peerCfg: iniPeer, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
			encapRunEstablished(t, ps, ini, table, myTr)

			if len(dp.sas) != 0 {
				t.Errorf("%d states installed, want none", len(dp.sas))
			}
			if !deleteListsSPI(t, resp, peerTr, ini.ChildInboundSPI) {
				t.Errorf("no ESP Delete listing ze's inbound SPI %#x reached the peer", ini.ChildInboundSPI)
			}
		})

		t.Run("CREATE_CHILD_SA initiated/"+name, func(t *testing.T) {
			r := createRig(t, ipsec.PFSDisable)
			r.sa.NATDetected = true
			bindNATT(t, r.sa)
			r.sa.peerEndpoint = nil
			encapMOBIKE(r.sa, mobike)
			installed := len(r.dp.installed)
			r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), slogutil.DiscardLogger())
			if r.ps.pendingRekey == nil {
				t.Fatal("a due creation sent no request")
			}
			inSPI := r.ps.pendingRekey.newInboundSPI
			var peerChild *ChildSA
			_, out := r.relayCreate(t, r.peerAnswersCreate(t, &peerChild))
			if out.createdChild != nil || r.ps.getChildSA() != nil || len(r.dp.installed) != installed {
				t.Error("a creation refused for want of an encapsulation port installed a Child SA")
			}
			if !deleteListsSPI(t, r.peer, r.peerTr, inSPI) {
				t.Errorf("no ESP Delete listing ze's inbound SPI %#x reached the peer", inSPI)
			}
			if r.sa.State != StateEstablished {
				t.Errorf("the IKE SA is %v, want established", r.sa.State)
			}
		})

		t.Run("Child SA rekey initiated/"+name, func(t *testing.T) {
			r := newRefusedRig(t)
			r.sa.NATDetected = true
			bindNATT(t, r.sa)
			r.sa.peerEndpoint = encapPeerAt(encapTestPeerPort)
			old, err := createFirstChildSA(r.sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, r.dp, slogutil.DiscardLogger())
			if err != nil {
				t.Fatalf("createFirstChildSA: %v", err)
			}
			r.ps.setChildSA(old)
			r.old = old
			installed := len(r.dp.installed)
			r.sa.peerEndpoint = nil
			encapMOBIKE(r.sa, mobike)

			r.ps.startChildRekey(r.sa, r.myTr, slogutil.DiscardLogger())
			if r.ps.pendingRekey == nil || r.ps.pendingRekey.kind != rekeyChild {
				t.Fatal("the rekey left no outstanding Child rekey")
			}
			inSPI := r.ps.pendingRekey.newInboundSPI
			answer := append([]wire.PayloadEntry{
				{Payload: espSAPayload(0x99AABBCC)},
				{Payload: &wire.PayloadNonce{NonceData: testNonce(81)}},
			}, childRekeyAnswerTS(t, old)...)
			r.relayCreate(t, r.peerScripted(t, answer))
			if len(r.dp.installed) != installed {
				t.Errorf("the refused rekey installed %d states", len(r.dp.installed)-installed)
			}
			if r.ps.getChildSA() != old {
				t.Error("the refused rekey replaced the Child SA in use")
			}
			if !deleteListsSPI(t, r.peer, r.peerTr, inSPI) {
				t.Errorf("no ESP Delete listing ze's inbound SPI %#x reached the peer", inSPI)
			}
		})

		t.Run("responder IKE_AUTH/"+name, func(t *testing.T) {
			dp := &mockDP{}
			loadFailClosedBackend(t, dp)
			iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "encap-refusal-responder")
			ini, resp, _ := encapSAInit(t, iniPeer, respPeer)
			bindNATT(t, resp)
			encapMOBIKE(resp, mobike)
			ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
			// No source address: the request authenticates and no endpoint is recorded.
			ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, slogutil.DiscardLogger())
			if resp.State != StateEstablished {
				t.Fatalf("the responder left the authenticated SA in state %v, want established", resp.State)
			}
			if len(dp.sas) != 0 || ps.getChildSA() != nil {
				t.Errorf("%d states installed, want none", len(dp.sas))
			}
			for _, pe := range lcyDecrypt(t, ini, resp.LastSentMsg) {
				switch p := pe.Payload.(type) {
				case *wire.PayloadSA, *wire.PayloadTS:
					t.Errorf("the refusing IKE_AUTH response carries payload type %d", p.Type())
				}
			}
		})

		t.Run("CREATE_CHILD_SA answered/"+name, func(t *testing.T) {
			r := createRig(t, ipsec.PFSDisable)
			r.sa.NATDetected = true
			bindNATT(t, r.sa)
			r.sa.peerEndpoint = nil
			encapMOBIKE(r.sa, mobike)
			installed := len(r.dp.installed)
			inner := peerNewChildRequest(t, r.ps.espGroup, 0x0c0ffee2, testNonce(82), nil, false, "0.0.0.0/0", "0.0.0.0/0")
			msg := &wire.Message{Header: wire.Header{MessageID: r.sa.ExpectedMsgID}}
			out := r.ps.handleCreateChildSAOwned(r.sa, msg, inner, false, r.myTr, r.dp, slogutil.DiscardLogger())
			raw := rtxRecv(t, r.peerTr)
			if raw == nil {
				t.Fatal("the refused request drew no answer")
			}
			if types := errNotifyIn(t, r.peer, raw); len(types) == 0 {
				t.Error("the refusal carries no error notify")
			}
			if out.createdChild != nil || r.ps.getChildSA() != nil || len(r.dp.installed) != installed {
				t.Error("a refused request installed a Child SA")
			}
			if r.sa.State != StateEstablished {
				t.Errorf("the IKE SA is %v, want established", r.sa.State)
			}
		})
	}
}
