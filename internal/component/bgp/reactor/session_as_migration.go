// Design: docs/architecture/core-design.md — RFC 7705 Internal BGP AS Migration
// Overview: peer_settings.go — PeerSettings.MigrationAS, the leaf these rules read
// Related: session_open_as.go — the OPEN rail that consumes peerASAccepted
// Related: session_negotiate.go — the OPEN builder that consumes openLocalAS
// RFC: rfc/short/rfc7705.md — AS migration mechanisms

package reactor

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// The two configurations RFC 7705 Section 4.2 cannot describe, refused at config load so
// no session ever runs under one. Both are returned by setMigrationAS.
var (
	ErrMigrationASEqualsLocal = errors.New("session asn migration equals the local as, so the session has one as and not two")
	ErrMigrationASNotPeerAS   = errors.New("session asn remote names neither the local as nor the migration as")
)

// setMigrationAS records the RFC 7705 Section 4.2 migration ASN on a peer's settings, and
// refuses the two configurations under which the mechanism cannot mean what Section 4.2
// says it means.
//
// The section describes ONE session running under TWO of this speaker's AS numbers: "a BGP
// speaker MUST accept BGP OPEN and establish an iBGP session from configured iBGP peers if
// the ASN value in "My Autonomous System" is either the globally configured ASN or a
// locally configured ASN provided when this capability is utilized."
//
// A migration ASN equal to the local AS gives one number rather than two. It reads to an
// operator as the mechanism being on, and it widens nothing, so a peer whose OPEN they
// expect to be accepted is refused with no line of config to point at.
//
// A remote AS that is neither of the two describes an EXTERNAL peer, and Section 4.2 is
// about iBGP throughout: "the BGP speaker MUST treat UPDATEs sent and received to this
// peer as if this was a natively configured iBGP session". Refusing it here is what lets
// peerASAccepted widen to exactly the two ASNs Section 4.2 names and no third value.
//
// A dynamic group states no remote AS at all -- it arrives in the member's OPEN (RFC 4271
// Section 4.2), so PeerAS is 0 on the template -- and the second check does not apply to
// it. Such a group's members are checked against the two ASNs at OPEN instead.
func setMigrationAS(n *PeerSettings, migrationAS uint32) error {
	if migrationAS == 0 {
		return nil
	}
	if migrationAS == n.LocalAS {
		return fmt.Errorf("%w: as %d", ErrMigrationASEqualsLocal, migrationAS)
	}
	if n.PeerAS != 0 && n.PeerAS != n.LocalAS && n.PeerAS != migrationAS {
		return fmt.Errorf("%w: remote %d, local %d, migration %d",
			ErrMigrationASNotPeerAS, n.PeerAS, n.LocalAS, migrationAS)
	}

	n.MigrationAS = migrationAS
	return nil
}

// A router being renumbered runs two AS numbers at once, and RFC 7705 Section 4.2 lets
// ONE iBGP session be formed under either of them. Three questions follow from that, and
// this file answers all three so no caller re-derives one:
//
//   - which AS numbers a peer may present in its OPEN (peerASAccepted)
//   - whether the session is internal (isIBGPWith, and PeerSettings.IsIBGP over it)
//   - which AS this speaker puts in the OPEN it sends (openLocalAS)
//
// The first two read CONFIGURED values alone. Deriving the internal verdict from what the
// peer advertised would let a peer choose whether ze prepends its AS on the forward path,
// which is the route leak in RFC 7705 Section 6.

// isIBGPWith reports whether a session with the given peer AS is internal.
//
// The ordinary rule is one equality: a session is internal when the peer's AS is ours.
// RFC 7705 Section 4.2 adds the second ASN of a migrating router to "ours", and requires
// the session to be internal either way: "In each case, the BGP speaker MUST treat UPDATEs
// sent and received to this peer as if this was a natively configured iBGP session, as
// defined by [RFC4271] and [RFC4456]."
//
// This is the ONE rule. IsIBGP, IsEBGP, Peer.IsIBGP, Peer.IsEBGP, the RFC 7606 branch
// (session_validation.go), the RFC 6286 internal-peer test (session_open_validation.go)
// and the plugin peer-type report (reactor_api.go) all reach the verdict through it. They
// were four separate copies of the equality, and a migration session classified iBGP by
// one and eBGP by another sends a route with the wrong AS_PATH to a peer that trusts it.
func (n *PeerSettings) isIBGPWith(peerAS uint32) bool {
	if peerAS == n.LocalAS {
		return true
	}
	if n.MigrationAS == 0 {
		return false
	}
	return peerAS == n.MigrationAS
}

// peerASAccepted reports whether an AS a peer presented in its OPEN is one this session is
// configured to establish with. It is the test behind RFC 4271 Section 6.2 "Bad Peer AS",
// and the exception RFC 7705 Section 4.2 carves out of it.
//
// advertised is the peer's REAL AS, read through the AS4 capability by openAdvertisedAS
// (peer.go), so a four-octet peer is judged on its own ASN rather than on the AS_TRANS it
// is required to put in My Autonomous System (RFC 6793 Section 3).
//
// advertised is never zero here: openClaimsASZero refuses AS 0 on both rails before this
// runs (session_open_as.go, RFC 7607 Section 2). So no branch below has to decide what an
// absent AS means, and none of them treats zero as a pass.
func (n *PeerSettings) peerASAccepted(advertised uint32) bool {
	if n.MigrationAS != 0 {
		// RFC 7705 Section 4.2: "a BGP speaker MUST accept BGP OPEN and establish an iBGP
		// session from configured iBGP peers if the ASN value in "My Autonomous System"
		// is either the globally configured ASN or a locally configured ASN provided when
		// this capability is utilized."
		//
		// The widening is exactly those two values and no more, which is what keeps an
		// attacker-supplied AS from being accepted on a migrating session. PeerAS is not
		// consulted because the two named here already contain it: PeersFromConfigTree
		// refuses a config whose remote AS is neither (validateMigrationAS, config.go).
		return advertised == n.LocalAS || advertised == n.MigrationAS
	}

	if n.PeerAS == 0 {
		// A DYNAMIC peer states no remote AS in configuration. buildDynamicPeerSettings
		// leaves PeerAS 0 and resolveDynamicPeerSettings fills it at establishment, long
		// after this runs, so there is no configured AS to compare against and the check
		// does not apply to this peer.
		//
		// The exemption is its own branch rather than a comparison a zero happens to fail,
		// because the two need opposite repairs and read the same from a caller: without
		// it every dynamic peer is refused at OPEN (ai/rules/principles.md). A dynamic
		// group that DOES configure MigrationAS never reaches here, and is checked against
		// the two ASNs above.
		return true
	}

	return advertised == n.PeerAS
}

// noteASMigrationRejection records that the peer answered OPEN Message Error / Bad Peer
// AS, so the NEXT connection attempt opens with the other of this session's two AS
// numbers. It is the consuming half of RFC 7705 Section 4.2's deadlock avoidance, and
// validateOpenPeerAS is the originating half.
//
// The flag TOGGLES rather than latching. Latching would answer the deadlock the section
// names and create a second one: two speakers that both fell back would then both be
// stuck on the migration AS, each refusing the other, with no attempt left that pairs the
// two ASNs the other way. Alternating walks the whole space, which is two attempts.
//
// A session with no migration AS configured never toggles anything, so a Bad Peer AS from
// an ordinary peer is reported and changes no future OPEN. A session with no owning Peer
// has nowhere to record the decision and is likewise unaffected.
func (s *Session) noteASMigrationRejection(notif *message.Notification) {
	if s.settings.MigrationAS == 0 || s.asMigrationFallback == nil {
		return
	}
	if notif.ErrorCode != message.NotifyOpenMessage || notif.ErrorSubcode != message.NotifyOpenBadPeerAS {
		return
	}

	next := !s.asMigrationFallback.Load()
	s.asMigrationFallback.Store(next)

	sendAS := s.settings.LocalAS
	if next {
		sendAS = s.settings.MigrationAS
	}
	sessionLogger().Info("RFC 7705 Section 4.2: peer answered Bad Peer AS, opening with the other AS",
		"peer", s.settings.Address,
		"local-as", s.settings.LocalAS,
		"migration-as", s.settings.MigrationAS,
		"next-open-as", sendAS)
}

// openLocalAS returns the AS this speaker puts in the OPEN it is about to send: in My
// Autonomous System, in the Four-octet AS capability, and in the ASN4 field the encoder
// reads to choose between them. One resolution, so the header and the capability cannot
// disagree about which AS is in play.
//
// RFC 7705 Section 4.2: "To avoid potential deadlocks when two BGP speakers are attempting
// to establish a BGP peering session and are both configured with this mechanism, the
// speaker SHOULD send BGP OPEN using the globally configured ASN first, and only send a
// BGP OPEN using the locally configured ASN as a fallback if the remote neighbor responds
// with the BGP error "Bad Peer AS"."
//
// LocalAS is that "globally configured ASN" for this session, so it is what ze sends until
// a peer answers Bad Peer AS. The fallback then alternates between the two ASNs, one per
// connection attempt: it is the Peer's connect-retry timer that bounds the rate, and two
// ASNs is the whole space, so a peer refusing both cannot drive more reconnects than it
// could by refusing one (the denial-of-service question in this spec's Security Review).
//
// fallback is nil for a Session no Peer owns, which is the shape a test builds with
// NewSession. That is the initial state rather than a missing answer: nothing has answered
// Bad Peer AS to a session that has never connected, so the globally configured ASN is
// what Section 4.2 asks for.
func openLocalAS(settings *PeerSettings, fallback *atomic.Bool) uint32 {
	if settings.MigrationAS == 0 {
		return settings.LocalAS
	}
	if fallback == nil || !fallback.Load() {
		return settings.LocalAS
	}
	return settings.MigrationAS
}
