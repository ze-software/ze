// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS attribute encoding
// RFC: rfc/short/rfc2869.md -- NAS-Port-Id (Section 5.17)
// RFC: rfc/short/rfc2865.md -- attribute Length is one octet (Section 5)
// Related: handler.go -- Access-Request emission
// Related: acct.go -- Accounting-Request emission

package l2tpauthradius

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// nasPortIDFacts holds the session facts a NAS-Port-Id template can name.
// Every field is known at BOTH emission points -- the Access-Request (before
// any interface exists) and every Accounting-Request -- so one session resolves
// to one text and a billing system can join the records.
type nasPortIDFacts struct {
	nasID     string
	tunnelID  uint16
	sessionID uint16
}

// nasPortIDPlaceholders names every placeholder resolveNASPortID understands.
// validateNASPortIDFormat and the config error message both read this list, so
// a new placeholder is added in one place (ai/rules/evidence.md).
var nasPortIDPlaceholders = []string{"nas-id", "tunnel-id", "session-id"}

// maxNASPortIDLen is the largest attribute value RADIUS can carry: RFC 2865
// Section 5 gives Length one octet, and Type + Length take two of it.
const maxNASPortIDLen = radius.MaxAttrLen - 2

// resolveNASPortID expands a validated format string against one session's
// facts. An unknown placeholder yields the empty string rather than partial
// text: the caller omits the attribute, so a template that escaped validation
// cannot put its own syntax on the wire.
func resolveNASPortID(format string, facts nasPortIDFacts) string {
	if format == "" {
		return ""
	}
	var b textbuf.Buffer
	b.Reset()
	for i := 0; i < len(format); {
		c := format[i]
		if c != '{' {
			b.Byte(c)
			i++
			continue
		}
		end := strings.IndexByte(format[i:], '}')
		if end < 0 {
			return ""
		}
		name := format[i+1 : i+end]
		switch name {
		case "nas-id":
			b.Str(facts.nasID)
		case "tunnel-id":
			b.Int(int64(facts.tunnelID))
		case "session-id":
			b.Int(int64(facts.sessionID))
		default:
			return ""
		}
		i += end + 1
	}
	return b.String()
}

// validateNASPortIDFormat refuses a format ze cannot resolve. It runs when the
// config is parsed, so an operator learns at commit time rather than by reading
// literal template syntax out of a RADIUS server's logs, or by finding no
// attribute at all at the server.
func validateNASPortIDFormat(format string) error {
	// RFC 2865 Section 5: an attribute Length is one octet. A template longer
	// than the value it can produce is refused here, so the daemon never
	// expands one per packet only to drop the result.
	if len(format) > maxNASPortIDLen {
		return fmt.Errorf("%d characters exceeds the %d-octet maximum of a RADIUS attribute value",
			len(format), maxNASPortIDLen)
	}
	for i := 0; i < len(format); {
		if format[i] != '{' {
			i++
			continue
		}
		end := strings.IndexByte(format[i:], '}')
		if end < 0 {
			return fmt.Errorf("unterminated placeholder at offset %d; supported placeholders: %s",
				i, nasPortIDPlaceholderList())
		}
		name := format[i+1 : i+end]
		if !slices.Contains(nasPortIDPlaceholders, name) {
			return fmt.Errorf("unknown placeholder %q; supported placeholders: %s",
				name, nasPortIDPlaceholderList())
		}
		i += end + 1
	}
	return nil
}

// nasPortIDPlaceholderList renders the supported placeholders for an error
// message, in the same braces an operator types.
func nasPortIDPlaceholderList() string {
	var b textbuf.Buffer
	b.Reset()
	for i, name := range nasPortIDPlaceholders {
		if i > 0 {
			b.Str(", ")
		}
		b.Byte('{').Str(name).Byte('}')
	}
	return b.String()
}

// nasPortIDAttr resolves the template and returns the NAS-Port-Id attribute,
// or false when none is to be sent.
func nasPortIDAttr(format string, facts nasPortIDFacts) (radius.Attr, bool) {
	return nasPortIDAttrFromText(resolveNASPortID(format, facts))
}

// nasPortIDAttrFromText wraps an already-resolved NAS-Port-Id. RFC 2869 Section
// 5.17 gives the attribute a minimum Length of 3 (Type, Length, and at least one
// text octet), so an empty resolution is no attribute at all, and a value past
// the one-octet Length is dropped rather than truncated. Both cases are refused
// when the config is committed (validateNASPortIDFormat); this is the guard that
// keeps a malformed value off the wire if one ever reaches here.
func nasPortIDAttrFromText(value string) (radius.Attr, bool) {
	if value == "" || len(value) > maxNASPortIDLen {
		return radius.Attr{}, false
	}
	return radius.Attr{Type: radius.AttrNASPortID, Value: radius.AttrString(value)}, true
}

// validateNASPortIDResolution refuses a format whose widest resolution cannot
// fit a RADIUS attribute value. This is a different bound from the one
// validateNASPortIDFormat applies: that one measures the TEMPLATE, and a
// template is shorter than what it resolves to. `{nas-id}` is nine characters
// that expand to the whole NAS-Identifier, which the YANG leaf does not bound,
// and `{tunnel-id}` and `{session-id}` each expand to as many as five digits.
// Without this check an operator commits a template that passes every other
// guard, and then finds no NAS-Port-Id in any packet, because
// nasPortIDAttrFromText drops the over-long value at the moment of emission and
// says nothing.
//
// The widest resolution is the one to measure: the tunnel and session
// identifiers are uint16, so 65535 is the longest text either can produce, and
// a format that fits at that width fits at every other.
func validateNASPortIDResolution(format, nasID string) error {
	widest := resolveNASPortID(format, nasPortIDFacts{
		nasID:     nasID,
		tunnelID:  math.MaxUint16,
		sessionID: math.MaxUint16,
	})
	if len(widest) > maxNASPortIDLen {
		return fmt.Errorf("resolves to %d octets for nas-identifier %q, past the %d-octet maximum of a RADIUS attribute value",
			len(widest), nasID, maxNASPortIDLen)
	}
	return nil
}
