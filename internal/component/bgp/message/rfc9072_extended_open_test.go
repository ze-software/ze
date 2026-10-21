// Related: open.go — Open.WriteTo, writeToExtended, UnpackOpen, ExtendedParams
// Related: ../../../core/bgp/capability/optional_params.go — PackOptionalParams
//
// VALIDATES: the OPEN ze sends, its Optional Parameters packed by
// capability.PackOptionalParams and written by Open.WriteTo, never carries the
// RFC 9072 extended envelope around parameters framed with RFC 4271 one-octet
// Parameter Lengths, and the decoder selects the extended form from the octet
// RFC 9072 names.
// PREVENTS: an OPEN that declares RFC 9072 framing and carries RFC 4271
// one-octet parameter lengths, which a conforming peer misframes from the first
// parameter onward — and which ze could not detect because ze read back what ze
// wrote.
package message

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// TestTheExtendedEnvelopeAndItsParametersAgree is the send-side half, and the
// interop defect this file exists for. An envelope declaring RFC 9072 framing
// around parameters carrying one-octet lengths is malformed to every conforming
// receiver, and ze produced exactly that while the capability total was 254 or
// 255 octets.
//
// Method: each parameter list is packed by PackOptionalParams, the packer every
// OPEN ze sends goes through, and written by Open.WriteTo with the framing flag
// the packer returned. The written octets are then read as a conforming
// receiver reads them, never through the encoder's flag: when the 255/255
// envelope is present, the parameters are walked with two-octet Parameter
// Lengths and MUST tile the Extended Opt. Parm. Length exactly, each parameter
// carrying the type and value it was packed with. A list whose base form is 255
// octets keeps the RFC 4271 envelope; every larger list, including the 256 and
// 257 octets the off-by-two wrapped in the extended envelope, is walked that
// way.
//
// RFC requirement: RFC9072-2-1 negative -- the extended envelope is never
// written around parameters that are not in the extended framing (S2): for
// Optional Parameters whose base form exceeds 255 octets, the parameters inside
// the 255/255 envelope walk exactly with two-octet Parameter Lengths.
func TestTheExtendedEnvelopeAndItsParametersAgree(t *testing.T) {
	cases := []struct {
		name   string
		values []int // value octets of each type-2 parameter
		extend bool  // the base form would exceed 255 octets
	}{
		{"one parameter, base form 255 octets", []int{253}, false},
		{"one parameter, base form 256 octets", []int{254}, true},
		{"one parameter, base form 257 octets", []int{255}, true},
		{"one parameter, base form 302 octets", []int{300}, true},
		{"two parameters, base form 257 octets", []int{126, 127}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			values := make([][]byte, len(tc.values))
			params := make([]capability.OptionalParam, len(tc.values))
			for i, octets := range tc.values {
				values[i] = bytes.Repeat([]byte{byte(0xA0 + i)}, octets)
				params[i] = capability.RawParam(2, values[i])
			}
			packed, extended, err := capability.PackOptionalParams(params)
			if err != nil {
				t.Fatalf("PackOptionalParams: %v", err)
			}

			open := &Open{
				Version:        4,
				MyAS:           65001,
				HoldTime:       180,
				BGPIdentifier:  0x0a000001,
				OptionalParams: packed,
				ExtendedParams: extended,
			}
			buf := make([]byte, open.Len(nil))
			if n := open.WriteTo(buf, 0, nil); n != len(buf) {
				t.Fatalf("WriteTo wrote %d octets, Len promised %d", n, len(buf))
			}
			body := buf[HeaderLen:]

			envelope := body[9] == ExtendedParamMarker && body[10] == ExtendedParamMarker
			if envelope != tc.extend {
				t.Fatalf("255/255 envelope written = %v, want %v (body[9:11] % x)",
					envelope, tc.extend, body[9:11])
			}
			if !envelope {
				return
			}

			fieldOctets := int(binary.BigEndian.Uint16(body[11:13]))
			field := body[13:]
			if len(field) != fieldOctets {
				t.Fatalf("Extended Opt. Parm. Length %d, but %d octets follow it",
					fieldOctets, len(field))
			}
			requireTwoOctetFraming(t, field, values)

			// The decoder reads the same form back from the octets.
			back, err := UnpackOpen(body)
			if err != nil {
				t.Fatalf("unpack: %v", err)
			}
			if !back.ExtendedParams {
				t.Fatal("UnpackOpen did not select the extended form from the 255/255 octets")
			}
			if !bytes.Equal(back.OptionalParams, field) {
				t.Fatalf("UnpackOpen returned other parameters:\n got % x\nwant % x",
					back.OptionalParams, field)
			}
		})
	}
}

// requireTwoOctetFraming walks field as RFC 9072 Section 2 frames it, Type (1)
// then a two-octet Parameter Length, and fails unless it yields exactly the
// type-2 parameters whose values are want, in order, with no octet left over.
// One-octet parameters read this way give a length that runs into the value
// octets, so the walk fails on the first parameter.
func requireTwoOctetFraming(t *testing.T, field []byte, want [][]byte) {
	t.Helper()
	off := 0
	for i, value := range want {
		if len(field)-off < 3 {
			t.Fatalf("parameter %d: %d octets left, a two-octet header needs 3", i, len(field)-off)
		}
		if field[off] != 2 {
			t.Fatalf("parameter %d: Type %d, want 2", i, field[off])
		}
		length := int(binary.BigEndian.Uint16(field[off+1:]))
		if length != len(value) {
			t.Fatalf("parameter %d: two-octet Parameter Length %d, want %d: the "+
				"parameters inside the extended envelope are not in its framing",
				i, length, len(value))
		}
		off += 3
		if !bytes.Equal(field[off:off+length], value) {
			t.Fatalf("parameter %d: value differs from what was packed", i)
		}
		off += length
	}
	if off != len(field) {
		t.Fatalf("%d octets left after the last parameter", len(field)-off)
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
