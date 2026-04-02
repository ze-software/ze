// Design: docs/architecture/core-design.md -- ExaBGP env file migration
// Overview: migrate.go -- ExaBGP migration orchestration
//
// Implements parsing of ExaBGP INI-format environment files and mapping
// to Ze configuration output. ExaBGP uses Python configparser format
// with [exabgp.<section>] headers and key = value lines.
// Reference: https://github.com/Exa-Networks/exabgp/blob/main/lib/exabgp/environment/setup.py

package migration

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/exabgp/topics"
)

// sectionLog is the section name for ExaBGP logging config.
const sectionLog = "lo" + "g"

// ExaEnvEntry represents a parsed key-value pair from an ExaBGP env file.
type ExaEnvEntry struct {
	Section string // Section name without "exabgp." prefix (e.g., "daemon", sectionLog)
	Key     string // Key within section (e.g., "user", "packets")
	Value   string // Raw value string
}

// ParseExaBGPEnv parses an ExaBGP INI-format environment file into entries.
// Lines starting with # or ; are comments. Sections are [exabgp.<name>].
// Non-exabgp sections are silently ignored.
func ParseExaBGPEnv(input string) ([]ExaEnvEntry, error) {
	var entries []ExaEnvEntry
	var currentSection string
	seenSection := false

	for lineNum, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments.
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}

		// Section header.
		if line[0] == '[' && line[len(line)-1] == ']' {
			seenSection = true
			section := line[1 : len(line)-1]
			if strings.HasPrefix(section, "exabgp.") {
				currentSection = section[len("exabgp."):]
			} else {
				currentSection = "" // Non-exabgp section, ignore keys.
			}
			continue
		}

		// Key = value.
		keyPart, valuePart, hasEquals := strings.Cut(line, "=")
		if !hasEquals {
			if !seenSection {
				return nil, fmt.Errorf("line %d: key without section: %s", lineNum+1, line)
			}
			continue
		}

		if !seenSection {
			return nil, fmt.Errorf("line %d: key without section: %s", lineNum+1, line)
		}

		// Skip keys in non-exabgp sections.
		if currentSection == "" {
			continue
		}

		key := strings.TrimSpace(keyPart)
		value := strings.TrimSpace(valuePart)

		entries = append(entries, ExaEnvEntry{
			Section: currentSection,
			Key:     key,
			Value:   value,
		})
	}

	return entries, nil
}

// envTopicToSubsystem is the canonical ExaBGP topic-to-Ze-subsystem mapping.
var envTopicToSubsystem = topics.TopicToSubsystem

// MapEnvToZe converts parsed ExaBGP env entries to Ze configuration output.
// Returns a string with Ze config lines and comments for unsupported keys.
// All log-related output (subsystem levels, level, destination) is merged into
// a single `environment { log { ... } }` block.
func MapEnvToZe(entries []ExaEnvEntry) string {
	var b strings.Builder

	// Collect subsystem levels (debug wins over disabled for duplicates).
	subsystems := make(map[string]string)
	// Collect log.level and log.destination for merged output.
	var logLevel, logDestination string
	var configLines []string

	for _, e := range entries {
		fullKey := e.Section + "." + e.Key

		// Topic booleans have dynamic keys, check first.
		if e.Section == sectionLog && isEnvLogTopic(e.Key) {
			subsystem := envTopicToSubsystem[e.Key]
			level := "disabled"
			if e.Value == "true" {
				level = "debug"
			}
			if existing, exists := subsystems[subsystem]; exists {
				if existing == "debug" {
					continue
				}
			}
			subsystems[subsystem] = level
			continue
		}

		// Collect log.level and log.destination for merged block.
		logPrefix := sectionLog + "."
		if fullKey == logPrefix+"level" {
			logLevel = strings.ToLower(e.Value)
			continue
		}
		if fullKey == logPrefix+"destination" {
			logDestination = e.Value
			continue
		}

		if !mapEnvKnownKey(fullKey, e.Value, &b, &configLines) {
			fmt.Fprintf(&b, "# %s = %s -- unknown ExaBGP setting\n", fullKey, e.Value)
		}
	}

	// Emit a single merged log block with subsystem levels, level, and destination.
	hasLogContent := len(subsystems) > 0 || logLevel != "" || logDestination != ""
	if hasLogContent {
		b.WriteString("environment {\n    " + sectionLog + " {\n")
		if logLevel != "" {
			fmt.Fprintf(&b, "        level %s;\n", logLevel)
		}
		if logDestination != "" {
			fmt.Fprintf(&b, "        destination %s;\n", logDestination)
		}
		for _, sub := range sortedMapKeys(subsystems) {
			fmt.Fprintf(&b, "        %s %s;\n", sub, subsystems[sub])
		}
		b.WriteString("    }\n}\n")
	}

	for _, line := range configLines {
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// mapEnvKnownKey handles a single env key, writing output to b or appending to configLines.
// Returns true if the key was recognized, false otherwise.
// Adding a new case to the switch is sufficient -- no separate list to update.
func mapEnvKnownKey(fullKey, value string, b *strings.Builder, configLines *[]string) bool {
	logPrefix := sectionLog + "."

	switch fullKey {
	case "tcp.bind", "tcp.port":
		fmt.Fprintf(b, "# %s = %s -- per-peer config in Ze, not global\n", fullKey, value)
		return true

	case "bgp.connect", "bgp.accept":
		fmt.Fprintf(b, "# %s = %s -- per-peer config in Ze\n", fullKey, value)
		return true

	case "debug.pdb":
		fmt.Fprintf(b, "# %s = %s -- Python-only, not applicable to Ze\n", fullKey, value)
		return true

	case logPrefix + "level", logPrefix + "destination":
		// Handled by merged log block in MapEnvToZe.
		return true

	case logPrefix + "enable", logPrefix + "all", logPrefix + "short":
		fmt.Fprintf(b, "# %s = %s -- Ze uses per-subsystem levels\n", fullKey, value)
		return true

	case "daemon.user":
		*configLines = append(*configLines, fmt.Sprintf("environment {\n    daemon {\n        user %s;\n    }\n}", value))
		return true

	case "api.encoder":
		*configLines = append(*configLines, fmt.Sprintf("# api.encoder = %s -- Ze uses JSON format", value))
		return true

	case "api.respawn":
		*configLines = append(*configLines, fmt.Sprintf("# api.respawn = %s -- Ze manages plugin lifecycle", value))
		return true

	case "daemon.drop", "daemon.daemonize", "daemon.pid", "cache.attributes", "cache.nexthops":
		fmt.Fprintf(b, "# %s = %s -- not applicable to Ze\n", fullKey, value)
		return true
	}

	return false
}

// ValidateEnvEntries validates parsed env entries for correctness.
// Returns an error if any entry has an invalid value.
func ValidateEnvEntries(entries []ExaEnvEntry) error {
	for _, e := range entries {
		if e.Section == "tcp" && e.Key == "port" {
			port, err := strconv.Atoi(e.Value)
			if err != nil {
				return fmt.Errorf("tcp.port: invalid port %q: %w", e.Value, err)
			}
			if port < 1 || port > 65535 {
				return fmt.Errorf("tcp.port: port %d out of range 1-65535", port)
			}
		}
	}
	return nil
}

// isEnvLogTopic returns true if the key is a known ExaBGP topic boolean.
func isEnvLogTopic(key string) bool {
	_, ok := envTopicToSubsystem[key]
	return ok
}

// sortedMapKeys returns map keys in sorted order.
func sortedMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
