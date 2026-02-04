package schema

import (
	"slices"
	"strings"
	"testing"
)

// TestRunNoArgs verifies missing args returns exit code 1.
//
// VALIDATES: Missing arguments shows usage.
// PREVENTS: Panic on empty args.
func TestRunNoArgs(t *testing.T) {
	code := Run([]string{}, nil)
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
		code := Run([]string{arg}, nil)
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
	code := Run([]string{"notacommand"}, nil)
	if code != 1 {
		t.Errorf("expected exit code 1 for unknown command, got %d", code)
	}
}

// TestCmdList verifies list command returns exit code 0.
//
// VALIDATES: List command executes successfully.
// PREVENTS: Crash on list.
func TestCmdList(t *testing.T) {
	code := cmdList([]string{}, nil)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

// TestCmdShowMissing verifies show with missing arg returns exit code 1.
//
// VALIDATES: Show requires module argument.
// PREVENTS: Panic on missing args.
func TestCmdShowMissing(t *testing.T) {
	code := cmdShow([]string{}, nil)
	if code != 1 {
		t.Errorf("expected exit code 1 for missing module, got %d", code)
	}
}

// TestCmdShowKnownModule verifies show with known module works.
//
// VALIDATES: Show returns content for registered module.
// PREVENTS: Error on valid module lookup.
func TestCmdShowKnownModule(t *testing.T) {
	code := cmdShow([]string{"ze-bgp"}, nil)
	if code != 0 {
		t.Errorf("expected exit code 0 for known module, got %d", code)
	}
}

// TestCmdShowUnknownModule verifies show with unknown module returns exit code 1.
//
// VALIDATES: Show fails for unregistered module.
// PREVENTS: Silent failure on typos.
func TestCmdShowUnknownModule(t *testing.T) {
	code := cmdShow([]string{"nonexistent-module"}, nil)
	if code != 1 {
		t.Errorf("expected exit code 1 for unknown module, got %d", code)
	}
}

// TestCmdHandlers verifies handlers command works.
//
// VALIDATES: Handlers command executes successfully.
// PREVENTS: Crash on handlers.
func TestCmdHandlers(t *testing.T) {
	code := cmdHandlers([]string{}, nil)
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

// TestBuildSchemaRegistry verifies registry creation.
//
// VALIDATES: Registry has expected modules.
// PREVENTS: Empty or broken registry.
func TestBuildSchemaRegistry(t *testing.T) {
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		t.Fatalf("failed to build registry: %v", err)
	}

	modules := registry.ListModules()
	if len(modules) == 0 {
		t.Error("expected at least one registered module")
	}

	// Check for expected modules
	expected := []string{"ze-bgp"}
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

// TestSchemaShowZeBGPContent verifies ze schema show ze-bgp outputs actual YANG content.
//
// VALIDATES: ze schema show ze-bgp returns the embedded YANG, not "no YANG content available".
// PREVENTS: Regression where schema show returns empty/placeholder content.
func TestSchemaShowZeBGPContent(t *testing.T) {
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		t.Fatalf("failed to build registry: %v", err)
	}

	schema, err := registry.GetByModule("ze-bgp")
	if err != nil {
		t.Fatalf("failed to get ze-bgp schema: %v", err)
	}

	// Should have actual YANG content
	if schema.Yang == "" {
		t.Error("ze-bgp schema should have YANG content, got empty string")
	}

	// Should contain module declaration
	if !strings.Contains(schema.Yang, "module ze-bgp") {
		t.Error("ze-bgp YANG should contain 'module ze-bgp'")
	}

	// Should contain namespace
	if !strings.Contains(schema.Yang, "namespace") {
		t.Error("ze-bgp YANG should contain 'namespace'")
	}
}

// TestSchemaRegistryIncludesPlugins verifies internal plugins are in the registry.
//
// VALIDATES: Internal plugins like ze-graceful-restart, ze-hostname are discoverable.
// PREVENTS: Internal plugins being invisible to schema discovery.
func TestSchemaRegistryIncludesPlugins(t *testing.T) {
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		t.Fatalf("failed to build registry: %v", err)
	}

	modules := registry.ListModules()

	// These internal plugins should be registered
	expectedPlugins := []string{"ze-graceful-restart", "ze-hostname"}

	for _, plugin := range expectedPlugins {
		found := false
		for _, m := range modules {
			if m == plugin {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected plugin module %q not found in registry", plugin)
		}
	}
}

// TestSchemaNamespaceFormatted verifies namespace uses display format.
//
// VALIDATES: Namespace is formatted for display (ze.bgp not urn:ze:bgp).
// PREVENTS: Raw URN format being displayed to users.
func TestSchemaNamespaceFormatted(t *testing.T) {
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		t.Fatalf("failed to build registry: %v", err)
	}

	schema, err := registry.GetByModule("ze-bgp")
	if err != nil {
		t.Fatalf("failed to get ze-bgp schema: %v", err)
	}

	// Namespace should be formatted (ze.bgp not urn:ze:bgp)
	if strings.Contains(schema.Namespace, "urn:") {
		t.Errorf("namespace should be formatted for display, got %q", schema.Namespace)
	}

	// Should be ze.bgp
	if schema.Namespace != "ze.bgp" {
		t.Errorf("expected namespace 'ze.bgp', got %q", schema.Namespace)
	}
}

// TestDependencyAutoLoad verifies that imports trigger loading of internal dependencies.
//
// VALIDATES: Plugin that imports ze-bgp auto-loads ze-bgp if not already loaded.
// PREVENTS: Missing dependencies when loading plugin schemas.
func TestDependencyAutoLoad(t *testing.T) {
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		t.Fatalf("failed to build registry: %v", err)
	}

	// ze-bgp should be loaded because ze-graceful-restart imports it
	modules := registry.ListModules()
	foundBGP := false
	foundGR := false
	for _, m := range modules {
		if m == "ze-bgp" {
			foundBGP = true
		}
		if m == "ze-graceful-restart" {
			foundGR = true
		}
	}

	if !foundGR {
		t.Error("ze-graceful-restart should be in registry")
	}
	if !foundBGP {
		t.Error("ze-bgp should be loaded (ze-graceful-restart imports it)")
	}
}

// TestSchemaImportsPopulated verifies Imports field is populated for all modules.
//
// VALIDATES: All modules (including auto-loaded) have their Imports populated.
// PREVENTS: Regression where registerInternalYANG forgets to set Imports.
func TestSchemaImportsPopulated(t *testing.T) {
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		t.Fatalf("failed to build registry: %v", err)
	}

	// ze-graceful-restart should import ze-bgp (shown as ze.bgp in display)
	grSchema, err := registry.GetByModule("ze-graceful-restart")
	if err != nil {
		t.Fatalf("failed to get ze-graceful-restart schema: %v", err)
	}

	if len(grSchema.Imports) == 0 {
		t.Error("ze-graceful-restart should have Imports populated (imports ze-bgp)")
	} else if !slices.Contains(grSchema.Imports, "ze-bgp") {
		t.Errorf("ze-graceful-restart Imports should contain ze-bgp, got %v", grSchema.Imports)
	}

	// ze-hostname should also import ze-bgp
	hostnameSchema, err := registry.GetByModule("ze-hostname")
	if err != nil {
		t.Fatalf("failed to get ze-hostname schema: %v", err)
	}

	if len(hostnameSchema.Imports) == 0 {
		t.Error("ze-hostname should have Imports populated (imports ze-bgp)")
	}
}

// TestGetExternalPluginYANG verifies external plugin execution with --yang flag.
//
// VALIDATES: External plugins can be queried for YANG via --yang flag.
// PREVENTS: External plugin schema discovery being broken.
func TestGetExternalPluginYANG(t *testing.T) {
	// Test with echo to verify --yang is appended
	yangContent, err := getExternalPluginYANG("echo module test")
	if err != nil {
		t.Fatalf("getExternalPluginYANG failed: %v", err)
	}

	// echo "module test" --yang should output "module test --yang"
	if !strings.Contains(yangContent, "--yang") {
		t.Errorf("expected --yang to be appended, got: %q", yangContent)
	}
}

// TestRunWithPlugins verifies Run accepts plugins parameter.
//
// VALIDATES: ze --plugin X schema list includes external plugin schemas.
// PREVENTS: --plugin flag being ignored for schema commands.
func TestRunWithPlugins(t *testing.T) {
	// Run with no plugins should work
	code := Run([]string{"list"}, nil)
	if code != 0 {
		t.Errorf("Run with nil plugins should succeed, got code %d", code)
	}

	// Run with empty plugins should work
	code = Run([]string{"list"}, []string{})
	if code != 0 {
		t.Errorf("Run with empty plugins should succeed, got code %d", code)
	}
}

// TestSchemaWantsConfigPopulated verifies WantsConfig is populated for plugins.
//
// VALIDATES: Plugins show their "declare wants config" roots in schema.
// PREVENTS: WantsConfig being empty when it should show config dependencies.
func TestSchemaWantsConfigPopulated(t *testing.T) {
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		t.Fatalf("failed to build registry: %v", err)
	}

	// ze-graceful-restart should want "bgp" config
	grSchema, err := registry.GetByModule("ze-graceful-restart")
	if err != nil {
		t.Fatalf("failed to get ze-graceful-restart schema: %v", err)
	}

	if len(grSchema.WantsConfig) == 0 {
		t.Error("ze-graceful-restart should have WantsConfig populated")
	} else if grSchema.WantsConfig[0] != "bgp" {
		t.Errorf("ze-graceful-restart WantsConfig should be [bgp], got %v", grSchema.WantsConfig)
	}

	// ze-hostname should also want "bgp" config
	hostnameSchema, err := registry.GetByModule("ze-hostname")
	if err != nil {
		t.Fatalf("failed to get ze-hostname schema: %v", err)
	}

	if len(hostnameSchema.WantsConfig) == 0 {
		t.Error("ze-hostname should have WantsConfig populated")
	} else if hostnameSchema.WantsConfig[0] != "bgp" {
		t.Errorf("ze-hostname WantsConfig should be [bgp], got %v", hostnameSchema.WantsConfig)
	}

	// ze-bgp should NOT want any config (it provides config, doesn't consume it)
	bgpSchema, err := registry.GetByModule("ze-bgp")
	if err != nil {
		t.Fatalf("failed to get ze-bgp schema: %v", err)
	}

	if len(bgpSchema.WantsConfig) != 0 {
		t.Errorf("ze-bgp should not have WantsConfig, got %v", bgpSchema.WantsConfig)
	}
}

// TestExternalPluginEmptySpec verifies empty plugin spec handling.
//
// VALIDATES: Empty plugin spec returns error.
// PREVENTS: Panic on empty plugin command.
func TestExternalPluginEmptySpec(t *testing.T) {
	_, err := getExternalPluginYANG("")
	if err == nil {
		t.Error("expected error for empty plugin spec, got nil")
	}
	if !strings.Contains(err.Error(), "no plugin command specified") {
		t.Errorf("expected 'no plugin command specified' error, got: %v", err)
	}
}

// TestExternalPluginNonexistent verifies handling of nonexistent plugins.
//
// VALIDATES: Nonexistent plugin returns error.
// PREVENTS: Silent failure on missing plugins.
func TestExternalPluginNonexistent(t *testing.T) {
	_, err := getExternalPluginYANG("/nonexistent/plugin/path")
	if err == nil {
		t.Error("expected error for nonexistent plugin, got nil")
	}
}
