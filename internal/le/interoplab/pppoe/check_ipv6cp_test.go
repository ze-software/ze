//go:build ze_l2tp

package pppoe

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	discovery "github.com/ze-software/ze/internal/component/l2tp/pppoe"
)

// The MACs of a session-stage frame pppd sent: client to ze.
var (
	testZeMAC     = []byte{0x02, 0, 0, 0, 0, 0x01}
	testClientMAC = []byte{0x02, 0, 0, 0, 0, 0x02}
)

// sessionFrame builds a session-stage PPPoE frame from source to target
// carrying one IPv6CP packet, the shape tcpdump hands parseIPv6CPFrames.
func sessionFrame(t *testing.T, source, target []byte, code, identifier uint8, options []byte) []byte {
	t.Helper()
	frame := make([]byte, pppSessionOverhead)
	copy(frame[0:6], target)
	copy(frame[6:12], source)
	binary.BigEndian.PutUint16(frame[12:14], discovery.EthPPPSes)
	frame[discovery.EthHdrLen] = 0x11 // PPPoE Ver 1, Type 1
	binary.BigEndian.PutUint16(frame[discovery.EthHdrLen+2:], 0x0007)
	built, err := rebuildIPv6CPConfigureRequest(frame, identifier, options)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	built[pppSessionOverhead+2] = code // the LCP Code byte after the PPP protocol field
	return built
}

// TestRebuildIPv6CPConfigureRequestCarriesTheZeroIdentifier proves the frame
// the zero-identifier checker replays: it parses back as a Configure-Request
// with the Interface-Identifier option present and all-zero, keeps the
// template's MACs and session, and its PPPoE Length covers the new payload.
func TestRebuildIPv6CPConfigureRequestCarriesTheZeroIdentifier(t *testing.T) {
	template := sessionFrame(t, testClientMAC, testZeMAC, ppp.LCPConfigureRequest, 4,
		interfaceIDOption([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))

	built, err := rebuildIPv6CPConfigureRequest(template, 5, interfaceIDOption([8]byte{}))
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	frame, ok, err := parseOneIPv6CPFrame(built)
	if err != nil || !ok {
		t.Fatalf("parse: ok=%v err=%v", ok, err)
	}
	if frame.code != ppp.LCPConfigureRequest || frame.lcpID != 5 {
		t.Fatalf("code %d id %d, want Configure-Request id 5", frame.code, frame.lcpID)
	}
	if !frame.hasOption {
		t.Fatal("the rebuilt request lost its Interface-Identifier option")
	}
	if frame.interfaceID != ([8]byte{}) {
		t.Fatalf("interface identifier %x, want all-zero", frame.interfaceID)
	}
	if !bytes.Equal(built[:discovery.EthHdrLen+4], template[:discovery.EthHdrLen+4]) {
		t.Fatal("the rebuilt frame changed the template's Ethernet or PPPoE session header")
	}
	length := binary.BigEndian.Uint16(built[discovery.EthHdrLen+4 : pppSessionOverhead])
	if int(length) != len(built)-pppSessionOverhead {
		t.Fatalf("PPPoE Length %d, want %d", length, len(built)-pppSessionOverhead)
	}

	missing, err := rebuildIPv6CPConfigureRequest(template, 6, nil)
	if err != nil {
		t.Fatalf("rebuild without option: %v", err)
	}
	frame, _, err = parseOneIPv6CPFrame(missing)
	if err != nil {
		t.Fatalf("parse without option: %v", err)
	}
	if frame.hasOption {
		t.Fatal("nil options still produced an Interface-Identifier option")
	}
}

// TestZeAnswerIgnoresTheClientsOwnFrame proves the answer the checker judges
// is ze's: pppd's own Configure-Nak with the same one-byte Identifier, sent
// from the client's MAC, is not taken for ze's answer, and ze's is.
func TestZeAnswerIgnoresTheClientsOwnFrame(t *testing.T) {
	template := sessionFrame(t, testClientMAC, testZeMAC, ppp.LCPConfigureRequest, 9, interfaceIDOption([8]byte{1}))
	suggestion := [8]byte{0, 0, 0, 0, 0, 0, 0, 0x42}
	frames, err := parseIPv6CPFramesFromRaw(t,
		sessionFrame(t, testClientMAC, testZeMAC, ppp.LCPConfigureNak, 10, interfaceIDOption([8]byte{0xff})),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := zeAnswer(frames, template, 10, ppp.LCPConfigureNak); ok {
		t.Fatal("the client's own Configure-Nak was taken for ze's answer")
	}

	frames, err = parseIPv6CPFramesFromRaw(t,
		sessionFrame(t, testClientMAC, testZeMAC, ppp.LCPConfigureNak, 10, interfaceIDOption([8]byte{0xff})),
		sessionFrame(t, testZeMAC, testClientMAC, ppp.LCPConfigureNak, 10, interfaceIDOption(suggestion)),
	)
	if err != nil {
		t.Fatal(err)
	}
	nak, ok := zeAnswer(frames, template, 10, ppp.LCPConfigureNak)
	if !ok {
		t.Fatal("ze's Configure-Nak was not found")
	}
	if nak.interfaceID != suggestion {
		t.Fatalf("suggestion %x, want %x", nak.interfaceID, suggestion)
	}
	if _, ok := zeAnswer(frames, template, 11, ppp.LCPConfigureNak); ok {
		t.Fatal("an answer to another Identifier was accepted")
	}
	if _, ok := zeAnswer(frames, template, 10, ppp.LCPConfigureAck); ok {
		t.Fatal("a Configure-Nak was taken for a Configure-Ack")
	}
}

func parseIPv6CPFramesFromRaw(t *testing.T, raws ...[]byte) ([]ipv6cpFrame, error) {
	t.Helper()
	frames := make([]ipv6cpFrame, 0, len(raws))
	for _, raw := range raws {
		frame, ok, err := parseOneIPv6CPFrame(raw)
		if err != nil {
			return nil, err
		}
		if !ok {
			t.Fatalf("frame did not parse as IPv6CP: %x", raw)
		}
		frames = append(frames, frame)
	}
	return frames, nil
}
