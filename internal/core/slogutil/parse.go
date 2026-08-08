// Design: docs/architecture/config/environment.md — structured logging utilities

package slogutil

import (
	"log/slog"
	"strconv"
	"strings"
)

// ParseLogLine extracts level, message, and attributes from a slog text line.
// Returns []any (not []slog.Attr) so result can be spread to slog.Group().
//
// Quoted values are decoded, so what slog's TextHandler wrote is what comes
// back out: `msg="got \"x\""` yields `got "x"`. See unquoteValue for the two
// properties that keep an escape-free or malformed line unchanged.
//
// For valid slog format:
//
//	Input:  "time=... level=DEBUG msg="parsed config" subsystem=gr peer=..."
//	Output: LevelDebug, "parsed config", ["subsystem", "gr", "peer", "..."]
//
// For malformed/non-slog text (e.g., panic, raw error):
//
//	Input:  "panic: runtime error: index out of range"
//	Output: LevelInfo, "panic: runtime error: index out of range", []any{}
func ParseLogLine(line string) (slog.Level, string, []any) {
	if line == "" {
		return slog.LevelInfo, "", nil
	}

	// Try to find level= and msg= in the line
	levelStart := strings.Index(line, "level=")
	if levelStart == -1 {
		// Not a valid slog line, return as-is
		return slog.LevelInfo, line, nil
	}

	levelEnd := strings.Index(line[levelStart+6:], " ")
	if levelEnd == -1 {
		levelEnd = len(line) - levelStart - 6
	}
	level := parseSlogLevel(line[levelStart+6 : levelStart+6+levelEnd])

	// Parse msg
	msgStart := strings.Index(line, "msg=")
	if msgStart == -1 {
		// Has level but no msg - not valid slog format
		return slog.LevelInfo, line, nil
	}

	// Handle quoted message
	msgValueStart := msgStart + 4
	if msgValueStart >= len(line) {
		return slog.LevelInfo, line, nil
	}

	var msg string
	var attrs []any

	if line[msgValueStart] == '"' {
		// Quoted message
		msgEnd := findClosingQuote(line, msgValueStart+1)
		if msgEnd == -1 {
			return slog.LevelInfo, line, nil
		}
		msg = unquoteValue(line[msgValueStart+1 : msgEnd])
		// Parse remaining attrs after msg
		attrs = parseAttrs(line[msgEnd+1:])
	} else {
		// Unquoted message (until next space or end)
		msgEnd := strings.Index(line[msgValueStart:], " ")
		if msgEnd == -1 {
			msg = line[msgValueStart:]
		} else {
			msg = line[msgValueStart : msgValueStart+msgEnd]
			attrs = parseAttrs(line[msgValueStart+msgEnd:])
		}
	}

	return level, msg, attrs
}

// parseSlogLevel converts slog level string to slog.Level.
func parseSlogLevel(s string) slog.Level {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// findClosingQuote finds the closing quote index for a quoted string.
// Returns -1 if not found.
//
// A backslash escapes the byte that follows it, so the scan steps over that
// byte rather than testing the one before each quote. The difference is
// parity: `"foo\\"` encodes the value `foo\`, and its closing quote IS
// preceded by a backslash. Reading only the previous byte skipped that
// terminator and ran on into the attributes after it.
func findClosingQuote(s string, start int) int {
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++ // the escaped byte is never a terminator
		case '"':
			return i
		}
	}
	return -1
}

// unquoteValue decodes the escapes inside a quoted slog value. inner is the
// text BETWEEN the quotes, as findClosingQuote delimits it.
//
// slog's TextHandler writes a string value with strconv.Quote whenever it
// holds a space, an `=`, a quote, or a non-printable byte
// (log/slog/text_handler.go, needsQuoting). Returning that text undecoded made
// the parser lossy in one direction only: it looked correct until a value
// carried a quote, and then the relay re-escaped the backslashes it had left
// in place, doubling them at every hop.
//
// Two properties keep this safe for the lines already flowing through here.
// A value with no backslash cannot hold an escape, so it returns unchanged by
// the fast path -- byte-identical to the previous behavior. A value that is
// not valid Go quoted syntax (a hand-written line, or a truncated one) fails
// to unquote and also returns unchanged, so a malformed line degrades exactly
// as it did before rather than becoming empty.
func unquoteValue(inner string) string {
	if !strings.Contains(inner, `\`) {
		return inner
	}
	if decoded, err := strconv.Unquote(`"` + inner + `"`); err == nil {
		return decoded
	}
	return inner
}

// parseAttrs extracts key=value pairs from remaining line.
// Handles quoted values with spaces (e.g., error="rpc error: unknown command")
// and decodes their escapes. An unquoted value is returned verbatim: slog
// leaves a lone backslash unquoted, so `path=C:\temp` is a literal path and
// never an escape.
// Returns []any for use with slog.Group().
func parseAttrs(s string) []any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	var attrs []any
	for s != "" {
		// Skip leading whitespace
		s = strings.TrimLeft(s, " ")
		if s == "" {
			break
		}

		// Find key=value separator
		eqIdx := strings.Index(s, "=")
		if eqIdx == -1 {
			break
		}
		key := s[:eqIdx]
		s = s[eqIdx+1:]

		// Extract value: quoted or unquoted
		var value string
		if s != "" && s[0] == '"' {
			// Quoted value — find closing quote (respects backslash escapes)
			end := findClosingQuote(s, 1)
			if end == -1 {
				// Unterminated quote — take rest of line
				value = s[1:]
				s = ""
			} else {
				value = unquoteValue(s[1:end])
				s = s[end+1:]
			}
		} else {
			// Unquoted value — up to next space or end
			spIdx := strings.Index(s, " ")
			if spIdx == -1 {
				value = s
				s = ""
			} else {
				value = s[:spIdx]
				s = s[spIdx:]
			}
		}

		attrs = append(attrs, key, value)
	}
	return attrs
}
