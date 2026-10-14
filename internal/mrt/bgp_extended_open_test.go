// Design: docs/architecture/mrt.md -- BGP message parsing for offline MRT analysis
// Overview: bgp.go -- parseOpen, openOptionalParams, parseCapabilities

package mrt

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// extendedOpenTestBody is an OPEN body (no header) carrying params, framed by
// the one packer, with the RFC 9072 Section 2 envelope (255/255, two-octet
// Extended Opt. Parm. Length) when the packer chose the extended form. It is
// written here rather than by message.Open.WriteTo because mrt imports no
// component package.
func extendedOpenTestBody(t *testing.T, params []capability.OptionalParam) ([]byte, bool) {
	t.Helper()
	optional, extended, err := capability.PackOptionalParams(params)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	body := []byte{4, 0x5b, 0xa0, 0, 90, 1, 2, 3, 4} // version 4, AS 23456, hold 90, id 1.2.3.4
	if extended {
		body = append(body, 255, 255)
		body = binary.BigEndian.AppendUint16(body, uint16(len(optional)))
	} else {
		body = append(body, byte(len(optional)))
	}
	return append(body, optional...), extended
}

// TestParseOpenReadsExtendedOpen proves the MRT decoder reads a recorded OPEN's
// capabilities under both framings: the four-octet AS capability sets the
// peer's AS from an OPEN in the RFC 4271 form and from one in the RFC 9072
// Section 2 form (several type-2 parameters, two-octet Parameter Lengths), and
// an extended parameter that runs past the field is an error.
//
// VALIDATES: spec-bgp-open-session-rfc-defects D1, the MRT received-OPEN reader:
// parseOpen reads the RFC 9072 envelope and parseCapabilities walks with
// capability.WalkOptionalParams and that framing.
// PREVENTS: the one-octet length at body[9] reading an extended OPEN as 255
// octets of one-octet parameters, so `ze mrt` shows AS 23456 and no capabilities.
func TestParseOpenReadsExtendedOpen(t *testing.T) {
	for _, tc := range []struct {
		name   string
		filler int
	}{{"base form", 10}, {"RFC 9072 form", 200}} {
		t.Run(tc.name, func(t *testing.T) {
			params := []capability.OptionalParam{capability.RawParam(2, []byte{65, 4, 0xfa, 0x56, 0xea, 0x01})}
			for range 2 {
				params = append(params, capability.RawParam(2, append([]byte{0xee, byte(tc.filler)}, make([]byte, tc.filler)...)))
			}
			body, extended := extendedOpenTestBody(t, params)
			if extended != (tc.filler == 200) {
				t.Fatalf("the test OPEN's framing is not the case's (extended=%v)", extended)
			}

			open, err := parseOpen(body)
			if err != nil {
				t.Fatalf("parseOpen: %v", err)
			}
			if open.ASN != 4200000001 {
				t.Errorf("ASN %d, want 4200000001 from the four-octet AS capability", open.ASN)
			}
			if len(open.Caps) != 3 {
				t.Fatalf("capabilities %d, want three", len(open.Caps))
			}
			for _, c := range open.Caps[1:] {
				if c.Code != 0xee {
					t.Errorf("filler code %d, want 238", c.Code)
				}
				if len(c.Value) != tc.filler {
					t.Errorf("filler value %d octets, want %d", len(c.Value), tc.filler)
				}
			}
		})
	}

	t.Run("parameter past the field", func(t *testing.T) {
		// RFC 9072 form: one type-2 parameter whose two-octet length (0x0100)
		// runs past the six-octet field.
		body := []byte{4, 0x5b, 0xa0, 0, 90, 1, 2, 3, 4, 255, 255, 0, 6, 2, 0x01, 0x00, 65, 4, 0}
		if _, err := parseOpen(body); !errors.Is(err, capability.ErrShortRead) {
			t.Fatalf("error %v, want capability.ErrShortRead", err)
		}
	})
}
