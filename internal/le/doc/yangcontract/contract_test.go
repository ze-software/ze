// The contract gate's tests. Every one calls the gate as a FUNCTION
// (spec-le-is-a-ze-binary, AC-5); the same assertions used to run the script
// with `go run` and could only read its printed page.

package docyangcontract

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/aihelp"
	"github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// repoRoot answers this checkout, which is the only tree carrying the command
// registrations the live registry is judged against.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the repository root: %v", err)
	}
	return root
}

// VALIDATES: the command table is sorted on wire method, THEN YANG path, THEN
// module, so one wire method reached from two YANG paths lands in one order.
// PREVENTS: the report changing between two runs over one unchanged tree. The
// script sorted on the wire method alone with an unstable sort, over rows a map
// walk produced, and five runs of it printed four different tables.
func TestSortCommandsIsATotalOrder(t *testing.T) {
	commands := []CommandEntry{
		{WireMethod: "b:one", YANGPath: "z", Module: "m"},
		{WireMethod: "a:two", YANGPath: "create > interface > unit", Module: "ze-iface-cmd"},
		{WireMethod: "a:two", YANGPath: "create > interface > dummy > unit", Module: "ze-iface-cmd"},
		{WireMethod: "a:two", YANGPath: "create > interface > dummy > unit", Module: "a-cmd"},
	}
	sortCommands(commands)

	want := []CommandEntry{
		{WireMethod: "a:two", YANGPath: "create > interface > dummy > unit", Module: "a-cmd"},
		{WireMethod: "a:two", YANGPath: "create > interface > dummy > unit", Module: "ze-iface-cmd"},
		{WireMethod: "a:two", YANGPath: "create > interface > unit", Module: "ze-iface-cmd"},
		{WireMethod: "b:one", YANGPath: "z", Module: "m"},
	}
	for i := range want {
		if commands[i] != want[i] {
			t.Fatalf("row %d is %v, want %v", i, commands[i], want[i])
		}
	}
}

// VALIDATES: two runs over this checkout answer the same table, byte for byte.
// PREVENTS: a report that cannot be diffed, which is what a table pasted into a
// document needs above everything else.
func TestValidateAnswersTheSameTableTwice(t *testing.T) {
	root := repoRoot(t)
	first, err := Validate(root)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	second, err := Validate(root)
	if err != nil {
		t.Fatalf("validate a second time: %v", err)
	}
	if first.Text() != second.Text() {
		t.Fatal("two runs over one unchanged tree answered different tables")
	}
}

// VALIDATES: every YANG ze:command node has a handler in this checkout.
// PREVENTS: an owner move dropping a registration across the command import
// islands (plugin/all, the cmd handler packages, the cli client, and this
// file's own import list). This is TestAllYangCommandsHaveRegisteredRPC, as a
// function call: it now names the orphans instead of grepping a printed page.
func TestEveryYANGCommandHasAHandler(t *testing.T) {
	result, err := Validate(repoRoot(t))
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.Total == 0 {
		t.Fatal("the YANG command tree is empty, so this test proves nothing about it")
	}
	if len(result.OrphanYANG) > 0 {
		t.Errorf("%d YANG commands have no handler: %v", len(result.OrphanYANG), result.OrphanYANG)
	}
	if !result.Valid {
		t.Errorf("the command contract is not satisfied: %d orphan handlers: %v",
			len(result.OrphanHandlers), result.OrphanHandlers)
	}
}

// VALIDATES: every local command handler in this checkout is declared by a
// YANG ze:command node at the path it registers, one declaration per command.
// PREVENTS: a command `ze <verb>` runs that help, completion and the command
// tree never offer. On 2026-10-08 fourteen such commands existed (the debug
// profile commands, explain, skills, support, generate wireguard keypair,
// show config graph, validate config). The verdict is read on its own, apart
// from TestEveryYANGCommandHasAHandler, so a red RPC set elsewhere cannot hide it.
func TestEveryLocalHandlerHasAYANGCommand(t *testing.T) {
	result, err := Validate(repoRoot(t))
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.TotalLocal == 0 {
		t.Fatal("no local handler was found, so this test proves nothing about them")
	}
	if len(result.OrphanLocalHandlers) > 0 {
		t.Errorf("%d local handlers have no YANG command: %v",
			len(result.OrphanLocalHandlers), result.OrphanLocalHandlers)
	}
}

// VALIDATES: a command registered with RegisterLocalData is counted as a local
// handler, and so is every other spelling of the registration.
// PREVENTS: a new registration API blinding this checker silently. Adding
// RegisterLocalData without adding it here reported twelve YANG commands as
// having no handler on the day they gained one. This is
// TestLocalDataRegistrationsAreCounted, moved to the level the cause lives at:
// it names the spelling rather than whichever commands happen to use it.
func TestLocalHandlersCoverEveryRegistrationSpelling(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, filepath.Join("cmd", "ze", "main.go"), "package main\n\nfunc main() {}\n")
	writeDoc(t, root, filepath.Join("cmd", "ze", "thing", "register.go"),
		"package thing\n\n"+
			"func init() {\n"+
			"\tregistry.MustRegisterLocal(\"show one\", nil)\n"+
			"\tregistry.MustRegisterLocalMeta(\"show two\", nil)\n"+
			"\tregistry.RegisterLocal(\"show three\", nil)\n"+
			"\tregistry.RegisterLocalMeta(\"show four\", nil)\n"+
			"\tregistry.MustRegisterLocalData(\"show five\", nil)\n"+
			"\tregistry.RegisterLocalData(\"show six\", nil)\n"+
			"\tcmdregistry.MustRegisterLocal(\"show seven\", nil)\n"+
			"\tregistry.RegisterSomethingElse(\"show eight\", nil)\n"+
			"\tother.MustRegisterLocal(\"show nine\", nil)\n"+
			"}\n")

	got, err := collectLocalHandlers(root)
	if err != nil {
		t.Fatalf("collect the local handlers: %v", err)
	}
	want := []string{"show five", "show four", "show one", "show seven", "show six", "show three", "show two"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("the local handlers are %v, want %v", got, want)
	}
}

// VALIDATES: the owner command packages under internal/ are scanned as well as
// cmd/ze.
// PREVENTS: a migrated owner's `show X` shortcut reading as unregistered,
// which is the half of the contract the filesystem walk exists for.
func TestLocalHandlersReachTheOwnerPackages(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, filepath.Join("cmd", "ze", "main.go"), "package main\n\nfunc main() {}\n")
	writeDoc(t, root, filepath.Join("internal", "plugins", "explain", "register.go"),
		"package explain\n\nfunc init() { registry.MustRegisterLocal(\"explain thing\", nil) }\n")
	writeDoc(t, root, filepath.Join("internal", "component", "cli", "register.go"),
		"package cli\n\nfunc init() { registry.MustRegisterLocal(\"show editor\", nil) }\n")
	writeDoc(t, root, filepath.Join("internal", "component", "elsewhere", "register.go"),
		"package elsewhere\n\nfunc init() { registry.MustRegisterLocal(\"show elsewhere\", nil) }\n")

	got, err := collectLocalHandlers(root)
	if err != nil {
		t.Fatalf("collect the local handlers: %v", err)
	}
	if strings.Join(got, "|") != "explain thing|show editor" {
		t.Fatalf("the local handlers are %v, want the explain and cli registrations only", got)
	}
}

// VALIDATES: a registry file that does not parse stops the gate.
// PREVENTS: a broken source file reading as a package that registers nothing,
// which reports every command it holds as having no handler.
func TestValidateFailsOnARegistryItCannotParse(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, filepath.Join("cmd", "ze", "main.go"), "package main\n\nfunc main() {\n")

	if _, err := collectLocalHandlers(root); err == nil {
		t.Fatal("a source file that does not parse was accepted")
	}
}

// VALIDATES: a tree with no internal/ is walked without complaint.
// PREVENTS: the fail-closed walk turning every fixture tree into an error.
func TestLocalHandlersAcceptATreeWithNoInternalDirectory(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, filepath.Join("cmd", "ze", "main.go"), "package main\n\nfunc main() {}\n")

	got, err := collectLocalHandlers(root)
	if err != nil {
		t.Fatalf("a tree with no internal/ was refused: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("a tree with no registration answered %v", got)
	}
}

// VALIDATES: a directory the walk cannot read stops the gate.
// PREVENTS: the fail-open the script carried: its walk answered nil on the
// error, so a register.go it could not reach was simply absent, and every
// command that file registers reads as unhandled in one direction and as
// nothing at all in the other.
func TestLocalHandlersRefuseATreeItCannotWalk(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a 0000 directory, so this fixture cannot be built as root")
	}
	root := t.TempDir()
	writeDoc(t, root, filepath.Join("cmd", "ze", "main.go"), "package main\n\nfunc main() {}\n")
	writeDoc(t, root, filepath.Join("internal", "closed", "cli", "register.go"),
		"package cli\n\nfunc init() { registry.MustRegisterLocal(\"show hidden\", nil) }\n")
	closed := filepath.Join(root, "internal", "closed")
	if err := os.Chmod(closed, 0o000); err != nil {
		t.Fatalf("close the fixture directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(closed, 0o750) })

	if _, err := collectLocalHandlers(root); err == nil {
		t.Fatal("a tree holding a directory the gate cannot read was accepted")
	}
}

// VALIDATES: the two editor mode handlers are skipped rather than reported as
// orphans, and every other handler is judged.
// PREVENTS: the skip list growing silently, which is how a real orphan hides.
func TestSkippedHandlersAreTheEditorModesOnly(t *testing.T) {
	if len(skippedWireMethods) != 2 {
		t.Fatalf("the skip list holds %d entries: %v", len(skippedWireMethods), skippedWireMethods)
	}
	for _, wm := range []string{"ze-editor:mode-command", "ze-editor:mode-edit"} {
		if !skippedWireMethods[wm] {
			t.Errorf("%s is no longer skipped", wm)
		}
	}
	if got := skipReason("ze-editor:mode-command"); got != "run -- editor mode switch" {
		t.Errorf("the reason for mode-command is %q", got)
	}
	if got := skipReason("ze-editor:mode-edit"); got != "edit -- editor mode switch" {
		t.Errorf("the reason for mode-edit is %q", got)
	}
}

// VALIDATES: a YANG path becomes the words an operator types.
// PREVENTS: the local-handler cross-check comparing two different spellings of
// one command, which reports every local handler as an orphan.
func TestYANGPathBecomesTheTypedCommand(t *testing.T) {
	if got := yangPathToCLIPath("show > env > list"); got != "show env list" {
		t.Fatalf("the typed command is %q", got)
	}
}

// VALIDATES: a local handler with no YANG command node fails the verdict
// (AC-16), and the report names it under the failure.
// PREVENTS: the hole this gate carried, where `contractSatisfied` read two of
// the three orphan sets and printed "Local handlers with no YANG command"
// above "All commands validated.", so the rows changed no verdict.
// MUTATION: drop the OrphanLocalHandlers guard from contractSatisfied and this
// test goes red.
func TestAnOrphanLocalHandlerFailsTheVerdict(t *testing.T) {
	result := ValidationResult{OrphanLocalHandlers: []string{"show thing"}}
	if contractSatisfied(&result) {
		t.Fatal("a local handler with no YANG command node satisfied the contract")
	}
	result.Valid = contractSatisfied(&result)
	text := result.Text()
	if !strings.Contains(text, "FAILED: 1 problem(s)") {
		t.Fatalf("the report does not fail on the orphan local handler:\n%s", text)
	}
	if !strings.Contains(text, "  show thing\n") {
		t.Fatalf("the report does not name the orphan local handler:\n%s", text)
	}
}

// VALIDATES: an rpc declaration whose published method no handler serves fails
// the verdict (AC-15).
// PREVENTS: a published method that resolves to nothing passing the gate.
// MUTATION: drop the OrphanRPCs clause from contractSatisfied and this test
// goes red.
func TestAnUnservedRPCDeclarationFailsTheVerdict(t *testing.T) {
	result := ValidationResult{OrphanRPCs: []RPCDeclaration{
		{WireMethod: "ze-fixture:socket-clear", Module: "ze-fixture-api", RPC: "socket-clear"},
	}}
	if contractSatisfied(&result) {
		t.Fatal("an rpc declaration no handler serves satisfied the contract")
	}
}

// fixtureDeclarations answers the published rpcs of the fixture `-api` module
// alone, read through the gate's own reader over the shared fixture loader.
func fixtureDeclarations(t *testing.T) []RPCDeclaration {
	t.Helper()
	declarations, err := publishedRPCs(shapeLoader(t, shapeModule, shapeAPIModule))
	if err != nil {
		t.Fatalf("read the published rpcs: %v", err)
	}
	var fixture []RPCDeclaration
	for _, rpc := range declarations {
		if rpc.Module == "ze-fixture-api" {
			fixture = append(fixture, rpc)
		}
	}
	return fixture
}

// VALIDATES: the gate reads every rpc of an `-api` module under the wire
// method the help surfaces publish, which strips `-api` from the module name.
// PREVENTS: the gate judging a spelling no caller reads. A declaration judged
// as `ze-fixture-api:socket-list` would never match the handler a caller
// reaches as `ze-fixture:socket-list`, in either direction.
func TestPublishedRPCsCarryTheWireMethodCallersRead(t *testing.T) {
	fixture := fixtureDeclarations(t)
	want := []RPCDeclaration{
		{WireMethod: "ze-fixture:socket-clear", Module: "ze-fixture-api", RPC: "socket-clear"},
		{WireMethod: "ze-fixture:socket-list", Module: "ze-fixture-api", RPC: "socket-list"},
	}
	if len(fixture) != len(want) {
		t.Fatalf("the fixture module publishes %v, want %v", fixture, want)
	}
	for i := range want {
		if fixture[i] != want[i] {
			t.Fatalf("row %d is %v, want %v", i, fixture[i], want[i])
		}
	}
}

// VALIDATES: a deliberately orphaned declaration is named by module, rpc and
// wire method, and fails the run, while its served sibling passes (AC-15).
// PREVENTS: the population hole: the gate opened `-cmd` modules only, so an
// `-api` rpc no handler serves was never read at all.
// MUTATION: make unservedRPCs answer nil and this test goes red.
func TestADeliberatelyOrphanedDeclarationIsNamed(t *testing.T) {
	served := map[string]bool{"ze-fixture:socket-list": true}
	result := ValidationResult{OrphanRPCs: unservedRPCs(fixtureDeclarations(t), served)}

	want := RPCDeclaration{WireMethod: "ze-fixture:socket-clear", Module: "ze-fixture-api", RPC: "socket-clear"}
	if len(result.OrphanRPCs) != 1 {
		t.Fatalf("the orphans are %v, want only %v", result.OrphanRPCs, want)
	}
	if result.OrphanRPCs[0] != want {
		t.Fatalf("the orphan is %v, want %v", result.OrphanRPCs[0], want)
	}
	result.Valid = contractSatisfied(&result)
	if result.Valid {
		t.Fatal("a run holding an orphaned declaration passed")
	}
	text := result.Text()
	if !strings.Contains(text, "  ze-fixture:socket-clear  (rpc socket-clear in ze-fixture-api)\n") {
		t.Fatalf("the report does not name the module, the rpc and the wire method:\n%s", text)
	}
}

// VALIDATES: a loader with no `-api` module stops the gate.
// PREVENTS: a run that judged no declaration answering that none is orphaned,
// the silent zero aihelp.SchemaRegistry answers after a loader error.
func TestPublishedRPCsRefuseALoaderWithNoAPIModule(t *testing.T) {
	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("load the embedded modules: %v", err)
	}
	if _, err := publishedRPCs(loader); !errors.Is(err, errNoAPIModule) {
		t.Fatalf("a loader with no -api module answered %v, want %v", err, errNoAPIModule)
	}
}

// VALIDATES: every method `ze help ai --json` publishes from the YANG schema
// has a registered handler in this checkout (AC-14). `ze schema methods`
// builds the same set through RegisterRPCs over the same `-api` modules.
// PREVENTS: an MCP client or an operator reading a method name that the
// daemon answers with "unknown method".
//
// The set is aihelp.SchemaRegistry, never aihelp.Build: Build appends every
// builtin handler to the declared methods, so a schema registry that came back
// empty after a loader error still published a non-empty list, every row of it
// served, and this test passed having judged no declaration.
func TestEveryPublishedMethodHasAHandler(t *testing.T) {
	served := map[string]bool{}
	for _, rpc := range pluginserver.AllBuiltinRPCs() {
		served[rpc.WireMethod] = true
	}
	published := aihelp.SchemaRegistry().ListRPCs("")
	if len(published) == 0 {
		t.Fatal("the help reference publishes no YANG method, so this test proves nothing")
	}
	var unserved []string
	for _, rpc := range published {
		if !served[rpc.WireMethod] {
			unserved = append(unserved, rpc.WireMethod)
		}
	}
	if len(unserved) > 0 {
		t.Errorf("%d published methods have no handler: %v", len(unserved), unserved)
	}
}
