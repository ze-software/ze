// Design: docs/architecture/core-design.md — BGP capability negotiation
// Overview: session.go — BGP session struct and lifecycle
// Related: negotiated.go — NegotiatedCapabilities produced by negotiation

package reactor

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// capabilityCodeLLGR is the Long-Lived Graceful Restart Capability code. The
// capability package names no constant for it, because no core decoder exists:
// the GR plugin emits and reads its value. RFC 9494 Section 8: "IANA has
// assigned a value of 71 from the "Capability Codes" registry".
const capabilityCodeLLGR capability.Code = 71

// retainAdvertisedLLGR records whether our own OPEN carried the LLGR
// Capability. negotiateWith calls it, on the receive goroutine, so the ingress
// step is told before this session's first UPDATE is read; a session whose OPEN
// did not carry it forgets the previous session's answer.
//
// RFC 9494 Section 4.3: "A BGP speaker that has advertised the Long-Lived
// Graceful Restart Capability to a neighbor MUST perform the following upon
// receiving a route from that neighbor with the LLGR_STALE community".
func (s *Session) retainAdvertisedLLGR(localCaps []capability.Capability) {
	advertised := false
	for _, cap := range localCaps {
		if cap.Code() == capabilityCodeLLGR {
			advertised = true
			break
		}
	}
	s.localLLGR.Store(advertised)
}

// advertisedLLGR reports whether the OPEN this session sent carried the LLGR
// Capability. Safe for concurrent use: localLLGR is an atomic, so it takes no
// lock. The relay rail calls it for a source session from a goroutine that is
// not that session's reader, while the reactor lock is held, so it MUST NOT take
// s.mu: closeConn holds s.mu across a flush, a drain and Close, and a stuck
// source would then stall the reactor.
func (s *Session) advertisedLLGR() bool {
	return s.localLLGR.Load()
}

// negotiateWith performs capability negotiation using pre-parsed capabilities.
func (s *Session) negotiateWith(localCaps, peerCaps []capability.Capability) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.localOpen == nil || s.peerOpen == nil {
		return
	}

	// Negotiate. The peer's AS is resolved ONCE (sessionPeerAS, peer.go) and the internal
	// verdict comes from the one rule that owns it (isIBGPWith, session_as_migration.go),
	// because negotiation itself can read neither off the wire.
	//
	// RFC 6793 Section 4.1: "When a NEW BGP speaker processes an OPEN message from another
	// NEW BGP speaker, it MUST use the AS number encoded in the Capability Value field of
	// the 'support for four-octet AS number capability' in lieu of the 'My Autonomous
	// System' field of the OPEN message." openAdvertisedAS is where ze obeys it, and
	// sessionPeerAS reaches it for a peer with no configured AS.
	//
	// RFC 7705 Section 4.2 is the second reason: a renumbering speaker forms one iBGP
	// session under either of two configured ASNs, so the internal verdict is a question
	// about configuration rather than about the two numbers being equal.
	//
	// s.peerOpen.ASN4 used to be passed here as the peer's AS. UnpackOpen never populates
	// that field, so the identity carried PeerASN 0 on every established session, and every
	// reader of the encoding context's IsIBGP -- the commit rail's LOCAL_PREF and AS_PATH
	// decisions among them (rib/commit.go) -- took the eBGP arm for a genuine iBGP peer.
	peerAS := sessionPeerAS(s.settings.PeerAS, s.peerOpen)
	negotiated := capability.Negotiate(localCaps, peerCaps, capability.PeerIdentity{
		LocalASN: s.settings.LocalAS,
		PeerASN:  peerAS,
		Internal: s.settings.isIBGPWith(peerAS),
	})
	// Writers read ASN4 while holding writeMu, so publish the negotiated
	// pointer under that lock as well as mu. No writer needs mu in reverse.
	s.writeMu.Lock()
	s.negotiated = negotiated

	// RFC 9234 Section 4.1: Role is code(1), length(1), value(1); value 2
	// denotes the LOCAL RS-client. Retain our advertised role, not the peer's:
	// loose-mode sessions need the transparent-RS exception without negotiation.
	s.localRSClient = false
	for _, cap := range localCaps {
		if cap.Code() != capability.CodeRole || cap.Len() != 3 {
			continue
		}
		var role [3]byte
		cap.WriteTo(role[:], 0)
		s.localRSClient = role[2] == 2
		break
	}

	// RFC 9494 Section 4.3
	s.retainAdvertisedLLGR(localCaps)

	s.initPathsLimit(s.negotiated.Encoding)
	s.initPathsLimitRecv(s.negotiated.Encoding)
	// RFC 8654 Section 4: "A BGP speaker MAY send BGP Extended Messages to a
	// peer only if the BGP Extended Message Capability was received from that peer."
	s.writeBuf.Resize(s.negotiated.ExtendedMessageSend)
	s.writeMu.Unlock()

	// RFC 8654 Section 6: "For all messages except for OPEN and KEEPALIVE
	// messages, if the receiver has advertised the BGP Extended Message
	// Capability, this document raises that limit to 65,535."
	s.extendedMessage = s.negotiated.ExtendedMessageRecv

	// RFC 4724 Section 4.2: a session without Graceful Restart removes the
	// stale routes at once, so the held generations that mirror them go too.
	s.heldTypedNLRI().beginSession(s.negotiated.GracefulRestart != nil)

	// RFC 4271 Section 4.2: "A BGP speaker MUST calculate the value of the
	// Hold Timer by using the smaller of its configured Hold Time and the
	// Hold Time received in the OPEN message. The Hold Time MUST be either
	// zero or at least three seconds."
	localHold := s.settings.ReceiveHoldTime
	peerHold := time.Duration(s.peerOpen.HoldTime) * time.Second

	var negotiatedHold time.Duration
	if localHold == 0 || peerHold == 0 {
		negotiatedHold = 0
	} else {
		negotiatedHold = min(peerHold, localHold)
		// RFC 4271 Section 4.2: Hold time value MUST be either zero or at least 3 seconds.
		if negotiatedHold > 0 && negotiatedHold < 3*time.Second {
			negotiatedHold = 3 * time.Second
		}
	}

	s.negotiated.HoldTime = uint16(negotiatedHold / time.Second) //nolint:gosec // Hold time max 65535s
	s.timers.SetHoldTime(negotiatedHold)

	// RFC 4271 Section 10: clamp keepalive when negotiated hold-time shrinks.
	// A configured keepalive >= negotiated hold-time would cause session flap.
	// The effective keepalive is: configured value if valid, otherwise hold/3.
	effectiveKeepalive := s.settings.KeepaliveTime
	if effectiveKeepalive == 0 || (negotiatedHold > 0 && effectiveKeepalive >= negotiatedHold) {
		effectiveKeepalive = negotiatedHold / 3
	}
	if ka := s.settings.KeepaliveTime; ka > 0 && negotiatedHold > 0 && ka >= negotiatedHold {
		s.timers.SetKeepaliveTime(effectiveKeepalive)
	}

	if s.onNegotiated != nil {
		s.onNegotiated(uint32(negotiatedHold/time.Second), uint32(effectiveKeepalive/time.Second))
	}
}

// sendOpen sends an OPEN message.
//
// RFC 7607 Section 2: "A router MUST NOT initiate a connection claiming to be AS 0."
// The refusal sits here rather than in buildOpen because this is the one place an OPEN
// reaches a socket, and buildOpen is also called by the reload swap decision to compare
// two OPENs that are never sent (peer_settings_negotiation.go).
//
// The YANG type already refuses AS 0 at config load (zt:asn is range "1..4294967295" in
// internal/component/config/yang/modules/ze-types.yang), so an operator cannot reach this
// through configuration. It is the second half of the pair: a peering whose LocalAS
// arrived from anywhere else still cannot put AS 0 on the wire.
func (s *Session) sendOpen(conn net.Conn) error {
	if s.settings.LocalAS == 0 {
		return ErrLocalASZero
	}

	s.mu.RLock()
	open := s.localOpen
	s.mu.RUnlock()
	if open == nil {
		if err := s.prepareOpen(); err != nil {
			return err
		}
		s.mu.RLock()
		open = s.localOpen
		s.mu.RUnlock()
	}

	err := s.writeMessage(conn, open)
	if err == nil && s.onOpenSent != nil {
		s.onOpenSent()
	}
	return err
}

// prepareOpen builds the proposal once, before negotiation or transmission.
// The caller MUST serialize initial OPEN work with openMu.
func (s *Session) prepareOpen() error {
	open, err := s.buildOpen(s.settings, s.configCapabilities())
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.localOpen = open
	if transport := s.transport.Load(); transport != nil {
		transport.localAS.Store(open.ASN4)
	}
	s.mu.Unlock()
	return nil
}

// configCapabilities returns the configured capability list, read under the
// Peer's lock when a Peer owns this session. A reload swap can replace the slice
// on the shared PeerSettings (hotSwappableSettings, peer_settings_apply.go).
//
// The direct read is the fallback for a Session a test built with NewSession: no
// Peer owns it, so nothing writes the field and the read cannot race.
func (s *Session) configCapabilities() []capability.Capability {
	if s.configCapGetter != nil {
		return s.configCapGetter()
	}
	return s.settings.Capabilities
}

// buildOpen builds the OPEN message this session sends under the given settings
// and configured capability list.
//
// It is the ONE producer of ze's OPEN. sendOpen writes what it returns, and the
// reload swap decision calls it a second time with the settings a reload proposes
// to answer "would the OPEN come out the same?" (peer_settings_negotiation.go).
// A second builder would let the decision be taken against an OPEN ze never sends.
//
// configCaps is passed rather than read off settings.Capabilities because the two
// have different lock rules: every other field this reads is set at construction
// and never mutated, while Capabilities is replaced by a reload swap.
func (s *Session) buildOpen(settings *PeerSettings, configCaps []capability.Capability) (*message.Open, error) {
	// Build capabilities in RFC-expected order:
	// 1. Multiprotocol (from config OR plugin decode families - not both)
	// 2. ASN4
	// 3. Other capabilities (extended-message, route-refresh, etc.)
	// 4. Plugin-declared capabilities
	caps := offeredMultiprotocol(configCaps, s.pluginFamiliesGetter)
	var otherCaps []capability.Capability
	for _, c := range configCaps {
		if c.Code() != capability.CodeMultiprotocol {
			otherCaps = append(otherCaps, c)
		}
	}

	// The AS this OPEN speaks under, resolved ONCE. The header field, the Four-octet AS
	// capability and the ASN4 field the encoder reads all take it from here, because two
	// resolutions is how a header saying one AS and a capability saying another get
	// written (openLocalAS, session_as_migration.go).
	openAS := openLocalAS(settings, s.asMigrationFallback)

	// Add ASN4 unless disabled in config.
	if !settings.DisableASN4 {
		caps = append(caps, &capability.ASN4{ASN: openAS})
	}

	// Add remaining capabilities.
	caps = append(caps, otherCaps...)

	// Add plugin-declared capabilities (e.g., hostname from RFC 9234 plugin).
	if s.pluginCapGetter != nil {
		caps = append(caps, s.pluginCapGetter()...)
	}

	// draft-abraitis-idr-addpath-paths-limit-04 Section 3: multiple families
	// "MUST" be included "in a single instance of the PATHS-LIMIT capability."
	// Merge configuration and plugin declarations before any length is encoded.
	caps, err := coalescePathsLimit(caps)
	if err != nil {
		return nil, err
	}

	// Every capability rides in one type-2 parameter; the packer chooses the
	// framing from the whole field length.
	// RFC 9072 Section 2
	var params []capability.OptionalParam
	if len(caps) > 0 {
		params = []capability.OptionalParam{capability.CapabilitiesParam(caps)}
	}
	optParams, extendedParams, err := capability.PackOptionalParams(params)
	if err != nil {
		return nil, fmt.Errorf("build OPEN: %w", err)
	}

	// Determine AS to put in header (AS_TRANS if > 65535).
	//
	// RFC 6793 Section 3: a speaker whose AS does not fit two octets sends AS_TRANS here
	// and its real AS in the Four-octet AS capability. openAS is what both read, so a
	// four-octet RFC 7705 migration ASN narrows to AS_TRANS on exactly the same terms as a
	// four-octet local AS.
	myAS := uint16(openAS) //nolint:gosec // Truncation intended for AS_TRANS
	if openAS > 65535 {
		myAS = 23456 // AS_TRANS
	}

	return &message.Open{
		Version:        4,
		MyAS:           myAS,
		HoldTime:       uint16(settings.ReceiveHoldTime / time.Second), //nolint:gosec // Hold time max 65535s
		BGPIdentifier:  settings.RouterID,
		ASN4:           openAS,
		OptionalParams: optParams,
		ExtendedParams: extendedParams,
	}, nil
}

// coalescePathsLimit compacts the builder-owned slice in place. Caller-owned
// capability objects remain unchanged, including their entry slices.
func coalescePathsLimit(caps []capability.Capability) ([]capability.Capability, error) {
	var merged *capability.PathsLimit
	var seen map[family.Family]bool
	count := 0
	for _, c := range caps {
		if c.Code() != capability.CodePathsLimit {
			caps[count] = c
			count++
			continue
		}

		var entries []capability.PathsLimitEntry
		if pl, ok := c.(*capability.PathsLimit); ok {
			entries = pl.Entries
		} else {
			// Plugins supply raw value bytes. Validate before WriteTo can narrow
			// the length to one octet; retain zero tuples for duplicate precedence.
			size := c.Len()
			if size < 2 || size > 257 || (size-2)%5 != 0 {
				return nil, fmt.Errorf("%w: PATHS-LIMIT value must contain at most 51 five-octet tuples", capability.ErrInvalidLength)
			}
			var wire [257]byte
			if c.WriteTo(wire[:], 0) != size {
				return nil, fmt.Errorf("%w: PATHS-LIMIT encoded size differs from declared size", capability.ErrInvalidLength)
			}
			entries = make([]capability.PathsLimitEntry, (size-2)/5)
			for i := range entries {
				off := 2 + i*5
				entries[i] = capability.PathsLimitEntry{
					AFI:   family.AFI(binary.BigEndian.Uint16(wire[off:])),
					SAFI:  family.SAFI(wire[off+2]),
					Limit: binary.BigEndian.Uint16(wire[off+3:]),
				}
			}
		}

		if merged == nil {
			merged = &capability.PathsLimit{}
			seen = make(map[family.Family]bool)
			caps[count] = merged
			count++
		}
		for _, entry := range entries {
			f := family.Family{AFI: entry.AFI, SAFI: entry.SAFI}
			// Section 3: "All others MUST be ignored." A first zero also wins.
			if seen[f] {
				continue
			}
			if len(merged.Entries) == 51 {
				return nil, fmt.Errorf("%w: PATHS-LIMIT exceeds 51 distinct AFI/SAFI tuples", capability.ErrInvalidLength)
			}
			seen[f] = true
			merged.Entries = append(merged.Entries, entry)
		}
	}
	return caps[:count], nil
}

// offeredMultiprotocol is the Multiprotocol capability list ze's OPEN offers a
// peer: the configured families when the config names any, and every plugin
// decode family otherwise. buildOpen sends it, and
// reactorAPIAdapter.SelectedPeersCarry reads it to answer whether a selected
// peer is configured for a route's family, so the two cannot disagree.
//
// If config has NO family block, use ALL plugin decode families.
// This allows plugins to define what families are available.
//
// KNOWN DEFECT, and the fix is a DESIGN decision, not a patch here. In the
// shipped tree this fallback advertises 17 families and none of them is
// ipv4/unicast, because every in-tree NLRI plugin declares decode families
// and none of them declares the core family. Negotiate applies its implicit
// ipv4/unicast default only when the local set is EMPTY, so 17 families
// means the default never fires and the OPEN positively states that ze does
// not speak IPv4 unicast. A peer with no family block negotiates nothing
// usable against an ordinary neighbor, and is offered BGP-LS nobody asked
// for.
//
// Three readings, and picking one silently is what this comment prevents:
// the fallback should ADD ipv4/unicast (breaks a deliberate single-family
// plugin, which TestBuildOpenPluginFamiliesUnchanged legitimately pins);
// config silence should mean ipv4/unicast alone, as ExaBGP does (drops the
// plugin-fills-the-gap behavior AC-3 was written for); or the getter's
// population is wrong, because the whole in-tree NLRI catalog is not an
// operator choice the way one purpose-built plugin is.
func offeredMultiprotocol(configCaps []capability.Capability, pluginFamilies func() []string) []capability.Capability {
	var caps []capability.Capability
	for _, c := range configCaps {
		if c.Code() == capability.CodeMultiprotocol {
			caps = append(caps, c)
		}
	}
	if len(caps) > 0 || pluginFamilies == nil {
		return caps
	}
	seen := make(map[family.Family]bool)
	for _, famStr := range pluginFamilies() {
		fam, ok := family.LookupFamily(famStr)
		if !ok {
			continue // Invalid family string, skip
		}
		if seen[fam] {
			continue // Avoid duplicates from multiple plugins
		}
		caps = append(caps, &capability.Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI})
		seen[fam] = true
	}
	return caps
}
