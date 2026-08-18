package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
)

// TestCommandRegistry_Register verifies command registration.
//
// VALIDATES: Commands can be registered with name, description, and options.
// PREVENTS: Silent failures on registration, missing metadata.
func TestCommandRegistry_Register(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	results := registry.Register(proc, []CommandDef{
		{Name: "show status", Description: "Show status"},
		{Name: "request reload", Description: "Reload config", Timeout: 60 * time.Second},
	})

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for _, r := range results {
		if !r.OK {
			t.Errorf("registration failed for %s: %s", r.Name, r.Error)
		}
	}

	// Verify lookup works
	cmd := registry.Lookup("show status")
	if cmd == nil {
		t.Fatal("Lookup returned nil for registered command")
		return
	}
	if cmd.Name != "show status" {
		t.Errorf("expected name 'show status', got %q", cmd.Name)
	}
	if cmd.Description != "Show status" {
		t.Errorf("expected description 'Show status', got %q", cmd.Description)
	}
	if cmd.Timeout != DefaultCommandTimeout {
		t.Errorf("expected default timeout, got %v", cmd.Timeout)
	}

	// Verify custom timeout
	cmd = registry.Lookup("request reload")
	if cmd == nil {
		t.Fatal("Lookup returned nil for request reload")
		return
	}
	if cmd.Timeout != 60*time.Second {
		t.Errorf("expected 60s timeout, got %v", cmd.Timeout)
	}
}

// TestCommandRegistry_BuiltinConflict verifies builtins cannot be shadowed.
//
// VALIDATES: Plugin commands cannot shadow builtin commands.
// PREVENTS: Security issues from shadowing daemon shutdown, etc.
func TestCommandRegistry_BuiltinConflict(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	// Add a builtin
	registry.AddBuiltin("show status")

	results := registry.Register(proc, []CommandDef{
		{Name: "show status", Description: "Fake status"},
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].OK {
		t.Error("expected registration to fail for builtin conflict")
	}
	if results[0].Error == "" {
		t.Error("expected error message for builtin conflict")
	}
}

// TestCommandRegistry_ProcessConflict verifies first-wins for process conflict.
//
// VALIDATES: First process to register a command wins.
// PREVENTS: Later processes from stealing commands.
func TestCommandRegistry_ProcessConflict(t *testing.T) {
	registry := NewCommandRegistry()
	proc1 := process.NewProcess(plugin.PluginConfig{Name: "proc1"})
	proc2 := process.NewProcess(plugin.PluginConfig{Name: "proc2"})

	// First process registers
	results := registry.Register(proc1, []CommandDef{
		{Name: "show status", Description: "Status from proc1"},
	})
	if !results[0].OK {
		t.Fatalf("first registration should succeed: %s", results[0].Error)
	}

	// Second process tries same command
	results = registry.Register(proc2, []CommandDef{
		{Name: "show status", Description: "Status from proc2"},
	})
	if results[0].OK {
		t.Error("second registration should fail")
	}

	// Verify first process still owns it
	cmd := registry.Lookup("show status")
	if cmd.Process != proc1 {
		t.Error("first process should still own the command")
	}
}

// TestCommandRegistry_Unregister verifies command unregistration.
//
// VALIDATES: Commands can be unregistered by owning process.
// PREVENTS: One process unregistering another's commands.
func TestCommandRegistry_Unregister(t *testing.T) {
	registry := NewCommandRegistry()
	proc1 := process.NewProcess(plugin.PluginConfig{Name: "proc1"})
	proc2 := process.NewProcess(plugin.PluginConfig{Name: "proc2"})

	registry.Register(proc1, []CommandDef{
		{Name: "show status", Description: "Status"},
	})
	registry.Register(proc2, []CommandDef{
		{Name: "set timeout", Description: "Set timeout"},
	})

	// proc2 cannot unregister proc1's command (no-op)
	registry.Unregister(proc2, []string{"show status"})
	if registry.Lookup("show status") == nil {
		t.Error("proc2 should not be able to unregister proc1's command")
	}

	// proc1 can unregister its own command
	registry.Unregister(proc1, []string{"show status"})
	if registry.Lookup("show status") != nil {
		t.Error("proc1 should be able to unregister its own command")
	}
}

// TestCommandRegistry_UnregisterAll verifies cleanup on process death.
//
// VALIDATES: All commands from a process are removed on death.
// PREVENTS: Orphaned commands after process exits.
func TestCommandRegistry_UnregisterAll(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	registry.Register(proc, []CommandDef{
		{Name: "show status", Description: "Status"},
		{Name: "request reload", Description: "Reload"},
		{Name: "request check", Description: "Check"},
	})

	if len(registry.All()) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(registry.All()))
	}

	registry.UnregisterAll(proc)

	if len(registry.All()) != 0 {
		t.Errorf("expected 0 commands after UnregisterAll, got %d", len(registry.All()))
	}
}

// TestCommandRegistry_Complete verifies command completion.
//
// VALIDATES: Partial command names return matching completions.
// PREVENTS: CLI completion failing to find plugin commands.
func TestCommandRegistry_Complete(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	registry.Register(proc, []CommandDef{
		{Name: "show status", Description: "Show status"},
		{Name: "show statistics", Description: "Show statistics"},
		{Name: "set timeout", Description: "Set timeout"},
	})

	completions := registry.Complete("show st")

	if len(completions) != 2 {
		t.Fatalf("expected 2 completions, got %d", len(completions))
	}

	// Verify both "show st…" commands match (and "set timeout" does not)
	found := make(map[string]bool)
	for _, c := range completions {
		found[c.Value] = true
	}
	if !found["show status"] {
		t.Error("expected 'show status' in completions")
	}
	if !found["show statistics"] {
		t.Error("expected 'show statistics' in completions")
	}
}

// completeTexts returns the completion texts a TreeCompleter offers for input,
// mapped to their descriptions. Shared by the injection tests below.
func completeTexts(tree *command.Node, input string) map[string]string {
	out := map[string]string{}
	for _, c := range command.NewTreeCompleter(tree).Complete(input) {
		out[c.Text] = c.Description
	}
	return out
}

// TestCommandRegistryInjectsIntoCompletionTree verifies plugin-registered
// commands surface in tab-completion after injection into the command tree.
//
// VALIDATES: AC-1 -- registered plugin commands appear in the completion tree.
// PREVENTS: plugin commands that dispatch but never surface in tab-completion.
func TestCommandRegistryInjectsIntoCompletionTree(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})
	registry.Register(proc, []CommandDef{
		{Name: "show status", Description: "Show status"},
		{Name: "show statistics", Description: "Show statistics"},
	})

	tree := &command.Node{Children: map[string]*command.Node{
		"show": {Name: "show", Children: map[string]*command.Node{}},
	}}
	command.MergeCommandPaths(tree, registry.VisibleCommandEntries())

	got := completeTexts(tree, "show st")
	if got["status"] != "Show status" {
		t.Errorf("status missing/wrong description: %v", got)
	}
	if got["statistics"] != "Show statistics" {
		t.Errorf("statistics missing/wrong description: %v", got)
	}
}

// TestHiddenCommandExcludedFromInjectedTree verifies a Hidden command is absent
// from the injected completion tree but still dispatches when typed in full.
// (The sibling TestHiddenCommandExcludedFromCompletion covers the registry
// Complete() path used by shell completion; this covers the tree path used by
// interactive SSH/web completion via VisibleCommandEntries + MergeCommandPaths.)
//
// VALIDATES: AC-2 -- Hidden suppresses completion, not execution.
// PREVENTS: a Hidden command leaking into tab-completion, or becoming
// un-runnable.
func TestHiddenCommandExcludedFromInjectedTree(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})
	registry.Register(proc, []CommandDef{
		{Name: "show status", Description: "Visible"},
		{Name: "show statistics", Description: "Hidden one", Hidden: true},
	})

	tree := &command.Node{Children: map[string]*command.Node{
		"show": {Name: "show", Children: map[string]*command.Node{}},
	}}
	command.MergeCommandPaths(tree, registry.VisibleCommandEntries())

	got := completeTexts(tree, "show st")
	if _, ok := got["status"]; !ok {
		t.Errorf("visible command missing from completion: %v", got)
	}
	if _, ok := got["statistics"]; ok {
		t.Errorf("Hidden command leaked into completion: %v", got)
	}
	// Still dispatchable when typed in full.
	if registry.Lookup("show statistics") == nil {
		t.Error("Hidden command must still be found by Lookup")
	}
}

// TestUnregisteredCommandRemovedFromCompletion verifies a command that has been
// unregistered (its process exited) is absent from a freshly rebuilt tree.
//
// VALIDATES: AC-3 -- unregistering removes the command from completion.
// PREVENTS: stale completions for commands whose owning plugin is gone.
func TestUnregisteredCommandRemovedFromCompletion(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})
	registry.Register(proc, []CommandDef{
		{Name: "show status", Description: "Show status"},
	})

	// Present before unregister.
	tree := &command.Node{Children: map[string]*command.Node{"show": {Name: "show"}}}
	command.MergeCommandPaths(tree, registry.VisibleCommandEntries())
	if _, ok := completeTexts(tree, "show ")["status"]; !ok {
		t.Fatal("command should be present before unregister")
	}

	registry.UnregisterAll(proc)

	// A tree rebuilt from current registry state (as each SSH session does) no
	// longer offers it.
	fresh := &command.Node{Children: map[string]*command.Node{"show": {Name: "show"}}}
	command.MergeCommandPaths(fresh, registry.VisibleCommandEntries())
	if _, ok := completeTexts(fresh, "show ")["status"]; ok {
		t.Error("unregistered command still present in rebuilt completion tree")
	}
}

// TestCommandRegistry_CaseInsensitive verifies case-insensitive matching.
//
// VALIDATES: Commands are matched case-insensitively.
// PREVENTS: Case sensitivity issues in command lookup.
func TestCommandRegistry_CaseInsensitive(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	registry.Register(proc, []CommandDef{
		{Name: "show status", Description: "Show status"},
	})

	// Lookup should be case-insensitive
	if registry.Lookup("SHOW STATUS") == nil {
		t.Error("Lookup should be case-insensitive")
	}
	if registry.Lookup("Show Status") == nil {
		t.Error("Lookup should be case-insensitive for mixed case")
	}
}

// TestCommandRegistry_Completable verifies completable flag handling.
//
// VALIDATES: Completable flag is stored and accessible.
// PREVENTS: Missing completion support for arg-completing commands.
func TestCommandRegistry_Completable(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	registry.Register(proc, []CommandDef{
		{Name: "show status", Description: "Show status", Args: "<component>", Completable: true},
		{Name: "request reload", Description: "Reload config", Completable: false},
	})

	cmd := registry.Lookup("show status")
	if !cmd.Completable {
		t.Error("show status should be completable")
	}
	if cmd.Args != "<component>" {
		t.Errorf("expected args '<component>', got %q", cmd.Args)
	}

	cmd = registry.Lookup("request reload")
	if cmd.Completable {
		t.Error("request reload should not be completable")
	}
}

// TestCommandRegistryFreeze verifies Freeze creates a snapshot and Lookup uses it.
//
// VALIDATES: AC-10 -- After Freeze(), Lookup uses atomic.Load on frozen snapshot.
// PREVENTS: Post-startup RLock overhead on the dispatch hot path.
func TestCommandRegistryFreeze(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	registry.Register(proc, []CommandDef{
		{Name: "show status", Description: "Show status"},
		{Name: "request reload", Description: "Reload config"},
	})

	registry.Freeze()

	// Lookup must work after freeze
	cmd := registry.Lookup("show status")
	assert.NotNil(t, cmd)
	assert.Equal(t, "show status", cmd.Name)

	cmd = registry.Lookup("request reload")
	assert.NotNil(t, cmd)
	assert.Equal(t, "request reload", cmd.Name)

	// Unknown returns nil
	assert.Nil(t, registry.Lookup("unknown"))
}

// TestCommandRegistryPreFreezeFallback verifies Lookup works before Freeze.
//
// VALIDATES: AC-11 -- Lookup falls back to RLock path before Freeze.
// PREVENTS: Crash if Lookup called during startup before Freeze.
func TestCommandRegistryPreFreezeFallback(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	registry.Register(proc, []CommandDef{
		{Name: "show status", Description: "Show status"},
	})

	// No Freeze() called -- must still work via RLock path
	cmd := registry.Lookup("show status")
	assert.NotNil(t, cmd)
	assert.Equal(t, "show status", cmd.Name)
}

// TestCommandRegistryConcurrentLookup verifies race safety after Freeze.
//
// VALIDATES: AC-12 -- 100 concurrent Lookup calls after Freeze are race-safe.
// PREVENTS: Race conditions on the frozen dispatch hot path.
func TestCommandRegistryConcurrentLookup(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	registry.Register(proc, []CommandDef{
		{Name: "show status", Description: "Show status"},
	})

	registry.Freeze()

	done := make(chan bool, 100)
	for range 100 {
		go func() {
			cmd := registry.Lookup("show status")
			assert.NotNil(t, cmd)
			assert.Equal(t, "show status", cmd.Name)
			done <- true
		}()
	}
	for range 100 {
		<-done
	}
}

// TestCommandRegistryUnregisterAfterFreeze verifies Unregister republishes frozen snapshot.
//
// VALIDATES: Unregister after Freeze publishes new snapshot; Lookup reflects removal.
// PREVENTS: Stale frozen snapshot serving dead-process commands after plugin crash.
func TestCommandRegistryUnregisterAfterFreeze(t *testing.T) {
	registry := NewCommandRegistry()
	proc1 := process.NewProcess(plugin.PluginConfig{Name: "proc1"})
	proc2 := process.NewProcess(plugin.PluginConfig{Name: "proc2"})

	registry.Register(proc1, []CommandDef{{Name: "show a"}})
	registry.Register(proc2, []CommandDef{{Name: "show b"}})

	registry.Freeze()

	// Both visible after freeze
	assert.NotNil(t, registry.Lookup("show a"))
	assert.NotNil(t, registry.Lookup("show b"))

	// Unregister proc1's commands
	registry.Unregister(proc1, []string{"show a"})

	// show a gone, show b still present
	assert.Nil(t, registry.Lookup("show a"))
	assert.NotNil(t, registry.Lookup("show b"))
}

// TestCommandRegistryUnregisterAllAfterFreeze verifies UnregisterAll republishes frozen snapshot.
//
// VALIDATES: UnregisterAll after Freeze publishes new snapshot; Lookup reflects removal.
// PREVENTS: Stale frozen snapshot after process death cleanup.
func TestCommandRegistryUnregisterAllAfterFreeze(t *testing.T) {
	registry := NewCommandRegistry()
	proc1 := process.NewProcess(plugin.PluginConfig{Name: "proc1"})
	proc2 := process.NewProcess(plugin.PluginConfig{Name: "proc2"})

	registry.Register(proc1, []CommandDef{{Name: "show a"}, {Name: "show c"}})
	registry.Register(proc2, []CommandDef{{Name: "show b"}})

	registry.Freeze()

	// All visible
	assert.NotNil(t, registry.Lookup("show a"))
	assert.NotNil(t, registry.Lookup("show b"))
	assert.NotNil(t, registry.Lookup("show c"))

	// Kill proc1 -- UnregisterAll
	registry.UnregisterAll(proc1)

	// proc1 commands gone, proc2 commands remain
	assert.Nil(t, registry.Lookup("show a"))
	assert.Nil(t, registry.Lookup("show c"))
	assert.NotNil(t, registry.Lookup("show b"))
}

// TestDeprecatedCommandWarning verifies deprecated aliases dispatch correctly
// and log a warning once per session.
//
// VALIDATES: Alias plumbing can map a retired spelling to a canonical command.
// PREVENTS: Future released-command deprecations breaking dispatch.
func TestDeprecatedCommandWarning(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "fixture"})

	results := registry.Register(proc, []CommandDef{
		{Name: "show fixture state", Description: "Show fixture state"},
	})
	assert.True(t, results[0].OK)

	assert.NoError(t, registry.registerDeprecated(proc, "show fixture old", "show fixture state"))

	// Lookup by new name works directly
	cmd := registry.Lookup("show fixture state")
	assert.NotNil(t, cmd)
	assert.Equal(t, "show fixture state", cmd.Name)

	cmd = registry.Lookup("show fixture old")
	assert.NotNil(t, cmd)
	assert.Equal(t, "show fixture state", cmd.Name, "deprecated alias should return canonical command")

	// Unknown name still returns nil
	assert.Nil(t, registry.Lookup("unknown command"))
}

// TestDeprecatedCommandAfterFreeze verifies deprecated aliases work after Freeze.
//
// VALIDATES: Deprecated lookup works on the frozen snapshot path (hot path).
// PREVENTS: Deprecated aliases silently breaking after startup barrier.
func TestDeprecatedCommandAfterFreeze(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "fixture"})

	registry.Register(proc, []CommandDef{
		{Name: "show fixture state", Description: "Show fixture state"},
	})
	assert.NoError(t, registry.registerDeprecated(proc, "show fixture old", "show fixture state"))

	registry.Freeze()

	// New name works after freeze
	cmd := registry.Lookup("show fixture state")
	assert.NotNil(t, cmd)
	assert.Equal(t, "show fixture state", cmd.Name)

	cmd = registry.Lookup("show fixture old")
	assert.NotNil(t, cmd)
	assert.Equal(t, "show fixture state", cmd.Name)
}

// TestDeprecatedPrefixLookup verifies prefix matching for deprecated aliases.
//
// VALIDATES: dispatchPlugin prefix matching resolves deprecated aliases.
// PREVENTS: Old command forms failing in the plugin dispatch path.
func TestDeprecatedPrefixLookup(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "fixture"})

	registry.Register(proc, []CommandDef{
		{Name: "show fixture state", Description: "Show fixture state"},
	})
	assert.NoError(t, registry.registerDeprecated(proc, "show fixture old", "show fixture state"))

	// Prefix lookup with trailing args
	cmd, matchLen := registry.lookupDeprecatedPrefix("show fixture old extra-arg")
	assert.NotNil(t, cmd)
	assert.Equal(t, "show fixture state", cmd.Name)
	assert.Equal(t, len("show fixture old"), matchLen)

	// Non-matching prefix returns nil
	cmd, matchLen = registry.lookupDeprecatedPrefix("unknown prefix")
	assert.Nil(t, cmd)
	assert.Equal(t, 0, matchLen)
}

// TestDeprecatedUnregisterAll verifies deprecated aliases are cleaned up on process death.
//
// VALIDATES: UnregisterAll removes deprecated aliases for the dying process.
// PREVENTS: Stale deprecated aliases after plugin crash/restart.
func TestDeprecatedUnregisterAll(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "fixture"})

	registry.Register(proc, []CommandDef{
		{Name: "show fixture state", Description: "Show fixture state"},
	})
	assert.NoError(t, registry.registerDeprecated(proc, "show fixture old", "show fixture state"))

	registry.Freeze()

	// Both work before unregister
	assert.NotNil(t, registry.Lookup("show fixture state"))
	assert.NotNil(t, registry.Lookup("show fixture old"))

	registry.UnregisterAll(proc)

	// Both gone after unregister
	assert.Nil(t, registry.Lookup("show fixture state"))
	assert.Nil(t, registry.Lookup("show fixture old"))
}

// TestValidateCommandNameRejectsMalformed verifies the command-name grammar:
// single-space-separated tokens of lowercase ASCII letters, digits, and
// interior hyphens, with a known first verb (commandVerbs).
//
// VALIDATES: validateCommandName rejects uppercase, Unicode, repeated/edge
// whitespace, empty tokens, unknown verbs, and edge hyphens.
// PREVENTS: invalid command names leaking into every dispatch surface.
func TestValidateCommandNameRejectsMalformed(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		want string // substring the error must contain
	}{
		{"empty", "", "empty"},
		{"uppercase verb", "Show status", "invalid character"},
		{"uppercase token", "show Status", "invalid character"},
		{"unicode confusable", "shоw status", "invalid character"}, // Cyrillic 'о'
		{"leading space", " show status", "leading or trailing whitespace"},
		{"trailing space", "show status ", "leading or trailing whitespace"},
		{"repeated whitespace", "show  status", "repeated whitespace"},
		{"tab separator", "show\tstatus", "invalid character"},
		{"unknown verb", "frob status", "unknown verb"},
		{"verb only unknown", "peer", "unknown verb"},
		{"leading hyphen token", "show -status", "leading or trailing hyphen"},
		{"trailing hyphen token", "show status-", "leading or trailing hyphen"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCommandName(tc.cmd)
			if err == nil {
				t.Fatalf("expected error for %q", tc.cmd)
			}
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestValidateCommandNameAcceptsShippingCommands verifies that real,
// verb-first command names continue to register (no compatibility removed).
//
// VALIDATES: every verb in commandVerbs is accepted with realistic names.
// PREVENTS: the tightened grammar rejecting a name that was always valid.
func TestValidateCommandNameAcceptsShippingCommands(t *testing.T) {
	for _, name := range []string{
		"show bgp rib status",
		"show ecmp-groups",
		"request bgp rib inject",
		"request bgp adj-rib-in replay",
		"clear bgp rib in",
		"set sysctl",
		"commit start",
		"cache retain",
	} {
		assert.NoError(t, validateCommandName(name), "should accept %q", name)
	}
}

// TestValidateCommandNameUsesCanonicalVerbs proves the registration gate derives
// its verb set from the one canonical command.Verbs registry (no second list to
// drift): every canonical verb is accepted, and a mutation token is rejected.
//
// VALIDATES: AC-1 single source of truth; AC-4 strengthened registration check.
// PREVENTS: the plugin gate and the grammar gate disagreeing on the verb set.
func TestValidateCommandNameUsesCanonicalVerbs(t *testing.T) {
	for verb := range command.Verbs {
		if err := validateCommandName(verb + " fixture status"); err != nil {
			t.Errorf("canonical verb %q rejected: %v", verb, err)
		}
	}
	// R7: a mutation token (add/remove) is rejected even under a valid verb.
	if err := validateCommandName("request interface addr add"); err == nil {
		t.Error("expected mutation-token command to be rejected (R7)")
	}
}

// TestValidateCommandNameErrorNamesOffender verifies the error identifies the
// offending command and reason, and derives the valid-verb list from the
// registry rather than a second hardcoded copy.
//
// VALIDATES: error message names the command, the bad verb, and lists verbs.
// PREVENTS: opaque registration errors; a drifting second verb list.
func TestValidateCommandNameErrorNamesOffender(t *testing.T) {
	err := validateCommandName("frob status")
	if err == nil {
		t.Fatal("expected error")
	}
	assert.Contains(t, err.Error(), `"frob status"`, "names the offending command")
	assert.Contains(t, err.Error(), `"frob"`, "names the bad verb")
	assert.Contains(t, err.Error(), validVerbList(), "lists valid verbs from the registry")
}

// TestRegisterDeprecatedRejectsConflicts verifies deprecated aliases are
// validated with the same parser and rejected on every conflict class before
// they are registered.
//
// VALIDATES: alias name validation plus alias-to-builtin, alias-to-command,
// alias-to-alias, and unregistered-canonical conflicts (handover 06 W4/W5).
// PREVENTS: an unreachable or shadowing alias entering the dispatch maps.
func TestRegisterDeprecatedRejectsConflicts(t *testing.T) {
	newProc := func() *process.Process {
		return process.NewProcess(plugin.PluginConfig{Name: "fixture"})
	}

	t.Run("malformed alias rejected before registration", func(t *testing.T) {
		registry := NewCommandRegistry()
		proc := newProc()
		registry.Register(proc, []CommandDef{{Name: "show fixture state"}})
		err := registry.registerDeprecated(proc, "Frob fixture", "show fixture state")
		assert.ErrorContains(t, err, "Frob fixture")
		assert.Nil(t, registry.Lookup("Frob fixture"), "rejected alias must not be registered")
	})

	t.Run("unknown verb alias", func(t *testing.T) {
		registry := NewCommandRegistry()
		proc := newProc()
		registry.Register(proc, []CommandDef{{Name: "show fixture state"}})
		assert.ErrorContains(t, registry.registerDeprecated(proc, "frob fixture", "show fixture state"), "unknown verb")
	})

	t.Run("alias conflicts with builtin", func(t *testing.T) {
		registry := NewCommandRegistry()
		proc := newProc()
		registry.AddBuiltin("show builtin thing")
		registry.Register(proc, []CommandDef{{Name: "show fixture state"}})
		assert.ErrorContains(t, registry.registerDeprecated(proc, "show builtin thing", "show fixture state"), "builtin")
	})

	t.Run("alias conflicts with registered command", func(t *testing.T) {
		registry := NewCommandRegistry()
		proc := newProc()
		registry.Register(proc, []CommandDef{
			{Name: "show fixture state"},
			{Name: "show fixture other"},
		})
		assert.ErrorContains(t, registry.registerDeprecated(proc, "show fixture other", "show fixture state"), "conflicts with command")
	})

	t.Run("alias conflicts with existing alias", func(t *testing.T) {
		registry := NewCommandRegistry()
		proc := newProc()
		registry.Register(proc, []CommandDef{{Name: "show fixture state"}})
		assert.NoError(t, registry.registerDeprecated(proc, "show fixture old", "show fixture state"))
		assert.ErrorContains(t, registry.registerDeprecated(proc, "show fixture old", "show fixture state"), "already registered")
	})

	t.Run("alias to unregistered canonical", func(t *testing.T) {
		registry := NewCommandRegistry()
		proc := newProc()
		assert.ErrorContains(t, registry.registerDeprecated(proc, "show fixture old", "show fixture missing"), "unregistered command")
	})
}

// TestHiddenCommandExcludedFromCompletion verifies that hidden commands are
// excluded from Complete() results but still reachable via Lookup().
//
// VALIDATES: AC-2 -- Hidden commands do not appear in tab-completion but work when typed.
// PREVENTS: Hidden commands leaking into CLI completion suggestions.
func TestHiddenCommandExcludedFromCompletion(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	registry.Register(proc, []CommandDef{
		{Name: "show status", Description: "Show status"},
		{Name: "show internal", Description: "Internal diagnostics", Hidden: true},
		{Name: "show statistics", Description: "Show statistics"},
	})

	completions := registry.Complete("show ")
	found := make(map[string]bool)
	for _, c := range completions {
		found[c.Value] = true
	}

	assert.True(t, found["show status"], "visible command should appear in completions")
	assert.True(t, found["show statistics"], "visible command should appear in completions")
	assert.False(t, found["show internal"], "hidden command should not appear in completions")

	// Hidden command still works when typed in full via Lookup
	cmd := registry.Lookup("show internal")
	assert.NotNil(t, cmd, "hidden command should be reachable via Lookup")
	assert.Equal(t, "show internal", cmd.Name)
	assert.True(t, cmd.Hidden)
}

// TestHiddenCommandPreservedInAll verifies that All() still returns hidden
// commands (needed by dispatch and system command list).
//
// VALIDATES: All() is unchanged -- dispatch path still finds hidden commands.
// PREVENTS: Hidden commands becoming unreachable for dispatch.
func TestHiddenCommandPreservedInAll(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	registry.Register(proc, []CommandDef{
		{Name: "show status", Description: "Show status"},
		{Name: "show internal", Description: "Internal diagnostics", Hidden: true},
	})

	all := registry.All()
	assert.Equal(t, 2, len(all), "All() should return both visible and hidden commands")
}

// TestCommandRegistry_CommandCountsByProcess verifies the number `show system
// subsystem list` reports as command-count.
//
// VALIDATES: the count comes from the registry dispatch resolves against, so it
// counts accepted registrations, drops on unregister, and attributes each command
// to its owning process.
// PREVENTS: the defect this replaced. Process kept a registeredCommands mirror fed
// by the text-protocol register and unregister handlers. The YANG RPC migration
// deleted those handlers and registered here instead, nobody repointed the mirror,
// and both `command-count` sites read an empty slice from 2026-03-27 to
// 2026-08-18. An operator was told every plugin had zero commands. A count that
// cannot go up is indistinguishable from a plugin that registered nothing, which
// is why this asserts a NON-zero number rather than agreement between two readers.
func TestCommandRegistry_CommandCountsByProcess(t *testing.T) {
	registry := NewCommandRegistry()
	proc1 := process.NewProcess(plugin.PluginConfig{Name: "proc1"})
	proc2 := process.NewProcess(plugin.PluginConfig{Name: "proc2"})

	registry.Register(proc1, []CommandDef{
		{Name: "show alpha", Description: "alpha"},
		{Name: "show beta", Description: "beta"},
	})
	registry.Register(proc2, []CommandDef{
		{Name: "show gamma", Description: "gamma"},
	})

	counts := registry.CommandCountsByProcess()
	assert.Equal(t, 2, counts["proc1"], "proc1 registered two commands")
	assert.Equal(t, 1, counts["proc2"], "proc2 registered one")

	// A registration the registry REFUSED is not counted. The mirror this replaced
	// appended only when the result was OK. The count keeps that meaning: what
	// dispatch can resolve, never what a plugin asked for.
	results := registry.Register(proc2, []CommandDef{
		{Name: "show alpha", Description: "collides with proc1"},
	})
	if results[0].OK {
		t.Fatal("the colliding registration was accepted, so it cannot measure a refusal")
	}
	counts = registry.CommandCountsByProcess()
	assert.Equal(t, 1, counts["proc2"], "a refused registration was counted")
	assert.Equal(t, 2, counts["proc1"], "the owner's count changed on somebody else's refusal")

	// Unregister lowers it, which is what removeRegisteredCommand did for the mirror.
	registry.Unregister(proc1, []string{"show beta"})
	counts = registry.CommandCountsByProcess()
	assert.Equal(t, 1, counts["proc1"], "unregister did not lower the count")

	// A process that registered nothing reads zero rather than being absent-and-wrong.
	assert.Equal(t, 0, counts["never-registered"])
}
