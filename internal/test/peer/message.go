// Design: docs/architecture/testing/ci-format.md — BGP message types and wire helpers
// Overview: peer.go — test peer that uses these messages
// Related: checker.go — message validation against expectations

package peer

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// BGP message types.
const (
	MsgOPEN         = 1
	MsgUPDATE       = 2
	MsgNOTIFICATION = 3
	MsgKEEPALIVE    = 4
	MsgROUTEREFRESH = 5
)

// BGP header length.
const HeaderLen = 19

// BGP marker (16 bytes of 0xFF).
var Marker = []byte{
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
}

// Message represents a BGP message.
type Message struct {
	Header []byte
	Body   []byte
}

// Kind returns the message type.
func (m *Message) Kind() byte {
	if len(m.Header) > 18 {
		return m.Header[18]
	}
	return 0
}

// IsKeepalive returns true if this is a KEEPALIVE message.
func (m *Message) IsKeepalive() bool { return m.Kind() == MsgKEEPALIVE }

// IsUpdate returns true if this is an UPDATE message.
func (m *Message) IsUpdate() bool { return m.Kind() == MsgUPDATE }

// IsEOR returns true if this is an End-of-RIB marker (RFC 4724 Section 2).
//
// The two encodings the RFC names are checked by CONTENT, never by length. A
// length test is what this used to be, and it silently swallowed a defect: a
// LENGTH of 11 was read as the multiprotocol marker, and a legacy marker (body
// 4) with a 7-byte attribute stamped onto it is also 11 bytes. An EoR that
// matches no expectation is accepted in silence (checker.go
// ExpectedOrKeepalive). A test asserting that a relayed marker arrives UNSTAMPED
// could therefore only fail by timing out. It never saw the stamped message it
// was there to refuse.
//
//	"An UPDATE message with no reachable Network Layer Reachability
//	 Information (NLRI) and empty withdrawn NLRI is specified as the End-of-RIB
//	 marker [...] For any other address family, it is an UPDATE message that
//	 contains only the MP_UNREACH_NLRI attribute [BGP-MP] with no withdrawn
//	 routes for that <AFI, SAFI>."
//	                                              -- RFC 4724 Section 2
//
// "contains only" is the load-bearing word and is quoted here because
// isBareMPUnreach enforces it: an UPDATE carrying MP_UNREACH_NLRI beside any
// other attribute is not a marker. An earlier paraphrase said "with the
// MP_UNREACH_NLRI attribute", which drops exactly that property.
//
// So: an empty UPDATE body, or one whose ONLY path attribute is an
// MP_UNREACH_NLRI carrying nothing but AFI and SAFI, with no withdrawn routes
// and no NLRI. Anything else is an ordinary UPDATE, however long it is.
func (m *Message) IsEOR() bool {
	if !m.IsUpdate() {
		return false
	}
	if len(m.Body) < 4 {
		return false
	}
	// RFC 4724 Section 2 asks for "empty withdrawn NLRI", so a non-zero length
	// settles it and the attribute length is then always readable at Body[2:4].
	if binary.BigEndian.Uint16(m.Body[0:2]) != 0 {
		return false
	}
	attrLen := int(binary.BigEndian.Uint16(m.Body[2:4]))
	// The NLRI field is whatever trails the attributes, and an EoR has none.
	if len(m.Body) != 4+attrLen {
		return false
	}
	if attrLen == 0 {
		return true // the legacy encoding: a completely empty UPDATE
	}
	return isBareMPUnreach(m.Body[4 : 4+attrLen])
}

// isBareMPUnreach reports whether an attribute section is exactly one
// MP_UNREACH_NLRI attribute whose value is AFI(2) + SAFI(1) and nothing else,
// which is the RFC 4724 Section 2 multiprotocol End-of-RIB marker.
func isBareMPUnreach(attrs []byte) bool {
	const (
		mpUnreachCode  = 15
		flagExtendedLn = 0x10
		afiSafiLen     = 3
	)
	if len(attrs) < 3 {
		return false
	}
	flags := attrs[0]
	hdrLen := 3
	valLen := int(attrs[2])
	if flags&flagExtendedLn != 0 {
		if len(attrs) < 4 {
			return false
		}
		hdrLen = 4
		valLen = int(binary.BigEndian.Uint16(attrs[2:4]))
	}
	// One attribute, exactly filling the section: a second one means this is not
	// a bare marker.
	if len(attrs) != hdrLen+valLen {
		return false
	}
	return attrs[1] == mpUnreachCode && valLen == afiSafiLen
}

// Stream returns the hex-encoded message.
func (m *Message) Stream() string {
	return textbuf.StringHexUpper(append(m.Header, m.Body...))
}

// ReadMessage reads a BGP message from a connection.
func ReadMessage(conn net.Conn) ([]byte, []byte, error) {
	header := make([]byte, HeaderLen)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, nil, err
	}

	length := binary.BigEndian.Uint16(header[16:18])
	if length < HeaderLen {
		return nil, nil, fmt.Errorf("invalid message length: %d", length)
	}

	bodyLen := int(length) - HeaderLen
	body := make([]byte, bodyLen)
	if bodyLen > 0 {
		if _, err := io.ReadFull(conn, body); err != nil {
			return nil, nil, err
		}
	}

	return header, body, nil
}

// KeepaliveMsg returns a BGP KEEPALIVE message.
func KeepaliveMsg() []byte {
	msg := make([]byte, 19)
	copy(msg, Marker)
	binary.BigEndian.PutUint16(msg[16:], 19)
	msg[18] = MsgKEEPALIVE
	return msg
}

// DefaultRouteMsg returns an UPDATE with route 0.0.0.0/32.
// Used for testing UPDATE receive handling.
func DefaultRouteMsg() []byte {
	return []byte{
		// BGP Header (16 bytes marker + 2 bytes length + 1 byte type)
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0x00, 0x31, // Length: 49 bytes (19 header + 30 body)
		0x02, // Type: UPDATE
		// UPDATE body (30 bytes)
		0x00, 0x00, // Withdrawn routes length: 0
		0x00, 0x15, // Path attributes length: 21
		// ORIGIN: IGP (0) - 4 bytes
		0x40, 0x01, 0x01, 0x00,
		// AS_PATH: empty - 3 bytes
		0x40, 0x02, 0x00,
		// NEXT_HOP: 127.0.0.1 - 7 bytes
		0x40, 0x03, 0x04, 0x7F, 0x00, 0x00, 0x01,
		// LOCAL_PREF: 100 - 7 bytes
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64,
		// NLRI: 0.0.0.0/32 - 5 bytes
		0x20,                   // Prefix length: 32 bits
		0x00, 0x00, 0x00, 0x00, // Prefix: 0.0.0.0
	}
}

// RouteToSend describes a custom route for ze-peer to send after OPEN.
// Parsed from option=update:value=send-route:prefix=X:origin-as=Y:next-hop=Z.
type RouteToSend struct {
	Prefix   string // CIDR prefix (e.g. "10.0.1.0/24")
	OriginAS uint32 // Origin ASN for AS_PATH
	NextHop  string // Next-hop IP (e.g. "10.0.0.1")
	ASSet    bool   // Use AS_SET instead of AS_SEQUENCE

	// Extended fields for loop detection tests.
	ASPath       []uint32 // Explicit AS_PATH sequence (overrides OriginAS if set)
	OriginatorID uint32   // ORIGINATOR_ID attribute (type 9); 0 = omit
	ClusterList  []uint32 // CLUSTER_LIST attribute (type 10); nil = omit

	// Labeled unicast (SAFI 4) fields.
	Labels []uint32 // MPLS label stack; nil = not labeled unicast
}

// BuildRouteMsg constructs a BGP UPDATE message with the given route.
// Uses 4-byte ASN encoding (ASN4 capability assumed).
func BuildRouteMsg(route RouteToSend) ([]byte, error) {
	if len(route.Labels) > 0 {
		return buildLabeledRouteMsg(route)
	}
	return buildIPv4RouteMsg(route)
}

func buildIPv4RouteMsg(route RouteToSend) ([]byte, error) {
	// Parse prefix
	prefixIP, prefixNet, err := net.ParseCIDR(route.Prefix)
	if err != nil {
		return nil, fmt.Errorf("invalid prefix %q: %w", route.Prefix, err)
	}
	_ = prefixIP
	prefixLen, _ := prefixNet.Mask.Size()

	// Parse next-hop
	nhIP := net.ParseIP(route.NextHop)
	if nhIP == nil {
		return nil, fmt.Errorf("invalid next-hop %q", route.NextHop)
	}
	nhIP = nhIP.To4()
	if nhIP == nil {
		return nil, fmt.Errorf("next-hop must be IPv4: %q", route.NextHop)
	}

	// Build path attributes
	// ORIGIN: IGP (0) - flags=0x40, type=1, len=1, value=0
	origin := []byte{0x40, 0x01, 0x01, 0x00}

	// AS_PATH with 4-byte ASNs
	segType := byte(0x02) // AS_SEQUENCE
	if route.ASSet {
		segType = 0x01 // AS_SET
	}
	asns := route.ASPath
	if len(asns) == 0 && route.OriginAS != 0 {
		asns = []uint32{route.OriginAS}
	}
	asPathData := make([]byte, 0, 2+len(asns)*4)
	if len(asns) > 0 {
		asPathData = append(asPathData, segType, byte(len(asns)))
		for _, asn := range asns {
			asPathData = append(asPathData, byte(asn>>24), byte(asn>>16), byte(asn>>8), byte(asn))
		}
	}
	asPath := make([]byte, 0, 3+len(asPathData))
	asPath = append(asPath, 0x40, 0x02, byte(len(asPathData)))
	asPath = append(asPath, asPathData...)

	// NEXT_HOP - flags=0x40, type=3, len=4
	nextHop := make([]byte, 0, 7)
	nextHop = append(nextHop, 0x40, 0x03, 0x04)
	nextHop = append(nextHop, nhIP...)

	// LOCAL_PREF: 100 - flags=0x40, type=5, len=4
	localPref := []byte{0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64}

	// Total path attributes
	attrs := make([]byte, 0, len(origin)+len(asPath)+len(nextHop)+len(localPref)+12)
	attrs = append(attrs, origin...)
	attrs = append(attrs, asPath...)
	attrs = append(attrs, nextHop...)
	attrs = append(attrs, localPref...)

	// Optional: ORIGINATOR_ID (type 9, RFC 4456)
	if route.OriginatorID != 0 {
		oid := route.OriginatorID
		attrs = append(attrs, 0x80, 0x09, 0x04,
			byte(oid>>24), byte(oid>>16), byte(oid>>8), byte(oid))
	}

	// Optional: CLUSTER_LIST (type 10, RFC 4456)
	if len(route.ClusterList) > 0 {
		clLen := len(route.ClusterList) * 4
		attrs = append(attrs, 0x80, 0x0A, byte(clLen))
		for _, cl := range route.ClusterList {
			attrs = append(attrs, byte(cl>>24), byte(cl>>16), byte(cl>>8), byte(cl))
		}
	}

	// NLRI: prefix-length byte + prefix bytes (ceiling division)
	nlriBytes := (prefixLen + 7) / 8
	nlri := make([]byte, 1+nlriBytes)
	nlri[0] = byte(prefixLen) //nolint:gosec // prefixLen is 0-32 for IPv4
	copy(nlri[1:], prefixNet.IP.To4()[:nlriBytes])

	// Build UPDATE body: withdrawn(2) + attr-len(2) + attrs + nlri
	bodyLen := 2 + 2 + len(attrs) + len(nlri)
	body := make([]byte, 0, bodyLen)
	body = append(body, 0x00, 0x00, byte(len(attrs)>>8), byte(len(attrs))) //nolint:gosec // len(attrs) < 256
	body = append(body, attrs...)
	body = append(body, nlri...)

	// Build full message: header + body
	msgLen := HeaderLen + len(body)
	msg := make([]byte, 0, msgLen)
	msg = append(msg, Marker...)
	header := []byte{byte(msgLen >> 8), byte(msgLen), MsgUPDATE} //nolint:gosec // msgLen < 4096
	msg = append(msg, header...)
	msg = append(msg, body...)

	return msg, nil
}

// buildLabeledRouteMsg constructs a BGP UPDATE with MP_REACH_NLRI for
// ipv4/mpls-label (AFI 1, SAFI 4). The label is encoded per RFC 8277.
func buildLabeledRouteMsg(route RouteToSend) ([]byte, error) {
	_, prefixNet, err := net.ParseCIDR(route.Prefix)
	if err != nil {
		return nil, fmt.Errorf("invalid prefix %q: %w", route.Prefix, err)
	}
	prefixLen, _ := prefixNet.Mask.Size()

	nhIP := net.ParseIP(route.NextHop)
	if nhIP == nil {
		return nil, fmt.Errorf("invalid next-hop %q", route.NextHop)
	}
	nhIP = nhIP.To4()
	if nhIP == nil {
		return nil, fmt.Errorf("next-hop must be IPv4: %q", route.NextHop)
	}

	origin := []byte{0x40, 0x01, 0x01, 0x00}

	segType := byte(0x02)
	asns := route.ASPath
	if len(asns) == 0 && route.OriginAS != 0 {
		asns = []uint32{route.OriginAS}
	}
	asPathData := make([]byte, 0, 2+len(asns)*4)
	if len(asns) > 0 {
		asPathData = append(asPathData, segType, byte(len(asns)))
		for _, asn := range asns {
			asPathData = append(asPathData, byte(asn>>24), byte(asn>>16), byte(asn>>8), byte(asn))
		}
	}
	asPath := make([]byte, 0, 3+len(asPathData))
	asPath = append(asPath, 0x40, 0x02, byte(len(asPathData)))
	asPath = append(asPath, asPathData...)

	localPref := []byte{0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64}

	// RFC 8277 labeled NLRI: [totalBits][label(3)*N][prefix-bytes]
	labelBytes := make([]byte, 0, len(route.Labels)*3)
	for i, label := range route.Labels {
		sbit := byte(0x00)
		if i == len(route.Labels)-1 {
			sbit = 0x01
		}
		labelBytes = append(labelBytes, byte(label>>12), byte(label>>4), byte(label<<4)|sbit)
	}
	prefixBytes := (prefixLen + 7) / 8
	totalBits := len(route.Labels)*24 + prefixLen
	nlri := make([]byte, 0, 1+3+prefixBytes)
	nlri = append(nlri, byte(totalBits)) //nolint:gosec // totalBits < 256
	nlri = append(nlri, labelBytes...)
	nlri = append(nlri, prefixNet.IP.To4()[:prefixBytes]...)

	// MP_REACH_NLRI (type 14): AFI(2) + SAFI(1) + NH-len(1) + NH(4) + reserved(1) + NLRI
	mpReachValue := make([]byte, 0, 2+1+1+4+1+len(nlri))
	mpReachValue = append(mpReachValue, 0x00, 0x01, 0x04, 0x04) // AFI 1, SAFI 4, NH-len 4
	mpReachValue = append(mpReachValue, nhIP...)
	mpReachValue = append(mpReachValue, 0x00) // reserved
	mpReachValue = append(mpReachValue, nlri...)

	// flags 0x90 = optional(0x80) + extended-length(0x10) for MP_REACH_NLRI
	mpReach := make([]byte, 0, 4+len(mpReachValue))
	mpReach = append(mpReach, 0x90, 0x0E, byte(len(mpReachValue)>>8), byte(len(mpReachValue)))
	mpReach = append(mpReach, mpReachValue...)

	attrs := make([]byte, 0, len(origin)+len(asPath)+len(localPref)+len(mpReach))
	attrs = append(attrs, origin...)
	attrs = append(attrs, asPath...)
	attrs = append(attrs, localPref...)
	attrs = append(attrs, mpReach...)

	// UPDATE body: withdrawn(2) + attr-len(2) + attrs (no legacy NLRI)
	body := make([]byte, 0, 4+len(attrs))
	body = append(body, 0x00, 0x00, byte(len(attrs)>>8), byte(len(attrs))) //nolint:gosec // len < 4096
	body = append(body, attrs...)

	msgLen := HeaderLen + len(body)
	msg := make([]byte, 0, msgLen)
	msg = append(msg, Marker...)
	msg = append(msg, byte(msgLen>>8), byte(msgLen), MsgUPDATE) //nolint:gosec // msgLen < 4096
	msg = append(msg, body...)

	return msg, nil
}

// NotificationMsg builds a BGP NOTIFICATION message with Cease/Administrative Shutdown.
// RFC 4271 Section 4.5 - NOTIFICATION Message Format.
// RFC 9003 - Extended BGP Administrative Shutdown Communication.
//
// Format: [Error Code 6][Subcode 2][Length][Shutdown Communication]
// - Error Code: 6 (Cease)
// - Subcode: 2 (Administrative Shutdown)
// - Length: 1 byte (0-255)
// - Shutdown Communication: UTF-8, max 255 bytes per RFC 9003.
func NotificationMsg(text string) []byte {
	textBytes := []byte(text)

	// RFC 9003: max 255 octets for shutdown communication
	// Must truncate at valid UTF-8 boundary to maintain RFC compliance
	if len(textBytes) > 255 {
		textBytes = truncateUTF8(textBytes, 255)
	}

	// Header (19) + Error Code (1) + Subcode (1) + Length (1) + Text
	msgLen := 19 + 3 + len(textBytes)

	msg := make([]byte, msgLen)
	copy(msg, Marker)
	binary.BigEndian.PutUint16(msg[16:], uint16(msgLen)) //nolint:gosec // msgLen max 277
	msg[18] = MsgNOTIFICATION
	msg[19] = 6                    // Cease
	msg[20] = 2                    // Administrative Shutdown (RFC 9003)
	msg[21] = byte(len(textBytes)) // Length of shutdown communication
	copy(msg[22:], textBytes)

	return msg
}

// truncateUTF8 truncates bytes to maxLen while preserving valid UTF-8.
// It finds the last valid rune boundary at or before maxLen.
func truncateUTF8(b []byte, maxLen int) []byte {
	if len(b) <= maxLen {
		return b
	}

	// Start at maxLen and work backwards to find valid UTF-8 boundary
	for i := maxLen; i > 0; i-- {
		if utf8.RuneStart(b[i]) {
			// Found a rune start - check if there's room for the full rune
			_, size := utf8.DecodeRune(b[i:])
			if i+size <= maxLen {
				return b[:i+size]
			}
			// Rune would exceed maxLen, try previous position
			continue
		}
	}

	// Fallback: no valid boundary found (shouldn't happen with valid UTF-8)
	return b[:maxLen]
}

func isTimeout(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}

// isConnReset reports whether err is a "connection reset by peer" error.
// This happens when the remote side closes with unread data, sending RST
// instead of FIN. Treated like EOF for test purposes: the remote side
// sent its final message (e.g., NOTIFICATION) and closed.
// Uses errors.Is which traverses the full Unwrap chain, matching the
// pattern in runner_validate.go:isTransientConnError.
func isConnReset(err error) bool {
	return errors.Is(err, syscall.ECONNRESET)
}
