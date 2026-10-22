// Design: docs/architecture/exabgp-bridge.md -- the reason of a `down` state event
// Related: bridge_event.go -- ExabgpJSON writes the reason this file names
// Related: bridge_event_text.go -- appendStateText writes it on the text line

package bridge

import "github.com/ze-software/ze/pkg/plugin/rpc"

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
//   - A NOTIFICATION is `notification sent (code,subcode)` or `notification
//     received (code,subcode)` in ExaBGP. Ze's down states neither the
//     direction nor the codes, which the NOTIFICATION event before it carries,
//     so the reason names the NOTIFICATION alone.
//
// It was absent until 2026-10-22: the bridge wrote ze's own word
// (`tcp-failure`) on the text line, and no reason at all on the JSON line.
var exabgpDownReasons = map[string]string{
	rpc.ReasonTCPFailure:   "closing connection",
	rpc.ReasonLocalClose:   "closing connection",
	rpc.ReasonPeerRemoved:  "stop, message [removed]",
	rpc.ReasonNotification: "notification",
}

// exabgpDownReason answers ExaBGP's words for the reason ze closed a session.
// The reason set is open (a plugin API value), so a reason this table does not
// name is written as ze stated it rather than dropped: the helper still reads
// why the session went down.
func exabgpDownReason(reason string) string {
	if words, known := exabgpDownReasons[reason]; known {
		return words
	}
	return reason
}
