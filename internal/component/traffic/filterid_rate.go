// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- Filter-Id rate parsing
// Related: config.go -- ParseRateBps, which reads one rate token

package traffic

import "strings"

// ParseFilterIDRate extracts the download and upload rates from a RADIUS
// Filter-Id value. Supported formats:
//
//   - "rate:<down>/<up>"  e.g. "rate:20mbit/5mbit"
//   - "rate:<symmetric>"  e.g. "rate:10mbit"
//   - "<down>/<up>"       e.g. "20mbit/5mbit"
//   - "<symmetric>"       e.g. "10mbit"
//
// Rate values use the same suffixes as ParseRateBps (bit/kbit/mbit/gbit/
// bps/kbps/mbps/gbps).
//
// It returns ok=false when the Filter-Id carries no parseable rate, because a
// Filter-Id is a free-form identifier that a deployment can use for something
// else entirely.
//
// This lives in the traffic component rather than in either consumer because
// the shaper plugin and the RADIUS CoA listener read the same attribute and
// MUST read it the same way. They did not: the CoA listener open-coded a
// ParseRateBps call over the whole string, so it accepted "rate:10mbit" and
// answered CoA-NAK to "rate:20mbit/5mbit", which is the value the shaper
// accepts at Access-Accept
// (plan/journal/helper-bypassed-by-an-open-coded-copy.md).
func ParseFilterIDRate(filterID string) (download, upload uint64, ok bool) {
	s := strings.TrimPrefix(filterID, "rate:")
	if s == "" {
		return 0, 0, false
	}

	if idx := strings.IndexByte(s, '/'); idx > 0 && idx < len(s)-1 {
		down, errD := ParseRateBps(s[:idx])
		up, errU := ParseRateBps(s[idx+1:])
		if errD != nil || errU != nil {
			return 0, 0, false
		}
		return down, up, true
	}

	rate, err := ParseRateBps(s)
	if err != nil {
		return 0, 0, false
	}
	return rate, rate, true
}
