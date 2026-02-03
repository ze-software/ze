// Package parse provides shared value parsers for BGP attributes.
// These parsers are used by both config parsing and API command parsing.
package parse

import (
	"fmt"
	"strings"
)

// Origin parses a BGP ORIGIN attribute string value.
// RFC 4271 Section 5.1.1: ORIGIN is a well-known mandatory attribute.
//
// Valid values:
//   - "igp" or "" (empty) → 0 (IGP)
//   - "egp" → 1 (EGP)
//   - "incomplete" or "?" → 2 (INCOMPLETE)
//
// Input is case-insensitive.
func Origin(s string) (uint8, error) {
	switch strings.ToLower(s) {
	case "", "igp":
		return 0, nil
	case "egp":
		return 1, nil
	case "incomplete", "?":
		return 2, nil
	}
	return 0, fmt.Errorf("invalid origin %q: valid values are igp, egp, incomplete", s)
}

// OriginString returns the string representation of an ORIGIN value.
// RFC 4271 Section 5.1.1: 0=IGP, 1=EGP, 2=INCOMPLETE.
func OriginString(v uint8) string {
	switch v {
	case 0:
		return "igp"
	case 1:
		return "egp"
	case 2:
		return "incomplete"
	}
	return fmt.Sprintf("unknown(%d)", v)
}
