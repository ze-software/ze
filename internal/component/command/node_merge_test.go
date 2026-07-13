package command

import "testing"

// VALIDATES: a plugin command path is inserted as a completable leaf under an
// existing grouping node.
// PREVENTS: plugin-registered commands missing from tab-completion (the whole
// point of injecting the registry into the YANG-derived tree).
func TestMergeCommandPathsInsertsNewCommand(t *testing.T) {
	root := testCommandTree()
	root.Children["show"] = &Node{Name: "show", Children: map[string]*Node{
		"bgp": {Name: "bgp"},
	}}

	MergeCommandPaths(root, []CommandEntry{
		{Name: "show bgp irr", Description: "Show IRR data"},
	})

	cc := NewTreeCompleter(root)
	comps := cc.Complete("show bgp ")
	if len(comps) != 1 || comps[0].Text != "irr" {
		t.Fatalf("expected [irr] under 'show bgp ', got %v", comps)
	}
	if comps[0].Description != "Show IRR data" {
		t.Errorf("description = %q, want %q", comps[0].Description, "Show IRR data")
	}
}

// VALIDATES: intermediate nodes are created when the whole path is new.
// PREVENTS: a top-level plugin verb with no YANG ancestor failing to complete.
func TestMergeCommandPathsCreatesIntermediateNodes(t *testing.T) {
	root := &Node{Children: map[string]*Node{}}

	MergeCommandPaths(root, []CommandEntry{
		{Name: "traffic top", Description: "Top talkers"},
	})

	cc := NewTreeCompleter(root)
	if comps := cc.Complete("traffic "); len(comps) != 1 || comps[0].Text != "top" {
		t.Fatalf("expected [top] under 'traffic ', got %v", comps)
	}
}

// VALIDATES: an existing (YANG-backed) node is never mutated by a colliding
// plugin entry — its WireMethod and Description survive.
// PREVENTS: a plugin command silently shadowing a builtin's metadata (AC-5:
// existing YANG commands continue to work exactly as before).
func TestMergeCommandPathsNonDestructive(t *testing.T) {
	root := &Node{Children: map[string]*Node{
		"daemon": {Name: "daemon", Children: map[string]*Node{
			"status": {Name: "status", Description: "Show daemon status", WireMethod: "ze:daemon-status"},
		}},
	}}

	MergeCommandPaths(root, []CommandEntry{
		{Name: "daemon status", Description: "PLUGIN OVERRIDE"},
	})

	got := root.Children["daemon"].Children["status"]
	if got.Description != "Show daemon status" {
		t.Errorf("description overwritten: got %q", got.Description)
	}
	if got.WireMethod != "ze:daemon-status" {
		t.Errorf("wire method lost: got %q", got.WireMethod)
	}
}

// VALIDATES: empty names and a nil root are handled without panic.
// PREVENTS: a crash when the registry is empty or the tree failed to build.
func TestMergeCommandPathsSkipsEmptyAndNilRoot(t *testing.T) {
	MergeCommandPaths(nil, []CommandEntry{{Name: "show bgp"}}) // must not panic

	root := &Node{Children: map[string]*Node{}}
	MergeCommandPaths(root, []CommandEntry{{Name: ""}, {Name: "   "}})
	if len(root.Children) != 0 {
		t.Errorf("empty names produced nodes: %v", root.Children)
	}
}
