package mcp

import (
	"encoding/json"
	"math/rand"
	"slices"
	"sort"
	"testing"
)

// determinismRuns is how many times a generation is repeated before its output
// is called stable. Go randomizes map iteration order per range statement, and
// groupCommands ranges over two maps, so a single pass can be lucky. Fifty
// passes over a set with several groups and several actions each makes a
// surviving ordering bug vanishingly unlikely, and the explicit sortedness
// assertion below catches the rest without relying on luck at all.
const determinismRuns = 50

// orderProbeCommands is a command set shaped to exercise every branch of
// groupCommands: a first token with several depth-2 subgroups (show), depth-2
// groups with several actions each, a two-token command, and a first token with
// only depth-1 commands (metrics).
func orderProbeCommands() []CommandInfo {
	return []CommandInfo{
		{Name: "show bgp rib status", Help: "RIB summary"},
		{Name: "show bgp rib best", Help: "Best paths"},
		{Name: "show bgp peer list", Help: "List peers"},
		{Name: "show bgp peer detail", Help: "Peer detail"},
		{Name: "show config dump", Help: "Dump config"},
		{Name: "show config diff", Help: "Diff config"},
		{Name: "show schema tree", Help: "Schema tree"},
		{Name: "show version", Help: "Version"},
		{Name: "metrics list", Help: "List metrics"},
		{Name: "metrics values", Help: "Metric values"},
		{Name: "clear bgp peer counters", Help: "Clear counters"},
		{Name: "clear dns cache", Help: "Clear DNS cache"},
	}
}

// TestToolOrderDeterministic covers AC-6 and AC-13.
//
// VALIDATES: repeated generation from the same command set, fed in a different
// order each time, produces byte-identical tool JSON; groups come out sorted by
// prefix; no two tools share a name; and no `action` enum repeats a value.
// PREVENTS: map iteration in groupCommands reaching the wire. MCP 2026-07-28
// server/tools: "Servers SHOULD return tools in a deterministic order (i.e.,
// the same ordering across requests when the underlying set of tools has not
// changed)", and "Tool names SHOULD be unique within a server". A wobbling
// order silently defeats both client caching and LLM prompt-cache hits, and a
// repeated enum value is invalid JSON Schema.
func TestToolOrderDeterministic(t *testing.T) {
	base := orderProbeCommands()

	shuffler := rand.New(rand.NewSource(1)) //nolint:gosec // deterministic test input shuffling, not cryptography
	var want string

	for run := range determinismRuns {
		// Feed the same set in a different order every run: the lister is a
		// registry walk upstream, so its order is not something MCP may rely on.
		shuffled := slices.Clone(base)
		shuffler.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})

		groups := groupCommands(shuffled)

		// Explicit sortedness, not just repeatability: a stable-but-wrong order
		// would pass a pure equality check across runs.
		if !sort.SliceIsSorted(groups, func(i, j int) bool { return groups[i].prefix < groups[j].prefix }) {
			prefixes := make([]string, len(groups))
			for i, g := range groups {
				prefixes[i] = g.prefix
			}
			t.Fatalf("run %d: groups not sorted by prefix: %v", run, prefixes)
		}
		for _, g := range groups {
			if !sort.SliceIsSorted(g.actions, func(i, j int) bool { return g.actions[i].name < g.actions[j].name }) {
				t.Fatalf("run %d: group %q actions not sorted by name", run, g.prefix)
			}
		}

		encoded, err := json.Marshal(generateTools(groups, handcraftedNames()))
		if err != nil {
			t.Fatalf("run %d: marshal: %v", run, err)
		}
		if run == 0 {
			want = string(encoded)
			continue
		}
		if string(encoded) != want {
			t.Fatalf("run %d produced a different tool list.\n first: %s\n  this: %s", run, want, encoded)
		}
	}
}

// TestGeneratedToolsHaveUniqueNamesAndEnums covers AC-13 independently of
// ordering.
//
// VALIDATES: no two generated tools share a `name`, and no `action` enum
// repeats a value.
// PREVENTS: R-4, two group prefixes differing only by a space-versus-hyphen
// separator collapsing onto one tool name through toolName; and the duplicated
// action A-4 produced, which put the same value in an enum twice and made the
// tool description vary between identical calls. Both are invisible to a test
// that only compares array lengths.
func TestGeneratedToolsHaveUniqueNamesAndEnums(t *testing.T) {
	tools := generateTools(groupCommands(orderProbeCommands()), handcraftedNames())
	if len(tools) == 0 {
		t.Fatal("no tools generated")
	}

	seenNames := make(map[string]bool, len(tools))
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		if seenNames[name] {
			t.Errorf("tool name %q appears more than once", name)
		}
		seenNames[name] = true

		raw, present := tool["inputSchema"].(json.RawMessage)
		if !present {
			t.Fatalf("tool %q has no inputSchema", name)
		}
		var schema struct {
			Properties struct {
				Action struct {
					Enum []string `json:"enum"`
				} `json:"action"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("tool %q: unmarshal schema: %v", name, err)
		}
		seenActions := make(map[string]bool, len(schema.Properties.Action.Enum))
		for _, value := range schema.Properties.Action.Enum {
			if seenActions[value] {
				t.Errorf("tool %q: action enum repeats %q, which is invalid JSON Schema", name, value)
			}
			seenActions[value] = true
		}
	}
}

// VALIDATES: two consecutive tools/list calls over the real transport return
// the same tool names in the same order, including the handcrafted tools that
// lead the list.
// PREVENTS: an ordering wobble that only appears once the descriptors are
// assembled and served, which the generation-level test above cannot see.
func TestToolsListOrderStableAcrossRequests(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{Commands: orderProbeCommands})
	defer cleanup()

	_, first := postMCP(t, hs, methodToolsList, capsNone, "")
	firstNames := toolNames(t, first)
	if len(firstNames) < 2 {
		t.Fatalf("tools/list returned %d tools, too few to judge order", len(firstNames))
	}
	// Handcrafted tools lead the list; that position is part of the contract.
	if firstNames[0] != "ze_execute" || firstNames[1] != "ze_reference" {
		t.Errorf("list starts %v, want the handcrafted tools first", firstNames[:2])
	}

	for range determinismRuns {
		_, next := postMCP(t, hs, methodToolsList, capsNone, "")
		nextNames := toolNames(t, next)
		if !slices.Equal(firstNames, nextNames) {
			t.Fatalf("tool order changed between requests.\nfirst: %v\n next: %v", firstNames, nextNames)
		}
	}
}
