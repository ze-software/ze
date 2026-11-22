// Design: docs/architecture/wire/messages.md -- BGP NOTIFICATION framing
// RFC: rfc/short/rfc8654.md -- Section 5, NOTIFICATION size limit to non-extended peers
// Related: notification.go -- Notification.Len, CheckNotificationData, BuildTeardownData, BuildCeaseData

package message

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// TestRFC8654NotificationMaxLengthNonExtended verifies that NOTIFICATION
// messages built for a peer that has not advertised the BGP Extended Message
// Capability never exceed 4,096 octets.
//
// RFC 8654 Section 5: "However, if a NOTIFICATION is to be sent to a BGP
// speaker that has not advertised the BGP Extended Message Capability, the
// size of the message MUST NOT exceed 4,096 octets."
//
// VALIDATES: BuildTeardownData and CheckNotificationData allow Data up to
// MaxNotificationDataLen (4075 octets, giving total Notification.Len == 4096)
// and refuse 4076+ octets; BuildCeaseData bounds Administrative Shutdown
// Communication to 128 octets so the NOTIFICATION fits well within 4096.
// PREVENTS: sending a >4096-octet NOTIFICATION to a peer that has not
// advertised RFC 8654 Extended Message support.
//
// RFC requirement: RFC8654-5-3 positive -- NOTIFICATION payloads up to
// MaxNotificationDataLen (4075 octets) and Cease Shutdown Communications
// produce total NOTIFICATION lengths <= 4096 octets (HeaderLen + 2 + len(Data))
// accepted by CheckNotificationData and ValidateLengthWithMax(false).
// RFC requirement: RFC8654-5-3 negative -- a NOTIFICATION Data payload of
// MaxNotificationDataLen+1 (4076 octets, total 4097 octets) is refused by
// CheckNotificationData and BuildTeardownData with ErrTeardownTextRefused, and
// a 4097-octet NOTIFICATION header is rejected when extended is false.
func TestRFC8654NotificationMaxLengthNonExtended(t *testing.T) {
	t.Parallel()

	// Exact 4096-octet boundary: 19-byte header + 2-byte code/subcode + 4075-byte Data.
	maxData, err := BuildTeardownData(NotifyCease, NotifyCeaseOtherConfigChange, strings.Repeat("a", MaxNotificationDataLen))
	require.NoError(t, err)
	require.NoError(t, CheckNotificationData(maxData))

	nMax := &Notification{ErrorCode: NotifyCease, ErrorSubcode: NotifyCeaseOtherConfigChange, Data: maxData}
	require.Equal(t, MaxMsgLen, nMax.Len(nil))
	require.NoError(t, (Header{Length: uint16(nMax.Len(nil)), Type: msgtype.TypeNOTIFICATION}).ValidateLengthWithMax(false))

	// Cease Administrative Shutdown with oversized text is capped to 128 octets.
	ceaseData := BuildCeaseData(NotifyCeaseAdminShutdown, strings.Repeat("x", 5000))
	nCease := &Notification{ErrorCode: NotifyCease, ErrorSubcode: NotifyCeaseAdminShutdown, Data: ceaseData}
	require.LessOrEqual(t, nCease.Len(nil), MaxMsgLen)

	// Negative: 4076 octets of Data would produce a 4097-octet NOTIFICATION; refused.
	overText := strings.Repeat("b", MaxNotificationDataLen+1)
	require.ErrorIs(t, CheckNotificationData([]byte(overText)), ErrTeardownTextRefused)
	_, err = BuildTeardownData(NotifyCease, NotifyCeaseOtherConfigChange, overText)
	require.ErrorIs(t, err, ErrTeardownTextRefused)
	require.Error(t, (Header{Length: MaxMsgLen + 1, Type: msgtype.TypeNOTIFICATION}).ValidateLengthWithMax(false))
}
