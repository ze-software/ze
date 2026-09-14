// Design: docs/architecture/config/syntax.md — parsing helpers
//
// Package parse provides shared value parsers for BGP attributes.
// These parsers are used by both config parsing and API command parsing.
package parse

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Origin parses a BGP ORIGIN attribute string value.
// RFC 4271 Section 5.1.1: ORIGIN is a well-known mandatory attribute.
//
// The three names are read from the attribute package, which is the one place
// Ze spells them (ai/rules/principles.md). This function adds the spellings the
// config and the ExaBGP API accept on top of them:
//   - "" (empty) → IGP, the config default
//   - "?" → INCOMPLETE, the ExaBGP API alias
//
// Input is case-insensitive.
func Origin(s string) (uint8, error) {
	switch s {
	case "":
		return uint8(attribute.OriginIGP), nil
	case "?":
		return uint8(attribute.OriginIncomplete), nil
	}
	if origin, ok := attribute.OriginFromText(strings.ToLower(s)); ok {
		return uint8(origin), nil
	}
	return 0, fmt.Errorf("invalid origin %q: valid values are %s", s, strings.Join(attribute.OriginTextNames(), ", "))
}

// OriginString returns the lowercase name of an ORIGIN value, as the attribute
// package spells it, or unknown(N) for a value RFC 4271 Section 5.1.1 does not
// define.
func OriginString(v uint8) string {
	if name := attribute.Origin(v).LowerString(); name != "" {
		return name
	}
	var b textbuf.Buffer
	return b.Reset().Str("unknown(").Int(int64(v)).Byte(')').String()
}
