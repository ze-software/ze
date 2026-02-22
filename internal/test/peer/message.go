// Design: docs/architecture/testing/ci-format.md — BGP message types and wire helpers
// Related: peer.go — test peer that uses these messages
// Related: checker.go — message validation against expectations

package peer

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"unicode/utf8"
)

// BGP message types.
const (
	MsgOPEN         = 1
	MsgUPDATE       = 2
	MsgNOTIFICATION = 3
	MsgKEEPALIVE    = 4
	MsgROUTEREFRESH = 5
)

// BGP header length.
const HeaderLen = 19

// BGP marker (16 bytes of 0xFF).
var Marker = []byte{
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
}

// Message represents a BGP message.
type Message struct {
	Header []byte
	Body   []byte
}

// Kind returns the message type.
func (m *Message) Kind() byte {
	if len(m.Header) > 18 {
		return m.Header[18]
	}
	return 0
}

// IsKeepalive returns true if this is a KEEPALIVE message.
func (m *Message) IsKeepalive() bool { return m.Kind() == MsgKEEPALIVE }

// IsUpdate returns true if this is an UPDATE message.
func (m *Message) IsUpdate() bool { return m.Kind() == MsgUPDATE }

// IsEOR returns true if this is an End-of-RIB marker.
func (m *Message) IsEOR() bool {
	if !m.IsUpdate() {
		return false
	}
	return len(m.Body) == 4 || len(m.Body) == 11
}

// Stream returns the hex-encoded message.
func (m *Message) Stream() string {
	return strings.ToUpper(hex.EncodeToString(append(m.Header, m.Body...)))
}

// ReadMessage reads a BGP message from a connection.
func ReadMessage(conn net.Conn) ([]byte, []byte, error) {
	header := make([]byte, HeaderLen)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, nil, err
	}

	length := binary.BigEndian.Uint16(header[16:18])
	if length < HeaderLen {
		return nil, nil, fmt.Errorf("invalid message length: %d", length)
	}

	bodyLen := int(length) - HeaderLen
	body := make([]byte, bodyLen)
	if bodyLen > 0 {
		if _, err := io.ReadFull(conn, body); err != nil {
			return nil, nil, err
		}
	}

	return header, body, nil
}

// KeepaliveMsg returns a BGP KEEPALIVE message.
func KeepaliveMsg() []byte {
	msg := make([]byte, 19)
	copy(msg, Marker)
	binary.BigEndian.PutUint16(msg[16:], 19)
	msg[18] = MsgKEEPALIVE
	return msg
}

// DefaultRouteMsg returns an UPDATE with route 0.0.0.0/32.
// Used for testing UPDATE receive handling.
func DefaultRouteMsg() []byte {
	return []byte{
		// BGP Header (16 bytes marker + 2 bytes length + 1 byte type)
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0x00, 0x31, // Length: 49 bytes (19 header + 30 body)
		0x02, // Type: UPDATE
		// UPDATE body (30 bytes)
		0x00, 0x00, // Withdrawn routes length: 0
		0x00, 0x15, // Path attributes length: 21
		// ORIGIN: IGP (0) - 4 bytes
		0x40, 0x01, 0x01, 0x00,
		// AS_PATH: empty - 3 bytes
		0x40, 0x02, 0x00,
		// NEXT_HOP: 127.0.0.1 - 7 bytes
		0x40, 0x03, 0x04, 0x7F, 0x00, 0x00, 0x01,
		// LOCAL_PREF: 100 - 7 bytes
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64,
		// NLRI: 0.0.0.0/32 - 5 bytes
		0x20,                   // Prefix length: 32 bits
		0x00, 0x00, 0x00, 0x00, // Prefix: 0.0.0.0
	}
}

// NotificationMsg builds a BGP NOTIFICATION message with Cease/Administrative Shutdown.
// RFC 4271 Section 4.5 - NOTIFICATION Message Format.
// RFC 9003 - Extended BGP Administrative Shutdown Communication.
//
// Format: [Error Code 6][Subcode 2][Length][Shutdown Communication]
// - Error Code: 6 (Cease)
// - Subcode: 2 (Administrative Shutdown)
// - Length: 1 byte (0-255)
// - Shutdown Communication: UTF-8, max 255 bytes per RFC 9003.
func NotificationMsg(text string) []byte {
	textBytes := []byte(text)

	// RFC 9003: max 255 octets for shutdown communication
	// Must truncate at valid UTF-8 boundary to maintain RFC compliance
	if len(textBytes) > 255 {
		textBytes = truncateUTF8(textBytes, 255)
	}

	// Header (19) + Error Code (1) + Subcode (1) + Length (1) + Text
	msgLen := 19 + 3 + len(textBytes)

	msg := make([]byte, msgLen)
	copy(msg, Marker)
	binary.BigEndian.PutUint16(msg[16:], uint16(msgLen)) //nolint:gosec // msgLen max 277
	msg[18] = MsgNOTIFICATION
	msg[19] = 6                    // Cease
	msg[20] = 2                    // Administrative Shutdown (RFC 9003)
	msg[21] = byte(len(textBytes)) // Length of shutdown communication
	copy(msg[22:], textBytes)

	return msg
}

// truncateUTF8 truncates bytes to maxLen while preserving valid UTF-8.
// It finds the last valid rune boundary at or before maxLen.
func truncateUTF8(b []byte, maxLen int) []byte {
	if len(b) <= maxLen {
		return b
	}

	// Start at maxLen and work backwards to find valid UTF-8 boundary
	for i := maxLen; i > 0; i-- {
		if utf8.RuneStart(b[i]) {
			// Found a rune start - check if there's room for the full rune
			_, size := utf8.DecodeRune(b[i:])
			if i+size <= maxLen {
				return b[:i+size]
			}
			// Rune would exceed maxLen, try previous position
			continue
		}
	}

	// Fallback: no valid boundary found (shouldn't happen with valid UTF-8)
	return b[:maxLen]
}

func isTimeout(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}
