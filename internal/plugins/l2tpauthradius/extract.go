// Design: plan/spec-bng-1-radius-attributes.md -- RADIUS attribute extraction
// Related: handler.go -- doRADIUS calls extractAuthMetadata on Access-Accept

package l2tpauthradius

import (
	"encoding/binary"
	"net"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/internal/component/radius"
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
// RFC 2866 Section 5.18: Acct-Interim-Interval is 4 octets, network order.
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

	if raw := resp.FindAttr(radius.AttrFilterID); len(raw) > 0 && len(raw) <= 253 {
		meta.FilterID = string(raw)
		found = true
	}

	if raw := resp.FindAttr(radius.AttrAcctInterimInterval); len(raw) == 4 {
		meta.AcctInterimInterval = binary.BigEndian.Uint32(raw)
		found = true
	}

	if !found {
		return nil
	}
	return &meta
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
