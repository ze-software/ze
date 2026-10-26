// Design: docs/architecture/exabgp-bridge.md -- the reason of a `down` state event
// Related: bridge_event.go -- exabgpJSON writes the reason this file names
// Related: bridge_event_text.go -- appendStateText writes it on the text line

package bridge

import (
	"strconv"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// exabgpDownReasons maps the reason ze closes a session for (the rpc.Reason*
// set the reactor derives, internal/component/bgp/reactor/peer_close.go,
// sessionCloseFor, and reactor_peers.go for a removal) to the words ExaBGP's
// reactor hands its encoders for the same close.
//
// ExaBGP's `reason` is "the text ExaBGP would log for the same event" (wiki,
// API/JSON-API-Reference.md, "Session DOWN"): Peer._close passes its message
// to Processes.down (src/exabgp/reactor/peer/peer.py), and each caller names
// the close.
//
//   - A transport that failed is `self._reset('closing connection', network)`,
//     the NetworkError branch of Peer._run.
//   - A peer removed from the configuration is `self._stop('removed', ...)`,
//     which closes with `f'stop, message [{message}]'`.
//   - A session ze closed itself with no NOTIFICATION reaching the peer is the
//     same transport close to the helper: ExaBGP reports a connection it drops
//     without a NOTIFICATION through the NetworkError branch too.
//   - A session whose received UPDATEs ze could not publish, when its Cease
//     never reached the peer, is that same close: no NOTIFICATION crossed the
//     wire, so the helper reads `closing connection`.
//   - A NOTIFICATION is `notification sent (code,subcode)` or `notification
//     received (code,subcode)` in ExaBGP (`self._reset(f'notification sent
//     ({notify.code},{notify.subcode})', notify)` in Peer._run). Ze's down
//     carries the closing NOTIFICATION's direction and codes under
//     `notification` (format.appendCloseNotificationJSON), and
//     exabgpDownReason writes them. The table's word stands only for a down
//     that names no direction.
//
// It was absent until 2026-10-22: the bridge wrote ze's own word
// (`tcp-failure`) on the text line, and no reason at all on the JSON line.
//
// exabgpTransportClose is ExaBGP's word for a session that ended with no
// NOTIFICATION on the wire, whichever side dropped the transport.
const exabgpTransportClose = "closing connection"

var exabgpDownReasons = map[string]string{
	rpc.ReasonTCPFailure:      exabgpTransportClose,
	rpc.ReasonLocalClose:      exabgpTransportClose,
	rpc.ReasonAdjRIBInInvalid: exabgpTransportClose,
	rpc.ReasonPeerRemoved:     "stop, message [removed]",
	rpc.ReasonNotification:    "notification",
}

// exabgpDownReason answers ExaBGP's words for the reason ze closed a session.
// closing is the down event's `notification` member, which ze writes only for
// a session a NOTIFICATION closed: `{"direction":"sent","code":6,"subcode":4}`.
// The reason set is open (a plugin API value), so a reason this table does not
// name is written as ze stated it rather than dropped: the helper still reads
// why the session went down.
func exabgpDownReason(reason string, closing any) string {
	if words, named := closingNotificationWords(closing); named {
		return words
	}
	if words, known := exabgpDownReasons[reason]; known {
		return words
	}
	return reason
}

// closingNotificationWords writes `notification sent (6,4)` or
// `notification received (4,0)` from the down event's `notification` member,
// and answers false when the member is absent or names no direction ExaBGP
// writes.
func closingNotificationWords(closing any) (string, bool) {
	fields, isObject := closing.(map[string]any)
	if !isObject {
		return "", false
	}
	direction, _ := fields["direction"].(string)
	if direction != "sent" && direction != "received" {
		return "", false
	}
	code, hasCode := fields["code"].(float64)
	if !hasCode {
		return "", false
	}
	subcode, hasSubcode := fields["subcode"].(float64)
	if !hasSubcode {
		return "", false
	}
	buf := make([]byte, 0, 32)
	buf = append(buf, "notification "...)
	buf = append(buf, direction...)
	buf = append(buf, " ("...)
	buf = strconv.AppendUint(buf, uint64(code), 10)
	buf = append(buf, ',')
	buf = strconv.AppendUint(buf, uint64(subcode), 10)
	buf = append(buf, ')')
	return string(buf), true
}
