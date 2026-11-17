package cli

import (
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	bgpyang "github.com/ze-software/ze/internal/component/bgp/yang"
	"github.com/ze-software/ze/internal/component/config/yang"
	_ "github.com/ze-software/ze/internal/component/hub/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
)

// captureStdout runs fn and returns whatever it wrote to os.Stdout.
//
// The reader MUST run while fn writes. A pipe holds 64 KiB on Linux, so reading
// only after fn returned deadlocks fn the moment it writes more than that, and
// one schema dump passes the bound: ze-bgp-conf.yang reached 84,229 bytes at
// bdba58c05e, after which TestPrintedSchemaFormsStillRender/show hung for its
// whole budget with the writer parked in os.File.Write offering 84,230 bytes.
// The test read as a slow test rather than a broken one, so every run of this
// package paid the full timeout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	// The reader owns r until it sends. Its stop path is w.Close() below, which
	// ends the ReadAll; fn cannot outlive this helper, so neither can it.
	type capture struct {
		text string
		err  error
	}
	drained := make(chan capture, 1)
	go func() {
		text, readErr := io.ReadAll(r)
		drained <- capture{text: string(text), err: readErr}
	}()

	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close() //nolint:errcheck,gosec // test cleanup
	os.Stdout = old

	out := <-drained
	r.Close() //nolint:errcheck,gosec // test cleanup
	if out.err != nil {
		t.Fatalf("read pipe: %v", out.err)
	}
	return out.text
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
	err := registerYANG(registry, bgpyang.ZeBGPConfYANG, "ze.bgp", []string{"bgp"}, nil, loaded)
	if err != nil {
		t.Fatalf("first registerYANG failed: %v", err)
	}

	// Verify it's in the loaded map
	if !loaded["ze-bgp-conf"] {
		t.Error("ze-bgp-conf should be marked as loaded after first registration")
	}

	// Register ze-bgp-conf second time - should return nil (skip), not error
	err = registerYANG(registry, bgpyang.ZeBGPConfYANG, "ze.bgp", []string{"bgp"}, nil, loaded)
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

	// BGP consumes its owned root and the BFD profiles its peers reference.
	bgpSchema, err := registry.GetByModule("ze-bgp-conf")
	if err != nil {
		t.Fatalf("failed to get ze-bgp-conf schema: %v", err)
	}

	if !slices.Equal(bgpSchema.WantsConfig, []string{"bgp", "bfd"}) {
		t.Errorf("ze-bgp WantsConfig should be [bgp bfd], got %v", bgpSchema.WantsConfig)
	}
}

// TestExternalPluginEmptySpec proves a `--plugin` whose value is empty or only
// spaces is refused: the registry is not built, the refusal is an operator
// error that says the option names no plugin and gives the command that lists
// the names, and `ze schema list` with such a value exits 1, writes that
// refusal to stderr and lists nothing.
//
// VALIDATES: Empty plugin spec returns error, and the command exits 1.
// PREVENTS: an empty spec skipped as an external program with an empty name,
// so `ze --plugin "" schema list` exits 0 with a warning about plugin "".
func TestExternalPluginEmptySpec(t *testing.T) {
	for _, spec := range []string{"", "   ", "\t"} {
		registry, err := buildSchemaRegistry([]string{spec})
		if err == nil {
			t.Fatalf("spec %q: expected error for empty plugin spec, got nil", spec)
		}
		if registry != nil {
			t.Errorf("spec %q: a registry was built beside the refusal", spec)
		}
		refusal, ok := errors.AsType[*opererr.Error](err)
		if !ok {
			t.Fatalf("spec %q: the refusal is not an operator error: %v", spec, err)
		}
		if refusal.Problem() != "ze schema: a --plugin option names no plugin: its value is empty" {
			t.Errorf("spec %q: problem = %q", spec, refusal.Problem())
		}
		if !strings.Contains(refusal.Why(), "names neither a plugin compiled into ze") {
			t.Errorf("spec %q: why = %q", spec, refusal.Why())
		}
		if refusal.Fix() != "ze show plugin list" {
			t.Errorf("spec %q: fix = %q", spec, refusal.Fix())
		}
	}

	var code int
	var stdout string
	stderr := captureStderr(t, func() {
		stdout = captureStdout(t, func() { code = Run([]string{"list"}, []string{""}) })
	})
	if code != 1 {
		t.Errorf("Run(list) with an empty --plugin = %d, want 1", code)
	}
	if !strings.Contains(stderr, "error: ze schema: a --plugin option names no plugin: its value is empty") {
		t.Errorf("stderr lacks the refusal: %q", stderr)
	}
	if !strings.Contains(stderr, "ze show plugin list") {
		t.Errorf("stderr lacks the fix: %q", stderr)
	}
	if strings.Contains(stderr, "not available with no running ze") {
		t.Errorf("an empty spec was reported as an external plugin: %q", stderr)
	}
	if stdout != "" {
		t.Errorf("a refused command listed schemas: %q", stdout)
	}
}

// TestRunWarnsOfExternalPluginOnStderr proves a `ze schema` command that the
// command line gives an external plugin still answers from the compiled-in
// modules, exits 0, and writes on stderr which plugin its answer shows nothing
// of, why, and the command that reaches a running ze. Every command that reads
// the registry does. A command line of compiled-in plugins writes no warning.
//
// VALIDATES: the owner ruling of 2026-11-16 at the command: the answer says
// the external plugin's schema is not available and names what to run.
// PREVENTS: commandLineRegistry losing its writeExternalNotShown call, after
// which the answer reads as the whole schema and every other test stays green.
func TestRunWarnsOfExternalPluginOnStderr(t *testing.T) {
	commands := [][]string{{"list"}, {"show", bgpConfModule}, {"handlers"}, {"methods"}, {"events"}}
	for _, args := range commands {
		var code int
		var stdout string
		stderr := captureStderr(t, func() {
			stdout = captureStdout(t, func() { code = Run(args, []string{"./acme-monitor"}) })
		})
		if code != 0 {
			t.Errorf("Run(%v) = %d, want 0: the compiled-in answer stands", args, code)
		}
		if stdout == "" {
			t.Errorf("Run(%v) wrote no answer", args)
		}
		for _, want := range []string{
			`warning: ze schema answered from the modules compiled into ze alone: the config schema of external plugin "./acme-monitor" is not available with no running ze`,
			"ze starts no external plugin's program",
			"ze cli",
		} {
			if !strings.Contains(stderr, want) {
				t.Errorf("Run(%v) stderr lacks %q: %q", args, want, stderr)
			}
		}
	}

	stderr := captureStderr(t, func() {
		captureStdout(t, func() { Run([]string{"list"}, []string{"ze.bgp"}) })
	})
	if strings.Contains(stderr, "warning:") {
		t.Errorf("a command line of compiled-in plugins wrote a warning: %q", stderr)
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

	// Every rpc the four modules declare is published under at least one
	// method, which is the method of a command node that points at it. The
	// expectation is read from the modules themselves, so a new rpc is
	// covered the day it is written, and an rpc no node points at is named.
	loader, err := yang.DefaultLoader()
	if err != nil {
		t.Fatalf("load YANG: %v", err)
	}
	published := make(map[string]bool)
	for _, rpc := range registry.ListRPCs("") {
		published[rpc.Module+":"+rpc.Name] = true
	}
	for _, module := range []string{"ze-bgp-api", "ze-system-api", "ze-plugin-api", "ze-rib-api"} {
		declared := yang.ExtractRPCs(loader, module)
		if len(declared) == 0 {
			t.Errorf("%s declares no rpc in this binary, so this test proves nothing about it", module)
		}
		for _, rpc := range declared {
			if !published[module+":"+rpc.Name] {
				t.Errorf("rpc %s in %s is published under no method", rpc.Name, module)
			}
		}
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
		"ze-bgp:peer-list", "ze-meta:bgp-help",
		"ze-cmd:bgp-subscribe", "ze-cmd:bgp-unsubscribe", "ze-bgp:commit",
		"ze-plugin:system-help", "ze-plugin:system-version-software", "ze-plugin:system-daemon-status",
		"ze-plugin:help", "ze-plugin:session-ping", "ze-plugin:session-bye",
		// The RIB show rpc is published under the method of the node that
		// points at it, `show rib routes`; no node declares ze-rib:show.
		"ze-bgp:rib-routes",
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
