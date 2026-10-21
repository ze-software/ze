// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- Child SA rekey
// Related: rekey.go -- initiateChildRekey, applyChildRekeyResponse and respondChildRekey
// Related: transport_mode.go -- transportModeNotify

package engine

import (
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// ctrRig is an established IKE SA pair with one installed Child SA on each end. ze is the
// IKE SA initiator (ini) and the peer the responder (resp). zeTransport and peerTransport
// choose the mode each end's Child SA runs, so a test can give the two ends the same mode
// or make the peer the end that runs tunnel.
type ctrRig struct {
	ini, resp          *SA
	zeChild, peerChild *ChildSA
}

func ctrSetup(t *testing.T, zeTransport, peerTransport bool) *ctrRig {
	t.Helper()
	log := slogutil.DiscardLogger()
	ini, resp, _ := establishPSK(t)
	ini.UseTransportMode = zeTransport
	resp.UseTransportMode = peerTransport
	// A transport-mode end runs host-to-host: its selectors are the IKE SA's own address
	// pair (RFC 7296 Section 2.23.1), so the IKE_AUTH scope both ends hold is that pair,
	// in IKE_AUTH's orientation (TSi is the initiator's address). The rekey's narrowing
	// floor is this scope.
	if zeTransport {
		ini.PeerCfg.Mode = dataplane.ModeTransport
	}
	if peerTransport {
		resp.PeerCfg.Mode = dataplane.ModeTransport
	}
	hostPair := []tsPair{{
		I: tsSelector{Net: mustNet(t, ini.PeerCfg.LocalAddress+"/32"), Port: ipsec.AnyPort()},
		R: tsSelector{Net: mustNet(t, ini.PeerCfg.RemoteAddress+"/32"), Port: ipsec.AnyPort()},
	}}
	for _, sa := range []*SA{ini, resp} {
		sa.NegotiatedPairs = hostPair
		sa.NegotiatedTSi, sa.NegotiatedTSr = hostPair[0].I.Net, hostPair[0].R.Net
	}
	zeChild, err := createFirstChildSA(ini, testESPGroup(), ini.PeerCfg.LocalAddress, ini.PeerCfg.RemoteAddress, 1, nil, log)
	if err != nil {
		t.Fatalf("ze's first Child SA: %v", err)
	}
	t.Cleanup(zeChild.Clear)
	peerChild, err := createFirstChildSA(resp, testESPGroup(), resp.PeerCfg.LocalAddress, resp.PeerCfg.RemoteAddress, 1, nil, log)
	if err != nil {
		t.Fatalf("the peer's first Child SA: %v", err)
	}
	t.Cleanup(peerChild.Clear)
	if want := modeOf(zeTransport); zeChild.Mode != want {
		t.Fatalf("setup: ze's Child SA runs mode %d, want %d", zeChild.Mode, want)
	}
	if want := modeOf(peerTransport); peerChild.Mode != want {
		t.Fatalf("setup: the peer's Child SA runs mode %d, want %d", peerChild.Mode, want)
	}
	return &ctrRig{ini: ini, resp: resp, zeChild: zeChild, peerChild: peerChild}
}

func modeOf(transport bool) uint8 {
	if transport {
		return modeTransport
	}
	return modeTunnel
}

// VALIDATES: a Child SA rekey request carries USE_TRANSPORT_MODE exactly when the SA it
// replaces runs transport mode.
//
// Goal: the peer reads the rekey as a transport-mode request. RFC 7296 Section 1.3.3: "The
// notifications described in Section 1.3.1 may also be sent in a rekeying exchange.
// Usually, these will be the same notifications that were used in the original exchange;
// for example, when rekeying a transport mode SA, the USE_TRANSPORT_MODE notification
// will be used." Section 1.3.1 gives the request without it its meaning: "Except when
// using this option to negotiate transport mode, all Child SAs will use tunnel mode."
// Method: ze builds the rekey of a transport and of a tunnel Child SA, and the peer
// decrypts each request off the wire bytes.
//
// PREVENTS: the AC-14 failure measured against strongSwan 5.9.14 behind a NAT. charon's
// process_payloads (child_create.c) defaults "this->mode = MODE_TUNNEL" and only the
// notify sets MODE_TRANSPORT; narrow_ts then skips its transport-mode NAT substitution,
// the TSr ze names (the peer's public address) matches none of charon's own addresses,
// and charon answers TS_UNACCEPTABLE ("no acceptable traffic selectors found").
func TestChildRekeyRequestCarriesUseTransportModeForATransportSA(t *testing.T) {
	for _, tc := range []struct {
		name      string
		transport bool
	}{
		{"transport", true},
		{"tunnel", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := ctrSetup(t, tc.transport, tc.transport)
			raw, pending, err := initiateChildRekey(r.ini, r.zeChild)
			if err != nil {
				t.Fatalf("initiateChildRekey: %v", err)
			}
			defer pending.clear()
			got := cctCarriesTransportNotify(mbDecrypt(t, r.resp, raw))
			if got != tc.transport {
				t.Errorf("the rekey of a %s Child SA carries USE_TRANSPORT_MODE = %v, want %v",
					tc.name, got, tc.transport)
			}
		})
	}
}

// VALIDATES: a responder that accepts a transport-mode Child SA rekey says so with
// USE_TRANSPORT_MODE, and ze, rekeying a transport Child SA, installs the replacement in
// transport mode when the answer carries it.
//
// Goal: both ends keep transport mode across a rekey either end starts. RFC 7296 Section
// 1.3.1: "If the request is accepted, the response MUST also include a notification of
// type USE_TRANSPORT_MODE." Method: ze's real rekey request is answered by the peer's real
// respondChildRekey, whose answer ze decrypts and applies.
//
// PREVENTS: ze answering a strongSwan-initiated transport rekey without the notify, which
// charon's process_payloads reads as tunnel mode and installs as tunnel mode, while ze
// installs transport.
func TestChildRekeyTransportAnswerCarriesUseTransportMode(t *testing.T) {
	log := slogutil.DiscardLogger()
	r := ctrSetup(t, true, true)
	raw, pending, err := initiateChildRekey(r.ini, r.zeChild)
	if err != nil {
		t.Fatalf("initiateChildRekey: %v", err)
	}
	defer pending.clear()
	answer, peerReplacement, err := respondChildRekey(r.resp, mbDecrypt(t, r.resp, raw), r.peerChild,
		parseMsg(t, raw).Header.MessageID, nil, log)
	if err != nil {
		t.Fatalf("respondChildRekey: %v", err)
	}
	defer peerReplacement.Clear()
	if peerReplacement.Mode != modeTransport {
		t.Errorf("the peer's replacement runs mode %d, want transport", peerReplacement.Mode)
	}
	inner := mbDecrypt(t, r.ini, answer)
	if !cctCarriesTransportNotify(inner) {
		t.Fatal("the answer to a transport-mode rekey carries no USE_TRANSPORT_MODE")
	}
	child, err := applyChildRekeyResponse(r.ini, pending, inner, nil, log)
	if err != nil {
		t.Fatalf("applyChildRekeyResponse refused a transport-mode answer: %v", err)
	}
	defer child.Clear()
	if child.Mode != modeTransport {
		t.Errorf("ze's replacement runs mode %d, want transport", child.Mode)
	}
}

// VALIDATES: ze refuses the answer to a transport-mode rekey that leaves USE_TRANSPORT_MODE
// out, and installs no replacement.
//
// Goal: ze never runs a transport-mode replacement opposite a peer that built a tunnel-mode
// one. RFC 7296 Section 1.3.1: "If the responder declines the request, the Child SA will
// be established in tunnel mode. If this is unacceptable to the initiator, the initiator
// MUST delete the SA." The replacement inherits the retired SA's transport mode, so a
// tunnel answer is unacceptable; the rekey response handler's errChildRekeyTransportDeclined
// branch (inbound.go) deletes the pair the peer installed, which
// TestChildRekeyTransportRefusalDeletesThePairAndHolds proves. Method: the peer's Child SA
// runs tunnel mode, so its real respondChildRekey answers without the notify.
//
// PREVENTS: the two ends of one Child SA running different modes after a rekey.
func TestChildRekeyInitiatorRefusesATunnelAnswerToATransportRekey(t *testing.T) {
	log := slogutil.DiscardLogger()
	r := ctrSetup(t, true, false)
	raw, pending, err := initiateChildRekey(r.ini, r.zeChild)
	if err != nil {
		t.Fatalf("initiateChildRekey: %v", err)
	}
	defer pending.clear()
	answer, peerReplacement, err := respondChildRekey(r.resp, mbDecrypt(t, r.resp, raw), r.peerChild,
		parseMsg(t, raw).Header.MessageID, nil, log)
	if err != nil {
		t.Fatalf("respondChildRekey: %v", err)
	}
	defer peerReplacement.Clear()
	inner := mbDecrypt(t, r.ini, answer)
	if cctCarriesTransportNotify(inner) {
		t.Fatal("setup: a tunnel-mode peer answered with USE_TRANSPORT_MODE")
	}
	child, err := applyChildRekeyResponse(r.ini, pending, inner, nil, log)
	if err == nil {
		child.Clear()
		t.Fatal("ze installed a transport-mode replacement for an answer in tunnel mode")
	}
	if !errors.Is(err, errChildRekeyTransportDeclined) {
		t.Errorf("refusal = %v, want errChildRekeyTransportDeclined", err)
	}
	if child != nil {
		t.Error("a refused rekey answer returned a replacement Child SA")
	}
}

// VALIDATES: through the owner loop, ze answers a tunnel-mode answer to its transport-mode
// rekey with an ESP Delete of the new pair, keeps the Child SA in use, and holds the next
// attempt for the refusal wait.
//
// Goal: the refusal deletes what the peer installed and does not repeat every second. RFC
// 7296 Section 1.3.1: "If this is unacceptable to the initiator, the initiator MUST delete
// the SA." The peer's configuration decides the answer, so an immediate retry draws the
// same one, and the owed rebind rekey is a level trigger on the one-second tick. Method:
// the rebind fixture with a transport-mode Child SA owes a rekey; the peer, whose Child SA
// runs tunnel mode, answers through its real respondChildRekey; the answer is delivered to
// handleOwnedInbound, and the next tick runs.
//
// PREVENTS: a CREATE_CHILD_SA and a Delete every second against a peer configured for
// tunnel mode, until the Child SA's hard lifetime.
func TestChildRekeyTransportRefusalDeletesThePairAndHolds(t *testing.T) {
	f := rbOwner(t, rbOptions{transport: true})
	f.peerChild.Mode = modeTunnel
	f.dpd(t, f.movedTr, false)
	f.tick()
	if f.ps.pendingRekey == nil {
		t.Fatal("the owed rekey sent no request")
	}
	newInbound := f.ps.pendingRekey.newInboundSPI
	req := mbReceive(t, f.movedTr)
	answer, peerReplacement, err := respondChildRekey(f.peer, mbDecrypt(t, f.peer, req.Data), f.peerChild,
		parseMsg(t, req.Data).Header.MessageID, nil, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("the peer answers the Child SA rekey: %v", err)
	}
	defer peerReplacement.Clear()
	f.ps.handleOwnedInbound(f.local, transport.Packet{
		Data: answer, RemoteAddr: nttPeerAddr(t, f.movedTr), LocalAddr: nttPeerAddr(t, f.myTr), NATT: true,
	}, f.myTr, f.dp, f.log)

	if f.ps.getChildSA() != f.child {
		t.Fatal("a refused tunnel-mode answer replaced the Child SA in use")
	}
	del := mbReceive(t, f.movedTr)
	if !ctrDeletesESP(mbDecrypt(t, f.peer, del.Data), newInbound) {
		t.Errorf("the peer received no ESP Delete for the new pair's SPI %#x", newInbound)
	}
	f.wantLog(t, childRekeyFixTransport)
	if !rekeyHeld(f.ps.childRekeyRefusedUntil, time.Now()) {
		t.Fatal("the refusal set no rekey hold")
	}
	f.tick()
	if f.ps.pendingRekey != nil {
		t.Fatal("the next tick sent the refused rekey again inside the hold")
	}
}

// ctrDeletesESP reports whether a payload chain carries an ESP Delete naming spi.
func ctrDeletesESP(inner []wire.PayloadEntry, spi uint32) bool {
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
	return false
}
