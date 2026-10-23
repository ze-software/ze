// Design: docs/architecture/wire/messages.md -- NOTIFICATION
// Related: rfc4760_mp_reach_test.go -- the RFC 4760 Section 7 session-reset trigger

package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMPAttributeResetNotification checks the NOTIFICATION each MP attribute session reset
// carries. Method: build one attribute section per fault, run ValidateUpdateRFC7606, and
// compare the result's Notification with the RFC 4760 Section 7 one built from the whole
// attribute as it sits in the section.
//
// VALIDATES: an MP_REACH_NLRI shorter than 5 octets, an MP_UNREACH_NLRI shorter than 3, and
// an MP_REACH_NLRI whose next hop overruns the attribute each reset with UPDATE Message
// Error / Optional Attribute Error (3/9) and the whole attribute (flags, type, length,
// value) as Data. A duplicated MP_REACH_NLRI still resets with no Notification, which the
// reactor sends as Malformed Attribute List (3/1), as RFC 7606 Section 3 (g) names.
// PREVENTS: one of these resets falling back to 3/1 with empty Data, and the duplicate
// being reported as a fault of one attribute.
func TestMPAttributeResetNotification(t *testing.T) {
	t.Parallel()

	reachShort := []byte{attrFlagOptional, attrCodeMPReachNLRI, 0x02, 0x00, 0x01}
	unreachShort := []byte{attrFlagOptional, attrCodeMPUnreachNLRI, 0x02, 0x00, 0x02}
	// AFI 1, SAFI 1, next-hop length 4 (a valid IPv4 length), and only 2 next-hop octets.
	nextHopOverrun := []byte{attrFlagOptional, attrCodeMPReachNLRI, 0x06, 0x00, 0x01, 0x01, 0x04, 0x0a, 0x00}

	for name, c := range map[string]struct {
		section []byte
		attr    []byte
	}{
		"mp_reach shorter than 5":         {sJoin(sOrigin, sASPath, reachShort), reachShort},
		"mp_unreach shorter than 3":       {sJoin(unreachShort), unreachShort},
		"mp_reach next hop overruns attr": {sJoin(sOrigin, sASPath, nextHopOverrun), nextHopOverrun},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			result := ValidateUpdateRFC7606(c.section, false, false, false)
			require.Equal(t, RFC7606ActionSessionReset, result.Action, result.Description)
			require.NotNil(t, result.Notification, "the reset must carry the RFC 4760 Section 7 NOTIFICATION")
			require.Equal(t, NotifyUpdateMessage, result.Notification.ErrorCode)
			require.Equal(t, NotifyUpdateOptionalAttr, result.Notification.ErrorSubcode)
			require.Equal(t, c.attr, result.Notification.Data)
		})
	}

	t.Run("duplicate mp_reach keeps malformed attribute list", func(t *testing.T) {
		t.Parallel()
		reach := mpReachIPv6Attr()
		result := ValidateUpdateRFC7606(sJoin(sOrigin, sASPath, reach, reach), false, false, false)
		require.Equal(t, RFC7606ActionSessionReset, result.Action, result.Description)
		require.Nil(t, result.Notification, "a duplicate is a fault of the list: the reactor sends 3/1")
	})
}
