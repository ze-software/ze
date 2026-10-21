package mup

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
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

// TestEncodeT1STWritesZeroSourceLength checks that a T1ST with no `source`
// still carries its Source Address Length octet, set to 0.
//
// VALIDATES: draft-ietf-bess-mup-safi Section 3.1.3.1, "Source Address Length
// is 0 bytes then the Source address is not carried within the NLRI.", and the
// TLV rule that measures remaining octets from "the offset of the end of the
// Source Address", so the length octet is a mandatory field. Method: encode a
// T1ST without `source` and compare the whole NLRI, which ExaBGP 6.0 writes
// the same way (0x18 octets ending in 00).
// PREVENTS: Ze stopping after the endpoint address, a NLRI one octet short that
// a receiver following the draft treats as withdrawn.
func TestEncodeT1STWritesZeroSourceLength(t *testing.T) {
	t.Parallel()

	args := "route-type mup-t1st rd 100:100 prefix 192.168.0.2/32 teid 12345 qfi 9 endpoint 10.0.0.1"
	encoded, err := EncodeNLRIHex("ipv4/mup", strings.Fields(args))
	if err != nil {
		t.Fatalf("EncodeNLRIHex(%q) error: %v", args, err)
	}
	// Architecture 1, route type 3, length 0x18, RD 0:100:100, prefix length 32,
	// 192.168.0.2, TEID 12345, QFI 9, endpoint length 32, 10.0.0.1, source length 0.
	const want = "0100031800000064000000642" + "0C0A80002000030390920" + "0A00000100"
	if !strings.EqualFold(encoded, want) {
		t.Fatalf("EncodeNLRIHex(%q) = %s, want %s", args, encoded, want)
	}
}

// TestEncodeRouteWritesNoNextHop checks the announced IPv4 MUP route.
//
// VALIDATES: RFC 4760 Section 3, "An UPDATE message that carries no NLRI, other
// than the one encoded in the MP_REACH_NLRI attribute, SHOULD NOT carry the
// NEXT_HOP attribute." Method: encode an ipv4/mup route with an IPv4 next hop,
// walk to the path attributes and look for code 3.
// PREVENTS: the announce rail restating the next hop as a NEXT_HOP beside
// MP_REACH_NLRI, which the config rail and ExaBGP 6.0 do not send.
func TestEncodeRouteWritesNoNextHop(t *testing.T) {
	t.Parallel()

	cmd := "mup-t1st 192.168.0.2/32 rd 100:100 teid 12345 qfi 9 endpoint 10.0.0.1 next-hop 10.0.0.2"
	update, _, err := EncodeRoute(cmd, "ipv4/mup", 65000, true, true, false)
	if err != nil {
		t.Fatalf("EncodeRoute: %v", err)
	}
	// Header 19, withdrawn length 2 (zero here), path attribute length 2.
	const attrsAt = 19 + 2 + 2
	if len(update) < attrsAt {
		t.Fatalf("update too short: %d bytes", len(update))
	}
	withdrawn := int(binary.BigEndian.Uint16(update[19:]))
	if withdrawn != 0 {
		t.Fatalf("withdrawn length = %d, want 0", withdrawn)
	}
	attrLen := int(binary.BigEndian.Uint16(update[21:]))
	if attrsAt+attrLen > len(update) {
		t.Fatalf("path attribute length %d exceeds update size %d", attrLen, len(update))
	}
	attrs := update[attrsAt : attrsAt+attrLen]
	if _, _, _, found := attribute.AttrFind(attrs, attribute.AttrNextHop); found {
		t.Errorf("ipv4/mup UPDATE carries NEXT_HOP beside MP_REACH_NLRI: % x", attrs)
	}
	if _, _, _, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI); !found {
		t.Errorf("ipv4/mup UPDATE carries no MP_REACH_NLRI: % x", attrs)
	}
}
