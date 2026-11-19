// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- RADIUS attribute extraction
// Related: handler.go -- doRADIUS calls extractAuthMetadata on Access-Accept

package l2tpauthradius

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/radius"
	coreCos "github.com/ze-software/ze/internal/core/cos"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

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
//
// subscriber names the subscriber the Access-Accept authorizes; a framed
// route Ze skips is reported under it (appendFramedRoutes).
func extractAuthMetadata(resp *radius.Packet, subscriber string) *l2tp.AuthMetadata {
	var meta l2tp.AuthMetadata
	var found bool

	if raw := resp.FindAttr(radius.AttrFramedIPAddress); len(raw) == 4 {
		addr := netip.AddrFrom4([4]byte(raw))
		// RFC 2865 Section 5.8: "The value 0xFFFFFFFE indicates that the NAS
		// should select an address for the user (e.g. Assigned from a pool of
		// addresses kept by the NAS)." It names no address, so FramedIP stays
		// unset and the pool chooses. RFC 2866 Section 4.1 then requires the
		// Accounting-Request to "contain the actual IP address assigned", which
		// the pinned 255.255.255.254 would have replaced. 0xFFFFFFFF, the
		// negotiate value, is refused below as limited broadcast.
		if addr == framedIPNASSelects {
			addr = netip.Addr{}
		}
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
		if down, up := extractVSARates(resp); down > 0 {
			meta.FilterID = mikrotikRateToFilterID(down, up)
			found = true
		}
	}

	if raw := resp.FindAttr(radius.AttrAcctInterimInterval); len(raw) == 4 {
		meta.AcctInterimInterval = binary.BigEndian.Uint32(raw)
		found = true
	}

	// RFC 2548 Sections 2.7.6 and 2.7.7: the subscriber's resolvers, which
	// the pool's radius source offers in IPCP.
	if primary, secondary := extractMSDNSServers(resp); primary.IsValid() || secondary.IsValid() {
		meta.DNSPrimary = primary
		meta.DNSSecondary = secondary
		found = true
	}

	// RFC 2865 Section 5.22: Framed-Route (multi-valued, text).
	// RFC 3162 Section 2.5: Framed-IPv6-Route (multi-valued, text).
	routes := extractFramedRoutes(resp, subscriber)
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
// RFC 3162 Section 2.5: format is "prefix[/len] gateway metric...", and a
// gateway of :: means the subscriber itself.
// An entry Ze cannot parse is skipped, logged at WARN and raised on the
// report bus under subscriber, and the rest of the authorization still
// applies: the text cannot fail the Access-Accept.
// Keep one valid entry beyond the supported limit as an overflow witness.
// The route owner must refuse the whole update, rather than accept a truncated
// authorization. A witness reports at least this many routes, not the full count.
// RFC 2865 Section 5.22: "It is intended to be human readable and MUST NOT affect operation of the protocol."
// RFC 3162 Section 2.5: "It is intended to be human readable and MUST NOT affect operation of the protocol."
func extractFramedRoutes(resp *radius.Packet, subscriber string) []l2tp.FramedRoute {
	routes, full := appendFramedRoutes(nil, resp, radius.AttrFramedRoute, "Framed-Route", subscriber)
	if full {
		return routes
	}
	routes, _ = appendFramedRoutes(routes, resp, radius.AttrFramedIPv6Route, "Framed-IPv6-Route", subscriber)
	return routes
}

// appendFramedRoutes appends every parseable value of one framed-route
// attribute to routes and logs each value it skips, raising it under
// subscriber (reportFramedRouteSkipped). It reports full once routes holds
// the overflow witness, one entry past the limit, so the caller stops
// reading. The logged value is route text, never a secret.
func appendFramedRoutes(routes []l2tp.FramedRoute, resp *radius.Packet, attr uint8, attrName, subscriber string) ([]l2tp.FramedRoute, bool) {
	for _, raw := range resp.FindAllAttr(attr) {
		r, err := parseFramedRoute(string(raw))
		if err != nil {
			logger().Warn("l2tp-auth-radius: skipped a framed route Ze cannot parse; the other routes of the Access-Accept still apply. Correct the subscriber's RADIUS profile",
				"code", diagnostic.CodeDoctorRADIUSFramedRouteSkipped, "subscriber", subscriber,
				"attribute", attrName, "value", string(raw), "reason", err.Error())
			reportFramedRouteSkipped(subscriber, attrName, err)
			continue
		}
		routes = append(routes, r)
		if len(routes) > l2tpevents.FramedRoutesMax {
			return routes, true
		}
	}
	return routes, false
}

// parseFramedRoute parses a single Framed-Route or Framed-IPv6-Route
// text value. Format: "prefix[/len] gateway [metric]".
// The gateway is kept for the PPP session, which installs the route through
// the subscriber's interface. The redistribution announcement does not use
// it: the subscriber is reached through Ze, so the route is announced with
// NextHop=zero (nhop self per-peer substitution).
// Returns the parsed route on success, and an error naming the field that
// failed on any parse error, including a gateway that is not an address of
// the prefix's family.
// RFC 2865 Section 5.22: "That is followed by a space, a gateway address in dotted quad form, a space, and one or more metrics separated by spaces."
func parseFramedRoute(text string) (l2tp.FramedRoute, error) {
	fields := strings.Fields(text)
	if len(fields) < 2 {
		return l2tp.FramedRoute{}, errors.New("want a prefix and a gateway separated by a space")
	}
	prefix, err := netip.ParsePrefix(fields[0])
	if err != nil {
		return l2tp.FramedRoute{}, fmt.Errorf("prefix %q is not an address/length", fields[0])
	}
	prefix = prefix.Masked()
	gateway, err := netip.ParseAddr(fields[1])
	if err != nil {
		return l2tp.FramedRoute{}, fmt.Errorf("gateway %q is not an address", fields[1])
	}
	if gateway.Zone() != "" {
		return l2tp.FramedRoute{}, fmt.Errorf("gateway %q carries a zone", fields[1])
	}
	if gateway.Is4() != prefix.Addr().Is4() {
		return l2tp.FramedRoute{}, fmt.Errorf("gateway %q is not of the prefix's address family", fields[1])
	}
	var metric uint32
	if len(fields) >= 3 {
		m, err := strconv.ParseUint(fields[2], 10, 32)
		if err != nil {
			return l2tp.FramedRoute{}, fmt.Errorf("metric %q is not a 32-bit decimal number", fields[2])
		}
		metric = uint32(m)
	}
	return l2tp.FramedRoute{Prefix: prefix, Gateway: gateway, Metric: metric}, nil
}

// framedIPNASSelects is the RFC 2865 Section 5.8 Framed-IP-Address value that
// asks the NAS to select the address.
var framedIPNASSelects = netip.AddrFrom4([4]byte{0xff, 0xff, 0xff, 0xfe})

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

const (
	// framedRouteEventWindow bounds the events one subscriber raises: a
	// profile that carries a bad route repeats it on every Access-Accept for
	// that subscriber, and the error ring is shared with every other owner.
	framedRouteEventWindow = time.Minute
	// framedRouteSkippedFix is the fix the event carries: the text came from
	// the subscriber's RADIUS profile, so the profile is what changes.
	framedRouteSkippedFix = "correct the subscriber's RADIUS profile"
	// framedRouteNoUserName is the subject of a request that carried no user
	// name: the bus refuses an empty subject, which would lose the event.
	framedRouteNoUserName = "(no user name)"
)

// reportFramedRouteSkipped raises the error event of one framed route Ze
// skipped, under the subscriber, with the attribute name and the parse
// error: the error names the offending field, and no other attribute or
// secret is copied. At most one event per subscriber per
// framedRouteEventWindow. Safe for concurrent use.
func reportFramedRouteSkipped(subscriber, attrName string, cause error) {
	subject := subscriber
	if subject == "" {
		subject = framedRouteNoUserName
	}
	report.RaiseErrorFixOnce(Name, diagnostic.CodeDoctorRADIUSFramedRouteSkipped, subject,
		report.FitMessage(attrName+": "+cause.Error()), framedRouteSkippedFix, framedRouteEventWindow)
}
