// Design: docs/research/l2tpv2-ze-integration.md -- Router Advertisement for PPP IPv6
// Related: ncp.go -- onNCPOpened triggers RA after IPv6CP completes
// Related: session_run.go -- afterLCPOpen starts RA goroutine

package ppp

import (
	"encoding/binary"
	"net/netip"
)

// RFC 4861 Section 4.2: Router Advertisement message format.
const (
	icmpv6TypeRA  = 134
	ndOptRDNSS    = 25 // RFC 8106 Section 5.1
	raFlagManaged = 0x80
	raFlagOther   = 0x40
)

// RAConfig holds the parameters for building a Router Advertisement.
type RAConfig struct {
	CurHopLimit    uint8
	Managed        bool
	OtherConfig    bool
	RouterLifetime uint16 // seconds
	ReachableTime  uint32 // milliseconds, 0 = unspecified
	RetransTimer   uint32 // milliseconds, 0 = unspecified
	RDNSS          []netip.Addr
	RDNSSLifetime  uint32 // seconds
}

// BuildRA writes a Router Advertisement into buf and returns the
// number of bytes written. The caller must ensure buf is large
// enough (16 + 24*len(RDNSS) bytes minimum). No prefix information
// is included because BNG uses DHCPv6-PD (M+O flags direct the
// subscriber to use DHCPv6). RFC 4861 Section 4.2.
func BuildRA(buf []byte, cfg RAConfig) int {
	off := 0

	// ICMPv6 header
	buf[off] = icmpv6TypeRA // Type
	buf[off+1] = 0          // Code
	buf[off+2] = 0          // Checksum (computed by kernel for raw sockets)
	buf[off+3] = 0
	off += 4

	// Cur Hop Limit
	buf[off] = cfg.CurHopLimit
	off++

	// Flags
	var flags uint8
	if cfg.Managed {
		flags |= raFlagManaged
	}
	if cfg.OtherConfig {
		flags |= raFlagOther
	}
	buf[off] = flags
	off++

	// Router Lifetime
	binary.BigEndian.PutUint16(buf[off:], cfg.RouterLifetime)
	off += 2

	// Reachable Time
	binary.BigEndian.PutUint32(buf[off:], cfg.ReachableTime)
	off += 4

	// Retrans Timer
	binary.BigEndian.PutUint32(buf[off:], cfg.RetransTimer)
	off += 4

	// RDNSS option (RFC 8106 Section 5.1)
	if len(cfg.RDNSS) > 0 {
		buf[off] = ndOptRDNSS
		buf[off+1] = uint8(1 + 2*len(cfg.RDNSS))   // length in 8-byte units
		binary.BigEndian.PutUint16(buf[off+2:], 0) // reserved
		binary.BigEndian.PutUint32(buf[off+4:], cfg.RDNSSLifetime)
		off += 8
		for _, addr := range cfg.RDNSS {
			a := addr.As16()
			copy(buf[off:off+16], a[:])
			off += 16
		}
	}

	return off
}
