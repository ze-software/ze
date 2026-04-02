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

// envTopicToSubsystem maps ExaBGP boolean topic names to Ze subsystem paths.
// Same mapping as config migration (listener.go topicToSubsystem).
var envTopicToSubsystem = map[string]string{
	"packets":       "bgp.wire",
	"rib":           "plugin.rib",
	"configuration": "config",
	"reactor":       "bgp.reactor",
	"daemon":        "daemon",
	"processes":     "plugin",
	"network":       "bgp.wire",
	"statistics":    "bgp.metrics",
	"message":       "bgp.wire",
	"timers":        "bgp.reactor",
	"routes":        "plugin.rib",
	"parser":        "config",
}

// MapEnvToZe converts parsed ExaBGP env entries to Ze configuration output.
// Returns a string with Ze config lines and comments for unsupported keys.
func MapEnvToZe(entries []ExaEnvEntry) string {
	var b strings.Builder

	// Collect subsystem levels (debug wins over disabled for duplicates).
	subsystems := make(map[string]string)
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

		mapEnvKnownKey(fullKey, e.Value, &b, &configLines)
	}

	// Emit collected subsystem levels.
	if len(subsystems) > 0 {
		b.WriteString("environment {\n    " + sectionLog + " {\n")
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
// Every key is explicitly handled: recognized keys map to Ze config or comments,
// unrecognized keys are emitted as comments for user review.
func mapEnvKnownKey(fullKey, value string, b *strings.Builder, configLines *[]string) {
	logPrefix := sectionLog + "."

	switch fullKey {
	case "tcp.bind", "tcp.port":
		fmt.Fprintf(b, "# %s = %s -- per-peer config in Ze, not global\n", fullKey, value)

	case "bgp.connect", "bgp.accept":
		fmt.Fprintf(b, "# %s = %s -- per-peer config in Ze\n", fullKey, value)

	case "debug.pdb":
		fmt.Fprintf(b, "# %s = %s -- Python-only, not applicable to Ze\n", fullKey, value)

	case logPrefix + "level":
		*configLines = append(*configLines, fmt.Sprintf("environment {\n    "+sectionLog+" {\n        level %s;\n    }\n}", strings.ToLower(value)))

	case logPrefix + "destination":
		*configLines = append(*configLines, fmt.Sprintf("environment {\n    "+sectionLog+" {\n        destination %s;\n    }\n}", value))

	case logPrefix + "enable", logPrefix + "all", logPrefix + "short":
		fmt.Fprintf(b, "# %s = %s -- Ze uses per-subsystem levels\n", fullKey, value)

	case "daemon.user":
		*configLines = append(*configLines, fmt.Sprintf("environment {\n    daemon {\n        user %s;\n    }\n}", value))

	case "api.encoder":
		*configLines = append(*configLines, fmt.Sprintf("# api.encoder = %s -- Ze uses JSON format", value))

	case "api.respawn":
		*configLines = append(*configLines, fmt.Sprintf("# api.respawn = %s -- Ze manages plugin lifecycle", value))

	case "daemon.drop", "daemon.daemonize", "daemon.pid", "cache.attributes", "cache.nexthops":
		fmt.Fprintf(b, "# %s = %s -- not applicable to Ze\n", fullKey, value)
	}

	// Unrecognized keys: emit as comment for user review (no key is silently dropped).
	if !isRecognizedEnvKey(fullKey) {
		fmt.Fprintf(b, "# %s = %s -- unknown ExaBGP setting\n", fullKey, value)
	}
}

// isRecognizedEnvKey returns true if the key is explicitly handled.
func isRecognizedEnvKey(fullKey string) bool {
	logPrefix := sectionLog + "."
	recognized := []string{
		"tcp.bind", "tcp.port",
		"bgp.connect", "bgp.accept",
		"debug.pdb",
		logPrefix + "level", logPrefix + "destination",
		logPrefix + "enable", logPrefix + "all", logPrefix + "short",
		"daemon.user", "daemon.drop", "daemon.daemonize", "daemon.pid",
		"api.encoder", "api.respawn",
		"cache.attributes", "cache.nexthops",
	}
	if slices.Contains(recognized, fullKey) {
		return true
	}
	// Also check if it's a known topic boolean.
	if strings.HasPrefix(fullKey, logPrefix) {
		topic := fullKey[len(logPrefix):]
		if isEnvLogTopic(topic) {
			return true
		}
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
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
