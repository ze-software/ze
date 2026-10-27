// Design: docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md -- RFC 7296 Section 2.23.1 transport mode NAT traversal
// Related: rfc7296_transport_nat_rekey_test.go -- the rig and the producer-level tests
// Related: inbound.go -- handleOwnedInbound, the session's entry point for every CREATE_CHILD_SA datagram
// Related: established.go -- adoptRekeyedIKESA, the IKE SA swap
// VALIDATES: the NATed transport-mode Child SA rekey after an IKE SA rekey reaches the
// substitution through the session entry point production runs, in both roles, and a
// rekey request that drops USE_TRANSPORT_MODE behind a NAT is refused rather than
// installed as a tunnel naming the NAT's address.
// PREVENTS: a producer-level pass that the session path does not reach.
// RFC naming: untagged -- the session-entry companion of rfc7296_transport_nat_rekey_test.go, whose tagged units carry the RFC 7296 Section 2.23.1 claims

package engine

import (
	"net"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// natSessionRig is natRekeyRig reached through ze's PeerSession: the peer rekeyed the
// IKE SA, ze answered it in handleOwnedInbound, the peer's Delete of the old IKE
// SA confirmed it in handleOwnedInbound, and adoptRekeyedIKESA swapped the successor in,
// as runEstablished does.
type natSessionRig struct {
	peer, ze           *SA
	peerChild, zeChild *ChildSA
	ps                 *PeerSession
}

// natOriginals is natRekeySetup's rig before its IKE SA rekey: the original pair, ze's
// session (the responder's, as establishPSK builds it) and the first transport-mode
// Child SA of each end, set up exactly as natRekeySetup sets them up.
type natOriginals struct {
	ini, resp          *SA
	ps                 *PeerSession
	peerChild, zeChild *ChildSA
}

// natOriginalsSetup builds natOriginals with both NATs present.
func natOriginalsSetup(t *testing.T) *natOriginals {
	t.Helper()
	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)
	ini.PeerCfg.LocalAddress, ini.PeerCfg.RemoteAddress = natClientReal, natServerPublic
	resp.PeerCfg.LocalAddress, resp.PeerCfg.RemoteAddress = natServerReal, natClientPublic
	ini.peerEndpoint = &net.UDPAddr{IP: net.ParseIP(natServerPublic), Port: 4500}
	resp.peerEndpoint = &net.UDPAddr{IP: net.ParseIP(natClientPublic), Port: 4500}
	for _, sa := range []*SA{ini, resp} {
		sa.PeerCfg.Mode = dataplane.ModeTransport
		sa.UseTransportMode = true
		sa.nattSocket = mbTransport(t, "127.0.0.1:0", true)
		sa.NATDetected = true
		sa.BehindNAT, sa.PeerBehindNAT = true, true
	}
	peerPair := []tsPair{{I: hostSel(t, natClientReal), R: hostSel(t, natServerPublic)}}
	zePair := []tsPair{{I: hostSel(t, natClientPublic), R: hostSel(t, natServerReal)}}
	ini.NegotiatedPairs, ini.NegotiatedTSi, ini.NegotiatedTSr = peerPair, peerPair[0].I.Net, peerPair[0].R.Net
	resp.NegotiatedPairs, resp.NegotiatedTSi, resp.NegotiatedTSr = zePair, zePair[0].I.Net, zePair[0].R.Net
	peerChild, err := createFirstChildSA(ini, testESPGroup(), ini.PeerCfg.LocalAddress, ini.PeerCfg.RemoteAddress, 1, nil, log)
	if err != nil {
		t.Fatalf("the peer's first Child SA: %v", err)
	}
	t.Cleanup(peerChild.Clear)
	zeChild, err := createFirstChildSA(resp, testESPGroup(), resp.PeerCfg.LocalAddress, resp.PeerCfg.RemoteAddress, 1, nil, log)
	if err != nil {
		t.Fatalf("ze's first Child SA: %v", err)
	}
	t.Cleanup(zeChild.Clear)
	if peerChild.Mode != modeTransport || zeChild.Mode != modeTransport {
		t.Fatalf("setup: first Child SAs run modes %d and %d, want transport", peerChild.Mode, zeChild.Mode)
	}
	return &natOriginals{ini: ini, resp: resp, ps: ps, peerChild: peerChild, zeChild: zeChild}
}

// requireNATSuccessors asserts the state the defect needs on the successors of the IKE
// SA rekey: no IKE_AUTH mode carried, and ze's NAT verdict inherited.
func requireNATSuccessors(t *testing.T, peer, ze *SA) {
	t.Helper()
	if peer.UseTransportMode || ze.UseTransportMode {
		t.Fatalf("setup: a successor IKE SA carries UseTransportMode (peer %v, ze %v); the rekey must not",
			peer.UseTransportMode, ze.UseTransportMode)
	}
	if !ze.PeerBehindNAT || !ze.BehindNAT {
		t.Fatal("setup: the IKE SA rekey did not carry ze's NAT verdict (inheritSendPath)")
	}
}

// natSessionSetup builds natSessionRig with both NATs present.
func natSessionSetup(t *testing.T) *natSessionRig {
	t.Helper()
	log := slogutil.DiscardLogger()
	o := natOriginalsSetup(t)
	o.ps.setChildSA(o.zeChild)

	req, pending, err := initiateIKERekey(o.ini, o.ini.IKEGroup)
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	defer pending.clear()
	reqID := parseMsg(t, req).Header.MessageID
	o.ps.handleOwnedInbound(o.resp, transport.Packet{Data: req}, nil, nil, log)
	if o.resp.ExpectedMsgID != reqID+1 {
		t.Fatalf("setup: ze did not accept the peer's IKE SA rekey request: expected id %d, want %d",
			o.resp.ExpectedMsgID, reqID+1)
	}
	if o.ps.pendingIKESwap == nil {
		t.Fatal("setup: the peer's IKE SA rekey staged no swap")
	}
	peer, err := applyIKERekeyResponse(o.ini, pending, mbDecrypt(t, o.ini, o.resp.lastResponse), log)
	if err != nil {
		t.Fatalf("the peer refused ze's IKE SA rekey answer: %v", err)
	}
	del := lcyRequest(t, o.ini, o.resp.ExpectedMsgID,
		[]wire.PayloadEntry{{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}}})
	out := o.ps.handleOwnedInbound(o.resp, transport.Packet{Data: del}, nil, nil, log)
	if out.newSA == nil {
		t.Fatal("setup: the peer's Delete of the old IKE SA produced no successor")
	}
	o.ps.adoptRekeyedIKESA(o.resp, out.newSA, NewSATable(), log)
	ze := out.newSA
	if o.ps.getSA() != ze {
		t.Fatal("setup: the session does not run the successor IKE SA")
	}
	requireNATSuccessors(t, peer, ze)
	if o.ps.getChildSA() != o.zeChild {
		t.Fatal("setup: the IKE SA rekey did not keep the transport-mode Child SA")
	}
	return &natSessionRig{peer: peer, ze: ze, peerChild: o.peerChild, zeChild: o.zeChild, ps: o.ps}
}

// TestTransportChildRekeyAfterIKERekeyThroughSession covers AC-1 and AC-3 at the entry
// point the Wiring Test names.
//
// Goal: the session path production runs, not only the rekey producers, substitutes the
// NATed transport-mode Child SA rekey after an IKE SA rekey and installs exactly one host
// pair. Method: natSessionSetup rekeys the IKE SA through ze's PeerSession; the Child SA
// rekey then enters handleOwnedInbound, which classifies and decrypts it before
// handleCreateChildSAOwned runs, as a request (the peer rekeys) or as the
// response to ze's own request, recorded where startChildRekey records it (ze rekeys).
func TestTransportChildRekeyAfterIKERekeyThroughSession(t *testing.T) {
	log := slogutil.DiscardLogger()

	t.Run("peer_rekeys", func(t *testing.T) {
		r := natSessionSetup(t)
		raw, pending, err := initiateChildRekey(r.peer, r.peerChild)
		if err != nil {
			t.Fatalf("the peer's initiateChildRekey: %v", err)
		}
		defer pending.clear()
		reqID := parseMsg(t, raw).Header.MessageID
		out := r.ps.handleOwnedInbound(r.ze, transport.Packet{Data: raw}, nil, &mockDP{}, log)
		if out.newChild == nil {
			t.Fatal("ze's session refused the peer's transport-mode Child SA rekey")
		}
		defer out.newChild.Clear()
		if r.ze.ExpectedMsgID != reqID+1 {
			t.Fatalf("ze's expected Message ID is %d after the rekey request, want %d", r.ze.ExpectedMsgID, reqID+1)
		}
		if r.ps.getChildSA() != out.newChild {
			t.Error("the session does not hold the replacement Child SA")
		}
		if out.newChild.Mode != modeTransport {
			t.Errorf("the replacement runs mode %d, want transport (%d)", out.newChild.Mode, modeTransport)
		}
		want := pairsText([]tsPair{{I: hostSel(t, natClientPublic), R: hostSel(t, natServerReal)}})
		if got := pairsText(out.newChild.Selectors); len(out.newChild.Selectors) != 1 || got != want {
			t.Errorf("the replacement selectors = %s, want the one substituted host pair %s", got, want)
		}
		answer := mbDecrypt(t, r.peer, r.ze.lastResponse)
		if !cctCarriesTransportNotify(answer) {
			t.Error("ze's answer carries no USE_TRANSPORT_MODE")
		}
		tsi, tsr := tsAddrs(answer)
		if len(tsi) != 1 || tsi[0] != natClientPublic || len(tsr) != 1 || tsr[0] != natServerReal {
			t.Errorf("ze answered TSi %v TSr %v, want the substituted [%s] [%s]", tsi, tsr, natClientPublic, natServerReal)
		}
	})

	t.Run("ze_rekeys", func(t *testing.T) {
		r := natSessionSetup(t)
		if !r.ze.reserveRequestWindow() {
			t.Fatal("setup: the successor IKE SA has a request outstanding")
		}
		raw, pending, err := initiateChildRekey(r.ze, r.zeChild)
		if err != nil {
			t.Fatalf("ze's initiateChildRekey: %v", err)
		}
		r.ps.pendingRekey = pending
		answer, peerNext, err := respondChildRekey(r.peer, mbDecrypt(t, r.peer, raw), r.peerChild,
			parseMsg(t, raw).Header.MessageID, nil, log)
		if err != nil {
			t.Fatalf("the peer refused ze's transport-mode Child SA rekey: %v", err)
		}
		defer peerNext.Clear()
		out := r.ps.handleOwnedInbound(r.ze, transport.Packet{Data: answer}, nil, &mockDP{}, log)
		if out.newChild == nil {
			t.Fatal("ze's session refused the peer's answer to its transport-mode rekey")
		}
		defer out.newChild.Clear()
		if r.ps.pendingRekey != nil {
			t.Fatal("the answer left ze's Child SA rekey pending")
		}
		if r.ze.reserveRequestWindow() {
			r.ze.releaseRequestWindow()
		} else {
			t.Error("the answer did not free ze's request window")
		}
		if r.ps.getChildSA() != out.newChild {
			t.Error("the session does not hold the replacement Child SA")
		}
		if out.newChild.Mode != modeTransport {
			t.Errorf("the replacement runs mode %d, want transport (%d)", out.newChild.Mode, modeTransport)
		}
		want := pairsText([]tsPair{{I: hostSel(t, natServerReal), R: hostSel(t, natClientPublic)}})
		got := pairsText(out.newChild.Selectors)
		if len(out.newChild.Selectors) != 1 || got != want || !out.newChild.SelectorsLocalIsTSi {
			t.Errorf("the replacement selectors = %s (local is TSi %v), want the one host pair %s in ze's space, local TSi",
				got, out.newChild.SelectorsLocalIsTSi, want)
		}
	})
}

// TestChildRekeyWithoutTransportNotifyBehindNATIsRefused covers AC-5 behind a NAT.
//
// Goal: a rekey request that drops USE_TRANSPORT_MODE for a NATed transport-mode Child
// SA is never installed as a tunnel. RFC 7296 Section 1.3.1: "Except when using this
// option to negotiate transport mode, all Child SAs will use tunnel mode." Without the
// notify the request asks for tunnel mode, and the proposal is judged unsubstituted, so
// it names the NAT's public address for ze (203.0.113.2) and the peer's real one, a
// pair that excludes the retired host pair; Section 2.9.2 forbids narrowing below the
// scope in use, so the answer is TS_UNACCEPTABLE. Method: the real rekey request,
// stripped of the notify and sealed again with the peer's keys, enters ze's session
// through handleOwnedInbound after a real IKE SA rekey; the session must answer
// TS_UNACCEPTABLE and keep the transport Child SA.
func TestChildRekeyWithoutTransportNotifyBehindNATIsRefused(t *testing.T) {
	log := slogutil.DiscardLogger()
	r := natSessionSetup(t)
	raw, pending, err := initiateChildRekey(r.peer, r.peerChild)
	if err != nil {
		t.Fatalf("the peer's initiateChildRekey: %v", err)
	}
	defer pending.clear()
	inner := withoutTransportNotify(mbDecrypt(t, r.ze, raw))
	stripped, err := buildEncryptedMessageEx(r.peer, inner, parseMsg(t, raw).Header.MessageID,
		wire.ExchangeCreateChildSA, initiatorFlag(r.peer))
	if err != nil {
		t.Fatalf("seal the stripped rekey request: %v", err)
	}
	out := r.ps.handleOwnedInbound(r.ze, transport.Packet{Data: stripped}, nil, &mockDP{}, log)
	if out.newChild != nil {
		defer out.newChild.Clear()
		t.Fatalf("the session installed a mode %d replacement for a tunnel rekey of a NATed transport Child SA",
			out.newChild.Mode)
	}
	if got := errNotifyIn(t, r.peer, r.ze.lastResponse); !slices.Contains(got, wire.NotifyTSUnacceptable) {
		t.Errorf("ze answered notifies %v, want TS_UNACCEPTABLE (%d)", got, wire.NotifyTSUnacceptable)
	}
	if r.ps.getChildSA() != r.zeChild {
		t.Error("the refused rekey replaced the session's transport-mode Child SA")
	}
}
