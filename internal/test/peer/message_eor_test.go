// Design: docs/architecture/testing/ci-format.md — BGP message types and wire helpers
// RFC: rfc/short/rfc4724.md — Section 2, the two End-of-RIB encodings
// Related: checker.go — an unmatched EoR is silently accepted, which is why this
// classification decides whether a .ci negative can fail at all

package peer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// eorMessage wraps an UPDATE body in a header so Message.IsEOR can read it.
func eorMessage(body []byte) *Message {
	hdr := make([]byte, 0, HeaderLen)
	hdr = append(hdr, Marker...)
	total := HeaderLen + len(body)
	hdr = append(hdr, byte(total>>8), byte(total), MsgUPDATE)
	return &Message{Header: hdr, Body: body}
}

// VALIDATES: RFC 4724 Section 2 -- an End-of-RIB marker is the empty UPDATE or a
// lone MP_UNREACH_NLRI carrying only AFI and SAFI. Classification is by CONTENT.
//
// PREVENTS: the length test this replaced. It accepted any 11-byte body, and a
// legacy marker (body 4) with a 7-byte attribute stamped onto it is exactly 11.
// Checker.ExpectedOrKeepalive silently accepts an unmatched EoR, so a .ci
// asserting that a relayed marker arrives UNSTAMPED could only fail by timing
// out: the stamped message it existed to refuse was classified as the very thing
// it was waiting for.
//
// NON-VACUITY (ai/rules/interop-and-goal-validation.md): the two accept rows and
// the two stamped-reject rows are the same fixtures with one attribute added, so
// a classifier that answered a constant fails one side or the other.
func TestMessageIsEOR(t *testing.T) {
	// MP_UNREACH_NLRI, 3-octet header, value AFI=2 SAFI=1: the multiprotocol
	// marker (RFC 4760 Section 4).
	mpMarker := []byte{0x80, 15, 3, 0x00, 0x02, 0x01}
	// The same attribute with a 4-octet header (extended length flag set).
	mpMarkerExt := []byte{0x90, 15, 0x00, 0x03, 0x00, 0x02, 0x01}
	// OTC (RFC 9234 Section 5): flags, code 35, len 4, ASN. 7 bytes on the wire.
	otc := []byte{0xC0, 35, 4, 0x00, 0x00, 0xFD, 0xE8}
	// AS_PATH [65000], 4-octet ASNs: 9 bytes on the wire. This is what the
	// forward rail used to synthesize onto an attribute-free UPDATE.
	asPath := []byte{0x40, 2, 6, 2, 1, 0x00, 0x00, 0xFD, 0xE8}

	body := func(attrs []byte, trailing ...byte) []byte {
		out := []byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}
		out = append(out, attrs...)
		return append(out, trailing...)
	}

	cases := []struct {
		name string
		body []byte
		want bool
	}{
		{"legacy: completely empty UPDATE", body(nil), true},
		{"multiprotocol: bare MP_UNREACH", body(mpMarker), true},
		{"multiprotocol: bare MP_UNREACH, extended header", body(mpMarkerExt), true},

		{"legacy marker with OTC stamped on it", body(otc), false},
		{"legacy marker with a synthesized AS_PATH", body(asPath), false},
		{"multiprotocol marker with OTC stamped on it", body(append(append([]byte{}, mpMarker...), otc...)), false},
		{"MP_UNREACH carrying a withdrawn prefix", body([]byte{0x80, 15, 7, 0x00, 0x02, 0x01, 32, 0x20, 0x01, 0x0d, 0xb8}), false},
		{"withdrawn routes present", []byte{0, 4, 24, 10, 0, 0, 0, 0}, false},
		{"NLRI present", body(nil, 24, 10, 0, 0), false},
		{"attribute is not MP_UNREACH", body([]byte{0x40, 1, 1, 0}), false},
		{"attribute length overruns the body", []byte{0, 0, 0, 9, 0x80}, false},
		{"body too short", []byte{0, 0, 0}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, eorMessage(tc.body).IsEOR())
		})
	}
}

// VALIDATES: only an UPDATE can be an End-of-RIB marker.
func TestMessageIsEORRejectsNonUpdate(t *testing.T) {
	m := eorMessage(nil)
	m.Header[18] = MsgKEEPALIVE
	assert.False(t, m.IsEOR())
}
