// Design: docs/architecture/api/commands.md — EVPN route type text parsing
// Overview: update_text.go — main update text parser and shared constants
// Related: update_text_nlri.go — generic NLRI section parsing
// Related: update_text_vpls.go — VPLS text parsing
package update

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/route"
)

// EVPN route type keywords.
const (
	kwMACIP     = "mac-ip"
	kwIPPrefix  = "ip-prefix"
	kwMulticast = "multicast"
	kwMAC       = "mac"
	kwIP        = "ip"
	kwPrefix    = "prefix"
	kwESI       = "esi"
	kwEtag      = "etag"
	kwGateway   = "gateway" // RFC 9136: GW IP Overlay Index for Type 5
)

// isEVPNBoundary returns true if token ends EVPN section (next section starts).
// EVPN-specific keywords (rd, label, mac, ip, etc.) are NOT boundaries.
func isEVPNBoundary(token string) bool {
	switch token {
	case kwRD, kwLabel, kwMAC, kwIP, kwPrefix, kwESI, kwEtag, kwGateway:
		return false // These are valid within EVPN
	case kwMACIP, kwIPPrefix, kwMulticast:
		return false // Route type keywords
	}
	return isBoundaryKeyword(token)
}

// parseEVPNSection parses EVPN NLRI section.
// RFC 7432: EVPN route types.
// Syntax: nlri l2vpn/evpn add <route-type> rd <rd> ...
func parseEVPNSection(args []string, family nlri.Family, _ nlriAccum) (nlriParseResult, error) {
	// args[0] = "nlri", args[1] = "l2vpn/evpn"
	consumed := 2
	i := 2

	mode := "" // "", "add", or "del"

	// EVPN common fields
	var rd nlri.RouteDistinguisher
	var esi [10]byte
	var ethernetTag uint32
	var labels []uint32
	hasRD := false

	// Type 2 specific
	var mac [6]byte
	var ip netip.Addr
	hasMAC := false

	// Type 5 specific
	var prefix netip.Prefix
	var gateway netip.Addr

	routeType := ""

	for i < len(args) {
		token := args[i]

		// Boundary keywords end this section (except EVPN-specific keywords)
		if isEVPNBoundary(token) {
			break
		}

		// Mode switches
		if token == kwAdd {
			mode = kwAdd
			i++
			consumed++
			continue
		}
		if token == kwDel {
			mode = kwDel
			i++
			consumed++
			continue
		}

		// Must have mode before route type and fields
		if mode == "" {
			return nlriParseResult{}, fmt.Errorf("%w: got %q", route.ErrMissingAddDel, token)
		}

		// Route type (after add/del)
		if routeType == "" {
			switch token {
			case kwMACIP, kwIPPrefix, kwMulticast:
				routeType = token
				i++
				consumed++
				continue
			default: // unknown route type — reject with error
				return nlriParseResult{}, fmt.Errorf("evpn requires route type (mac-ip, ip-prefix, multicast), got: %s", token)
			}
		}

		// Parse EVPN-specific fields
		switch token {
		case kwRD:
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("rd requires value")
			}
			var err error
			rd, err = nlri.ParseRDString(args[i+1])
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("invalid rd: %w", err)
			}
			hasRD = true
			i += 2
			consumed += 2

		case kwMAC:
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("mac requires value")
			}
			macBytes, err := parseMAC(args[i+1])
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("invalid mac: %w", err)
			}
			mac = macBytes
			hasMAC = true
			i += 2
			consumed += 2

		case kwIP:
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("ip requires value")
			}
			var err error
			ip, err = netip.ParseAddr(args[i+1])
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("invalid ip: %w", err)
			}
			i += 2
			consumed += 2

		case kwPrefix:
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("prefix requires value")
			}
			var err error
			prefix, err = netip.ParsePrefix(args[i+1])
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("invalid prefix: %w", err)
			}
			i += 2
			consumed += 2

		case kwLabel:
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("label requires value")
			}
			val, err := strconv.ParseUint(args[i+1], 10, 32)
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("invalid label: %w", err)
			}
			labels = append(labels, uint32(val))
			i += 2
			consumed += 2

		case kwESI:
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("esi requires value")
			}
			esiBytes, err := parseESI(args[i+1])
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("invalid esi: %w", err)
			}
			esi = esiBytes
			i += 2
			consumed += 2

		case kwEtag:
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("etag requires value")
			}
			val, err := strconv.ParseUint(args[i+1], 10, 32)
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("invalid etag: %w", err)
			}
			ethernetTag = uint32(val)
			i += 2
			consumed += 2

		case kwGateway:
			// RFC 9136 Section 3.1: GW IP Address for Overlay Index resolution
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("gateway requires value")
			}
			var err error
			gateway, err = netip.ParseAddr(args[i+1])
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("invalid gateway: %w", err)
			}
			i += 2
			consumed += 2

		default: // unknown EVPN keyword — reject with error
			return nlriParseResult{}, fmt.Errorf("unknown evpn keyword: %s", token)
		}
	}

	// Validate required fields
	if routeType == "" {
		return nlriParseResult{}, errors.New("evpn requires route type")
	}
	if !hasRD {
		return nlriParseResult{}, errors.New("evpn requires rd")
	}

	// Create EVPN NLRI via registry encoder
	// Build args: route-type specific key-value pairs
	var encodeArgs []string
	switch routeType {
	case kwMACIP:
		if !hasMAC {
			return nlriParseResult{}, errors.New("mac-ip route requires mac")
		}
		encodeArgs = append(encodeArgs, "type2", "rd", rd.String(),
			"esi", formatESIString(esi),
			"ethernet-tag", strconv.FormatUint(uint64(ethernetTag), 10),
			"mac", formatMACString(mac))
		if ip.IsValid() {
			encodeArgs = append(encodeArgs, "ip", ip.String())
		}
		for _, l := range labels {
			encodeArgs = append(encodeArgs, "label", strconv.FormatUint(uint64(l), 10))
		}

	case kwIPPrefix:
		if !prefix.IsValid() {
			return nlriParseResult{}, errors.New("ip-prefix route requires prefix")
		}
		// RFC 9136 Section 3.1: prefix and gateway MUST be same IP address family
		if gateway.IsValid() && prefix.Addr().Is4() != gateway.Is4() {
			return nlriParseResult{}, errors.New("ip-prefix route: gateway must be same IP family as prefix (RFC 9136)")
		}
		// RFC 9136 Section 3.2: ESI and GW IP MUST NOT both be non-zero
		esiNonZero := esi != [10]byte{}
		if esiNonZero && gateway.IsValid() {
			return nlriParseResult{}, errors.New("ip-prefix route: esi and gateway are mutually exclusive (RFC 9136)")
		}
		encodeArgs = append(encodeArgs, "type5", "rd", rd.String(),
			"esi", formatESIString(esi),
			"ethernet-tag", strconv.FormatUint(uint64(ethernetTag), 10),
			"prefix", prefix.String())
		if gateway.IsValid() {
			encodeArgs = append(encodeArgs, "gateway", gateway.String())
		}
		for _, l := range labels {
			encodeArgs = append(encodeArgs, "label", strconv.FormatUint(uint64(l), 10))
		}

	case kwMulticast:
		// Type 3: Inclusive Multicast Ethernet Tag route
		originatorIP := ip
		if !originatorIP.IsValid() {
			return nlriParseResult{}, errors.New("multicast route requires ip (originator)")
		}
		encodeArgs = append(encodeArgs, "type3", "rd", rd.String(),
			"ethernet-tag", strconv.FormatUint(uint64(ethernetTag), 10),
			"ip", originatorIP.String())
	}

	evpnNLRI, err := encodeViaRegistry(family, encodeArgs, false)
	if err != nil {
		return nlriParseResult{}, err
	}

	return buildSingleNLRIResult(family, mode, evpnNLRI, consumed)
}

// parseMAC parses a MAC address string (e.g., "00:11:22:33:44:55").
func parseMAC(s string) ([6]byte, error) {
	var mac [6]byte
	parts := strings.Split(s, ":")
	if len(parts) != 6 {
		// Try dash separator
		parts = strings.Split(s, "-")
		if len(parts) != 6 {
			return mac, fmt.Errorf("invalid mac format: %s", s)
		}
	}
	for i, p := range parts {
		val, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return mac, fmt.Errorf("invalid mac byte: %s", p)
		}
		mac[i] = byte(val)
	}
	return mac, nil
}

// parseESI parses an ESI string (10 bytes, colon-separated hex).
func parseESI(s string) ([10]byte, error) {
	var esi [10]byte
	parts := strings.Split(s, ":")
	if len(parts) != 10 {
		return esi, fmt.Errorf("invalid esi format (need 10 bytes): %s", s)
	}
	for i, p := range parts {
		val, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return esi, fmt.Errorf("invalid esi byte: %s", p)
		}
		esi[i] = byte(val)
	}
	return esi, nil
}

// formatMACString formats a MAC address as colon-separated hex (e.g., "00:11:22:33:44:55").
func formatMACString(mac [6]byte) string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}

// formatESIString formats an ESI as colon-separated hex (e.g., "00:11:22:33:44:55:66:77:88:99").
func formatESIString(esi [10]byte) string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x",
		esi[0], esi[1], esi[2], esi[3], esi[4], esi[5], esi[6], esi[7], esi[8], esi[9])
}
