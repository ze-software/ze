// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- LCP reentry from later phases.
// Related: session.go, network_phase.go -- the three frame consumers.
// RFC naming: untagged -- scripted renegotiation probes, not recorded RFC conformance proofs.

package pppoeclient

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestClientDiscardsMalformedReentryDuringAuth keeps the outstanding CHAP
// response valid across discarded options, then completes the same exchange.
func TestClientDiscardsMalformedReentryDuringAuth(t *testing.T) {
	for _, options := range [][]byte{{5, 6}, {5}} {
		frames := make(chan readFrame, 3)
		frames <- chapChallengeFrame(0x42, []byte{4, 1, 2, 3, 4})
		frames <- serverFrame(ppp.LCPConfigureRequest, 0x73, options)
		frames <- readFrame{data: chapReplyFrame(t, 3, 0x42, "ok")}
		close(frames)
		w := &recordingRWC{}
		var buf [ppp.MaxFrameBufLen]byte
		err := runClientAuth(w, testFrameReader(t, frames, nil), buf[:], lcpResult{authProto: ppp.ProtoCHAP}, sessionConfig{username: "user", password: "pass"}, clientMagic, make(chan struct{}), slog.New(slog.NewTextHandler(io.Discard, nil)), new(uint8))
		if err != nil {
			t.Fatalf("options % x interrupted CHAP: %v", options, err)
		}
		if got := len(w.snapshot()); got != 1 {
			t.Fatalf("options % x: wrote %d frames, want only the CHAP Response", options, got)
		}
		if got := len(chapPacketsWritten(t, w)); got != 1 {
			t.Fatalf("CHAP Responses = %d, want 1", got)
		}
	}
}

// TestClientDiscardsMalformedReentryDuringIPCP requires both directions of the
// original IPCP exchange to finish after a truncated LCP option arrives.
func TestClientDiscardsMalformedReentryDuringIPCP(t *testing.T) {
	for _, options := range [][]byte{{5, 6}, {5}} {
		frames := make(chan readFrame, 4)
		frames <- ipcpFrame(ppp.LCPConfigureNak, 1, []byte{3, 6, 192, 0, 2, 2})
		frames <- serverFrame(ppp.LCPConfigureRequest, 0x73, options)
		frames <- ipcpFrame(ppp.LCPConfigureAck, 2, ipcpRequestFor([]byte{3, 6, 192, 0, 2, 2}))
		frames <- ipcpFrame(ppp.LCPConfigureRequest, 0x70, []byte{3, 6, 192, 0, 2, 1})
		close(frames)
		w := &recordingRWC{}
		var buf [ppp.MaxFrameBufLen]byte
		var identifier uint8
		result, err := negotiateIPCP(w, testFrameReader(t, frames, nil), buf[:], clientMagic, 1492, nil, &identifier, make(chan struct{}), slog.New(slog.NewTextHandler(io.Discard, nil)), new(uint8))
		if err != nil {
			t.Fatalf("options % x interrupted IPCP: %v", options, err)
		}
		if result.localIP != netip.MustParseAddr("192.0.2.2") {
			t.Fatalf("local address = %v", result.localIP)
		}
		if result.peerIP != netip.MustParseAddr("192.0.2.1") {
			t.Fatalf("peer address = %v", result.peerIP)
		}
		written := w.snapshot()
		if len(written) != 3 {
			t.Fatalf("written frames = % x, want two IPCP requests and an Ack", written)
		}
		if !bytes.Equal(written[2], []byte{0x80, 0x21, 2, 0x70, 0, 10, 3, 6, 192, 0, 2, 1}) {
			t.Fatalf("IPCP Ack = % x", written[2])
		}
		if identifier != 2 {
			t.Fatalf("IPCP Identifier = %d, want 2", identifier)
		}
	}
}

// TestClientDiscardsMalformedReentryInNetworkPhase puts an Echo-Request after
// malformed options: its exact reply proves keepalive continued in Opened.
func TestClientDiscardsMalformedReentryInNetworkPhase(t *testing.T) {
	for _, options := range [][]byte{{5, 6}, {5}} {
		frames := make(chan readFrame, 2)
		frames <- serverFrame(ppp.LCPConfigureRequest, 0x73, options)
		frames <- serverFrame(ppp.LCPEchoRequest, 0x74, []byte{0x55, 0x66, 0x77, 0x88})
		close(frames)
		w := &recordingRWC{}
		result := sessionResult{reader: testFrameReader(t, frames, nil), magic: clientMagic, peerMRU: 1492}
		session := newTestConnectedSession(t, w, result, sessionConfig{mtu: 1492})
		keepaliveLoop(session, make(chan struct{}), make(chan struct{}),
			slog.New(slog.NewTextHandler(io.Discard, nil)))
		written := w.snapshot()
		if len(written) != 1 {
			t.Fatalf("options % x: written frames = % x, want only Echo-Reply", options, written)
		}
		want := []byte{0xc0, 0x21, ppp.LCPEchoReply, 0x74, 0, 8, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(want[6:], clientMagic)
		if !bytes.Equal(written[0], want) {
			t.Fatalf("Echo-Reply = % x, want % x", written[0], want)
		}
	}
}

// TestClientReentryRejectsUninitialized checks the constructor's non-request
// refusal and the safe zero-value contract before negotiation writes anything.
func TestClientReentryRejectsUninitialized(t *testing.T) {
	if _, admitted := requestLCPReentry(ppp.LCPPacket{}); admitted {
		t.Fatal("constructor admitted an uninitialized packet")
	}
	w := &recordingRWC{}
	frames := make(chan readFrame)
	close(frames)
	var buf [ppp.MaxFrameBufLen]byte
	_, err := negotiateLCP(w, testFrameReader(t, frames, nil), buf[:], sessionConfig{mtu: 1492}, clientMagic, &lcpReentry{}, make(chan struct{}), slog.New(slog.NewTextHandler(io.Discard, nil)), new(uint8))
	if err == nil {
		t.Fatal("uninitialized reentry opened LCP")
	}
	if got := len(w.snapshot()); got != 0 {
		t.Fatalf("uninitialized reentry wrote %d frames", got)
	}
}

// TestRFC1661ClientRenegotiatesLCPDuringAuth requires an answer to an LCP
// restart after a CHAP Response, using the real authentication frame consumer.
func TestRFC1661ClientRenegotiatesLCPDuringAuth(t *testing.T) {
	w := runRestartExchange(t, false, false, ppp.ProtoCHAP)
	if got := len(chapPacketsWritten(t, &w.recordingRWC)); got != 2 {
		t.Fatalf("CHAP Responses = %d, want one before and one after LCP restart", got)
	}
	assertClientRenegotiationReply(t, &w.recordingRWC)
}

// TestClientReentryAdmitsNakAndReject requires contained bad lengths and unknown
// options to reach the existing LCP response path after authentication handoff.
func TestClientReentryAdmitsNakAndReject(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []byte
		code    uint8
	}{
		{name: "contained bad Magic length", options: []byte{5, 1}, code: ppp.LCPConfigureNak},
		{name: "unknown option", options: []byte{0xfe, 2}, code: ppp.LCPConfigureReject},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frames := make(chan readFrame, 1)
			frames <- serverFrame(ppp.LCPConfigureRequest, 0x73, tc.options)
			close(frames)
			w := &recordingRWC{}
			var buf [ppp.MaxFrameBufLen]byte
			cfg := sessionConfig{mtu: 1492}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			reader := testFrameReader(t, frames, nil)
			err := runClientAuth(w, reader, buf[:], lcpResult{authProto: ppp.ProtoCHAP},
				cfg, clientMagic, make(chan struct{}), logger, new(uint8))
			var pending *lcpReentry
			if !errors.As(err, &pending) {
				t.Fatalf("request was not admitted: %v", err)
			}
			_, err = negotiateLCP(w, reader, buf[:], cfg, clientMagic, pending, make(chan struct{}), logger, new(uint8))
			if err == nil {
				t.Fatal("LCP opened without peer acceptance")
			}
			packets := w.lcpPackets(t)
			if len(packets) != 2 {
				t.Fatalf("LCP packets = %+v, want own request then refusal", packets)
			}
			if packets[0].Code != ppp.LCPConfigureRequest {
				t.Fatalf("first LCP code = %d, want Configure-Request", packets[0].Code)
			}
			if packets[1].Code != tc.code {
				t.Fatalf("reply code = %d, want %d", packets[1].Code, tc.code)
			}
			if packets[1].Identifier != 0x73 {
				t.Fatalf("reply Identifier = %x, want 73", packets[1].Identifier)
			}
			if tc.code == ppp.LCPConfigureReject {
				if !bytes.Equal(packets[1].Data, tc.options) {
					t.Fatalf("Reject options = % x, want % x", packets[1].Data, tc.options)
				}
			}
		})
	}
}

// TestRFC1661ClientRenegotiatesLCPDuringIPCP requires a restart response while
// IPCP is negotiating, using its channel frame consumer before PPPIOCCONNECT.
func TestRFC1661ClientRenegotiatesLCPDuringIPCP(t *testing.T) {
	w := runRestartExchange(t, true, false, ppp.ProtoCHAP)
	assertClientRenegotiationReply(t, &w.recordingRWC)
}

// TestRFC1661ClientRenegotiatesLCPInNetworkPhase requires a restart response
// from keepalive's channel consumer, without a second descriptor reader.
func TestRFC1661ClientRenegotiatesLCPInNetworkPhase(t *testing.T) {
	w := &recordingRWC{}
	var log bytes.Buffer
	result := sessionResult{reader: testFrameReader(t, clientRenegotiationFrames(), nil), magic: clientMagic, peerMRU: 1492}
	session := newTestConnectedSession(t, w, result, sessionConfig{mtu: 1492})
	keepaliveLoop(session, make(chan struct{}), make(chan struct{}),
		slog.New(slog.NewTextHandler(&log, nil)))
	t.Logf("network phase: %s", log.String())
	assertClientRenegotiationReply(t, w)
}

// clientRenegotiationRequest offers a new, nonzero Magic-Number in a request
// with a distinctive Identifier, so an unrelated Ack cannot satisfy the probe.
func clientRenegotiationRequest() readFrame {
	// RFC 1661 Sections 5.1 and 6.4: Code, Identifier, Length, then Magic-Number option.
	// Frame offsets: 0..1 Protocol, 2 Code, 3 Identifier, 4..5 Length,
	// 6 Type, 7 option Length, 8..11 Magic-Number.
	return readFrame{data: []byte{0xc0, 0x21, 1, 0x73, 0, 10, 5, 6, 0x55, 0x66, 0x77, 0x88}}
}

// clientRenegotiationFrames bounds each probe to one request then EOF, so a
// missing dispatch fails on its output rather than waiting for a phase timeout.
func clientRenegotiationFrames() <-chan readFrame {
	frames := make(chan readFrame, 1)
	frames <- clientRenegotiationRequest()
	close(frames)
	return frames
}

// assertClientRenegotiationReply checks the exact peer Ack and scr-before-sca
// ordering from RFC 1661 Section 4.1, Opened/RCR+, rather than a helper call.
func assertClientRenegotiationReply(t *testing.T, w *recordingRWC) {
	t.Helper()
	ack := []byte{0xc0, 0x21, 2, 0x73, 0, 10, 5, 6, 0x55, 0x66, 0x77, 0x88}
	var previous []byte
	for _, frame := range w.snapshot() {
		if bytes.Equal(frame, ack) {
			if len(previous) < 6 {
				t.Fatalf("packet before exact Configure-Ack is too short: % x", previous)
			}
			if previous[2] != ppp.LCPConfigureRequest {
				t.Fatalf("packet before exact Configure-Ack = % x, want own Configure-Request", previous)
			}
			if binary.BigEndian.Uint16(previous) != ppp.ProtoLCP {
				t.Fatalf("packet before exact Configure-Ack = % x, want LCP protocol c021", previous)
			}
			return
		}
		previous = frame
	}
	t.Fatalf("missing Configure-Ack % x; written frames: % x", ack, w.snapshot())
}

// TestRFC1661ClientNaksRenegotiationZeroMagic proves scr precedes scn and a
// corrected peer offer is required before authentication can resume.
func TestRFC1661ClientNaksRenegotiationZeroMagic(t *testing.T) {
	w := runRestartExchange(t, false, true, ppp.ProtoCHAP)
	packets := w.lcpPackets(t)
	for i, pkt := range packets {
		if pkt.Identifier != 0x73 {
			continue
		}
		if pkt.Code != ppp.LCPConfigureNak {
			t.Fatalf("pending zero Magic reply = %+v, want Nak", pkt)
		}
		if i == 0 {
			t.Fatal("Nak preceded own Configure-Request")
		}
		if packets[i-1].Code != ppp.LCPConfigureRequest {
			t.Fatalf("packet before Nak = %+v, want own Configure-Request", packets[i-1])
		}
		if len(pkt.Data) != 6 {
			t.Fatalf("Nak options = % x, want six octets", pkt.Data)
		}
		if pkt.Data[0] != 5 {
			t.Fatalf("Nak option type = %d, want Magic-Number", pkt.Data[0])
		}
		if pkt.Data[1] != 6 {
			t.Fatalf("Nak option length = %d, want 6", pkt.Data[1])
		}
		if binary.BigEndian.Uint32(pkt.Data[2:]) == 0 {
			t.Fatalf("Nak suggested zero Magic: % x", pkt.Data)
		}
		return
	}
	t.Fatal("missing Nak for zero Magic")
}

// TestRFC1661ClientRenegotiationChangesAuthProto drives the phase owner from
// CHAP into PAP or no authentication, then injects a CHAP challenge in IPCP.
func TestRFC1661ClientRenegotiationChangesAuthProto(t *testing.T) {
	for _, proto := range []uint16{ppp.ProtoPAP, 0} {
		w := runRestartExchange(t, false, false, proto)
		if got := len(chapPacketsWritten(t, &w.recordingRWC)); got != 1 {
			t.Fatalf("auth %x: CHAP responses = %d, want only the initial response", proto, got)
		}
	}
}

// TestClientRenegotiationNakNeedsPeerAcceptance withholds the corrected peer
// request: an Ack for our request alone must not reopen the rejected link.
func TestClientRenegotiationNakNeedsPeerAcceptance(t *testing.T) {
	w := &restartExchange{t: t, frames: make(chan readFrame, 32), zeroMagic: true, rejectOnly: true, auth: ppp.ProtoCHAP}
	link := &sessionLink{channel: w}
	_, err := negotiateSession(link, testFrameReader(t, w.frames, nil), -1, -1,
		sessionConfig{mtu: 1492, chanFD: -1, username: "user", password: "pass"},
		make(chan struct{}), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("uncorrected peer proposal opened LCP")
	}
	if !errors.Is(err, io.EOF) {
		t.Fatalf("uncorrected peer proposal = %v, want EOF while LCP is still unaccepted", err)
	}
	if w.responses != 1 {
		t.Fatalf("rejected peer resumed auth: responses=%d, want 1", w.responses)
	}
	if w.ipcpRequests != 0 {
		t.Fatalf("rejected peer started IPCP: requests=%d, want 0", w.ipcpRequests)
	}
}

// restartExchange answers writes synchronously, with a bounded input queue.
// It holds no descriptors or goroutines. Each request earns only its next reply.
type restartExchange struct {
	recordingRWC
	t                              *testing.T
	frames                         chan readFrame
	duringIPCP, zeroMagic          bool
	auth                           uint16
	epoch, responses, ipcpRequests int
	magic                          uint32
	lastIPCP                       uint8
	ipcpAckWritten                 bool
	rejectOnly                     bool
}

func (w *restartExchange) Write(data []byte) (int, error) {
	if _, err := w.recordingRWC.Write(data); err != nil {
		return 0, err
	}
	proto, payload, _, err := ppp.ParseFrame(data)
	if err != nil {
		w.t.Fatal(err)
	}
	pkt, err := ppp.ParseLCPPacket(payload)
	if err != nil {
		w.t.Fatal(err)
	}
	switch proto {
	case ppp.ProtoLCP:
		if pkt.Code != ppp.LCPConfigureRequest {
			return len(data), nil
		}
		w.epoch++
		magic := binary.BigEndian.Uint32(pkt.Data[len(pkt.Data)-4:])
		if w.epoch > 1 {
			if magic == w.magic {
				w.t.Fatal("local Magic reused on restart")
			}
		}
		w.magic = magic
		if w.epoch == 2 {
			if w.rejectOnly {
				w.frames <- serverFrame(ppp.LCPConfigureAck, pkt.Identifier, append([]byte(nil), pkt.Data...))
				close(w.frames)
				return len(data), nil
			}
		}
		auth := ppp.ProtoCHAP
		id := uint8(0x30)
		if w.epoch > 1 {
			auth, id = w.auth, 0x74
		}
		opts := []byte{5, 6, 0x11, 0x22, 0x33, 0x44}
		if auth == ppp.ProtoCHAP {
			opts = append(opts, 3, 5, 0xc2, 0x23, 5)
		}
		if auth == ppp.ProtoPAP {
			opts = append(opts, 3, 4, 0xc0, 0x23)
		}
		w.frames <- serverFrame(ppp.LCPConfigureRequest, id, opts)
		w.frames <- serverFrame(ppp.LCPConfigureAck, pkt.Identifier, append([]byte(nil), pkt.Data...))
		if auth == ppp.ProtoCHAP {
			if w.epoch > 1 {
				w.frames <- readFrame{data: chapReplyFrame(w.t, 3, 0x42, "stale")}
			}
			w.frames <- chapChallengeFrame(uint8(0x41+w.epoch), []byte{4, 1, 2, 3, 4})
		}
	case ppp.ProtoCHAP:
		wantID := uint8(0x41 + w.epoch)
		// The independent digest includes the wire Identifier, secret and
		// challenge value, as RFC 1994 Section 4.1 specifies.
		digest := md5.Sum([]byte{wantID, 'p', 'a', 's', 's', 1, 2, 3, 4})
		want := append([]byte{16}, digest[:]...)
		want = append(want, 'u', 's', 'e', 'r')
		if pkt.Code != 2 {
			w.t.Fatalf("CHAP code = %d, want Response", pkt.Code)
		}
		if pkt.Identifier != wantID {
			w.t.Fatalf("CHAP Identifier = %x, want %x", pkt.Identifier, wantID)
		}
		if !bytes.Equal(pkt.Data, want) {
			w.t.Fatalf("CHAP response data = % x, want % x", pkt.Data, want)
		}
		w.responses++
		if w.epoch == 1 {
			if !w.duringIPCP {
				w.enqueueRestart()
				return len(data), nil
			}
		}
		w.frames <- readFrame{data: chapReplyFrame(w.t, 3, pkt.Identifier, "ok")}
	case ppp.ProtoPAP:
		if w.epoch != 2 {
			w.t.Fatalf("unexpected PAP in epoch %d", w.epoch)
		}
		if w.auth != ppp.ProtoPAP {
			w.t.Fatalf("unexpected PAP for negotiated auth %x", w.auth)
		}
		w.frames <- readFrame{data: []byte{0xc0, 0x23, 2, pkt.Identifier, 0, 5, 0}}
	case ppp.ProtoIPCP:
		if pkt.Code != ppp.LCPConfigureRequest {
			if pkt.Code == ppp.LCPConfigureAck {
				if !bytes.Equal(data, []byte{0x80, 0x21, 2, 0x70, 0, 10, 3, 6, 192, 0, 2, 1}) {
					w.t.Fatalf("IPCP peer Ack = % x", data)
				}
				w.ipcpAckWritten = true
			}
			return len(data), nil
		}
		if w.epoch == 2 {
			if w.auth == ppp.ProtoCHAP {
				if w.responses != 2 {
					w.t.Fatal("IPCP started before fresh CHAP Response")
				}
			}
		}
		w.ipcpRequests++
		if pkt.Identifier != w.lastIPCP+1 {
			w.t.Fatalf("IPCP Identifier = %d, want %d across requests/restart", pkt.Identifier, w.lastIPCP+1)
		}
		if w.epoch == 2 {
			if w.duringIPCP {
				if w.ipcpRequests == 3 {
					w.frames <- ipcpFrame(ppp.LCPConfigureNak, w.lastIPCP, []byte{3, 6, 192, 0, 2, 99})
					w.frames <- ipcpFrame(ppp.LCPConfigureAck, w.lastIPCP, []byte{3, 6, 192, 0, 2, 2})
				}
			}
		}
		w.lastIPCP = pkt.Identifier
		if w.epoch == 1 {
			if w.ipcpRequests == 2 {
				w.enqueueRestart()
				return len(data), nil
			}
		}
		if bytes.Equal(pkt.Data, ipcpRequestFor([]byte{3, 6, 0, 0, 0, 0})) {
			if w.epoch == 2 {
				if w.auth != ppp.ProtoCHAP {
					w.frames <- chapChallengeFrame(0x66, []byte{4, 1, 2, 3, 4})
				}
			}
			w.frames <- ipcpFrame(ppp.LCPConfigureNak, pkt.Identifier, []byte{3, 6, 192, 0, 2, 2})
		} else {
			if !bytes.Equal(pkt.Data, ipcpRequestFor([]byte{3, 6, 192, 0, 2, 2})) {
				w.t.Fatalf("IPCP local proposal = % x", pkt.Data)
			}
			w.frames <- ipcpFrame(ppp.LCPConfigureAck, pkt.Identifier, append([]byte(nil), pkt.Data...))
			w.frames <- ipcpFrame(ppp.LCPConfigureRequest, 0x70, []byte{3, 6, 192, 0, 2, 1})
			close(w.frames)
		}
	}
	return len(data), nil
}

func (w *restartExchange) enqueueRestart() {
	if w.responses != 1 {
		w.t.Fatalf("responses before restart = %d, want 1", w.responses)
	}
	frame := clientRenegotiationRequest()
	if w.zeroMagic {
		clear(frame.data[8:])
	}
	w.frames <- frame
}

func runRestartExchange(t *testing.T, duringIPCP, zeroMagic bool, auth uint16) *restartExchange {
	t.Helper()
	w := &restartExchange{t: t, frames: make(chan readFrame, 32), duringIPCP: duringIPCP, zeroMagic: zeroMagic, auth: auth}
	link := &sessionLink{channel: w}
	_, err := negotiateSession(link, testFrameReader(t, w.frames, nil), -1, -1,
		sessionConfig{mtu: 1492, chanFD: -1, username: "user", password: "pass"},
		make(chan struct{}), slog.New(slog.NewTextHandler(io.Discard, nil)))
	// Invalid descriptors deliberately stop the real entry point only AFTER
	// both protocol directions have opened. Kernel ownership is Phase 3/5 proof.
	if err == nil {
		t.Fatal("negotiation unexpectedly connected invalid descriptors")
	}
	if !strings.Contains(err.Error(), "PPPIOCCONNECT") {
		t.Fatalf("negotiation = %v, want completed IPCP followed by invalid-descriptor Connect", err)
	}
	if w.epoch != 2 {
		t.Fatalf("epochs=%d, want 2", w.epoch)
	}
	if !w.ipcpAckWritten {
		t.Fatalf("missing exact IPCP Configure-Ack; written frames: % x", w.snapshot())
	}
	return w
}
