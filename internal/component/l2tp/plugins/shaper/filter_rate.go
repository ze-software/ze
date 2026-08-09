// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- Filter-Id rate parsing

package l2tpshaper

import (
	"strings"

	"github.com/ze-software/ze/internal/component/traffic"
)

// parseFilterRate extracts download and upload rates from a RADIUS
// Filter-Id value. Supported formats:
//
//   - "rate:<down>/<up>"  e.g. "rate:20mbit/5mbit"
//   - "rate:<symmetric>"  e.g. "rate:10mbit"
//   - "<down>/<up>"       e.g. "20mbit/5mbit"
//   - "<symmetric>"       e.g. "10mbit"
//
// Rate values use the same suffixes as traffic.ParseRateBps
// (bit/kbit/mbit/gbit/bps/kbps/mbps/gbps).
//
// Returns (0, 0, false) when the Filter-Id does not contain a
// parseable rate (it may be a non-rate filter identifier).
func parseFilterRate(filterID string) (download, upload uint64, ok bool) {
	s := strings.TrimPrefix(filterID, "rate:")
	if s == "" {
		return 0, 0, false
	}

	if idx := strings.IndexByte(s, '/'); idx > 0 && idx < len(s)-1 {
		down, errD := traffic.ParseRateBps(s[:idx])
		up, errU := traffic.ParseRateBps(s[idx+1:])
		if errD != nil || errU != nil {
			return 0, 0, false
		}
		return down, up, true
	}

	rate, err := traffic.ParseRateBps(s)
	if err != nil {
		return 0, 0, false
	}
	return rate, rate, true
}
