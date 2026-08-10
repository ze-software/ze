// Design: docs/architecture/core-design.md — config reload delivers changed peer settings
// RFC: rfc/short/rfc5492.md — capability negotiation is the intersection of two OPENs
// Related: session_negotiate.go — buildOpen, the ONE producer of ze's OPEN
// Related: peer_settings_apply.go — the swap-or-restart decision this feeds
package reactor

import (
	"reflect"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// setConfigCapabilityGetter sets the callback buildOpen uses to read the
// configured capabilities under the Peer's lock. Called by Peer at session
// creation (peer_run.go). See the configCapGetter field on Session.
func (s *Session) setConfigCapabilityGetter(getter func() []capability.Capability) {
	s.configCapGetter = getter
}

// negotiationOutcomeUnchanged reports whether the capability set in next would
// produce the SAME negotiated result as the OPEN this session actually sent.
//
// It implements the owner's ruling of 2026-08-07, quoted verbatim in
// plan/spec-bgp-peer-settings-reload-ignored.md: "if capabilities are removed
// from the peer which were not used or if added when the other peer does not use
// them, ie: if the resulting negotiation would be similar and lead to the same
// encoding and same families, we can accept the change and keep the BGP session
// up, otherwise have to re-start and re-negotiate".
//
// The ruling is a procedure, not a list. The test is not "did a capability
// change" but "would the negotiation come out the same", so it is answered by
// running the negotiation, not by classifying fields:
//
//  1. The BASELINE is the OPEN ze really sent, s.localOpen, not an OPEN rebuilt
//     from the settings ze believes it sent. What the peer negotiated against is
//     a fact on the wire; a rebuild is a claim about it.
//  2. The CANDIDATE is buildOpen under next, the same producer sendOpen uses, so
//     the decision can never be taken against an OPEN ze would not send.
//  3. Both are negotiated against the capabilities the peer really advertised,
//     parsed from s.peerOpen, which is RFC 5492 Section 4's intersection run
//     twice over one unchanged remote side.
//
// FAIL-CLOSED (ai/rules/evidence.md). Every path that cannot PROVE the outcome
// identical returns false, which restarts: no session, no OPEN exchanged, an
// OPEN that does not parse, a changed OPEN header. A wrong restart costs one
// reconverge and announces itself; a wrong swap leaves the session encoding one
// thing while the config says another, and nothing reports it.
//
// The OPEN header is compared as well as the negotiated result because the
// header is what the peer keys its own state off: MyAS and ASN4 (RFC 4271
// Section 4.2, RFC 6793), the BGP Identifier (RFC 4271 Section 6.8 collision
// detection, RFC 4456 route reflection) and the Hold Time (RFC 4271 Section 4.2)
// all leave the negotiated struct unchanged on ze's side while changing what the
// peer computed. Negotiated equality alone would wave those through.
func (s *Session) negotiationOutcomeUnchanged(next *PeerSettings) bool {
	if s == nil || next == nil {
		return false
	}

	s.mu.RLock()
	localOpen := s.localOpen
	peerOpen := s.peerOpen
	negotiated := s.negotiated
	s.mu.RUnlock()

	// No OPEN pair means no negotiation to preserve. A peer that is idle,
	// connecting or still in OpenSent restarts for free, so restart.
	if localOpen == nil || peerOpen == nil || negotiated == nil {
		return false
	}

	baseCaps, err := capability.ParseFromOptionalParams(localOpen.OptionalParams)
	if err != nil {
		return false
	}
	peerCaps, err := capability.ParseFromOptionalParams(peerOpen.OptionalParams)
	if err != nil {
		return false
	}

	nextOpen := s.buildOpen(next, next.Capabilities)
	if !openHeaderEqual(localOpen, nextOpen) {
		return false
	}
	nextCaps, err := capability.ParseFromOptionalParams(nextOpen.OptionalParams)
	if err != nil {
		return false
	}

	base := capability.Negotiate(baseCaps, peerCaps, localOpen.ASN4, peerOpen.ASN4)
	candidate := capability.Negotiate(nextCaps, peerCaps, nextOpen.ASN4, peerOpen.ASN4)
	return negotiatedOutcomeEqual(base, candidate)
}

// openHeaderEqual compares the fixed OPEN fields of two OPEN messages, ignoring
// the optional parameters that carry the capabilities.
//
// The capabilities are excluded on purpose: they are exactly what the ruling
// allows to differ, provided the negotiation over them lands in the same place.
// Everything else in the OPEN reaches the peer unmediated by any negotiation, so
// a difference there is a difference the peer can see.
//
// The comparison is DERIVED, for the same reason negotiatedOutcomeEqual is: copy
// both, neutralize the one excluded member, DeepEqual the rest. A hand-picked
// field list was total over message.Open when it was written, and a field added
// to that struct tomorrow would escape it in the FAIL-OPEN direction -- the probe
// would report the header unchanged, the swap would be taken, and the session
// would keep running under a negotiation the peer never agreed to. Deriving the
// list covers a new field by construction. message.Open holds only scalars and
// the OptionalParams slice, so both the copy and DeepEqual are well-defined.
//
// A nil side is not a comparison, and it answers false rather than panicking:
// buildOpen is the candidate's producer and the fail-closed direction is the one
// this whole probe is built on.
func openHeaderEqual(a, b *message.Open) bool {
	if a == nil || b == nil {
		return false
	}
	ac, bc := *a, *b
	ac.OptionalParams, bc.OptionalParams = nil, nil
	return reflect.DeepEqual(&ac, &bc)
}

// negotiatedOutcomeEqual reports whether two negotiation results agree on
// everything that governs the running session: the same encoding, the same
// families, the same session-level capabilities and the same peer identity.
//
// It compares the DERIVED sub-components rather than a hand-picked field list.
// Negotiated.buildSubComponents (internal/core/bgp/capability/negotiated.go)
// assembles Encoding, Session and Identity from every negotiated value, with the
// family slice sorted and the maps copied, so DeepEqual over them is total and a
// capability negotiated in a future release is covered without an edit here.
//
// Two members of SessionCaps are excluded, each for a stated reason:
//
//   - Mismatches is RFC 5492 Section 3 reporting, not negotiated state. Removing
//     a capability the peer never had removes its mismatch entry, so including
//     it would make the ruling's first example -- a capability removed that the
//     peer never used -- always compare unequal, and the procedure vacuous.
//   - HoldTime is 0 in both: Negotiate never sets it, negotiateWith does
//     (session_negotiate.go), from the OPEN hold times. openHeaderEqual covers
//     its input.
func negotiatedOutcomeEqual(a, b *capability.Negotiated) bool {
	if a == nil || b == nil {
		return false
	}
	if !reflect.DeepEqual(a.Encoding, b.Encoding) {
		return false
	}
	if !reflect.DeepEqual(a.Identity, b.Identity) {
		return false
	}
	if a.Session == nil || b.Session == nil {
		return false
	}
	as, bs := *a.Session, *b.Session
	as.Mismatches, bs.Mismatches = nil, nil
	as.HoldTime, bs.HoldTime = 0, 0
	return reflect.DeepEqual(&as, &bs)
}

// negotiatedCapabilitySettings copies the fields a running session can take a new
// value for ONLY when negotiationOutcomeUnchanged has proved the negotiation
// lands in the same place.
//
// Capabilities is the whole set, and the reason it qualifies is read at its
// consumers rather than assumed. Its three readers are buildOpen, which runs once
// per connection (session_negotiate.go), GetPeerCapabilityConfigs (reactor_api.go)
// and validatePeerFamilies (reactor.go), which answer on demand; all three now go
// through the p.mu-guarded Peer.ConfiguredCapabilities. Nothing in an established
// session caches it, so the new set governs the next OPEN while the running
// session keeps the negotiation the probe proved unchanged.
//
// CapabilityConfigJSON and RawCapabilityConfig are NOT here, and that is not an
// oversight. They carry the peer's capability block to PLUGINS
// (parseCapabilitiesFromTree, config_capabilities.go), and a plugin's own
// capabilities enter the OPEN through pluginCapGetter, which reads the injector
// store the plugin has already written (Server.GetPluginCapabilitiesForPeer).
// The probe builds its candidate OPEN from that same store, so it sees what the
// plugin holds NOW and cannot see what the plugin would inject once it received
// the new config. An edit whose wire effect arrives later, by a path the probe
// does not run, is the one thing the procedure cannot determine, so it restarts
// (ai/rules/evidence.md).
func negotiatedCapabilitySettings(dst, src *PeerSettings) {
	dst.Capabilities = src.Capabilities
}

// hotSwappableWithCapabilities is the swap set when the negotiation is proved
// unchanged: the always-swappable fields plus the capability set.
//
// It exists so that ONE function is both neutralized in the restart decision and
// applied to the running peer, which is the invariant that keeps a field from
// being judged swappable without being delivered (hotSwappableSettings,
// peer_settings_apply.go).
func hotSwappableWithCapabilities(dst, src *PeerSettings) {
	hotSwappableSettings(dst, src)
	negotiatedCapabilitySettings(dst, src)
}
