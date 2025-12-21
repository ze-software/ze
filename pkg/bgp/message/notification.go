package message

import (
	"fmt"
	"unicode/utf8"
)

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
//	7 - Unsupported Capability (RFC 5492)
//	11 - Role Mismatch (RFC 9234)
const (
	NotifyOpenUnsupportedVersion    uint8 = 1
	NotifyOpenBadPeerAS             uint8 = 2
	NotifyOpenBadBGPID              uint8 = 3
	NotifyOpenUnsupportedOptParam   uint8 = 4
	NotifyOpenUnacceptableHoldTime  uint8 = 6
	NotifyOpenUnsupportedCapability uint8 = 7  // RFC 5492
	NotifyOpenRoleMismatch          uint8 = 11 // RFC 9234
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

// RFC 6608 - BGP Finite State Machine Error Subcodes:
//
//	0 - Unspecified Error
//	1 - Receive Unexpected Message in OpenSent State
//	2 - Receive Unexpected Message in OpenConfirm State
//	3 - Receive Unexpected Message in Established State
const (
	NotifyFSMUnspecified           uint8 = 0 // RFC 6608
	NotifyFSMUnexpectedOpenSent    uint8 = 1 // RFC 6608
	NotifyFSMUnexpectedOpenConfirm uint8 = 2 // RFC 6608
	NotifyFSMUnexpectedEstablished uint8 = 3 // RFC 6608
)

// RFC 4271 Section 4.5 - Cease is Error Code 6, subcodes defined in RFC 4486.
// RFC 4271 Section 6.7 - Cease NOTIFICATION is used for non-fatal connection termination.
const (
	NotifyCeaseMaxPrefixes         uint8 = 1  // RFC 4486
	NotifyCeaseAdminShutdown       uint8 = 2  // RFC 4486
	NotifyCeasePeerDeconfigured    uint8 = 3  // RFC 4486
	NotifyCeaseAdminReset          uint8 = 4  // RFC 4486
	NotifyCeaseConnectionRejected  uint8 = 5  // RFC 4486
	NotifyCeaseOtherConfigChange   uint8 = 6  // RFC 4486
	NotifyCeaseConnectionCollision uint8 = 7  // RFC 4486
	NotifyCeaseOutOfResources      uint8 = 8  // RFC 4486
	NotifyCeaseHardReset           uint8 = 9  // RFC 8538
	NotifyCeaseBFDDown             uint8 = 10 // RFC 9384
)

// RFC 7313 Section 5 - ROUTE-REFRESH Message Error subcodes (Error Code 7):
//
//	1 - Invalid Message Length
const (
	NotifyRouteRefreshInvalidLength uint8 = 1 // RFC 7313
)

// subcodeUnspecific is the string for unspecific/unspecified subcodes.
const subcodeUnspecific = "Unspecific"

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

// Error implements the error interface, allowing *Notification to be returned
// as an error from parsing functions. This enables proper BGP error handling
// where the notification can be sent to the peer before closing the connection.
// RFC 4271 Section 4.5 - NOTIFICATION messages indicate protocol errors.
func (n *Notification) Error() string {
	return n.String()
}

// ShutdownMessage extracts the shutdown communication from a Cease NOTIFICATION.
// RFC 9003 Section 2 - For Cease/Admin Shutdown (6,2) or Cease/Admin Reset (6,4):
// - Data field contains: 1-byte length + UTF-8 message (up to 255 bytes)
// - If length is 0 or no data, returns empty string
// - Returns error if length exceeds buffer or UTF-8 is invalid
//
// For non-shutdown notifications, returns empty string and no error.
func (n *Notification) ShutdownMessage() (string, error) {
	// RFC 9003 only applies to Cease with Admin Shutdown (2) or Admin Reset (4)
	if n.ErrorCode != NotifyCease {
		return "", nil
	}
	if n.ErrorSubcode != NotifyCeaseAdminShutdown && n.ErrorSubcode != NotifyCeaseAdminReset {
		return "", nil
	}

	// No data = old-style shutdown without message
	if len(n.Data) == 0 {
		return "", nil
	}

	// RFC 9003 Section 2: First byte is length
	msgLen := int(n.Data[0])
	if msgLen == 0 {
		return "", nil
	}

	payload := n.Data[1:]
	if len(payload) < msgLen {
		return "", fmt.Errorf("shutdown message length %d exceeds available data %d", msgLen, len(payload))
	}

	// RFC 9003 Section 2: Message MUST be UTF-8 encoded
	msg := payload[:msgLen]
	if !utf8.Valid(msg) {
		return "", fmt.Errorf("shutdown message contains invalid UTF-8")
	}

	return string(msg), nil
}

// RFC 4271 Section 4.5 - subcodeString returns a human-readable subcode name.
// If no appropriate Error Subcode is defined, a zero (Unspecific) value is used.
func (n *Notification) subcodeString() string {
	switch n.ErrorCode { //nolint:exhaustive // Only some codes have specific subcode strings
	case NotifyMessageHeader:
		return headerSubcodeString(n.ErrorSubcode)
	case NotifyOpenMessage:
		return openSubcodeString(n.ErrorSubcode)
	case NotifyUpdateMessage:
		return updateSubcodeString(n.ErrorSubcode)
	case NotifyFSMError:
		return fsmSubcodeString(n.ErrorSubcode)
	case NotifyCease:
		return ceaseSubcodeString(n.ErrorSubcode)
	case NotifyRouteRefresh:
		return routeRefreshSubcodeString(n.ErrorSubcode)
	default:
		if n.ErrorSubcode == 0 {
			return subcodeUnspecific
		}
		return fmt.Sprintf("Subcode(%d)", n.ErrorSubcode)
	}
}

// headerSubcodeString returns the string for Message Header Error subcodes.
// RFC 4271 Section 6.1
func headerSubcodeString(subcode uint8) string {
	switch subcode {
	case 0:
		return subcodeUnspecific
	case NotifyHeaderConnectionNotSync:
		return "Connection Not Synchronized"
	case NotifyHeaderBadLength:
		return "Bad Message Length"
	case NotifyHeaderBadType:
		return "Bad Message Type"
	default:
		return fmt.Sprintf("Subcode(%d)", subcode)
	}
}

// fsmSubcodeString returns the string for FSM Error subcodes.
// RFC 6608 Section 3
func fsmSubcodeString(subcode uint8) string {
	switch subcode {
	case NotifyFSMUnspecified:
		return "Unspecified Error"
	case NotifyFSMUnexpectedOpenSent:
		return "Receive Unexpected Message in OpenSent State"
	case NotifyFSMUnexpectedOpenConfirm:
		return "Receive Unexpected Message in OpenConfirm State"
	case NotifyFSMUnexpectedEstablished:
		return "Receive Unexpected Message in Established State"
	default:
		return fmt.Sprintf("Subcode(%d)", subcode)
	}
}

// routeRefreshSubcodeString returns the string for Route-Refresh Error subcodes.
// RFC 7313 Section 5
func routeRefreshSubcodeString(subcode uint8) string {
	switch subcode {
	case 0:
		return "Reserved"
	case NotifyRouteRefreshInvalidLength:
		return "Invalid Message Length"
	default:
		return fmt.Sprintf("Subcode(%d)", subcode)
	}
}

func ceaseSubcodeString(subcode uint8) string {
	switch subcode {
	case 0:
		return subcodeUnspecific
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
	case NotifyCeaseBFDDown:
		return "BFD Down"
	default:
		return fmt.Sprintf("Subcode(%d)", subcode)
	}
}

func openSubcodeString(subcode uint8) string {
	switch subcode {
	case 0:
		return subcodeUnspecific
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
	case NotifyOpenRoleMismatch:
		return "Role Mismatch"
	default:
		return fmt.Sprintf("Subcode(%d)", subcode)
	}
}

func updateSubcodeString(subcode uint8) string {
	switch subcode {
	case 0:
		return subcodeUnspecific
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
