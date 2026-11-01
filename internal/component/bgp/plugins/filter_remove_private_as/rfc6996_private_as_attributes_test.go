package filter_remove_private_as

import (
	"encoding/binary"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// rfc6996Filter runs one UPDATE through the configured strip filter, the same
// entry point the engine calls for every advertised route.
func rfc6996Filter(t *testing.T, fields string, raw []byte) *sdk.FilterUpdateOutput {
	t.Helper()
	defs := map[string]*removePrivateASDef{"STRIP": {name: "STRIP", mode: removeModeStrip}}
	defsByName.Store(&defs)
	route := removePrivateRoute("STRIP", fields, raw)
	out, err := evaluateRemovePrivateAS(&route)
	if err != nil {
		t.Fatalf("evaluate %s: %v", fields, err)
	}
	return out
}

// rfc6996AS4PathPayload builds an UPDATE payload whose only attribute is an
// AS4_PATH holding one AS_SEQUENCE of the given ASNs.
func rfc6996AS4PathPayload(asns ...uint32) []byte {
	value := make([]byte, 2+4*len(asns))
	value[0] = 2
	value[1] = byte(len(asns))
	for i, asn := range asns {
		binary.BigEndian.PutUint32(value[2+4*i:], asn)
	}
	attr := append([]byte{0xC0, 17, byte(len(value))}, value...)
	payload := make([]byte, 4+len(attr))
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(attr))) //nolint:gosec // test payload is far below 64 KiB
	copy(payload[4:], attr)
	return payload
}

// RFC requirement: RFC6996-4-1 positive -- every Private Use ASN is removed from
// both AS path attributes before advertisement: an AS_PATH of [64512],
// [65534], [4200000000] or [4294967294] beside public ASNs, each end of the
// two-octet and the four-octet Private Use ranges, draws the remove-private
// intent the engine applies to AS_PATH and AS4_PATH, and so does an AS4_PATH
// that carries a Private Use ASN (4294967294) behind an AS_PATH of AS_TRANS
// alone.
//
// VALIDATES: RFC 6996 Section 4 removal over AS_PATH and AS4_PATH at the range edges.
// PREVENTS: a range bound off by one, or AS4_PATH left out of the removal.
func TestRFC6996RemovesPrivateUseASNsFromBothPathAttributes(t *testing.T) {
	for _, asn := range []string{"64512", "65534", "4200000000", "4294967294"} {
		wantRemoveIntent(t, rfc6996Filter(t, `{"as-path":[64496,`+asn+`,64497]}`, nil), `"private"`)
	}
	wantRemoveIntent(t, rfc6996Filter(t, `{"as-path":[23456]}`, rfc6996AS4PathPayload(4294967294)), `"private"`)
}

// RFC requirement: RFC6996-4-1 negative -- removal is confined to the Private
// Use ranges: an AS_PATH of the ASNs just outside both ranges (64511 65535
// 4199999999 4294967295) and an AS4_PATH of the same ASNs are advertised
// unchanged, with no remove-private intent.
//
// VALIDATES: the ASNs adjacent to each Private Use range are kept.
// PREVENTS: a widened range stripping globally routable or reserved ASNs.
func TestRFC6996KeepsASNsOutsideThePrivateUseRanges(t *testing.T) {
	out := rfc6996Filter(t, `{"as-path":[64511,65535,4199999999,4294967295]}`, nil)
	if out.Action != sdk.FilterAccept {
		t.Fatalf("AS_PATH outside the ranges: action = %v (sets %+v), want accept", out.Action, out.Sets)
	}

	out = rfc6996Filter(t, `{"as-path":[23456]}`, rfc6996AS4PathPayload(64511, 65535, 4199999999, 4294967295))
	if out.Action != sdk.FilterAccept {
		t.Fatalf("AS4_PATH outside the ranges: action = %v (sets %+v), want accept", out.Action, out.Sets)
	}
}
