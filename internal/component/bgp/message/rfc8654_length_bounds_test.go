package message

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// rfc8654RequireBadLength asserts err is the RFC 4271 Section 6.1 Bad Message
// Length NOTIFICATION whose Data carries the erroneous Length field.
func rfc8654RequireBadLength(t *testing.T, err error, length uint16, what string) {
	t.Helper()
	var notif *Notification
	require.True(t, errors.As(err, &notif), "%s: want a NOTIFICATION, got %v", what, err)
	require.Equal(t, NotifyMessageHeader, notif.ErrorCode, what)
	require.Equal(t, NotifyHeaderBadLength, notif.ErrorSubcode, what)
	require.Equal(t, []byte{byte(length >> 8), byte(length)}, notif.Data, "%s: Data is the erroneous Length", what)
}

// TestRFC8654LengthBoundsPerTypeAndCapability proves the 19..4096 and
// 19..65535 Length bounds by message type and by Extended Message state, with
// the RFC 4271 error each violation raises.
//
// VALIDATES: without Extended Message every type is bounded at 4096; with it,
// UPDATE, NOTIFICATION and ROUTE-REFRESH accept up to 65535 while OPEN stays
// at 4096; a Length below 19 is refused for every type in both states. Every
// refusal is Message Header Error / Bad Message Length with the erroneous
// Length in Data.
// PREVENTS: the raised limit leaking to OPEN, a missing lower bound, and a
// refusal that does not follow RFC 4271 Section 6.1.
//
// RFC requirement: RFC8654-6-1 positive -- Length 19 for every type, 4096 for OPEN, UPDATE,
// NOTIFICATION and ROUTE-REFRESH without the capability, and 65535 for UPDATE, NOTIFICATION
// and ROUTE-REFRESH with it, are accepted.
// RFC requirement: RFC8654-6-1 negative -- Length 0 and 18 for every type in both states,
// 4097 for every type without the capability, and 4097 or 65535 for OPEN with it, are refused
// as Bad Message Length carrying the erroneous Length.
// RFC requirement: RFC8654-5-4 negative -- an over-length UPDATE (4097 without the
// capability) is refused with the RFC 4271 Section 6.1 NOTIFICATION: Message Header Error,
// Bad Message Length, Data the erroneous Length field.
func TestRFC8654LengthBoundsPerTypeAndCapability(t *testing.T) {
	raised := []msgtype.MessageType{msgtype.TypeUPDATE, msgtype.TypeNOTIFICATION, msgtype.TypeROUTEREFRESH}
	floorOK := map[msgtype.MessageType]uint16{
		msgtype.TypeOPEN: 29, msgtype.TypeUPDATE: 23, msgtype.TypeNOTIFICATION: 21,
		msgtype.TypeKEEPALIVE: 19, msgtype.TypeROUTEREFRESH: 23,
	}
	for msgType, floor := range floorOK {
		for _, extended := range []bool{false, true} {
			require.NoError(t, Header{Length: floor, Type: msgType}.ValidateLengthWithMax(extended), "%s %d ext=%v", msgType, floor, extended)
			for _, length := range []uint16{0, 18} {
				err := Header{Length: length, Type: msgType}.ValidateLengthWithMax(extended)
				rfc8654RequireBadLength(t, err, length, msgType.String())
			}
			err := Header{Length: MaxMsgLen + 1, Type: msgType}.ValidateLengthWithMax(false)
			rfc8654RequireBadLength(t, err, MaxMsgLen+1, msgType.String())
		}
	}
	for _, msgType := range []msgtype.MessageType{msgtype.TypeOPEN, msgtype.TypeUPDATE, msgtype.TypeNOTIFICATION, msgtype.TypeROUTEREFRESH} {
		require.NoError(t, Header{Length: MaxMsgLen, Type: msgType}.ValidateLengthWithMax(false), msgType.String())
	}
	for _, msgType := range raised {
		require.NoError(t, Header{Length: ExtMsgLen, Type: msgType}.ValidateLengthWithMax(true), msgType.String())
	}
	for _, length := range []uint16{MaxMsgLen + 1, ExtMsgLen} {
		err := Header{Length: length, Type: msgtype.TypeOPEN}.ValidateLengthWithMax(true)
		rfc8654RequireBadLength(t, err, length, "OPEN with Extended Message")
	}
}
