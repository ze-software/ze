// Design: docs/research/l2tpv2-ze-integration.md -- Router Advertisement for PPP IPv6
// Related: ncp.go -- onNCPOpened triggers RA after IPv6CP completes
// Related: session_run.go -- afterLCPOpen starts RA goroutine
// Related: ra_parity_test.go -- pins the wire bytes this file produces

package ppp

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/ndp"
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

// BuildRA writes a Router Advertisement into buf and returns the number of
// bytes written. The caller must ensure buf is large enough (16 + 8 +
// 16*len(RDNSS) bytes); a buffer that is too small writes nothing and returns
// 0. No prefix information is included because BNG uses DHCPv6-PD (M+O flags
// direct the subscriber to use DHCPv6), and no Source Link-layer Address option
// is included because pppN is a point-to-point link with no link-layer address.
//
// The encoding lives in internal/core/ndp so the LAN Router Advertisement
// sender emits the same wire format. RFC 4861 Section 4.2.
func BuildRA(buf []byte, cfg RAConfig) int {
	return ndp.BuildRA(buf, 0, ndp.RAConfig{
		CurHopLimit:    cfg.CurHopLimit,
		Managed:        cfg.Managed,
		OtherConfig:    cfg.OtherConfig,
		RouterLifetime: cfg.RouterLifetime,
		ReachableTime:  cfg.ReachableTime,
		RetransTimer:   cfg.RetransTimer,
		RDNSS:          cfg.RDNSS,
		RDNSSLifetime:  cfg.RDNSSLifetime,
	})
}
