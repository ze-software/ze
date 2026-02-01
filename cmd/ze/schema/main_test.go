package schema

import (
	"testing"
)

// TestRunNoArgs verifies missing args returns exit code 1.
//
// VALIDATES: Missing arguments shows usage.
// PREVENTS: Panic on empty args.
func TestRunNoArgs(t *testing.T) {
	code := Run([]string{})
	if code != 1 {
		t.Errorf("expected exit code 1 for no args, got %d", code)
	}
}

// TestRunHelp verifies help returns exit code 0.
//
// VALIDATES: Help command works.
// PREVENTS: Help returning error code.
func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"help", "-h", "--help"} {
		code := Run([]string{arg})
		if code != 0 {
			t.Errorf("Run(%q) = %d, want 0", arg, code)
		}
	}
}

// TestRunUnknownCommand verifies unknown command returns exit code 1.
//
// VALIDATES: Unknown commands rejected.
// PREVENTS: Silent failures on typos.
func TestRunUnknownCommand(t *testing.T) {
	code := Run([]string{"notacommand"})
	if code != 1 {
		t.Errorf("expected exit code 1 for unknown command, got %d", code)
	}
}

// TestCmdList verifies list command returns exit code 0.
//
// VALIDATES: List command executes successfully.
// PREVENTS: Crash on list with demo registry.
func TestCmdList(t *testing.T) {
	code := cmdList([]string{})
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

// TestCmdShowMissing verifies show with missing arg returns exit code 1.
//
// VALIDATES: Show requires module argument.
// PREVENTS: Panic on missing args.
func TestCmdShowMissing(t *testing.T) {
	code := cmdShow([]string{})
	if code != 1 {
		t.Errorf("expected exit code 1 for missing module, got %d", code)
	}
}

// TestCmdShowKnownModule verifies show with known module works.
//
// VALIDATES: Show returns content for registered module.
// PREVENTS: Error on valid module lookup.
func TestCmdShowKnownModule(t *testing.T) {
	// ze-bgp is registered in the demo registry
	code := cmdShow([]string{"ze-bgp"})
	if code != 0 {
		t.Errorf("expected exit code 0 for known module, got %d", code)
	}
}

// TestCmdShowUnknownModule verifies show with unknown module returns exit code 1.
//
// VALIDATES: Show fails for unregistered module.
// PREVENTS: Silent failure on typos.
func TestCmdShowUnknownModule(t *testing.T) {
	code := cmdShow([]string{"nonexistent-module"})
	if code != 1 {
		t.Errorf("expected exit code 1 for unknown module, got %d", code)
	}
}

// TestCmdHandlers verifies handlers command works.
//
// VALIDATES: Handlers command executes successfully.
// PREVENTS: Crash on handlers with demo registry.
func TestCmdHandlers(t *testing.T) {
	code := cmdHandlers([]string{})
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

// TestCmdProtocol verifies protocol command works.
//
// VALIDATES: Protocol command executes successfully.
// PREVENTS: Crash on protocol info display.
func TestCmdProtocol(t *testing.T) {
	code := cmdProtocol()
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

// TestGetSchemaRegistry verifies demo registry creation.
//
// VALIDATES: Demo registry has expected modules.
// PREVENTS: Empty or broken demo registry.
func TestGetSchemaRegistry(t *testing.T) {
	registry := getSchemaRegistry()

	modules := registry.ListModules()
	if len(modules) == 0 {
		t.Error("expected at least one registered module")
	}

	// Check for expected demo modules
	expected := []string{"ze-bgp", "ze-plugin", "ze-types"}
	for _, name := range expected {
		found := false
		for _, m := range modules {
			if m == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected module %q not found in registry", name)
		}
	}
}
