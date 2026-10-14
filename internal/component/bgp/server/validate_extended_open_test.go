// Design: docs/architecture/wire/capabilities.md -- WalkOptionalParams, the one reader of the parameter framing
// Overview: validate.go -- openMessageToRPC, extractCapabilitiesFromOptParams

package server

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// TestOpenMessageToRPCReadsExtendedOpen proves the validate-open RPC hands the
// policy plugins the same capabilities under both framings: an OPEN whose
// parameters stay at most 255 octets and one in the RFC 9072 Section 2 form
// (two-octet Parameter Lengths) both carry the Role capability and the two
// filler capabilities, and Optional Parameters that run past the field are an
// error rather than a shorter list.
//
// VALIDATES: spec-bgp-open-session-rfc-defects D1, the OPEN validation reader:
// extractCapabilitiesFromOptParams walks with capability.WalkOptionalParams and
// the OPEN's own ExtendedParams.
// PREVENTS: a one-octet Parameter Length walk misframing an extended OPEN, so a
// role plugin judges a capability list that is not the peer's.
func TestOpenMessageToRPCReadsExtendedOpen(t *testing.T) {
	for _, tc := range []struct {
		name   string
		filler int
	}{{"base form", 10}, {"RFC 9072 form", 200}} {
		t.Run(tc.name, func(t *testing.T) {
			params := []capability.OptionalParam{capability.RawParam(2, []byte{9, 1, 0x03})}
			for range 2 {
				params = append(params, capability.RawParam(2, append([]byte{0xee, byte(tc.filler)}, make([]byte, tc.filler)...)))
			}
			optional, extended, err := capability.PackOptionalParams(params)
			if err != nil {
				t.Fatalf("pack: %v", err)
			}
			if extended != (tc.filler == 200) {
				t.Fatalf("the test OPEN's framing is not the case's (extended=%v)", extended)
			}
			open := &message.Open{MyAS: 65000, HoldTime: 90, BGPIdentifier: 0x01020304, OptionalParams: optional, ExtendedParams: extended}

			msg, err := openMessageToRPC(open)
			if err != nil {
				t.Fatalf("openMessageToRPC: %v", err)
			}
			if len(msg.Capabilities) != 3 {
				t.Fatalf("capabilities %v, want three", msg.Capabilities)
			}
			if msg.Capabilities[0].Code != 9 {
				t.Errorf("first capability code %d, want 9 (role)", msg.Capabilities[0].Code)
			}
			if msg.Capabilities[0].Hex != "03" {
				t.Errorf("role value %q, want 03", msg.Capabilities[0].Hex)
			}
			for _, c := range msg.Capabilities[1:] {
				if c.Code != 0xee {
					t.Errorf("filler code %d, want 238", c.Code)
				}
				if len(c.Hex) != 2*tc.filler {
					t.Errorf("filler value %d hex digits, want %d", len(c.Hex), 2*tc.filler)
				}
			}
		})
	}

	t.Run("parameter past the field", func(t *testing.T) {
		// RFC 9072 form: type 2, Parameter Length 0x0100, three value octets.
		open := &message.Open{MyAS: 65000, OptionalParams: []byte{2, 0x01, 0x00, 9, 1, 0x03}, ExtendedParams: true}
		_, err := openMessageToRPC(open)
		if !errors.Is(err, capability.ErrShortRead) {
			t.Fatalf("error %v, want capability.ErrShortRead", err)
		}
	})
}
