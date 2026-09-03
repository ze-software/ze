package command

import (
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

// entriesByName indexes a listing by child name, so a test states the property
// it cares about rather than a position in a slice.
func entriesByName(entries []HelpEntry) map[string]string {
	byName := make(map[string]string, len(entries))
	for _, entry := range entries {
		byName[entry.Name] = entry.Desc
	}
	return byName
}

// VALIDATES: the top-level listing names every verb and carries its summary.
// PREVENTS: a missing verb in the help listing.
// Retargeted onto HelpEntries when the second, unshipped renderer was deleted:
// the listing an operator reads is built from these entries
// (cmd/ze/command_help_page.go, commandHelpPage).
func TestHelpTopLevel(t *testing.T) {
	entries := entriesByName(HelpEntries(testVerbTree(), nil))

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
		got, ok := entries[verb]
		if !ok {
			t.Errorf("the top-level listing is missing the verb %q", verb)
			continue
		}
		if got != desc {
			t.Errorf("the verb %q carries the summary %q, want %q", verb, got, desc)
		}
	}
}

// VALIDATES: a verb's listing names the commands under that verb.
// PREVENTS: the listing not reflecting the YANG tree.
func TestHelpVerbLevel(t *testing.T) {
	entries := entriesByName(HelpEntries(testVerbTree(), []string{"show"}))

	if _, ok := entries["bgp"]; !ok {
		t.Error("the show listing is missing the bgp subcommand")
	}
	if _, ok := entries["version"]; !ok {
		t.Error("the show listing is missing the version subcommand")
	}
}

// VALIDATES: a nested path lists its leaf commands.
// PREVENTS: the listing not descending into a nested path.
func TestHelpNestedLevel(t *testing.T) {
	entries := entriesByName(HelpEntries(testVerbTree(), []string{"show", "bgp"}))

	if _, ok := entries["peer"]; !ok {
		t.Error("the show bgp listing is missing peer")
	}
	if _, ok := entries["decode"]; !ok {
		t.Error("the show bgp listing is missing decode")
	}
}

// VALIDATES: an unknown path lists nothing.
// PREVENTS: a mistyped path producing entries from the wrong node.
func TestHelpUnknownPath(t *testing.T) {
	if entries := HelpEntries(testVerbTree(), []string{"nonexistent"}); entries != nil {
		t.Errorf("an unknown path listed %+v", entries)
	}
}

// VALIDATES: a listing row carries the description declared in YANG.
// PREVENTS: a row rendering a name with no summary beside it.
func TestHelpIncludesDescriptions(t *testing.T) {
	entries := entriesByName(HelpEntries(testVerbTree(), []string{"show"}))

	if entries["bgp"] != "BGP introspection" {
		t.Errorf("the bgp row carries %q, want its declared description", entries["bgp"])
	}
}

// VALIDATES: a listing row carries the child's declared SUMMARY and never its
// long explanation, which belongs to that child's own help page.
// PREVENTS: the listing dumping unindented YANG paragraphs. The mechanism
// changed with this spec: the summary is declared one line long rather than cut
// out of a longer string, so the listing shortens nothing.
func TestHelpEntryUsesSummaryLine(t *testing.T) {
	tree := &Node{
		Children: map[string]*Node{
			"audit": {
				Name:        "audit",
				Description: "Show who did what and when on this box.",
				LongHelp:    "Returns audit log entries with timestamps, actors, and actions.",
			},
		},
	}

	entries := entriesByName(HelpEntries(tree, nil))
	got, ok := entries["audit"]
	if !ok {
		t.Fatalf("the listing is missing the audit entry: %+v", entries)
	}
	if got != "Show who did what and when on this box." {
		t.Fatalf("the audit row carries %q, want its declared summary", got)
	}
	if strings.Contains(got, "Returns audit log entries") {
		t.Fatalf("a listing row carried the long explanation: %q", got)
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

// VALIDATES: a leaf lists no children, so its help page is its own two texts
// and nothing else.
// PREVENTS: a leaf's page growing a Commands section with no rows in it.
// The old assertion also said a leaf's page shows its own description. That
// half moved to the shipped renderer with the page itself, and is now
// TestHelpPageCarriesBothDeclaredHelpTexts (cmd/ze/command_help_page_test.go).
func TestHelpLeafNode(t *testing.T) {
	tree := testVerbTree()

	if node := FindNode(tree, []string{"show", "version"}); node == nil {
		t.Fatal("the leaf path was not found")
	}
	if entries := HelpEntries(tree, []string{"show", "version"}); entries != nil {
		t.Errorf("a leaf listed children: %+v", entries)
	}
}

// VALIDATES: a grouping node with no description of its own falls back to a
// summary of its children.
// PREVENTS: an empty row for an intermediate node.
func TestHelpDescribeChildren(t *testing.T) {
	entries := entriesByName(HelpEntries(testVerbTree(), []string{"set"}))

	// set > system has no description, so the row states its subcommands.
	if entries["system"] != "subcommands: file-descriptors" {
		t.Errorf("the system row carries %q, want a summary of its children", entries["system"])
	}
}

// VALIDATES: describeChildren truncates when >4 children.
// PREVENTS: an overly long row for a large command group.
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

	entries := entriesByName(HelpEntries(tree, nil))
	if !strings.Contains(entries["big"], "... (5 total)") {
		t.Errorf("the big row carries %q, want a truncated summary with the count", entries["big"])
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

// VALIDATES: FindNode and HelpEntries handle a nil root safely.
// PREVENTS: a nil pointer dereference panic.
func TestHelpNilRoot(t *testing.T) {
	if FindNode(nil, []string{"show"}) != nil {
		t.Error("FindNode(nil, ...) should return nil")
	}
	if FindNode(nil, nil) != nil {
		t.Error("FindNode(nil, nil) should return nil")
	}
	if HelpEntries(nil, nil) != nil {
		t.Error("HelpEntries with a nil root should list nothing")
	}
	if HelpEntries(nil, []string{"show"}) != nil {
		t.Error("HelpEntries with a nil root and a path should list nothing")
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

// VALIDATES: a listing row is the node's declared summary, byte for byte.
// PREVENTS: prose that does not belong in a summary hiding behind a cut.
// This test used to pin the opposite property. It said the row was unchanged
// whether or not a description carried an authored `Usage:` sentence, BECAUSE
// the listing stopped at the first sentence. That cut is deleted, so an
// authored sentence now reaches the operator's listing. `./le docvalid
// usage-contract` keeps one out of a description. This test is why that gate
// has to exist, because nothing downstream hides one any more. The usage line
// itself is generated from the command model (usage.go).
func TestHelpListingIsTheDeclaredSummaryByteForByte(t *testing.T) {
	const meaning = "Add a VLAN sub-interface to the dummy."
	const withProse = meaning + "\nUsage: create interface dummy name <name> unit <vid>."

	root := func(description string) *Node {
		return &Node{Children: map[string]*Node{
			"unit": {Name: "unit", Description: description},
		}}
	}

	if got := entriesByName(HelpEntries(root(meaning), nil))["unit"]; got != meaning {
		t.Errorf("the listing row is %q, want the declared summary %q", got, meaning)
	}

	authored := entriesByName(HelpEntries(root(withProse), nil))["unit"]
	if authored != withProse {
		t.Errorf("the listing row is %q, want the declared summary %q", authored, withProse)
	}
	if !strings.Contains(authored, "Usage:") {
		t.Errorf("an authored usage sentence was hidden by the listing: %q", authored)
	}
}

// VALIDATES: a `ze:modifier "choice"` child is not listed as a subcommand, so
// a command whose only child is a choice lists nothing at all.
// PREVENTS: the regression declaring `[import|export]` would otherwise cause.
// `show policy chain peer` has no listed children today and shows its own
// description. Adding the choice container would turn that page into a listing
// of one word no operator types. That the page still carries the command's own
// description is asserted where the page is built:
// TestHelpPageCarriesBothDeclaredHelpTexts (cmd/ze/command_help_page_test.go).
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

	if entries := HelpEntries(root, []string{"chain", "peer"}); len(entries) != 0 {
		t.Errorf("the entries list the choice container: %+v", entries)
	}
}

// VALIDATES: an entry carries the child's declared summary byte for byte.
// PREVENTS: a renderer cutting the summary at a sentence break. The deleted
// helpfmt.Summary dropped every word after the first full stop.
func TestHelpEntriesKeepTheWholeSummary(t *testing.T) {
	// An unconverted node, whose one description still holds both halves. The
	// listing must carry what it is given rather than choose a boundary.
	const declared = "Show one row per session. The row carries state and uptime."
	tree := &Node{
		Children: map[string]*Node{
			"summary": {Name: "summary", Description: declared},
		},
	}

	if got := entriesByName(HelpEntries(tree, nil))["summary"]; got != declared {
		t.Errorf("the entry lost the tail of its summary: %q", got)
	}
}

// VALIDATES: a `ze:modifier "one-of"` child is not listed, and its members are
// listed in its place with their own descriptions.
// PREVENTS: `ze announce flowspec help` naming `action`, a word the handler
// rejects, while hiding `community`, `rate-limit` and `discard`, the three it
// accepts. HelpEntries is what the page builder reads
// (commandHelpPage, cmd/ze/command_help_page.go), so the listing is asserted here
// and the rendered page in cmd/ze/command_help_page_test.go.
func TestHelpListsTheOneOfMembersNotTheWrapper(t *testing.T) {
	node := &Node{
		Name:        "flowspec",
		WireMethod:  "ze-bgp:announce-flowspec",
		Description: "Originate a FlowSpec rule on demand.",
		Children: map[string]*Node{
			"action": {
				Name:        "action",
				Modifier:    ModifierOneOf,
				Description: "The traffic action.",
				Children: map[string]*Node{
					"community":  {Name: "community", Modifier: ModifierOnce, Description: "The action community."},
					"rate-limit": {Name: "rate-limit", Modifier: ModifierOnce, Description: "Rate-limit the matched traffic."},
					"discard":    {Name: "discard", Modifier: ModifierOnce, Description: "Discard the matched traffic."},
				},
			},
			"tag": {Name: "tag", Modifier: ModifierOnce, Description: "A key and a value."},
		},
	}
	root := &Node{Name: "root", Children: map[string]*Node{"announce": {Name: "announce", Children: map[string]*Node{"flowspec": node}}}}

	listed := make(map[string]string, 4)
	for _, entry := range HelpEntries(root, []string{"announce", "flowspec"}) {
		listed[entry.Name] = entry.Desc
	}
	if _, ok := listed["action"]; ok {
		t.Errorf("the entries list the one-of container: %+v", listed)
	}
	for _, member := range []string{"community", "rate-limit", "discard", "tag"} {
		if _, ok := listed[member]; !ok {
			t.Errorf("the entries do not list %q: %+v", member, listed)
		}
	}
	if listed["rate-limit"] != "Rate-limit the matched traffic." {
		t.Errorf("a member lost its own description: %+v", listed)
	}
}
