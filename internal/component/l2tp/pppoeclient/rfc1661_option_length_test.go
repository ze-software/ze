// VALIDATES: RFC 1661 Section 6 option Length and Data handling on the PPPoE
// client's LCP receive path -- a server Configure-Request whose option Data runs
// past the end of the packet draws no reply and leaves the negotiation where it
// was, while an option whose Length is merely invalid draws a Configure-Nak
// carrying the option the client wants.
// PREVENTS: the client answering a malformed Configure-Request with a
// Configure-Reject that echoes the sender's own octets back, which both violates
// Section 6 and makes the client a reflector for a packet an off-path sender can
// forge.
//
// RFC: rfc/short/rfc1661.md
package pppoeclient

import (
	"bytes"
	"encoding/binary"
	"io"
	"log/slog"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// frameLog records every frame the client writes, one entry per Write, so a
// test can tell "the client sent nothing" from "the client sent something the
// test did not read".
type frameLog struct {
	frames [][]byte
}

func (f *frameLog) Write(p []byte) (int, error) {
	f.frames = append(f.frames, append([]byte(nil), p...))
	return len(p), nil
}

// lcpPackets decodes every LCP packet the client wrote.
func (f *frameLog) lcpPackets(t *testing.T) []ppp.LCPPacket {
	t.Helper()
	var out []ppp.LCPPacket
	for _, frame := range f.frames {
		proto, payload, _, err := ppp.ParseFrame(frame)
		if err != nil {
			t.Fatalf("ParseFrame(% x): %v", frame, err)
		}
		if proto != ppp.ProtoLCP {
			continue
		}
		pkt, err := ppp.ParseLCPPacket(payload)
		if err != nil {
			t.Fatalf("ParseLCPPacket(% x): %v", payload, err)
		}
		out = append(out, pkt)
	}
	return out
}

// serverFrame builds one LCP frame as the server would send it.
func serverFrame(code, id uint8, data []byte) readFrame {
	buf := make([]byte, ppp.MaxFrameLen)
	off := ppp.WriteFrame(buf, 0, ppp.ProtoLCP, nil)
	off += ppp.WriteLCPPacket(buf, off, code, id, data)
	return readFrame{data: buf[:off]}
}

// clientMagic is the Magic-Number runClientLCP gives the client.
const clientMagic uint32 = 0x11223344

// runClientLCP drives negotiateLCP against a scripted server. The script is
// followed by the two packets that carry the client to Opened, so the call
// returns rather than waiting out its 30-second deadline: a Configure-Ack of the
// client's own request, then a well-formed server Configure-Request the client
// Acks. Returns every frame the client wrote, in order.
func runClientLCP(t *testing.T, script ...readFrame) *frameLog {
	t.Helper()
	return runClientLCPWithMagic(t, clientMagic, script...)
}

// runClientLCPWithMagic is runClientLCP with the client's own Magic-Number
// chosen by the caller. A test that reads the Magic-Number the client offers a
// peer needs two runs that differ in that value, to tell a value derived from
// the client's own Magic-Number from a constant.
func runClientLCPWithMagic(t *testing.T, magic uint32, script ...readFrame) *frameLog {
	t.Helper()
	const clientMTU = 1492

	frames := make(chan readFrame, len(script)+2)
	for _, f := range script {
		frames <- f
	}
	// MRU 1492 at Length 4: recognized, acceptable, and inside the packet.
	goodOptions := []byte{ppp.LCPOptMRU, 4, 0x05, 0xD4}
	frames <- serverFrame(ppp.LCPConfigureAck, 1, nil)
	frames <- serverFrame(ppp.LCPConfigureRequest, 0x20, goodOptions)

	log := &frameLog{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, err := negotiateLCP(log, frames, make([]byte, ppp.MaxFrameLen),
		sessionConfig{mtu: clientMTU}, magic, make(chan struct{}), logger); err != nil {
		t.Fatalf("negotiateLCP: %v", err)
	}
	return log
}

// RFC requirement: RFC1661-6-2 positive -- a server Configure-Request whose
// option Data is indicated by its Length to extend beyond the end of the
// Information field makes the PPPoE client discard the entire packet: it writes
// no reply for that packet and its negotiation is untouched, which the
// well-formed request that follows proves by still drawing a Configure-Ack
// (producer negotiateLCP's LCPConfigureRequest arm, fed by ppp.WalkLCPOptions,
// internal/component/l2tp/pppoeclient/session.go).
// RFC requirement: RFC1661-6-2 negative -- the silence is confined to packets
// that are invalid. A Configure-Request whose options fit inside the packet
// still draws a Configure-Ack, so the client is not simply refusing to answer.
func TestRFC1661ClientRequestPastEndSilentlyDiscarded(t *testing.T) {
	pastEndCases := []struct {
		name string
		data []byte
	}{
		// MRU declares Length 4 with only two of those four octets present,
		// so its Data runs two octets past the packet.
		{"data runs past the end", []byte{ppp.LCPOptMRU, 4, 0x05}},
		// One octet left over cannot hold a Type and a Length, so the
		// option's own Length field is already past the end.
		{"header does not fit", []byte{ppp.LCPOptMRU}},
	}
	for _, tc := range pastEndCases {
		t.Run(tc.name, func(t *testing.T) {
			log := runClientLCP(t, serverFrame(ppp.LCPConfigureRequest, 0x10, tc.data))

			pkts := log.lcpPackets(t)
			for _, pkt := range pkts {
				if pkt.Identifier == 0x10 {
					t.Fatalf("the client answered a Configure-Request RFC 1661 Section 6 discards with code %d carrying % x", pkt.Code, pkt.Data)
				}
				if pkt.Code == ppp.LCPConfigureReject {
					t.Fatalf("the client sent a Configure-Reject carrying % x; RFC 1661 Section 6 discards the packet instead", pkt.Data)
				}
			}
			// The negotiation must still be live: the well-formed request
			// runClientLCP sends last is Acked.
			if !hasReply(pkts, ppp.LCPConfigureAck, 0x20) {
				t.Fatalf("no Configure-Ack for the well-formed request after the discard; frames=%d", len(pkts))
			}
		})
	}
}

// RFC requirement: RFC1661-6-1 positive -- a negotiable option received by the
// PPPoE client with an invalid Length draws a Configure-Nak carrying the desired
// Configuration Option with an appropriate Length and Data: MRU at Length 4
// holding the 1492 the client accepts (producer negotiateLCP's
// LCPOptionsBadLength arm feeding ppp.LCPNakOrReject,
// internal/component/l2tp/pppoeclient/session.go).
// RFC requirement: RFC1661-6-1 negative -- the Nak is confined to the invalid
// Length. A request whose options all carry a valid Length and acceptable values
// draws a Configure-Ack, and no Configure-Reject echoing the request's own Data
// is sent for either.
func TestRFC1661ClientInvalidOptionLengthDrawsNak(t *testing.T) {
	// MRU declares Length 1, fewer octets than its own Type and Length fields
	// occupy, while every octet of the option is inside the packet.
	log := runClientLCP(t, serverFrame(ppp.LCPConfigureRequest, 0x11, []byte{ppp.LCPOptMRU, 1, 0x05, 0xDC}))

	pkts := log.lcpPackets(t)
	var nak ppp.LCPPacket
	found := false
	for _, pkt := range pkts {
		if pkt.Code == ppp.LCPConfigureReject {
			t.Fatalf("the client sent a Configure-Reject carrying % x; RFC 1661 Section 6 wants a Configure-Nak with the desired option", pkt.Data)
		}
		if pkt.Code == ppp.LCPConfigureNak && pkt.Identifier == 0x11 {
			nak = pkt
			found = true
		}
	}
	if !found {
		t.Fatalf("no Configure-Nak for the invalid-Length option; frames=%d", len(pkts))
	}

	opts, err := ppp.ParseLCPOptions(nak.Data)
	if err != nil {
		t.Fatalf("Configure-Nak Data % x does not parse: %v; RFC 1661 Section 6 wants the desired option at an appropriate Length", nak.Data, err)
	}
	if len(opts) != 1 || opts[0].Type != ppp.LCPOptMRU {
		t.Fatalf("Configure-Nak Data % x carries no MRU option", nak.Data)
	}
	if len(opts[0].Data) != 2 {
		t.Fatalf("Nak'd MRU carries %d data octets, want the 2 RFC 1661 Section 6.1 gives it", len(opts[0].Data))
	}
	if got := binary.BigEndian.Uint16(opts[0].Data); got != 1492 {
		t.Errorf("Nak'd MRU = %d, want the 1492 the client accepts", got)
	}
	if !hasReply(pkts, ppp.LCPConfigureAck, 0x20) {
		t.Fatalf("no Configure-Ack for the well-formed request that followed; frames=%d", len(pkts))
	}
}

// RFC requirement: RFC1661-5.4-1 positive -- a server Configure-Request carrying
// both an unrecognized option Type and an invalid option Length draws the
// Configure-Reject Section 5.4 makes mandatory, carrying the unrecognized Type
// and not the whole request (producer ppp.LCPNakOrReject, reached from
// internal/component/l2tp/pppoeclient/session.go).
// RFC requirement: RFC1661-5.4-1 negative -- the Reject carries only the option
// that earned it. The client does not echo the request's own Data back, which is
// what made it a reflector before.
func TestRFC1661ClientUnrecognizedTypeOutranksInvalidLength(t *testing.T) {
	request := []byte{99, 3, 0xAA, ppp.LCPOptMRU, 1, 0x05}
	log := runClientLCP(t, serverFrame(ppp.LCPConfigureRequest, 0x12, request))

	pkts := log.lcpPackets(t)
	var rej ppp.LCPPacket
	found := false
	for _, pkt := range pkts {
		if pkt.Code == ppp.LCPConfigureNak && pkt.Identifier == 0x12 {
			t.Fatalf("the client answered with a Configure-Nak carrying % x; RFC 1661 Section 5.4 makes the Configure-Reject mandatory", pkt.Data)
		}
		if pkt.Code == ppp.LCPConfigureReject && pkt.Identifier == 0x12 {
			rej = pkt
			found = true
		}
	}
	if !found {
		t.Fatalf("no Configure-Reject for the unrecognized option Type; frames=%d", len(pkts))
	}
	opts, err := ppp.ParseLCPOptions(rej.Data)
	if err != nil {
		t.Fatalf("Configure-Reject Data % x does not parse: %v", rej.Data, err)
	}
	if len(opts) != 1 || opts[0].Type != 99 {
		t.Fatalf("Configure-Reject Data % x does not carry the unrecognized option Type 99 alone", rej.Data)
	}
}

// RFC requirement: RFC1661-5.4-2 positive -- the PPPoE client's Configure-
// Reject carries the refused option exactly as it arrived, its own Length octet
// included, for the two Lengths ze cannot re-encode. RFC 1661 Section 5.4: "The
// Options field is filled with only the unacceptable Configuration Options from
// the Configure-Request. All recognizable and negotiable Configuration Options
// are filtered out of the Configure-Reject, but otherwise the Configuration
// Options MUST NOT be reordered or modified in any way." A Length below 2 is
// one the option encoder can never write, so rebuilding the option from its
// Type sends back an option the server never sent (producer
// ppp.LCPNakOrReject's LCPOptionsBadLength arm feeding ppp.LCPOption.Raw
// through ppp.WriteLCPOptions, reached from negotiateLCP's LCPOptionsBadLength
// arm, internal/component/l2tp/pppoeclient/session.go).
// RFC requirement: RFC1661-5.4-2 negative -- "modified in any way" governs the
// options the Reject carries, never which options it carries. The
// Authentication-Protocol option the client recognizes and accepts is still
// filtered OUT, so the echo does not turn the Configure-Reject into a copy of
// the server's request.
func TestRFC1661ClientRejectEchoesTheRefusedOptionUnmodified(t *testing.T) {
	// Type 99 is unrecognized, so the client holds no value to Nak it with and
	// Section 5.4 takes it. Both Lengths count fewer octets than the option's
	// own Type and Length fields occupy, and both are inside the packet.
	echoCases := []struct {
		name string
		data []byte
	}{
		{"unrecognized Type at Length 0", []byte{99, 0}},
		{"unrecognized Type at Length 1", []byte{99, 1}},
	}
	for _, tc := range echoCases {
		t.Run(tc.name, func(t *testing.T) {
			log := runClientLCP(t, serverFrame(ppp.LCPConfigureRequest, 0x15, tc.data))

			rej := clientRejectFor(t, log, 0x15)
			if !bytes.Equal(rej.Data, tc.data) {
				t.Fatalf("Configure-Reject Data = % x, want the % x that arrived; RFC 1661 Section 5.4 forbids modifying the option in any way", rej.Data, tc.data)
			}
		})
	}

	t.Run("a recognizable negotiable option is filtered out of the Reject", func(t *testing.T) {
		// CHAP (0xC223) at Length 4 is well formed, recognized and accepted by
		// clientLCPPolicy, so Section 5.4 filters it out. The unrecognized
		// Type at Length 0 that follows is the whole of what the Reject owes.
		refused := []byte{99, 0}
		request := append([]byte{ppp.LCPOptAuthProto, 4, 0xC2, 0x23}, refused...)
		log := runClientLCP(t, serverFrame(ppp.LCPConfigureRequest, 0x16, request))

		rej := clientRejectFor(t, log, 0x16)
		if !bytes.Equal(rej.Data, refused) {
			t.Fatalf("Configure-Reject Data = % x, want the % x that earned it alone; RFC 1661 Section 5.4 filters every recognizable and negotiable option out", rej.Data, refused)
		}
	})
}

// clientRejectFor returns the Configure-Reject the client wrote for id, and
// fails when it wrote a Configure-Nak for it instead. Ze holds no value for an
// unrecognized Type, so a Nak here is the reply RFC 1661 Section 5.4 takes away
// rather than a second acceptable answer.
func clientRejectFor(t *testing.T, log *frameLog, id uint8) ppp.LCPPacket {
	t.Helper()
	for _, pkt := range log.lcpPackets(t) {
		if pkt.Identifier != id {
			continue
		}
		if pkt.Code == ppp.LCPConfigureNak {
			t.Fatalf("the client answered with a Configure-Nak carrying % x; RFC 1661 Section 5.4 makes the Configure-Reject mandatory for an option it cannot negotiate", pkt.Data)
		}
		if pkt.Code == ppp.LCPConfigureReject {
			return pkt
		}
	}
	t.Fatalf("the client wrote no Configure-Reject for id 0x%02x", id)
	return ppp.LCPPacket{}
}

// hasReply reports whether the client wrote a packet with the given code and
// Identifier.
func hasReply(pkts []ppp.LCPPacket, code, id uint8) bool {
	for _, pkt := range pkts {
		if pkt.Code == code && pkt.Identifier == id {
			return true
		}
	}
	return false
}

// clientNakOptions returns the options of the Configure-Nak the client wrote
// for id, and fails when it wrote a Configure-Reject for it instead.
func clientNakOptions(t *testing.T, log *frameLog, id uint8) []ppp.LCPOption {
	t.Helper()
	for _, pkt := range log.lcpPackets(t) {
		if pkt.Identifier != id {
			continue
		}
		if pkt.Code == ppp.LCPConfigureReject {
			t.Fatalf("the client answered with a Configure-Reject carrying % x, not the Configure-Nak the invalid Length earns", pkt.Data)
		}
		if pkt.Code != ppp.LCPConfigureNak {
			continue
		}
		opts, err := ppp.ParseLCPOptions(pkt.Data)
		if err != nil {
			t.Fatalf("Configure-Nak Data % x does not parse: %v", pkt.Data, err)
		}
		return opts
	}
	t.Fatalf("the client wrote no Configure-Nak for id 0x%02x", id)
	return nil
}

// VALIDATES: the PPPoE client accepts the server's Authentication-Protocol
//
//	option when it judges a Configure-Request, rather than Configure-
//	Rejecting the one option the client exists to read.
//
// PREVENTS: clientLCPPolicy losing AcceptAuthProto. Ze's negotiation policy
// Configure-Rejects the Authentication-Protocol option whenever that field is
// false (negotiatePeerOption's LCPOptAuthProto arm,
// internal/component/l2tp/ppp/lcp_options.go), which refuses the method the
// server picked and leaves the client with no authentication to run.
//
// RFC requirement: RFC1661-5.4-1 negative -- the Configure-Reject is owed only
// for options that are "not recognizable or are not acceptable for
// negotiation". The client recognizes the Authentication-Protocol option and
// accepts it, so the reply it builds must not carry one (producer
// clientLCPPolicy feeding ppp.LCPNakOrReject,
// internal/component/l2tp/pppoeclient/session.go).
func TestRFC1661ClientAcceptsServerAuthProtocol(t *testing.T) {
	// CHAP (0xC223) at Length 4 is well formed, recognized and acceptable. The
	// MRU that follows declares Length 1, fewer octets than its own header
	// holds, so the walk stops there and the client answers through
	// ppp.LCPNakOrReject, which judges the option read before the fault
	// against clientLCPPolicy.
	request := []byte{ppp.LCPOptAuthProto, 4, 0xC2, 0x23, ppp.LCPOptMRU, 1, 0x05}
	log := runClientLCP(t, serverFrame(ppp.LCPConfigureRequest, 0x13, request))

	for _, pkt := range log.lcpPackets(t) {
		if pkt.Code != ppp.LCPConfigureReject {
			continue
		}
		opts, err := ppp.ParseLCPOptions(pkt.Data)
		if err != nil {
			t.Fatalf("Configure-Reject Data % x does not parse: %v", pkt.Data, err)
		}
		for _, opt := range opts {
			if opt.Type == ppp.LCPOptAuthProto {
				t.Fatalf("the client Configure-Rejected the server's Authentication-Protocol option (Data % x); it is recognized and acceptable, so RFC 1661 Section 5.4 leaves nothing to Reject and the client has no authentication method to run", pkt.Data)
			}
		}
	}

	// The invalid Length is still answered, so the Auth-Protocol option was
	// judged rather than skipped.
	opts := clientNakOptions(t, log, 0x13)
	if len(opts) != 1 || opts[0].Type != ppp.LCPOptMRU {
		t.Fatalf("Configure-Nak carries %+v, want the MRU option alone", opts)
	}
}

// VALIDATES: the Magic-Number the PPPoE client offers in a Configure-Nak
//
//	follows from the client's own Magic-Number, so it can never be the value
//	that would read as a looped-back link.
//
// PREVENTS: clientLCPPolicy losing LocalMagic. With that field zero, nakMagic
// (internal/component/l2tp/ppp/lcp_options.go) offers the fixed 0xFFFFFFFF to
// every peer, which is ze's own Magic-Number for one client in 2**32 and is the
// same number for all of them.
//
// RFC requirement: RFC1661-6.4-3 positive -- a Magic-Number of zero draws a
// Configure-Nak from the client, carrying a Magic-Number at the four octets
// Section 6.4 gives it. RFC 1661 Section 6.4: "A Magic-Number of zero is
// illegal and MUST always be Nak'd, if it is not Rejected outright" (producer
// negotiatePeerOption's LCPOptMagic arm, reached through clientLCPPolicy from
// internal/component/l2tp/pppoeclient/session.go).
// RFC requirement: RFC1661-6.4-3 negative -- the value offered is not a
// constant the client would send whatever its own Magic-Number is. Two runs
// that differ only in the client's Magic-Number offer two different values, so
// the offer is derived from it and RFC 1661 Section 6.4's "a Configure-Nak MUST
// be sent specifying a different Magic-Number value" stays satisfiable.
func TestRFC1661ClientNaksZeroMagicWithAValueOfItsOwn(t *testing.T) {
	// Magic-Number zero at Length 6 is well formed and illegal. The MRU that
	// follows carries the invalid Length that routes the reply through
	// ppp.LCPNakOrReject.
	request := []byte{ppp.LCPOptMagic, 6, 0, 0, 0, 0, ppp.LCPOptMRU, 1, 0x05}

	offered := func(t *testing.T, ownMagic uint32) uint32 {
		t.Helper()
		log := runClientLCPWithMagic(t, ownMagic, serverFrame(ppp.LCPConfigureRequest, 0x14, request))
		for _, opt := range clientNakOptions(t, log, 0x14) {
			if opt.Type != ppp.LCPOptMagic {
				continue
			}
			if len(opt.Data) != 4 {
				t.Fatalf("Nak'd Magic-Number carries %d octets, want the 4 RFC 1661 Section 6.4 gives it", len(opt.Data))
			}
			value := binary.BigEndian.Uint32(opt.Data)
			if value == 0 {
				t.Fatal("the client offered a Magic-Number of zero, which RFC 1661 Section 6.4 calls illegal")
			}
			if value == ownMagic {
				t.Fatalf("the client offered its own Magic-Number 0x%08x; RFC 1661 Section 6.4 reads two equal Magic-Numbers as a looped-back link", value)
			}
			return value
		}
		t.Fatalf("the client's Configure-Nak carries no Magic-Number option; RFC 1661 Section 6.4 requires the zero value to be Nak'd")
		return 0
	}

	first := offered(t, clientMagic)
	second := offered(t, 0x55667788)
	if first == second {
		t.Fatalf("the client offered 0x%08x from both of its own Magic-Numbers; the offer must follow from the client's Magic-Number, or it is a constant that collides with the peer's whatever the client chose", first)
	}
}
