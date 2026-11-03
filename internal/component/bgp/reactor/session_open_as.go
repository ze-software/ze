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
	"errors"
	"fmt"
	"math"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// validateOpenPeerAS parses peer capabilities before checking the advertised AS.
// Malformed capabilities retain their OPEN error instead of turning AS_TRANS
// into a peer-AS mismatch. It returns the parsed capabilities for negotiation.
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
func (s *Session) validateOpenPeerAS(open *message.Open) ([]capability.Capability, error) {
	advertised := uint32(open.MyAS)
	var caps []capability.Capability
	if advertised != 0 {
		var err error
		caps, err = capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)
		if err != nil {
			return nil, s.rejectOpenCapabilityError(err)
		}
		// RFC 6793 Section 4.1
		advertised = capability.OpenPeerAS(open.MyAS, caps, !s.settings.DisableASN4)
	}
	if advertised == 0 {
		s.rejectOpenPeerAS()
		sessionLogger().Warn("RFC 7607 Section 2: peer AS is zero in OPEN",
			"peer", s.settings.Address,
			"my-as", open.MyAS,
			"effect", "the connection is aborted with NOTIFICATION 2/2 Bad Peer AS")
		return nil, ErrBadPeerAS
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
	if s.settings.peerASAccepted(advertised) {
		return caps, nil
	}

	s.rejectOpenPeerAS()
	sessionLogger().Warn("RFC 4271 Section 6.2: peer AS in OPEN is not an AS this peer is configured for",
		"peer", s.settings.Address,
		"advertised-as", advertised,
		"configured-as", s.settings.PeerAS,
		"local-as", s.settings.LocalAS,
		"migration-as", s.settings.MigrationAS,
		"effect", "the connection is aborted with NOTIFICATION 2/2 Bad Peer AS",
		"fix", s.peerASMismatchFix(advertised))

	return nil, fmt.Errorf("%w: advertised %d", ErrPeerASMismatch, advertised)
}

// peerASMismatchFix names the config change that makes a refused peer AS acceptable, for
// the Bad Peer AS warning above (ai/rules/user-facing-errors.md).
//
// A session configured `asn4 disable` is the case an operator does not expect: it judges
// the peer by My AS (capability.OpenPeerAS), which a peer with a four-octet AS sends as AS_TRANS
// (RFC 6793 Section 4.2.1), so the four-octet AS the operator configured can never match.
func (s *Session) peerASMismatchFix(advertised uint32) string {
	if s.settings.DisableASN4 {
		return "this peer has capability { asn4 disable; }, so its AS is read from the two-octet My AS field" +
			" (AS_TRANS 23456 for a four-octet AS): set session asn remote to the AS the peer sends," +
			" or set capability { asn4 enable; }"
	}
	if advertised == uint32(message.AS_TRANS) {
		return "the peer sends AS_TRANS 23456 without a four-octet AS capability:" +
			" set session asn remote 23456, or have the peer advertise its four-octet AS"
	}
	return "check session asn remote on this peer against the peer's own local AS, and correct whichever is wrong"
}

// ErrASN4DisabledFourOctetRemote is the config refusal checkASN4DisabledReachable returns.
var ErrASN4DisabledFourOctetRemote = errors.New("capability asn4 disable reads the peer AS from the two-octet My AS field," +
	" which can never carry a four-octet AS: set session asn remote 23456 (AS_TRANS), or set capability { asn4 enable; }")

// checkASN4DisabledReachable refuses, at config load, a peer configured `asn4 disable`
// whose every acceptable peer AS is four-octet.
//
// Without the capability the session judges the peer by My AS (capability.OpenPeerAS, RFC 6793
// Section 4.1), and My AS is two octets, so a peer whose AS does not fit sends AS_TRANS
// there (Section 4.2.1). If no AS peerASAccepted takes fits two octets, every OPEN the
// peer can send is refused with Bad Peer AS and the session never establishes: a config
// ze cannot apply as written, refused rather than run (exact-or-reject).
//
// The candidates are the values peerASAccepted can accept, each asked of the rule itself
// so the two never disagree: the configured remote AS, the RFC 7705 pair, and AS_TRANS,
// which answers for a dynamic group (no configured remote AS, every AS accepted).
func checkASN4DisabledReachable(n *PeerSettings) error {
	if !n.DisableASN4 {
		return nil
	}
	for _, candidate := range [...]uint32{n.PeerAS, n.LocalAS, n.MigrationAS, uint32(message.AS_TRANS)} {
		if candidate == 0 {
			continue
		}
		if candidate > math.MaxUint16 {
			continue
		}
		if n.peerASAccepted(candidate) {
			return nil
		}
	}
	return fmt.Errorf("%w: remote %d", ErrASN4DisabledFourOctetRemote, n.PeerAS)
}

// judgedPeerAS returns the AS this session judges its peer by: the configured remote AS,
// or for a dynamic peer the AS open names under RFC 6793 Section 4.1 (sessionPeerAS,
// peer.go). It is the one place a Session states whether it advertises the Four-octet AS
// capability, so the collision tie-break, the RFC 6286 identifier scope, the negotiated
// identity and the reload comparison all read the AS validateOpenPeerAS accepted.
//
// Reads s.settings without s.mu: PeerAS and DisableASN4 are written before the session
// starts and, for a dynamic peer, by resolveDynamicPeerSettings after the last of these
// callers ran (reactor_dynamic.go).
func (s *Session) judgedPeerAS(open *message.Open) uint32 {
	return sessionPeerAS(s.settings.PeerAS, !s.settings.DisableASN4, open)
}

// rejectOpenPeerAS reports OPEN Message Error / Bad Peer AS to the peer and tears the
// connection down. Both refusals above share it, so neither can report the verdict and
// leave the connection up.
//
// RFC 4271 Section 6.2 defines no Data field for this subcode, so none is sent.
//
// It also counts the refusal in ze_bgp_open_rejected_bad_peer_as_total, so a peer whose
// configured AS is wrong shows on the metrics endpoint and not only in the log.
func (s *Session) rejectOpenPeerAS() {
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if s.prefixMetrics != nil {
		s.prefixMetrics.openBadPeerAS.With(s.addrLabel).Inc()
	}
	s.logNotifyErr(conn, message.NotifyOpenMessage, message.NotifyOpenBadPeerAS, nil)
	s.logFSMEvent(fsm.EventBGPOpenMsgErr)
	s.closeConn()
}
