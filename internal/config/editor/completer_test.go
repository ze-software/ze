package editor

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/config"
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

	// "set " should show top-level config keywords (now includes bgp, environment, etc.)
	completions := c.Complete("set ", nil)
	require.NotEmpty(t, completions)

	texts := completionTexts(completions)
	assert.Contains(t, texts, "bgp")
	assert.Contains(t, texts, "environment")
	assert.Contains(t, texts, "plugin")
}

func TestCompleterSetPartialKeyword(t *testing.T) {
	c := NewCompleter(config.BGPSchema())

	// "set bg" should complete to "bgp"
	completions := c.Complete("set bg", nil)
	require.NotEmpty(t, completions)

	texts := completionTexts(completions)
	assert.Contains(t, texts, "bgp")
}

func TestCompleterNestedPath(t *testing.T) {
	c := NewCompleter(config.BGPSchema())

	// Inside neighbor context (bgp.peer), should show neighbor fields
	completions := c.Complete("set ", []string{"bgp", "peer", "192.168.1.1"})
	require.NotEmpty(t, completions)

	texts := completionTexts(completions)
	assert.Contains(t, texts, "peer-as")
	assert.Contains(t, texts, "local-as")
	assert.Contains(t, texts, "hold-time")
}

func TestCompleterValueTypeHints(t *testing.T) {
	c := NewCompleter(config.BGPSchema())

	// After "set router-id " inside bgp context should hint IPv4
	completions := c.Complete("set router-id ", []string{"bgp"})
	require.NotEmpty(t, completions)
	assert.Equal(t, "value", completions[0].Type)
	assert.Contains(t, completions[0].Description, "IPv4")
}

func TestCompleterGhostTextSingleMatch(t *testing.T) {
	c := NewCompleter(config.BGPSchema())

	// "set router" should ghost "-id" (single match) inside bgp context
	ghost := c.GhostText("set router", []string{"bgp"})
	assert.Equal(t, "-id", ghost)
}

func TestCompleterGhostTextMultipleMatches(t *testing.T) {
	c := NewCompleter(config.BGPSchema())

	// "set local" could match "local-as" and "local-address" in neighbor context
	// At bgp level, only "local-as" exists
	ghost := c.GhostText("set local", []string{"bgp"})
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

	// "edit " inside bgp should show peer
	completions := c.Complete("edit ", []string{"bgp"})
	require.NotEmpty(t, completions)

	texts := completionTexts(completions)
	assert.Contains(t, texts, "peer")
}

func TestCompleterWildcard(t *testing.T) {
	c := NewCompleter(config.BGPSchema())

	// "edit peer " inside bgp should show list key completions
	// (including "*" only if there were glob patterns)
	completions := c.Complete("edit peer ", []string{"bgp"})
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

func TestCompleterWithYANG(t *testing.T) {
	c := NewCompleterWithYANG()

	// Should still complete commands
	completions := c.Complete("", nil)
	require.NotEmpty(t, completions)
	texts := completionTexts(completions)
	assert.Contains(t, texts, "set")

	// Should complete bgp children from YANG
	completions = c.Complete("set ", []string{"bgp"})
	require.NotEmpty(t, completions)
	texts = completionTexts(completions)
	assert.Contains(t, texts, "local-as")
	assert.Contains(t, texts, "router-id")
}

func TestCompleterYANGDescription(t *testing.T) {
	c := NewCompleterWithYANG()

	// Descriptions should come from YANG model
	completions := c.Complete("set ", []string{"bgp"})
	require.NotEmpty(t, completions)

	// Find local-as completion
	var localAS *Completion
	for i := range completions {
		if completions[i].Text == "local-as" {
			localAS = &completions[i]
			break
		}
	}
	require.NotNil(t, localAS, "local-as should be in completions")
	assert.NotEmpty(t, localAS.Description, "should have YANG description")
}

func TestCompleterYANGMandatory(t *testing.T) {
	c := NewCompleterWithYANG()

	// Mandatory fields should be marked in description
	completions := c.Complete("set ", []string{"bgp"})

	// Find local-as (mandatory) and peer (not mandatory - it's a list)
	var localAS, peer *Completion
	for i := range completions {
		switch completions[i].Text {
		case "local-as":
			localAS = &completions[i]
		case "peer":
			peer = &completions[i]
		}
	}

	require.NotNil(t, localAS, "local-as should be in completions")
	require.NotNil(t, peer, "peer should be in completions")

	// Mandatory should be indicated
	assert.Contains(t, localAS.Description, "required")
	// List is not mandatory
	assert.NotContains(t, peer.Description, "required")
}
