// Design: docs/architecture/wire/nlri.md -- SR-Policy NLRI plugin
// RFC: rfc/short/rfc9830.md -- SR-Policy NLRI wire format (SAFI 73)
//
// Package srpolicy implements SR-Policy NLRI (SAFI 73, RFC 9830).
//
// Wire format per MP_REACH_NLRI / MP_UNREACH_NLRI:
//
//	[length-bits:1][distinguisher:4][color:4][endpoint:4|16]
//
// The length byte contains the bit count of the body:
//   - IPv4 (AFI 1): 96 bits  (12 bytes: 4+4+4)
//   - IPv6 (AFI 2): 192 bits (24 bytes: 4+4+16)
package srpolicy

import (
	"encoding/binary"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Fixed NLRI body sizes (excluding 1-byte length prefix).
const (
	ipv4BodyLen = 12 // distinguisher(4) + color(4) + endpoint(4)
	ipv6BodyLen = 24 // distinguisher(4) + color(4) + endpoint(16)
)

var (
	ErrSRPolicyTruncated = errors.New("srpolicy: truncated data")
)

// SRPolicy represents an SR-Policy NLRI (RFC 9830 Section 2.1).
type SRPolicy struct {
	distinguisher uint32
	color         uint32
	endpoint      netip.Addr
	afi           family.AFI
}

// New creates an SR-Policy NLRI from semantic values.
func New(afi family.AFI, distinguisher, color uint32, endpoint netip.Addr) *SRPolicy {
	return &SRPolicy{
		distinguisher: distinguisher,
		color:         color,
		endpoint:      endpoint,
		afi:           afi,
	}
}

// Parse parses an SR-Policy NLRI from the body bytes (after the
// 1-byte length prefix has been consumed by the splitter).
func Parse(afi family.AFI, body []byte) (*SRPolicy, error) {
	expected := bodyLen(afi)
	if len(body) < expected {
		return nil, ErrSRPolicyTruncated
	}
	sp := &SRPolicy{
		distinguisher: binary.BigEndian.Uint32(body[0:4]),
		color:         binary.BigEndian.Uint32(body[4:8]),
		afi:           afi,
	}
	if afi == family.AFIIPv6 {
		sp.endpoint = netip.AddrFrom16([16]byte(body[8:24]))
	} else {
		sp.endpoint = netip.AddrFrom4([4]byte(body[8:12]))
	}
	return sp, nil
}

// Family returns the address family.
func (s *SRPolicy) Family() family.Family {
	return family.Family{AFI: s.afi, SAFI: family.SAFISRPolicy}
}

// Distinguisher returns the 4-byte distinguisher field.
func (s *SRPolicy) Distinguisher() uint32 { return s.distinguisher }

// Color returns the 4-byte policy color.
func (s *SRPolicy) Color() uint32 { return s.color }

// Endpoint returns the endpoint address.
func (s *SRPolicy) Endpoint() netip.Addr { return s.endpoint }

// Len returns the wire-format length including the 1-byte length prefix.
func (s *SRPolicy) Len() int { return 1 + bodyLen(s.afi) }

// WriteTo writes the SR-Policy NLRI to buf at offset, including the
// 1-byte length prefix. Returns the number of bytes written.
func (s *SRPolicy) WriteTo(buf []byte, off int) int {
	blen := bodyLen(s.afi)
	buf[off] = byte(blen * 8) // RFC 9830: length in bits
	binary.BigEndian.PutUint32(buf[off+1:], s.distinguisher)
	binary.BigEndian.PutUint32(buf[off+5:], s.color)
	if s.afi == family.AFIIPv6 {
		a := s.endpoint.As16()
		copy(buf[off+9:], a[:])
	} else {
		a := s.endpoint.As4()
		copy(buf[off+9:], a[:])
	}
	return 1 + blen
}

// Bytes allocates a standalone slice and delegates to WriteTo.
func (s *SRPolicy) Bytes() []byte {
	buf := make([]byte, s.Len())
	s.WriteTo(buf, 0)
	return buf
}

// PathID returns 0 (SR-Policy does not use ADD-PATH).
func (s *SRPolicy) PathID() uint32 { return 0 }

// HasPathID returns false.
func (s *SRPolicy) HasPathID() bool { return false }

// SupportsAddPath returns false.
func (s *SRPolicy) SupportsAddPath() bool { return false }

// String returns a human-readable representation.
func (s *SRPolicy) String() string {
	var tb textbuf.Buffer
	return tb.Str("sr-policy d=").Uint32(s.distinguisher).
		Str(" c=").Uint32(s.color).
		Str(" ep=").Str(s.endpoint.String()).String()
}

func bodyLen(afi family.AFI) int {
	if afi == family.AFIIPv6 {
		return ipv6BodyLen
	}
	return ipv4BodyLen
}
