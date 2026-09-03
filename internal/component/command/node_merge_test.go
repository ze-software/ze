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

// VALIDATES: the two help fields of a plugin entry are decided one at a time.
// A field the existing node already holds survives; a field it holds nothing
// in takes the plugin's text.
// PREVENTS: a plugin that states a summary and no explanation blanking a
// builtin's explanation, and a builtin's summary blocking a plugin's
// explanation from ever reaching the help page.
func TestMergeCommandPathsDecidesEachHelpFieldOnItsOwn(t *testing.T) {
	root := &Node{Children: map[string]*Node{
		"daemon": {Name: "daemon", Children: map[string]*Node{
			"status": {Name: "status", Description: "Show daemon status"},
			"health": {Name: "health", LongHelp: "Prints one line for each subsystem."},
		}},
	}}

	MergeCommandPaths(root, []CommandEntry{
		{Name: "daemon status", Description: "PLUGIN OVERRIDE", LongHelp: "The plugin's explanation."},
		{Name: "daemon health", Description: "Show subsystem health.", LongHelp: "PLUGIN OVERRIDE"},
		{Name: "daemon trace", Description: "Trace one request.", LongHelp: "The plugin's explanation."},
	})

	status := root.Children["daemon"].Children["status"]
	if status.Description != "Show daemon status" {
		t.Errorf("summary overwritten: got %q", status.Description)
	}
	if status.LongHelp != "The plugin's explanation." {
		t.Errorf("empty explanation not filled: got %q", status.LongHelp)
	}

	health := root.Children["daemon"].Children["health"]
	if health.Description != "Show subsystem health." {
		t.Errorf("empty summary not filled: got %q", health.Description)
	}
	if health.LongHelp != "Prints one line for each subsystem." {
		t.Errorf("explanation overwritten: got %q", health.LongHelp)
	}

	trace := root.Children["daemon"].Children["trace"]
	if trace == nil {
		t.Fatal("a created leaf is missing")
	}
	if trace.Description != "Trace one request." || trace.LongHelp != "The plugin's explanation." {
		t.Errorf("a created leaf takes both fields: got %q / %q", trace.Description, trace.LongHelp)
	}
}
