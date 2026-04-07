// Design: (none -- new tool, predates documentation)
// Overview: benchmark.go -- benchmark orchestration using session I/O

package perf

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// SessionConfig holds the parameters needed to establish a BGP session
// for performance testing. Unlike chaos/peer.SessionConfig, this uses a single
// family string rather than a slice, matching the perf tool's simpler model.
type SessionConfig struct {
	// ASN is the local autonomous system number.
	ASN uint32

	// RouterID is the BGP router identifier.
	RouterID netip.Addr

	// HoldTime is the proposed hold time in seconds.
	HoldTime uint16

	// Family is the address family to negotiate ("ipv4/unicast" or "ipv6/unicast").
	Family string
}

// familyPair holds an AFI/SAFI pair for multiprotocol capability construction.
type familyPair struct {
	afi  family.AFI
	safi family.SAFI
}

// familyLookup maps family strings to (AFI, SAFI) pairs.
var familyLookup = map[string]familyPair{
	"ipv4/unicast": {family.AFIIPv4, family.SAFIUnicast},
	"ipv6/unicast": {family.AFIIPv6, family.SAFIUnicast},
}

// BuildOpen constructs a serialized BGP OPEN message with capabilities:
// ASN4, Multiprotocol (for the configured family), and RouteRefresh.
func BuildOpen(cfg SessionConfig) []byte {
	fam := cfg.Family
	if fam == "" {
		fam = "ipv4/unicast"
	}

	var caps []capability.Capability

	if pair, ok := familyLookup[fam]; ok {
		caps = append(caps, &capability.Multiprotocol{
			AFI:  pair.afi,
			SAFI: pair.safi,
		})
	}

	caps = append(caps, &capability.ASN4{ASN: cfg.ASN}, &capability.RouteRefresh{})

	optParams := packOptionalParams(caps)

	myAS := uint16(cfg.ASN) //nolint:gosec // Truncation intended for AS_TRANS
	if cfg.ASN > 65535 {
		myAS = message.AS_TRANS
	}

	rid := cfg.RouterID.As4()

	open := &message.Open{
		Version:        4,
		MyAS:           myAS,
		HoldTime:       cfg.HoldTime,
		BGPIdentifier:  binary.BigEndian.Uint32(rid[:]),
		ASN4:           cfg.ASN,
		OptionalParams: optParams,
	}

	return SerializeMsg(open)
}

// packOptionalParams builds optional parameters from capabilities.
// Each capability is wrapped in its own parameter (type 2) per RFC 5492.
func packOptionalParams(caps []capability.Capability) []byte {
	if len(caps) == 0 {
		return nil
	}

	total := 0
	for _, c := range caps {
		total += 2 + c.Len() // param type (1) + param length (1) + cap TLV
	}

	buf := make([]byte, total)
	off := 0

	for _, c := range caps {
		capLen := c.Len()
		buf[off] = 2              // Parameter type: Capabilities (RFC 5492)
		buf[off+1] = byte(capLen) //nolint:gosec // Capability TLVs are always <256 bytes
		off += 2
		off += c.WriteTo(buf, off)
	}

	return buf
}

// BuildKeepalive constructs a serialized BGP KEEPALIVE message (19 bytes).
func BuildKeepalive() []byte {
	ka := message.NewKeepalive()
	return SerializeMsg(ka)
}

// BuildCeaseNotification constructs a NOTIFICATION Cease/AdminShutdown message.
func BuildCeaseNotification() []byte {
	notif := &message.Notification{
		ErrorCode:    message.NotifyCease,
		ErrorSubcode: message.NotifyCeaseAdminShutdown,
	}
	return SerializeMsg(notif)
}

// SerializeMsg serializes any BGP message to wire bytes.
func SerializeMsg(msg message.Message) []byte {
	size := msg.Len(nil)
	buf := make([]byte, size)
	msg.WriteTo(buf, 0, nil)

	return buf
}

// ReadMessage reads one complete BGP message from a reader.
// Returns the message type and the full message bytes (including header).
// The caller MUST set appropriate read deadlines on the underlying connection
// before calling (when using a buffered reader over a net.Conn).
func ReadMessage(r io.Reader) (message.MessageType, []byte, error) {
	hdr := make([]byte, message.HeaderLen)
	return readMessageWithHdr(r, hdr)
}

// ReadMessageBuf reads one complete BGP message using a caller-provided header
// buffer. The hdr buffer MUST be at least message.HeaderLen (19) bytes.
// This avoids per-call header allocation in hot loops.
func ReadMessageBuf(r io.Reader, hdr []byte) (message.MessageType, []byte, error) {
	return readMessageWithHdr(r, hdr)
}

func readMessageWithHdr(r io.Reader, hdr []byte) (message.MessageType, []byte, error) {
	if len(hdr) < message.HeaderLen {
		return 0, nil, fmt.Errorf("header buffer too small: %d < %d", len(hdr), message.HeaderLen)
	}

	if _, err := io.ReadFull(r, hdr[:message.HeaderLen]); err != nil {
		return 0, nil, fmt.Errorf("reading header: %w", err)
	}

	msgLen := int(binary.BigEndian.Uint16(hdr[16:18]))
	if msgLen < message.HeaderLen {
		return 0, nil, fmt.Errorf("invalid message length: %d", msgLen)
	}

	if msgLen > message.MaxMsgLen {
		return 0, nil, fmt.Errorf("message length %d exceeds RFC 4271 limit %d", msgLen, message.MaxMsgLen)
	}

	msg := make([]byte, msgLen)
	copy(msg[:message.HeaderLen], hdr)

	if msgLen > message.HeaderLen {
		if _, err := io.ReadFull(r, msg[message.HeaderLen:]); err != nil {
			return 0, nil, fmt.Errorf("reading body: %w", err)
		}
	}

	return message.MessageType(hdr[18]), msg, nil
}

// WriteMessage writes a complete BGP message to a connection.
func WriteMessage(conn net.Conn, msg []byte) error {
	n, err := conn.Write(msg)
	if err != nil {
		return fmt.Errorf("writing message: %w", err)
	}

	if n != len(msg) {
		return fmt.Errorf("short write: %d/%d", n, len(msg))
	}

	return nil
}

// DoHandshake performs the client side of a BGP OPEN/KEEPALIVE handshake.
// Sends OPEN, reads peer's OPEN, sends KEEPALIVE, reads peer's KEEPALIVE.
// Returns the time taken for the handshake. The caller MUST set a deadline
// on the connection before calling (e.g., connect timeout).
func DoHandshake(conn net.Conn, cfg SessionConfig) (time.Duration, error) {
	start := time.Now()

	if err := WriteMessage(conn, BuildOpen(cfg)); err != nil {
		return 0, fmt.Errorf("sending OPEN: %w", err)
	}

	msgType, rawMsg, err := ReadMessage(conn)
	if err != nil {
		return 0, fmt.Errorf("reading peer OPEN: %w", err)
	}

	if msgType != message.TypeOPEN {
		detail := ""
		if msgType == message.TypeNOTIFICATION && len(rawMsg) >= message.HeaderLen+2 {
			detail = fmt.Sprintf(" (error=%d subcode=%d)", rawMsg[message.HeaderLen], rawMsg[message.HeaderLen+1])
		}

		return 0, fmt.Errorf("expected OPEN, got type %d%s", msgType, detail)
	}

	if err := WriteMessage(conn, BuildKeepalive()); err != nil {
		return 0, fmt.Errorf("sending KEEPALIVE: %w", err)
	}

	msgType, _, err = ReadMessage(conn)
	if err != nil {
		return 0, fmt.Errorf("reading peer KEEPALIVE: %w", err)
	}

	if msgType != message.TypeKEEPALIVE {
		return 0, fmt.Errorf("expected KEEPALIVE, got type %d", msgType)
	}

	return time.Since(start), nil
}
