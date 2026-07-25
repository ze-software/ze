//go:build ze_web

package hub

import (
	"testing"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
)

func hasCompletion(comps []cli.Completion, text string) bool {
	for _, s := range comps {
		if s.Text == text {
			return true
		}
	}
	return false
}

// TestPluginAwareCommandCompleterIsLive verifies the web completer overlays
// plugin commands from the CURRENT source on every request: a command that
// appears after the completer is built shows up, and one that later disappears
// (its plugin exited) is gone -- without rebuilding the completer. This is the
// spec's R-1 mitigation (clear and rebuild on re-registration) and closes AC-3
// for the web surface.
//
// VALIDATES: AC-1 (web) + AC-3 (web) -- live register/unregister in completion.
// PREVENTS: a stale snapshot that keeps offering commands of exited plugins.
func TestPluginAwareCommandCompleterIsLive(t *testing.T) {
	tree := &command.Node{Children: map[string]*command.Node{
		"show": {Name: "show", Children: map[string]*command.Node{}},
	}}
	var live []command.CommandEntry
	c := newPluginAwareCommandCompleter(tree, func() []command.CommandEntry { return live })

	// Nothing registered yet: only the YANG surface (empty under show).
	comps := c.Complete("show myplugin ")
	if len(comps) != 0 {
		t.Fatalf("expected no completions before registration, got %v", comps)
	}

	// Register: the command appears immediately, no rebuild.
	live = []command.CommandEntry{{Name: "show myplugin thing", Description: "plugin cmd"}}
	if comps := c.Complete("show myplugin "); !hasCompletion(comps, "thing") {
		t.Fatalf("registered plugin command 'thing' missing: %v", comps)
	}

	// Unregister (plugin exited): the command disappears on the next request.
	live = nil
	if comps := c.Complete("show myplugin "); hasCompletion(comps, "thing") {
		t.Errorf("unregistered plugin command 'thing' still offered: %v", comps)
	}
}

// TestPluginAwareCommandCompleterYANGWins verifies the immutable YANG tree is
// never shadowed by a plugin entry that names the same token, preserving builtin
// precedence at the completion layer.
//
// VALIDATES: AC-5 -- YANG-backed commands keep their description/behavior.
// PREVENTS: a plugin entry overwriting a builtin's help text in completion.
func TestPluginAwareCommandCompleterYANGWins(t *testing.T) {
	tree := &command.Node{Children: map[string]*command.Node{
		"show": {Name: "show", Children: map[string]*command.Node{
			"status": {Name: "status", Description: "YANG status"},
		}},
	}}
	c := newPluginAwareCommandCompleter(tree, func() []command.CommandEntry {
		return []command.CommandEntry{{Name: "show status", Description: "PLUGIN"}}
	})

	comps := c.Complete("show ")
	var count int
	for _, s := range comps {
		if s.Text == "status" {
			count++
			if s.Description != "YANG status" {
				t.Errorf("YANG description overridden by plugin: %q", s.Description)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one 'status' completion (deduped), got %d", count)
	}
}
