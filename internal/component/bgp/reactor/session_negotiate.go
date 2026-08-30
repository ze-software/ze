// Design: docs/architecture/core-design.md — BGP capability negotiation
// Overview: session.go — BGP session struct and lifecycle
// Related: negotiated.go — NegotiatedCapabilities produced by negotiation

package reactor

import (
	"encoding/binary"
	"log/slog"
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// negotiateWith performs capability negotiation using pre-parsed capabilities.
func (s *Session) negotiateWith(localCaps, peerCaps []capability.Capability) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.localOpen == nil || s.peerOpen == nil {
		return
	}

	// Negotiate.
	s.negotiated = capability.Negotiate(
		localCaps,
		peerCaps,
		s.settings.LocalAS,
		s.peerOpen.ASN4,
	)

	// RFC 8654: If extended message is negotiated, track for pool selection.
	// MUST be capable of receiving/sending messages up to 65535 octets.
	if s.negotiated.ExtendedMessage {
		s.extendedMessage = true
		s.writeMu.Lock()
		s.writeBuf.Resize(true) // Expand to 65535 if needed
		s.writeMu.Unlock()
	}

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
func (s *Session) sendOpen(conn net.Conn) error {
	open := s.buildOpen(s.settings, s.configCapabilities())

	s.mu.Lock()
	s.localOpen = open
	s.mu.Unlock()

	err := s.writeMessage(conn, open)
	if err == nil && s.onOpenSent != nil {
		s.onOpenSent()
	}
	return err
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
func (s *Session) buildOpen(settings *PeerSettings, configCaps []capability.Capability) *message.Open {
	// Build capabilities in RFC-expected order:
	// 1. Multiprotocol (from config OR plugin decode families - not both)
	// 2. ASN4
	// 3. Other capabilities (extended-message, route-refresh, etc.)
	// 4. Plugin-declared capabilities
	var caps []capability.Capability
	var otherCaps []capability.Capability
	var configHasFamilies bool

	// Separate Multiprotocol capabilities from others.
	// If config specifies families, use ONLY those (plugin families ignored).
	for _, c := range configCaps {
		if c.Code() == capability.CodeMultiprotocol {
			caps = append(caps, c)
			configHasFamilies = true
		} else {
			otherCaps = append(otherCaps, c)
		}
	}

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
	if !configHasFamilies && s.pluginFamiliesGetter != nil {
		seen := make(map[family.Family]bool)
		for _, famStr := range s.pluginFamiliesGetter() {
			fam, ok := family.LookupFamily(famStr)
			if !ok {
				continue // Invalid family string, skip
			}
			if seen[fam] {
				continue // Avoid duplicates from multiple plugins
			}
			caps = append(caps, &capability.Multiprotocol{
				AFI:  fam.AFI,
				SAFI: fam.SAFI,
			})
			seen[fam] = true
		}
	}

	// Add ASN4 unless disabled in config.
	if !settings.DisableASN4 {
		caps = append(caps, &capability.ASN4{ASN: settings.LocalAS})
	}

	// draft-abraitis-idr-addpath-paths-limit: suppress PATHS-LIMIT for RS fast-path peers
	// (RS fast-path forwards raw UPDATEs without per-prefix path tracking).
	if settings.RSFastPath {
		filtered := make([]capability.Capability, 0, len(otherCaps))
		for _, c := range otherCaps {
			if c.Code() != capability.CodePathsLimit {
				filtered = append(filtered, c)
			}
		}
		otherCaps = filtered
	}

	// Add remaining capabilities.
	caps = append(caps, otherCaps...)

	// Add plugin-declared capabilities (e.g., hostname from RFC 9234 plugin).
	if s.pluginCapGetter != nil {
		caps = append(caps, s.pluginCapGetter()...)
	}

	// Build optional parameters (capabilities).
	optParams, extendedParams := buildOptionalParams(caps)

	// Determine AS to put in header (AS_TRANS if > 65535).
	myAS := uint16(settings.LocalAS) //nolint:gosec // Truncation intended for AS_TRANS
	if settings.LocalAS > 65535 {
		myAS = 23456 // AS_TRANS
	}

	return &message.Open{
		Version:        4,
		MyAS:           myAS,
		HoldTime:       uint16(settings.ReceiveHoldTime / time.Second), //nolint:gosec // Hold time max 65535s
		BGPIdentifier:  settings.RouterID,
		ASN4:           settings.LocalAS,
		OptionalParams: optParams,
		ExtendedParams: extendedParams,
	}
}

// buildOptionalParams packs capabilities into a single type-2 optional
// parameter, and reports whether it used the RFC 9072 Section 2 framing.
//
// RFC 5492 Section 4 carries capabilities in a type-2 optional parameter. Its
// Parameter Length is one octet, so RFC 4271 framing holds at most 255 octets of
// capabilities. RFC 9072 Section 2 extends that field to two octets, and one
// parameter then holds every capability ze can send.
//
// The returned flag is the second half of a decision the encoder makes: Open.
// WriteTo chooses the ENVELOPE from the total length, and this chooses the
// framing of the parameters inside it. They are in different packages, and when
// they disagreed ze wrapped one-octet parameters in an extended envelope. A peer
// reading it per RFC 9072 takes the first parameter's type and the high half of
// its length as a two-octet length, and misframes every parameter after it. It
// went unnoticed because ze read back what ze wrote.
//
// Splitting across several parameters is gone with it. It existed only to keep
// each length under 256, which the extended framing makes unnecessary, and a
// split cannot be expressed at all once one parameter can hold everything.
func buildOptionalParams(caps []capability.Capability) ([]byte, bool) {
	if len(caps) == 0 {
		return nil, false
	}

	capTotal := 0
	for _, c := range caps {
		capTotal += c.Len()
	}

	if capTotal <= 255 {
		buf := make([]byte, 2+capTotal)
		buf[0] = 2
		buf[1] = byte(capTotal) //nolint:gosec // guarded by <= 255 check above
		off := 2
		for _, c := range caps {
			off += c.WriteTo(buf, off)
		}
		return buf, false
	}

	// RFC 9072 Section 2: one type-2 parameter with a two-octet Parameter Length.
	if capTotal > maxExtendedParamLen {
		slog.Error("capabilities exceed the RFC 9072 two-octet parameter length, truncating",
			"bytes", capTotal, "limit", maxExtendedParamLen)
		return nil, false
	}

	buf := make([]byte, 3+capTotal)
	buf[0] = 2
	binary.BigEndian.PutUint16(buf[1:3], uint16(capTotal)) //nolint:gosec // bounded above
	off := 3
	for _, c := range caps {
		off += c.WriteTo(buf, off)
	}

	return buf, true
}

// maxExtendedParamLen is the largest value the RFC 9072 Section 2 two-octet
// Parameter Length field can carry.
const maxExtendedParamLen = 65535
