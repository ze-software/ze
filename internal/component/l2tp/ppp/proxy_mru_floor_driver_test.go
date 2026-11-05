package ppp

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

// TestProxyMRUBelowIPv4FloorRenegotiates enters through SessionsIn with a
// valid local policy and permitted PAP. A small peer receive limit must not
// reach the shortcut whose interface floor would exceed that limit. After
// fresh negotiation corrects it, the same subscriber must remain usable.
func TestProxyMRUBelowIPv4FloorRenegotiates(t *testing.T) {
	for _, peerMRU := range []uint16{1, 64, 67} {
		t.Run(fmt.Sprint(peerMRU), func(t *testing.T) {
			td := startProxyTransmitCapDriver(t, peerMRU)
			request := awaitProxyMRUFloorRenegotiation(t, td)
			opts, err := ParseLCPOptions(request.Data)
			if err != nil {
				t.Fatal(err)
			}
			if mru, present := lookupMRUOption(opts); !present || mru != 1492 {
				t.Fatalf("fresh negotiation MRU=%d present=%v, want1492", mru, present)
			}
			auth, present := lookupOption(opts, LCPOptAuthProto)
			if !present || !bytes.Equal(auth, []byte{0xc0, 0x23}) {
				t.Fatalf("fresh negotiation auth=%x, want explicitly permitted PAP", auth)
			}
			if calls := td.backend.MTUCalls(); len(calls) != 0 {
				t.Fatalf("invalid proxy installed an interface before fresh negotiation: %+v", calls)
			}
			writeLifecyclePacket(t, td.peer, ProtoLCP, LCPConfigureRequest, 0x61,
				optStream(mruOpt(68), magicOpt(proxyTransmitPeerMagic)))
			ack := readLifecyclePacket(t, td.peer, ProtoLCP, LCPConfigureAck)
			if ack.Identifier != 0x61 {
				t.Fatalf("corrected peer Ack identifier=%d, want0x61", ack.Identifier)
			}
			writeLifecyclePacket(t, td.peer, ProtoLCP, LCPConfigureAck, request.Identifier, request.Data)
			assertProxyTransmitNegotiation(t, td.driver, 68)
			completeProxyTransmitNetwork(t, td)
			assertProxyTransmitInterface(t, td, 68, 1)
			assertProxyTransmitEcho(t, td, proxyTransmitPeerMagic)
		})
	}
}

// A peer at the actual floor stays on the proxy path and completes its
// permitted authentication, address assignment, MTU installation and Echo.
func TestProxyMRUIPv4FloorRemainsUsable(t *testing.T) {
	td := startProxyTransmitCapDriver(t, 68)
	assertProxyTransmitNegotiation(t, td.driver, 68)
	completeProxyTransmitNetwork(t, td)
	assertProxyTransmitInterface(t, td, 68, 1)
	assertProxyTransmitEcho(t, td, proxyTransmitPeerMagic)
}

// A producer that bypasses admission still cannot make the interface setter
// raise a peer's agreed limit to the IP floor or install an unusably small MTU.
func TestInterfaceSetupRefusesMRUBelowIPv4Floor(t *testing.T) {
	s, _, events := newRFC1661Session(LCPStateOpened)
	s.maxMRU = 1492
	s.negotiatedMRU = 67
	// Keep the control usable if the guard regresses: auth and both-disabled
	// NCP can complete, exposing the wrong interface installation directly.
	s.authRespCh <- authResponseMsg{accept: true}
	if s.afterLCPOpen() {
		t.Fatal("interface setup accepted an unsupported peer MRU67")
	}
	event := <-events
	down, ok := event.(EventSessionDown)
	if !ok || !bytes.Contains([]byte(down.Reason), []byte("MRU 67")) {
		t.Fatalf("interface setup result=%+v, want explicit MRU67 failure", event)
	}
	if calls := s.backend.(*fakeBackend).MTUCalls(); len(calls) != 0 {
		t.Fatalf("invalid peer MRU reached interface installation: %+v", calls)
	}
}

func awaitProxyMRUFloorRenegotiation(t *testing.T, td *ncpTestDriver) LCPPacket {
	t.Helper()
	type response struct {
		packet LCPPacket
		err    error
	}
	read := make(chan response, 1)
	if err := td.peer.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	// One test-lifetime reader races the first wire packet against an invalid
	// shortcut event. Closing td.peer in cleanup releases a pending read.
	go func() {
		var frame [MaxFrameBufLen]byte
		n, err := td.peer.Read(frame[:])
		if err != nil {
			read <- response{err: err}
			return
		}
		proto, payload, _, err := ParseFrame(frame[:n])
		if err != nil {
			read <- response{err: err}
			return
		}
		if proto != ProtoLCP {
			read <- response{err: fmt.Errorf("fresh negotiation protocol0x%04x, wantLCP", proto)}
			return
		}
		packet, err := ParseLCPPacket(payload)
		read <- response{packet: packet, err: err}
	}()
	select {
	case event := <-td.driver.EventsOut():
		t.Fatalf("undersized peer MRU was admitted without fresh negotiation: %+v", event)
	case result := <-read:
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.packet.Code != LCPConfigureRequest {
			t.Fatalf("fresh negotiation packet=%+v, wantConfigureRequest", result.packet)
		}
		return result.packet
	case <-time.After(5 * time.Second):
		t.Fatal("undersized proxy did not start fresh negotiation")
	}
	return LCPPacket{}
}
