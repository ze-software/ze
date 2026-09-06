// Design: docs/architecture/behavior/fsm-established.md — OPEN acceptance
// Overview: session.go — BGP session struct and lifecycle
// Related: session_open_validation.go — the RFC 6286 identifier validator on the same rails
// Related: session_handlers.go — handleOpen, the first rail that calls this
// Related: session_connection.go — processOpen, the collision-winner rail
// Related: session_as_migration.go — peerASAccepted, which ASNs this session accepts
// RFC: rfc/short/rfc7607.md — AS 0 is reserved and never appears on the wire
// RFC: rfc/short/rfc4271.md — OPEN Message Error, Bad Peer AS is subcode 2
// RFC: rfc/short/rfc7705.md — one iBGP session under either of two ASNs

package reactor

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// validateOpenPeerAS enforces RFC 4271 Section 6.2 and RFC 7607 Section 2 on a received
// OPEN: the AS the peer presents must be an AS this session is configured for, and it must
// not be zero. Both answers are OPEN Message Error / Bad Peer AS (subcode 2).
//
// RFC 7607 Section 2: "If a BGP speaker receives zero as the peer AS in an OPEN message,
// it MUST abort the connection and send a NOTIFICATION with Error Code 'OPEN Message
// Error' and subcode 'Bad Peer AS' (see Section 6 of [RFC4271])."
//
// The zero test is about the AS the PEER puts on the wire. It is not about ze's own AS 0
// sentinel, which several internal surfaces use to mean "not known yet" and which never
// leaves the process: a dynamic peer carries PeerAS 0 until resolveDynamicPeerSettings
// fills it at establishment (reactor_dynamic.go, peer_run.go), and collisionPeerAS and
// validateOpenIdentifier both read that 0 as absence rather than as an AS. Reading the
// wire field is what keeps the two apart, so a dynamic peer announcing a real AS is never
// refused as an AS 0 claim.
//
// The MISMATCH test below does read s.settings.PeerAS, and that same sentinel is why it
// carries an explicit dynamic-peer branch rather than a comparison against 0
// (peerASAccepted, session_as_migration.go).
//
// Both OPEN rails call it. On rejection it sends the NOTIFICATION, logs the FSM error
// event and closes the connection, so no caller can accept a session ze must abort.
func (s *Session) validateOpenPeerAS(open *message.Open) error {
	if openClaimsASZero(open) {
		s.rejectOpenPeerAS()
		sessionLogger().Warn("RFC 7607 Section 2: peer AS is zero in OPEN",
			"peer", s.settings.Address,
			"my-as", open.MyAS,
			"effect", "the connection is aborted with NOTIFICATION 2/2 Bad Peer AS")
		return ErrBadPeerAS
	}

	// RFC 4271 Section 6.2: "If the Autonomous System field of the OPEN message is
	// unacceptable, then the Error Subcode MUST be set to Bad Peer AS."
	//
	// Ze accepted an OPEN from ANY AS until this check existed, so a peer whose remote-as
	// was mistyped established, and every AS-scoped decision after it -- the eBGP prepend,
	// the RFC 6286 identifier scope, the RFC 4456 reflection rules -- ran against an AS
	// the peer never claimed. peerASAccepted (session_as_migration.go) holds which ASNs
	// are acceptable, including the RFC 7705 Section 4.2 pair, so the exception a
	// migrating session needs is a rule rather than the absence of one.
	advertised := openAdvertisedAS(open)
	if s.settings.peerASAccepted(advertised) {
		return nil
	}

	s.rejectOpenPeerAS()
	sessionLogger().Warn("RFC 4271 Section 6.2: peer AS in OPEN is not an AS this peer is configured for",
		"peer", s.settings.Address,
		"advertised-as", advertised,
		"configured-as", s.settings.PeerAS,
		"local-as", s.settings.LocalAS,
		"migration-as", s.settings.MigrationAS,
		"effect", "the connection is aborted with NOTIFICATION 2/2 Bad Peer AS")

	return fmt.Errorf("%w: advertised %d", ErrPeerASMismatch, advertised)
}

// rejectOpenPeerAS reports OPEN Message Error / Bad Peer AS to the peer and tears the
// connection down. Both refusals above share it, so neither can report the verdict and
// leave the connection up.
//
// RFC 4271 Section 6.2 defines no Data field for this subcode, so none is sent.
func (s *Session) rejectOpenPeerAS() {
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	s.logNotifyErr(conn, message.NotifyOpenMessage, message.NotifyOpenBadPeerAS, nil)
	s.logFSMEvent(fsm.EventBGPOpenMsgErr)
	s.closeConn()
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
