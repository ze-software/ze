package hub

import (
	"testing"

	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestBuildCommandMeta_DedupesPluginProxiedCommand covers the union of the two
// command sources, which describe one command twice for every plugin-proxied
// registration.
//
// VALIDATES: a name present in both the dispatcher and the plugin registry
// yields exactly ONE commandMeta, carrying the dispatcher's richer fields.
// PREVENTS: tools/list emitting a duplicated JSON Schema enum value and a tool
// description whose content depends on map iteration order. Dispatcher.
// RegisterWithOptions deliberately skips AddBuiltin when opts.PluginProxy is
// set, so both registrations coexist by design and the join must dedupe.
func TestBuildCommandMeta_DedupesPluginProxiedCommand(t *testing.T) {
	const name = "show isis neighbor"

	dispatcher := []*pluginserver.Command{
		{Name: name, Description: "Show IS-IS neighbors", ReadOnly: true},
	}
	plugin := []*pluginserver.RegisteredCommand{
		{Name: name, LowerName: name, Description: "plugin-side description"},
	}

	got := buildCommandMeta(dispatcher, plugin,
		map[string][]commandParam{name: {{Name: "detail", Type: "boolean"}}},
		map[string]string{name: "required"},
		nil)

	if len(got) != 1 {
		t.Fatalf("merged list length = %d, want 1; entries = %+v", len(got), got)
	}
	if got[0].Name != name {
		t.Errorf("Name = %q, want %q", got[0].Name, name)
	}
	// The dispatcher entry is a strict superset, so its help must survive and
	// the YANG-derived fields must be attached to the surviving entry.
	if got[0].Description != "Show IS-IS neighbors" {
		t.Errorf("Description = %q, want the dispatcher (YANG) help", got[0].Description)
	}
	if !got[0].ReadOnly {
		t.Error("ReadOnly = false, want true from the dispatcher entry")
	}
	if len(got[0].Params) != 1 {
		t.Errorf("Params = %+v, want the YANG-derived param to survive dedupe", got[0].Params)
	}
	if got[0].TaskSupport != "required" {
		t.Errorf("TaskSupport = %q, want %q", got[0].TaskSupport, "required")
	}
}

// VALIDATES: dedupe matches case-insensitively, because both sources key their
// maps on strings.ToLower(name).
// PREVENTS: a case difference between the YANG path and the plugin's
// CommandDecl reintroducing the duplicate the dedupe exists to remove.
func TestBuildCommandMeta_DedupeIsCaseInsensitive(t *testing.T) {
	dispatcher := []*pluginserver.Command{{Name: "show isis neighbor"}}
	plugin := []*pluginserver.RegisteredCommand{{Name: "Show ISIS Neighbor"}}

	if got := buildCommandMeta(dispatcher, plugin, nil, nil, nil); len(got) != 1 {
		t.Fatalf("merged list length = %d, want 1; entries = %+v", len(got), got)
	}
}

// VALIDATES: when the dispatcher entry has no help (the YANG node carries no
// description, so pathToDesc yields ""), the plugin's description fills the gap
// instead of being dropped.
// PREVENTS: dedupe silently losing the only help text a command has.
func TestBuildCommandMeta_PluginHelpFillsEmptyDispatcherHelp(t *testing.T) {
	const name = "show isis hostname"

	got := buildCommandMeta(
		[]*pluginserver.Command{{Name: name, Description: ""}},
		[]*pluginserver.RegisteredCommand{{Name: name, Description: "plugin help"}},
		nil, nil, nil)

	if len(got) != 1 {
		t.Fatalf("merged list length = %d, want 1", len(got))
	}
	if got[0].Description != "plugin help" {
		t.Errorf("Description = %q, want the plugin description to fill the empty dispatcher help", got[0].Description)
	}
}

// VALIDATES: a plugin command with no dispatcher counterpart still appears.
// PREVENTS: the dedupe over-matching and dropping plugin-only commands, which
// would silently shrink the tool surface.
func TestBuildCommandMeta_KeepsPluginOnlyCommand(t *testing.T) {
	got := buildCommandMeta(
		[]*pluginserver.Command{{Name: "show bgp"}},
		[]*pluginserver.RegisteredCommand{{Name: "show widget status", Description: "widget"}},
		nil, nil, nil)

	if len(got) != 2 {
		t.Fatalf("merged list length = %d, want 2; entries = %+v", len(got), got)
	}
	var found bool
	for _, c := range got {
		if c.Name == "show widget status" && c.Description == "widget" {
			found = true
		}
	}
	if !found {
		t.Errorf("plugin-only command missing from %+v", got)
	}
}

// VALIDATES: output order is by name and does not depend on input order.
// PREVENTS: the non-determinism both sources inherit from ranging over Go maps
// reaching consumers that cache and diff this list. Feeding the same set in two
// different orders is what a map's randomized iteration does in practice.
func TestBuildCommandMeta_OrderIsDeterministic(t *testing.T) {
	forward := []*pluginserver.Command{
		{Name: "show bgp"},
		{Name: "clear isis counters"},
		{Name: "show isis neighbor"},
	}
	reversed := []*pluginserver.Command{
		{Name: "show isis neighbor"},
		{Name: "clear isis counters"},
		{Name: "show bgp"},
	}

	a := buildCommandMeta(forward, nil, nil, nil, nil)
	b := buildCommandMeta(reversed, nil, nil, nil, nil)

	if len(a) != len(b) {
		t.Fatalf("lengths differ: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			t.Fatalf("order differs at %d: %q vs %q", i, a[i].Name, b[i].Name)
		}
	}
	want := []string{"clear isis counters", "show bgp", "show isis neighbor"}
	for i, w := range want {
		if a[i].Name != w {
			t.Errorf("position %d = %q, want %q", i, a[i].Name, w)
		}
	}
}

// TestBuildCommandMeta_SkipsHiddenPluginCommand covers the Hidden flag a plugin
// sets on a CommandDecl.
//
// VALIDATES: a plugin command with Hidden true is absent from the merged list,
// and a non-hidden sibling from the same plugin is present.
// PREVENTS: a hidden command reaching the MCP tools/list result and the API
// command list. buildCommandMeta is the single source for both surfaces.
// VisibleCommandEntries hides the command from the completion tree, and every
// MCP client still saw it.
func TestBuildCommandMeta_SkipsHiddenPluginCommand(t *testing.T) {
	got := buildCommandMeta(
		[]*pluginserver.Command{{Name: "show bgp"}},
		[]*pluginserver.RegisteredCommand{
			{Name: "show widget status", Description: "visible one"},
			{Name: "show widget secret", Description: "hidden one", Hidden: true},
		},
		nil, nil, nil)

	names := make(map[string]bool, len(got))
	for _, c := range got {
		names[c.Name] = true
	}

	if names["show widget secret"] {
		t.Errorf("hidden plugin command present in %+v, want it absent", got)
	}
	if !names["show widget status"] {
		t.Errorf("non-hidden plugin command missing from %+v", got)
	}
	if len(got) != 2 {
		t.Errorf("merged list length = %d, want 2; entries = %+v", len(got), got)
	}
}

// VALIDATES: a hidden plugin command whose name also exists in the dispatcher
// contributes nothing, and it does not fill the dispatcher entry's empty help.
// PREVENTS: the duplicate branch reintroducing hidden text. The dispatcher
// entry stays, because a builtin command has no hidden state, but the hidden
// plugin description must not describe it.
func TestBuildCommandMeta_HiddenPluginCommandDoesNotFillHelp(t *testing.T) {
	const name = "show widget status"

	got := buildCommandMeta(
		[]*pluginserver.Command{{Name: name, Description: ""}},
		[]*pluginserver.RegisteredCommand{{Name: name, Description: "hidden help", Hidden: true}},
		nil, nil, nil)

	if len(got) != 1 {
		t.Fatalf("merged list length = %d, want 1; entries = %+v", len(got), got)
	}
	if got[0].Description != "" {
		t.Errorf("Description = %q, want the hidden plugin description to be dropped", got[0].Description)
	}
}

// VALIDATES: the UI-resource lookup still attaches to the surviving entry after
// dedupe, including via the parent-path inheritance lookupUIResource performs.
// PREVENTS: a deduped command losing its MCP Apps annotation.
func TestBuildCommandMeta_UIResourceSurvivesDedupe(t *testing.T) {
	const name = "show bgp peer list"

	got := buildCommandMeta(
		[]*pluginserver.Command{{Name: name}},
		[]*pluginserver.RegisteredCommand{{Name: name}},
		nil, nil,
		map[string]yangloader.UIResourceEntry{
			"show bgp peer": {Path: "bgp-peer/index.html", Permissions: "none", CSP: "default-src 'self'"},
		})

	if len(got) != 1 {
		t.Fatalf("merged list length = %d, want 1", len(got))
	}
	if got[0].UIResource == nil {
		t.Fatal("UIResource = nil, want the parent-path annotation to be inherited")
	}
	if got[0].UIResource.Path != "bgp-peer/index.html" {
		t.Errorf("UIResource.Path = %q, want %q", got[0].UIResource.Path, "bgp-peer/index.html")
	}
}

// TestBuildCommandMetaCarriesBothHelpTexts proves two things. The neutral
// metadata carries the summary and the explanation as two fields. The plugin
// gap-fill decides each of them on its own.
//
// The dispatcher's two fields come from YANG, through LoadBuiltins reading
// PathToDescription and PathToHelp. A plugin sends its own pair on CommandDecl.
// A command with a YANG summary and a plugin explanation keeps both.
//
// VALIDATES: story 7 and story 8 upstream of MCP and OpenAPI, which read this
// type.
// PREVENTS: the explanation being dropped at the merge, and the gap-fill
// overwriting one half because the other was empty.
func TestBuildCommandMetaCarriesBothHelpTexts(t *testing.T) {
	const yangOnly = "show isis neighbor"
	const halfEach = "show widget status"
	const pluginOnly = "widget clear"

	got := buildCommandMeta(
		[]*pluginserver.Command{
			{Name: yangOnly, Description: "List the IS-IS neighbors.", LongHelp: "One row for each adjacency.\nThe hold time is what the neighbor advertised."},
			{Name: halfEach, Description: "Show the widget state."},
		},
		[]*pluginserver.RegisteredCommand{
			{Name: halfEach, Description: "plugin summary", LongHelp: "The widget count is since the last clear."},
			{Name: pluginOnly, Description: "Clear the widget counters.", LongHelp: "The counters restart at zero."},
		},
		nil, nil, nil)

	byName := make(map[string]commandMeta, len(got))
	for _, c := range got {
		byName[c.Name] = c
	}

	yang := byName[yangOnly]
	if yang.Description != "List the IS-IS neighbors." {
		t.Errorf("Description = %q, want the YANG summary", yang.Description)
	}
	if yang.LongHelp != "One row for each adjacency.\nThe hold time is what the neighbor advertised." {
		t.Errorf("LongHelp = %q, want the YANG explanation with its newline", yang.LongHelp)
	}

	// The dispatcher declared a summary and no explanation, so the plugin fills
	// the explanation and does NOT replace the summary.
	half := byName[halfEach]
	if half.Description != "Show the widget state." {
		t.Errorf("Description = %q, want the dispatcher summary to win", half.Description)
	}
	if half.LongHelp != "The widget count is since the last clear." {
		t.Errorf("LongHelp = %q, want the plugin explanation to fill the empty half", half.LongHelp)
	}

	only := byName[pluginOnly]
	if only.Description != "Clear the widget counters." || only.LongHelp != "The counters restart at zero." {
		t.Errorf("plugin-only command = %+v, want both halves carried", only)
	}
}
