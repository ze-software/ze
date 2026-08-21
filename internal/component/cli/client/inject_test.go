package client

import (
	"testing"

	"github.com/stretchr/testify/assert"

	cmd "github.com/ze-software/ze/internal/component/command"
)

// TestInjectPluginCommands verifies that plugin commands are added to the
// completion tree when they don't already exist from YANG proxy RPCs.
//
// VALIDATES: AC-1 -- plugin-registered commands appear in completion tree.
// PREVENTS: Plugin commands missing from tab-completion.
func TestInjectPluginCommands(t *testing.T) {
	// Build a tree with one existing YANG-backed command
	tree := cmd.BuildTree([]cmd.RPCInfo{
		{CLICommand: "show bgp health"},
	}, false)

	commands := []commandEntry{
		{Value: "show bgp health", Help: "BGP health"},       // already in tree
		{Value: "show bgp irr", Help: "Show IRR filter data"}, // new plugin command
		{Value: "update bgp irr", Help: "Update IRR data"},    // new plugin command
	}

	injectPluginCommands(tree, commands, nil)

	// Existing command still present
	assert.NotNil(t, tree.Children["show"])
	assert.NotNil(t, tree.Children["show"].Children["bgp"])
	assert.NotNil(t, tree.Children["show"].Children["bgp"].Children["health"])

	// Plugin commands injected
	assert.NotNil(t, tree.Children["show"].Children["bgp"].Children["irr"],
		"plugin command 'show bgp irr' should be injected")
	assert.Equal(t, "Show IRR filter data",
		tree.Children["show"].Children["bgp"].Children["irr"].Description)

	assert.NotNil(t, tree.Children["update"],
		"plugin command 'update bgp irr' should create 'update' node")
	assert.NotNil(t, tree.Children["update"].Children["bgp"].Children["irr"])
	assert.Equal(t, "Update IRR data",
		tree.Children["update"].Children["bgp"].Children["irr"].Description)
}

// TestInjectPluginCommandsSkipsHidden verifies that hidden commands are not
// injected into the completion tree.
//
// VALIDATES: AC-2 -- hidden commands absent from completion tree.
// PREVENTS: Internal/debug commands cluttering tab-completion.
func TestInjectPluginCommandsSkipsHidden(t *testing.T) {
	tree := &cmd.Node{Children: make(map[string]*cmd.Node)}

	commands := []commandEntry{
		{Value: "show status", Help: "Show status"},
		{Value: "show internal", Help: "Internal debug", Hidden: true},
	}
	hidden := map[string]bool{
		"show internal": true,
	}

	injectPluginCommands(tree, commands, hidden)

	assert.NotNil(t, tree.Children["show"].Children["status"],
		"visible command should be injected")
	if tree.Children["show"] != nil {
		_, hasInternal := tree.Children["show"].Children["internal"]
		assert.False(t, hasInternal, "hidden command should not be injected")
	}
}

// TestInjectPluginCommandsPreservesExisting verifies that injection does not
// overwrite existing tree nodes or descriptions from YANG.
//
// VALIDATES: AC-5 -- existing YANG-backed commands unchanged.
// PREVENTS: Plugin injection overwriting YANG descriptions or tree structure.
func TestInjectPluginCommandsPreservesExisting(t *testing.T) {
	tree := cmd.BuildTree([]cmd.RPCInfo{
		{CLICommand: "show bgp health"},
	}, false)
	// Set a YANG-sourced description
	tree.Children["show"].Children["bgp"].Children["health"].Description = "YANG description"

	commands := []commandEntry{
		{Value: "show bgp health", Help: "Plugin description"},
	}

	injectPluginCommands(tree, commands, nil)

	// YANG description preserved (not overwritten)
	assert.Equal(t, "YANG description",
		tree.Children["show"].Children["bgp"].Children["health"].Description)
}

// TestInjectPluginCommandsNilTree verifies that injection handles a nil tree
// gracefully without panicking.
//
// VALIDATES: Defensive nil check.
// PREVENTS: Panic on nil tree (e.g., BuildTree failure).
func TestInjectPluginCommandsNilTree(t *testing.T) {
	assert.NotPanics(t, func() {
		injectPluginCommands(nil, []commandEntry{{Value: "show x"}}, nil)
	})
}

// TestInjectPluginCommandsEmptyCommands verifies that injection with no
// commands is a no-op.
//
// VALIDATES: No-op on empty input.
// PREVENTS: Spurious tree mutations on empty command list.
func TestInjectPluginCommandsEmptyCommands(t *testing.T) {
	tree := &cmd.Node{Children: make(map[string]*cmd.Node)}
	injectPluginCommands(tree, nil, nil)
	assert.Empty(t, tree.Children)
}
