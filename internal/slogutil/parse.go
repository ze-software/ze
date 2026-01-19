package slogutil

import (
	"log/slog"
	"strings"
)

// ParseLogLine extracts level, message, and attributes from a slog text line.
// Returns []any (not []slog.Attr) so result can be spread to slog.Group().
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
	levelStr := line[levelStart+6 : levelStart+6+levelEnd]
	level := parseSlogLevel(levelStr)

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
		msg = line[msgValueStart+1 : msgEnd]
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
func findClosingQuote(s string, start int) int {
	for i := start; i < len(s); i++ {
		if s[i] == '"' && (i == start || s[i-1] != '\\') {
			return i
		}
	}
	return -1
}

// parseAttrs extracts key=value pairs from remaining line.
// Returns []any for use with slog.Group().
func parseAttrs(s string) []any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	var attrs []any
	for _, part := range strings.Fields(s) {
		idx := strings.Index(part, "=")
		if idx == -1 {
			continue
		}
		key := part[:idx]
		value := part[idx+1:]
		// Remove quotes from value if present
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		attrs = append(attrs, key, value)
	}
	return attrs
}
