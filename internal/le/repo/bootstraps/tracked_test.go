// Contract for the tracked-import port.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11. The check still asks the committed
// tree the question that the Python code asked. It uses failure modes that can
// occur in a compiled binary.
// PREVENTS: a tool that is committed and compiles but is absent from le. The
// Python failure was an area that would not IMPORT. A Go area cannot fail in
// that way. The corresponding failure is an import that nobody added.

package repobootstraps

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

const registerGo = `package le

import (
	"os"
	"github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/gamma"
	_ "github.com/ze-software/ze/internal/le/alpha"
	_ "github.com/ze-software/ze/internal/le/beta"
)
`

func TestTheToolImportsAreRead(t *testing.T) {
	got, err := toolImports([]byte(registerGo))
	if err != nil {
		t.Fatalf("ToolImports: %v", err)
	}
	if !slices.Equal(got.blank, []string{"alpha", "beta"}) {
		t.Errorf("blank imports = %v", got.blank)
	}
	if !slices.Equal(got.named, []string{"gamma", "le/root"}) {
		t.Errorf("named imports = %v", got.named)
	}
}

// TestAnImportThatIsNotAToolIsRefused keeps the composition root to its one
// job. A blank import of anything but an le tool package means the file has
// stopped being a list of tools.
func TestAnImportThatIsNotAToolIsRefused(t *testing.T) {
	src := "package main\n\nimport _ \"github.com/ze-software/ze/internal/component/bgp\"\n"
	if _, err := toolImports([]byte(src)); err == nil {
		t.Error("a product import in the composition root was accepted")
	}
}

// TestNamedSupportImportsAreIgnored separates the root's own support
// packages from the composition population: a named le import that registers
// nothing (le/root) is no tool, and a named import outside le (os) is ignored.
func TestNamedSupportImportsAreIgnored(t *testing.T) {
	got, err := toolImports([]byte(registerGo))
	if err != nil {
		t.Fatalf("ToolImports: %v", err)
	}
	if wired := got.wired([]string{"alpha", "beta"}); !slices.Equal(wired, []string{"alpha", "beta"}) {
		t.Errorf("wired included a named support import: %v", wired)
	}
}

// TestANamedImportOfARegisteringPackageIsWired is the case that reddened HEAD
// on 2026-10-25: register.go imported scratch by name for its cache helpers.
// A named import runs init() as a blank one does, so scratch's command is in
// le, and the gate MUST count it as wired rather than report it unimported.
func TestANamedImportOfARegisteringPackageIsWired(t *testing.T) {
	got, err := toolImports([]byte(registerGo))
	if err != nil {
		t.Fatalf("ToolImports: %v", err)
	}
	registering := []string{"alpha", "beta", "gamma"}
	wired := got.wired(registering)
	if !slices.Equal(wired, []string{"alpha", "beta", "gamma"}) {
		t.Fatalf("wired = %v, want alpha beta gamma", wired)
	}
	if broken := Compare(wired, registering, nil, []string{"a", "b", "g"}); len(broken) != 0 {
		t.Errorf("a named import of a registering package was reported broken: %v", broken)
	}
}

func TestUnparsableSourceIsAnError(t *testing.T) {
	if _, err := toolImports([]byte("this is not Go")); err == nil {
		t.Error("unparsable source answered an import list")
	}
}

// --- Reading the help page --------------------------------------------------

const helpPage = `le - the Ze repository and development entry point

Usage:
  le <command> [options] [| json | yaml | table]

Commands:
  fuzz                       Go fuzzing: every ` + "`func Fuzz`" + ` under internal/
  lint                       lint and type-check the Python half of the tree
  tracked                    does le still work from what git holds

`

func TestTheCommandNamesAreReadFromTheHelpPage(t *testing.T) {
	got := commandNames(helpPage)
	if !slices.Equal(got, []string{"fuzz", "lint", "tracked"}) {
		t.Errorf("CommandNames = %v", got)
	}
}

// TestTheUsageLineIsNotACommand is the trap in that page: it is indented the
// same way and it sits under a heading of its own.
func TestTheUsageLineIsNotACommand(t *testing.T) {
	for _, name := range commandNames(helpPage) {
		if strings.HasPrefix(name, "le") && name != "lint" {
			t.Errorf("the usage line was read as a command: %q", name)
		}
	}
}

func TestAPageWithNoCommandsSectionAnswersNothing(t *testing.T) {
	if got := commandNames("le - a program\n\nUsage:\n  le <command>\n"); len(got) != 0 {
		t.Errorf("CommandNames = %v, want none", got)
	}
}

// groupedHelpPage is the page le prints once its commands are grouped. A
// tracked run gets the page of the commit it judges, so both shapes are read
// by the same parser.
const groupedHelpPage = `le - the Ze repository and development entry point

Usage:
  le <command> [options] [| json | yaml | table]

Workflow (you type these while working):
  commit   prepare explicit commits
  session  manage this development session's state

Gates (judge the tree, answer a verdict):
  tier     module-tier placement
  tracked  does le still work from what git holds

Reports (read the tree, gate nothing):
  inventory  what ze is made of
`

// treeHelpPage is the page le prints once its root page is laid out by tree:
// one block of single-word commands, one per namespace, and a description
// wrapped onto a continuation line that names no command.
const treeHelpPage = `le - the Ze repository and development entry point

Usage:
  le <command> [options] [| json | yaml | table]

Commands:
  inventory        [report] what ze is made of, a description long enough
                   to wrap onto a second line
spec:
  spec claim       [workflow] claim a spec
  spec current     [workflow] the spec this session claimed
`

func TestEverySectionOfATreePageListsCommandsAndNoContinuation(t *testing.T) {
	got := commandNames(treeHelpPage)
	want := []string{"inventory", "spec", "spec"}
	if !slices.Equal(got, want) {
		t.Errorf("CommandNames = %v, want %v", got, want)
	}
}

func TestEverySectionOfAGroupedPageListsCommands(t *testing.T) {
	got := commandNames(groupedHelpPage)
	want := []string{"commit", "session", "tier", "tracked", "inventory"}
	if !slices.Equal(got, want) {
		t.Errorf("CommandNames = %v, want %v", got, want)
	}
}

// TestTheUsageLineIsNotACommandOnAGroupedPage is the same trap as on the flat
// page. A blank line no longer ends the reading, so the usage block is skipped
// by its heading alone.
func TestTheUsageLineIsNotACommandOnAGroupedPage(t *testing.T) {
	for _, name := range commandNames(groupedHelpPage) {
		if strings.HasPrefix(name, "le") {
			t.Errorf("the usage line was read as a command: %q", name)
		}
	}
}

// --- The comparison ---------------------------------------------------------

func TestAgreementIsClean(t *testing.T) {
	broken := Compare([]string{"alpha", "beta"}, []string{"alpha", "beta"}, nil, []string{"alpha", "beta"})
	if len(broken) != 0 {
		t.Errorf("a tree that agrees reported %v", broken)
	}
}

// TestAPackageThatRegistersAndIsNotImportedIsBroken is the Go-shaped failure
// this gate exists for: the tool is committed, it compiles, and le does not
// carry it.
func TestAPackageThatRegistersAndIsNotImportedIsBroken(t *testing.T) {
	broken := Compare([]string{"alpha"}, []string{"alpha", "beta"}, nil, []string{"alpha"})
	if len(broken) != 1 {
		t.Fatalf("Compare = %v, want one finding", broken)
	}
	if broken[0].Package != "beta" {
		t.Errorf("the finding names %q, want beta", broken[0].Package)
	}
}

// TestAnImportOfAPackageThatRegistersNothingIsBroken is the mirror: an import
// that adds no command is a tool that was wired and never written.
func TestAnImportOfAPackageThatRegistersNothingIsBroken(t *testing.T) {
	broken := Compare([]string{"alpha", "ghost"}, []string{"alpha"}, nil, []string{"alpha"})
	if len(broken) != 1 {
		t.Fatalf("Compare = %v, want one finding", broken)
	}
	if broken[0].Package != "ghost" {
		t.Errorf("the finding names %q, want ghost", broken[0].Package)
	}
}

// TestACommandTheBinaryDoesNotOfferIsBroken is the runtime half. The static
// halves can agree while the built binary offers fewer commands, which is what
// an init() that registered under another name looks like.
func TestACommandTheBinaryDoesNotOfferIsBroken(t *testing.T) {
	broken := Compare([]string{"alpha", "beta"}, []string{"alpha", "beta"}, nil, []string{"alpha"})
	if len(broken) != 1 {
		t.Fatalf("Compare = %v, want one finding", broken)
	}
	if !strings.Contains(broken[0].Detail, "command") {
		t.Errorf("the finding reads %q", broken[0].Detail)
	}
}

// TestAnEmptyCommittedTreeIsBroken is the vacuity guard. A registry that lists
// nothing would make every assertion above answer clean.
func TestAnEmptyCommittedTreeIsBroken(t *testing.T) {
	if broken := Compare(nil, nil, nil, nil); len(broken) == 0 {
		t.Error("a commit carrying no tool at all was reported clean")
	}
}

// TestANamespacePackageIsWiredAndOffersNoRow is the case that reddened HEAD at
// 145da76fb9. A namespace package (internal/le/ai) holds only its summary: the
// root page prints it as the heading "ai: <short>" over its members' rows, so
// it offers no row of its own. Counting it as a wired row made the page look
// short by one row for every namespace.
func TestANamespacePackageIsWiredAndOffersNoRow(t *testing.T) {
	broken := Compare([]string{"ai", "ai/rules", "commit"}, []string{"ai/rules", "commit"}, []string{"ai"},
		[]string{"commit", "ai"})
	if len(broken) != 0 {
		t.Errorf("a wired namespace package was reported broken: %v", broken)
	}
}

// TestANamespaceTheRootDoesNotImportIsBroken keeps the namespace population
// checked: a namespace package nobody imports leaves its members with no
// summary, which le refuses at startup.
func TestANamespaceTheRootDoesNotImportIsBroken(t *testing.T) {
	broken := Compare([]string{"ai/rules"}, []string{"ai/rules"}, []string{"ai"}, []string{"ai"})
	if len(broken) != 1 {
		t.Fatalf("Compare = %v, want one finding", broken)
	}
	if broken[0].Package != "ai" {
		t.Errorf("the finding names %q, want ai", broken[0].Package)
	}
	if !strings.Contains(broken[0].Detail, "namespace") {
		t.Errorf("the finding reads %q", broken[0].Detail)
	}
}

// fileDeclaresCases pins which leroot calls a source file makes. The calls are
// parsed, so the spelling inside a string literal or a comment calls nothing:
// le/root's own panic text names `leroot.RegisterNamespace(` to tell the
// author which call to add, and that file registers nothing.
var fileDeclaresCases = []struct {
	name string
	src  string
	want declaration
}{
	{"a tool", `package a
import leroot "github.com/ze-software/ze/internal/le/le/root"
func init() { leroot.Register("a", 0, nil, nil) }
`, declaration{tool: true}},
	{"a namespace", `package ai
import leroot "github.com/ze-software/ze/internal/le/le/root"
func init() {
	leroot.RegisterNamespace("ai", "agent tooling", "Agent tooling.")
}
`, declaration{namespace: true}},
	{"an unrenamed import", `package a
import "github.com/ze-software/ze/internal/le/le/root"
func init() { leroot.Register("a", 0, nil, nil) }
`, declaration{tool: true}},
	{"a renamed import", `package a
import root "github.com/ze-software/ze/internal/le/le/root"
func init() { root.RegisterNamespace("a", "s", "d") }
`, declaration{namespace: true}},
	{"the spelling in a string and a comment", `package leroot
// Call leroot.Register( from init.
func missing(name string) {
	panic("BUG: call leroot.RegisterNamespace(\"" + name + "\")")
}
`, declaration{}},
	{"another package's Register", `package a
import "github.com/ze-software/ze/internal/core/registry"
func init() { registry.Register("a") }
`, declaration{}},
	{"another leroot call", `package a
import leroot "github.com/ze-software/ze/internal/le/le/root"
func init() { leroot.RegisterActions("a", nil) }
`, declaration{}},
}

func TestFileDeclaresReadsTheCallsNotTheText(t *testing.T) {
	for _, tc := range fileDeclaresCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fileDeclares("case.go", []byte(tc.src))
			if err != nil {
				t.Fatalf("fileDeclares: %v", err)
			}
			if got != tc.want {
				t.Errorf("fileDeclares = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestADotImportOfLerootIsRefused: a dot import makes the calls unqualified,
// so the scan could not see them and would answer "no call" for a package that
// registers. It answers an error instead.
func TestADotImportOfLerootIsRefused(t *testing.T) {
	src := `package a
import . "github.com/ze-software/ze/internal/le/le/root"
func init() { Register("a", 0, nil, nil) }
`
	if _, err := fileDeclares("case.go", []byte(src)); err == nil {
		t.Error("a dot import of leroot answered a declaration")
	}
}

// --- The rendering ----------------------------------------------------------

func TestACleanVerdictNamesTheCountAndTheRevision(t *testing.T) {
	text := Verdict{Rev: "HEAD", Areas: 34, OK: true}.Text()
	for _, want := range []string{"34 area(s) load from HEAD", "==> loading every le area from HEAD"} {
		if !strings.Contains(text, want) {
			t.Errorf("the page does not say %q:\n%s", want, text)
		}
	}
}

func TestABrokenVerdictNamesEveryFinding(t *testing.T) {
	text := Verdict{
		Rev:    "HEAD",
		Areas:  33,
		Broken: []Broken{{Package: "beta", Detail: "registers a command and internal/le/register.go does not import it"}},
	}.Text()
	for _, want := range []string{"BROKEN  beta", "1 of 33 area(s) do not load from HEAD"} {
		if !strings.Contains(text, want) {
			t.Errorf("the page does not say %q:\n%s", want, text)
		}
	}
}

// --- The whole run, over this checkout ---------------------------------------

// TestACommitWithNoCompositionRootCannotBeJudged uses the real extraction path
// with a commit that predates the development-tool composition root. It
// verifies the third exit code.
//
// Extraction stops when the file is missing. Therefore, this test performs an
// archive but does not perform a build.
func TestACommitWithNoCompositionRootCannotBeJudged(t *testing.T) {
	root := repoRoot(t)
	// This commit predates both the former and current composition roots.
	const beforeLe = "12ce0438a3d7c9f1953dd0599f62d67a4bf77716~1"

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	verdict, code, err := Run(ctx, root, beforeLe)
	if code != 2 {
		t.Errorf("a commit with no composition root exited %d, want 2 (could not judge)", code)
	}
	if err == nil {
		t.Error("a commit with no composition root reported no reason")
	}
	if verdict.OK {
		t.Error("a commit that could not be judged was reported OK")
	}
}

// TestTheCheckoutsOwnHeadIsClean complements the preceding non-vacuity test.
// The preceding test verifies a refusal. This test proves that the same code
// path returns a clean verdict for a valid commit. Without this test, a Run that
// refused every commit would pass.
func TestTheCheckoutsOwnHeadIsClean(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the cmd/ze le personality out of a git archive")
	}
	root := repoRoot(t)

	ctx, cancel := context.WithTimeout(context.Background(), DefaultDeadline)
	defer cancel()

	verdict, code, err := Run(ctx, root, "HEAD")
	if err != nil {
		t.Fatalf("Run over HEAD: %v", err)
	}
	if code != 0 {
		t.Fatalf("HEAD is not clean (exit %d):\n%s", code, verdict.Text())
	}
	if verdict.Areas == 0 || len(verdict.Commands) == 0 {
		t.Errorf("a clean verdict counted %d areas and %d commands", verdict.Areas, len(verdict.Commands))
	}
}

// repoRoot answers the checkout these two cases judge.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Skipf("no checkout: %v", err)
	}
	return root
}
