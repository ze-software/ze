// Design: docs/architecture/exabgp-bridge.md -- the NOTIFICATION an ExaBGP script reads
// Overview: bridge_event.go -- the JSON encoder
// Related: bridge_event_text.go -- the text encoder
// Related: bridge_event_open.go -- the OPEN an ExaBGP JSON script reads
//
// RFC: rfc/short/rfc9003.md

package bridge

import (
	"encoding/hex"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Notification fields as ExaBGP's Notification (src/exabgp/bgp/message/
// notification.py) shows them.
const (
	notifyCodeCease            = 6
	notifySubcodeAdminShutdown = 2
	notifySubcodeAdminReset    = 4
	printableFirst             = 0x20 // PRINTABLE_FIRST
	printableLast              = 0x7E // PRINTABLE_LAST
)

// notificationData is one NOTIFICATION's Data field, read off ze's event,
// which writes it in lowercase hex (internal/component/bgp/format/json.go,
// Notification).
type notificationData struct {
	code, subcode int
	raw           []byte
	// undecodable holds ze's field when it is not hex, which ze's encoder never
	// writes: it is then shown as it came rather than as the empty field it
	// would decode to.
	undecodable string
}

// readNotificationData reads the NOTIFICATION of an event.
func (e Event) readNotificationData() notificationData {
	code, _ := e.Data["code"].(float64)
	subcode, _ := e.Data["subcode"].(float64)
	field, _ := e.Data["data"].(string)
	data := notificationData{code: int(code), subcode: int(subcode)}
	raw, err := hex.DecodeString(field)
	if err != nil {
		data.undecodable = field
		return data
	}
	data.raw = raw
	return data
}

// isShutdown answers whether the Data field is an RFC 9003 Shutdown
// Communication: Notification.SHUTDOWN_SUBCODES, (6, 2) and (6, 4). RFC 9003
// Section 2: "If a BGP speaker decides to terminate its session with a BGP
// neighbor, and it sends a NOTIFICATION message with the Error Code 'Cease'
// and Error Subcode 'Administrative Shutdown' or 'Administrative Reset' ...,
// it MAY include a UTF-8-encoded string."
func (n notificationData) isShutdown() bool {
	if n.code != notifyCodeCease {
		return false
	}
	return n.subcode == notifySubcodeAdminShutdown || n.subcode == notifySubcodeAdminReset
}

// exabgpHexString is ExaBGP's hexstring: "0x" and the octets in uppercase hex.
func exabgpHexString(raw []byte) string {
	return "0x" + strings.ToUpper(hex.EncodeToString(raw))
}

// wire is `data` for an API 6 helper: the field as on the wire.
func (n notificationData) wire() string {
	if n.undecodable != "" {
		return n.undecodable
	}
	return exabgpHexString(n.raw)
}

// v4Data is `data` for an API 4 helper, JSON and text alike: hexstring of
// Notification.v4_data, what 5.x hexed. That is the Shutdown Communication
// decoded, else the field as on the wire.
func (n notificationData) v4Data() string {
	if n.undecodable != "" {
		return n.undecodable
	}
	if !n.isShutdown() {
		return exabgpHexString(n.raw)
	}
	return exabgpHexString([]byte(n.shutdownText()))
}

// readable is `message`, Notification.text: the Data field made readable, a
// Shutdown Communication decoded, printable ASCII as it is, anything else in
// hex so a control character the peer chose reaches no terminal.
func (n notificationData) readable() string {
	if n.undecodable != "" {
		return n.undecodable
	}
	if n.isShutdown() {
		return n.shutdownText()
	}
	for _, octet := range n.raw {
		if octet < printableFirst || octet > printableLast {
			return exabgpHexString(n.raw)
		}
	}
	return string(n.raw)
}

// shutdownText is Notification.v4_data for a Shutdown Communication, decoded.
//
// RFC 9003 Section 2: the Data field is a Length (1 octet) and that many
// octets of UTF-8 Shutdown Communication.
//
//	+---------+---------------------------+
//	| Length  | Shutdown Communication ...|
//	+---------+---------------------------+
//	 offset 0   offset 1
func (n notificationData) shutdownText() string {
	if len(n.raw) == 0 {
		// The shutdown without a communication, the old way.
		return ""
	}
	length := int(n.raw[0])
	payload := n.raw[1:]
	if length == 0 {
		return "empty Shutdown Communication."
	}
	if len(payload) < length {
		return "invalid Shutdown Communication (buffer underrun) length : " +
			strconv.Itoa(length) + " [" + exabgpHexString(payload) + "]"
	}
	message := payload[:length]
	if !utf8.Valid(message) {
		return "invalid Shutdown Communication (invalid UTF-8) length : " +
			strconv.Itoa(length) + " [" + exabgpHexString(payload) + "]"
	}
	// Shown, not interpreted: a character Python's isprintable refuses becomes
	// a space, as v4_data does.
	shown := strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return ' '
	}, string(message))
	text := `Shutdown Communication: "` + shown + `"`
	if trailer := payload[length:]; len(trailer) > 0 {
		text += ", trailing data: " + exabgpHexString(trailer)
	}
	return text
}

// notificationJSON renders the `notification` member as ExaBGP's
// JSON.notification writes it: `data` the wire for API 6 and v4_data for
// API 4, `message` the readable form for both.
func (e Event) notificationJSON(format Format) map[string]any {
	data := e.readNotificationData()
	field := data.wire()
	if format == FormatV4JSON {
		field = data.v4Data()
	}
	return map[string]any{
		"code":    e.Data["code"],
		"subcode": e.Data["subcode"],
		"data":    field,
		"message": data.readable(),
	}
}
