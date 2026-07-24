// Design: docs/architecture/config/syntax.md — config parsing and loading

package config

import "strings"

// ConfigType represents the type of configuration detected.
type ConfigType string

// Config type constants.
const (
	ConfigTypeBGP     ConfigType = "bgp"
	ConfigTypeHub     ConfigType = "hub"
	ConfigTypeUnknown ConfigType = "unknown"
)

// ProbeConfigType scans config content for top-level blocks without full parsing.
// Handles both hierarchical format (bgp { ... }) and set format (set bgp ...).
// Returns ConfigTypeBGP for bgp config, ConfigTypeHub for plugin config, ConfigTypeUnknown otherwise.
// BGP takes precedence if both blocks are present.
func ProbeConfigType(content string) ConfigType {
	// Try set-format detection first (handles both set and set-with-meta formats).
	format := DetectFormat(content)
	if format == FormatSet || format == FormatSetMeta {
		return probeSetFormat(content)
	}

	// Hierarchical format: look for top-level bgp { } or plugin { } blocks.
	tok := NewTokenizer(content)

	hasBGP := false
	hasPlugin := false
	// hasOther records any top-level block that is neither `plugin` nor the
	// hub-native `env`: an ospf/firewall/environment/... block means the config
	// drives the full YANG daemon (which runs built-in protocols AND forks the
	// external plugins declared in `plugin {}`, wiring a TLS acceptor for them),
	// not the standalone plugin-hub orchestrator (which runs no built-in protocol
	// and never sets an acceptor). Without this, an OSPF-only config with an
	// external observer plugin misroutes to the orchestrator: built-in OSPF never
	// starts and the observer dies "no TLS acceptor configured".
	hasOther := false
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
			if depth == 0 {
				next := tok.Peek()
				if next.Type == TokenLBrace {
					switch t.Value {
					case string(ConfigTypeBGP):
						hasBGP = true
					case sectionPlugin:
						hasPlugin = true
					case "env":
						// hub-native block; on its own it neither forces the
						// full daemon nor is a YANG protocol indicator.
					default:
						hasOther = true
					}
				}
			}
		}
	}

	if hasBGP {
		return ConfigTypeBGP
	}
	// A YANG config block outside the hub-native set routes to the full daemon,
	// even alongside a `plugin {}` block. Only a pure plugin/env hub is ConfigTypeHub.
	if hasOther {
		return ConfigTypeUnknown
	}
	if hasPlugin {
		return ConfigTypeHub
	}
	return ConfigTypeUnknown
}

// probeSetFormat scans set-format content for "set bgp" or "set plugin" lines.
// Metadata prefixes (#user @source %time) are skipped to find the set command.
func probeSetFormat(content string) ConfigType {
	hasBGP := false
	hasPlugin := false
	// hasOther mirrors the hierarchical path: a `set ospf ...` (or any block
	// outside the hub-native plugin/env set) means the committed config drives
	// the full YANG daemon, not the plugin-hub orchestrator. A committed
	// OSPF + external-plugin config serializes to set format and is probed on
	// `ze start`/reboot, so without this the reboot path keeps the misroute.
	hasOther := false

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

		switch fields[1] {
		case string(ConfigTypeBGP):
			hasBGP = true
		case sectionPlugin:
			hasPlugin = true
		case "env":
			// hub-native block; not a YANG-daemon indicator on its own.
		default:
			hasOther = true
		}
	}

	if hasBGP {
		return ConfigTypeBGP
	}
	if hasOther {
		return ConfigTypeUnknown
	}
	if hasPlugin {
		return ConfigTypeHub
	}
	return ConfigTypeUnknown
}
