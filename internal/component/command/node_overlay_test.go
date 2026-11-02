package command

import "testing"

// TestOverlayCommandPathsLeavesBaseUntouched checks the per-request overlay the
// web admin console reads: the answer holds the base tree and every entry, the
// base keeps exactly its own nodes and help, and a builtin's help is never
// overwritten by a plugin entry on the same path.
//
// VALIDATES: OverlayCommandPaths merges like MergeCommandPaths without writing
// to the shared tree.
// PREVENTS: a plugin that exited staying on the admin console because an
// earlier request merged it into the long-lived tree, and two requests racing
// on one map.
func TestOverlayCommandPathsLeavesBaseUntouched(t *testing.T) {
	base := &Node{Children: map[string]*Node{
		"show": {Name: "show", Children: map[string]*Node{
			"version": {Name: "version", ShortHelp: "builtin help", WireMethod: "ze-system:version"},
		}},
	}}

	got := OverlayCommandPaths(base, []CommandEntry{
		{Name: "show zz plugin", ShortHelp: "plugin help", Description: "plugin explanation"},
		{Name: "show version", ShortHelp: "plugin must not win", Description: "plugin explanation"},
	})

	leaf := got.Children["show"].Children["zz"].Children["plugin"]
	if leaf == nil {
		t.Fatal("the overlay lacks the plugin entry show zz plugin")
	}
	if leaf.ShortHelp != "plugin help" || leaf.Description != "plugin explanation" {
		t.Errorf("plugin leaf help = %q / %q, want the entry's two texts", leaf.ShortHelp, leaf.Description)
	}
	version := got.Children["show"].Children["version"]
	if version.ShortHelp != "builtin help" || version.WireMethod != "ze-system:version" {
		t.Errorf("show version = %q %q, want the builtin's help and wire method kept", version.ShortHelp, version.WireMethod)
	}
	if version.Description != "plugin explanation" {
		t.Errorf("show version description = %q, want the plugin's explanation filling the empty field", version.Description)
	}

	if _, leaked := base.Children["show"].Children["zz"]; leaked {
		t.Error("the overlay wrote show zz into the base tree")
	}
	if d := base.Children["show"].Children["version"].Description; d != "" {
		t.Errorf("the overlay wrote %q into the base tree's show version", d)
	}
	if len(base.Children["show"].Children) != 1 {
		t.Errorf("base show has %d children, want its 1", len(base.Children["show"].Children))
	}

	if alone := OverlayCommandPaths(nil, []CommandEntry{{Name: "a b"}}); alone.Children["a"].Children["b"] == nil {
		t.Error("a nil base did not answer a tree holding the entry")
	}
}
