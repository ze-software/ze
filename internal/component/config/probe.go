// Design: docs/architecture/config/syntax.md — config parsing and loading

package config

import "strings"

// ConfigType represents the type of configuration detected.
type ConfigType string

// Config type constants.
const (
	ConfigTypeBGP     ConfigType = "bgp"
	ConfigTypeUnknown ConfigType = "unknown"
)

// ProbeConfigType reports whether config content declares a top-level bgp
// block, without a full parse. It handles the hierarchical format (bgp { ... })
// and the set format (set bgp ...).
//
// Every config runs one daemon on one parser (cmd/ze/hub/main.go runYANGConfig),
// so this answer never selects a runtime. Its one caller that acts on the
// difference is the config editor, which starts a web-only daemon for a config
// that declares no bgp block (internal/component/config/cli/cmd_edit.go).
func ProbeConfigType(content string) ConfigType {
	// Try set-format detection first (handles both set and set-with-meta formats).
	format := DetectFormat(content)
	if format == FormatSet || format == FormatSetMeta {
		return probeSetFormat(content)
	}

	// Hierarchical format: look for a top-level bgp { } block.
	tok := NewTokenizer(content)
	depth := 0

	for {
		t := tok.Next()
		if t.Type == TokenEOF {
			break
		}

		switch t.Type { //nolint:exhaustive // Only care about braces and words
		case TokenLBrace:
			depth++
		case TokenRBrace:
			depth--
		case TokenWord:
			if depth == 0 && t.Value == string(ConfigTypeBGP) && tok.Peek().Type == TokenLBrace {
				return ConfigTypeBGP
			}
		}
	}

	return ConfigTypeUnknown
}

// probeSetFormat scans set-format content for a "set bgp" or "delete bgp" line.
// Metadata prefixes (#user @source %time) are skipped to find the set command.
func probeSetFormat(content string) ConfigType {
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || (line[0] == '#' && (len(line) == 1 || line[1] == ' ')) {
			continue // blank or comment (# or # text)
		}

		// Skip metadata prefixes: #user @source %time ^previous
		// Uses simple space-delimited splitting (not quote-aware for ^previous).
		// This is fine because ProbeConfigType is called on committed configs,
		// which do not contain ^previous metadata.
		for line != "" && (line[0] == '#' || line[0] == '@' || line[0] == '%' || line[0] == '^') {
			idx := strings.IndexByte(line, ' ')
			if idx < 0 {
				line = ""
				break
			}
			line = strings.TrimSpace(line[idx+1:])
		}

		// Now line should start with "set bgp ..." or "delete bgp ..." etc.
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		verb := fields[0]
		if verb != cmdSet && verb != cmdDelete {
			continue
		}

		if fields[1] == string(ConfigTypeBGP) {
			return ConfigTypeBGP
		}
	}

	return ConfigTypeUnknown
}
