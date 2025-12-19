package message

import "fmt"

// NotifyErrorCode represents NOTIFICATION error codes (RFC 4271).
type NotifyErrorCode uint8

// NOTIFICATION error codes (RFC 4271 Section 4.5)
const (
	NotifyMessageHeader    NotifyErrorCode = 1
	NotifyOpenMessage      NotifyErrorCode = 2
	NotifyUpdateMessage    NotifyErrorCode = 3
	NotifyHoldTimerExpired NotifyErrorCode = 4
	NotifyFSMError         NotifyErrorCode = 5
	NotifyCease            NotifyErrorCode = 6
	NotifyRouteRefresh     NotifyErrorCode = 7 // RFC 7313
)

// Message Header Error subcodes
const (
	NotifyHeaderConnectionNotSync uint8 = 1
	NotifyHeaderBadLength         uint8 = 2
	NotifyHeaderBadType           uint8 = 3
)

// OPEN Message Error subcodes
const (
	NotifyOpenUnsupportedVersion    uint8 = 1
	NotifyOpenBadPeerAS             uint8 = 2
	NotifyOpenBadBGPID              uint8 = 3
	NotifyOpenUnsupportedOptParam   uint8 = 4
	NotifyOpenUnacceptableHoldTime  uint8 = 6
	NotifyOpenUnsupportedCapability uint8 = 7
)

// UPDATE Message Error subcodes
const (
	NotifyUpdateMalformedAttr    uint8 = 1
	NotifyUpdateUnrecognizedAttr uint8 = 2
	NotifyUpdateMissingAttr      uint8 = 3
	NotifyUpdateAttrFlags        uint8 = 4
	NotifyUpdateAttrLength       uint8 = 5
	NotifyUpdateInvalidOrigin    uint8 = 6
	NotifyUpdateInvalidNextHop   uint8 = 8
	NotifyUpdateOptionalAttr     uint8 = 9
	NotifyUpdateInvalidNetwork   uint8 = 10
	NotifyUpdateMalformedASPath  uint8 = 11
)

// Cease subcodes (RFC 4486)
const (
	NotifyCeaseMaxPrefixes         uint8 = 1
	NotifyCeaseAdminShutdown       uint8 = 2
	NotifyCeasePeerDeconfigured    uint8 = 3
	NotifyCeaseAdminReset          uint8 = 4
	NotifyCeaseConnectionRejected  uint8 = 5
	NotifyCeaseOtherConfigChange   uint8 = 6
	NotifyCeaseConnectionCollision uint8 = 7
	NotifyCeaseOutOfResources      uint8 = 8
	NotifyCeaseHardReset           uint8 = 9 // RFC 8538
)

// String returns a human-readable name for the error code.
func (c NotifyErrorCode) String() string {
	switch c {
	case NotifyMessageHeader:
		return "Message Header Error"
	case NotifyOpenMessage:
		return "OPEN Message Error"
	case NotifyUpdateMessage:
		return "UPDATE Message Error"
	case NotifyHoldTimerExpired:
		return "Hold Timer Expired"
	case NotifyFSMError:
		return "FSM Error"
	case NotifyCease:
		return "Cease"
	case NotifyRouteRefresh:
		return "Route Refresh Error"
	default:
		return fmt.Sprintf("Unknown(%d)", c)
	}
}

// Notification represents a BGP NOTIFICATION message (RFC 4271 Section 4.5).
type Notification struct {
	ErrorCode    NotifyErrorCode
	ErrorSubcode uint8
	Data         []byte
}

// Type returns the message type (NOTIFICATION).
func (n *Notification) Type() MessageType {
	return TypeNOTIFICATION
}

// Pack serializes the NOTIFICATION to wire format.
func (n *Notification) Pack(neg *Negotiated) ([]byte, error) {
	body := make([]byte, 2+len(n.Data))
	body[0] = byte(n.ErrorCode)
	body[1] = n.ErrorSubcode
	copy(body[2:], n.Data)
	return packWithHeader(TypeNOTIFICATION, body), nil
}

// UnpackNotification parses a NOTIFICATION message body.
func UnpackNotification(data []byte) (*Notification, error) {
	if len(data) < 2 {
		return nil, ErrShortRead
	}
	return &Notification{
		ErrorCode:    NotifyErrorCode(data[0]),
		ErrorSubcode: data[1],
		Data:         data[2:],
	}, nil
}

// String returns a human-readable representation of the notification.
func (n *Notification) String() string {
	subcodeStr := n.subcodeString()
	if len(n.Data) > 0 {
		return fmt.Sprintf("%s/%s (data: %x)", n.ErrorCode, subcodeStr, n.Data)
	}
	return fmt.Sprintf("%s/%s", n.ErrorCode, subcodeStr)
}

// subcodeString returns a human-readable subcode name.
func (n *Notification) subcodeString() string {
	switch n.ErrorCode {
	case NotifyCease:
		return ceaseSubcodeString(n.ErrorSubcode)
	case NotifyOpenMessage:
		return openSubcodeString(n.ErrorSubcode)
	case NotifyUpdateMessage:
		return updateSubcodeString(n.ErrorSubcode)
	default:
		if n.ErrorSubcode == 0 {
			return "Unspecific"
		}
		return fmt.Sprintf("Subcode(%d)", n.ErrorSubcode)
	}
}

func ceaseSubcodeString(subcode uint8) string {
	switch subcode {
	case NotifyCeaseMaxPrefixes:
		return "Maximum Number of Prefixes Reached"
	case NotifyCeaseAdminShutdown:
		return "Administrative Shutdown"
	case NotifyCeasePeerDeconfigured:
		return "Peer De-configured"
	case NotifyCeaseAdminReset:
		return "Administrative Reset"
	case NotifyCeaseConnectionRejected:
		return "Connection Rejected"
	case NotifyCeaseOtherConfigChange:
		return "Other Configuration Change"
	case NotifyCeaseConnectionCollision:
		return "Connection Collision Resolution"
	case NotifyCeaseOutOfResources:
		return "Out of Resources"
	case NotifyCeaseHardReset:
		return "Hard Reset"
	default:
		return fmt.Sprintf("Subcode(%d)", subcode)
	}
}

func openSubcodeString(subcode uint8) string {
	switch subcode {
	case NotifyOpenUnsupportedVersion:
		return "Unsupported Version Number"
	case NotifyOpenBadPeerAS:
		return "Bad Peer AS"
	case NotifyOpenBadBGPID:
		return "Bad BGP Identifier"
	case NotifyOpenUnsupportedOptParam:
		return "Unsupported Optional Parameter"
	case NotifyOpenUnacceptableHoldTime:
		return "Unacceptable Hold Time"
	case NotifyOpenUnsupportedCapability:
		return "Unsupported Capability"
	default:
		return fmt.Sprintf("Subcode(%d)", subcode)
	}
}

func updateSubcodeString(subcode uint8) string {
	switch subcode {
	case NotifyUpdateMalformedAttr:
		return "Malformed Attribute List"
	case NotifyUpdateUnrecognizedAttr:
		return "Unrecognized Well-known Attribute"
	case NotifyUpdateMissingAttr:
		return "Missing Well-known Attribute"
	case NotifyUpdateAttrFlags:
		return "Attribute Flags Error"
	case NotifyUpdateAttrLength:
		return "Attribute Length Error"
	case NotifyUpdateInvalidOrigin:
		return "Invalid ORIGIN Attribute"
	case NotifyUpdateInvalidNextHop:
		return "Invalid NEXT_HOP Attribute"
	case NotifyUpdateOptionalAttr:
		return "Optional Attribute Error"
	case NotifyUpdateInvalidNetwork:
		return "Invalid Network Field"
	case NotifyUpdateMalformedASPath:
		return "Malformed AS_PATH"
	default:
		return fmt.Sprintf("Subcode(%d)", subcode)
	}
}
