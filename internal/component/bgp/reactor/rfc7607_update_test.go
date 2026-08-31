// VALIDATES: an UPDATE carrying AS 0 is stopped at Session.enforceRFC7606, the receive
// entry point that runs before any plugin callback, so no route holding AS 0 ever reaches
// the RIB and nothing can relay one. AS_PATH is treat-as-withdrawn and the three
// informational attributes are discarded, which is what RFC 7606 and RFC 6793 each
// prescribe for the attribute RFC 7607 makes malformed.
// PREVENTS: relaying the AS that RFC 6491 uses to mark a prefix as not routable, which is
// how a malicious party hijacks a resource its holder declared unrouted. It also prevents
// the opposite failure: an ordinary UPDATE must survive this path untouched, and the
// accepted case here is what would break if the scan misread a segment length as an AS.

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// rfc7607Attrs builds the path attributes of a complete IPv4 unicast UPDATE: the three
// well-known mandatory attributes, with the AS_PATH carrying the supplied two-octet ASNs,
// followed by whatever extra attribute the caller appends.
func rfc7607Attrs(asns ...uint16) []byte {
	asPath := []byte{0x02, byte(len(asns))}
	for _, asn := range asns {
		asPath = append(asPath, byte(asn>>8), byte(asn))
	}

	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN = IGP
	attrs = append(attrs, 0x40, 0x02, byte(len(asPath)))
	attrs = append(attrs, asPath...)
	return append(attrs, 0x40, 0x03, 0x04, 192, 0, 2, 254) // NEXT_HOP
}

// rfc7607Session builds an EBGP session (local AS 65001, peer AS 65002) to receive on.
func rfc7607Session() *Session {
	return rfc9552Session()
}

// TestRFC7607UpdateWithASZeroIsNotAccepted drives the whole receive rail rather than the
// leaf validator: enforceRFC7606 is what processMessage calls before any plugin sees the
// UPDATE, so a verdict reached here is the verdict that decides whether the route can be
// propagated at all.
//
// RFC requirement: RFC7607-2-1 positive -- a route whose AS_PATH holds AS 0 is
// treat-as-withdrawn at receive, so it never enters the RIB and cannot be propagated.
// RFC requirement: RFC7607-2-2 positive -- the RFC 7606 procedure for a malformed AS_PATH
// is treat-as-withdraw, and that is the action the session takes.
func TestRFC7607UpdateWithASZeroIsNotAccepted(t *testing.T) {
	s := rfc7607Session()
	nlri := []byte{24, 192, 0, 2}
	body := makeUpdateBody(nil, rfc7607Attrs(65002, 0), nlri)

	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))

	require.NoError(t, err, "a malformed AS_PATH must not reset the session")
	assert.Equal(t, message.RFC7606ActionTreatAsWithdraw, action,
		"RFC 7607 Section 2 with RFC 7606 Section 7.2: AS 0 in AS_PATH is treat-as-withdraw")
}

// TestRFC7607UpdateWithRealASIsAccepted is the discrimination for the test above. The two
// UPDATEs differ in one AS number, so the verdict is bound to the zero and not to the
// shape of the message.
//
// RFC requirement: RFC7607-2-1 negative -- a route whose AS_PATH holds only real AS
// numbers is accepted and stays available to propagate.
// RFC requirement: RFC7607-2-2 negative -- the same UPDATE with a real AS draws no RFC
// 7606 action at all.
func TestRFC7607UpdateWithRealASIsAccepted(t *testing.T) {
	s := rfc7607Session()
	nlri := []byte{24, 192, 0, 2}
	body := makeUpdateBody(nil, rfc7607Attrs(65002, 65003), nlri)

	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))

	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionNone, action,
		"an UPDATE with no AS 0 must pass the receive path untouched")
}

// TestRFC7607AggregatorASZeroIsDiscarded drives the same rail with a well-formed AS_PATH
// and an AGGREGATOR naming AS 0, so the only thing that can move the verdict is the
// aggregator's AS.
//
// RFC requirement: RFC7607-2-1 positive -- an AGGREGATOR naming AS 0 is stripped at
// receive, so ze cannot propagate one.
// RFC requirement: RFC7607-2-2 positive -- the RFC 7606 procedure for a malformed
// AGGREGATOR is attribute discard, and that is the action the session takes.
func TestRFC7607AggregatorASZeroIsDiscarded(t *testing.T) {
	s := rfc7607Session()
	attrs := append(rfc7607Attrs(65002), 0xC0, 0x07, 0x06, 0x00, 0x00, 192, 0, 2, 1)
	body := makeUpdateBody(nil, attrs, []byte{24, 192, 0, 2})

	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))

	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionAttributeDiscard, action,
		"RFC 7607 Section 2 with RFC 7606 Section 7.7: AS 0 in AGGREGATOR is attribute discard")
}

// RFC requirement: RFC7607-2-1 negative -- an AGGREGATOR naming a real AS survives the
// receive path, so the discard above is bound to the zero.
func TestRFC7607AggregatorRealASIsKept(t *testing.T) {
	s := rfc7607Session()
	attrs := append(rfc7607Attrs(65002), 0xC0, 0x07, 0x06, 0xFD, 0xEA, 192, 0, 2, 1)
	body := makeUpdateBody(nil, attrs, []byte{24, 192, 0, 2})

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))

	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionNone, action,
		"an AGGREGATOR naming a real AS is not malformed")

	count, _ := countAttrCode(rfc8669PathAttrs(t, wu.Payload()), 0x07)
	assert.Equal(t, 1, count, "the AGGREGATOR must still be on the wire")
}

// TestRFC7607AS4AggregatorASZeroIsDiscarded covers the four-octet sibling, whose action
// RFC 6793 rather than RFC 7606 names.
//
// RFC requirement: RFC7607-2-3 positive -- an AS4_AGGREGATOR naming AS 0 draws the RFC
// 6793 Section 6 attribute discard on the real receive rail.
func TestRFC7607AS4AggregatorASZeroIsDiscarded(t *testing.T) {
	s := rfc7607Session()
	attrs := append(rfc7607Attrs(65002), 0xC0, 0x12, 0x08, 0x00, 0x00, 0x00, 0x00, 192, 0, 2, 1)
	body := makeUpdateBody(nil, attrs, []byte{24, 192, 0, 2})

	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))

	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionAttributeDiscard, action,
		"RFC 7607 Section 2 with RFC 6793 Section 6: AS 0 in AS4_AGGREGATOR is attribute discard")
}

// RFC requirement: RFC7607-2-3 negative -- an AS4_AGGREGATOR naming a real four-octet AS
// survives the receive path with its bytes intact.
func TestRFC7607AS4AggregatorRealASIsKept(t *testing.T) {
	s := rfc7607Session()
	attrs := append(rfc7607Attrs(65002), 0xC0, 0x12, 0x08, 0x00, 0x01, 0x00, 0x01, 192, 0, 2, 1)
	body := makeUpdateBody(nil, attrs, []byte{24, 192, 0, 2})

	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))

	require.NoError(t, err)
	assert.Equal(t, message.RFC7606ActionNone, action,
		"an AS4_AGGREGATOR naming a real AS is not malformed")

	count, _ := countAttrCode(rfc8669PathAttrs(t, wu.Payload()), 0x12)
	assert.Equal(t, 1, count, "the AS4_AGGREGATOR must still be on the wire")
}
