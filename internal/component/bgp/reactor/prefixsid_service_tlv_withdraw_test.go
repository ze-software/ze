// VALIDATES: a received UPDATE whose SRv6 Service TLV is malformed (the pre-RFC
//            9252 shape ExaBGP 4.2 encodes) reaches the plugins as a withdrawal
//            of its routes (RFC 9252 Section 7, RFC 7606 Section 2)
// PREVENTS: treat-as-withdraw delivered as an empty UPDATE, which leaves an
//           earlier announcement of the route installed

package reactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
)

// prefixSIDServiceSessionAttrs returns ORIGIN IGP, an empty AS_PATH (an
// internal peer), NEXT_HOP 192.0.2.254 and a BGP Prefix-SID attribute (code
// 40, optional transitive) holding one SRv6 L3 Service TLV (type 5) whose
// value is serviceValue.
//
//	Prefix-SID attribute value (RFC 9252 Section 2):
//	+--------+----------------+------------------------------+
//	| type 5 | length (2)     | Service TLV value            |
//	+--------+----------------+------------------------------+
//	offset 0  offset 1-2       offset 3..
func prefixSIDServiceSessionAttrs(serviceValue []byte) []byte {
	tlv := append([]byte{0x05, 0x00, byte(len(serviceValue))}, serviceValue...)
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x00,
		0x40, 0x03, 0x04, 192, 0, 2, 254,
		0xc0, 0x28, byte(len(tlv)),
	}
	return append(attrs, tlv...)
}

// srv6SID is 2001:db8::1, the SID both cases carry.
var srv6SID = []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01}

// TestPrefixSIDMalformedServiceTLVWithdrawsTheRoutes drives the pre-RFC 9252
// SRv6 L3 Service TLV ExaBGP 4.2 encodes through a live, Established session
// and asserts what the session hands to the RIB, the forward rails and the
// plugins (the onMessageReceived callback every event is rendered from).
//
// Method: 4.2 writes the Reserved octet, then the SID, flags and behavior
// straight after it, with no SID Information Sub-TLV header (20 octets). Read
// as RFC 9252 Section 3.1 lays it out, the octets after Reserved are a Sub-TLV
// of type 0x20 and length 0x010d, which overruns the 19 octets left, so the
// Service TLV is malformed. RFC 9252 Section 7: "The treat-as-withdraw action
// [RFC7606] MUST be performed when at least one malformed SRv6 Service TLV is
// present in the BGP Prefix-SID attribute." RFC 7606 Section 2: the UPDATE
// "MUST be treated as though all contained routes had been withdrawn just as if
// they had been listed in the WITHDRAWN ROUTES field". So the one dispatched
// UPDATE carries 10.0.0.0/24 in its Withdrawn Routes field, no attribute and no
// NLRI, the session stays Established, and no NOTIFICATION is sent. A plugin
// therefore sees a withdrawal of the route, never an empty update.
//
// MUTATION: map PrefixSIDMalformedServiceTLV to RFC7606ActionAttributeDiscard
// in message.validatePrefixSIDValue, and the dispatched UPDATE announces the
// route without the attribute: this test goes red.
func TestPrefixSIDMalformedServiceTLVWithdrawsTheRoutes(t *testing.T) {
	session, client, capture, cleanup := setupCapturingSession(t, 65001, false, false)
	defer cleanup()
	nlri := []byte{0x18, 0x0a, 0x00, 0x00}
	service := append([]byte{0x00}, srv6SID...) // Reserved, then the SID with no Sub-TLV header
	service = append(service, 0x00, 0x00, 0x13) // flags, endpoint behavior End.DT4
	attrs := prefixSIDServiceSessionAttrs(service)

	answer := make(chan []byte, 1)
	go func() {
		_, _ = client.Write(buildUpdateMsg(receivedUpdateBody(attrs, nlri)))
		answer <- readOnce(client, 200*time.Millisecond)
	}()

	require.NoError(t, session.ReadAndProcess())
	require.Equal(t, fsm.StateEstablished, session.State())
	got := capture.all()
	require.Len(t, got, 1, "the malformed UPDATE is dispatched once, as a withdrawal")
	gotWithdrawn, gotAttrs, gotNLRI := payloadSections(t, got[0])
	assert.Equal(t, nlri, gotWithdrawn, "the announced route is withdrawn")
	assert.Empty(t, gotAttrs, "the withdrawal carries no path attributes")
	assert.Empty(t, gotNLRI, "nothing is announced")
	assert.Empty(t, <-answer, "treat-as-withdraw sends no NOTIFICATION")
}

// TestPrefixSIDWellFormedServiceTLVKeepsTheRoutes is the counterpart: the same
// UPDATE with the RFC 9252 shape, a SID Information Sub-TLV (Section 3.1,
// Figure 3: type 1, length 21) inside the Service TLV, is dispatched
// unchanged, as an announcement.
func TestPrefixSIDWellFormedServiceTLVKeepsTheRoutes(t *testing.T) {
	session, client, capture, cleanup := setupCapturingSession(t, 65001, false, false)
	defer cleanup()
	nlri := []byte{0x18, 0x0a, 0x00, 0x00}
	// RFC 9252 Section 3.1: SID Information Sub-TLV = type 1, length 2 octets,
	// Reserved1 (1), SRv6 SID (16), SID Flags (1), Endpoint Behavior (2),
	// Reserved2 (1): a length of 21.
	subTLV := []byte{0x01, 0x00, 21, 0x00}
	subTLV = append(subTLV, srv6SID...)
	subTLV = append(subTLV, 0x00, 0x00, 0x13, 0x00)
	service := append([]byte{0x00}, subTLV...) // Service TLV Reserved, then the Sub-TLV
	attrs := prefixSIDServiceSessionAttrs(service)

	answer := make(chan []byte, 1)
	go func() {
		_, _ = client.Write(buildUpdateMsg(receivedUpdateBody(attrs, nlri)))
		answer <- readOnce(client, 200*time.Millisecond)
	}()

	require.NoError(t, session.ReadAndProcess())
	require.Equal(t, fsm.StateEstablished, session.State())
	got := capture.all()
	require.Len(t, got, 1)
	gotWithdrawn, gotAttrs, gotNLRI := payloadSections(t, got[0])
	assert.Empty(t, gotWithdrawn, "nothing is withdrawn")
	assert.Equal(t, attrs, gotAttrs, "the attributes arrive unchanged")
	assert.Equal(t, nlri, gotNLRI, "the route is announced")
	assert.Empty(t, <-answer, "a well-formed UPDATE draws no NOTIFICATION")
}
