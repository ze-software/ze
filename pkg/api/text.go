package api

import (
	"fmt"
	"net/netip"
	"strings"
)

// Encoding constants for process output formatting.
const (
	EncodingJSON = "json"
	EncodingText = "text"
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

// ToRouteUpdate converts a ReceivedRoute to a RouteUpdate for JSON encoding.
func (r ReceivedRoute) ToRouteUpdate() RouteUpdate {
	afi := AFINameIPv4
	if r.Prefix.Addr().Is6() {
		afi = AFINameIPv6
	}
	return RouteUpdate{
		Prefix:    r.Prefix.String(),
		NextHop:   r.NextHop.String(),
		AFI:       afi,
		SAFI:      SAFINameUnicast,
		Origin:    r.Origin,
		ASPath:    r.ASPath,
		LocalPref: r.LocalPreference,
		MED:       r.MED,
	}
}

// FormatReceivedUpdateWithEncoding formats received routes using the specified encoding.
// encoding: "json" or "text" (default).
//
// NOTE: For JSON encoding, this creates a new encoder per call (counter resets).
// Production code should use Server.encoder directly for proper counter tracking.
// This function is primarily for testing encoder output format.
func FormatReceivedUpdateWithEncoding(peer PeerInfo, routes []ReceivedRoute, encoding string) string {
	if encoding == EncodingJSON {
		// Convert to RouteUpdate for JSON encoder
		updates := make([]RouteUpdate, len(routes))
		for i, r := range routes {
			updates[i] = r.ToRouteUpdate()
		}
		encoder := NewJSONEncoder("6.0.0")
		return encoder.RouteAnnounce(peer, updates)
	}
	// Default to text
	return FormatReceivedUpdate(peer.Address, routes)
}

// FormatMessage formats a RawMessage based on ContentConfig.
// Handles encoding (json/text) and format (parsed/raw/full).
func FormatMessage(peer PeerInfo, msg RawMessage, content ContentConfig) string {
	content = content.WithDefaults()

	switch content.Format {
	case FormatRaw:
		return formatRaw(peer, msg, content.Encoding)
	case FormatFull:
		return formatFull(peer, msg, content.Encoding)
	default: // FormatParsed
		return formatParsed(peer, msg, content.Encoding)
	}
}

// formatRaw formats message as raw hex bytes only.
func formatRaw(peer PeerInfo, msg RawMessage, encoding string) string {
	rawHex := fmt.Sprintf("%x", msg.RawBytes)
	if encoding == EncodingJSON {
		return fmt.Sprintf(`{"type":"%s","raw":"%s","peer":"%s"}`,
			msg.Type.String(), rawHex, peer.Address)
	}
	return fmt.Sprintf("neighbor %s receive raw %s %s\n",
		peer.Address, msg.Type.String(), rawHex)
}

// formatParsed formats message with parsed content (no raw bytes).
func formatParsed(peer PeerInfo, msg RawMessage, encoding string) string {
	// Parse UPDATE and format routes
	routes := DecodeUpdateRoutes(msg.RawBytes)
	if len(routes) == 0 {
		// No routes to format (could be withdraw-only or parse error)
		if encoding == EncodingJSON {
			return fmt.Sprintf(`{"type":"update","peer":{"address":{"peer":"%s"}},"announce":{}}`, peer.Address)
		}
		return fmt.Sprintf("neighbor %s receive update\n", peer.Address)
	}

	if encoding == EncodingJSON {
		updates := make([]RouteUpdate, len(routes))
		for i, r := range routes {
			updates[i] = r.ToRouteUpdate()
		}
		encoder := NewJSONEncoder("6.0.0")
		return encoder.RouteAnnounce(peer, updates)
	}
	return FormatReceivedUpdate(peer.Address, routes)
}

// formatFull formats message with both parsed content AND raw bytes.
func formatFull(peer PeerInfo, msg RawMessage, encoding string) string {
	rawHex := fmt.Sprintf("%x", msg.RawBytes)
	routes := DecodeUpdateRoutes(msg.RawBytes)

	if encoding == EncodingJSON {
		// Use proper JSON encoder with raw field support
		encoder := NewJSONEncoder("6.0.0")
		if len(routes) > 0 {
			updates := make([]RouteUpdate, len(routes))
			for i, r := range routes {
				updates[i] = r.ToRouteUpdate()
			}
			return encoder.RouteAnnounceWithRaw(peer, updates, rawHex)
		}
		// Empty routes - still include raw bytes
		return encoder.RouteAnnounceWithRaw(peer, nil, rawHex)
	}

	// Text format: include both parsed routes and raw hex
	if len(routes) > 0 {
		parsed := FormatReceivedUpdate(peer.Address, routes)
		return parsed + fmt.Sprintf("neighbor %s receive update raw %s\n", peer.Address, rawHex)
	}
	return fmt.Sprintf("neighbor %s receive update raw %s\n", peer.Address, rawHex)
}

// FormatOpen formats an OPEN message as ExaBGP text encoder output.
// Format: neighbor <ip> receive open version <v> asn <asn> hold_time <t> router_id <id> capabilities [...].
func FormatOpen(peerAddr netip.Addr, open DecodedOpen) string {
	capsStr := "[]"
	if len(open.Capabilities) > 0 {
		capsStr = "[" + strings.Join(open.Capabilities, ", ") + "]"
	}
	return fmt.Sprintf("neighbor %s receive open version %d asn %d hold_time %d router_id %s capabilities %s\n",
		peerAddr, open.Version, open.ASN, open.HoldTime, open.RouterID, capsStr)
}

// FormatNotification formats a NOTIFICATION message as ExaBGP text encoder output.
// Format: neighbor <ip> receive notification code <c> subcode <s> data <hex> [name].
func FormatNotification(peerAddr netip.Addr, notify DecodedNotification) string {
	// ExaBGP format: code {num} subcode {num} data {hex}
	// We add human-readable names at the end as extension
	dataHex := ""
	if len(notify.Data) > 0 {
		dataHex = fmt.Sprintf("%x", notify.Data)
	}

	base := fmt.Sprintf("neighbor %s receive notification code %d subcode %d data %s",
		peerAddr, notify.ErrorCode, notify.ErrorSubcode, dataHex)

	// Add human-readable names
	names := fmt.Sprintf(" [%s/%s]", notify.ErrorCodeName, notify.ErrorSubcodeName)

	return base + names + "\n"
}

// FormatKeepalive formats a KEEPALIVE message as ExaBGP text encoder output.
// Format: neighbor <ip> receive keepalive.
func FormatKeepalive(peerAddr netip.Addr) string {
	return fmt.Sprintf("neighbor %s receive keepalive\n", peerAddr)
}

// FormatStateChange formats a peer state change event.
// State events are separate from BGP protocol messages.
// Common states: "up", "down", "connected", "established".
func FormatStateChange(peer PeerInfo, state string, encoding string) string {
	if encoding == EncodingJSON {
		return formatStateChangeJSON(peer, state)
	}
	return formatStateChangeText(peer, state)
}

func formatStateChangeJSON(peer PeerInfo, state string) string {
	// Manual JSON construction to match ExaBGP format
	return fmt.Sprintf(`{"type":"state","peer":{"address":"%s","asn":%d},"state":"%s"}`+"\n",
		peer.Address, peer.PeerAS, state)
}

func formatStateChangeText(peer PeerInfo, state string) string {
	return fmt.Sprintf("neighbor %s state %s\n", peer.Address, state)
}
