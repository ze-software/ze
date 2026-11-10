// Design: docs/contributing/ze-go-style.md -- the segments a format string writes
// Related: errorwrap.go -- the trace that compares them

package archerrorwrap

import (
	"strings"
)

// segmentSeparator is what joins one layer's context to the next in a Go
// error chain: `reload: parse config: line 3`.
const segmentSeparator = ": "

// verbMark stands in for a formatting verb in the literal text, so a segment
// holding one (`line %d`) is told apart from a literal one. No format string
// in Go source carries a NUL byte, which is why it is the mark.
const verbMark = '\x00'

// verbModifiers are the bytes that may sit between `%` and the verb letter:
// flags, width, precision, `*` and an explicit argument index.
const verbModifiers = "+-# 0123456789.*["

// format is what one format string says about the error it builds.
type format struct {
	// outer is the literal segments before the first %w, in order. Each one is
	// a whole part ending in ": " that holds no verb.
	outer []string
	// wrapped is the operand index of each %w verb, counted from the first
	// operand after the format string.
	wrapped []int
	// whole is the verb-free segments of a format with no %w: an error that
	// wraps nothing carries its whole text.
	whole []string
}

// produced answers the segments the built error carries itself.
func (f format) produced() []string {
	if len(f.wrapped) > 0 {
		return f.outer
	}
	return f.whole
}

// parseFormat reads a fmt format string. It answers false for a format whose
// operands it cannot number, one with an explicit argument index (`%[2]w`):
// a guessed operand would trace the wrong error.
func parseFormat(text string) (format, bool) {
	var form format
	var rendered strings.Builder
	operand := 0
	firstWrap := -1
	for cursor := 0; cursor < len(text); cursor++ {
		if text[cursor] != '%' {
			rendered.WriteByte(text[cursor])
			continue
		}
		if cursor+1 < len(text) && text[cursor+1] == '%' {
			rendered.WriteByte('%')
			cursor++
			continue
		}
		end := cursor + 1
		for end < len(text) && strings.IndexByte(verbModifiers, text[end]) >= 0 {
			if text[end] == '[' {
				return format{}, false
			}
			if text[end] == '*' {
				operand++
			}
			end++
		}
		if end >= len(text) {
			return format{}, false
		}
		if text[end] == 'w' {
			form.wrapped = append(form.wrapped, operand)
			if firstWrap < 0 {
				firstWrap = rendered.Len()
			}
		}
		rendered.WriteByte(verbMark)
		operand++
		cursor = end
	}
	built := rendered.String()
	if firstWrap < 0 {
		form.whole = literalSegments(strings.Split(built, segmentSeparator))
		return form, true
	}
	// The last part abuts the %w, so it is not a whole segment: in
	// `reload: parse config: %w` it is empty, and in `failed %w` it is the
	// sentence the wrapped error completes.
	parts := strings.Split(built[:firstWrap], segmentSeparator)
	form.outer = literalSegments(parts[:len(parts)-1])
	return form, true
}

// messageSegments answers the segments of a plain error message, the text of
// an errors.New.
func messageSegments(text string) []string {
	return literalSegments(strings.Split(text, segmentSeparator))
}

// literalSegments keeps the parts that are literal text, trimmed: an empty
// part, or one holding a verb, names nothing an operator could read twice.
func literalSegments(parts []string) []string {
	var segments []string
	for _, part := range parts {
		segment := strings.TrimSpace(part)
		if segment == "" {
			continue
		}
		if strings.IndexByte(segment, verbMark) >= 0 {
			continue
		}
		segments = append(segments, segment)
	}
	return segments
}
