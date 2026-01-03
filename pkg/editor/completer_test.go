package editor

import (
	"testing"

	"github.com/exa-networks/zebgp/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompleterCommands(t *testing.T) {
	c := NewCompleter(config.BGPSchema())

	// Empty input should show commands
	completions := c.Complete("", nil)
	require.NotEmpty(t, completions)

	// Should include set, delete, edit, show, etc.
	texts := completionTexts(completions)
	assert.Contains(t, texts, "set")
	assert.Contains(t, texts, "delete")
	assert.Contains(t, texts, "edit")
	assert.Contains(t, texts, "show")
}

func TestCompleterSetKeywords(t *testing.T) {
	c := NewCompleter(config.BGPSchema())

	// "set " should show top-level config keywords
	completions := c.Complete("set ", nil)
	require.NotEmpty(t, completions)

	texts := completionTexts(completions)
	assert.Contains(t, texts, "router-id")
	assert.Contains(t, texts, "local-as")
	assert.Contains(t, texts, "peer")
}

func TestCompleterSetPartialKeyword(t *testing.T) {
	c := NewCompleter(config.BGPSchema())

	// "set local" should complete to "local-as"
	completions := c.Complete("set local", nil)
	require.NotEmpty(t, completions)

	texts := completionTexts(completions)
	assert.Contains(t, texts, "local-as")
}

func TestCompleterNestedPath(t *testing.T) {
	c := NewCompleter(config.BGPSchema())

	// Inside neighbor context, should show neighbor fields
	completions := c.Complete("set ", []string{"peer", "192.168.1.1"})
	require.NotEmpty(t, completions)

	texts := completionTexts(completions)
	assert.Contains(t, texts, "peer-as")
	assert.Contains(t, texts, "local-as")
	assert.Contains(t, texts, "hold-time")
}

func TestCompleterValueTypeHints(t *testing.T) {
	c := NewCompleter(config.BGPSchema())

	// After "set router-id " should hint IPv4
	completions := c.Complete("set router-id ", nil)
	require.NotEmpty(t, completions)
	assert.Equal(t, "value", completions[0].Type)
	assert.Contains(t, completions[0].Description, "IPv4")
}

func TestCompleterGhostTextSingleMatch(t *testing.T) {
	c := NewCompleter(config.BGPSchema())

	// "set router" should ghost "-id" (single match)
	ghost := c.GhostText("set router", nil)
	assert.Equal(t, "-id", ghost)
}

func TestCompleterGhostTextMultipleMatches(t *testing.T) {
	c := NewCompleter(config.BGPSchema())

	// "set local" could match "local-as" and "local-address" in neighbor context
	// At root level, only "local-as" exists
	ghost := c.GhostText("set local", nil)
	assert.Equal(t, "-as", ghost)
}

func TestCompleterGhostTextNoMatch(t *testing.T) {
	c := NewCompleter(config.BGPSchema())

	// "set xyz" has no matches
	ghost := c.GhostText("set xyz", nil)
	assert.Empty(t, ghost)
}

func TestCompleterEditPath(t *testing.T) {
	c := NewCompleter(config.BGPSchema())

	// "edit " should show list types (neighbor, process)
	completions := c.Complete("edit ", nil)
	require.NotEmpty(t, completions)

	texts := completionTexts(completions)
	assert.Contains(t, texts, "peer")
}

func TestCompleterWildcard(t *testing.T) {
	c := NewCompleter(config.BGPSchema())

	// "edit peer " should show list key completions
	// (including "*" only if there were glob patterns, which current schema doesn't have at root)
	completions := c.Complete("edit peer ", nil)
	// peer requires IP address, so completions may include existing peers or be empty
	// This test verifies the completer doesn't panic and handles list paths
	_ = completions
}

func completionTexts(completions []Completion) []string {
	texts := make([]string, len(completions))
	for i, c := range completions {
		texts[i] = c.Text
	}
	return texts
}
