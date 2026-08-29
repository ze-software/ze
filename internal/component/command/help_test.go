package command

import (
	"bytes"
	"strings"
	"testing"
)

// testVerbTree builds a command tree with verb-level structure
// matching the unified CLI design.
func testVerbTree() *Node {
	return &Node{
		Children: map[string]*Node{
			"show": {
				Name:        "show",
				Description: "Read-only introspection commands",
				Children: map[string]*Node{
					"bgp": {
						Name:        "bgp",
						Description: "BGP introspection",
						Children: map[string]*Node{
							"peer":   {Name: "peer", Description: "Show peer(s) details", WireMethod: "ze-show:bgp-peer"},
							"decode": {Name: "decode", Description: "Decode BGP message from hex", WireMethod: "ze-show:bgp-decode"},
						},
					},
					"version": {Name: "version", Description: "Show version and build date", WireMethod: "ze-system:version"},
				},
			},
			"set": {
				Name:        "set",
				Description: "Modify configuration",
				Children: map[string]*Node{
					"system": {
						Name: "system",
						Children: map[string]*Node{
							"file-descriptors": {Name: "file-descriptors", Description: "Raise file descriptor limit", WireMethod: "ze-set:system-file-descriptors"},
						},
					},
				},
			},
			"clear": {
				Name:        "clear",
				Description: "Reset operational state",
				Children: map[string]*Node{
					"bgp": {
						Name: "bgp",
						Children: map[string]*Node{
							"rib": {Name: "rib", Description: "Clear RIB state", WireMethod: "ze-rib-api:clear-in"},
						},
					},
				},
			},
			"delete": {
				Name:        "delete",
				Description: "Remove configuration",
				Children: map[string]*Node{
					"bgp": {
						Name: "bgp",
						Children: map[string]*Node{
							"peer": {Name: "peer", Description: "Remove a peer dynamically", WireMethod: "ze-delete:bgp-peer"},
						},
					},
				},
			},
			"request": {
				Name:        "request",
				Description: "Request an operational action",
				Children: map[string]*Node{
					"bgp": {
						Name: "bgp",
						Children: map[string]*Node{
							"rib": {Name: "rib", Description: "Request RIB action", WireMethod: "ze-rib-api:inject"},
						},
					},
				},
			},
			"update": {
				Name:        "update",
				Description: "Refresh stale data from external sources",
				Children: map[string]*Node{
					"peeringdb": {Name: "peeringdb", Description: "Refresh PeeringDB data", WireMethod: "ze-update:peeringdb"},
				},
			},
			"validate": {
				Name:        "validate",
				Description: "Check without changing",
				Children: map[string]*Node{
					"config": {Name: "config", Description: "Validate configuration file", WireMethod: "ze-repository-check:config"},
				},
			},
			"monitor": {
				Name:        "monitor",
				Description: "Streaming, continuous observation",
				Children: map[string]*Node{
					"bgp": {
						Name: "bgp",
						Children: map[string]*Node{
							"events": {Name: "events", Description: "Stream live BGP events", WireMethod: "ze-bgp:monitor"},
						},
					},
				},
			},
		},
	}
}

// VALIDATES: Top-level help lists all verbs with descriptions.
// PREVENTS: missing verbs in help output.
func TestHelpTopLevel(t *testing.T) {
	tree := testVerbTree()
	var buf bytes.Buffer
	writeHelp(&buf, tree, nil)
	output := buf.String()

	// Check verbs and their descriptions appear together.
	checks := map[string]string{
		"show":     "Read-only introspection commands",
		"set":      "Modify configuration",
		"clear":    "Reset operational state",
		"request":  "Request an operational action",
		"delete":   "Remove configuration",
		"update":   "Refresh stale data from external sources",
		"validate": "Check without changing",
		"monitor":  "Streaming, continuous observation",
	}
	for verb, desc := range checks {
		if !strings.Contains(output, verb) {
			t.Errorf("top-level help missing verb %q", verb)
		}
		if !strings.Contains(output, desc) {
			t.Errorf("top-level help missing description %q for verb %q", desc, verb)
		}
	}
}

// VALIDATES: Verb-level help lists commands under that verb.
// PREVENTS: help not reflecting YANG tree.
func TestHelpVerbLevel(t *testing.T) {
	tree := testVerbTree()
	var buf bytes.Buffer
	writeHelp(&buf, tree, []string{"show"})
	output := buf.String()

	if !strings.Contains(output, "bgp") {
		t.Error("show help missing 'bgp' subcommand")
	}
	if !strings.Contains(output, "version") {
		t.Error("show help missing 'version' subcommand")
	}
}

// VALIDATES: Nested help lists leaf commands.
// PREVENTS: help not descending into nested paths.
func TestHelpNestedLevel(t *testing.T) {
	tree := testVerbTree()
	var buf bytes.Buffer
	writeHelp(&buf, tree, []string{"show", "bgp"})
	output := buf.String()

	if !strings.Contains(output, "peer") {
		t.Error("show bgp help missing 'peer'")
	}
	if !strings.Contains(output, "decode") {
		t.Error("show bgp help missing 'decode'")
	}
}

// VALIDATES: Help for unknown path returns false.
// PREVENTS: panic on invalid help path.
func TestHelpUnknownPath(t *testing.T) {
	tree := testVerbTree()
	var buf bytes.Buffer
	ok := writeHelp(&buf, tree, []string{"nonexistent"})

	if ok {
		t.Error("expected WriteHelp to return false for unknown path")
	}
}

// VALIDATES: Help includes descriptions from YANG tree.
// PREVENTS: descriptions missing in help output.
func TestHelpIncludesDescriptions(t *testing.T) {
	tree := testVerbTree()
	var buf bytes.Buffer
	writeHelp(&buf, tree, []string{"show"})
	output := buf.String()

	if !strings.Contains(output, "BGP introspection") {
		t.Error("show help missing description for bgp")
	}
}

// VALIDATES: List help shows compact one-line summaries for multi-line descriptions.
// PREVENTS: top-level help dumping unindented YANG paragraphs.
func TestHelpEntryUsesSummaryLine(t *testing.T) {
	tree := &Node{
		Children: map[string]*Node{
			"audit": {
				Name:        "audit",
				Description: "Show who did what and when on this box.\nReturns audit log entries with timestamps, actors, and actions.",
			},
		},
	}
	var buf bytes.Buffer
	writeHelp(&buf, tree, nil)
	output := buf.String()

	if !strings.Contains(output, "audit") {
		t.Fatalf("help missing audit entry: %q", output)
	}
	if !strings.Contains(output, "Show who did what and when on this box.") {
		t.Fatalf("help missing summary line: %q", output)
	}
	if strings.Contains(output, "Returns audit log entries") {
		t.Fatalf("help should not dump detailed continuation text: %q", output)
	}
}

// VALIDATES: Leaf help keeps full multi-line descriptions visibly indented.
// PREVENTS: continuation lines starting at column 1 in help output.
func TestHelpLeafMultilineDescriptionIndented(t *testing.T) {
	tree := &Node{
		Children: map[string]*Node{
			"crashes": {
				Name:        "crashes",
				Description: "View saved crash reports from panics.\nUse latest to see the newest crash.",
			},
		},
	}
	var buf bytes.Buffer
	writeHelp(&buf, tree, []string{"crashes"})
	output := buf.String()

	if !strings.Contains(output, "  View saved crash reports from panics.\n") {
		t.Fatalf("help missing first indented line: %q", output)
	}
	if !strings.Contains(output, "  Use latest to see the newest crash.\n") {
		t.Fatalf("help missing indented continuation line: %q", output)
	}
	if strings.Contains(output, "\nUse latest") {
		t.Fatalf("help continuation line was not indented: %q", output)
	}
}

// VALIDATES: Core verbs present in tree.
// PREVENTS: missing verb classification.
func TestUnifiedTreeVerbs(t *testing.T) {
	tree := testVerbTree()
	expectedVerbs := []string{"show", "set", "clear", "request", "delete", "update", "validate", "monitor"}

	for _, verb := range expectedVerbs {
		if _, ok := tree.Children[verb]; !ok {
			t.Errorf("tree missing verb %q", verb)
		}
	}
}

// VALIDATES: IsReadOnly correctly classifies verbs.
// PREVENTS: wrong authorization for commands.
func TestVerbClassification(t *testing.T) {
	readOnlyVerbs := []string{"show", "validate", "monitor"}
	mutatingVerbs := []string{"set", "clear", "request", "delete", "update"}

	for _, verb := range readOnlyVerbs {
		if !IsReadOnlyVerb(verb) {
			t.Errorf("expected %q to be read-only", verb)
		}
	}
	for _, verb := range mutatingVerbs {
		if IsReadOnlyVerb(verb) {
			t.Errorf("expected %q to be mutating, not read-only", verb)
		}
	}
}

// VALIDATES: Leaf node help displays its description.
// PREVENTS: leaf nodes producing empty help output.
func TestHelpLeafNode(t *testing.T) {
	tree := testVerbTree()
	var buf bytes.Buffer
	ok := writeHelp(&buf, tree, []string{"show", "version"})
	output := buf.String()

	if !ok {
		t.Error("expected WriteHelp to return true for leaf node")
	}
	if !strings.Contains(output, "Show version and build date") {
		t.Errorf("leaf help missing description, got: %q", output)
	}
}

// VALIDATES: Grouping nodes without description show subcommand summary.
// PREVENTS: empty help for intermediate nodes.
func TestHelpDescribeChildren(t *testing.T) {
	tree := testVerbTree()
	var buf bytes.Buffer
	writeHelp(&buf, tree, []string{"set"})
	output := buf.String()

	// set > system has no description, should show "subcommands: file-descriptors"
	if !strings.Contains(output, "subcommands: file-descriptors") {
		t.Errorf("set help should describe system children, got: %q", output)
	}
}

// VALIDATES: describeChildren truncates when >4 children.
// PREVENTS: overly long help output for large command groups.
func TestHelpDescribeChildrenTruncation(t *testing.T) {
	tree := &Node{
		Children: map[string]*Node{
			"big": {
				Name: "big",
				Children: map[string]*Node{
					"alpha":   {Name: "alpha", Description: "a"},
					"bravo":   {Name: "bravo", Description: "b"},
					"charlie": {Name: "charlie", Description: "c"},
					"delta":   {Name: "delta", Description: "d"},
					"echo":    {Name: "echo", Description: "e"},
				},
			},
		},
	}
	var buf bytes.Buffer
	writeHelp(&buf, tree, nil)
	output := buf.String()

	if !strings.Contains(output, "... (5 total)") {
		t.Errorf("expected truncated summary with count, got: %q", output)
	}
}

// VALIDATES: FindNode with empty path returns root.
// PREVENTS: panic on empty path.
func TestFindNodeEmptyPath(t *testing.T) {
	tree := testVerbTree()
	node := FindNode(tree, []string{})
	if node != tree {
		t.Error("FindNode with empty path should return root")
	}
	node = FindNode(tree, nil)
	if node != tree {
		t.Error("FindNode with nil path should return root")
	}
}

// VALIDATES: FindNode and WriteHelp handle nil root safely.
// PREVENTS: nil pointer dereference panic.
func TestHelpNilRoot(t *testing.T) {
	if FindNode(nil, []string{"show"}) != nil {
		t.Error("FindNode(nil, ...) should return nil")
	}
	if FindNode(nil, nil) != nil {
		t.Error("FindNode(nil, nil) should return nil")
	}

	var buf bytes.Buffer
	if writeHelp(&buf, nil, nil) {
		t.Error("WriteHelp with nil root should return false")
	}
	if writeHelp(&buf, nil, []string{"show"}) {
		t.Error("WriteHelp with nil root and path should return false")
	}
}

// VALIDATES: FindNode navigates tree by path.
// PREVENTS: broken tree traversal.
func TestFindNode(t *testing.T) {
	tree := testVerbTree()

	tests := []struct {
		path []string
		want string
	}{
		{[]string{"show"}, "show"},
		{[]string{"show", "bgp"}, "bgp"},
		{[]string{"show", "bgp", "peer"}, "peer"},
		{[]string{"delete", "bgp", "peer"}, "peer"},
		{[]string{"nonexistent"}, ""},
		{[]string{"show", "nonexistent"}, ""},
	}

	for _, tt := range tests {
		node := FindNode(tree, tt.path)
		if tt.want == "" {
			if node != nil {
				t.Errorf("FindNode(%v) = %q, want nil", tt.path, node.Name)
			}
		} else {
			if node == nil {
				t.Errorf("FindNode(%v) = nil, want %q", tt.path, tt.want)
			} else if node.Name != tt.want {
				t.Errorf("FindNode(%v) = %q, want %q", tt.path, node.Name, tt.want)
			}
		}
	}
}

// VALIDATES: A-3 -- a listing line is byte-identical with and without the
// authored usage sentence, because the listing stops at the first sentence.
// PREVENTS: deleting the 80 authored sentences reddening every .ci that asserts
// a `ze help` listing line.
func TestHelpListingUnchangedWithoutUsageProse(t *testing.T) {
	const meaning = "Add a VLAN sub-interface to the dummy."
	const withProse = meaning + "\nUsage: create interface dummy name <name> unit <vid>."

	root := func(description string) *Node {
		return &Node{Children: map[string]*Node{
			"unit": {Name: "unit", Description: description},
		}}
	}

	var before, after strings.Builder
	if !writeHelp(&before, root(withProse), nil) {
		t.Fatal("the listing with prose was not written")
	}
	if !writeHelp(&after, root(meaning), nil) {
		t.Fatal("the listing without prose was not written")
	}

	if before.String() != after.String() {
		t.Errorf("the listing changed when the prose went:\n before: %q\n after:  %q",
			before.String(), after.String())
	}
	if !strings.Contains(before.String(), meaning) {
		t.Errorf("the listing does not state the meaning: %q", before.String())
	}
	if strings.Contains(before.String(), "Usage:") {
		t.Errorf("the listing carries the authored sentence: %q", before.String())
	}
}

// VALIDATES: a `ze:modifier "choice"` child is not listed as a subcommand, so
// a command whose only child is a choice still shows its own description.
// PREVENTS: the regression declaring `[import|export]` would otherwise cause.
// `show policy chain peer` has no children today and shows its description;
// adding the choice container would turn that page into a listing of one word
// no operator ever types.
func TestHelpDoesNotListAChoiceGroupAsASubcommand(t *testing.T) {
	node := &Node{
		Name:        "peer",
		WireMethod:  "ze-show:policy-chain",
		Description: "Show the import/export filter chain applied to a peer.",
		Children: map[string]*Node{
			"direction": {
				Name:     "direction",
				Modifier: ModifierChoice,
				ArgDefs:  []ArgDef{{Name: "direction", Kind: ArgEnum, EnumValues: []string{"import", "export"}}},
			},
		},
	}
	root := &Node{Name: "root", Children: map[string]*Node{"chain": {Name: "chain", Children: map[string]*Node{"peer": node}}}}

	var out strings.Builder
	if !writeHelp(&out, root, []string{"chain", "peer"}) {
		t.Fatal("the path was not found")
	}
	if strings.Contains(out.String(), "direction") {
		t.Errorf("the page lists the choice container: %q", out.String())
	}
	if !strings.Contains(out.String(), "filter chain applied to a peer") {
		t.Errorf("the page dropped the command's own description: %q", out.String())
	}
	if entries := HelpEntries(root, []string{"chain", "peer"}); len(entries) != 0 {
		t.Errorf("the entries list the choice container: %+v", entries)
	}
}
