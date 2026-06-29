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

	huTag := pkt.FindTag(pppoe.TagHostUniq)
	if huTag == nil || len(huTag.Value) != 2 {
		t.Errorf("Host-Uniq not echoed")
	}
}

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

func TestParseBroadcastSource(t *testing.T) {
	frame := buildDiscFrame(discBcastMAC, discBcastMAC, pppoe.CodePADI, nil)

	_, err := pppoe.ParseDiscovery(frame)
	if !errors.Is(err, pppoe.ErrBroadcastSource) {
		t.Errorf("err = %v, want ErrBroadcastSource", err)
	}
}

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
