package pppoe_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp/pppoe"
)

// buildDiscFrame builds a raw discovery frame for testing. Session ID
// is always zero (discovery frames before PADS).
func buildDiscFrame(dst, src [pppoe.EthALen]byte, code byte, tags []pppoe.Tag) []byte {
	buf := make([]byte, pppoe.EthMaxLen)
	copy(buf[0:], dst[:])
	copy(buf[pppoe.EthALen:], src[:])
	binary.BigEndian.PutUint16(buf[2*pppoe.EthALen:], pppoe.EthPPPDisc)

	buf[pppoe.EthHdrLen] = pppoe.PPPoEVerType
	buf[pppoe.EthHdrLen+1] = code
	binary.BigEndian.PutUint16(buf[pppoe.EthHdrLen+2:], 0)

	off := pppoe.EthHdrLen + pppoe.PPPoEHdrLen
	for _, t := range tags {
		binary.BigEndian.PutUint16(buf[off:], t.Type)
		binary.BigEndian.PutUint16(buf[off+2:], uint16(len(t.Value)))
		if len(t.Value) > 0 {
			copy(buf[off+4:], t.Value)
		}
		off += 4 + len(t.Value)
	}

	payloadLen := off - pppoe.EthHdrLen - pppoe.PPPoEHdrLen
	binary.BigEndian.PutUint16(buf[pppoe.EthHdrLen+4:], uint16(payloadLen))

	return buf[:off]
}

var (
	discClientMAC = [pppoe.EthALen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	discACMAC     = [pppoe.EthALen]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	discBcastMAC  = [pppoe.EthALen]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
)

// RFC requirement: RFC2516-x-3 positive -- a frame whose ver/type octet is 0x11 (VER=1, TYPE=1) parses successfully instead of being discarded.
// RFC requirement: RFC2516-x-7 positive -- a discovery frame with a unicast source MAC is accepted.
func TestParsePADI(t *testing.T) {
	hostUniq := []byte{0xDE, 0xAD}
	frame := buildDiscFrame(discBcastMAC, discClientMAC, pppoe.CodePADI, []pppoe.Tag{
		{Type: pppoe.TagServiceName, Value: []byte("internet")},
		{Type: pppoe.TagHostUniq, Value: hostUniq},
	})

	pkt, err := pppoe.ParseDiscovery(frame)
	if err != nil {
		t.Fatalf("ParseDiscovery: %v", err)
	}
	if pkt.Code != pppoe.CodePADI {
		t.Errorf("Code = 0x%02x, want 0x%02x", pkt.Code, pppoe.CodePADI)
	}
	if pkt.SID != 0 {
		t.Errorf("SID = %d, want 0", pkt.SID)
	}
	if pkt.SrcMAC != discClientMAC {
		t.Errorf("SrcMAC = %v, want %v", pkt.SrcMAC, discClientMAC)
	}

	svc := pkt.FindTag(pppoe.TagServiceName)
	if svc == nil {
		t.Fatal("Service-Name tag not found")
	}
	if string(svc.Value) != "internet" {
		t.Errorf("Service-Name = %q, want %q", svc.Value, "internet")
	}

	hu := pkt.FindTag(pppoe.TagHostUniq)
	if hu == nil {
		t.Fatal("Host-Uniq tag not found")
	}
	if len(hu.Value) != 2 || hu.Value[0] != 0xDE || hu.Value[1] != 0xAD {
		t.Errorf("Host-Uniq = %x, want DEAD", hu.Value)
	}
}

func TestParsePADI_EmptyServiceName(t *testing.T) {
	frame := buildDiscFrame(discBcastMAC, discClientMAC, pppoe.CodePADI, []pppoe.Tag{
		{Type: pppoe.TagServiceName, Value: nil},
	})

	pkt, err := pppoe.ParseDiscovery(frame)
	if err != nil {
		t.Fatalf("ParseDiscovery: %v", err)
	}
	if pkt.ServiceNameString() != "" {
		t.Errorf("ServiceNameString = %q, want empty", pkt.ServiceNameString())
	}
}

func TestParsePADR(t *testing.T) {
	cookie := []byte("test-cookie-value-32bytes-long!!")
	frame := buildDiscFrame(discACMAC, discClientMAC, pppoe.CodePADR, []pppoe.Tag{
		{Type: pppoe.TagServiceName, Value: []byte("internet")},
		{Type: pppoe.TagACCookie, Value: cookie},
		{Type: pppoe.TagHostUniq, Value: []byte{0x01}},
	})

	pkt, err := pppoe.ParseDiscovery(frame)
	if err != nil {
		t.Fatalf("ParseDiscovery: %v", err)
	}
	if pkt.Code != pppoe.CodePADR {
		t.Errorf("Code = 0x%02x, want 0x%02x", pkt.Code, pppoe.CodePADR)
	}

	ck := pkt.FindTag(pppoe.TagACCookie)
	if ck == nil {
		t.Fatal("AC-Cookie tag not found")
	}
	if !bytes.Equal(ck.Value, cookie) {
		t.Errorf("AC-Cookie mismatch")
	}
}

// RFC requirement: RFC2516-5.2-1 positive -- BuildPADO echoes the Host-Uniq from the PADI unchanged in the PADO.
// RFC requirement: RFC2516-5.2-3 positive -- the PADO carries the Host-Uniq that was present in the PADI.
func TestBuildPADO(t *testing.T) {
	padi := buildDiscFrame(discBcastMAC, discClientMAC, pppoe.CodePADI, []pppoe.Tag{
		{Type: pppoe.TagServiceName, Value: []byte("internet")},
		{Type: pppoe.TagHostUniq, Value: []byte{0x42}},
	})

	padiPkt, err := pppoe.ParseDiscovery(padi)
	if err != nil {
		t.Fatalf("parse PADI: %v", err)
	}

	cookie := []byte("cookie-data")
	var buf [pppoe.EthMaxLen]byte
	frame := pppoe.BuildPADO(buf[:], discACMAC, &padiPkt, "ze-ac", []string{"internet"}, cookie)
	if frame == nil {
		t.Fatal("BuildPADO returned nil")
	}

	pkt, err := pppoe.ParseDiscovery(frame)
	if err != nil {
		t.Fatalf("parse PADO: %v", err)
	}
	if pkt.Code != pppoe.CodePADO {
		t.Errorf("Code = 0x%02x, want 0x%02x", pkt.Code, pppoe.CodePADO)
	}
	if pkt.DstMAC != discClientMAC {
		t.Errorf("DstMAC = %v, want %v", pkt.DstMAC, discClientMAC)
	}
	if pkt.SrcMAC != discACMAC {
		t.Errorf("SrcMAC = %v, want %v", pkt.SrcMAC, discACMAC)
	}

	acNameTag := pkt.FindTag(pppoe.TagACName)
	if acNameTag == nil || string(acNameTag.Value) != "ze-ac" {
		t.Errorf("AC-Name = %v, want %q", acNameTag, "ze-ac")
	}

	cookieTag := pkt.FindTag(pppoe.TagACCookie)
	if cookieTag == nil || string(cookieTag.Value) != "cookie-data" {
		t.Errorf("AC-Cookie mismatch")
	}

	huTag := pkt.FindTag(pppoe.TagHostUniq)
	if huTag == nil || len(huTag.Value) != 1 || huTag.Value[0] != 0x42 {
		t.Errorf("Host-Uniq not echoed correctly: %v", huTag)
	}
}

// RFC requirement: RFC2516-5.2-1 positive -- BuildPADS echoes the Host-Uniq from the PADR unchanged in the PADS.
// RFC requirement: RFC2516-5.4-1 positive -- the PADS carries exactly one Service-Name tag echoed from the PADR.
func TestBuildPADS(t *testing.T) {
	padr := buildDiscFrame(discACMAC, discClientMAC, pppoe.CodePADR, []pppoe.Tag{
		{Type: pppoe.TagServiceName, Value: []byte("internet")},
		{Type: pppoe.TagHostUniq, Value: []byte{0x01, 0x02}},
	})

	padrPkt, err := pppoe.ParseDiscovery(padr)
	if err != nil {
		t.Fatalf("parse PADR: %v", err)
	}

	var buf [pppoe.EthMaxLen]byte
	frame := pppoe.BuildPADS(buf[:], discACMAC, &padrPkt, "ze-ac", 42)
	if frame == nil {
		t.Fatal("BuildPADS returned nil")
	}

	pkt, err := pppoe.ParseDiscovery(frame)
	if err != nil {
		t.Fatalf("parse PADS: %v", err)
	}
	if pkt.Code != pppoe.CodePADS {
		t.Errorf("Code = 0x%02x, want 0x%02x", pkt.Code, pppoe.CodePADS)
	}
	if pkt.SID != 42 {
		t.Errorf("SID = %d, want 42", pkt.SID)
	}

	svcTag := pkt.FindTag(pppoe.TagServiceName)
	if svcTag == nil || string(svcTag.Value) != "internet" {
		t.Errorf("Service-Name not echoed")
	}
	if n := len(pkt.FindAllTags(pppoe.TagServiceName)); n != 1 {
		t.Errorf("PADS Service-Name tag count = %d, want exactly 1", n)
	}

	huTag := pkt.FindTag(pppoe.TagHostUniq)
	if huTag == nil || len(huTag.Value) != 2 {
		t.Errorf("Host-Uniq not echoed")
	}
}

// RFC requirement: RFC2516-x-6 positive -- BuildPADT addresses the PADT to the peer's unicast MAC (the destination is unicast, never broadcast).
func TestBuildPADT(t *testing.T) {
	var buf [pppoe.EthMaxLen]byte
	frame := pppoe.BuildPADT(buf[:], discACMAC, discClientMAC, 100, "ze-ac")
	if frame == nil {
		t.Fatal("BuildPADT returned nil")
	}

	pkt, err := pppoe.ParseDiscovery(frame)
	if err != nil {
		t.Fatalf("parse PADT: %v", err)
	}
	if pkt.Code != pppoe.CodePADT {
		t.Errorf("Code = 0x%02x, want 0x%02x", pkt.Code, pppoe.CodePADT)
	}
	if pkt.SID != 100 {
		t.Errorf("SID = %d, want 100", pkt.SID)
	}
	if pkt.DstMAC != discClientMAC {
		t.Errorf("DstMAC = %v, want %v", pkt.DstMAC, discClientMAC)
	}
}

// RFC requirement: RFC2516-5.2-4 positive -- when the requested Service-Name is served, MatchServiceName returns true, so handlePADI (server.go:62) proceeds to send a PADO.
// RFC requirement: RFC2516-5.2-4 negative -- when the requested Service-Name cannot be served, MatchServiceName returns false, so handlePADI (server.go:62) returns without sending a PADO.
func TestServiceNameFilter(t *testing.T) {
	tests := []struct {
		name    string
		svcName []byte
		allowed []string
		want    bool
	}{
		{"empty allows accepts any", []byte("anything"), nil, true},
		{"empty tag matches any configured", nil, []string{"internet"}, true},
		{"exact match", []byte("internet"), []string{"internet", "voip"}, true},
		{"no match", []byte("gaming"), []string{"internet", "voip"}, false},
		{"no service-name tag", nil, []string{"internet"}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var tags []pppoe.Tag
			if tc.svcName != nil || tc.name == "empty tag matches any configured" || tc.name == "no service-name tag" {
				tags = append(tags, pppoe.Tag{Type: pppoe.TagServiceName, Value: tc.svcName})
			}

			frame := buildDiscFrame(discBcastMAC, discClientMAC, pppoe.CodePADI, tags)
			pkt, err := pppoe.ParseDiscovery(frame)
			if err != nil {
				t.Fatalf("ParseDiscovery: %v", err)
			}

			got := pppoe.MatchServiceName(&pkt, tc.allowed)
			if got != tc.want {
				t.Errorf("MatchServiceName = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestServiceNameFilter_NoTag(t *testing.T) {
	frame := buildDiscFrame(discBcastMAC, discClientMAC, pppoe.CodePADI, nil)
	pkt, err := pppoe.ParseDiscovery(frame)
	if err != nil {
		t.Fatalf("ParseDiscovery: %v", err)
	}

	if pppoe.MatchServiceName(&pkt, []string{"internet"}) {
		t.Error("expected false when no Service-Name tag present and allowedNames non-empty")
	}

	if !pppoe.MatchServiceName(&pkt, nil) {
		t.Error("expected true when allowedNames is empty")
	}
}

func TestParseShortPacket(t *testing.T) {
	_, err := pppoe.ParseDiscovery([]byte{0x00, 0x01})
	if !errors.Is(err, pppoe.ErrFrameTooShort) {
		t.Errorf("err = %v, want ErrFrameTooShort", err)
	}
}

// RFC requirement: RFC2516-x-1 negative -- a frame whose VER nibble is not 1 (ver/type 0x21) is rejected with ErrBadVersion, not parsed.
// RFC requirement: RFC2516-x-3 negative -- a frame with ver/type other than 0x11 is silently discarded (ParseDiscovery returns ErrBadVersion rather than a Packet).
func TestParseBadVersion(t *testing.T) {
	frame := buildDiscFrame(discBcastMAC, discClientMAC, pppoe.CodePADI, nil)
	frame[pppoe.EthHdrLen] = 0x21 // bad version

	_, err := pppoe.ParseDiscovery(frame)
	if !errors.Is(err, pppoe.ErrBadVersion) {
		t.Errorf("err = %v, want ErrBadVersion", err)
	}
}

func TestParseInvalidTagLength(t *testing.T) {
	frame := buildDiscFrame(discBcastMAC, discClientMAC, pppoe.CodePADI, []pppoe.Tag{
		{Type: pppoe.TagServiceName, Value: []byte("test")},
	})

	// Corrupt the tag length to extend past the payload.
	tagOff := pppoe.EthHdrLen + pppoe.PPPoEHdrLen
	binary.BigEndian.PutUint16(frame[tagOff+2:], 999)

	_, err := pppoe.ParseDiscovery(frame)
	if !errors.Is(err, pppoe.ErrInvalidTagLength) {
		t.Errorf("err = %v, want ErrInvalidTagLength", err)
	}
}

// RFC requirement: RFC2516-x-7 negative -- a frame whose source MAC is the broadcast address is rejected (ErrBroadcastSource), never treated as a valid packet.
func TestParseBroadcastSource(t *testing.T) {
	frame := buildDiscFrame(discBcastMAC, discBcastMAC, pppoe.CodePADI, nil)

	_, err := pppoe.ParseDiscovery(frame)
	if !errors.Is(err, pppoe.ErrBroadcastSource) {
		t.Errorf("err = %v, want ErrBroadcastSource", err)
	}
}

// RFC requirement: RFC2516-x-7 negative -- a frame whose source MAC is a multicast address is rejected (ErrMulticastSource), never treated as a valid packet.
func TestParseMulticastSource(t *testing.T) {
	mcastMAC := [pppoe.EthALen]byte{0x01, 0x00, 0x5E, 0x00, 0x00, 0x01}
	frame := buildDiscFrame(discBcastMAC, mcastMAC, pppoe.CodePADI, nil)

	_, err := pppoe.ParseDiscovery(frame)
	if !errors.Is(err, pppoe.ErrMulticastSource) {
		t.Errorf("err = %v, want ErrMulticastSource", err)
	}
}

func TestParseNoTags(t *testing.T) {
	frame := buildDiscFrame(discBcastMAC, discClientMAC, pppoe.CodePADI, nil)

	pkt, err := pppoe.ParseDiscovery(frame)
	if err != nil {
		t.Fatalf("ParseDiscovery: %v", err)
	}
	if len(pkt.Tags) != 0 {
		t.Errorf("Tags = %v, want empty", pkt.Tags)
	}
}

// RFC requirement: RFC2516-x-4 positive -- a zero-length End-Of-List tag terminates tag parsing; tags after it are not returned.
func TestParseEndOfListTag(t *testing.T) {
	frame := buildDiscFrame(discBcastMAC, discClientMAC, pppoe.CodePADI, []pppoe.Tag{
		{Type: pppoe.TagServiceName, Value: []byte("svc")},
		{Type: pppoe.TagEndOfList, Value: nil},
		{Type: pppoe.TagHostUniq, Value: []byte{0xFF}},
	})

	pkt, err := pppoe.ParseDiscovery(frame)
	if err != nil {
		t.Fatalf("ParseDiscovery: %v", err)
	}
	if len(pkt.Tags) != 1 {
		t.Errorf("got %d tags, want 1 (end-of-list should stop parsing)", len(pkt.Tags))
	}
}

func TestBuildPADSError(t *testing.T) {
	padr := buildDiscFrame(discACMAC, discClientMAC, pppoe.CodePADR, []pppoe.Tag{
		{Type: pppoe.TagServiceName, Value: []byte("unknown")},
	})

	padrPkt, err := pppoe.ParseDiscovery(padr)
	if err != nil {
		t.Fatalf("parse PADR: %v", err)
	}

	var buf [pppoe.EthMaxLen]byte
	frame := pppoe.BuildPADSError(buf[:], discACMAC, &padrPkt, "ze-ac", pppoe.TagSvcNameError)
	if frame == nil {
		t.Fatal("BuildPADSError returned nil")
	}

	pkt, err := pppoe.ParseDiscovery(frame)
	if err != nil {
		t.Fatalf("parse error PADS: %v", err)
	}
	if pkt.SID != 0 {
		t.Errorf("SID = %d, want 0 for error PADS", pkt.SID)
	}
	if pkt.FindTag(pppoe.TagSvcNameError) == nil {
		t.Error("Service-Name-Error tag not found")
	}
}

func TestFindAllTags(t *testing.T) {
	frame := buildDiscFrame(discBcastMAC, discClientMAC, pppoe.CodePADI, []pppoe.Tag{
		{Type: pppoe.TagServiceName, Value: []byte("svc1")},
		{Type: pppoe.TagServiceName, Value: []byte("svc2")},
		{Type: pppoe.TagHostUniq, Value: []byte{0x01}},
	})

	pkt, err := pppoe.ParseDiscovery(frame)
	if err != nil {
		t.Fatalf("ParseDiscovery: %v", err)
	}

	svcs := pkt.FindAllTags(pppoe.TagServiceName)
	if len(svcs) != 2 {
		t.Errorf("got %d Service-Name tags, want 2", len(svcs))
	}
}

func TestPayloadLengthTruncated(t *testing.T) {
	frame := buildDiscFrame(discBcastMAC, discClientMAC, pppoe.CodePADI, []pppoe.Tag{
		{Type: pppoe.TagServiceName, Value: []byte("test")},
	})

	// Set payload length to exceed actual frame data.
	binary.BigEndian.PutUint16(frame[pppoe.EthHdrLen+4:], 500)

	_, err := pppoe.ParseDiscovery(frame)
	if !errors.Is(err, pppoe.ErrFrameTooShort) {
		t.Errorf("err = %v, want ErrFrameTooShort", err)
	}
}

func TestConstants(t *testing.T) {
	if pppoe.PPPoEMaxPayload != 1494 {
		t.Errorf("PPPoEMaxPayload = %d, want 1494", pppoe.PPPoEMaxPayload)
	}
	if pppoe.PPPoEMaxMTU != 1492 {
		t.Errorf("PPPoEMaxMTU = %d, want 1492", pppoe.PPPoEMaxMTU)
	}
	if pppoe.MinDiscFrame != 20 {
		t.Errorf("MinDiscFrame = %d, want 20", pppoe.MinDiscFrame)
	}
}

// RFC requirement: RFC2516-x-1 positive -- a built discovery frame carries VER=1 in the high nibble of the ver/type octet.
// RFC requirement: RFC2516-x-2 positive -- a built discovery frame carries TYPE=1 in the low nibble of the ver/type octet.
func TestBuildFrameVerType(t *testing.T) {
	var buf [pppoe.EthMaxLen]byte
	frame := pppoe.BuildPADI(buf[:], discClientMAC, "internet", []byte{0x01, 0x02})
	if frame == nil {
		t.Fatal("BuildPADI returned nil")
	}
	verType := frame[pppoe.EthHdrLen]
	if verType != pppoe.PPPoEVerType {
		t.Fatalf("ver/type octet = 0x%02x, want 0x11", verType)
	}
	if ver := verType >> 4; ver != 1 {
		t.Errorf("VER = %d, want 1", ver)
	}
	if typ := verType & 0x0F; typ != 1 {
		t.Errorf("TYPE = %d, want 1", typ)
	}
}

// RFC requirement: RFC2516-x-2 negative -- a frame whose TYPE nibble is not 1 (ver/type 0x12) is rejected with ErrBadVersion, not parsed.
func TestParseBadType(t *testing.T) {
	frame := buildDiscFrame(discBcastMAC, discClientMAC, pppoe.CodePADI, nil)
	frame[pppoe.EthHdrLen] = 0x12 // VER=1, TYPE=2

	_, err := pppoe.ParseDiscovery(frame)
	if !errors.Is(err, pppoe.ErrBadVersion) {
		t.Errorf("err = %v, want ErrBadVersion", err)
	}
}

// RFC requirement: RFC2516-5.1-1 positive -- BuildPADI writes exactly one Service-Name tag.
// RFC requirement: RFC2516-5.1-2 positive -- BuildPADI addresses the PADI to the Ethernet broadcast MAC.
func TestBuildPADIDiscovery(t *testing.T) {
	var buf [pppoe.EthMaxLen]byte
	frame := pppoe.BuildPADI(buf[:], discClientMAC, "internet", []byte{0xDE, 0xAD})
	if frame == nil {
		t.Fatal("BuildPADI returned nil")
	}
	pkt, err := pppoe.ParseDiscovery(frame)
	if err != nil {
		t.Fatalf("parse PADI: %v", err)
	}
	if pkt.Code != pppoe.CodePADI {
		t.Errorf("Code = 0x%02x, want PADI", pkt.Code)
	}
	if n := len(pkt.FindAllTags(pppoe.TagServiceName)); n != 1 {
		t.Errorf("PADI Service-Name tag count = %d, want exactly 1", n)
	}
	if pkt.DstMAC != discBcastMAC {
		t.Errorf("PADI DstMAC = %v, want broadcast %v", pkt.DstMAC, discBcastMAC)
	}
}

// RFC requirement: RFC2516-5.3-1 positive -- BuildPADR echoes the AC-Cookie from the PADO unchanged in the PADR.
// RFC requirement: RFC2516-5.3-3 positive -- the PADR carries the AC-Cookie that was in the selected PADO.
// RFC requirement: RFC2516-5.3-4 positive -- BuildPADR includes the Host-Uniq (the value the Host used in its PADI) in the PADR.
// RFC requirement: RFC2516-5.3-2 positive -- BuildPADR writes exactly one Service-Name tag.
func TestBuildPADREchoesTags(t *testing.T) {
	cookie := []byte("ac-cookie-1234567890")
	// PADO is sent BY the AC: source = AC MAC, destination = Host MAC.
	pado := buildDiscFrame(discClientMAC, discACMAC, pppoe.CodePADO, []pppoe.Tag{
		{Type: pppoe.TagACName, Value: []byte("ze-ac")},
		{Type: pppoe.TagServiceName, Value: []byte("internet")},
		{Type: pppoe.TagACCookie, Value: cookie},
	})
	padoPkt, err := pppoe.ParseDiscovery(pado)
	if err != nil {
		t.Fatalf("parse PADO: %v", err)
	}

	hostUniq := []byte{0x11, 0x22, 0x33, 0x44}
	var buf [pppoe.EthMaxLen]byte
	frame := pppoe.BuildPADR(buf[:], discClientMAC, &padoPkt, "internet", hostUniq)
	if frame == nil {
		t.Fatal("BuildPADR returned nil")
	}
	padr, err := pppoe.ParseDiscovery(frame)
	if err != nil {
		t.Fatalf("parse PADR: %v", err)
	}
	if padr.DstMAC != discACMAC {
		t.Errorf("PADR DstMAC = %v, want AC %v", padr.DstMAC, discACMAC)
	}
	ck := padr.FindTag(pppoe.TagACCookie)
	if ck == nil || !bytes.Equal(ck.Value, cookie) {
		t.Errorf("AC-Cookie = %v, want echoed %x", ck, cookie)
	}
	hu := padr.FindTag(pppoe.TagHostUniq)
	if hu == nil || !bytes.Equal(hu.Value, hostUniq) {
		t.Errorf("Host-Uniq = %v, want %x", hu, hostUniq)
	}
	if n := len(padr.FindAllTags(pppoe.TagServiceName)); n != 1 {
		t.Errorf("PADR Service-Name tag count = %d, want exactly 1", n)
	}
}

// RFC requirement: RFC2516-5.3-1 negative -- when the PADO carried no AC-Cookie, BuildPADR emits no AC-Cookie tag (it does not invent one).
// RFC requirement: RFC2516-5.3-3 negative -- a PADR built from a cookieless PADO carries no AC-Cookie tag.
// RFC requirement: RFC2516-5.3-4 negative -- with no Host-Uniq supplied, BuildPADR emits no Host-Uniq tag.
func TestBuildPADRNoOptionalTags(t *testing.T) {
	// PADO is sent BY the AC: source = AC MAC, destination = Host MAC.
	pado := buildDiscFrame(discClientMAC, discACMAC, pppoe.CodePADO, []pppoe.Tag{
		{Type: pppoe.TagACName, Value: []byte("ze-ac")},
		{Type: pppoe.TagServiceName, Value: []byte("internet")},
	})
	padoPkt, err := pppoe.ParseDiscovery(pado)
	if err != nil {
		t.Fatalf("parse PADO: %v", err)
	}

	var buf [pppoe.EthMaxLen]byte
	frame := pppoe.BuildPADR(buf[:], discClientMAC, &padoPkt, "internet", nil)
	if frame == nil {
		t.Fatal("BuildPADR returned nil")
	}
	padr, err := pppoe.ParseDiscovery(frame)
	if err != nil {
		t.Fatalf("parse PADR: %v", err)
	}
	if padr.FindTag(pppoe.TagACCookie) != nil {
		t.Error("PADR must not carry an AC-Cookie when the PADO had none")
	}
	if padr.FindTag(pppoe.TagHostUniq) != nil {
		t.Error("PADR must not carry a Host-Uniq when none was supplied")
	}
}

// RFC requirement: RFC2516-5.2-1 negative -- when the PADI/PADR carried no Host-Uniq, BuildPADO and BuildPADS emit no Host-Uniq tag.
// RFC requirement: RFC2516-5.2-3 negative -- a PADO built from a PADI without Host-Uniq carries no Host-Uniq tag.
func TestBuildNoHostUniqEcho(t *testing.T) {
	padi := buildDiscFrame(discBcastMAC, discClientMAC, pppoe.CodePADI, []pppoe.Tag{
		{Type: pppoe.TagServiceName, Value: []byte("internet")},
	})
	padiPkt, err := pppoe.ParseDiscovery(padi)
	if err != nil {
		t.Fatalf("parse PADI: %v", err)
	}
	var buf [pppoe.EthMaxLen]byte
	pado := pppoe.BuildPADO(buf[:], discACMAC, &padiPkt, "ze-ac", []string{"internet"}, []byte("cookie"))
	padoPkt, err := pppoe.ParseDiscovery(pado)
	if err != nil {
		t.Fatalf("parse PADO: %v", err)
	}
	if padoPkt.FindTag(pppoe.TagHostUniq) != nil {
		t.Error("PADO must not carry a Host-Uniq when the PADI had none")
	}

	padr := buildDiscFrame(discACMAC, discClientMAC, pppoe.CodePADR, []pppoe.Tag{
		{Type: pppoe.TagServiceName, Value: []byte("internet")},
	})
	padrPkt, err := pppoe.ParseDiscovery(padr)
	if err != nil {
		t.Fatalf("parse PADR: %v", err)
	}
	var buf2 [pppoe.EthMaxLen]byte
	pads := pppoe.BuildPADS(buf2[:], discACMAC, &padrPkt, "ze-ac", 7)
	padsPkt, err := pppoe.ParseDiscovery(pads)
	if err != nil {
		t.Fatalf("parse PADS: %v", err)
	}
	if padsPkt.FindTag(pppoe.TagHostUniq) != nil {
		t.Error("PADS must not carry a Host-Uniq when the PADR had none")
	}
}

// RFC requirement: RFC2516-x-5 positive -- BuildPADO, BuildPADR, and BuildPADS carry the Relay-Session-Id unchanged in each subsequent Discovery packet of the exchange.
func TestRelaySessionIDEcho(t *testing.T) {
	relayID := []byte{0xAB, 0xCD, 0xEF, 0x01}

	padi := buildDiscFrame(discBcastMAC, discClientMAC, pppoe.CodePADI, []pppoe.Tag{
		{Type: pppoe.TagServiceName, Value: []byte("internet")},
		{Type: pppoe.TagRelaySessionID, Value: relayID},
	})
	padiPkt, err := pppoe.ParseDiscovery(padi)
	if err != nil {
		t.Fatalf("parse PADI: %v", err)
	}
	var b1 [pppoe.EthMaxLen]byte
	pado := pppoe.BuildPADO(b1[:], discACMAC, &padiPkt, "ze-ac", []string{"internet"}, []byte("cookie"))
	padoPkt, err := pppoe.ParseDiscovery(pado)
	if err != nil {
		t.Fatalf("parse PADO: %v", err)
	}
	if tag := padoPkt.FindTag(pppoe.TagRelaySessionID); tag == nil || !bytes.Equal(tag.Value, relayID) {
		t.Errorf("PADO Relay-Session-Id = %v, want echoed %x", tag, relayID)
	}

	var b2 [pppoe.EthMaxLen]byte
	padr := pppoe.BuildPADR(b2[:], discClientMAC, &padoPkt, "internet", nil)
	padrPkt, err := pppoe.ParseDiscovery(padr)
	if err != nil {
		t.Fatalf("parse PADR: %v", err)
	}
	if tag := padrPkt.FindTag(pppoe.TagRelaySessionID); tag == nil || !bytes.Equal(tag.Value, relayID) {
		t.Errorf("PADR Relay-Session-Id = %v, want echoed %x", tag, relayID)
	}

	var b3 [pppoe.EthMaxLen]byte
	pads := pppoe.BuildPADS(b3[:], discACMAC, &padrPkt, "ze-ac", 9)
	padsPkt, err := pppoe.ParseDiscovery(pads)
	if err != nil {
		t.Fatalf("parse PADS: %v", err)
	}
	if tag := padsPkt.FindTag(pppoe.TagRelaySessionID); tag == nil || !bytes.Equal(tag.Value, relayID) {
		t.Errorf("PADS Relay-Session-Id = %v, want echoed %x", tag, relayID)
	}
}

// RFC requirement: RFC2516-x-5 negative -- when no Relay-Session-Id is present in the input, the built Discovery packet carries none (no spurious Relay-Session-Id is invented).
func TestRelaySessionIDNoEcho(t *testing.T) {
	padi := buildDiscFrame(discBcastMAC, discClientMAC, pppoe.CodePADI, []pppoe.Tag{
		{Type: pppoe.TagServiceName, Value: []byte("internet")},
	})
	padiPkt, err := pppoe.ParseDiscovery(padi)
	if err != nil {
		t.Fatalf("parse PADI: %v", err)
	}
	var buf [pppoe.EthMaxLen]byte
	pado := pppoe.BuildPADO(buf[:], discACMAC, &padiPkt, "ze-ac", []string{"internet"}, []byte("cookie"))
	padoPkt, err := pppoe.ParseDiscovery(pado)
	if err != nil {
		t.Fatalf("parse PADO: %v", err)
	}
	if padoPkt.FindTag(pppoe.TagRelaySessionID) != nil {
		t.Error("PADO must not carry a Relay-Session-Id when the PADI had none")
	}
}

// RFC requirement: RFC2516-x-8 positive -- a frame carrying an unknown tag type parses without error, the unknown tag is retained, and known tags remain findable.
func TestUnknownTagIgnored(t *testing.T) {
	const unknownTag uint16 = 0x9999
	frame := buildDiscFrame(discBcastMAC, discClientMAC, pppoe.CodePADI, []pppoe.Tag{
		{Type: unknownTag, Value: []byte{0xDE, 0xAD}},
		{Type: pppoe.TagServiceName, Value: []byte("internet")},
	})
	pkt, err := pppoe.ParseDiscovery(frame)
	if err != nil {
		t.Fatalf("ParseDiscovery must not error on an unknown tag: %v", err)
	}
	if svc := pkt.FindTag(pppoe.TagServiceName); svc == nil || string(svc.Value) != "internet" {
		t.Error("known Service-Name tag must still be found alongside an unknown tag")
	}
	if pkt.FindTag(unknownTag) == nil {
		t.Error("unknown tag should be retained, not dropped")
	}
}
