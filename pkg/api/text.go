package api

import (
	"fmt"
	"net/netip"
	"strings"
)

// originLower returns the lowercase origin string for ExaBGP compatibility.
// ExaBGP uses lowercase: "igp", "egp", "incomplete".
func originLower(origin string) string {
	return strings.ToLower(origin)
}

// ReceivedRoute represents a route received from a BGP peer.
// Used for formatting received UPDATE messages to API processes.
type ReceivedRoute struct {
	Prefix          netip.Prefix
	NextHop         netip.Addr
	Origin          string // "igp", "egp", "incomplete"
	LocalPreference uint32
	MED             uint32
	ASPath          []uint32
}

// FormatReceivedUpdate formats received routes as ExaBGP text encoder output.
// Format matches ExaBGP's text.py update() method:
//
//	neighbor <ip> receive update start
//	neighbor <ip> receive update announced <prefix> next-hop <nh> <attrs>
//	neighbor <ip> receive update end
func FormatReceivedUpdate(peerAddr netip.Addr, routes []ReceivedRoute) string {
	var sb strings.Builder
	prefix := fmt.Sprintf("neighbor %s receive update", peerAddr)

	sb.WriteString(prefix)
	sb.WriteString(" start\n")

	for _, route := range routes {
		sb.WriteString(prefix)
		sb.WriteString(" announced ")
		sb.WriteString(route.Prefix.String())
		sb.WriteString(" next-hop ")
		sb.WriteString(route.NextHop.String())

		// Format attributes (lowercase origin for ExaBGP compatibility)
		if route.Origin != "" {
			sb.WriteString(" origin ")
			sb.WriteString(originLower(route.Origin))
		}
		if route.LocalPreference > 0 {
			sb.WriteString(fmt.Sprintf(" local-preference %d", route.LocalPreference))
		}
		if route.MED > 0 {
			sb.WriteString(fmt.Sprintf(" med %d", route.MED))
		}
		if len(route.ASPath) > 0 {
			sb.WriteString(" as-path [")
			for i, asn := range route.ASPath {
				if i > 0 {
					sb.WriteString(" ")
				}
				sb.WriteString(fmt.Sprintf("%d", asn))
			}
			sb.WriteString("]")
		}

		sb.WriteString("\n")
	}

	sb.WriteString(prefix)
	sb.WriteString(" end\n")

	return sb.String()
}

// FormatReceivedWithdraw formats withdrawn routes as ExaBGP text encoder output.
// Format:
//
//	neighbor <ip> receive update start
//	neighbor <ip> receive update withdrawn <prefix>
//	neighbor <ip> receive update end
func FormatReceivedWithdraw(peerAddr netip.Addr, prefixes []netip.Prefix) string {
	var sb strings.Builder
	prefix := fmt.Sprintf("neighbor %s receive update", peerAddr)

	sb.WriteString(prefix)
	sb.WriteString(" start\n")

	for _, p := range prefixes {
		sb.WriteString(prefix)
		sb.WriteString(" withdrawn ")
		sb.WriteString(p.String())
		sb.WriteString("\n")
	}

	sb.WriteString(prefix)
	sb.WriteString(" end\n")

	return sb.String()
}
