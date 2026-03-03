package editor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

func TestCompleterCommands(t *testing.T) {
	c := NewCompleter()

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
	c := NewCompleter()

	// "set " at bgp context should show bgp children from YANG
	completions := c.Complete("set ", []string{"bgp"})
	require.NotEmpty(t, completions)

	texts := completionTexts(completions)
	assert.Contains(t, texts, "local-as")
	assert.Contains(t, texts, "router-id")
	assert.Contains(t, texts, "peer")
}

func TestCompleterSetPartialKeyword(t *testing.T) {
	c := NewCompleter()

	// "set local" should complete to "local-as" in bgp context
	completions := c.Complete("set local", []string{"bgp"})
	require.NotEmpty(t, completions)

	texts := completionTexts(completions)
	assert.Contains(t, texts, "local-as")
}

func TestCompleterNestedPath(t *testing.T) {
	c := NewCompleter()

	// Inside bgp.peer context, should show peer children from YANG
	completions := c.Complete("set ", []string{"bgp", "peer"})
	require.NotEmpty(t, completions)

	texts := completionTexts(completions)
	assert.Contains(t, texts, "peer-as")
	assert.Contains(t, texts, "address")
}

func TestCompleterValueTypeHints(t *testing.T) {
	c := NewCompleter()

	// After "set router-id " inside bgp context should hint value
	completions := c.Complete("set router-id ", []string{"bgp"})
	require.NotEmpty(t, completions)
	assert.Equal(t, "value", completions[0].Type)
}

func TestCompleterGhostTextSingleMatch(t *testing.T) {
	c := NewCompleter()

	// "set router" should ghost "-id" (single match) inside bgp context
	ghost := c.GhostText("set router", []string{"bgp"})
	assert.Equal(t, "-id", ghost)
}

func TestCompleterGhostTextNoMatch(t *testing.T) {
	c := NewCompleter()

	// "set xyz" has no matches
	ghost := c.GhostText("set xyz", []string{"bgp"})
	assert.Empty(t, ghost)
}

func TestCompleterEditPath(t *testing.T) {
	c := NewCompleter()

	// "edit " inside bgp should show peer (a list)
	completions := c.Complete("edit ", []string{"bgp"})
	require.NotEmpty(t, completions)

	texts := completionTexts(completions)
	assert.Contains(t, texts, "peer")
}

func TestCompleterYANGDescription(t *testing.T) {
	c := NewCompleter()

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
	c := NewCompleter()

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

func TestCompleterEnumValues(t *testing.T) {
	c := NewCompleter()

	// In capability/add-path context, should show send/receive children
	completions := c.Complete("set ", []string{"bgp", "peer", "capability", "add-path"})
	require.NotEmpty(t, completions)

	texts := completionTexts(completions)
	assert.Contains(t, texts, "receive")
	assert.Contains(t, texts, "send")
}

// TestCompleterSetListKeys verifies that "set bgp peer " shows list key completions.
//
// VALIDATES: Navigating to a list via tokens shows list keys, not schema children.
// PREVENTS: "set bgp peer <tab>" showing peer-as instead of peer IPs.
func TestCompleterSetListKeys(t *testing.T) {
	c := NewCompleter()

	// "set bgp peer " should show list key hints (* for template, <value> for new)
	// NOT schema children like peer-as, address, etc.
	completions := c.Complete("set bgp peer ", nil)
	require.NotEmpty(t, completions)

	texts := completionTexts(completions)
	// Should show wildcard and value hint for list keys
	assert.Contains(t, texts, "*", "should show wildcard for template")
	assert.Contains(t, texts, "<value>", "should show value hint for new key")
	// Should NOT show schema children (those are for inside a peer)
	assert.NotContains(t, texts, "peer-as", "should not show peer-as (that's inside peer)")
	assert.NotContains(t, texts, "address", "should not show address (that's inside peer)")
}

// TestCompleterListKeysInContext verifies that list key completions work
// when the context path includes list entries (e.g., inside a peer).
//
// VALIDATES: Navigating the config tree through list entries finds sublist keys.
// PREVENTS: "edit update <tab>" inside a peer showing only <value> instead of existing keys.
func TestCompleterListKeysInContext(t *testing.T) {
	c := NewCompleter()

	// Build a tree: bgp { peer 1.1.1.1 { update { ... } update named { ... } } }
	tree := config.NewTree()
	bgp := config.NewTree()
	tree.SetContainer("bgp", bgp)

	peer := config.NewTree()
	peer.Set("peer-as", "65001")
	bgp.AddListEntry("peer", "1.1.1.1", peer)

	update1 := config.NewTree()
	peer.AddListEntry("update", config.KeyDefault, update1)

	update2 := config.NewTree()
	peer.AddListEntry("update", "named", update2)

	c.SetTree(tree)

	// "edit update " inside peer context should show #N for unnamed, actual key for named
	contextPath := []string{"bgp", "peer", "1.1.1.1"}
	completions := c.Complete("edit update ", contextPath)
	require.NotEmpty(t, completions)

	texts := completionTexts(completions)
	assert.Contains(t, texts, "*", "should show wildcard")
	assert.Contains(t, texts, "#1", "unnamed default entry shown as #1")
	assert.Contains(t, texts, "named", "named entry shown by actual key")
	// Should NOT show raw internal KeyDefault
	assert.NotContains(t, texts, config.KeyDefault, "should not show raw 'default' key")
}

// TestCompleterListKeySingleEntry verifies that a single list entry
// does not show key completions — the user should not be asked to pick.
//
// VALIDATES: Single-entry lists auto-select without requiring a key.
// PREVENTS: Asking for a key when there's only one option.
func TestCompleterListKeySingleEntry(t *testing.T) {
	c := NewCompleter()

	// Build a tree: bgp { peer 1.1.1.1 { update { ... } } }
	tree := config.NewTree()
	bgp := config.NewTree()
	tree.SetContainer("bgp", bgp)

	peer := config.NewTree()
	bgp.AddListEntry("peer", "1.1.1.1", peer)

	update := config.NewTree()
	peer.AddListEntry("update", config.KeyDefault, update)

	c.SetTree(tree)

	// "edit update " with only one entry should return empty — no key needed
	contextPath := []string{"bgp", "peer", "1.1.1.1"}
	completions := c.Complete("edit update ", contextPath)
	assert.Empty(t, completions, "single entry should not show key completions")
}

func completionTexts(completions []Completion) []string {
	texts := make([]string, len(completions))
	for i, c := range completions {
		texts[i] = c.Text
	}
	return texts
}
