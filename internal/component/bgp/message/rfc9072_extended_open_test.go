// Related: open.go — Open.WriteTo, writeToExtended, UnpackOpen, ExtendedParams
//
// VALIDATES: the extended envelope and the parameters inside it agree, and the
// decoder selects the extended form from the octet RFC 9072 names.
// PREVENTS: an OPEN that declares RFC 9072 framing and carries RFC 4271
// one-octet parameter lengths, which a conforming peer misframes from the first
// parameter onward — and which ze could not detect because ze read back what ze
// wrote.
package message

import (
	"bytes"
	"testing"
)

// TestTheExtendedEnvelopeAndItsParametersAgree is the send-side half, and the
// interop defect this file exists for. An envelope declaring RFC 9072 framing
// around parameters carrying one-octet lengths is malformed to every conforming
// receiver, and ze produced exactly that.
//
// RFC requirement: RFC9072-2-1 negative -- the extended envelope is never
// written around parameters that are not in the extended framing (S2).
func TestTheExtendedEnvelopeAndItsParametersAgree(t *testing.T) {
	// One parameter, extended framing, 300 octets of capability payload.
	payload := make([]byte, 300)
	params := append([]byte{0x02, 0x01, 0x2c}, payload...)

	open := &Open{
		Version:        4,
		MyAS:           65001,
		HoldTime:       180,
		BGPIdentifier:  0x0a000001,
		OptionalParams: params,
		ExtendedParams: true,
	}

	buf := make([]byte, open.Len(nil))
	n := open.WriteTo(buf, 0, nil)
	if n != len(buf) {
		t.Fatalf("WriteTo wrote %d octets, Len promised %d", n, len(buf))
	}

	body := buf[HeaderLen:]
	if body[9] != ExtendedParamMarker || body[10] != ExtendedParamMarker {
		t.Fatalf("the extended markers were not written: % x", body[9:11])
	}

	// Round-trip: what a conforming receiver reads back must be what was packed.
	back, err := UnpackOpen(body)
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}
	if !back.ExtendedParams {
		t.Fatal("the round-trip lost the extended framing")
	}
	if !bytes.Equal(back.OptionalParams, params) {
		t.Fatalf("round-trip params differ:\n got % x\nwant % x", back.OptionalParams, params)
	}
}

// TestAStandardOpenStillUsesTheStandardEnvelope keeps the common path. Without
// it the fix could be read as "always go extended", which RFC 9072 Section 2
// discourages: standard encoding SHOULD be used below 256 octets.
func TestAStandardOpenStillUsesTheStandardEnvelope(t *testing.T) {
	params := []byte{0x02, 0x06, 0x41, 0x04, 0x00, 0x00, 0xfd, 0xe9}
	open := &Open{
		Version: 4, MyAS: 65001, HoldTime: 180, BGPIdentifier: 0x0a000001,
		OptionalParams: params,
	}

	buf := make([]byte, open.Len(nil))
	open.WriteTo(buf, 0, nil)
	body := buf[HeaderLen:]

	if body[9] != byte(len(params)) {
		t.Fatalf("Opt Param Len = %d, want %d: an OPEN under 256 octets keeps the "+
			"RFC 4271 envelope", body[9], len(params))
	}

	back, err := UnpackOpen(body)
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}
	if back.ExtendedParams {
		t.Error("a standard OPEN was read as extended")
	}
	if !bytes.Equal(back.OptionalParams, params) {
		t.Errorf("round-trip params % x, want % x", back.OptionalParams, params)
	}
}
