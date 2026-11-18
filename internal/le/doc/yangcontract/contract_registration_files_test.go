// Design: docs/contributing/documentation-testing.md -- registration and owner contracts.
// Related: contract.go -- source discovery and registered wire-method validation.

package docyangcontract

import (
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestLocalHandlersReachSplitRegistrationFiles reads actual registration calls
// from split owner files while excluding test, backup and unrelated Go files.
func TestLocalHandlersReachSplitRegistrationFiles(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, filepath.Join("cmd", "ze", "main.go"), "package main\nfunc main() {}\n")
	files := map[string]string{
		"cmd/ze/thing/register.go":                                     "command.MustRegisterLocal(\"show baseline\", nil)",
		"cmd/ze/thing/register_output.go":                              "command.MustRegisterLocal(\"show output\", nil)",
		"internal/component/config/cli/register_supplied_utilities.go": "command.MustRegisterLocalData(\"show config supplied completion\", nil); command.MustRegisterLocalData(\"show secret encode\", nil); command.MustRegisterLocalData(\"show secret decode\", nil)",
		"internal/plugins/thing/register_detail.go":                    "command.MustRegisterLocalMeta(\"offline detail\", nil, registry.Meta{DaemonPath: \"show detail\"})",
		"internal/plugins/thing/register_detail_test.go":               "command.MustRegisterLocal(\"show test-only\", nil)",
		"internal/plugins/thing/register_detail.go.orig":               "command.MustRegisterLocal(\"show backup\", nil)",
		"internal/plugins/thing/other.go":                              "command.MustRegisterLocal(\"show unrelated\", nil)",
		"cmd/ze/thing/register_output_test.go":                         "command.MustRegisterLocal(\"show root-test-only\", nil)",
	}
	for path, registration := range files {
		writeDoc(t, root, filepath.FromSlash(path), "package thing\nfunc init() { "+registration+" }\n")
	}

	got, err := scanLocalHandlers(root)
	if err != nil {
		t.Fatalf("scan split registrations: %v", err)
	}
	want := map[string]string{
		"show baseline":                   "",
		"show output":                     "",
		"show config supplied completion": "",
		"show secret encode":              "",
		"show secret decode":              "",
		"offline detail":                  "show detail",
	}
	if !maps.Equal(got, want) {
		t.Fatalf("local handlers = %v, want exactly %v", got, want)
	}
}

// TestLocalHandlersRefuseAnUnparseableSplitRegistration requires both discovery
// roots to report a broken split file instead of silently omitting its commands.
func TestLocalHandlersRefuseAnUnparseableSplitRegistration(t *testing.T) {
	for _, path := range []string{
		"cmd/ze/thing/register_output.go",
		"internal/component/config/cli/register_supplied_utilities.go",
	} {
		t.Run(path, func(t *testing.T) {
			root := t.TempDir()
			writeDoc(t, root, filepath.Join("cmd", "ze", "main.go"), "package main\nfunc main() {}\n")
			writeDoc(t, root, filepath.FromSlash(path), "package thing\nfunc init() {\n")
			if _, err := scanLocalHandlers(root); err == nil {
				t.Fatal("an unparseable split registration was silently omitted")
			}
		})
	}
}

// TestArtifactMethodsUseTheirRegisteringOwner checks every artifact model and
// handler method together against the actual registering package's namespace.
func TestArtifactMethodsUseTheirRegisteringOwner(t *testing.T) {
	const registrar = "github.com/ze-software/ze/internal/component/config/cli"
	owned, err := pluginserver.OwnerPrefix(registrar)
	if err != nil {
		t.Fatalf("derive artifact registrar prefix: %v", err)
	}
	handlers := make(map[string]bool)
	var foreign []string
	for _, rpc := range pluginserver.AllBuiltinRPCs() {
		if rpc.Registrar() != registrar {
			continue
		}
		_, operation, _ := strings.Cut(rpc.WireMethod, ":")
		if !strings.HasPrefix(operation, "artifact-") {
			continue
		}
		if handlers[rpc.WireMethod] {
			t.Fatalf("artifact method %q has duplicate builtin handlers", rpc.WireMethod)
		}
		handlers[rpc.WireMethod] = true
		if !strings.HasPrefix(rpc.WireMethod, owned+":") {
			foreign = append(foreign, rpc.WireMethod)
		}
	}
	if len(handlers) == 0 {
		t.Fatal("no artifact builtin handler was inspected")
	}
	if len(foreign) > 0 {
		t.Errorf("artifact handlers registered by %s must own %s: %v", registrar, owned, foreign)
	}

	schema, err := yang.DefaultLoader()
	if err != nil {
		t.Fatalf("load artifact model: %v", err)
	}
	entry := schema.GetEntry("ze-config-artifact-cmd")
	if entry == nil {
		t.Fatal("the artifact command module was not loaded")
	}
	// The artifact module augments show config in another command module.
	// Follow Validate's full resolved-module walk before selecting operations.
	var commands []CommandEntry
	for _, module := range schema.ModuleNames() {
		if !strings.HasSuffix(module, cmdModuleSuffix) {
			continue
		}
		walkEntry(schema.GetEntry(module), "", module, &commands)
	}
	modeled := make(map[string]bool)
	for _, command := range commands {
		_, operation, _ := strings.Cut(command.WireMethod, ":")
		if !strings.HasPrefix(operation, "artifact-") {
			continue
		}
		if !strings.HasPrefix(command.WireMethod, owned+":") {
			t.Errorf("artifact model command %s names foreign method %s; want %s", command.YANGPath, command.WireMethod, owned)
		}
		if modeled[command.WireMethod] {
			t.Fatalf("artifact method %q has duplicate model commands", command.WireMethod)
		}
		modeled[command.WireMethod] = true
	}
	if len(modeled) == 0 {
		t.Fatal("no artifact model command was inspected")
	}
	if !maps.Equal(handlers, modeled) {
		t.Fatalf("artifact builtin methods %v differ from modeled methods %v", handlers, modeled)
	}
}
