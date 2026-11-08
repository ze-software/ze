// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- IPv6CP in the network phase
// Related: ipv6cp_client.go -- the automaton under test

package pppoeclient

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// ipv6cpWire records each frame the IPv6CP automaton writes, one entry per
// Write, so a test reads the client's packets in order.
type ipv6cpWire struct{ frames [][]byte }

func (w *ipv6cpWire) Write(p []byte) (int, error) {
	w.frames = append(w.frames, append([]byte(nil), p...))
	return len(p), nil
}

// ipv6cpTestMAC is the source interface EUI-48; its modified EUI-64 is
// 50:54:00:ff:fe:12:34:56 (RFC 5072 Section 4.1 case 1).
var ipv6cpTestMAC = []byte{0x52, 0x54, 0x00, 0x12, 0x34, 0x56}

var ipv6cpTestEUI64 = ppp.IPv6CPInterfaceID{0x50, 0x54, 0x00, 0xff, 0xfe, 0x12, 0x34, 0x56}

// lastIPv6CP parses the most recent frame the client wrote, failing the test
// when it is not an IPv6CP packet.
func lastIPv6CP(t *testing.T, w *ipv6cpWire) ppp.LCPPacket {
	t.Helper()
	if len(w.frames) == 0 {
		t.Fatal("the client wrote no frame")
	}
	proto, payload, _, err := ppp.ParseFrame(w.frames[len(w.frames)-1])
	if err != nil {
		t.Fatalf("parse frame: %v", err)
	}
	if proto != ppp.ProtoIPv6CP {
		t.Fatalf("protocol 0x%04x, want IPv6CP 0x8057", proto)
	}
	pkt, err := ppp.ParseLCPPacket(payload)
	if err != nil {
		t.Fatalf("parse packet: %v", err)
	}
	return pkt
}

// ipv6cpOption is one Interface-Identifier option as it goes on the wire.
func ipv6cpOption(id ppp.IPv6CPInterfaceID) []byte {
	option := make([]byte, ppp.IPv6CPInterfaceIDOptionLen)
	ppp.WriteIPv6CPInterfaceID(option, 0, id)
	return option
}

func startedIPv6CP(t *testing.T) (*clientIPv6CP, *ipv6cpWire, []byte) {
	t.Helper()
	c := newClientIPv6CP(ipv6cpTestMAC, 1492)
	w := &ipv6cpWire{}
	buf := make([]byte, ppp.MaxFrameBufLen)
	if err := c.start(w, buf); err != nil {
		t.Fatalf("start: %v", err)
	}
	return c, w, buf
}

// TestRFC5072ClientConfigureRequestOneIdentifier checks AC-1's request: the
// first Configure-Request carries exactly one Interface-Identifier option,
// non-zero, built from the source interface's EUI-48. Method: start the
// automaton and parse the one frame it writes.
//
// RFC requirement: RFC5072-4.1-1 positive -- the PPPoE client's first IPv6CP
// Configure-Request carries exactly one 10-octet Interface-Identifier option,
// holding the modified EUI-64 of the source interface's EUI-48.
func TestRFC5072ClientConfigureRequestOneIdentifier(t *testing.T) {
	_, w, _ := startedIPv6CP(t)
	if len(w.frames) != 1 {
		t.Fatalf("start wrote %d frames, want 1", len(w.frames))
	}
	pkt := lastIPv6CP(t, w)
	if pkt.Code != ppp.LCPConfigureRequest {
		t.Fatalf("code %d, want Configure-Request", pkt.Code)
	}
	if len(pkt.Data) != ppp.IPv6CPInterfaceIDOptionLen {
		t.Fatalf("options %x: want exactly one 10-octet Interface-Identifier", pkt.Data)
	}
	id, present, err := ppp.ParseIPv6CPInterfaceID(pkt.Data)
	if err != nil || !present {
		t.Fatalf("Interface-Identifier present=%v err=%v", present, err)
	}
	if id != ipv6cpTestEUI64 {
		t.Fatalf("identifier %x, want the modified EUI-64 %x", id, ipv6cpTestEUI64)
	}
}

// TestRFC5072ClientTentativeWithoutMACClearsUBit checks Section 4.1 case 3:
// with no EUI-48 the identifier is random, non-zero, with the "u" bit zero.
func TestRFC5072ClientTentativeWithoutMACClearsUBit(t *testing.T) {
	for range 64 {
		id, err := ppp.TentativeIPv6CPInterfaceID(nil)
		if err != nil {
			t.Fatalf("tentative: %v", err)
		}
		if !ppp.ValidIPv6CPInterfaceID(id) {
			t.Fatalf("identifier %x is not valid", id)
		}
		if id[0]&0x02 != 0 {
			t.Fatalf("identifier %x has the u bit set", id)
		}
	}
}

// TestRFC5072ClientAdoptsNakSuggestion checks AC-2: after a Configure-Nak
// that suggests an identifier, the next Configure-Request carries it, under a
// new Identifier.
//
// RFC requirement: RFC5072-4.1-7 positive -- after a Configure-Nak suggesting
// identifier 02:00:00:00:00:00:00:77, the PPPoE client's next IPv6CP
// Configure-Request carries that identifier, under a new Identifier.
func TestRFC5072ClientAdoptsNakSuggestion(t *testing.T) {
	c, w, buf := startedIPv6CP(t)
	first := lastIPv6CP(t, w)
	suggested := ppp.IPv6CPInterfaceID{0x02, 0, 0, 0, 0, 0, 0, 0x77}
	nak := ppp.LCPPacket{Code: ppp.LCPConfigureNak, Identifier: first.Identifier, Data: ipv6cpOption(suggested)}
	if _, err := c.handle(w, buf, nak, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("handle Nak: %v", err)
	}
	next := lastIPv6CP(t, w)
	if next.Code != ppp.LCPConfigureRequest {
		t.Fatalf("code %d, want a new Configure-Request", next.Code)
	}
	if next.Identifier == first.Identifier {
		t.Fatalf("Identifier %d reused after a valid reply", next.Identifier)
	}
	id, _, err := ppp.ParseIPv6CPInterfaceID(next.Data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if id != suggested {
		t.Fatalf("identifier %x, want the suggestion %x", id, suggested)
	}
}

// TestRFC5072ClientNakEqualToOwnNakChoosesNew checks the Section 4.1 rule:
// a Nak that suggests the value this client last Naked makes it choose a new
// identifier rather than adopt it.
func TestRFC5072ClientNakEqualToOwnNakChoosesNew(t *testing.T) {
	c, w, buf := startedIPv6CP(t)
	logger := slogutil.DiscardLogger()
	ours := lastIPv6CP(t, w)
	// The peer requests our own identifier, so the client Naks a suggestion.
	request := ppp.LCPPacket{Code: ppp.LCPConfigureRequest, Identifier: 9, Data: ipv6cpOption(ipv6cpTestEUI64)}
	if _, err := c.handle(w, buf, request, logger); err != nil {
		t.Fatalf("handle request: %v", err)
	}
	nakSent := lastIPv6CP(t, w)
	if nakSent.Code != ppp.LCPConfigureNak {
		t.Fatalf("code %d, want Configure-Nak for an equal identifier", nakSent.Code)
	}
	suggestion, _, err := ppp.ParseIPv6CPInterfaceID(nakSent.Data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if suggestion == ipv6cpTestEUI64 {
		t.Fatal("the suggestion equals the client's own identifier")
	}
	if suggestion[0]&0x02 != 0 {
		t.Fatalf("suggestion %x has the u bit set", suggestion)
	}
	// The peer Naks our request with that same suggestion.
	nak := ppp.LCPPacket{Code: ppp.LCPConfigureNak, Identifier: ours.Identifier, Data: ipv6cpOption(suggestion)}
	if _, err := c.handle(w, buf, nak, logger); err != nil {
		t.Fatalf("handle Nak: %v", err)
	}
	id, _, err := ppp.ParseIPv6CPInterfaceID(lastIPv6CP(t, w).Data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if id == suggestion {
		t.Fatalf("identifier %x adopted from a Nak equal to the client's own Nak", id)
	}
}

// TestRFC5072ClientAnswersPeerIdentifier checks the Section 4.1 comparison
// table: a different non-zero identifier is Acknowledged, a zero one Naked
// with a non-zero suggestion.
func TestRFC5072ClientAnswersPeerIdentifier(t *testing.T) {
	cases := []struct {
		name string
		peer ppp.IPv6CPInterfaceID
		code uint8
	}{
		{"different-non-zero", ppp.IPv6CPInterfaceID{0, 0, 0, 0, 0, 0, 0, 1}, ppp.LCPConfigureAck},
		{"zero", ppp.IPv6CPInterfaceID{}, ppp.LCPConfigureNak},
		{"equal", ipv6cpTestEUI64, ppp.LCPConfigureNak},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w, buf := startedIPv6CP(t)
			request := ppp.LCPPacket{Code: ppp.LCPConfigureRequest, Identifier: 4, Data: ipv6cpOption(tc.peer)}
			if _, err := c.handle(w, buf, request, slogutil.DiscardLogger()); err != nil {
				t.Fatalf("handle: %v", err)
			}
			reply := lastIPv6CP(t, w)
			if reply.Code != tc.code {
				t.Fatalf("code %d, want %d", reply.Code, tc.code)
			}
			if reply.Identifier != 4 {
				t.Fatalf("reply Identifier %d, want the request's 4", reply.Identifier)
			}
			if tc.code != ppp.LCPConfigureNak {
				return
			}
			suggestion, _, err := ppp.ParseIPv6CPInterfaceID(reply.Data)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if !ppp.ValidIPv6CPInterfaceID(suggestion) {
				t.Fatalf("suggestion %x is not a usable identifier", suggestion)
			}
		})
	}
}

// TestRFC5072ClientLinkLocalFromIdentifier checks AC-1's address: once both
// directions are acknowledged IPv6CP is Opened and the link-local address is
// fe80:: plus the negotiated identifier (RFC 5072 Section 5).
func TestRFC5072ClientLinkLocalFromIdentifier(t *testing.T) {
	c, w, buf := startedIPv6CP(t)
	logger := slogutil.DiscardLogger()
	ours := lastIPv6CP(t, w)
	peer := ppp.LCPPacket{Code: ppp.LCPConfigureRequest, Identifier: 1, Data: ipv6cpOption(ppp.IPv6CPInterfaceID{0, 0, 0, 0, 0, 0, 0, 1})}
	event, err := c.handle(w, buf, peer, logger)
	if err != nil {
		t.Fatalf("handle request: %v", err)
	}
	if event != ipv6cpUnchanged {
		t.Fatalf("event %d after the peer's request alone, want unchanged", event)
	}
	if c.linkLocal().IsValid() {
		t.Fatal("link-local published before IPv6CP opened")
	}
	ack := ppp.LCPPacket{Code: ppp.LCPConfigureAck, Identifier: ours.Identifier, Data: ours.Data}
	event, err = c.handle(w, buf, ack, logger)
	if err != nil {
		t.Fatalf("handle Ack: %v", err)
	}
	if event != ipv6cpOpened {
		t.Fatalf("event %d, want opened", event)
	}
	want := netip.MustParseAddr("fe80::5054:ff:fe12:3456")
	if got := c.linkLocal(); got != want {
		t.Fatalf("link-local %v, want %v", got, want)
	}
}

// TestRFC5072ClientIgnoresMismatchedAck checks that an Ack whose options are
// not the outstanding request's opens nothing (RFC 1661 Section 5.2).
func TestRFC5072ClientIgnoresMismatchedAck(t *testing.T) {
	c, w, buf := startedIPv6CP(t)
	ours := lastIPv6CP(t, w)
	ack := ppp.LCPPacket{Code: ppp.LCPConfigureAck, Identifier: ours.Identifier, Data: ipv6cpOption(ppp.IPv6CPInterfaceID{9})}
	if _, err := c.handle(w, buf, ack, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if c.state != ppp.LCPStateReqSent {
		t.Fatalf("state %d after a mismatched Ack, want Req-Sent", c.state)
	}
}
