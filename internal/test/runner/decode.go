// Design: docs/architecture/testing/ci-format.md — test runner framework

package runner

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/decode"
)

// Re-export shared types and functions for use within the runner package.
// This avoids updating every callsite in report.go/diff.go/etc.
type DecodedMessage = decode.DecodedMessage
type DecodedAttribute = decode.DecodedAttribute

var (
	DecodeMessage      = decode.DecodeMessage
	DecodeMessageBytes = decode.DecodeMessageBytes
	Diff               = decode.Diff
	AttrCodeName       = decode.AttrCodeName
)

// ColoredString returns a colored human-readable representation.
func ColoredString(m *decode.DecodedMessage, c *Colors) string {
	var sb textbuf.Buffer

	sb.Str("  ").Str(c.Cyan("type:")).Str("      ").Str(m.Type).Str(" (len=").Int(int64(m.Length)).Str(")\n")

	for _, attr := range m.Attributes {
		sb.Str("    ").Str(c.Gray(attr.Name)).Str(": ").Str(attr.Value).Byte(10)
	}

	if len(m.NLRI) > 0 {
		sb.Str("  ").Str(c.Gray("nlri:")).Str("      ").Join(m.NLRI, ", ").Byte(10)
	}

	if len(m.Withdrawn) > 0 {
		sb.Str("  ").Str(c.Gray("withdrawn:")).Byte(' ').Join(m.Withdrawn, ", ").Byte(10)
	}

	return sb.String()
}

// ColoredDiff compares two messages with colored output.
func ColoredDiff(expected, received string, c *Colors) string {
	expMsg, expErr := decode.DecodeMessage(expected)
	rcvMsg, rcvErr := decode.DecodeMessage(received)

	var sb textbuf.Buffer

	if expErr != nil || rcvErr != nil {
		// Use plain diff when colored decode fails
		return decode.Diff(expected, received)
	}

	// Build maps for comparison
	expAttrs := make(map[string]string)
	rcvAttrs := make(map[string]string)

	for _, a := range expMsg.Attributes {
		expAttrs[a.Name] = a.Value
	}
	for _, a := range rcvMsg.Attributes {
		rcvAttrs[a.Name] = a.Value
	}

	// Find differences
	allKeys := make(map[string]bool)
	for k := range expAttrs {
		allKeys[k] = true
	}
	for k := range rcvAttrs {
		allKeys[k] = true
	}

	var tb textbuf.Buffer
	for key := range allKeys {
		expVal, hasExp := expAttrs[key]
		rcvVal, hasRcv := rcvAttrs[key]

		switch {
		case !hasExp:
			sb.Str("  ").Str(key).Str(": ").Str(c.Red(tb.Reset().Byte('+').Str(rcvVal).String())).Str(" (unexpected)\n")
		case !hasRcv:
			sb.Str("  ").Str(key).Str(": ").Str(c.Green(tb.Reset().Byte('-').Str(expVal).String())).Str(" (missing)\n")
		case expVal != rcvVal:
			sb.Str("  ").Str(key).Str(": ").Str(c.Green(tb.Reset().Byte('-').Str(expVal).String())).Byte(' ').Str(c.Red(tb.Reset().Byte('+').Str(rcvVal).String())).Byte(10)
		}
	}

	// NLRI differences
	expNLRI := textbuf.Join(expMsg.NLRI, ",")
	rcvNLRI := textbuf.Join(rcvMsg.NLRI, ",")
	if expNLRI != rcvNLRI {
		sb.Str("  NLRI: ").Str(c.Green(tb.Reset().Byte('-').Str(expNLRI).String())).Byte(' ').Str(c.Red(tb.Reset().Byte('+').Str(rcvNLRI).String())).Byte(10)
	}

	// Find byte-level differences
	byteDiff := decode.FindByteDiff(expected, received)
	if byteDiff != "" {
		sb.Str("  ").Str(c.Gray("raw diff:")).Byte(' ').Str(byteDiff).Byte(10)
	}

	return sb.String()
}
