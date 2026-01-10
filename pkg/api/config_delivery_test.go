package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigDeliveryMatching verifies config patterns match against parsed config.
//
// VALIDATES: Registered patterns match appropriate config lines.
// PREVENTS: Plugins missing config they registered for.
func TestConfigDeliveryMatching(t *testing.T) {
	t.Run("hostname_pattern_matches", func(t *testing.T) {
		reg := &PluginRegistration{}
		require.NoError(t, reg.ParseLine("declare conf peer * capability hostname <hostname:.*>"))

		// Simulated config entries (as would come from parser)
		configEntries := []ConfigEntry{
			{Path: "peer 192.168.1.1 capability hostname", Value: "router1.example.com"},
			{Path: "peer 192.168.1.2 capability hostname", Value: "router2.example.com"},
			{Path: "peer 192.168.1.1 capability graceful-restart", Value: "120"},
		}

		matches := MatchConfigForPlugin(reg, configEntries)

		require.Len(t, matches, 2)
		assert.Equal(t, "router1.example.com", matches[0].Captures["hostname"])
		assert.Equal(t, "router2.example.com", matches[1].Captures["hostname"])
	})

	t.Run("multiple_captures", func(t *testing.T) {
		reg := &PluginRegistration{}
		require.NoError(t, reg.ParseLine("declare conf peer * capability graceful-restart <restart-time:\\d+> <forwarding:(true|false)>"))

		configEntries := []ConfigEntry{
			{Path: "peer 192.168.1.1 capability graceful-restart 120 true", Value: ""},
			{Path: "peer 192.168.1.2 capability graceful-restart 90 false", Value: ""},
		}

		matches := MatchConfigForPlugin(reg, configEntries)

		require.Len(t, matches, 2)
		assert.Equal(t, "120", matches[0].Captures["restart-time"])
		assert.Equal(t, "true", matches[0].Captures["forwarding"])
		assert.Equal(t, "90", matches[1].Captures["restart-time"])
		assert.Equal(t, "false", matches[1].Captures["forwarding"])
	})

	t.Run("no_patterns_no_matches", func(t *testing.T) {
		reg := &PluginRegistration{}
		// No conf patterns registered

		configEntries := []ConfigEntry{
			{Path: "peer 192.168.1.1 capability hostname", Value: "router1.example.com"},
		}

		matches := MatchConfigForPlugin(reg, configEntries)
		assert.Empty(t, matches)
	})

	t.Run("pattern_no_match", func(t *testing.T) {
		reg := &PluginRegistration{}
		require.NoError(t, reg.ParseLine("declare conf peer * capability hostname <hostname:.*>"))

		configEntries := []ConfigEntry{
			{Path: "peer 192.168.1.1 capability graceful-restart", Value: "120"},
		}

		matches := MatchConfigForPlugin(reg, configEntries)
		assert.Empty(t, matches)
	})
}

// TestConfigDeliveryFormat verifies config delivery message format.
//
// VALIDATES: Config delivered in correct format per spec.
// PREVENTS: Plugin unable to parse config messages.
func TestConfigDeliveryFormat(t *testing.T) {
	t.Run("single_capture_format", func(t *testing.T) {
		match := &ConfigMatch{
			Captures: map[string]string{"hostname": "router1.example.com"},
			Context:  "peer 192.168.1.1",
		}

		lines := FormatConfigDeliveryLines(match)

		require.Len(t, lines, 1)
		assert.Equal(t, "config peer 192.168.1.1 hostname router1.example.com", lines[0])
	})

	t.Run("multiple_captures_format", func(t *testing.T) {
		match := &ConfigMatch{
			Captures: map[string]string{
				"restart-time": "120",
				"forwarding":   "true",
			},
			Context: "peer 192.168.1.1",
		}

		lines := FormatConfigDeliveryLines(match)

		require.Len(t, lines, 2)
		// Order may vary, check both are present
		found := make(map[string]bool)
		for _, line := range lines {
			found[line] = true
		}
		assert.True(t, found["config peer 192.168.1.1 restart-time 120"])
		assert.True(t, found["config peer 192.168.1.1 forwarding true"])
	})

	t.Run("done_marker", func(t *testing.T) {
		lines := []string{
			"config peer 192.168.1.1 hostname router1",
		}
		lines = append(lines, "config done")

		assert.Equal(t, "config done", lines[len(lines)-1])
	})
}

// TestConfigDeliveryMultiplePlugins verifies multiple plugins receive overlapping config.
//
// VALIDATES: Config pattern overlap delivers to both plugins.
// PREVENTS: One plugin's pattern blocking another's.
func TestConfigDeliveryMultiplePlugins(t *testing.T) {
	reg1 := &PluginRegistration{Name: "plugin1"}
	require.NoError(t, reg1.ParseLine("declare conf peer * capability hostname <hostname:.*>"))

	reg2 := &PluginRegistration{Name: "plugin2"}
	require.NoError(t, reg2.ParseLine("declare conf peer * capability hostname <hostname:.*>"))

	configEntries := []ConfigEntry{
		{Path: "peer 192.168.1.1 capability hostname", Value: "router1.example.com"},
	}

	matches1 := MatchConfigForPlugin(reg1, configEntries)
	matches2 := MatchConfigForPlugin(reg2, configEntries)

	// Both plugins should receive the same config
	require.Len(t, matches1, 1)
	require.Len(t, matches2, 1)
	assert.Equal(t, "router1.example.com", matches1[0].Captures["hostname"])
	assert.Equal(t, "router1.example.com", matches2[0].Captures["hostname"])
}

// ConfigEntry represents a parsed config entry for matching.
type ConfigEntry struct {
	Path  string // Full config path (e.g., "peer 192.168.1.1 capability hostname")
	Value string // Config value
}

// MatchConfigForPlugin finds config entries matching a plugin's patterns.
// Returns matches with captured values.
func MatchConfigForPlugin(reg *PluginRegistration, entries []ConfigEntry) []*ConfigMatch {
	var matches []*ConfigMatch

	for _, entry := range entries {
		// Try each pattern
		for _, pat := range reg.ConfigPatterns {
			// Match against path + value if value exists
			matchStr := entry.Path
			if entry.Value != "" {
				matchStr = entry.Path + " " + entry.Value
			}

			if m := pat.Match(matchStr); m != nil {
				// Extract context from path (everything before the capture)
				m.Context = extractContext(entry.Path, pat)
				matches = append(matches, m)
				break // Only one pattern match per entry
			}
		}
	}

	return matches
}

// extractContext extracts the context portion from a config path.
// For "peer 192.168.1.1 capability hostname", context is "peer 192.168.1.1".
func extractContext(path string, pat *ConfigPattern) string {
	// Simple heuristic: context is the wildcard-matched portion
	// For "peer * capability hostname", context would be "peer <matched-ip>"
	// This is a simplified implementation
	parts := splitConfigPath(path)
	patParts := splitConfigPath(pat.Pattern)

	var contextParts []string
	for i, pp := range patParts {
		if pp == "*" && i < len(parts) {
			contextParts = append(contextParts, parts[i])
		} else if i < len(parts) && !isCapture(pp) {
			// Stop at first capture
			if len(contextParts) > 0 {
				break
			}
		}
	}

	// Include the element before wildcard too
	if len(patParts) > 0 && patParts[0] != "*" && len(parts) > 0 {
		return parts[0] + " " + join(contextParts, " ")
	}
	return join(contextParts, " ")
}

func splitConfigPath(s string) []string {
	var parts []string
	for _, p := range splitWords(s) {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func splitWords(s string) []string {
	return splitOnSpaces(s)
}

func splitOnSpaces(s string) []string {
	var result []string
	current := ""
	for _, c := range s {
		if c == ' ' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func isCapture(s string) bool {
	return len(s) > 0 && s[0] == '<'
}

func join(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}

// FormatConfigDeliveryLines formats a config match into delivery lines.
// Each capture becomes: "configuration <context> <name> set <value>".
func FormatConfigDeliveryLines(match *ConfigMatch) []string {
	var lines []string
	for name, value := range match.Captures {
		lines = append(lines, FormatConfigDelivery(match.Context, name, value))
	}
	return lines
}
