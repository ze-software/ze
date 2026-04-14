package schema

import (
	"encoding/json"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	bgpschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/schema"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// captureStdout runs fn and returns whatever it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close() //nolint:errcheck,gosec // test cleanup
	os.Stdout = old
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(out)
}

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
	code := cmdShow([]string{"ze-bgp-conf"}, nil)
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
	expected := []string{"ze-bgp-conf"}
	for _, name := range expected {
		found := slices.Contains(modules, name)
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

	schema, err := registry.GetByModule("ze-bgp-conf")
	if err != nil {
		t.Fatalf("failed to get ze-bgp-conf schema: %v", err)
	}

	// Should have actual YANG content
	if schema.Yang == "" {
		t.Error("ze-bgp-conf schema should have YANG content, got empty string")
	}

	// Should contain module declaration
	if !strings.Contains(schema.Yang, "module ze-bgp-conf") {
		t.Error("ze-bgp-conf YANG should contain 'module ze-bgp-conf'")
	}

	// Should contain namespace
	if !strings.Contains(schema.Yang, "namespace") {
		t.Error("ze-bgp-conf YANG should contain 'namespace'")
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
		found := slices.Contains(modules, plugin)
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

	schema, err := registry.GetByModule("ze-bgp-conf")
	if err != nil {
		t.Fatalf("failed to get ze-bgp-conf schema: %v", err)
	}

	// Namespace should be formatted (ze.bgp.conf not urn:ze:bgp:conf)
	if strings.Contains(schema.Namespace, "urn:") {
		t.Errorf("namespace should be formatted for display, got %q", schema.Namespace)
	}

	// Should be ze.bgp.conf
	if schema.Namespace != "ze.bgp.conf" {
		t.Errorf("expected namespace 'ze.bgp.conf', got %q", schema.Namespace)
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

	// ze-bgp-conf should be loaded because ze-graceful-restart imports it
	modules := registry.ListModules()
	foundBGP := false
	foundGR := false
	for _, m := range modules {
		if m == "ze-bgp-conf" {
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
		t.Error("ze-bgp-conf should be loaded (ze-graceful-restart imports it)")
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
		t.Error("ze-graceful-restart should have Imports populated (imports ze-bgp-conf)")
	} else if !slices.Contains(grSchema.Imports, "ze-bgp-conf") {
		t.Errorf("ze-graceful-restart Imports should contain ze-bgp-conf, got %v", grSchema.Imports)
	}

	// ze-hostname should also import ze-bgp-conf
	hostnameSchema, err := registry.GetByModule("ze-hostname")
	if err != nil {
		t.Fatalf("failed to get ze-hostname schema: %v", err)
	}

	if len(hostnameSchema.Imports) == 0 {
		t.Error("ze-hostname should have Imports populated (imports ze-bgp-conf)")
	}
}

// TestRegisterYANGDeduplication verifies duplicate module registration is skipped.
//
// VALIDATES: Registering the same module twice returns nil (no error) and module appears once.
// PREVENTS: Duplicate registration causing errors or duplicate entries.
func TestRegisterYANGDeduplication(t *testing.T) {
	registry := pluginserver.NewSchemaRegistry()
	loaded := make(map[string]bool)

	// Register ze-bgp first time
	err := registerYANG(registry, bgpschema.ZeBGPConfYANG, "ze.bgp", []string{"bgp"}, nil, loaded)
	if err != nil {
		t.Fatalf("first registerYANG failed: %v", err)
	}

	// Verify it's in the loaded map
	if !loaded["ze-bgp-conf"] {
		t.Error("ze-bgp-conf should be marked as loaded after first registration")
	}

	// Register ze-bgp-conf second time - should return nil (skip), not error
	err = registerYANG(registry, bgpschema.ZeBGPConfYANG, "ze.bgp", []string{"bgp"}, nil, loaded)
	if err != nil {
		t.Errorf("second registerYANG should return nil (skip), got error: %v", err)
	}

	// Verify module only appears once in registry
	modules := registry.ListModules()
	count := 0
	for _, m := range modules {
		if m == "ze-bgp-conf" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("ze-bgp-conf should appear exactly once, found %d times", count)
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

	// ze-bgp-conf should NOT want any config (it provides config, doesn't consume it)
	bgpSchema, err := registry.GetByModule("ze-bgp-conf")
	if err != nil {
		t.Fatalf("failed to get ze-bgp-conf schema: %v", err)
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

// TestCmdMethods verifies ze schema methods lists RPCs from all YANG API modules.
//
// VALIDATES: Methods command loads API YANG modules and lists RPCs with correct wire methods.
// PREVENTS: Empty output or missing modules when YANG modules define RPCs.
func TestCmdMethods(t *testing.T) {
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		t.Fatalf("failed to build registry: %v", err)
	}

	rpcs := registry.ListRPCs("")

	// Verify RPCs from all 4 modules are present
	modules := make(map[string]int)
	for _, rpc := range rpcs {
		modules[rpc.Module]++
	}

	if modules["ze-bgp-api"] != 26 {
		t.Errorf("expected 26 BGP RPCs, got %d", modules["ze-bgp-api"])
	}
	if modules["ze-system-api"] != 13 {
		t.Errorf("expected 13 system RPCs, got %d", modules["ze-system-api"])
	}
	if modules["ze-plugin-api"] != 8 {
		t.Errorf("expected 8 plugin RPCs, got %d", modules["ze-plugin-api"])
	}
	if modules["ze-rib-api"] != 13 {
		t.Errorf("expected 13 RIB RPCs, got %d", modules["ze-rib-api"])
	}
}

// TestCmdMethodsWithModule verifies ze schema methods filters by module.
//
// VALIDATES: Module filter restricts output to one module's RPCs only.
// PREVENTS: Module filter being ignored or including other modules.
func TestCmdMethodsWithModule(t *testing.T) {
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		t.Fatalf("failed to build registry: %v", err)
	}

	rpcs := registry.ListRPCs("ze-system-api")
	for _, rpc := range rpcs {
		if rpc.Module != "ze-system-api" {
			t.Errorf("filtered by ze-system-api but got RPC from %s: %s", rpc.Module, rpc.WireMethod)
		}
	}
	if len(rpcs) == 0 {
		t.Error("expected RPCs for ze-system-api, got none")
	}
}

// TestCmdMethodsUnknownModule verifies ze schema methods returns error for unknown module.
//
// VALIDATES: Unknown module returns exit code 1.
// PREVENTS: Typos silently returning empty results.
func TestCmdMethodsUnknownModule(t *testing.T) {
	code := cmdMethods([]string{"ze-bogus-api"}, nil)
	if code != 1 {
		t.Errorf("expected exit code 1 for unknown module, got %d", code)
	}
}

// TestCmdEvents verifies ze schema events lists notifications from YANG API modules.
//
// VALIDATES: Events command loads API YANG modules and lists notifications from multiple modules.
// PREVENTS: Empty output when YANG modules define notifications.
func TestCmdEvents(t *testing.T) {
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		t.Fatalf("failed to build registry: %v", err)
	}

	notifs := registry.ListNotifications("")
	if len(notifs) != 8 {
		t.Errorf("expected 8 notifications, got %d", len(notifs))
	}

	// Verify we have notifications from both BGP and RIB modules
	modules := make(map[string]bool)
	for _, notif := range notifs {
		modules[notif.Module] = true
	}
	if !modules["ze-bgp-api"] {
		t.Error("expected notifications from ze-bgp-api")
	}
	if !modules["ze-rib-api"] {
		t.Error("expected notifications from ze-rib-api")
	}
}

// TestCmdEventsWithModule verifies ze schema events filters by module.
//
// VALIDATES: Module filter restricts output to one module's notifications only.
// PREVENTS: Module filter being ignored.
func TestCmdEventsWithModule(t *testing.T) {
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		t.Fatalf("failed to build registry: %v", err)
	}

	notifs := registry.ListNotifications("ze-bgp-api")
	for _, notif := range notifs {
		if notif.Module != "ze-bgp-api" {
			t.Errorf("filtered by ze-bgp-api but got notification from %s", notif.Module)
		}
	}
	if len(notifs) != 7 {
		t.Errorf("expected 7 BGP notifications, got %d", len(notifs))
	}
}

// TestCmdEventsUnknownModule verifies ze schema events returns error for unknown module.
//
// VALIDATES: Unknown module returns exit code 1.
// PREVENTS: Typos silently returning empty results.
func TestCmdEventsUnknownModule(t *testing.T) {
	code := cmdEvents([]string{"ze-bogus-api"}, nil)
	if code != 1 {
		t.Errorf("expected exit code 1 for unknown module, got %d", code)
	}
}

// TestBuildSchemaRegistryRPCs verifies that buildSchemaRegistry populates RPCs.
//
// VALIDATES: Registry contains RPCs from all 4 API modules with correct wire methods.
// PREVENTS: Methods command returning empty or missing modules.
func TestBuildSchemaRegistryRPCs(t *testing.T) {
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		t.Fatalf("failed to build registry: %v", err)
	}

	rpcs := registry.ListRPCs("")

	// Build wire method set for quick lookup
	wireSet := make(map[string]bool, len(rpcs))
	for _, rpc := range rpcs {
		wireSet[rpc.WireMethod] = true
	}

	// Verify key RPCs from each module
	expected := []string{
		"ze-bgp:peer-list", "ze-bgp:help",
		"ze-bgp:subscribe", "ze-bgp:unsubscribe", "ze-bgp:commit",
		"ze-system:help", "ze-system:version-software", "ze-system:daemon-status",
		"ze-plugin:help", "ze-plugin:session-ping", "ze-plugin:session-bye",
		"ze-rib:help", "ze-rib:show", "ze-rib:event-list",
	}
	for _, method := range expected {
		if !wireSet[method] {
			t.Errorf("expected %s in registry", method)
		}
	}
}

// TestBuildSchemaRegistryNotifications verifies that buildSchemaRegistry populates notifications.
//
// VALIDATES: Registry contains notifications from API YANG modules.
// PREVENTS: Events command returning empty when YANG defines notifications.
func TestBuildSchemaRegistryNotifications(t *testing.T) {
	registry, err := buildSchemaRegistry(nil)
	if err != nil {
		t.Fatalf("failed to build registry: %v", err)
	}

	notifs := registry.ListNotifications("")

	wireSet := make(map[string]bool, len(notifs))
	for _, notif := range notifs {
		wireSet[notif.WireMethod] = true
	}

	expected := []string{
		"ze-bgp:peer-state-change",
		"ze-bgp:session-established",
		"ze-rib:rib-change",
	}
	for _, method := range expected {
		if !wireSet[method] {
			t.Errorf("expected %s notification in registry", method)
		}
	}
}

// TestCmdListJSON verifies list --json outputs valid JSON with expected fields.
//
// VALIDATES: --json flag produces valid JSON array with module entries.
// PREVENTS: --json flag being ignored, producing invalid JSON, or empty output.
func TestCmdListJSON(t *testing.T) {
	var code int
	output := captureStdout(t, func() {
		code = cmdList([]string{"--json"}, nil)
	})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	var entries []map[string]any
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, output)
	}
	if len(entries) == 0 {
		t.Fatal("expected non-empty JSON array")
	}
	// Every entry must have "module" and "namespace" keys.
	foundBGP := false
	for _, e := range entries {
		if _, ok := e["module"]; !ok {
			t.Errorf("entry missing 'module' key: %v", e)
		}
		if _, ok := e["namespace"]; !ok {
			t.Errorf("entry missing 'namespace' key: %v", e)
		}
		if e["module"] == "ze-bgp-conf" {
			foundBGP = true
		}
	}
	if !foundBGP {
		t.Error("expected ze-bgp-conf in JSON list output")
	}
}

// TestCmdShowJSON verifies show --json outputs valid JSON with module metadata.
//
// VALIDATES: --json flag produces valid JSON object with module, namespace, yang fields.
// PREVENTS: --json flag producing invalid JSON or missing YANG content.
func TestCmdShowJSON(t *testing.T) {
	var code int
	output := captureStdout(t, func() {
		code = cmdShow([]string{"--json", "ze-bgp-conf"}, nil)
	})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(output), &obj); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, output)
	}
	if obj["module"] != "ze-bgp-conf" {
		t.Errorf("module = %v, want 'ze-bgp-conf'", obj["module"])
	}
	if _, ok := obj["yang"]; !ok {
		t.Error("expected 'yang' key in show JSON output")
	}
}

// TestCmdHandlersJSON verifies handlers --json outputs valid JSON map.
//
// VALIDATES: --json flag produces valid JSON object mapping handlers to modules.
// PREVENTS: --json flag being ignored or producing invalid JSON.
func TestCmdHandlersJSON(t *testing.T) {
	var code int
	output := captureStdout(t, func() {
		code = cmdHandlers([]string{"--json"}, nil)
	})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(output), &obj); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, output)
	}
	if len(obj) == 0 {
		t.Error("expected non-empty JSON object for handlers")
	}
	if _, ok := obj["bgp"]; !ok {
		t.Error("expected 'bgp' handler in JSON output")
	}
}

// TestCmdMethodsJSON verifies methods --json outputs valid JSON array.
//
// VALIDATES: --json flag produces valid JSON array with method, module, description fields.
// PREVENTS: --json flag producing invalid JSON or missing fields.
func TestCmdMethodsJSON(t *testing.T) {
	var code int
	output := captureStdout(t, func() {
		code = cmdMethods([]string{"--json"}, nil)
	})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	var entries []map[string]string
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, output)
	}
	if len(entries) == 0 {
		t.Fatal("expected non-empty JSON array for methods")
	}
	for _, e := range entries {
		if e["method"] == "" {
			t.Errorf("entry missing 'method': %v", e)
		}
		if e["module"] == "" {
			t.Errorf("entry missing 'module': %v", e)
		}
	}
}

// TestCmdEventsJSON verifies events --json outputs valid JSON array.
//
// VALIDATES: --json flag produces valid JSON array with event entries.
// PREVENTS: --json flag producing invalid JSON or empty output.
func TestCmdEventsJSON(t *testing.T) {
	var code int
	output := captureStdout(t, func() {
		code = cmdEvents([]string{"--json"}, nil)
	})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	var entries []map[string]string
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, output)
	}
	if len(entries) == 0 {
		t.Fatal("expected non-empty JSON array for events")
	}
	for _, e := range entries {
		if e["method"] == "" {
			t.Errorf("entry missing 'method': %v", e)
		}
		if e["module"] == "" {
			t.Errorf("entry missing 'module': %v", e)
		}
	}
}

// TestRunMethods verifies ze schema methods via the Run dispatch.
//
// VALIDATES: "methods" dispatches to the correct handler.
// PREVENTS: Missing dispatch entry for new command.
func TestRunMethods(t *testing.T) {
	code := Run([]string{"methods"}, nil)
	if code != 0 {
		t.Errorf("Run(methods) = %d, want 0", code)
	}
}

// TestRunEvents verifies ze schema events via the Run dispatch.
//
// VALIDATES: "events" dispatches to the correct handler.
// PREVENTS: Missing dispatch entry for new command.
func TestRunEvents(t *testing.T) {
	code := Run([]string{"events"}, nil)
	if code != 0 {
		t.Errorf("Run(events) = %d, want 0", code)
	}
}
