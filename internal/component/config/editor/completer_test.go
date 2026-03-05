package editor

import (
	"strings"
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
	assert.NotContains(t, texts, "address", "list key 'address' should not appear in completions")
}

func TestCompleterValueTypeHints(t *testing.T) {
	c := NewCompleter()

	// After "set router-id " inside bgp context should hint value
	completions := c.Complete("set router-id ", []string{"bgp"})
	require.NotEmpty(t, completions)
	assert.Equal(t, "hint", completions[0].Type)
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

// TestCompleterListKeyTypedValueAccepted verifies that typing a new list key
// value and pressing Tab accepts it (offers it as a completion to add space).
//
// VALIDATES: Typed list key is accepted and offered as completion.
// PREVENTS: "set bgp peer 10.0.0.1<Tab>" replacing IP with "<value>" or doing nothing.
func TestCompleterListKeyTypedValueAccepted(t *testing.T) {
	c := NewCompleter()

	// No existing peers — empty tree
	tree := config.NewTree()
	bgp := config.NewTree()
	tree.SetContainer("bgp", bgp)
	c.SetTree(tree)

	// User has typed a new peer IP without trailing space
	completions := c.Complete("set bgp peer 10.0.0.1", nil)

	texts := completionTexts(completions)
	// Should offer the typed value as a completion (Tab accepts it, adds space)
	assert.Contains(t, texts, "10.0.0.1",
		"typed value should be offered as completion so Tab accepts it")
	// Should NOT contain <value> placeholder
	assert.NotContains(t, texts, "<value>",
		"should not show <value> placeholder when user has typed a value")
}

// TestCompleterListKeyAcceptedThenShowsChildren verifies that after accepting
// a list key (trailing space), completions switch to showing children.
//
// VALIDATES: After Tab accepts key, next completions show peer children.
// PREVENTS: Getting stuck at key position instead of advancing to children.
func TestCompleterListKeyAcceptedThenShowsChildren(t *testing.T) {
	c := NewCompleter()

	// Tree with the peer already added (simulates after key is accepted)
	tree := config.NewTree()
	bgp := config.NewTree()
	tree.SetContainer("bgp", bgp)
	peer := config.NewTree()
	bgp.AddListEntry("peer", "10.0.0.1", peer)
	c.SetTree(tree)

	// After Tab accepted the key, input now has trailing space
	completions := c.Complete("set bgp peer 10.0.0.1 ", nil)

	texts := completionTexts(completions)
	// Should show peer children, not key completions
	assert.Contains(t, texts, "peer-as", "should show peer children after key")
	assert.Contains(t, texts, "hold-time", "should show peer children after key")
	assert.NotContains(t, texts, "<value>", "should not show key hint inside peer")
}

// TestCompleterListKeyEmptyShowsHint verifies that empty list key position
// still shows the <value> hint when no text is typed.
//
// VALIDATES: <value> hint shown when user hasn't typed anything at list key position.
// PREVENTS: Removing helpful hint for empty list key position.
func TestCompleterListKeyEmptyShowsHint(t *testing.T) {
	c := NewCompleter()

	// No existing peers — empty tree
	tree := config.NewTree()
	bgp := config.NewTree()
	tree.SetContainer("bgp", bgp)
	c.SetTree(tree)

	// Trailing space — user is at the key position but hasn't typed anything
	completions := c.Complete("set bgp peer ", nil)

	texts := completionTexts(completions)
	assert.Contains(t, texts, "<value>",
		"empty list key position should show <value> hint")
}

// TestCompleterListKeySingleEntryWithPrefix verifies that typing a list key
// that matches the single existing entry still offers it as a completion.
//
// VALIDATES: "set bgp peer 1.1.1.1<Tab>" works even when 1.1.1.1 is the only peer.
// PREVENTS: Tab doing nothing because single-entry auto-select returns nil.
func TestCompleterListKeySingleEntryWithPrefix(t *testing.T) {
	c := NewCompleter()

	// Build a tree with exactly one peer
	tree := config.NewTree()
	bgp := config.NewTree()
	tree.SetContainer("bgp", bgp)
	peer := config.NewTree()
	peer.Set("peer-as", "65001")
	bgp.AddListEntry("peer", "1.1.1.1", peer)
	c.SetTree(tree)

	// "set bgp peer 1.1.1.1" (no trailing space) should offer the typed value
	completions := c.Complete("set bgp peer 1.1.1.1", nil)
	require.NotEmpty(t, completions, "single entry with matching prefix should still offer completion")

	texts := completionTexts(completions)
	assert.Contains(t, texts, "1.1.1.1", "should offer the typed key as completion")
}

// TestCompleterListKeyInvalidIPRejected verifies that an invalid IP address
// is not offered as a completion for the peer list key.
//
// VALIDATES: "set bgp peer not-an-ip<Tab>" does not offer "not-an-ip" as completion.
// PREVENTS: Tab accepting invalid list key values that fail YANG type validation.
func TestCompleterListKeyInvalidIPRejected(t *testing.T) {
	c := NewCompleter()

	// No existing peers
	tree := config.NewTree()
	bgp := config.NewTree()
	tree.SetContainer("bgp", bgp)
	c.SetTree(tree)

	// Invalid IP: should NOT be offered as completion
	completions := c.Complete("set bgp peer not-an-ip", nil)
	texts := completionTexts(completions)
	assert.NotContains(t, texts, "not-an-ip",
		"invalid IP should not be offered as peer key completion")

	// Partial invalid IP: "999.999.999.999"
	completions = c.Complete("set bgp peer 999.999.999.999", nil)
	texts = completionTexts(completions)
	assert.NotContains(t, texts, "999.999.999.999",
		"out-of-range IP should not be offered as peer key completion")

	// Valid IP: should be offered
	completions = c.Complete("set bgp peer 10.0.0.1", nil)
	texts = completionTexts(completions)
	assert.Contains(t, texts, "10.0.0.1",
		"valid IP should be offered as peer key completion")

	// Valid IPv6: should be offered
	completions = c.Complete("set bgp peer ::1", nil)
	texts = completionTexts(completions)
	assert.Contains(t, texts, "::1",
		"valid IPv6 should be offered as peer key completion")
}

// TestCompleterInvalidKeyWithSpaceNoChildren verifies that after typing
// an invalid list key followed by a space, no children are shown.
//
// VALIDATES: "set bgp peer 1.1.1 " does NOT show peer-as, hold-time, etc.
// PREVENTS: Navigating past an invalid key and showing schema children.
func TestCompleterInvalidKeyWithSpaceNoChildren(t *testing.T) {
	c := NewCompleter()

	tree := config.NewTree()
	bgp := config.NewTree()
	tree.SetContainer("bgp", bgp)
	c.SetTree(tree)

	// Invalid IP with trailing space — should show NO completions
	completions := c.Complete("set bgp peer 1.1.1 ", nil)
	assert.Empty(t, completions,
		"invalid key with trailing space should not show children")

	// Valid IP with trailing space — should show children
	completions = c.Complete("set bgp peer 1.1.1.1 ", nil)
	texts := completionTexts(completions)
	assert.Contains(t, texts, "peer-as",
		"valid key with trailing space should show children")
}

// TestCompleterValidateValueAtPath verifies that ValidateValueAtPath
// checks values against YANG leaf types.
//
// VALIDATES: Invalid values for typed leaves are rejected.
// PREVENTS: Setting non-IP address for peer address, non-numeric for hold-time.
func TestCompleterValidateValueAtPath(t *testing.T) {
	c := NewCompleter()

	tests := []struct {
		name  string
		path  []string
		value string
		valid bool
	}{
		{"valid hold-time", []string{"bgp", "peer", "hold-time"}, "90", true},
		{"zero hold-time", []string{"bgp", "peer", "hold-time"}, "0", true},
		{"invalid hold-time string", []string{"bgp", "peer", "hold-time"}, "abc", false},
		{"hold-time too large", []string{"bgp", "peer", "hold-time"}, "99999999", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.ValidateValueAtPath(tt.path, tt.value)
			if tt.valid {
				assert.NoError(t, err, "value %q should be valid for %v", tt.value, tt.path)
			} else {
				assert.Error(t, err, "value %q should be invalid for %v", tt.value, tt.path)
			}
		})
	}
}

// TestCompleterHintTypeNotApplicable verifies that hint-type completions
// (like <value>, <string>) are typed as "hint" so Tab skips them.
//
// VALIDATES: Placeholder completions use "hint" type, not "value".
// PREVENTS: Tab replacing user input with literal "<value>" text.
func TestCompleterHintTypeNotApplicable(t *testing.T) {
	c := NewCompleter()

	// "set bgp peer " with no tree entries → shows <value> hint
	completions := c.Complete("set bgp peer ", nil)
	require.NotEmpty(t, completions)

	// Find the <value> completion
	for _, comp := range completions {
		if comp.Text == "<value>" {
			assert.Equal(t, "hint", comp.Type,
				"<value> placeholder should have hint type, not value/list-key")
		}
	}

	// Value type hints (e.g., "set router-id " → <value>) should also be hints
	completions = c.Complete("set router-id ", []string{"bgp"})
	require.NotEmpty(t, completions)
	assert.Equal(t, "hint", completions[0].Type,
		"type hint should use hint type")
	assert.True(t, strings.HasPrefix(completions[0].Text, "<"),
		"type hint text should start with <")
}

func completionTexts(completions []Completion) []string {
	texts := make([]string, len(completions))
	for i, c := range completions {
		texts[i] = c.Text
	}
	return texts
}
