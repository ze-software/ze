// VALIDATES: every plugin's runner declaration is also on its registration,
// and the gate that says so goes red when one is not.
// PREVENTS: a command the daemon serves and the published catalog never names.

package plugindeclarations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

// TestEveryRunnerDeclarationIsOnItsRegistration is AC-7 of
// plan/spec-daemon-backed-command-catalog.md, over the real checkout: a plugin
// declaring a command to Stage 1 and not to its registry.Registration is
// invisible to every reader that does not start a daemon.
func TestEveryRunnerDeclarationIsOnItsRegistration(t *testing.T) {
	tree, err := lepath.Root()
	if err != nil {
		t.Fatalf("locate the checkout: %v", err)
	}

	findings, err := Check(tree, packageFloor)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	for _, finding := range findings {
		t.Errorf("%s declares %q at %s:%d and its registration does not carry it: %s",
			finding.Package, finding.Command, finding.File, finding.Line, finding.Reason)
	}
}

// TestCheckNamesAPluginThatDeclaresOnlyToStageOne is the gate's own red. A
// check written against a tree that already agrees has never been observed to
// fail, so its discrimination is unproven until a fixture forces one
// (ai/rules/interop-and-goal-validation.md).
func TestCheckNamesAPluginThatDeclaresOnlyToStageOne(t *testing.T) {
	tree := writeFixture(t, "drifter", `
func init() {
	_ = registry.Register(registry.Registration{
		Name:      "drifter",
		RunEngine: runDrifter,
	})
}

func runDrifter(conn net.Conn) int {
	var p sdk.Plugin
	_ = p.Run(nil, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "show drifter status"},
			{Name: "show drifter peers"},
		},
	})
	return 0
}
`)

	findings, err := Check(tree, 0)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("the gate answered %d finding(s) for two undeclared commands: %+v", len(findings), findings)
	}
	for _, want := range []string{"show drifter peers", "show drifter status"} {
		if !namesCommand(findings, want) {
			t.Errorf("the gate did not name %q: %+v", want, findings)
		}
	}
	if findings[0].Package != "internal/plugins/drifter" {
		t.Errorf("the finding names package %q, and the plugin is internal/plugins/drifter", findings[0].Package)
	}
	if !strings.Contains(findings[0].File, "plugin.go") {
		t.Errorf("the finding points at %q rather than the file the runner declares in", findings[0].File)
	}
}

// TestCheckAcceptsOneFunctionServingBothReaders is the green half: the shape
// every plugin is meant to have, where one commandDecls() is named by the
// registration and by the runner. The gate must follow the call rather than
// compare two spellings of it.
func TestCheckAcceptsOneFunctionServingBothReaders(t *testing.T) {
	tree := writeFixture(t, "agreeing", `
func init() {
	_ = registry.Register(registry.Registration{
		Name:      "agreeing",
		RunEngine: runAgreeing,
		Commands:  commandDecls(),
	})
}

func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{Name: "show agreeing status"},
		{Name: cmdShowPeers},
	}
}

func runAgreeing(conn net.Conn) int {
	var p sdk.Plugin
	_ = p.Run(nil, sdk.Registration{Commands: commandDecls()})
	return 0
}
`)

	findings, err := Check(tree, 0)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("one function serving both readers was reported as a disagreement: %+v", findings)
	}
}

// TestCheckReportsADeclarationItCannotRead pins the fail-closed answer. A
// Commands field the reader cannot resolve is a comparison it cannot make, and
// answering "they agree" for one would be a zero standing in for an answer
// (ai/rules/principles.md).
func TestCheckReportsADeclarationItCannotRead(t *testing.T) {
	tree := writeFixture(t, "opaque", `
func init() {
	_ = registry.Register(registry.Registration{
		Name:      "opaque",
		RunEngine: runOpaque,
		Commands:  commandDecls(),
	})
}

func commandDecls() []sdk.CommandDecl { return nil }

func runOpaque(conn net.Conn) int {
	var p sdk.Plugin
	_ = p.Run(nil, sdk.Registration{Commands: decksFromElsewhere(1)})
	return 0
}

func decksFromElsewhere(n int) []sdk.CommandDecl { return nil }
`)

	findings, err := Check(tree, 0)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("an unreadable declaration answered %d finding(s): %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Reason, "cannot be read") {
		t.Errorf("the finding reads %q, and it must say the declaration could not be read", findings[0].Reason)
	}
}

// TestCheckPassesOverAPackageThatRegistersNoPlugin pins the scope rule. A test
// fixture builds an sdk.Registration and registers no plugin, so it has no
// registration to agree with and is not this gate's subject.
func TestCheckPassesOverAPackageThatRegistersNoPlugin(t *testing.T) {
	tree := t.TempDir()
	dir := filepath.Join(tree, "internal", "test", "fixture")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("create %s: %v", dir, err)
	}
	source := `package fixture

import (
	"net"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func drive(conn net.Conn) {
	var p sdk.Plugin
	_ = p.Run(nil, sdk.Registration{Commands: []sdk.CommandDecl{{Name: "show fixture thing"}}})
}
`
	if err := os.WriteFile(filepath.Join(dir, "fixture.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write the fixture: %v", err)
	}

	findings, err := Check(tree, 0)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("a package registering no plugin was judged: %+v", findings)
	}
}

// TestCheckRefusesAWalkThatReadTooLittle pins the floor. An empty answer over a
// tree the walk never reached is a failed read, and reporting it as a clean
// tree is the certifying zero this repository has paid for before.
func TestCheckRefusesAWalkThatReadTooLittle(t *testing.T) {
	tree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tree, "internal"), 0o750); err != nil {
		t.Fatalf("create the empty tree: %v", err)
	}

	if _, err := Check(tree, packageFloor); err == nil {
		t.Fatal("a walk that found no plugin package answered a clean tree")
	}
}

// writeFixture writes body into internal/plugins/<name>/plugin.go of a fresh
// temporary tree, under the imports every plugin file carries, and answers the
// tree root. The package name is the plugin directory.
func writeFixture(t *testing.T, name, body string) string {
	t.Helper()

	tree := t.TempDir()
	dir := filepath.Join(tree, "internal", "plugins", name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("create %s: %v", dir, err)
	}

	var source strings.Builder
	source.WriteString("package " + name + "\n\nimport (\n\t\"net\"\n\n")
	source.WriteString("\t\"github.com/ze-software/ze/internal/component/plugin/registry\"\n")
	source.WriteString("\t\"github.com/ze-software/ze/pkg/plugin/sdk\"\n)\n")
	source.WriteString(body)

	if err := os.WriteFile(filepath.Join(dir, "plugin.go"), []byte(source.String()), 0o600); err != nil {
		t.Fatalf("write the plugin fixture: %v", err)
	}
	return tree
}

// namesCommand reports whether any finding names command.
func namesCommand(findings Findings, command string) bool {
	for _, finding := range findings {
		if finding.Command == command {
			return true
		}
	}
	return false
}

// VALIDATES: the gate covers the PIPE channel, not the command channel alone. A
// plugin that declares a pipe alias to Stage 1 and leaves it off
// registry.Registration.Pipes is named, and the finding identifies the alias by
// the pair its registry keys it on.
// PREVENTS: the drift this spec removed for answer shapes coming back on the
// alias channel. An alias on Stage 1 alone reaches no in-process reader, so the
// published catalog would list a command without the name it answers to and
// nothing would go red.
func TestCheckNamesAPipeAliasMissingFromTheRegistration(t *testing.T) {
	tree := writeFixture(t, "piper", `
func init() {
	_ = registry.Register(registry.Registration{
		Name:      "piper",
		RunEngine: runPiper,
		Commands:  commandDecls(),
	})
}

func commandDecls() []sdk.CommandDecl { return []sdk.CommandDecl{{Name: "show piper status"}} }

func runPiper(conn net.Conn) int {
	var p sdk.Plugin
	_ = p.Run(nil, sdk.Registration{
		Commands: commandDecls(),
		Pipes: []sdk.PipeDecl{
			{Command: "show piper status", Name: "summary", Expansion: "display total"},
		},
	})
	return 0
}
`)

	findings, err := Check(tree, 0)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("an undeclared pipe alias answered %d finding(s): %+v", len(findings), findings)
	}
	if findings[0].Command != "show piper status | summary" {
		t.Errorf("the finding names %q, and a pipe alias is identified by its command and its name",
			findings[0].Command)
	}
	if !strings.Contains(findings[0].Reason, "registry.Registration.Pipes") {
		t.Errorf("the finding reads %q, and it must name the field that is missing it", findings[0].Reason)
	}
}

// VALIDATES: a field both literals write as a call to ONE parameterless
// function is compared by that function's identity, not by reading its body.
// One function answers one slice, so the two readings cannot disagree.
// PREVENTS: the gate refusing the shape its own remedy text asks for. bgp-rib
// builds its declaration list in a loop over the dispatch table that already
// holds every summary, which is the one-declaration answer this repository
// wants and which no syntactic reader can evaluate; before this rule the gate
// reported it as "cannot be read".
func TestCheckAcceptsADeclarationFunctionItCannotEvaluate(t *testing.T) {
	tree := writeFixture(t, "looper", `
func init() {
	_ = registry.Register(registry.Registration{
		Name:      "looper",
		RunEngine: runLooper,
		Commands:  commandDecls(),
	})
}

var table = map[string]string{"show looper status": "Show the status"}

func commandDecls() []sdk.CommandDecl {
	decls := make([]sdk.CommandDecl, 0, len(table))
	for name, help := range table {
		decls = append(decls, sdk.CommandDecl{Name: name, Description: help})
	}
	return decls
}

func runLooper(conn net.Conn) int {
	var p sdk.Plugin
	_ = p.Run(nil, sdk.Registration{Commands: commandDecls()})
	return 0
}
`)

	findings, err := Check(tree, 0)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("one function serving both readers answered %d finding(s): %+v", len(findings), findings)
	}
}

// VALIDATES: the two literals are compared FIELD by field, not by name alone.
// PREVENTS: the gate passing over exactly the fields this spec added. Shape,
// Columns and AddressFields decide which pipe operators a command publishes, and
// the catalog is now generated from the registration, so a Shape the two
// literals spell differently is a published answer no running command gives.
func TestCheckNamesAFieldTheTwoLiteralsSpellDifferently(t *testing.T) {
	tree := writeFixture(t, "diverging", `
func init() {
	_ = registry.Register(registry.Registration{
		Name:      "diverging",
		RunEngine: runDiverging,
		Commands: []sdk.CommandDecl{
			{Name: "show diverging rows", Shape: "doc"},
		},
	})
}

func runDiverging(conn net.Conn) int {
	var p sdk.Plugin
	_ = p.Run(nil, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "show diverging rows", Shape: "tab", Columns: []string{"peer"}},
		},
	})
	return 0
}
`)

	findings, err := Check(tree, 0)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("two literals stating different shapes answered %d finding(s): %+v", len(findings), findings)
	}
	if findings[0].Command != "show diverging rows" {
		t.Errorf("the finding names %q, and the two literals disagree about \"show diverging rows\"", findings[0].Command)
	}
	if !strings.Contains(findings[0].Reason, "different fields") {
		t.Errorf("the finding reads %q, and it must say the two state different fields", findings[0].Reason)
	}
}

// VALIDATES: the comparison runs in BOTH directions. A command on the
// registration that the runner never declares is named too.
// PREVENTS: a phantom command on the published website. The catalog is
// generated FROM registry.Registration.Commands, so a registration-only command
// is published to an operator and answered by no daemon, and that is the
// direction the gate had no rule for.
func TestCheckNamesACommandOnlyTheRegistrationCarries(t *testing.T) {
	tree := writeFixture(t, "phantom", `
func init() {
	_ = registry.Register(registry.Registration{
		Name:      "phantom",
		RunEngine: runPhantom,
		Commands: []sdk.CommandDecl{
			{Name: "show phantom status"},
			{Name: "show phantom ghost"},
		},
	})
}

func runPhantom(conn net.Conn) int {
	var p sdk.Plugin
	_ = p.Run(nil, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "show phantom status"},
		},
	})
	return 0
}
`)

	findings, err := Check(tree, 0)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("a registration-only command answered %d finding(s): %+v", len(findings), findings)
	}
	if findings[0].Command != "show phantom ghost" {
		t.Errorf("the finding names %q, and only \"show phantom ghost\" is registration-only", findings[0].Command)
	}
	if !strings.Contains(findings[0].Reason, "never declared to Stage 1") {
		t.Errorf("the finding reads %q, and it must say the runner never declares it", findings[0].Reason)
	}
}

// VALIDATES: the gate REFUSES a package it cannot pair rather than pooling both
// sides and answering that they agree.
// PREVENTS: two plugins in one package wired to each other's declaration
// functions. Pooled, each side holds the union of both plugins and the two
// agree; one by one, each plugin's registration carries the OTHER plugin's
// commands, so dropping either plugin takes the surviving one's catalog entries
// with it while the daemon still serves them.
func TestCheckRefusesAPackageItCannotPair(t *testing.T) {
	tree := writeFixture(t, "twinned", `
func init() {
	_ = registry.Register(registry.Registration{
		Name:      "twinned-alpha",
		RunEngine: runAlpha,
		Commands:  betaDecls(),
	})
	_ = registry.Register(registry.Registration{
		Name:      "twinned-beta",
		RunEngine: runBeta,
		Commands:  alphaDecls(),
	})
}

func alphaDecls() []sdk.CommandDecl { return []sdk.CommandDecl{{Name: "show twinned alpha"}} }

func betaDecls() []sdk.CommandDecl { return []sdk.CommandDecl{{Name: "show twinned beta"}} }

func runAlpha(conn net.Conn) int {
	var p sdk.Plugin
	_ = p.Run(nil, sdk.Registration{Commands: alphaDecls()})
	return 0
}

func runBeta(conn net.Conn) int {
	var p sdk.Plugin
	_ = p.Run(nil, sdk.Registration{Commands: betaDecls()})
	return 0
}
`)

	findings, err := Check(tree, 0)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("a package the gate cannot pair answered %d finding(s): %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Reason, "pairs one runner to one registration") {
		t.Errorf("the finding reads %q, and it must say the gate cannot pair the package", findings[0].Reason)
	}
}
