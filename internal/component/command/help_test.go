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
					"bgp": {
						Name: "bgp",
						Children: map[string]*Node{
							"peer": {Name: "peer", Description: "Add or modify a peer", WireMethod: "ze-set:bgp-peer"},
						},
					},
				},
			},
			"del": {
				Name:        "del",
				Description: "Remove configuration",
				Children: map[string]*Node{
					"bgp": {
						Name: "bgp",
						Children: map[string]*Node{
							"peer": {Name: "peer", Description: "Remove a peer dynamically", WireMethod: "ze-del:bgp-peer"},
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
					"config": {Name: "config", Description: "Validate configuration file", WireMethod: "ze-validate:config"},
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
	WriteHelp(&buf, tree, nil)
	output := buf.String()

	// Check verbs and their descriptions appear together.
	checks := map[string]string{
		"show":     "Read-only introspection commands",
		"set":      "Modify configuration",
		"del":      "Remove configuration",
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
	WriteHelp(&buf, tree, []string{"show"})
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
	WriteHelp(&buf, tree, []string{"show", "bgp"})
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
	ok := WriteHelp(&buf, tree, []string{"nonexistent"})

	if ok {
		t.Error("expected WriteHelp to return false for unknown path")
	}
}

// VALIDATES: Help includes descriptions from YANG tree.
// PREVENTS: descriptions missing in help output.
func TestHelpIncludesDescriptions(t *testing.T) {
	tree := testVerbTree()
	var buf bytes.Buffer
	WriteHelp(&buf, tree, []string{"show"})
	output := buf.String()

	if !strings.Contains(output, "BGP introspection") {
		t.Error("show help missing description for bgp")
	}
}

// VALIDATES: Core verbs present in tree.
// PREVENTS: missing verb classification.
func TestUnifiedTreeVerbs(t *testing.T) {
	tree := testVerbTree()
	expectedVerbs := []string{"show", "set", "del", "update", "validate", "monitor"}

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
	mutatingVerbs := []string{"set", "del", "update"}

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
	ok := WriteHelp(&buf, tree, []string{"show", "version"})
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
	WriteHelp(&buf, tree, []string{"set"})
	output := buf.String()

	// set > bgp has no description, should show "subcommands: peer"
	if !strings.Contains(output, "subcommands: peer") {
		t.Errorf("set help should describe bgp children, got: %q", output)
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
	WriteHelp(&buf, tree, nil)
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
	if WriteHelp(&buf, nil, nil) {
		t.Error("WriteHelp with nil root should return false")
	}
	if WriteHelp(&buf, nil, []string{"show"}) {
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
		{[]string{"del", "bgp", "peer"}, "peer"},
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
