package plugin

import (
	"testing"
	"time"
)

// TestCommandRegistry_Register verifies command registration.
//
// VALIDATES: Commands can be registered with name, description, and options.
// PREVENTS: Silent failures on registration, missing metadata.
func TestCommandRegistry_Register(t *testing.T) {
	registry := NewCommandRegistry()
	proc := &Process{config: ProcessConfig{Name: "test-proc"}}

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
	proc := &Process{config: ProcessConfig{Name: "test-proc"}}

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
	proc1 := &Process{config: ProcessConfig{Name: "proc1"}}
	proc2 := &Process{config: ProcessConfig{Name: "proc2"}}

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
	proc1 := &Process{config: ProcessConfig{Name: "proc1"}}
	proc2 := &Process{config: ProcessConfig{Name: "proc2"}}

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
	proc := &Process{config: ProcessConfig{Name: "test-proc"}}

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
	proc := &Process{config: ProcessConfig{Name: "test-proc"}}

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
	proc := &Process{config: ProcessConfig{Name: "test-proc"}}

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
	proc := &Process{config: ProcessConfig{Name: "test-proc"}}

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
