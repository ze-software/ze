// Design: docs/architecture/behavior/fsm-established.md — OPEN acceptance
// Overview: session.go — BGP session struct and lifecycle
// Related: session_open_validation.go — the RFC 6286 identifier validator on the same rails
// Related: session_handlers.go — handleOpen, the first rail that calls this
// Related: session_connection.go — processOpen, the collision-winner rail
// RFC: rfc/short/rfc7607.md — AS 0 is reserved and never appears on the wire

package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// validateOpenPeerAS enforces RFC 7607 Section 2 on a received OPEN.
//
// RFC 7607 Section 2: "If a BGP speaker receives zero as the peer AS in an OPEN message,
// it MUST abort the connection and send a NOTIFICATION with Error Code 'OPEN Message
// Error' and subcode 'Bad Peer AS' (see Section 6 of [RFC4271])."
//
// This is about the AS the PEER puts on the wire. It is not about ze's own AS 0 sentinel,
// which several internal surfaces use to mean "not known yet" and which never leaves the
// process: a dynamic peer carries PeerAS 0 until resolveDynamicPeerSettings fills it at
// establishment (reactor_dynamic.go, peer_run.go), and collisionPeerAS and
// validateOpenIdentifier both read that 0 as absence rather than as an AS. Reading the
// wire field instead of s.settings.PeerAS is what keeps the two apart, so a dynamic peer
// announcing a real AS is unaffected by this check.
//
// Both OPEN rails call it. On rejection it sends the NOTIFICATION, logs the FSM error
// event and closes the connection, so no caller can accept a session ze must abort.
func (s *Session) validateOpenPeerAS(open *message.Open) error {
	if !openClaimsASZero(open) {
		return nil
	}

	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	// RFC 4271 Section 6.2 defines no Data field for this subcode, so none is sent.
	s.logNotifyErr(conn, message.NotifyOpenMessage, message.NotifyOpenBadPeerAS, nil)
	s.logFSMEvent(fsm.EventBGPOpenMsgErr)
	s.closeConn()

	sessionLogger().Warn("RFC 7607 Section 2: peer AS is zero in OPEN",
		"peer", s.settings.Address,
		"my-as", open.MyAS,
		"effect", "the connection is aborted with NOTIFICATION 2/2 Bad Peer AS")

	return ErrBadPeerAS
}

// openClaimsASZero reports whether a received OPEN presents AS 0 as the peer's AS.
//
// Two fields can carry it. The two-octet My Autonomous System field is the one RFC 4271
// Section 4.2 defines. A speaker with a four-octet AS puts AS_TRANS there and its real AS
// in the Four-octet AS capability instead (RFC 6793 Section 3), so a peer claiming AS 0
// through that capability is claiming it just as plainly, and both are refused.
//
// A capability list that does not parse reports no zero. What is wrong with such an OPEN
// is the encoding rather than the AS, and rejectOpenCapabilityError owns that verdict on
// both rails; answering it here would report the wrong subcode to the peer.
func openClaimsASZero(open *message.Open) bool {
	if open.MyAS == 0 {
		return true
	}

	caps, err := capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)
	if err != nil {
		return false
	}
	for _, entry := range caps {
		asn4, ok := entry.(*capability.ASN4)
		if !ok {
			continue
		}
		return asn4.ASN == 0
	}
	return false
}
