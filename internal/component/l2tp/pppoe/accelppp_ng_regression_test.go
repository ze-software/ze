// Design: docs/architecture/wire/l2tp.md -- PPPoE discovery under hostile input
// Related: discovery.go -- ParseDiscovery, parseTags and the Builder under test
// Related: server.go -- handlePADI and handlePADR, the admission path under test

package pppoe

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// The tests in this file replay the malformed inputs behind accel-ppp-ng's
// PPPoE security fixes against Ze's discovery code. Each test names the
// accel-ppp-ng commit its input comes from, and pairs each refusal with a
// positive control so a server that answers nothing fails too.

// ngDiscFrame builds a raw discovery frame from src with the given VER/TYPE
// octet, code and payload octets; the PPPoE LENGTH field covers the payload.
func ngDiscFrame(src [EthALen]byte, verType, code byte, payload []byte) []byte {
	buf := make([]byte, MinDiscFrame+len(payload))
	copy(buf, BroadcastMAC)
	copy(buf[EthALen:], src[:])
	binary.BigEndian.PutUint16(buf[2*EthALen:], EthPPPDisc)
	buf[EthHdrLen] = verType
	buf[EthHdrLen+1] = code
	binary.BigEndian.PutUint16(buf[EthHdrLen+4:], uint16(len(payload)))
	copy(buf[MinDiscFrame:], payload)
	return buf
}

// ngTag encodes one TLV tag.
func ngTag(tagType uint16, value []byte) []byte {
	b := make([]byte, TagHdrLen+len(value))
	binary.BigEndian.PutUint16(b, tagType)
	binary.BigEndian.PutUint16(b[2:], uint16(len(value)))
	copy(b[TagHdrLen:], value)
	return b
}

var ngHost = [EthALen]byte{0x02, 0x00, 0x5E, 0x00, 0x00, 0x01}

// VALIDATES: a PADI whose Host-Uniq is too large to echo inside one Ethernet
// frame draws no PADO at all, never a truncated one, and a normal Host-Uniq is
// echoed unmodified.
//
// Source: accel-ppp-ng c93b367 and 8305387 (T8825), "Fix buffer overflow caused
// by oversized Host-Uniq tag": add_tag2 copied the tag into the PADO without
// checking the room left.
//
// RFC 2516 Appendix A, Host-Uniq: "If the Access Concentrator receives this
// TAG, it MUST include the TAG unmodified in the associated PADO or PADS
// response." A PADO that cannot carry it whole is not sent (Builder.Finish
// answers nil).
func TestNGOversizedHostUniqDrawsNoPADO(t *testing.T) {
	maxPayload := EthMaxLen - MinDiscFrame
	service := ngTag(TagServiceName, nil)
	huge := ngTag(TagHostUniq, make([]byte, maxPayload-len(service)-TagHdrLen))
	frame := ngDiscFrame(ngHost, PPPoEVerType, CodePADI, append(append([]byte(nil), service...), huge...))
	if len(frame) != EthMaxLen {
		t.Fatalf("test frame is %d octets, want a full %d-octet Ethernet frame", len(frame), EthMaxLen)
	}
	pkt, err := ParseDiscovery(frame)
	if err != nil {
		t.Fatalf("ParseDiscovery of a full-size PADI: %v", err)
	}

	var sent [][]byte
	s := newRecordingServer(&sent, nil, 8)
	s.acName = "ze-ac"
	s.handlePADI(&pkt)
	if len(sent) != 0 {
		t.Fatalf("PADI with a %d-octet Host-Uniq drew %d frame(s), want none", len(huge)-TagHdrLen, len(sent))
	}

	// Positive control: a 16-octet Host-Uniq draws one PADO that echoes it.
	hostUniq := bytes.Repeat([]byte{0x5A}, 16)
	small, err := ParseDiscovery(ngDiscFrame(ngHost, PPPoEVerType, CodePADI,
		append(append([]byte(nil), service...), ngTag(TagHostUniq, hostUniq)...)))
	if err != nil {
		t.Fatalf("ParseDiscovery of a small PADI: %v", err)
	}
	s.handlePADI(&small)
	if len(sent) != 1 {
		t.Fatalf("small PADI drew %d frame(s), want one PADO", len(sent))
	}
	pado := parseSent(t, sent[0])
	if pado.Code != CodePADO {
		t.Fatalf("reply code = 0x%02x, want PADO", pado.Code)
	}
	if echoed := pado.FindTag(TagHostUniq); echoed == nil || !bytes.Equal(echoed.Value, hostUniq) {
		t.Fatalf("PADO Host-Uniq = %v, want %x unmodified", echoed, hostUniq)
	}
	if len(sent[0]) > EthMaxLen {
		t.Fatalf("PADO is %d octets, over the %d-octet Ethernet frame", len(sent[0]), EthMaxLen)
	}
}

// VALIDATES: a tag whose TAG_LENGTH runs past the PPPoE payload is refused,
// and 1 to 3 octets left over after the last whole tag never become a tag.
//
// Source: accel-ppp-ng 93163c8 (T9238), "pppoe: check for a truncated tag
// header in PADI": pppoe_recv_PADI read a tag header past the payload.
//
// Ze reads a tag header only when four octets remain (parseTags), so the
// fragment is not read. accel-ppp-ng drops such a PADI; Ze parses the whole
// tags before the fragment and ignores the fragment.
func TestNGTruncatedTagHeaderNeverRead(t *testing.T) {
	service := ngTag(TagServiceName, []byte("internet"))

	overrun := append([]byte(nil), service...)
	overrun = append(overrun, 0x01, 0x03, 0x00, 0x40, 0xAA) // Host-Uniq claiming 64 octets, 1 present
	if _, err := ParseDiscovery(ngDiscFrame(ngHost, PPPoEVerType, CodePADI, overrun)); !errors.Is(err, ErrInvalidTagLength) {
		t.Fatalf("tag running past the payload: error %v, want ErrInvalidTagLength", err)
	}

	for fragment := 1; fragment < TagHdrLen; fragment++ {
		payload := append(append([]byte(nil), service...), bytes.Repeat([]byte{0x01}, fragment)...)
		pkt, err := ParseDiscovery(ngDiscFrame(ngHost, PPPoEVerType, CodePADI, payload))
		if err != nil {
			t.Fatalf("%d-octet fragment: %v", fragment, err)
		}
		if len(pkt.Tags) != 1 || pkt.Tags[0].Type != TagServiceName || string(pkt.Tags[0].Value) != "internet" {
			t.Fatalf("%d-octet fragment: tags %+v, want only Service-Name \"internet\"", fragment, pkt.Tags)
		}
	}
}

// VALIDATES: a discovery frame whose VER or TYPE is not 1 is refused with
// ErrBadVersion; VER=1, TYPE=1 parses.
//
// Source: accel-ppp-ng e042eab (T9238), "drop packets with unsupported PPPoE
// type": the unsupported-type branch fell through to processing.
//
// RFC 2516 Section 4: "The VER field is four bits and MUST be set to 0x1 for
// this version of the PPPoE specification." and "The TYPE field is four bits
// and MUST be set to 0x1 for this version of the PPPoE specification."
func TestNGUnsupportedVerTypeRefused(t *testing.T) {
	payload := ngTag(TagServiceName, nil)
	for _, vt := range []byte{0x12, 0x21, 0x22, 0x10, 0x01, 0x00, 0xFF} {
		if _, err := ParseDiscovery(ngDiscFrame(ngHost, vt, CodePADI, payload)); !errors.Is(err, ErrBadVersion) {
			t.Errorf("VER/TYPE 0x%02x: error %v, want ErrBadVersion", vt, err)
		}
	}
	if _, err := ParseDiscovery(ngDiscFrame(ngHost, PPPoEVerType, CodePADI, payload)); err != nil {
		t.Fatalf("VER/TYPE 0x11: %v", err)
	}
}

// VALIDATES: a PADR with no Service-Name tag is judged only after its
// AC-Cookie. With no valid cookie it draws nothing; with a valid cookie it
// draws one PADS with SESSION_ID 0 and Service-Name-Error, and no session.
//
// Source: accel-ppp-ng ef7323f and 01baa01 (T8464), "PADR packet with missing
// service-name tag bypasses cookie check": the Service-Name refusal ran
// before the cookie check, so a spoofed PADR drew a PADS.
//
// RFC 2516 Section 5.4: "If the Access Concentrator does not like the
// Service-Name in the PADR, then it MUST reply with a PADS containing a TAG
// of TAG_TYPE Service-Name-Error (and any number of other TAG types). In this
// case the SESSION_ID MUST be set to 0x0000."
func TestNGPADRWithoutServiceNameChecksCookieFirst(t *testing.T) {
	var sent [][]byte
	s := newRecordingServer(&sent, nil, 8)
	s.acName = "ze-ac"

	noCookie := &Packet{Code: CodePADR, DstMAC: s.hwAddr, SrcMAC: ngHost}
	badCookie := &Packet{Code: CodePADR, DstMAC: s.hwAddr, SrcMAC: ngHost,
		Tags: []Tag{{Type: TagACCookie, Value: bytes.Repeat([]byte{0xEE}, 24)}}}
	for name, pkt := range map[string]*Packet{"no cookie": noCookie, "forged cookie": badCookie} {
		s.handlePADR(pkt)
		if len(sent) != 0 {
			t.Fatalf("%s, no Service-Name: PADR drew %d frame(s), want none", name, len(sent))
		}
	}

	cookie := GenerateCookie(s.cookieKey, s.hwAddr[:], ngHost[:], nil)
	s.handlePADR(&Packet{Code: CodePADR, DstMAC: s.hwAddr, SrcMAC: ngHost,
		Tags: []Tag{{Type: TagACCookie, Value: cookie}}})
	if len(sent) != 1 {
		t.Fatalf("valid cookie, no Service-Name: PADR drew %d frame(s), want one PADS", len(sent))
	}
	pads := parseSent(t, sent[0])
	if pads.Code != CodePADS || pads.SID != 0 {
		t.Fatalf("reply code 0x%02x SID 0x%04x, want PADS with SESSION_ID 0", pads.Code, pads.SID)
	}
	if pads.FindTag(TagSvcNameError) == nil {
		t.Fatal("refusal PADS carries no Service-Name-Error tag")
	}
	if held := s.sessions.sessionsByMAC(ngHost[:]); len(held) != 0 {
		t.Fatalf("table holds %d session(s) for a refused PADR, want 0", len(held))
	}
}

// VALIDATES: zero-length tags advance the walk by their header, so a payload
// made only of them ends with at most maxTags tags.
//
// Source: accel-ppp-ng 2fa6475 (T8550), "Zero length packet leading to
// infinite CPU loop" in SSTP; the same length-driven loop shape lives in
// parseTags.
func TestNGZeroLengthTagsTerminate(t *testing.T) {
	payload := bytes.Repeat(ngTag(TagVendorSpecific, nil), (EthMaxLen-MinDiscFrame)/TagHdrLen)
	pkt, err := ParseDiscovery(ngDiscFrame(ngHost, PPPoEVerType, CodePADI, payload))
	if err != nil {
		t.Fatalf("ParseDiscovery: %v", err)
	}
	if len(pkt.Tags) != maxTags {
		t.Fatalf("tags parsed = %d, want the cap %d", len(pkt.Tags), maxTags)
	}
}
