// VALIDATES: an UPDATE whose LARGE_COMMUNITY attribute repeats a value gets
// no RFC 7606 action on the receive path, and the repeat is removed from the
// attribute Ze retains and passes along; distinct values are all kept.
// PREVENTS: a duplicate check in the malformed-attribute decision that would
// treat the route as withdrawn or discard the attribute, and a relay that
// forwards the redundant values the peer sent.

package reactor

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestRFC8092DuplicateLargeCommunityIsNotMalformed drives an UPDATE carrying a
// LARGE_COMMUNITY attribute with the same 12-octet value twice through
// enforceRFC7606, the malformed-attribute decision Ze acts on.
// Method: the verdict must be no action and no error, and the attribute must
// still be present after receive, so the duplicate neither withdraws the route
// nor discards the attribute.
//
// RFC requirement: RFC8092-6-3 positive -- a LARGE_COMMUNITY attribute holding one value twice gets no RFC 7606 action and no error on the receive path, and the attribute is kept.
func TestRFC8092DuplicateLargeCommunityIsNotMalformed(t *testing.T) {
	value := []byte{
		0, 0, 0xFD, 0xE9, 0, 0, 0, 1, 0, 0, 0, 2,
		0, 0, 0xFD, 0xE9, 0, 0, 0, 1, 0, 0, 0, 2,
	}
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 192, 0, 2, 1, // NEXT_HOP
		0xC0, byte(attribute.AttrLargeCommunity), byte(len(value)),
	}
	attrs = append(attrs, value...)
	s := rfc7311EBGPSession()
	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(makeUpdateBody(nil, attrs, []byte{24, 10, 40, 0}), 0))
	require.NoError(t, err, "a duplicate value must not reset the session")
	require.Equal(t, message.RFC7606ActionNone, action, "a duplicate value must not make the attribute malformed")
	count, _ := countAttrCode(rfc8669PathAttrs(t, wu.Payload()), uint8(attribute.AttrLargeCommunity))
	require.Equal(t, 1, count, "the attribute must not be discarded")
}

// rfc8092Value returns the 12-octet Large Community 65001:n:n+1000.
func rfc8092Value(n uint32) []byte {
	v := []byte{0, 0, 0xFD, 0xE9, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(v[4:], n)
	binary.BigEndian.PutUint32(v[8:], n+1000)
	return v
}

// rfc8092Values concatenates the Large Community values named by ns.
func rfc8092Values(ns ...uint32) []byte {
	out := make([]byte, 0, len(ns)*12)
	for _, n := range ns {
		out = append(out, rfc8092Value(n)...)
	}
	return out
}

// rfc8092Receive drives an IPv4 unicast UPDATE carrying value as its
// LARGE_COMMUNITY through enforceRFC7606, using the Extended Length header
// when the value needs it, and returns the published UPDATE's attribute
// section, the attribute section a peer receives when the route is passed
// along, the published payload and the received one.
func rfc8092Receive(t *testing.T, value []byte) (retained, sent, published, received []byte) {
	t.Helper()
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 192, 0, 2, 1, // NEXT_HOP
	}
	if len(value) > 255 {
		attrs = append(attrs, 0xD0, byte(attribute.AttrLargeCommunity), byte(len(value)>>8), byte(len(value)))
	} else {
		attrs = append(attrs, 0xC0, byte(attribute.AttrLargeCommunity), byte(len(value)))
	}
	attrs = append(attrs, value...)
	body := makeUpdateBody(nil, attrs, []byte{24, 10, 40, 0})
	received = append([]byte{}, body...)
	s := rfc7311EBGPSession()
	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err, "redundant values must be removed silently, not reported as an error")
	require.Equal(t, message.RFC7606ActionNone, action, "redundant values must be removed silently, with no RFC 7606 action")
	return rfc8669PathAttrs(t, wu.Payload()), teForwardedAttrs(t, wu.Payload(), false), wu.Payload(), received
}

// TestRFC8092RedundantLargeCommunityRemovedBeforePropagation receives UPDATEs
// whose LARGE_COMMUNITY repeats values and checks the attribute Ze retains
// and the one it puts on a peer's wire.
// Method: two shapes, a one-octet-length attribute holding one value twice and
// an Extended Length attribute of 23 values of which three repeat earlier ones
// (which exercises the filter the redundancy scan uses above 16 values). Each
// must get no RFC 7606 action and no error, and both the published attribute
// and the forwarded one must hold each value once, in first-occurrence order.
//
// RFC requirement: RFC8092-3-2 positive -- a received LARGE_COMMUNITY holding a value twice, or 23 values with 3 repeats, gets no error and no RFC 7606 action, and the attribute Ze publishes and forwards to a peer holds each value once in first-occurrence order.
func TestRFC8092RedundantLargeCommunityRemovedBeforePropagation(t *testing.T) {
	tests := []struct {
		name     string
		received []byte
		want     []byte
	}{
		{"one value twice", rfc8092Values(1, 1), rfc8092Values(1)},
		{
			"23 values three repeated, extended length",
			rfc8092Values(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 3, 11, 12, 13, 14, 15, 16, 1, 17, 18, 19, 20, 20),
			rfc8092Values(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retained, sent, _, _ := rfc8092Receive(t, tt.received)
			count, large := countAttrCode(retained, uint8(attribute.AttrLargeCommunity))
			require.Equal(t, 1, count)
			require.Equal(t, tt.want, large, "the attribute Ze retains must hold each value once")
			count, large = countAttrCode(sent, uint8(attribute.AttrLargeCommunity))
			require.Equal(t, 1, count)
			require.Equal(t, tt.want, large, "the redundant values must be removed before the route is passed along")
		})
	}
}

// TestRFC8092DistinctLargeCommunitiesKeptWhole receives UPDATEs whose
// LARGE_COMMUNITY holds no value twice and checks nothing is removed.
// Method: a three-value attribute and a 22-value Extended Length one, whose
// values share the Global Administrator and differ in the Local Data parts
// only. The published payload must be the received body octet for octet, and
// the forwarded attribute must hold every value, so the removal is confined
// to values that are redundant.
//
// RFC requirement: RFC8092-3-2 negative -- a received LARGE_COMMUNITY of 3, or 22, distinct values is published octet-equal to the received body and forwarded with every value, so values that are not redundant are never removed.
func TestRFC8092DistinctLargeCommunitiesKeptWhole(t *testing.T) {
	tests := []struct {
		name     string
		received []byte
	}{
		{"three values", rfc8092Values(1, 2, 3)},
		{"22 values, extended length", rfc8092Values(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retained, sent, published, received := rfc8092Receive(t, tt.received)
			require.Equal(t, received, published, "the published body must be the received one, octet for octet")
			_, large := countAttrCode(retained, uint8(attribute.AttrLargeCommunity))
			require.Equal(t, tt.received, large, "a value that is not redundant must be kept")
			_, large = countAttrCode(sent, uint8(attribute.AttrLargeCommunity))
			require.Equal(t, tt.received, large, "a value that is not redundant must be passed along")
		})
	}
}
