// Design: docs/architecture/api/json-format.md -- the raw member of a `full` event
// Related: json.go -- the payload of each message kind

package format

import (
	"encoding/hex"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// jsonKeyRaw is the member a `full` subscriber's event carries the wire bytes
// in, beside the parsed object. An UPDATE writes its own (appendFullFromResult).
const jsonKeyRaw = "raw"

// Message returns the JSON event of one OPEN, NOTIFICATION, KEEPALIVE or
// ROUTE-REFRESH message, or "" for any other type: an UPDATE has its own
// formatter (AppendMessage), and a type ze does not decode has no event.
//
// fmtMode is the subscriber's format. A `full` subscriber's event also carries
// the message body, the octets after the 19-octet header, in hexadecimal:
//
//	{"type":"bgp","bgp":{"message":{...},"peer":{...},"open":{...},"raw":{"message":"04fc..."}}}
//
// It is the member an UPDATE carries its bytes in (`raw.update`), under the
// name the `raw` format already gives a message that is not an UPDATE
// (appendNonUpdate). Until 2026-10-22 only an UPDATE carried its bytes, so the
// ExaBGP bridge could write none of ExaBGP's `packets` or `consolidate` lines
// for the other four kinds. A KEEPALIVE has no body, so its member is empty and
// still present: the absence of the member means the subscriber did not ask.
func (e *JSONEncoder) Message(peer *plugin.PeerInfo, msg bgptypes.RawMessage, fmtMode string) string {
	var payload map[string]any
	switch msg.Type {
	case msgtype.TypeOPEN:
		payload = e.openPayload(peer, DecodeOpen(msg.RawBytes), msg.Direction, msg.MessageID)
	case msgtype.TypeNOTIFICATION:
		payload = e.notificationPayload(peer, DecodeNotification(msg.RawBytes), msg.Direction, msg.MessageID)
	case msgtype.TypeKEEPALIVE:
		payload = e.keepalivePayload(peer, msg.Direction, msg.MessageID)
	case msgtype.TypeROUTEREFRESH:
		payload = e.routeRefreshPayload(peer, DecodeRouteRefresh(msg.RawBytes), msg.Direction, msg.MessageID)
	case msgtype.TypeUPDATE:
		// AppendMessage formats an UPDATE, its bytes included.
		return ""
	default:
		// Message types are an open wire set; ze has no event for this one.
		return ""
	}
	if fmtMode == plugin.FormatFull {
		payload[jsonKeyRaw] = map[string]any{jsonKeyMessage: hex.EncodeToString(msg.RawBytes)}
	}
	return e.marshal(payload)
}
