// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- RADIUS attribute extraction
// Related: handler.go -- doRADIUS calls extractAuthMetadata on Access-Accept

package l2tpauthradius

import (
	"encoding/binary"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/radius"
	coreCos "github.com/ze-software/ze/internal/core/cos"
)

const maxFramedRoutesPerSession = 64

// extractAuthMetadata extracts subscriber profile attributes from a
// RADIUS Access-Accept response. Returns nil when no profile
// attributes are present.
//
// RFC 2865 Section 5.8: Framed-IP-Address is 4 octets, network order.
// RFC 2865 Section 5.9: Framed-IP-Netmask is 4 octets, network order.
// RFC 2865 Section 5.11: Filter-Id is a UTF-8 string.
// RFC 2865 Section 5.27: Session-Timeout is 4 octets, network order.
// RFC 2865 Section 5.28: Idle-Timeout is 4 octets, network order.
// RFC 2869 Section 5.16: Acct-Interim-Interval is 4 octets, network order.
// Framed-Pool (type 88) is a UTF-8 string.
func extractAuthMetadata(resp *radius.Packet) *l2tp.AuthMetadata {
	var meta l2tp.AuthMetadata
	var found bool

	if raw := resp.FindAttr(radius.AttrFramedIPAddress); len(raw) == 4 {
		addr := netip.AddrFrom4([4]byte(raw))
		if isValidSubscriberIP(addr) {
			meta.FramedIP = addr
			found = true
		}
	}

	if raw := resp.FindAttr(radius.AttrFramedIPNetmask); len(raw) == 4 {
		meta.FramedNetmask = net.IPv4Mask(raw[0], raw[1], raw[2], raw[3])
		found = true
	}

	// RFC 2865: attribute value max is 253 bytes; cap defensively.
	if raw := resp.FindAttr(radius.AttrFramedPool); len(raw) > 0 && len(raw) <= 253 {
		meta.FramedPool = string(raw)
		found = true
	}

	if raw := resp.FindAttr(radius.AttrSessionTimeout); len(raw) == 4 {
		meta.SessionTimeout = binary.BigEndian.Uint32(raw)
		found = true
	}

	if raw := resp.FindAttr(radius.AttrIdleTimeout); len(raw) == 4 {
		meta.IdleTimeout = binary.BigEndian.Uint32(raw)
		found = true
	}

	for _, raw := range resp.FindAllAttr(radius.AttrFilterID) {
		if len(raw) == 0 || len(raw) > 253 {
			continue
		}
		s := string(raw)
		if name, ok := coreCos.ParseFilterID(s); ok && meta.CoSProfile == "" {
			meta.CoSProfile = name
		} else if meta.FilterID == "" {
			meta.FilterID = s
		}
		found = true
	}

	if meta.CoSProfile == "" {
		if name := extractVSACoSProfile(resp); name != "" {
			meta.CoSProfile = name
			found = true
		}
	}
	if meta.FilterID == "" {
		if rate := extractVSARate(resp); rate > 0 {
			meta.FilterID = mikrotikRateToFilterID(rate)
			found = true
		}
	}

	if raw := resp.FindAttr(radius.AttrAcctInterimInterval); len(raw) == 4 {
		meta.AcctInterimInterval = binary.BigEndian.Uint32(raw)
		found = true
	}

	// RFC 2865 Section 5.22: Framed-Route (multi-valued, text).
	// RFC 6911 Section 3.2: Framed-IPv6-Route (multi-valued, text).
	routes := extractFramedRoutes(resp)
	if len(routes) > 0 {
		meta.FramedRoutes = routes
		found = true
	}

	if !found {
		return nil
	}
	return &meta
}

// extractFramedRoutes parses all Framed-Route (attr 22) and
// Framed-IPv6-Route (attr 99) attributes from a RADIUS response.
// RFC 2865 Section 5.22: format is "destination[/mask] gateway [metric]".
// RFC 6911 Section 3.2: format is "prefix/len gateway [metric]".
// Malformed entries are silently skipped.
func extractFramedRoutes(resp *radius.Packet) []l2tp.FramedRoute {
	var routes []l2tp.FramedRoute
	for _, raw := range resp.FindAllAttr(radius.AttrFramedRoute) {
		if r, ok := parseFramedRoute(string(raw)); ok {
			routes = append(routes, r)
			if len(routes) >= maxFramedRoutesPerSession {
				return routes
			}
		}
	}
	for _, raw := range resp.FindAllAttr(radius.AttrFramedIPv6Route) {
		if r, ok := parseFramedRoute(string(raw)); ok {
			routes = append(routes, r)
			if len(routes) >= maxFramedRoutesPerSession {
				return routes
			}
		}
	}
	return routes
}

// parseFramedRoute parses a single Framed-Route or Framed-IPv6-Route
// text value. Format: "prefix[/len] gateway [metric]".
// The gateway field (fields[1]) is intentionally ignored: in the BNG
// use case the subscriber is always the next-hop, so the route is
// emitted with NextHop=zero (nhop self per-peer substitution).
// Returns the parsed route and true on success, zero value and false
// on any parse error.
func parseFramedRoute(text string) (l2tp.FramedRoute, bool) {
	fields := strings.Fields(text)
	if len(fields) < 2 {
		return l2tp.FramedRoute{}, false
	}
	prefix, err := netip.ParsePrefix(fields[0])
	if err != nil {
		return l2tp.FramedRoute{}, false
	}
	prefix = prefix.Masked()
	var metric uint32
	if len(fields) >= 3 {
		m, err := strconv.ParseUint(fields[2], 10, 32)
		if err != nil {
			return l2tp.FramedRoute{}, false
		}
		metric = uint32(m)
	}
	return l2tp.FramedRoute{Prefix: prefix, Metric: metric}, true
}

// isValidSubscriberIP checks that an address is suitable for
// assignment to a subscriber: valid, unicast, globally routable IPv4.
func isValidSubscriberIP(addr netip.Addr) bool {
	if !addr.IsValid() || !addr.Is4() {
		return false
	}
	if addr.IsMulticast() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsUnspecified() {
		return false
	}
	// RFC 919: 255.255.255.255 is limited broadcast.
	if addr == netip.MustParseAddr("255.255.255.255") {
		return false
	}
	return true
}
