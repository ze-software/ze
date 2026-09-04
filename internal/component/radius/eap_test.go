// Related: eap.go -- appendEAPMessage and eapPacketFrom, the two directions
// RFC: rfc/short/rfc3579.md -- Section 3.1
//
// VALIDATES: that one EAP packet crosses the RADIUS attribute boundary
// unchanged in both directions, split at 253 octets into consecutive
// EAP-Message attributes on the way out and concatenated on the way in.
// PREVENTS: a fragmenter that splits at 255 (the Length, not the String), one
// that emits the pieces out of order, and a reassembler that joins two separate
// runs into an EAP packet neither side wrote.

package radius

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// eapMessageValues returns the values of the EAP-Message attributes in attrs,
// and fails when they are not one consecutive run in order.
func eapMessageValues(t *testing.T, attrs []Attr) [][]byte {
	t.Helper()
	var values [][]byte
	first, count := -1, 0
	for index, a := range attrs {
		if a.Type != AttrEAPMessage {
			continue
		}
		if first < 0 {
			first = index
		}
		require.Equal(t, first+count, index,
			"RFC 3579 Section 3.1 requires the EAP-Message attributes to be consecutive")
		count++
		values = append(values, a.Value)
	}
	return values
}

// TestEAPMessageSplitsAtTheAttributeLimit walks the boundary the RADIUS
// attribute Length imposes on an EAP packet.
//
// VALIDATES: AC-4 -- 253 octets is one attribute, 254 is two of 253 and 1, and
// every piece but the last is full.
// PREVENTS: a split at 255, which is the attribute LENGTH and not the String it
// can carry: RFC 2865 Section 5 counts the two header octets inside that
// Length, so a 255-octet value produces an attribute that cannot be encoded.
//
// RFC requirement: RFC3579-3.1-1 positive -- an EAP packet longer than 253
// octets is carried in consecutive, in-order EAP-Message attributes
// (eap.go appendEAPMessage).
func TestEAPMessageSplitsAtTheAttributeLimit(t *testing.T) {
	cases := []struct {
		name   string
		size   int
		pieces []int
	}{
		{"one octet", 1, []int{1}},
		{"exactly the limit", 253, []int{253}},
		{"one octet over", 254, []int{253, 1}},
		{"two full attributes", 506, []int{253, 253}},
		{"two full and a remainder", 507, []int{253, 253, 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			packet := make([]byte, tc.size)
			for i := range packet {
				packet[i] = byte(i)
			}

			attrs, err := appendEAPMessage(nil, packet)
			require.NoError(t, err)

			values := eapMessageValues(t, attrs)
			require.Len(t, values, len(tc.pieces))
			for index, want := range tc.pieces {
				assert.Len(t, values[index], want, "piece %d", index)
			}
			// In ORDER: joining the pieces reproduces the packet exactly.
			assert.Equal(t, packet, bytes.Join(values, nil))
		})
	}
}

// TestEAPMessageKeepsTheRunConsecutive proves the split does not let another
// attribute into the middle of the run.
//
// VALIDATES: AC-4 -- an attribute already in the list stays before the run, and
// the State the challenge loop appends stays after it.
// PREVENTS: a builder that interleaves, which is the one way to satisfy "in
// order" and still break "consecutive".
//
// RFC requirement: RFC3579-3.1-1 negative -- no attribute of another type
// appears between two EAP-Message attributes (eap.go appendEAPMessage,
// authenticator_eap.go eapCredential).
func TestEAPMessageKeepsTheRunConsecutive(t *testing.T) {
	packet := make([]byte, 600)
	attrs, err := eapCredential(packet, []byte("opaque-state"))
	require.NoError(t, err)

	values := eapMessageValues(t, attrs)
	require.Len(t, values, 3, "600 octets is three EAP-Message attributes")

	// The two attributes the credential owes sit AFTER the run, never inside it.
	require.Len(t, attrs, 5)
	assert.Equal(t, uint8(AttrMessageAuthenticator), attrs[3].Type)
	assert.Equal(t, uint8(AttrState), attrs[4].Type)
	assert.Equal(t, []byte("opaque-state"), attrs[4].Value)
}

// TestEAPMessageRefusesAnEmptyPacket holds the zero-octet boundary.
//
// VALIDATES: an EAP packet of 0 octets is refused before the RADIUS packet is
// built, so no empty EAP-Message goes out.
// PREVENTS: an EAP-Message of Length 2. RFC 3579 Section 3.1 gives the
// attribute a Length of ">= 3", and RFC 2865 Section 5 states that "Text of
// length zero (0) MUST NOT be sent; omit the entire attribute instead".
func TestEAPMessageRefusesAnEmptyPacket(t *testing.T) {
	_, err := appendEAPMessage(nil, nil)
	require.Error(t, err)
	_, err = appendEAPMessage(nil, []byte{})
	require.Error(t, err)
}

// TestEAPMessageConcatenatesOnTheWayIn is the receive direction.
//
// VALIDATES: AC-5 -- the values of several EAP-Message attributes are
// concatenated into one EAP packet before it is decoded, and attributes of
// other types on either side of the run are ignored.
// PREVENTS: a reader that takes FindAttr's first value and decodes a fragment
// as a whole EAP packet.
//
// RFC requirement: RFC3579-3.1-1 positive -- "Where more than one EAP-Message
// attribute is included, it is assumed that the attributes are to be
// concatenated to form a single EAP packet" (eap.go eapPacketFrom).
func TestEAPMessageConcatenatesOnTheWayIn(t *testing.T) {
	packet := make([]byte, 400)
	for i := range packet {
		packet[i] = byte(i * 7)
	}
	run, err := appendEAPMessage(nil, packet)
	require.NoError(t, err)
	require.Len(t, run, 2)

	pkt := &Packet{Code: CodeAccessChallenge}
	pkt.Attrs = append(pkt.Attrs, Attr{Type: AttrServiceType, Value: AttrUint32(1)})
	pkt.Attrs = append(pkt.Attrs, run...)
	pkt.Attrs = append(pkt.Attrs, Attr{Type: AttrState, Value: []byte("s")})

	got, err := eapPacketFrom(pkt)
	require.NoError(t, err)
	assert.Equal(t, packet, got)
}

// TestEAPMessageRefusesASecondRun is the "one EAP packet per RADIUS packet"
// rule at the decoder.
//
// VALIDATES: AC-13 -- two runs of EAP-Message attributes, separated by another
// attribute, are refused rather than concatenated across the gap.
// PREVENTS: a reassembler that joins every EAP-Message value in the packet. A
// server sending two EAP packets, or a run whose order was broken, would then
// produce one EAP packet neither side wrote, and the peer would answer it.
//
// RFC requirement: RFC3579-3.1-2 negative -- "Multiple EAP packets MUST NOT be
// encoded within EAP-Message attributes contained within a single
// Access-Challenge, Access-Accept, Access-Reject or Access-Request packet"
// (eap.go eapPacketFrom).
func TestEAPMessageRefusesASecondRun(t *testing.T) {
	pkt := &Packet{
		Code: CodeAccessChallenge,
		Attrs: []Attr{
			{Type: AttrEAPMessage, Value: []byte{0x01, 0x01, 0x00, 0x05, 0x01}},
			{Type: AttrState, Value: []byte("gap")},
			{Type: AttrEAPMessage, Value: []byte{0x01, 0x02, 0x00, 0x05, 0x01}},
		},
	}
	_, err := eapPacketFrom(pkt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "consecutive")
}

// TestEAPMessageAbsenceIsNotAnError separates "no EAP packet" from "a broken
// EAP packet", because the two get different answers upstream.
//
// VALIDATES: a reply carrying no EAP-Message yields nil and no error, so the
// challenge loop can tell a plain Access-Reject from a malformed one.
// PREVENTS: an error on every non-EAP reply, which would turn a legitimate
// Access-Reject into an infrastructure failure and let the AAA chain continue
// past a server that refused the login.
func TestEAPMessageAbsenceIsNotAnError(t *testing.T) {
	pkt := &Packet{Code: CodeAccessReject, Attrs: []Attr{{Type: AttrReplyMessage, Value: []byte("no")}}}
	got, err := eapPacketFrom(pkt)
	require.NoError(t, err)
	assert.Nil(t, got)
}

// TestEAPMessageRefusesAnEmptyRun covers a run whose every value is empty.
//
// VALIDATES: a reply whose EAP-Message attributes carry no octets is refused,
// rather than producing a zero-length EAP packet.
// PREVENTS: a nil return that the caller cannot tell from "no EAP-Message
// present", which is the silently-wrong zero ai/rules/principles.md forbids.
func TestEAPMessageRefusesAnEmptyRun(t *testing.T) {
	pkt := &Packet{
		Code:  CodeAccessChallenge,
		Attrs: []Attr{{Type: AttrEAPMessage, Value: []byte{}}},
	}
	_, err := eapPacketFrom(pkt)
	require.Error(t, err)
}
