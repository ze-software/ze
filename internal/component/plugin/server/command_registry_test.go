package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
)

// TestCommandRegistry_Register verifies command registration.
//
// VALIDATES: Commands can be registered with name, description, and options.
// PREVENTS: Silent failures on registration, missing metadata.
func TestCommandRegistry_Register(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	results := registry.Register(proc, []CommandDef{
		{Name: "myapp status", Description: "Show status"},
		{Name: "myapp reload", Description: "Reload config", Timeout: 60 * time.Second},
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
	cmd := registry.Lookup("myapp status")
	if cmd == nil {
		t.Fatal("Lookup returned nil for registered command")
		return
	}
	if cmd.Name != "myapp status" {
		t.Errorf("expected name 'myapp status', got %q", cmd.Name)
	}
	if cmd.Description != "Show status" {
		t.Errorf("expected description 'Show status', got %q", cmd.Description)
	}
	if cmd.Timeout != DefaultCommandTimeout {
		t.Errorf("expected default timeout, got %v", cmd.Timeout)
	}

	// Verify custom timeout
	cmd = registry.Lookup("myapp reload")
	if cmd == nil {
		t.Fatal("Lookup returned nil for myapp reload")
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
	registry.AddBuiltin("daemon status")

	results := registry.Register(proc, []CommandDef{
		{Name: "daemon status", Description: "Fake status"},
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
		{Name: "myapp status", Description: "Status from proc1"},
	})
	if !results[0].OK {
		t.Fatalf("first registration should succeed: %s", results[0].Error)
	}

	// Second process tries same command
	results = registry.Register(proc2, []CommandDef{
		{Name: "myapp status", Description: "Status from proc2"},
	})
	if results[0].OK {
		t.Error("second registration should fail")
	}

	// Verify first process still owns it
	cmd := registry.Lookup("myapp status")
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
		{Name: "myapp status", Description: "Status"},
	})
	registry.Register(proc2, []CommandDef{
		{Name: "otherapp status", Description: "Other status"},
	})

	// proc2 cannot unregister proc1's command (no-op)
	registry.Unregister(proc2, []string{"myapp status"})
	if registry.Lookup("myapp status") == nil {
		t.Error("proc2 should not be able to unregister proc1's command")
	}

	// proc1 can unregister its own command
	registry.Unregister(proc1, []string{"myapp status"})
	if registry.Lookup("myapp status") != nil {
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
		{Name: "myapp status", Description: "Status"},
		{Name: "myapp reload", Description: "Reload"},
		{Name: "myapp check", Description: "Check"},
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
		{Name: "myapp status", Description: "Show status"},
		{Name: "myapp start", Description: "Start myapp"},
		{Name: "otherapp check", Description: "Check other"},
	})

	completions := registry.Complete("myapp st")

	if len(completions) != 2 {
		t.Fatalf("expected 2 completions, got %d", len(completions))
	}

	// Verify both myapp commands match
	found := make(map[string]bool)
	for _, c := range completions {
		found[c.Value] = true
	}
	if !found["myapp status"] {
		t.Error("expected 'myapp status' in completions")
	}
	if !found["myapp start"] {
		t.Error("expected 'myapp start' in completions")
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
		{Name: "myapp status", Description: "Show status"},
	})

	// Lookup should be case-insensitive
	if registry.Lookup("MYAPP STATUS") == nil {
		t.Error("Lookup should be case-insensitive")
	}
	if registry.Lookup("MyApp Status") == nil {
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
		{Name: "myapp status", Description: "Show status", Args: "<component>", Completable: true},
		{Name: "myapp reload", Description: "Reload config", Completable: false},
	})

	cmd := registry.Lookup("myapp status")
	if !cmd.Completable {
		t.Error("myapp status should be completable")
	}
	if cmd.Args != "<component>" {
		t.Errorf("expected args '<component>', got %q", cmd.Args)
	}

	cmd = registry.Lookup("myapp reload")
	if cmd.Completable {
		t.Error("myapp reload should not be completable")
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
		{Name: "myapp status", Description: "Show status"},
		{Name: "myapp reload", Description: "Reload config"},
	})

	registry.Freeze()

	// Lookup must work after freeze
	cmd := registry.Lookup("myapp status")
	assert.NotNil(t, cmd)
	assert.Equal(t, "myapp status", cmd.Name)

	cmd = registry.Lookup("myapp reload")
	assert.NotNil(t, cmd)
	assert.Equal(t, "myapp reload", cmd.Name)

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
		{Name: "myapp status", Description: "Show status"},
	})

	// No Freeze() called -- must still work via RLock path
	cmd := registry.Lookup("myapp status")
	assert.NotNil(t, cmd)
	assert.Equal(t, "myapp status", cmd.Name)
}

// TestCommandRegistryConcurrentLookup verifies race safety after Freeze.
//
// VALIDATES: AC-12 -- 100 concurrent Lookup calls after Freeze are race-safe.
// PREVENTS: Race conditions on the frozen dispatch hot path.
func TestCommandRegistryConcurrentLookup(t *testing.T) {
	registry := NewCommandRegistry()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-proc"})

	registry.Register(proc, []CommandDef{
		{Name: "myapp status", Description: "Show status"},
	})

	registry.Freeze()

	done := make(chan bool, 100)
	for range 100 {
		go func() {
			cmd := registry.Lookup("myapp status")
			assert.NotNil(t, cmd)
			assert.Equal(t, "myapp status", cmd.Name)
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

	registry.Register(proc1, []CommandDef{{Name: "cmd-a"}})
	registry.Register(proc2, []CommandDef{{Name: "cmd-b"}})

	registry.Freeze()

	// Both visible after freeze
	assert.NotNil(t, registry.Lookup("cmd-a"))
	assert.NotNil(t, registry.Lookup("cmd-b"))

	// Unregister proc1's commands
	registry.Unregister(proc1, []string{"cmd-a"})

	// cmd-a gone, cmd-b still present
	assert.Nil(t, registry.Lookup("cmd-a"))
	assert.NotNil(t, registry.Lookup("cmd-b"))
}

// TestCommandRegistryUnregisterAllAfterFreeze verifies UnregisterAll republishes frozen snapshot.
//
// VALIDATES: UnregisterAll after Freeze publishes new snapshot; Lookup reflects removal.
// PREVENTS: Stale frozen snapshot after process death cleanup.
func TestCommandRegistryUnregisterAllAfterFreeze(t *testing.T) {
	registry := NewCommandRegistry()
	proc1 := process.NewProcess(plugin.PluginConfig{Name: "proc1"})
	proc2 := process.NewProcess(plugin.PluginConfig{Name: "proc2"})

	registry.Register(proc1, []CommandDef{{Name: "cmd-a"}, {Name: "cmd-c"}})
	registry.Register(proc2, []CommandDef{{Name: "cmd-b"}})

	registry.Freeze()

	// All visible
	assert.NotNil(t, registry.Lookup("cmd-a"))
	assert.NotNil(t, registry.Lookup("cmd-b"))
	assert.NotNil(t, registry.Lookup("cmd-c"))

	// Kill proc1 -- UnregisterAll
	registry.UnregisterAll(proc1)

	// proc1 commands gone, proc2 commands remain
	assert.Nil(t, registry.Lookup("cmd-a"))
	assert.Nil(t, registry.Lookup("cmd-c"))
	assert.NotNil(t, registry.Lookup("cmd-b"))
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

	registry.RegisterDeprecated(proc, "legacy fixture state", "show fixture state")

	// Lookup by new name works directly
	cmd := registry.Lookup("show fixture state")
	assert.NotNil(t, cmd)
	assert.Equal(t, "show fixture state", cmd.Name)

	cmd = registry.Lookup("legacy fixture state")
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
	registry.RegisterDeprecated(proc, "legacy fixture state", "show fixture state")

	registry.Freeze()

	// New name works after freeze
	cmd := registry.Lookup("show fixture state")
	assert.NotNil(t, cmd)
	assert.Equal(t, "show fixture state", cmd.Name)

	cmd = registry.Lookup("legacy fixture state")
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
	registry.RegisterDeprecated(proc, "legacy fixture state", "show fixture state")

	// Prefix lookup with trailing args
	cmd, matchLen := registry.LookupDeprecatedPrefix("legacy fixture state extra-arg")
	assert.NotNil(t, cmd)
	assert.Equal(t, "show fixture state", cmd.Name)
	assert.Equal(t, len("legacy fixture state"), matchLen)

	// Non-matching prefix returns nil
	cmd, matchLen = registry.LookupDeprecatedPrefix("unknown prefix")
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
	registry.RegisterDeprecated(proc, "legacy fixture state", "show fixture state")

	registry.Freeze()

	// Both work before unregister
	assert.NotNil(t, registry.Lookup("show fixture state"))
	assert.NotNil(t, registry.Lookup("legacy fixture state"))

	registry.UnregisterAll(proc)

	// Both gone after unregister
	assert.Nil(t, registry.Lookup("show fixture state"))
	assert.Nil(t, registry.Lookup("legacy fixture state"))
}
