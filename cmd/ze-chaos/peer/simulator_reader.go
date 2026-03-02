// Design: docs/architecture/chaos-web-dashboard.md — BGP message reading and parsing
// Overview: simulator.go — main simulation loop and types
// Related: simulator_actions.go — chaos and route action execution

package peer

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
)

// readLoop reads BGP messages from conn and emits route events.
// It runs until the connection closes or ctx is canceled.
// Uses an unbounded EventBuffer so event emission never blocks TCP reads.
func readLoop(ctx context.Context, conn net.Conn, peerIndex int, events chan<- Event) {
	// Child context so Drain goroutine stops when readLoop returns
	// (e.g., connection closed before ctx is canceled).
	drainCtx, drainCancel := context.WithCancel(ctx)

	buf := NewEventBuffer()
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		buf.Drain(drainCtx, events)
	}()
	defer func() {
		drainCancel()
		<-drainDone // Wait for Drain to finish before readLoop returns.
	}()

	for {
		if ctx.Err() != nil {
			return
		}

		header := make([]byte, message.HeaderLen)
		if _, err := io.ReadFull(conn, header); err != nil {
			return // Connection closed.
		}

		msgLen := int(binary.BigEndian.Uint16(header[16:18]))
		if msgLen < message.HeaderLen {
			return
		}

		var body []byte
		if msgLen > message.HeaderLen {
			body = make([]byte, msgLen-message.HeaderLen)
			if _, err := io.ReadFull(conn, body); err != nil {
				return
			}
		}

		if len(header) < 19 {
			return
		}

		// Track all message bytes (KEEPALIVE, UPDATE, etc.) for throughput display.
		// Non-UPDATE bytes accumulate in the buffer and flush on the next UPDATE event.
		buf.AddBytesRecv(int64(msgLen))

		msgType := header[18]
		if msgType != 2 { // Not UPDATE — skip (KEEPALIVE, etc.)
			continue
		}

		// Parse IPv4/unicast UPDATE for announced and withdrawn prefixes.
		parseUpdatePrefixes(body, peerIndex, buf)
	}
}

// parseUpdatePrefixes extracts announced and withdrawn prefixes from an
// UPDATE message body (after the 19-byte header). Handles both IPv4/unicast
// NLRI (trailing field) and MP_REACH_NLRI / MP_UNREACH_NLRI attributes for
// IPv6/unicast.
func parseUpdatePrefixes(body []byte, peerIndex int, buf *EventBuffer) {
	if len(body) < 4 {
		return
	}

	// Withdrawn routes length (2 bytes).
	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	off := 2

	// Parse IPv4/unicast withdrawn prefixes.
	end := off + withdrawnLen
	if end > len(body) {
		return
	}
	for off < end {
		prefix, n := parseIPv4Prefix(body[off:end])
		if n <= 0 {
			break
		}
		off += n
		buf.Push(Event{Type: EventRouteWithdrawn, PeerIndex: peerIndex, Time: time.Now(), Prefix: prefix, Family: familyIPv4Unicast})
	}

	// Total path attribute length (2 bytes).
	if off+2 > len(body) {
		return
	}
	attrLen := int(binary.BigEndian.Uint16(body[off : off+2]))
	off += 2

	// Walk attributes looking for MP_REACH_NLRI (14) and MP_UNREACH_NLRI (15).
	attrEnd := min(off+attrLen, len(body))
	for off < attrEnd {
		if off+3 > attrEnd {
			break
		}
		flags := body[off]
		code := body[off+1]
		off += 2

		// Attribute length: 1 byte normally, 2 bytes if extended-length flag set.
		var aLen int
		if flags&0x10 != 0 { // Extended length.
			if off+2 > attrEnd {
				break
			}
			aLen = int(binary.BigEndian.Uint16(body[off : off+2]))
			off += 2
		} else {
			aLen = int(body[off])
			off++
		}
		if off+aLen > attrEnd {
			break
		}

		switch code {
		case 14: // MP_REACH_NLRI
			parseMPReachNLRI(body[off:off+aLen], peerIndex, buf)
		case 15: // MP_UNREACH_NLRI
			parseMPUnreachNLRI(body[off:off+aLen], peerIndex, buf)
		}
		off += aLen
	}
	off = attrEnd

	// Parse trailing IPv4/unicast NLRI (announced prefixes).
	for off < len(body) {
		prefix, n := parseIPv4Prefix(body[off:])
		if n <= 0 {
			break
		}
		off += n
		buf.Push(Event{Type: EventRouteReceived, PeerIndex: peerIndex, Time: time.Now(), Prefix: prefix, Family: familyIPv4Unicast})
	}
}

// afiSafiFamily maps AFI/SAFI to the family string used throughout chaos.
// Returns empty string for unrecognized combinations.
func afiSafiFamily(afi uint16, safi uint8) string {
	switch {
	case afi == 1 && safi == 1:
		return familyIPv4Unicast
	case afi == 1 && safi == 2:
		return "ipv4/multicast"
	case afi == 2 && safi == 1:
		return familyIPv6Unicast
	case afi == 2 && safi == 2:
		return "ipv6/multicast"
	case afi == 1 && safi == 128:
		return "ipv4/vpn"
	case afi == 2 && safi == 128:
		return "ipv6/vpn"
	case afi == 25 && safi == 70:
		return "l2vpn/evpn"
	case afi == 1 && safi == 133:
		return "ipv4/flow"
	case afi == 2 && safi == 133:
		return "ipv6/flow"
	default:
		return ""
	}
}

// parseMPReachNLRI parses MP_REACH_NLRI (type 14) and emits EventRouteReceived.
// Format: AFI(2) + SAFI(1) + NH-len(1) + NH(variable) + reserved(1) + NLRI...
//
// For IPv4/IPv6 unicast: parses individual prefixes from the NLRI field.
// For other families (VPN, EVPN, FlowSpec): emits one event per UPDATE
// with the family tag. In the chaos simulator each UPDATE carries exactly
// one non-unicast NLRI, so the count stays accurate.
func parseMPReachNLRI(data []byte, peerIndex int, buf *EventBuffer) {
	if len(data) < 5 { // AFI(2) + SAFI(1) + NH-len(1) + reserved(1) minimum
		return
	}
	afi := binary.BigEndian.Uint16(data[0:2])
	safi := data[2]
	family := afiSafiFamily(afi, safi)
	if family == "" {
		return
	}
	nhLen := int(data[3])
	off := 4 + nhLen + 1 // Skip next-hop + reserved byte.
	if off > len(data) {
		return
	}

	emitNLRIEvents(data[off:], family, EventRouteReceived, peerIndex, buf)
}

// parseMPUnreachNLRI parses MP_UNREACH_NLRI (type 15) and emits EventRouteWithdrawn.
// Format: AFI(2) + SAFI(1) + withdrawn-NLRI...
//
// For IPv4/IPv6 unicast: parses individual prefixes.
// For other families: emits one event per UPDATE with the family tag.
func parseMPUnreachNLRI(data []byte, peerIndex int, buf *EventBuffer) {
	if len(data) < 3 { // AFI(2) + SAFI(1) minimum
		return
	}
	afi := binary.BigEndian.Uint16(data[0:2])
	safi := data[2]
	family := afiSafiFamily(afi, safi)
	if family == "" {
		return
	}
	emitNLRIEvents(data[3:], family, EventRouteWithdrawn, peerIndex, buf)
}

// emitNLRIEvents dispatches NLRI parsing by family and sends events.
// For unicast families, individual prefixes are parsed. For others (VPN,
// EVPN, FlowSpec), one event per UPDATE is emitted since the chaos
// simulator sends exactly one NLRI per UPDATE for non-unicast families.
func emitNLRIEvents(data []byte, family string, evType EventType, peerIndex int, buf *EventBuffer) {
	switch family {
	case familyIPv4Unicast:
		emitPrefixEvents(data, parseIPv4Prefix, family, evType, peerIndex, buf)
	case familyIPv6Unicast:
		emitPrefixEvents(data, parseIPv6Prefix, family, evType, peerIndex, buf)
	default:
		// VPN, EVPN, FlowSpec: one NLRI per UPDATE in chaos simulator.
		if len(data) > 0 {
			buf.Push(Event{Type: evType, PeerIndex: peerIndex, Time: time.Now(), Family: family})
		}
	}
}

// emitPrefixEvents parses consecutive unicast prefixes and emits an event for each.
func emitPrefixEvents(data []byte, parse func([]byte) (netip.Prefix, int), family string, evType EventType, peerIndex int, buf *EventBuffer) {
	off := 0
	for off < len(data) {
		prefix, n := parse(data[off:])
		if n <= 0 {
			break
		}
		off += n
		buf.Push(Event{Type: evType, PeerIndex: peerIndex, Time: time.Now(), Prefix: prefix, Family: family})
	}
}

// parseIPv4Prefix parses a single IPv4 prefix from wire format.
// Returns the prefix and the number of bytes consumed, or 0 on error.
func parseIPv4Prefix(data []byte) (netip.Prefix, int) {
	if len(data) < 1 {
		return netip.Prefix{}, 0
	}

	prefixLen := int(data[0])
	if prefixLen > 32 {
		return netip.Prefix{}, 0
	}

	byteLen := (prefixLen + 7) / 8
	if len(data) < 1+byteLen {
		return netip.Prefix{}, 0
	}

	var addr [4]byte
	copy(addr[:], data[1:1+byteLen])
	prefix := netip.PrefixFrom(netip.AddrFrom4(addr), prefixLen)

	return prefix, 1 + byteLen
}

// parseIPv6Prefix parses a single IPv6 prefix from wire format.
// Returns the prefix and the number of bytes consumed, or 0 on error.
func parseIPv6Prefix(data []byte) (netip.Prefix, int) {
	if len(data) < 1 {
		return netip.Prefix{}, 0
	}

	prefixLen := int(data[0])
	if prefixLen > 128 {
		return netip.Prefix{}, 0
	}

	byteLen := (prefixLen + 7) / 8
	if len(data) < 1+byteLen {
		return netip.Prefix{}, 0
	}

	var addr [16]byte
	copy(addr[:], data[1:1+byteLen])
	prefix := netip.PrefixFrom(netip.AddrFrom16(addr), prefixLen)

	return prefix, 1 + byteLen
}

// writeMsg serializes and sends a BGP message on a connection.
func writeMsg(conn net.Conn, msg message.Message) error {
	data := SerializeMessage(msg)
	_, err := conn.Write(data)
	return err
}

// readMsg reads and discards a single BGP message from the connection.
func readMsg(conn net.Conn) error {
	header := make([]byte, message.HeaderLen)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("reading header: %w", err)
	}

	msgLen := int(binary.BigEndian.Uint16(header[16:18]))
	if msgLen < message.HeaderLen {
		return fmt.Errorf("invalid message length: %d", msgLen)
	}

	if msgLen > message.HeaderLen {
		body := make([]byte, msgLen-message.HeaderLen)
		if _, err := io.ReadFull(conn, body); err != nil {
			return fmt.Errorf("reading body: %w", err)
		}
	}

	return nil
}

// sendCease sends a NOTIFICATION Cease (best-effort on shutdown).
func sendCease(conn net.Conn, peerIndex int, quiet bool) {
	notif := BuildCeaseNotification()
	_ = writeMsg(conn, notif)

	if !quiet {
		fmt.Fprintf(os.Stderr, "ze-chaos | peer %d | sent NOTIFICATION cease\n", peerIndex)
	}
}
