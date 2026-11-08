// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- IPv6CP in the network phase
// Related: session.go -- negotiateIPCP starts IPv6CP beside IPCP
// Related: ipv6cp_client.go -- enableIPv6CP applies the state negotiation reached

package pppoeclient

import (
	"bytes"
	"net/netip"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// recordedNCP parses frame index of w, failing the test on a parse error.
func recordedNCP(t *testing.T, w *frameLog, index int) (uint16, ppp.LCPPacket) {
	t.Helper()
	written := w.snapshot()
	if index >= len(written) {
		t.Fatalf("frame %d requested, the client wrote %d", index, len(written))
	}
	proto, payload, _, err := ppp.ParseFrame(written[index])
	if err != nil {
		t.Fatalf("parse frame %d: %v", index, err)
	}
	pkt, err := ppp.ParseLCPPacket(payload)
	if err != nil {
		t.Fatalf("parse packet %d: %v", index, err)
	}
	return proto, pkt
}

// ncpReadFrame is one frame the access concentrator sends.
func ncpReadFrame(proto uint16, code, id uint8, data []byte) readFrame {
	return readFrame{data: exchangeFrame(proto, code, id, data)}
}

// TestIPv6CPStartsBesideIPCP checks that IPv6CP does not wait for IPCP.
// RFC 1661 Section 3.6 configures each network-layer protocol separately once
// the network phase is reached; accel-ppp with ipv6=allow holds IPv6CP passive
// and activates the session as soon as IPCP opens, so a Configure-Request it
// has not seen by then leaves the session IPv4-only for good
// (plan/journal/ncp-started-behind-another-ncp.md).
// Method: run negotiateIPCP with an IPv6CP automaton over a scripted reader,
// assert the first network-phase transmission carries both Configure-Requests,
// then complete IPv6CP while IPCP is still negotiating and assert its frames
// were answered rather than dropped, before IPCP opens.
func TestIPv6CPStartsBesideIPCP(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &frameLog{}
		frames := make(chan readFrame)
		stop := make(chan struct{})
		defer close(stop)
		c := newClientIPv6CP(ipv6cpTestMAC, 0)
		type outcome struct {
			result ipcpResult
			err    error
		}
		done := make(chan outcome, 1)
		go func() {
			var buf [ppp.MaxFrameBufLen]byte
			result, err := negotiateIPCP(w, testFrameReader(t, frames, nil), buf[:], 1, 1492, nil,
				new(uint8), stop, slogutil.DiscardLogger(), new(uint8), c)
			done <- outcome{result, err}
		}()
		synctest.Wait()

		if written := w.snapshot(); len(written) != 2 {
			t.Fatalf("first network-phase transmission = %d frames, want IPCP and IPv6CP Configure-Requests", len(written))
		}
		proto, ipcpRequest := recordedNCP(t, w, 0)
		if proto != ppp.ProtoIPCP || ipcpRequest.Code != ppp.LCPConfigureRequest {
			t.Fatalf("frame 0 = protocol %#x code %d, want IPCP Configure-Request", proto, ipcpRequest.Code)
		}
		proto, ours := recordedNCP(t, w, 1)
		if proto != ppp.ProtoIPv6CP || ours.Code != ppp.LCPConfigureRequest {
			t.Fatalf("frame 1 = protocol %#x code %d, want IPv6CP Configure-Request", proto, ours.Code)
		}
		if !bytes.Equal(ours.Data, ipv6cpOption(ipv6cpTestEUI64)) {
			t.Fatalf("IPv6CP request options = % x, want the one EUI-64 identifier", ours.Data)
		}

		// The peer's IPv6CP Configure-Request arrives while IPCP negotiates.
		peerID := ppp.IPv6CPInterfaceID{0, 0, 0, 0, 0, 0, 0, 1}
		frames <- ncpReadFrame(ppp.ProtoIPv6CP, ppp.LCPConfigureRequest, 41, ipv6cpOption(peerID))
		synctest.Wait()
		proto, answer := recordedNCP(t, w, len(w.snapshot())-1)
		if proto != ppp.ProtoIPv6CP || answer.Code != ppp.LCPConfigureAck || answer.Identifier != 41 {
			t.Fatalf("answer = protocol %#x code %d id %d, want IPv6CP Configure-Ack 41", proto, answer.Code, answer.Identifier)
		}
		frames <- ncpReadFrame(ppp.ProtoIPv6CP, ppp.LCPConfigureAck, ours.Identifier, ours.Data)
		synctest.Wait()
		select {
		case got := <-done:
			t.Fatalf("IPCP returned before it opened: %+v", got)
		default:
		}
		if !c.opened() {
			t.Fatal("IPv6CP did not open while IPCP was negotiating")
		}

		// IPCP then opens on its own exchange, and IPv6CP keeps its state.
		assigned := netip.MustParseAddr("192.0.2.2")
		peer := netip.MustParseAddr("192.0.2.1")
		frames <- ipcpFrame(ppp.LCPConfigureNak, ipcpRequest.Identifier, ipcpAddressOption(assigned))
		synctest.Wait()
		_, next := recordedNCP(t, w, len(w.snapshot())-1)
		frames <- ipcpFrame(ppp.LCPConfigureAck, next.Identifier, next.Data)
		frames <- ipcpFrame(ppp.LCPConfigureRequest, 77, ipcpAddressOption(peer))
		got := <-done
		if got.err != nil {
			t.Fatalf("IPCP: %v", got.err)
		}
		if got.result.localIP != assigned {
			t.Fatalf("local %v, want %v", got.result.localIP, assigned)
		}
		if !c.opened() {
			t.Fatal("IPv6CP left Opened when IPCP opened")
		}
	})
}

// TestIPv6CPProtocolRejectDuringIPCP checks RFC 1661 Section 5.7 while IPCP
// negotiates: "Upon reception of a Protocol-Reject, the implementation MUST
// stop sending packets of the indicated protocol at the earliest
// opportunity." Method: Protocol-Reject IPv6CP after the first requests, run
// the Restart timer, and assert only IPCP is resent.
func TestIPv6CPProtocolRejectDuringIPCP(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &frameLog{}
		frames := make(chan readFrame)
		stop := make(chan struct{})
		c := newClientIPv6CP(ipv6cpTestMAC, 0)
		ended := make(chan struct{})
		go func() {
			defer close(ended)
			var buf [ppp.MaxFrameBufLen]byte
			_, _ = negotiateIPCP(w, testFrameReader(t, frames, nil), buf[:], 1, 1492, nil, // Stopped before IPCP opens, so the result says nothing.
				new(uint8), stop, slogutil.DiscardLogger(), new(uint8), c)
		}()
		synctest.Wait()
		_, ours := recordedNCP(t, w, 1)
		rejected := append([]byte{0x80, 0x57}, exchangeFrame(ppp.ProtoIPv6CP, ppp.LCPConfigureRequest, ours.Identifier, ours.Data)[2:]...)
		frames <- ncpReadFrame(ppp.ProtoLCP, ppp.LCPProtocolReject, 9, rejected)
		synctest.Wait()
		before := len(w.snapshot())
		// sleep(timer): advance the shared Restart timer once.
		time.Sleep(ipv6cpRestartInterval)
		synctest.Wait()
		written := w.snapshot()
		if len(written) != before+1 {
			t.Fatalf("Restart timer wrote %d frames, want the IPCP request alone", len(written)-before)
		}
		if proto, _ := recordedNCP(t, w, before); proto != ppp.ProtoIPCP {
			t.Fatalf("resent protocol %#x, want IPCP only", proto)
		}
		close(stop)
		<-ended
	})
}

// TestIPv6CPDisabledDropsDuringIPCP checks that with no ipv6 configured the
// client sends no IPv6CP and drops the peer's IPv6CP frames, as before.
// Method: run negotiateIPCP with a nil automaton, deliver an IPv6CP request,
// and assert the only frame written is the IPCP Configure-Request.
func TestIPv6CPDisabledDropsDuringIPCP(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &frameLog{}
		frames := make(chan readFrame)
		stop := make(chan struct{})
		ended := make(chan struct{})
		go func() {
			defer close(ended)
			var buf [ppp.MaxFrameBufLen]byte
			_, _ = negotiateIPCP(w, testFrameReader(t, frames, nil), buf[:], 1, 1492, nil, // Stopped before IPCP opens, so the result says nothing.
				new(uint8), stop, slogutil.DiscardLogger(), new(uint8), nil)
		}()
		synctest.Wait()
		frames <- ncpReadFrame(ppp.ProtoIPv6CP, ppp.LCPConfigureRequest, 41, ipv6cpOption(ppp.IPv6CPInterfaceID{0, 0, 0, 0, 0, 0, 0, 1}))
		synctest.Wait()
		if written := w.snapshot(); len(written) != 1 {
			t.Fatalf("client wrote %d frames, want the IPCP Configure-Request alone", len(written))
		}
		if proto, _ := recordedNCP(t, w, 0); proto != ppp.ProtoIPCP {
			t.Fatalf("protocol %#x, want IPCP", proto)
		}
		close(stop)
		<-ended
	})
}

// TestEnableIPv6CPAppliesNegotiatedState checks that the connected session
// applies the state IPv6CP reached during IPCP negotiation. Method: attach an
// opened and a started automaton and assert the IPv6 mode and the published
// link-local address, with IPv4's mode never touched.
func TestEnableIPv6CPAppliesNegotiatedState(t *testing.T) {
	logger := slogutil.DiscardLogger()

	t.Run("opened-passes", func(t *testing.T) {
		c, _, _ := openedIPv6CP(t)
		var v4, v6 []ppp.NPMode
		s := independenceSession(t, nil, &v4, &v6)
		if err := s.enableIPv6CP(c, s.setIPv6Mode, logger); err != nil {
			t.Fatalf("enable: %v", err)
		}
		if len(v6) != 1 || v6[0] != ppp.NPModePass {
			t.Fatalf("IPv6 modes %v, want [pass]", v6)
		}
		if len(v4) != 0 {
			t.Fatalf("IPv4 modes %v, want untouched", v4)
		}
		got := <-s.changes
		if got.LinkLocal != c.linkLocal() {
			t.Fatalf("published link-local %v, want %v", got.LinkLocal, c.linkLocal())
		}
	})

	t.Run("negotiating-drops", func(t *testing.T) {
		c, _, _ := startedIPv6CP(t)
		var v4, v6 []ppp.NPMode
		s := independenceSession(t, nil, &v4, &v6)
		if err := s.enableIPv6CP(c, s.setIPv6Mode, logger); err != nil {
			t.Fatalf("enable: %v", err)
		}
		if len(v6) != 1 || v6[0] != ppp.NPModeDrop {
			t.Fatalf("IPv6 modes %v, want [drop]", v6)
		}
		select {
		case got := <-s.changes:
			t.Fatalf("published %+v before IPv6CP opened", got)
		default:
		}
	})
}
