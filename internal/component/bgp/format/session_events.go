// Design: docs/architecture/api/json-format.md -- the connected and fsm events, and the closing NOTIFICATION of a down
// Related: text_json.go -- appendStateChangeJSON writes the closing NOTIFICATION
// Related: text_human.go -- appendStateChangeText writes it on the text line

package format

import (
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// AppendConnected appends a `connected` event to buf: the peer's TCP
// connection is established and no OPEN has been exchanged on it yet.
//
// RFC 4271 Section 8.2.2: "If the TCP connection succeeds (Event 16 or
// Event 17), the local system checks the DelayOpen attribute prior to
// processing."
//
// JSON: {"type":"bgp","bgp":{"message":{"type":"connected"},"peer":{...}}}.
// Text: peer <address> remote as <asn> connected.
func AppendConnected(buf []byte, peer *plugin.PeerInfo, encoding string) []byte {
	if encoding == plugin.EncodingJSON {
		buf = append(buf, `{"type":"bgp","bgp":{"message":{"type":"connected"},`...)
		buf = appendPeerJSON(buf, peer)
		buf = append(buf, `}}`...)
		return append(buf, '\n')
	}
	buf = appendPeerTextPrefix(buf, peer)
	buf = append(buf, " connected"...)
	return append(buf, '\n')
}

// AppendFSM appends an `fsm` event to buf: the peer's FSM moved from one
// RFC 4271 Section 8.2.2 state to another. The names are fsm.State's
// (IDLE, CONNECT, ACTIVE, OPENSENT, OPENCONFIRM, ESTABLISHED).
//
// JSON: {"type":"bgp","bgp":{"message":{"type":"fsm"},"peer":{...},"fsm":{"from":"CONNECT","to":"OPENSENT"}}}.
// Text: peer <address> remote as <asn> fsm <from> <to>.
func AppendFSM(buf []byte, peer *plugin.PeerInfo, from, to fsm.State, encoding string) []byte {
	if encoding == plugin.EncodingJSON {
		buf = append(buf, `{"type":"bgp","bgp":{"message":{"type":"fsm"},`...)
		buf = appendPeerJSON(buf, peer)
		buf = append(buf, `,"fsm":{"from":"`...)
		buf = append(buf, from.String()...)
		buf = append(buf, `","to":"`...)
		buf = append(buf, to.String()...)
		buf = append(buf, `"}}}`...)
		return append(buf, '\n')
	}
	buf = appendPeerTextPrefix(buf, peer)
	buf = append(buf, " fsm "...)
	buf = append(buf, from.String()...)
	buf = append(buf, ' ')
	buf = append(buf, to.String()...)
	return append(buf, '\n')
}

// AppendSignal appends a `signal` event to buf: the daemon took the
// operating-system signal number, whose C name is name ("SIGHUP"). It is told
// per peer, as ExaBGP tells it (5.0.1 reactor/loop.py, `_signalled`).
//
// JSON: {"type":"bgp","bgp":{"message":{"type":"signal"},"peer":{...},"signal":{"code":1,"name":"SIGHUP"}}}.
// Text: peer <address> remote as <asn> signal <code> <name>.
//
//nolint:misspell // `_signalled` is ExaBGP's own identifier in reactor/loop.py, quoted as spelled
func AppendSignal(buf []byte, peer *plugin.PeerInfo, number int, name, encoding string) []byte {
	if encoding == plugin.EncodingJSON {
		buf = append(buf, `{"type":"bgp","bgp":{"message":{"type":"signal"},`...)
		buf = appendPeerJSON(buf, peer)
		buf = append(buf, `,"signal":{"code":`...)
		buf = strconv.AppendInt(buf, int64(number), 10)
		buf = append(buf, `,"name":"`...)
		buf = append(buf, name...)
		buf = append(buf, `"}}}`...)
		return append(buf, '\n')
	}
	buf = appendPeerTextPrefix(buf, peer)
	buf = append(buf, " signal "...)
	buf = strconv.AppendInt(buf, int64(number), 10)
	buf = append(buf, ' ')
	buf = append(buf, name...)
	return append(buf, '\n')
}

// appendPeerTextPrefix writes `peer <address> remote as <asn>`, the start of
// every per-peer text event line.
func appendPeerTextPrefix(buf []byte, peer *plugin.PeerInfo) []byte {
	buf = append(buf, "peer "...)
	buf = peer.Address.AppendTo(buf)
	buf = append(buf, " remote as "...)
	return appendRemoteASText(buf, peer.PeerAS)
}

// appendCloseNotificationJSON writes the NOTIFICATION that closed a session
// as a member of the down event, `"notification":{"direction":"sent",
// "code":6,"subcode":4}`, and nothing for a state change no NOTIFICATION
// closed (Direction NotifNone).
func appendCloseNotificationJSON(buf []byte, closing plugin.CloseNotification) []byte {
	if closing.Direction == plugin.NotifNone {
		return buf
	}
	buf = append(buf, `,"notification":{"direction":"`...)
	buf = append(buf, closing.Direction.String()...)
	buf = append(buf, `","code":`...)
	buf = strconv.AppendUint(buf, uint64(closing.Code), 10)
	buf = append(buf, `,"subcode":`...)
	buf = strconv.AppendUint(buf, uint64(closing.Subcode), 10)
	return append(buf, '}')
}

// appendCloseNotificationText writes ` notification <direction> <code>
// <subcode>` on a down text line, and nothing when no NOTIFICATION closed it.
func appendCloseNotificationText(buf []byte, closing plugin.CloseNotification) []byte {
	if closing.Direction == plugin.NotifNone {
		return buf
	}
	buf = append(buf, " notification "...)
	buf = append(buf, closing.Direction.String()...)
	buf = append(buf, ' ')
	buf = strconv.AppendUint(buf, uint64(closing.Code), 10)
	buf = append(buf, ' ')
	return strconv.AppendUint(buf, uint64(closing.Subcode), 10)
}

// appendRemoteASText writes the AS of a text line's `remote as <asn>`, or
// `unknown` while a dynamic peer's AS is not yet negotiated. The word keeps
// the line's token positions, so a positional parser still finds what follows.
func appendRemoteASText(buf []byte, peerAS rpc.RemoteAS) []byte {
	asn, known := peerAS.ASN()
	if !known {
		return append(buf, "unknown"...)
	}
	return strconv.AppendUint(buf, uint64(asn), 10)
}
