// Design: docs/architecture/core-design.md — BGP capability negotiation
// Overview: session.go — BGP session struct and lifecycle
// Related: negotiated.go — NegotiatedCapabilities produced by negotiation

package reactor

import (
	"log/slog"
	"net"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
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
}

// sendOpen sends an OPEN message.
func (s *Session) sendOpen(conn net.Conn) error {
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
	for _, c := range s.settings.Capabilities {
		if c.Code() == capability.CodeMultiprotocol {
			caps = append(caps, c)
			configHasFamilies = true
		} else {
			otherCaps = append(otherCaps, c)
		}
	}

	// If config has NO family block, use ALL plugin decode families.
	// This allows plugins to define what families are available.
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
	if !s.settings.DisableASN4 {
		caps = append(caps, &capability.ASN4{ASN: s.settings.LocalAS})
	}

	// Add remaining capabilities.
	caps = append(caps, otherCaps...)

	// Add plugin-declared capabilities (e.g., hostname from RFC 9234 plugin).
	if s.pluginCapGetter != nil {
		caps = append(caps, s.pluginCapGetter()...)
	}

	// Build optional parameters (capabilities).
	optParams := buildOptionalParams(caps)

	// Determine AS to put in header (AS_TRANS if > 65535).
	myAS := uint16(s.settings.LocalAS) //nolint:gosec // Truncation intended for AS_TRANS
	if s.settings.LocalAS > 65535 {
		myAS = 23456 // AS_TRANS
	}

	open := &message.Open{
		Version:        4,
		MyAS:           myAS,
		HoldTime:       uint16(s.settings.ReceiveHoldTime / time.Second), //nolint:gosec // Hold time max 65535s
		BGPIdentifier:  s.settings.RouterID,
		ASN4:           s.settings.LocalAS,
		OptionalParams: optParams,
	}

	s.mu.Lock()
	s.localOpen = open
	s.mu.Unlock()

	return s.writeMessage(conn, open)
}

// buildOptionalParams builds optional parameters from capabilities.
// RFC 5492 Section 4: capabilities are packed into type-2 optional parameters.
// If total capability bytes fit in one parameter (<=255), a single type-2
// parameter is used. If they exceed 255 bytes, capabilities are split across
// multiple type-2 parameters, each within the 255-byte length limit.
func buildOptionalParams(caps []capability.Capability) []byte {
	if len(caps) == 0 {
		return nil
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
		return buf
	}

	// Split into multiple type-2 parameters when total exceeds 255 bytes.
	var buf []byte
	paramLen := 0
	var paramCaps []capability.Capability

	flush := func() {
		if paramLen == 0 {
			return
		}
		param := make([]byte, 2+paramLen)
		param[0] = 2
		param[1] = byte(paramLen) //nolint:gosec // paramLen <= 255 enforced by split logic
		off := 2
		for _, c := range paramCaps {
			off += c.WriteTo(param, off)
		}
		buf = append(buf, param...)
		paramCaps = paramCaps[:0]
		paramLen = 0
	}

	for _, c := range caps {
		cLen := c.Len()
		if cLen > 255 {
			slog.Debug("capability exceeds 255-byte parameter limit, skipping",
				"code", c.Code(), "len", cLen)
			continue
		}
		if paramLen+cLen > 255 {
			flush()
		}
		paramCaps = append(paramCaps, c)
		paramLen += cLen
	}
	flush()

	return buf
}
