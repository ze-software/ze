package message

import "fmt"

// RFC 4271 Section 4.5 - Error Code is a 1-octet unsigned integer indicating the type of NOTIFICATION.
// NotifyErrorCode represents NOTIFICATION error codes.
type NotifyErrorCode uint8

// RFC 4271 Section 4.5 - NOTIFICATION Error Codes defined in the RFC:
//
//	1 - Message Header Error (Section 6.1)
//	2 - OPEN Message Error (Section 6.2)
//	3 - UPDATE Message Error (Section 6.3)
//	4 - Hold Timer Expired (Section 6.5)
//	5 - Finite State Machine Error (Section 6.6)
//	6 - Cease (Section 6.7)
const (
	NotifyMessageHeader    NotifyErrorCode = 1
	NotifyOpenMessage      NotifyErrorCode = 2
	NotifyUpdateMessage    NotifyErrorCode = 3
	NotifyHoldTimerExpired NotifyErrorCode = 4
	NotifyFSMError         NotifyErrorCode = 5
	NotifyCease            NotifyErrorCode = 6
	NotifyRouteRefresh     NotifyErrorCode = 7 // RFC 7313
)

// RFC 4271 Section 4.5 - Message Header Error subcodes:
//
//	1 - Connection Not Synchronized
//	2 - Bad Message Length
//	3 - Bad Message Type
const (
	NotifyHeaderConnectionNotSync uint8 = 1
	NotifyHeaderBadLength         uint8 = 2
	NotifyHeaderBadType           uint8 = 3
)

// RFC 4271 Section 4.5 - OPEN Message Error subcodes:
//
//	1 - Unsupported Version Number
//	2 - Bad Peer AS
//	3 - Bad BGP Identifier
//	4 - Unsupported Optional Parameter
//	5 - [Deprecated - see Appendix A]
//	6 - Unacceptable Hold Time
//
// Note: Subcode 7 (Unsupported Capability) is from RFC 5492.
const (
	NotifyOpenUnsupportedVersion    uint8 = 1
	NotifyOpenBadPeerAS             uint8 = 2
	NotifyOpenBadBGPID              uint8 = 3
	NotifyOpenUnsupportedOptParam   uint8 = 4
	NotifyOpenUnacceptableHoldTime  uint8 = 6
	NotifyOpenUnsupportedCapability uint8 = 7 // RFC 5492
)

// RFC 4271 Section 4.5 - UPDATE Message Error subcodes:
//
//	 1 - Malformed Attribute List
//	 2 - Unrecognized Well-known Attribute
//	 3 - Missing Well-known Attribute
//	 4 - Attribute Flags Error
//	 5 - Attribute Length Error
//	 6 - Invalid ORIGIN Attribute
//	 7 - [Deprecated - see Appendix A]
//	 8 - Invalid NEXT_HOP Attribute
//	 9 - Optional Attribute Error
//	10 - Invalid Network Field
//	11 - Malformed AS_PATH
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

// RFC 4271 Section 4.5 - Cease is Error Code 6, subcodes defined in RFC 4486.
// RFC 4271 Section 6.7 - Cease NOTIFICATION is used for non-fatal connection termination.
const (
	NotifyCeaseMaxPrefixes         uint8 = 1 // RFC 4486
	NotifyCeaseAdminShutdown       uint8 = 2 // RFC 4486
	NotifyCeasePeerDeconfigured    uint8 = 3 // RFC 4486
	NotifyCeaseAdminReset          uint8 = 4 // RFC 4486
	NotifyCeaseConnectionRejected  uint8 = 5 // RFC 4486
	NotifyCeaseOtherConfigChange   uint8 = 6 // RFC 4486
	NotifyCeaseConnectionCollision uint8 = 7 // RFC 4486
	NotifyCeaseOutOfResources      uint8 = 8 // RFC 4486
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

// RFC 4271 Section 4.5 - NOTIFICATION message format:
//
//	0                   1                   2                   3
//	0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	| Error code    | Error subcode |   Data (variable)             |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// A NOTIFICATION message is sent when an error condition is detected.
// The BGP connection is closed immediately after it is sent.
type Notification struct {
	ErrorCode    NotifyErrorCode // RFC 4271 Section 4.5 - 1-octet error type
	ErrorSubcode uint8           // RFC 4271 Section 4.5 - 1-octet error subcode (0 if unspecific)
	Data         []byte          // RFC 4271 Section 4.5 - variable-length diagnostic data
}

// Type returns the message type (NOTIFICATION).
func (n *Notification) Type() MessageType {
	return TypeNOTIFICATION
}

// RFC 4271 Section 4.5 - Pack serializes the NOTIFICATION to wire format.
// The minimum NOTIFICATION message length is 21 octets (including the 19-octet header).
// Message Length = 21 + Data Length
func (n *Notification) Pack(neg *Negotiated) ([]byte, error) {
	body := make([]byte, 2+len(n.Data))
	body[0] = byte(n.ErrorCode)
	body[1] = n.ErrorSubcode
	copy(body[2:], n.Data)
	return packWithHeader(TypeNOTIFICATION, body), nil
}

// RFC 4271 Section 4.5 - UnpackNotification parses a NOTIFICATION message body.
// The minimum body length is 2 octets (Error Code + Error Subcode).
// Data field length can be determined from: Message Length - 21
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

// RFC 4271 Section 4.5 - subcodeString returns a human-readable subcode name.
// If no appropriate Error Subcode is defined, a zero (Unspecific) value is used.
func (n *Notification) subcodeString() string {
	switch n.ErrorCode { //nolint:exhaustive // Only some codes have specific subcode strings
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
