package ppp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net/netip"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Fixtures shared by the RFC 1661 compliance tests.
//
// Every test in this file drives a real producer -- the FSM transition
// function (ppp_fsm.go), the option negotiator (lcp_options.go), the frame
// codec (frame.go) or the session packet handler (session_run.go) -- and
// asserts on the bytes those producers put on the wire.
// ---------------------------------------------------------------------------

// frameRecorder is a chan-fd stand-in that keeps every Write as a separate
// frame. bytes.Buffer would concatenate them, which hides how many frames a
// single FSM transition emitted and in what order.
type frameRecorder struct {
	mu     sync.Mutex
	frames [][]byte
}

func (r *frameRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frames = append(r.frames, append([]byte(nil), p...))
	return len(p), nil
}

func (r *frameRecorder) Read([]byte) (int, error) { return 0, io.EOF }

func (r *frameRecorder) Close() error { return nil }

func (r *frameRecorder) all() [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([][]byte, len(r.frames))
	copy(out, r.frames)
	return out
}

// count returns the number of frames recorded so far.
func (r *frameRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.frames)
}

// decodeFrames parses each recorded frame into (protocol, LCP packet). All
// three protocols ze speaks on the control plane (LCP, IPCP, IPv6CP) share
// the RFC 1661 Section 5 packet shape, so one decoder covers them.
func decodeFrames(t *testing.T, r *frameRecorder) []struct {
	Proto uint16
	Pkt   LCPPacket
} {
	t.Helper()
	var out []struct {
		Proto uint16
		Pkt   LCPPacket
	}
	for i, f := range r.all() {
		proto, payload, _, err := ParseFrame(f)
		if err != nil {
			t.Fatalf("frame %d: ParseFrame: %v", i, err)
		}
		pkt, err := ParseLCPPacket(payload)
		if err != nil {
			t.Fatalf("frame %d: ParseLCPPacket: %v", i, err)
		}
		out = append(out, struct {
			Proto uint16
			Pkt   LCPPacket
		}{proto, pkt})
	}
	return out
}

// findCode returns the first decoded frame carrying the given LCP code, or
// ok=false when no recorded frame carries it.
func findCode(t *testing.T, r *frameRecorder, code uint8) (LCPPacket, bool) {
	t.Helper()
	for _, d := range decodeFrames(t, r) {
		if d.Pkt.Code == code {
			return d.Pkt, true
		}
	}
	return LCPPacket{}, false
}

// newRFC1661Session builds a pppSession in the requested LCP state wired to a
// frameRecorder. Both NCPs are disabled so a transition into Opened does not
// block on an IP handler; tests that need the NCPs enable them explicitly.
func newRFC1661Session(state LCPState) (*pppSession, *frameRecorder, chan Event) {
	rec := &frameRecorder{}
	events := make(chan Event, 16)
	ops, _, _ := newFakeOps()
	s := &pppSession{
		tunnelID:             1,
		sessionID:            2,
		chanFile:             rec,
		unitFD:               99,
		unitNum:              7,
		backend:              &fakeBackend{},
		ops:                  ops,
		eventsOut:            events,
		authEventsOut:        make(chan AuthEvent, 16),
		ipEventsOut:          make(chan IPEvent, 16),
		authRespCh:           make(chan authResponseMsg, 1),
		ipRespCh:             make(chan ipResponseMsg, 2),
		stopCh:               make(chan struct{}),
		sessStop:             make(chan struct{}),
		done:                 make(chan struct{}),
		logger:               discardLogger(),
		state:                state,
		maxMRU:               MaxFrameLen,
		magic:                0x01020304,
		negotiatedMRU:        MaxFrameLen,
		configuredAuthMethod: AuthMethodNone,
		authFallbackOrder:    defaultAuthFallbackOrder(),
		disableIPCP:          true,
		disableIPv6CP:        true,
	}
	return s, rec, events
}

// optStream serializes an option list into the Data field of a Configure-*
// packet.
func optStream(opts ...LCPOption) []byte {
	buf := make([]byte, 256)
	n := WriteLCPOptions(buf, 0, opts)
	return buf[:n]
}

// mruOption builds an MRU (type 1) option carrying v.
func mruOption(v uint16) LCPOption {
	d := make([]byte, 2)
	binary.BigEndian.PutUint16(d, v)
	return LCPOption{Type: LCPOptMRU, Data: d}
}

// magicOption builds a Magic-Number (type 5) option carrying v.
func magicOption(v uint32) LCPOption {
	d := make([]byte, 4)
	binary.BigEndian.PutUint32(d, v)
	return LCPOption{Type: LCPOptMagic, Data: d}
}

// authOption builds an Authentication-Protocol (type 3) option.
func authOption(proto uint16, extra ...byte) LCPOption {
	d := make([]byte, 2+len(extra))
	binary.BigEndian.PutUint16(d[:2], proto)
	copy(d[2:], extra)
	return LCPOption{Type: LCPOptAuthProto, Data: d}
}

// lcpFrame wraps an LCP packet in a PPP frame with the given protocol.
func lcpFrame(proto uint16, code, id uint8, data []byte) []byte {
	buf := make([]byte, MaxFrameLen)
	off := WriteFrame(buf, 0, proto, nil)
	off += WriteLCPPacket(buf, off, code, id, data)
	return buf[:off]
}

// ---------------------------------------------------------------------------
// Section 2 -- PPP encapsulation
// ---------------------------------------------------------------------------

// VALIDATES: a PPP frame whose Protocol field violates the RFC 1661 Section 2
//
//	parity rules is treated as carrying an unrecognized Protocol -- it is
//	dropped without a reply and without touching the LCP automaton.
//
// PREVENTS: a dispatcher that masks or normalizes the Protocol field and so
//
//	routes 0xC020 to the LCP handler.
//
// RFC requirement: RFC1661-2-1 positive -- Protocol 0xC020 has the least
// significant bit of its least significant octet equal to 0, so it is not a
// legal PPP Protocol; handleFrame (session_run.go) recognizes only 0xC021,
// 0x8021 and 0x8057 and drops everything else as unrecognized.
func TestRFC1661NonCompliantProtocolTreatedUnrecognized(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	// A well-formed Echo-Request body carried under an illegal (even)
	// Protocol value.
	body := []byte{0x11, 0x22, 0x33, 0x44}
	if term := s.handleFrame(lcpFrame(0xC020, LCPEchoRequest, 0x31, body)); term {
		t.Fatal("handleFrame terminated the session on an unrecognized Protocol")
	}
	if n := rec.count(); n != 0 {
		t.Fatalf("wrote %d frames in reply to an unrecognized Protocol, want 0", n)
	}
	if got := s.currentState(); got != LCPStateOpened {
		t.Fatalf("state = %s, want opened (automaton must not move)", got)
	}
}

// VALIDATES: the same body under the compliant LCP Protocol 0xC021 IS
//
//	recognized and answered, so the drop above is parity-driven rather than
//	a blanket refusal.
//
// RFC requirement: RFC1661-2-1 negative -- 0xC021 satisfies both parity rules
// (LSB of the least significant octet is 1, LSB of the most significant octet
// is 0) and is dispatched to the LCP handler instead of being dropped.
func TestRFC1661CompliantProtocolRecognized(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	body := []byte{0x11, 0x22, 0x33, 0x44}
	if term := s.handleFrame(lcpFrame(ProtoLCP, LCPEchoRequest, 0x31, body)); term {
		t.Fatal("handleFrame terminated the session on a valid LCP frame")
	}
	pkt, ok := findCode(t, rec, LCPEchoReply)
	if !ok {
		t.Fatalf("no Echo-Reply emitted for a compliant Protocol; frames=%d", rec.count())
	}
	if pkt.Identifier != 0x31 {
		t.Fatalf("Echo-Reply Identifier = 0x%02x, want 0x31", pkt.Identifier)
	}
}

// VALIDATES: ze always emits the two-octet (uncompressed) PPP Protocol field.
//
// RFC requirement: RFC1661-6.5-1 positive -- WriteFrame (frame.go) writes the
// Protocol with binary.BigEndian.PutUint16, so every frame ze produces carries
// a two-octet Protocol field.
//
// RFC requirement: RFC1661-6.5-2 positive -- LCPOptions.PFC being set does not
// change the encoder: WriteFrame has no compressed branch at all, so a
// single-octet Protocol field is never transmitted whether or not
// Protocol-Field-Compression was negotiated.
func TestRFC1661TwoOctetProtocolField(t *testing.T) {
	for _, pfc := range []bool{false, true} {
		// PFC is carried in the local option set; it must not alter framing.
		opts := BuildLocalConfigRequest(LCPOptions{MRU: 1500, PFC: pfc})
		buf := make([]byte, MaxFrameLen)
		off := WriteFrame(buf, 0, ProtoLCP, nil)
		if off != 2 {
			t.Fatalf("pfc=%v: protocol field wrote %d octets, want 2", pfc, off)
		}
		if buf[0] != 0xC0 || buf[1] != 0x21 {
			t.Fatalf("pfc=%v: protocol octets = %02x%02x, want c021", pfc, buf[0], buf[1])
		}
		if len(opts) == 0 {
			t.Fatalf("pfc=%v: expected a local option list", pfc)
		}
	}
}

// VALIDATES: a one-octet Protocol form is not accepted by the frame parser.
//
// RFC requirement: RFC1661-6.5-1 negative -- a frame carrying a single-octet
// Protocol field is rejected by ParseFrame (frame.go) rather than being
// silently promoted to the two-octet form.
func TestRFC1661SingleOctetProtocolRejected(t *testing.T) {
	_, _, _, err := ParseFrame([]byte{0x21})
	if !errors.Is(err, errFrameTooShort) {
		t.Fatalf("ParseFrame(1 octet) err = %v, want errFrameTooShort", err)
	}
}

// ---------------------------------------------------------------------------
// Section 3 -- PPP phases
// ---------------------------------------------------------------------------

// VALIDATES: the first frame a session puts on the wire is an LCP
//
//	Configure-Request.
//
// RFC requirement: RFC1661-3.1-1 positive -- run() (session_run.go) drives the
// synthetic Initial->Closed->ReqSent sequence whose actions are [irc, scr], so
// the session's first wire act is an LCP Configure-Request that configures and
// tests the data link.
func TestRFC1661LCPPacketsSentFirst(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateClosed)
	tr := LCPDoTransition(LCPStateClosed, LCPEventOpen)
	for _, act := range tr.Actions {
		if !s.performAction(act, LCPPacket{}) {
			t.Fatalf("performAction(%s) failed", act)
		}
	}
	frames := decodeFrames(t, rec)
	if len(frames) == 0 {
		t.Fatal("no frame emitted on Open")
	}
	if frames[0].Proto != ProtoLCP {
		t.Fatalf("first frame proto = 0x%04x, want LCP 0xC021", frames[0].Proto)
	}
	if frames[0].Pkt.Code != LCPConfigureRequest {
		t.Fatalf("first frame code = %d, want Configure-Request", frames[0].Pkt.Code)
	}
}

// VALIDATES: an enabled NCP is configured by sending its own NCP
//
//	Configure-Request once LCP has opened.
//
// RFC requirement: RFC1661-3.1-2 positive -- sendNCPConfigureRequest (ncp.go)
// emits a Configure-Request under the network-layer protocol's own NCP
// protocol number, which is how ze chooses and configures IPv4 on the link.
//
// RFC requirement: RFC1661-3.6-1 positive -- IPv4 is configured by IPCP
// (0x8021) and IPv6 by IPv6CP (0x8057); each family gets its own NCP packet.
func TestRFC1661NCPConfiguresEachFamilySeparately(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	s.localIPv4 = netip.MustParseAddr("10.0.0.1")
	s.localInterfaceID = [8]byte{0x02, 0, 0, 0, 0, 0, 0, 0x01}

	if !s.sendNCPConfigureRequest(AddressFamilyIPv4) {
		t.Fatal("sendNCPConfigureRequest(IPv4) failed")
	}
	if !s.sendNCPConfigureRequest(AddressFamilyIPv6) {
		t.Fatal("sendNCPConfigureRequest(IPv6) failed")
	}
	frames := decodeFrames(t, rec)
	if len(frames) != 2 {
		t.Fatalf("emitted %d frames, want 2", len(frames))
	}
	if frames[0].Proto != ProtoIPCP {
		t.Fatalf("IPv4 NCP proto = 0x%04x, want 0x8021", frames[0].Proto)
	}
	if frames[1].Proto != ProtoIPv6CP {
		t.Fatalf("IPv6 NCP proto = 0x%04x, want 0x8057", frames[1].Proto)
	}
	for i, f := range frames {
		if f.Pkt.Code != LCPConfigureRequest {
			t.Fatalf("frame %d code = %d, want Configure-Request", i, f.Pkt.Code)
		}
	}
}

// VALIDATES: with every NCP disabled the NCP phase emits no NCP packet, so the
//
//	Configure-Requests above are driven by which network-layer protocols are
//	in play rather than sent unconditionally.
//
// RFC requirement: RFC1661-3.1-2 negative -- runNCPPhase (ncp.go) returns
// without putting any NCP Configure-Request on the wire when no network-layer
// protocol is to be configured.
func TestRFC1661NoNCPPacketsWhenNoNetworkProtocol(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	if !s.runNCPPhase() {
		t.Fatal("runNCPPhase returned false with both NCPs disabled")
	}
	if n := rec.count(); n != 0 {
		t.Fatalf("emitted %d NCP frames with no network protocol enabled, want 0", n)
	}
}

// VALIDATES: the per-family NCP FSM states are independent -- opening IPCP does
//
//	not mark IPv6CP configured.
//
// RFC requirement: RFC1661-3.6-1 negative -- setNCPState / ncpState (ncp.go)
// keep ipcpState and ipv6cpState in separate fields, so one NCP reaching Opened
// leaves the other family unconfigured rather than implicitly configuring it.
func TestRFC1661NCPStatesAreIndependent(t *testing.T) {
	s, _, _ := newRFC1661Session(LCPStateOpened)
	s.setNCPState(AddressFamilyIPv4, LCPStateOpened)
	if got := s.ncpState(AddressFamilyIPv4); got != LCPStateOpened {
		t.Fatalf("IPv4 NCP state = %s, want opened", got)
	}
	if got := s.ncpState(AddressFamilyIPv6); got != LCPStateInitial {
		t.Fatalf("IPv6 NCP state = %s, want initial (families must not share state)", got)
	}
}

// VALIDATES: a network-layer packet arriving on the control-plane channel while
//
//	its NCP has not Opened is silently discarded.
//
// RFC requirement: RFC1661-3.6-2 positive -- handleFrame (session_run.go)
// dispatches only 0xC021 / 0x8021 / 0x8057; an IPv4 (0x0021) or IPv6 (0x0057)
// packet falls through to the drop path with no reply and no state change.
func TestRFC1661NetworkLayerPacketDiscardedBeforeNCPOpened(t *testing.T) {
	for _, proto := range []uint16{ProtoIPv4, ProtoIPv6} {
		s, rec, _ := newRFC1661Session(LCPStateOpened)
		if s.ipcpState != LCPStateInitial || s.ipv6cpState != LCPStateInitial {
			t.Fatal("fixture should start with both NCPs un-opened")
		}
		frame := make([]byte, 60)
		n := WriteFrame(frame, 0, proto, bytes.Repeat([]byte{0x45}, 40))
		if term := s.handleFrame(frame[:n]); term {
			t.Fatalf("proto 0x%04x: handleFrame terminated the session", proto)
		}
		if c := rec.count(); c != 0 {
			t.Fatalf("proto 0x%04x: wrote %d frames, want 0", proto, c)
		}
		if got := s.currentState(); got != LCPStateOpened {
			t.Fatalf("proto 0x%04x: state = %s, want opened", proto, got)
		}
	}
}

// VALIDATES: the Authentication-Protocol option is requested during Link
//
//	Establishment when ze wants the peer to authenticate.
//
// RFC requirement: RFC1661-3.5-1 positive -- sendConfigureRequest
// (session_run.go) reads configuredAuthMethod and BuildLocalConfigRequest
// (lcp_options.go) appends the Authentication-Protocol option, so the request
// for CHAP-MD5 is carried in the Link Establishment Configure-Request.
func TestRFC1661AuthProtocolRequestedDuringEstablishment(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateReqSent)
	s.configuredAuthMethod = AuthMethodCHAPMD5
	if !s.sendConfigureRequest() {
		t.Fatal("sendConfigureRequest failed")
	}
	pkt, ok := findCode(t, rec, LCPConfigureRequest)
	if !ok {
		t.Fatal("no Configure-Request emitted")
	}
	opts, err := ParseLCPOptions(pkt.Data)
	if err != nil {
		t.Fatalf("ParseLCPOptions: %v", err)
	}
	data, found := lookupOption(opts, LCPOptAuthProto)
	if !found {
		t.Fatal("Configure-Request carries no Authentication-Protocol option")
	}
	if binary.BigEndian.Uint16(data[:2]) != authProtoCHAP {
		t.Fatalf("Auth-Protocol = 0x%04x, want 0xC223", binary.BigEndian.Uint16(data[:2]))
	}
}

// VALIDATES: no Authentication-Protocol option is requested when ze does not
//
//	require the peer to authenticate.
//
// RFC requirement: RFC1661-3.5-1 negative -- authMethodToLCPOptions (auth.go)
// returns a zero Auth-Protocol for AuthMethodNone and BuildLocalConfigRequest
// omits the option entirely, so the option appears only when the protocol is
// actually desired.
func TestRFC1661NoAuthProtocolWhenNotDesired(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateReqSent)
	s.configuredAuthMethod = AuthMethodNone
	if !s.sendConfigureRequest() {
		t.Fatal("sendConfigureRequest failed")
	}
	pkt, ok := findCode(t, rec, LCPConfigureRequest)
	if !ok {
		t.Fatal("no Configure-Request emitted")
	}
	opts, err := ParseLCPOptions(pkt.Data)
	if err != nil {
		t.Fatalf("ParseLCPOptions: %v", err)
	}
	if _, found := lookupOption(opts, LCPOptAuthProto); found {
		t.Fatal("Configure-Request carries an Authentication-Protocol option when none is desired")
	}
}

// VALIDATES: the Network-Layer Protocol phase is not entered when
//
//	authentication is refused.
//
// RFC requirement: RFC1661-3.5-3 positive -- afterLCPOpen (session_run.go)
// calls runAuthPhase and returns immediately on failure, so none of the
// post-authentication network-layer work (PPPIOCCONNECT, MRU, MTU, interface
// up, EventSessionUp) runs.
func TestRFC1661NoNetworkPhaseUntilAuthCompletes(t *testing.T) {
	s, _, _ := newRFC1661Session(LCPStateAckSent)
	backend := &fakeBackend{}
	s.backend = backend
	ops, opsCalls, opsMu := newFakeOps()
	s.ops = ops
	s.authRespCh <- authResponseMsg{accept: false, message: "denied"}

	if s.afterLCPOpen() {
		t.Fatal("afterLCPOpen succeeded despite a rejected authentication")
	}
	if calls := backend.MTUCalls(); len(calls) != 0 {
		t.Fatalf("SetMTU calls = %+v, want none before authentication completes", calls)
	}
	if calls := backend.UpCalls(); len(calls) != 0 {
		t.Fatalf("SetAdminUp calls = %+v, want none before authentication completes", calls)
	}
	opsMu.Lock()
	n := len(*opsCalls)
	opsMu.Unlock()
	if n != 0 {
		t.Fatalf("setMRU calls = %d, want none before authentication completes", n)
	}
}

// VALIDATES: once authentication completes the network-layer phase does run,
//
//	so the block above is the auth decision and not a dead path.
//
// RFC requirement: RFC1661-3.5-3 negative -- with the auth handler accepting,
// afterLCPOpen (session_run.go) proceeds past runAuthPhase into the
// network-layer work and programs the interface.
func TestRFC1661NetworkPhaseRunsAfterAuthCompletes(t *testing.T) {
	s, _, _ := newRFC1661Session(LCPStateAckSent)
	backend := &fakeBackend{}
	s.backend = backend
	ops, opsCalls, opsMu := newFakeOps()
	s.ops = ops
	s.authRespCh <- authResponseMsg{accept: true}

	if !s.afterLCPOpen() {
		t.Fatal("afterLCPOpen failed with an accepted authentication")
	}
	if calls := backend.MTUCalls(); len(calls) != 1 || calls[0].name != "ppp7" {
		t.Fatalf("SetMTU calls = %+v, want one for ppp7", calls)
	}
	if calls := backend.UpCalls(); len(calls) != 1 {
		t.Fatalf("SetAdminUp calls = %v, want one", calls)
	}
	opsMu.Lock()
	n := len(*opsCalls)
	opsMu.Unlock()
	if n != 1 {
		t.Fatalf("setMRU calls = %d, want 1", n)
	}
}

// VALIDATES: the receiver of a Terminate-Request answers with a Terminate-Ack
//
//	and does NOT drop the link at that moment.
//
// RFC requirement: RFC1661-3.7-1 positive -- handleLCPPacket (session_run.go)
// takes the Opened+RTR edge to Stopping; Stopping is neither Closed nor
// Stopped, so no EventSessionDown is emitted and the session keeps running
// after the Terminate-Ack goes out.
//
// RFC requirement: RFC1661-5.5-1 positive -- the RTR edge performs sta, which
// sendTerminateAck turns into a Terminate-Ack echoing the request Identifier.
func TestRFC1661TerminateAckSentAndLinkHeld(t *testing.T) {
	s, rec, events := newRFC1661Session(LCPStateOpened)
	term := s.handleLCPPacket(LCPPacket{Code: LCPTerminateRequest, Identifier: 0x77})
	if term {
		t.Fatal("session terminated immediately on Terminate-Request")
	}
	pkt, ok := findCode(t, rec, LCPTerminateAck)
	if !ok {
		t.Fatal("no Terminate-Ack emitted")
	}
	if pkt.Identifier != 0x77 {
		t.Fatalf("Terminate-Ack Identifier = 0x%02x, want 0x77", pkt.Identifier)
	}
	if got := s.currentState(); got != LCPStateStopping {
		t.Fatalf("state = %s, want stopping (link must not be dropped yet)", got)
	}
	select {
	case ev := <-events:
		t.Fatalf("unexpected lifecycle event %T; link was disconnected too early", ev)
	default:
	}
}

// VALIDATES: a Terminate-Ack received does not itself provoke a Terminate-Ack,
//
//	so the reply above is specific to Terminate-Request.
//
// RFC requirement: RFC1661-5.5-1 negative -- the Opened+RTA edge in
// LCPDoTransition (ppp_fsm.go) is [tld, scr], which sendConfigureRequest turns
// into a Configure-Request; no Terminate-Ack is produced.
func TestRFC1661NoTerminateAckForTerminateAck(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	if term := s.handleLCPPacket(LCPPacket{Code: LCPTerminateAck, Identifier: 0x77}); term {
		t.Fatal("session terminated on Terminate-Ack in Opened")
	}
	if _, ok := findCode(t, rec, LCPTerminateAck); ok {
		t.Fatal("Terminate-Ack emitted in response to a Terminate-Ack")
	}
}

// ---------------------------------------------------------------------------
// Section 4 -- the option negotiation automaton
// ---------------------------------------------------------------------------

// VALIDATES: a Configure-Request received while Opened immediately restarts
//
//	configuration.
//
// RFC requirement: RFC1661-4.3-1 positive -- LCPDoTransition (ppp_fsm.go) maps
// Opened+RCR+ to [tld, scr, sca] and Opened+RCR- to [tld, scr, scn], so a peer
// Configure-Request in Opened drops the layer and renegotiates at once.
func TestRFC1661RenegotiateOnConfigureRequestInOpened(t *testing.T) {
	cases := []struct {
		name  string
		event LCPEvent
		want  []LCPAction
		next  LCPState
	}{
		{"acceptable options", LCPEventRCRPlus, []LCPAction{LCPActTLD, LCPActSCR, LCPActSCA}, LCPStateAckSent},
		{"unacceptable options", LCPEventRCRMinus, []LCPAction{LCPActTLD, LCPActSCR, LCPActSCN}, LCPStateReqSent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr := LCPDoTransition(LCPStateOpened, tc.event)
			if tr.NewState != tc.next {
				t.Fatalf("next state = %s, want %s", tr.NewState, tc.next)
			}
			if len(tr.Actions) != len(tc.want) {
				t.Fatalf("actions = %v, want %v", tr.Actions, tc.want)
			}
			for i := range tc.want {
				if tr.Actions[i] != tc.want[i] {
					t.Fatalf("actions = %v, want %v", tr.Actions, tc.want)
				}
			}
		})
	}
}

// VALIDATES: not every packet received in Opened restarts configuration.
//
// RFC requirement: RFC1661-4.3-1 negative -- the Opened+RXR edge
// (ppp_fsm.go) is [ser] only: an Echo keeps the layer up and emits no
// Send-Configure-Request, so the renegotiation above is specific to a received
// Configure-Request.
func TestRFC1661EchoDoesNotRenegotiate(t *testing.T) {
	tr := LCPDoTransition(LCPStateOpened, LCPEventRXR)
	if tr.NewState != LCPStateOpened {
		t.Fatalf("next state = %s, want opened", tr.NewState)
	}
	for _, a := range tr.Actions {
		if a == LCPActSCR || a == LCPActTLD {
			t.Fatalf("actions = %v, want no scr/tld on an Echo", tr.Actions)
		}
	}
}

// VALIDATES: after answering a Terminate-Request the automaton is still able to
//
//	take a new Configure-Request without any administrative action.
//
// RFC requirement: RFC1661-4.3-2 positive -- the RTR edges in ReqSent,
// AckRcvd and AckSent all land in ReqSent (ppp_fsm.go), and ReqSent+RCR+ is a
// live edge to AckSent with sca, so a fresh Configure-Request is accepted with
// no operator intervention in between.
func TestRFC1661NewConfigureRequestAcceptedAfterTerminateRequest(t *testing.T) {
	for _, from := range []LCPState{LCPStateReqSent, LCPStateAckRcvd, LCPStateAckSent} {
		after := LCPDoTransition(from, LCPEventRTR)
		if after.NewState != LCPStateReqSent {
			t.Fatalf("%s + RTR -> %s, want req-sent", from, after.NewState)
		}
		next := LCPDoTransition(after.NewState, LCPEventRCRPlus)
		if next.NewState != LCPStateAckSent {
			t.Fatalf("%s: post-RTR RCR+ -> %s, want ack-sent", from, next.NewState)
		}
		if len(next.Actions) != 1 || next.Actions[0] != LCPActSCA {
			t.Fatalf("%s: post-RTR RCR+ actions = %v, want [sca]", from, next.Actions)
		}
	}
}

// ---------------------------------------------------------------------------
// Section 5.1 / 5.2 -- Configure-Request and Configure-Ack
// ---------------------------------------------------------------------------

// VALIDATES: an administrative Open transmits a Configure-Request.
//
// RFC requirement: RFC1661-5.1-1 positive -- LCPDoTransition (ppp_fsm.go) maps
// Closed+Open and Starting+Up to [irc, scr]; scr is the Send-Configure-Request
// action performAction routes to sendConfigureRequest.
func TestRFC1661OpenTransmitsConfigureRequest(t *testing.T) {
	for _, tc := range []struct {
		state LCPState
		event LCPEvent
	}{
		{LCPStateClosed, LCPEventOpen},
		{LCPStateStarting, LCPEventUp},
	} {
		tr := LCPDoTransition(tc.state, tc.event)
		if tr.NewState != LCPStateReqSent {
			t.Fatalf("%s: next state = %s, want req-sent", tc.state, tr.NewState)
		}
		found := false
		for _, a := range tr.Actions {
			if a == LCPActSCR {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: actions = %v, want an scr", tc.state, tr.Actions)
		}
	}
}

// VALIDATES: a lower-layer Up without an administrative Open does NOT transmit
//
//	a Configure-Request.
//
// RFC requirement: RFC1661-5.1-1 negative -- Initial+Up in LCPDoTransition
// (ppp_fsm.go) moves to Closed with no actions, so an implementation that does
// not wish to open a connection sends nothing.
func TestRFC1661UpWithoutOpenSendsNoConfigureRequest(t *testing.T) {
	tr := LCPDoTransition(LCPStateInitial, LCPEventUp)
	if tr.NewState != LCPStateClosed {
		t.Fatalf("next state = %s, want closed", tr.NewState)
	}
	if len(tr.Actions) != 0 {
		t.Fatalf("actions = %v, want none", tr.Actions)
	}
}

// VALIDATES: a Configure-Request whose options are all recognized and
//
//	acceptable is answered with a Configure-Ack that echoes the option field
//	byte-for-byte, in the original order.
//
// RFC requirement: RFC1661-5.1-2 positive -- handleLCPPacket (session_run.go)
// classifies the request as RCR+ and the FSM's sca action reaches
// sendConfigureAck, which transmits the reply.
//
// RFC requirement: RFC1661-5.2-1 positive -- every option is acceptable, so
// the reply code is Configure-Ack (2) rather than Nak or Reject.
//
// RFC requirement: RFC1661-5.2-2 positive -- sendConfigureAck writes req.Data
// unchanged, so the acknowledged options are neither reordered nor modified.
func TestRFC1661ConfigureAckEchoesOptionsVerbatim(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateReqSent)
	data := optStream(mruOption(1400), magicOption(0xDEADBEEF))
	if term := s.handleLCPPacket(LCPPacket{
		Code: LCPConfigureRequest, Identifier: 0x21, Data: data,
	}); term {
		t.Fatal("session terminated on an acceptable Configure-Request")
	}
	pkt, ok := findCode(t, rec, LCPConfigureAck)
	if !ok {
		t.Fatalf("no Configure-Ack emitted; frames=%d", rec.count())
	}
	if pkt.Identifier != 0x21 {
		t.Fatalf("Ack Identifier = 0x%02x, want 0x21", pkt.Identifier)
	}
	if !bytes.Equal(pkt.Data, data) {
		t.Fatalf("Ack options = %x, want the request's %x", pkt.Data, data)
	}
}

// VALIDATES: the Configure-Ack tracks the request's option ORDER rather than
//
//	any canonical order of ze's own.
//
// RFC requirement: RFC1661-5.2-2 negative -- feeding the same two options in
// the opposite order produces an Ack in that opposite order, so
// sendConfigureAck (session_run.go) really echoes the request instead of
// re-serializing a normalized list that would happen to match in one order.
func TestRFC1661ConfigureAckDoesNotReorderOptions(t *testing.T) {
	first := optStream(mruOption(1400), magicOption(0xDEADBEEF))
	second := optStream(magicOption(0xDEADBEEF), mruOption(1400))
	if bytes.Equal(first, second) {
		t.Fatal("fixture bug: the two orders must differ on the wire")
	}
	for _, data := range [][]byte{first, second} {
		s, rec, _ := newRFC1661Session(LCPStateReqSent)
		if term := s.handleLCPPacket(LCPPacket{
			Code: LCPConfigureRequest, Identifier: 0x22, Data: data,
		}); term {
			t.Fatal("session terminated on an acceptable Configure-Request")
		}
		pkt, ok := findCode(t, rec, LCPConfigureAck)
		if !ok {
			t.Fatal("no Configure-Ack emitted")
		}
		if !bytes.Equal(pkt.Data, data) {
			t.Fatalf("Ack options = %x, want %x (order must follow the request)", pkt.Data, data)
		}
	}
}

// VALIDATES: a Configure-Request carrying an unrecognized option is answered
//
//	with a Configure-Reject, not a Configure-Ack.
//
// RFC requirement: RFC1661-5.1-2 negative -- the appropriate reply for an
// unacceptable request is not an Ack: handleLCPPacket classifies it RCR- and
// sendConfigureNakOrReject (session_run.go) emits a Configure-Reject.
//
// RFC requirement: RFC1661-5.2-1 negative -- because one option is not
// recognizable, the Configure-Ack branch is not taken.
//
// RFC requirement: RFC1661-5.4-1 positive -- the unrecognized option drives a
// Configure-Reject reply.
//
// RFC requirement: RFC1661-5.4-2 positive -- the rejected option is echoed
// with its original Type and Data, unmodified.
func TestRFC1661ConfigureRejectForUnrecognizedOption(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateReqSent)
	unknown := LCPOption{Type: 99, Data: []byte{0xAA, 0xBB}}
	data := optStream(mruOption(1400), unknown)
	if term := s.handleLCPPacket(LCPPacket{
		Code: LCPConfigureRequest, Identifier: 0x23, Data: data,
	}); term {
		t.Fatal("session terminated on an unrecognizable Configure-Request")
	}
	if _, ok := findCode(t, rec, LCPConfigureAck); ok {
		t.Fatal("Configure-Ack emitted for a request carrying an unrecognized option")
	}
	pkt, ok := findCode(t, rec, LCPConfigureReject)
	if !ok {
		t.Fatalf("no Configure-Reject emitted; frames=%d", rec.count())
	}
	want := optStream(unknown)
	if !bytes.Equal(pkt.Data, want) {
		t.Fatalf("Reject options = %x, want exactly the offending option %x", pkt.Data, want)
	}
}

// VALIDATES: a fully acceptable request draws no Configure-Reject, so the
//
//	Reject above is driven by the offending option.
//
// RFC requirement: RFC1661-5.4-1 negative -- NegotiatePeerOptions
// (lcp_options.go) returns an empty reject list when every option is
// recognized and negotiable, and sendConfigureNakOrReject is never reached.
func TestRFC1661NoConfigureRejectWhenAllOptionsRecognized(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateReqSent)
	data := optStream(mruOption(1400), magicOption(0x11223344))
	if term := s.handleLCPPacket(LCPPacket{
		Code: LCPConfigureRequest, Identifier: 0x24, Data: data,
	}); term {
		t.Fatal("session terminated on an acceptable Configure-Request")
	}
	if _, ok := findCode(t, rec, LCPConfigureReject); ok {
		t.Fatal("Configure-Reject emitted for a fully acceptable Configure-Request")
	}
}

// VALIDATES: the Configure-Reject preserves the order of the offending options.
//
// RFC requirement: RFC1661-5.4-2 negative -- swapping the two unrecognized
// options in the request swaps them in the Reject, so NegotiatePeerOptions
// (lcp_options.go) walks the request in order instead of emitting a
// normalized list.
func TestRFC1661ConfigureRejectDoesNotReorderOptions(t *testing.T) {
	a := LCPOption{Type: 99, Data: []byte{0xAA}}
	b := LCPOption{Type: 98, Data: []byte{0xBB, 0xCC}}
	for _, order := range [][]LCPOption{{a, b}, {b, a}} {
		opts := append([]LCPOption(nil), order...)
		_, _, rejects := NegotiatePeerOptions(opts, LCPNegPolicy{})
		if len(rejects) != 2 {
			t.Fatalf("rejects = %d, want 2", len(rejects))
		}
		for i := range opts {
			if rejects[i].Type != opts[i].Type {
				t.Fatalf("reject %d type = %d, want %d (order must be preserved)",
					i, rejects[i].Type, opts[i].Type)
			}
			if !bytes.Equal(rejects[i].Data, opts[i].Data) {
				t.Fatalf("reject %d data = %x, want %x (must not be modified)",
					i, rejects[i].Data, opts[i].Data)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Section 5.3 -- Configure-Nak
// ---------------------------------------------------------------------------

// VALIDATES: a recognized option with an unacceptable value draws a
//
//	Configure-Nak carrying a value the responder will accept.
//
// RFC requirement: RFC1661-5.3-1 positive -- every option is recognizable and
// only the MRU value is unacceptable, so sendConfigureNakOrReject
// (session_run.go) takes the Nak branch.
//
// RFC requirement: RFC1661-5.3-3 positive -- MRU is a single-instance option;
// negotiatePeerOption (lcp_options.go) replaces the requested 2000 with the
// acceptable 1500 rather than echoing it.
func TestRFC1661ConfigureNakSuggestsAcceptableValue(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateReqSent)
	data := optStream(mruOption(2000))
	if term := s.handleLCPPacket(LCPPacket{
		Code: LCPConfigureRequest, Identifier: 0x31, Data: data,
	}); term {
		t.Fatal("session terminated on an unacceptable-value Configure-Request")
	}
	pkt, ok := findCode(t, rec, LCPConfigureNak)
	if !ok {
		t.Fatalf("no Configure-Nak emitted; frames=%d", rec.count())
	}
	opts, err := ParseLCPOptions(pkt.Data)
	if err != nil {
		t.Fatalf("ParseLCPOptions: %v", err)
	}
	if len(opts) != 1 || opts[0].Type != LCPOptMRU {
		t.Fatalf("Nak options = %+v, want one MRU option", opts)
	}
	if got := binary.BigEndian.Uint16(opts[0].Data); got != MaxFrameLen {
		t.Fatalf("Nak MRU = %d, want %d", got, MaxFrameLen)
	}
}

// VALIDATES: an acceptable value is not modified and draws no Nak, so the
//
//	substitution above happens only on the unacceptable path.
//
// RFC requirement: RFC1661-5.3-1 negative -- with all values acceptable, the
// reply is a Configure-Ack, not a Configure-Nak.
//
// RFC requirement: RFC1661-5.3-3 negative -- negotiatePeerOption
// (lcp_options.go) returns the request's own Data for an acceptable MRU, so
// the value is left untouched.
func TestRFC1661NoNakForAcceptableValue(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateReqSent)
	data := optStream(mruOption(1460))
	if term := s.handleLCPPacket(LCPPacket{
		Code: LCPConfigureRequest, Identifier: 0x32, Data: data,
	}); term {
		t.Fatal("session terminated on an acceptable Configure-Request")
	}
	if _, ok := findCode(t, rec, LCPConfigureNak); ok {
		t.Fatal("Configure-Nak emitted for an acceptable MRU")
	}
	pkt, ok := findCode(t, rec, LCPConfigureAck)
	if !ok {
		t.Fatal("no Configure-Ack emitted")
	}
	if !bytes.Equal(pkt.Data, data) {
		t.Fatalf("Ack options = %x, want unmodified %x", pkt.Data, data)
	}
}

// VALIDATES: boolean options (Length 2, no value field) are refused with a
//
//	Configure-Reject, never a Configure-Nak.
//
// RFC requirement: RFC1661-5.3-2 positive -- negotiatePeerOption
// (lcp_options.go) returns negReject for a malformed PFC / ACFC option; there
// is no negNak branch for either boolean option, so they can only ever be
// Acked or Rejected.
func TestRFC1661BooleanOptionsUseRejectNotNak(t *testing.T) {
	opts := []LCPOption{
		{Type: LCPOptPFC, Data: []byte{0xAA}},
		{Type: LCPOptACFC, Data: []byte{0xBB, 0xCC}},
	}
	_, naks, rejects := NegotiatePeerOptions(opts, LCPNegPolicy{})
	if len(naks) != 0 {
		t.Fatalf("naks = %+v, want none for boolean options", naks)
	}
	if len(rejects) != 2 {
		t.Fatalf("rejects = %d, want 2", len(rejects))
	}
}

// VALIDATES: an option that DOES carry a value field is Nak'd, proving the
//
//	Reject choice above is specific to the value-less boolean options.
//
// RFC requirement: RFC1661-5.3-2 negative -- the MRU option has a value field,
// and negotiatePeerOption (lcp_options.go) answers an unacceptable MRU with a
// Nak, so the Nak path exists and is deliberately not used for booleans.
func TestRFC1661ValuedOptionUsesNak(t *testing.T) {
	_, naks, rejects := NegotiatePeerOptions([]LCPOption{mruOption(2000)}, LCPNegPolicy{MaxMRU: 1500})
	if len(rejects) != 0 {
		t.Fatalf("rejects = %+v, want none", rejects)
	}
	if len(naks) != 1 {
		t.Fatalf("naks = %d, want 1 for an unacceptable valued option", len(naks))
	}
}

// VALIDATES: the value a Configure-Nak proposes is itself acceptable -- feeding
//
//	it back through the negotiator yields an Ack.
//
// RFC requirement: RFC1661-5.3-5 positive -- the suggestion negotiatePeerOption
// (lcp_options.go) writes into the Nak passes its own acceptance check, so the
// value field really does indicate a value acceptable to the Nak sender.
func TestRFC1661NakValueIsAcceptable(t *testing.T) {
	policy := LCPNegPolicy{MaxMRU: 1500}
	for _, bad := range []uint16{2000, 32} {
		_, naks, _ := NegotiatePeerOptions([]LCPOption{mruOption(bad)}, policy)
		if len(naks) != 1 {
			t.Fatalf("MRU %d: naks = %d, want 1", bad, len(naks))
		}
		acks, naks2, rejects := NegotiatePeerOptions([]LCPOption{naks[0]}, policy)
		if len(acks) != 1 || len(naks2) != 0 || len(rejects) != 0 {
			t.Fatalf("MRU %d: re-offering the Nak value gave ack=%d nak=%d rej=%d, want 1/0/0",
				bad, len(acks), len(naks2), len(rejects))
		}
	}
}

// VALIDATES: the originally-requested unacceptable value is still refused when
//
//	re-offered, so acceptance is value-driven.
//
// RFC requirement: RFC1661-5.3-5 negative -- re-submitting the value the peer
// asked for draws another Nak from negotiatePeerOption (lcp_options.go), which
// is what makes the accepted suggestion above meaningful.
func TestRFC1661RejectedValueStaysUnacceptable(t *testing.T) {
	policy := LCPNegPolicy{MaxMRU: 1500}
	for range 2 {
		_, naks, _ := NegotiatePeerOptions([]LCPOption{mruOption(2000)}, policy)
		if len(naks) != 1 {
			t.Fatalf("naks = %d, want 1 on every offer of the unacceptable value", len(naks))
		}
	}
}

// VALIDATES: the Configure-Nak lists options in the order the request carried
//
//	them.
//
// RFC requirement: RFC1661-5.3-6 positive -- NegotiatePeerOptions
// (lcp_options.go) appends one entry per received option in receive order, so
// two Nak-worthy MRU options come back in the order they arrived.
func TestRFC1661NakPreservesRequestOrder(t *testing.T) {
	policy := LCPNegPolicy{MaxMRU: 1500}
	_, naks, _ := NegotiatePeerOptions([]LCPOption{mruOption(2000), mruOption(32)}, policy)
	if len(naks) != 2 {
		t.Fatalf("naks = %d, want 2", len(naks))
	}
	if binary.BigEndian.Uint16(naks[0].Data) != 1500 || binary.BigEndian.Uint16(naks[1].Data) != 64 {
		t.Fatalf("nak values = %d,%d, want 1500,64 in that order",
			binary.BigEndian.Uint16(naks[0].Data), binary.BigEndian.Uint16(naks[1].Data))
	}
}

// VALIDATES: reversing the request order reverses the Nak order, so the Nak is
//
//	not emitted in a fixed order that only coincidentally matched.
//
// RFC requirement: RFC1661-5.3-6 negative -- with the two MRU options swapped,
// NegotiatePeerOptions (lcp_options.go) produces the swapped Nak list.
func TestRFC1661NakOrderFollowsRequestNotAFixedOrder(t *testing.T) {
	policy := LCPNegPolicy{MaxMRU: 1500}
	_, naks, _ := NegotiatePeerOptions([]LCPOption{mruOption(32), mruOption(2000)}, policy)
	if len(naks) != 2 {
		t.Fatalf("naks = %d, want 2", len(naks))
	}
	if binary.BigEndian.Uint16(naks[0].Data) != 64 || binary.BigEndian.Uint16(naks[1].Data) != 1500 {
		t.Fatalf("nak values = %d,%d, want 64,1500 in that order",
			binary.BigEndian.Uint16(naks[0].Data), binary.BigEndian.Uint16(naks[1].Data))
	}
}

// VALIDATES: a Configure-Nak whose option Length differs from the one ze sent
//
//	is handled rather than mis-decoded.
//
// RFC requirement: RFC1661-5.3-8 positive -- ze advertises CHAP-MD5 as a
// 5-octet Authentication-Protocol option (proto + algorithm); the peer Naks
// with the 4-octet PAP form, and adjustAuthOnNakOrReject (auth.go) parses the
// shorter option and switches the configured method to PAP.
func TestRFC1661NakHandlesDifferentOptionLength(t *testing.T) {
	s, _, _ := newRFC1661Session(LCPStateReqSent)
	s.configuredAuthMethod = AuthMethodCHAPMD5
	// ze's own option is 5 octets; the Nak's is 4.
	nak := optStream(authOption(authProtoPAP))
	if len(nak) != 4 {
		t.Fatalf("fixture: Nak option is %d octets, want 4", len(nak))
	}
	s.adjustAuthOnNakOrReject(LCPPacket{Code: LCPConfigureNak, Identifier: 1, Data: nak})
	if s.configuredAuthMethod != AuthMethodPAP {
		t.Fatalf("configured method = %s, want pap after a shorter-option Nak", s.configuredAuthMethod)
	}
}

// VALIDATES: a Nak option too short to carry a protocol value is not decoded as
//
//	one; the method is cleared instead of being read from out-of-range bytes.
//
// RFC requirement: RFC1661-5.3-8 negative -- adjustAuthOnNakOrReject (auth.go)
// length-checks the option data before decoding, so a 1-octet
// Authentication-Protocol value falls to AuthMethodNone rather than being
// mis-parsed as a valid suggestion.
func TestRFC1661NakTooShortOptionNotDecoded(t *testing.T) {
	s, _, _ := newRFC1661Session(LCPStateReqSent)
	s.configuredAuthMethod = AuthMethodCHAPMD5
	nak := optStream(LCPOption{Type: LCPOptAuthProto, Data: []byte{0xC0}})
	s.adjustAuthOnNakOrReject(LCPPacket{Code: LCPConfigureNak, Identifier: 1, Data: nak})
	if s.configuredAuthMethod != AuthMethodNone {
		t.Fatalf("configured method = %s, want none for a truncated Nak option", s.configuredAuthMethod)
	}
}

// ---------------------------------------------------------------------------
// Section 5.6 -- Code-Reject
// ---------------------------------------------------------------------------

// VALIDATES: an LCP packet with an unknown Code is reported back with a
//
//	Code-Reject carrying the offending packet.
//
// RFC requirement: RFC1661-5.6-1 positive -- codeToEvent (session_run.go) maps
// an unrecognized Code to RUC, the FSM answers with scj, and sendCodeReject
// rebuilds the rejected packet's Code, Identifier, Length and Data into the
// Rejected-Packet field.
func TestRFC1661CodeRejectForUnknownCode(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	if term := s.handleLCPPacket(LCPPacket{Code: 99, Identifier: 0x41, Data: []byte{0xAA, 0xBB}}); term {
		t.Fatal("session terminated on an unknown Code")
	}
	pkt, ok := findCode(t, rec, LCPCodeReject)
	if !ok {
		t.Fatalf("no Code-Reject emitted; frames=%d", rec.count())
	}
	if len(pkt.Data) < 4 {
		t.Fatalf("Rejected-Packet = %x, want at least the 4-octet header", pkt.Data)
	}
	if pkt.Data[0] != 99 {
		t.Fatalf("Rejected-Packet Code = %d, want 99", pkt.Data[0])
	}
	if pkt.Data[1] != 0x41 {
		t.Fatalf("Rejected-Packet Identifier = 0x%02x, want 0x41", pkt.Data[1])
	}
	if got := binary.BigEndian.Uint16(pkt.Data[2:4]); got != 6 {
		t.Fatalf("Rejected-Packet Length = %d, want 6", got)
	}
	if !bytes.Equal(pkt.Data[4:], []byte{0xAA, 0xBB}) {
		t.Fatalf("Rejected-Packet Data = %x, want aabb", pkt.Data[4:])
	}
}

// VALIDATES: a known Code is not Code-Rejected.
//
// RFC requirement: RFC1661-5.6-1 negative -- codeToEvent (session_run.go) maps
// Terminate-Request to RTR, whose edge is sta, so a recognized Code draws its
// own reply and never a Code-Reject.
func TestRFC1661NoCodeRejectForKnownCode(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	if term := s.handleLCPPacket(LCPPacket{Code: LCPTerminateRequest, Identifier: 0x42}); term {
		t.Fatal("session terminated on a Terminate-Request")
	}
	if _, ok := findCode(t, rec, LCPCodeReject); ok {
		t.Fatal("Code-Reject emitted for a recognized Code")
	}
}

// ---------------------------------------------------------------------------
// Section 5.8 / 5.9 -- Echo and Discard
// ---------------------------------------------------------------------------

// VALIDATES: an Echo-Request received in Opened is answered with an Echo-Reply
//
//	carrying ze's own Magic-Number and the request's Identifier.
//
// RFC requirement: RFC1661-5.8-1 positive -- the Opened+RXR edge (ppp_fsm.go)
// is ser, which performAction routes to sendEchoReply (session_run.go).
//
// RFC requirement: RFC1661-5.8-2 negative -- the Echo-Reply IS transmitted here
// because the automaton is in Opened, which is what makes the "only in Opened"
// restriction below observable rather than vacuous.
//
// RFC requirement: RFC1661-5.9-2 negative -- an Echo-Request in the same state
// DOES draw a reply, so the silence Discard-Request receives is specific to
// Discard-Request rather than a blanket refusal to answer maintenance packets.
//
// RFC requirement: RFC1661-6.4-2 positive -- BuildLCPEchoReply (echo.go) writes
// the session's negotiated Magic-Number into the Magic-Number field.
func TestRFC1661EchoReplyInOpened(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	peerMagic := []byte{0x99, 0x88, 0x77, 0x66}
	if term := s.handleLCPPacket(LCPPacket{
		Code: LCPEchoRequest, Identifier: 0x51, Data: peerMagic,
	}); term {
		t.Fatal("session terminated on an Echo-Request")
	}
	pkt, ok := findCode(t, rec, LCPEchoReply)
	if !ok {
		t.Fatalf("no Echo-Reply emitted; frames=%d", rec.count())
	}
	if pkt.Identifier != 0x51 {
		t.Fatalf("Echo-Reply Identifier = 0x%02x, want 0x51", pkt.Identifier)
	}
	magic, err := ParseLCPEchoMagic(pkt.Data)
	if err != nil {
		t.Fatalf("ParseLCPEchoMagic: %v", err)
	}
	if magic != s.magic {
		t.Fatalf("Echo-Reply Magic-Number = 0x%08x, want the local 0x%08x", magic, s.magic)
	}
}

// VALIDATES: an Echo-Request received outside Opened draws no Echo-Reply.
//
// RFC requirement: RFC1661-5.8-1 negative -- outside Opened the RXR edges in
// LCPDoTransition (ppp_fsm.go) carry no ser action, so no Echo packet is put
// on the wire.
//
// RFC requirement: RFC1661-5.8-2 positive -- Echo-Request and Echo-Reply are
// emitted only from the Opened state; in Req-Sent, Ack-Sent and Ack-Rcvd the
// automaton stays put and transmits nothing.
func TestRFC1661NoEchoOutsideOpened(t *testing.T) {
	for _, st := range []LCPState{LCPStateReqSent, LCPStateAckSent, LCPStateAckRcvd, LCPStateStopped} {
		s, rec, _ := newRFC1661Session(st)
		if term := s.handleLCPPacket(LCPPacket{
			Code: LCPEchoRequest, Identifier: 0x52, Data: []byte{1, 2, 3, 4},
		}); term {
			t.Fatalf("%s: session terminated on an Echo-Request", st)
		}
		if _, ok := findCode(t, rec, LCPEchoReply); ok {
			t.Fatalf("%s: Echo-Reply emitted outside the Opened state", st)
		}
		if _, ok := findCode(t, rec, LCPEchoRequest); ok {
			t.Fatalf("%s: Echo-Request emitted outside the Opened state", st)
		}
	}
}

// VALIDATES: a received Discard-Request is silently discarded.
//
// RFC requirement: RFC1661-5.9-2 positive -- performAction (session_run.go)
// guards the ser action on the packet being an Echo-Request, so a
// Discard-Request in Opened produces no wire output, no state change and no
// lifecycle event.
func TestRFC1661DiscardRequestSilentlyDiscarded(t *testing.T) {
	s, rec, events := newRFC1661Session(LCPStateOpened)
	if term := s.handleLCPPacket(LCPPacket{
		Code: LCPDiscardRequest, Identifier: 0x53, Data: []byte{1, 2, 3, 4, 5, 6},
	}); term {
		t.Fatal("session terminated on a Discard-Request")
	}
	if n := rec.count(); n != 0 {
		t.Fatalf("wrote %d frames in reply to a Discard-Request, want 0", n)
	}
	if got := s.currentState(); got != LCPStateOpened {
		t.Fatalf("state = %s, want opened", got)
	}
	select {
	case ev := <-events:
		t.Fatalf("unexpected lifecycle event %T on a Discard-Request", ev)
	default:
	}
}

// ---------------------------------------------------------------------------
// Section 6 -- Configuration options
// ---------------------------------------------------------------------------

// VALIDATES: at most one Authentication-Protocol option is placed in a
//
//	Configure-Request.
//
// RFC requirement: RFC1661-6.2-1 positive -- LCPOptions carries a single
// AuthProto scalar and BuildLocalConfigRequest (lcp_options.go) appends the
// option once, so no configuration can produce two of them.
func TestRFC1661SingleAuthProtocolOptionInRequest(t *testing.T) {
	for _, m := range []AuthMethod{AuthMethodPAP, AuthMethodCHAPMD5, AuthMethodMSCHAPv2} {
		proto, extra := authMethodToLCPOptions(m)
		opts := BuildLocalConfigRequest(LCPOptions{MRU: 1500, Magic: 0x11223344, AuthProto: proto, AuthData: extra})
		count := 0
		for _, o := range opts {
			if o.Type == LCPOptAuthProto {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("%s: Configure-Request carries %d Authentication-Protocol options, want 1", m, count)
		}
	}
}

// VALIDATES: a well-formed Magic-Number option from the peer is acknowledged,
//
//	never Configure-Rejected.
//
// RFC requirement: RFC1661-6.4-1 positive -- ze always transmits a
// Magic-Number option (the session draws a non-zero magic at start-up and
// sendConfigureRequest always includes it), and negotiatePeerOption
// (lcp_options.go) answers a well-formed peer Magic-Number with negAck, so it
// is never in the Configure-Reject list.
//
// RFC requirement: RFC1661-6.4-3 negative -- a non-zero Magic-Number is
// accepted, which is what makes the mandatory refusal of zero meaningful.
func TestRFC1661PeerMagicNumberAcked(t *testing.T) {
	acks, naks, rejects := NegotiatePeerOptions([]LCPOption{magicOption(0xDEADBEEF)}, LCPNegPolicy{})
	if len(rejects) != 0 {
		t.Fatalf("rejects = %+v, want none for a well-formed Magic-Number", rejects)
	}
	if len(naks) != 0 {
		t.Fatalf("naks = %+v, want none", naks)
	}
	if len(acks) != 1 || acks[0].Type != LCPOptMagic {
		t.Fatalf("acks = %+v, want the Magic-Number option", acks)
	}
}

// VALIDATES: options ze does not implement ARE Configure-Rejected, so the
//
//	acceptance of Magic-Number above is a deliberate exclusion.
//
// RFC requirement: RFC1661-6.4-1 negative -- the Configure-Reject path is live
// in negotiatePeerOption (lcp_options.go) for unrecognized option types; the
// Magic-Number option is kept out of it.
func TestRFC1661UnknownOptionRejectedWhileMagicIsNot(t *testing.T) {
	opts := []LCPOption{{Type: 99, Data: []byte{0x01}}, magicOption(0xCAFEBABE)}
	acks, _, rejects := NegotiatePeerOptions(opts, LCPNegPolicy{})
	if len(rejects) != 1 || rejects[0].Type != 99 {
		t.Fatalf("rejects = %+v, want only the unknown type 99", rejects)
	}
	if len(acks) != 1 || acks[0].Type != LCPOptMagic {
		t.Fatalf("acks = %+v, want the Magic-Number option", acks)
	}
}

// VALIDATES: a Magic-Number of zero is refused outright.
//
// RFC requirement: RFC1661-6.4-3 positive -- negotiatePeerOption
// (lcp_options.go) returns negReject when the four Magic-Number octets decode
// to zero, so a zero Magic-Number is Rejected rather than accepted.
func TestRFC1661ZeroMagicNumberRefused(t *testing.T) {
	acks, _, rejects := NegotiatePeerOptions([]LCPOption{magicOption(0)}, LCPNegPolicy{})
	if len(acks) != 0 {
		t.Fatalf("acks = %+v, want none for a zero Magic-Number", acks)
	}
	if len(rejects) != 1 {
		t.Fatalf("rejects = %d, want 1", len(rejects))
	}
}

// VALIDATES: the Echo-Reply carries ze's own Magic-Number, not the peer's value
//
//	echoed back.
//
// RFC requirement: RFC1661-6.4-2 negative -- BuildLCPEchoReply (echo.go) drops
// the request's first four octets (the peer's magic) and writes the local
// Magic-Number, so a reply never mirrors the peer's value.
func TestRFC1661EchoReplyDoesNotMirrorPeerMagic(t *testing.T) {
	const localMagic uint32 = 0x01020304
	const peerMagic uint32 = 0xA1A2A3A4
	req := make([]byte, 4)
	binary.BigEndian.PutUint32(req, peerMagic)

	buf := make([]byte, 64)
	n := BuildLCPEchoReply(buf, 0, 0x61, localMagic, req)
	pkt, err := ParseLCPPacket(buf[:n])
	if err != nil {
		t.Fatalf("ParseLCPPacket: %v", err)
	}
	got, err := ParseLCPEchoMagic(pkt.Data)
	if err != nil {
		t.Fatalf("ParseLCPEchoMagic: %v", err)
	}
	if got == peerMagic {
		t.Fatal("Echo-Reply mirrored the peer's Magic-Number")
	}
	if got != localMagic {
		t.Fatalf("Echo-Reply Magic-Number = 0x%08x, want the local 0x%08x", got, localMagic)
	}
}
