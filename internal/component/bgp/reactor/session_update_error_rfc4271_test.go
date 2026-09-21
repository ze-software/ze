// VALIDATES: the RFC 4271 Section 6.3 UPDATE rules RFC 7606 leaves in force reach the
// wire: a section-length overrun and a duplicate MP attribute each draw NOTIFICATION 3/1
// Malformed Attribute List, and an UPDATE with correct attributes and no NLRI is valid.
// PREVENTS: a session reset that tells the peer nothing, and a validator that refuses
// every attribute-only UPDATE passing the positive arm alone.

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// rfc4271WellKnownAttrs is ORIGIN=IGP, an empty AS_PATH and NEXT_HOP 10.0.0.1: the three
// well-known mandatory attributes, each well formed, 14 octets in all.
var rfc4271WellKnownAttrs = []byte{
	0x40, 0x01, 0x01, 0x00,
	0x40, 0x02, 0x00,
	0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01,
}

// rfc4271MPReachIPv4 is one MP_REACH_NLRI (optional, code 14) for IPv4 unicast, next hop
// 10.0.0.1, announcing 10.0.0.0/8. 11 octets of value.
var rfc4271MPReachIPv4 = []byte{
	0x80, 0x0e, 0x0b,
	0x00, 0x01, 0x01, // AFI 1, SAFI 1
	0x04, 0x0a, 0x00, 0x00, 0x01, // next hop length 4, 10.0.0.1
	0x00,       // reserved
	0x08, 0x0a, // 10.0.0.0/8
}

// rfc4271Community is one COMMUNITY (optional transitive, code 8) carrying 65001:1.
var rfc4271Community = []byte{0xc0, 0x08, 0x04, 0xfd, 0xe9, 0x00, 0x01}

// rfc4271OverrunBody is an UPDATE whose Total Attribute Length claims one octet more
// than the message carries: Withdrawn Routes Length 0, Total Attribute Length 15, then
// the 14 octets of rfc4271WellKnownAttrs and nothing else. 0 + 15 + 23 exceeds the
// message length of 18 + 19.
func rfc4271OverrunBody() []byte {
	body := makeUpdateBody(nil, rfc4271WellKnownAttrs, nil)
	body[3] = byte(len(rfc4271WellKnownAttrs) + 1)
	return body
}

// TestRFC4271UpdateMalformedAttributeList reads the NOTIFICATION off the socket for the
// two Section 6.3 cases whose subcode RFC 7606 keeps: the section-length overrun
// (Section 3(b), "remains unchanged") and the duplicate MP_REACH_NLRI (Section 3(g)).
//
// RFC requirement: RFC4271-6.3-4 positive — a Total Attribute Length that overruns the
// message, and a Withdrawn Routes Length that does, each draw NOTIFICATION 3/1 Malformed
// Attribute List and a session reset.
// RFC requirement: RFC4271-6.3-4 negative — the same attributes with consistent section
// lengths are accepted with no NOTIFICATION and no error.
// RFC requirement: RFC4271-6.3-15 positive — MP_REACH_NLRI appearing twice draws
// NOTIFICATION 3/1 Malformed Attribute List and a session reset.
// RFC requirement: RFC4271-6.3-15 negative — COMMUNITY appearing twice draws no
// NOTIFICATION: RFC 7606 Section 3(g) discards the second occurrence and the UPDATE is
// processed with exactly one COMMUNITY left in it.
func TestRFC4271UpdateMalformedAttributeList(t *testing.T) {
	withdrawnOverrun := makeUpdateBody(nil, rfc4271WellKnownAttrs, nil)
	withdrawnOverrun[1] = byte(len(withdrawnOverrun)) // claims more than the body holds

	duplicateMPReach := makeUpdateBody(nil,
		append(append(append([]byte{}, rfc4271MPReachIPv4...), rfc4271MPReachIPv4...),
			rfc4271WellKnownAttrs[:7]...), // ORIGIN and AS_PATH; MP_REACH carries the next hop
		nil)

	duplicateCommunity := makeUpdateBody(nil,
		append(append(append([]byte{}, rfc4271WellKnownAttrs...), rfc4271Community...),
			rfc4271Community...),
		[]byte{0x08, 0x0a})

	tests := []struct {
		name      string
		body      []byte
		wantReset bool
	}{
		{"total attribute length overrun", rfc4271OverrunBody(), true},
		{"withdrawn routes length overrun", withdrawnOverrun, true},
		{"consistent lengths", makeUpdateBody(nil, rfc4271WellKnownAttrs, []byte{0x08, 0x0a}), false},
		{"duplicate MP_REACH_NLRI", duplicateMPReach, true},
		{"duplicate COMMUNITY", duplicateCommunity, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, written := openSentSessionAS(t, 65002, 0x0A000002)

			out, action, err := session.enforceRFC7606(wireu.NewWireUpdate(tt.body, 0))

			if !tt.wantReset {
				require.NoError(t, err, "a well-formed UPDATE is accepted")
				assert.NotEqual(t, message.RFC7606ActionSessionReset, action)
				assert.NotEqual(t, message.RFC7606ActionTreatAsWithdraw, action)
				code, subcode, found := notificationFrom(t, written)
				assert.False(t, found, "no NOTIFICATION for an accepted UPDATE, got %d/%d", code, subcode)
				count, _ := countAttrCode(attrSection(out.Payload()), 8)
				if tt.name == "duplicate COMMUNITY" {
					assert.Equal(t, 1, count, "RFC 7606 Section 3(g): all occurrences but the first are discarded")
				}
				return
			}

			require.Error(t, err, "a session reset surfaces as an error")
			assert.Equal(t, message.RFC7606ActionSessionReset, action)

			code, subcode, found := notificationFrom(t, written)
			require.True(t, found, "a session reset is indicated by a NOTIFICATION")
			assert.Equal(t, message.NotifyUpdateMessage, message.NotifyErrorCode(code),
				"error code must be UPDATE Message Error")
			assert.Equal(t, message.NotifyUpdateMalformedAttr, subcode,
				"subcode must be Malformed Attribute List")
		})
	}
}

// TestRFC4271UpdateWithoutNLRIIsValid feeds an UPDATE whose path attributes are correct
// and whose NLRI field is empty, and checks it is neither refused nor withdrawn.
//
// RFC requirement: RFC4271-6.3-17 positive — ORIGIN, AS_PATH and NEXT_HOP with no NLRI
// and no withdrawn routes returns no error, no session reset, no treat-as-withdraw and no
// NOTIFICATION.
// RFC requirement: RFC4271-6.3-17 negative — the same attribute-only UPDATE with a Total
// Attribute Length overrun is refused with NOTIFICATION 3/1: the acceptance rests on the
// attributes being correct, not on the NLRI being absent.
func TestRFC4271UpdateWithoutNLRIIsValid(t *testing.T) {
	t.Run("correct attributes and no NLRI", func(t *testing.T) {
		session, written := openSentSessionAS(t, 65002, 0x0A000002)

		_, action, err := session.enforceRFC7606(
			wireu.NewWireUpdate(makeUpdateBody(nil, rfc4271WellKnownAttrs, nil), 0))

		require.NoError(t, err, "an UPDATE with correct attributes and no NLRI is valid")
		assert.Equal(t, message.RFC7606ActionNone, action)
		code, subcode, found := notificationFrom(t, written)
		assert.False(t, found, "a valid UPDATE draws no NOTIFICATION, got %d/%d", code, subcode)
	})

	t.Run("incorrect attributes and no NLRI", func(t *testing.T) {
		session, written := openSentSessionAS(t, 65002, 0x0A000002)

		_, action, err := session.enforceRFC7606(wireu.NewWireUpdate(rfc4271OverrunBody(), 0))

		require.Error(t, err, "an attribute-list overrun is refused whether or not NLRI follows")
		assert.Equal(t, message.RFC7606ActionSessionReset, action)
		code, subcode, found := notificationFrom(t, written)
		require.True(t, found, "the refusal is indicated by a NOTIFICATION")
		assert.Equal(t, message.NotifyUpdateMessage, message.NotifyErrorCode(code))
		assert.Equal(t, message.NotifyUpdateMalformedAttr, subcode)
	})
}
