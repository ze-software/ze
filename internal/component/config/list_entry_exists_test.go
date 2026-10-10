package config

import (
	"strings"
	"testing"
)

// TestCopyRenameRefuseExistingDestination proves copy and rename never
// overwrite an existing list entry and tell the operator to delete it first.
//
// VALIDATES: spec-session-editor-file-mode-parity AC-30.
// PREVENTS: a refusal that names the clash but not the way out.
func TestCopyRenameRefuseExistingDestination(t *testing.T) {
	build := func() *Tree {
		tree := NewTree()
		tree.AddListEntry("peer", "a", NewTree())
		tree.AddListEntry("peer", "b", NewTree())
		return tree
	}
	for name, run := range map[string]func(*Tree) error{
		"rename": func(tree *Tree) error { return tree.RenameListEntry("peer", "a", "b") },
		"copy":   func(tree *Tree) error { return tree.CopyListEntry("peer", "a", "b") },
	} {
		tree := build()
		err := run(tree)
		if err == nil {
			t.Fatalf("%s onto an existing entry succeeded", name)
		}
		for _, want := range []string{"b already exists in peer", "delete it first"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: error %q does not say %q", name, err, want)
			}
		}
		if got := len(tree.GetList("peer")); got != 2 {
			t.Errorf("%s: peer holds %d entries after the refusal, want 2", name, got)
		}
	}
}
