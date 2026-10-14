package mup

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

// TestEncodeT1STRefusesWhatParseMUPWithdraws checks that Ze never originates a
// T1ST its own receiver treats as withdrawn (spec-bgp-update-propagation-rfc-defects,
// Q-2 (a)). Method: each refused command lacks one mandatory field the draft
// names, or carries the TEID value the draft calls malformed, and must fail at
// EncodeNLRIHex with the matching error; the accepted command must encode to a
// NLRI ParseMUP reads back whole, with the TEID and endpoint it was given.
func TestEncodeT1STRefusesWhatParseMUPWithdraws(t *testing.T) {
	t.Parallel()

	refused := []struct {
		name string
		args string
		want error
	}{
		{"no teid", "route-type mup-t1st rd 100:100 prefix 192.168.0.2/32 qfi 9 endpoint 10.0.0.1", errMupT1stRequiresTEID},
		{"teid zero", "route-type mup-t1st rd 100:100 prefix 192.168.0.2/32 teid 0 qfi 9 endpoint 10.0.0.1", errMupT1stTEIDZero},
		{"no endpoint", "route-type mup-t1st rd 100:100 prefix 192.168.0.2/32 teid 12345 qfi 9", errMupT1stRequiresEndpoint},
		{"neither", "route-type mup-t1st rd 100:100 prefix 192.168.0.2/32 qfi 9", errMupT1stRequiresTEID},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			encoded, err := EncodeNLRIHex("ipv4/mup", strings.Fields(tc.args))
			if !errors.Is(err, tc.want) {
				t.Fatalf("EncodeNLRIHex(%q) = %q, %v; want error %v", tc.args, encoded, err, tc.want)
			}
		})
	}

	t.Run("teid and endpoint accepted and read back", func(t *testing.T) {
		t.Parallel()
		args := "route-type mup-t1st rd 100:100 prefix 192.168.0.2/32 teid 12345 qfi 9 endpoint 10.0.0.1"
		encoded, err := EncodeNLRIHex("ipv4/mup", strings.Fields(args))
		if err != nil {
			t.Fatalf("EncodeNLRIHex(%q) error: %v", args, err)
		}
		wire, err := hex.DecodeString(encoded)
		if err != nil {
			t.Fatalf("hex decode: %v", err)
		}
		parsed, rest, err := ParseMUP(AFIIPv4, wire)
		if err != nil {
			t.Fatalf("ParseMUP refused what Ze originated (%X): %v", wire, err)
		}
		if len(rest) != 0 {
			t.Fatalf("rest = %d octets, want 0", len(rest))
		}
		if parsed.RouteType() != MUPT1ST {
			t.Fatalf("route type = %d, want T1ST", parsed.RouteType())
		}
		// TEID 12345 = 0x00003039, QFI 9, Endpoint Address Length 32, 10.0.0.1.
		if !strings.Contains(strings.ToUpper(encoded), "0000303909200A000001") {
			t.Fatalf("encoded %s lacks TEID, QFI, endpoint length and endpoint 0000303909200A000001", encoded)
		}
	})
}
