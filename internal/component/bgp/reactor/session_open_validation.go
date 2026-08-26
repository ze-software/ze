// Design: docs/architecture/core-design.md — BGP OPEN message validation
// Overview: session.go — BGP session struct and lifecycle
// Related: session_handlers.go — the handleOpen rail that calls these validators
// Related: session_connection.go — the collision-winner processOpen rail
// RFC: rfc/short/rfc6286.md — AS-wide unique BGP Identifier

package reactor

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// validateOpenIdentifier enforces RFC 6286 Section 2.2 on a received OPEN.
//
// RFC 6286 Section 2.2: "If the BGP Identifier field of the OPEN message is zero, or if it is the
// same as the BGP Identifier of the local BGP speaker and the message is from an internal peer,
// then the Error Subcode is set to 'Bad BGP Identifier'."
//
// "Internal peer" is the same test the RFC 7606 path uses for iBGP (LocalAS == PeerAS), preferring
// the CONFIGURED peer AS because the two-octet My AS carries AS_TRANS for a 4-byte AS (RFC 6793)
// and would misjudge exactly the speakers most likely to be renumbering.
//
// A DYNAMIC peer has no configured AS here: buildDynamicPeerSettings sets PeerAS to 0 and
// resolveDynamicPeerSettings only fills it at establishment, long after this runs. Reading that 0
// as the peer's AS made `internal` false for every dynamic peer against any real LocalAS, so the
// second MUST of Section 2.2 -- reject this speaker's OWN identifier from an internal peer -- was
// never enforced on that rail: a genuine iBGP dynamic peer presenting our identifier was ACCEPTED.
// The zero value silently selected the permissive branch (ai/rules/evidence.md). So when
// the configured AS is absent, fall back to the AS the peer advertises, read through the AS4
// capability rather than My AS so a 4-byte-AS peer is judged on its real ASN and not on AS_TRANS.
//
// Both OPEN rails (handleOpen and processOpen) call this; on rejection it sends the NOTIFICATION,
// logs the FSM error event, and closes the connection, so no caller can skip the mandated report.
func (s *Session) validateOpenIdentifier(open *message.Open) error {
	peerAS := s.settings.PeerAS
	if peerAS == 0 {
		peerAS = openAdvertisedAS(open)
	}
	internal := s.settings.LocalAS == peerAS
	err := open.ValidateBGPIdentifier(s.settings.RouterID, internal)
	if err == nil {
		return nil
	}

	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if notif, ok := errors.AsType[*message.Notification](err); ok {
		s.logNotifyErr(conn, notif.ErrorCode, notif.ErrorSubcode, notif.Data)
	}
	s.logFSMEvent(fsm.EventBGPOpenMsgErr)
	s.closeConn()

	sessionLogger().Warn("RFC 6286 Section 2.2: bad BGP Identifier in OPEN",
		"peer", s.settings.Address,
		"bgp-identifier", open.RouterID(),
		"local-identifier", localRouterIDString(s.settings.RouterID),
		"internal-peer", internal)

	return fmt.Errorf("%w: bgp identifier %s", ErrBadBGPIdentifier, open.RouterID())
}

// localRouterIDString renders this speaker's BGP Identifier for a log line.
func localRouterIDString(id uint32) string {
	return netip.AddrFrom4([4]byte{byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id)}).String()
}

// runOpenValidator invokes the peer-level OPEN validator (plugin validation such as RFC 9234 Role,
// plus the RFC 6286 Section 2.1 AS-wide identifier claim) and enforces its verdict.
//
// Shared by both OPEN rails. processOpen -- the rail a connection takes after WINNING collision
// resolution -- used to skip the validator entirely, so any per-peer OPEN policy could be bypassed
// by arriving as the second connection.
func (s *Session) runOpenValidator(open *message.Open) error {
	s.mu.RLock()
	localOpen := s.localOpen
	s.mu.RUnlock()

	if s.openValidator == nil || localOpen == nil {
		return nil
	}

	// Use peer name for plugin lookup (plugins key by name from config).
	peerID := s.settings.Name
	if peerID == "" {
		peerID = s.settings.Address.String()
	}

	err := s.openValidator(peerID, localOpen, open)
	if err == nil {
		return nil
	}

	s.mu.RLock()
	valConn := s.conn
	s.mu.RUnlock()

	// Check for OpenValidationError with specific NOTIFICATION codes.
	var valErr interface{ NotifyCodes() (uint8, uint8) }
	notifyCode := message.NotifyOpenMessage
	notifySubcode := message.NotifyOpenRoleMismatch
	if errors.As(err, &valErr) {
		code, sub := valErr.NotifyCodes()
		notifyCode = message.NotifyErrorCode(code)
		notifySubcode = sub
	}

	s.logNotifyErr(valConn, notifyCode, notifySubcode, nil)
	return fmt.Errorf("open validation failed: %w", err)
}
