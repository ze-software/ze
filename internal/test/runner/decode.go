package runner

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/test/decode"
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
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("  %s%s%s (len=%d)\n", c.Cyan("type:"), "      ", m.Type, m.Length))

	for _, attr := range m.Attributes {
		sb.WriteString(fmt.Sprintf("    %s: %s\n", c.Gray(attr.Name), attr.Value))
	}

	if len(m.NLRI) > 0 {
		sb.WriteString(fmt.Sprintf("  %s%s%s\n", c.Gray("nlri:"), "      ", strings.Join(m.NLRI, ", ")))
	}

	if len(m.Withdrawn) > 0 {
		sb.WriteString(fmt.Sprintf("  %s%s%s\n", c.Gray("withdrawn:"), " ", strings.Join(m.Withdrawn, ", ")))
	}

	return sb.String()
}

// ColoredDiff compares two messages with colored output.
func ColoredDiff(expected, received string, c *Colors) string {
	expMsg, expErr := decode.DecodeMessage(expected)
	rcvMsg, rcvErr := decode.DecodeMessage(received)

	var sb strings.Builder

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

	for key := range allKeys {
		expVal, hasExp := expAttrs[key]
		rcvVal, hasRcv := rcvAttrs[key]

		switch {
		case !hasExp:
			sb.WriteString(fmt.Sprintf("  %s: %s (unexpected)\n", key, c.Red("+"+rcvVal)))
		case !hasRcv:
			sb.WriteString(fmt.Sprintf("  %s: %s (missing)\n", key, c.Green("-"+expVal)))
		case expVal != rcvVal:
			sb.WriteString(fmt.Sprintf("  %s: %s %s\n", key, c.Green("-"+expVal), c.Red("+"+rcvVal)))
		}
	}

	// NLRI differences
	expNLRI := strings.Join(expMsg.NLRI, ",")
	rcvNLRI := strings.Join(rcvMsg.NLRI, ",")
	if expNLRI != rcvNLRI {
		sb.WriteString(fmt.Sprintf("  NLRI: %s %s\n", c.Green("-"+expNLRI), c.Red("+"+rcvNLRI)))
	}

	// Find byte-level differences
	byteDiff := decode.FindByteDiff(expected, received)
	if byteDiff != "" {
		sb.WriteString(fmt.Sprintf("  %s %s\n", c.Gray("raw diff:"), byteDiff))
	}

	return sb.String()
}
