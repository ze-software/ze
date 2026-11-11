// Design: docs/architecture/core-design.md -- native Go gates run through le
// Overview: hookcheck.go -- selftest orchestration and structured report
//
// Dispatcher fixtures are typed Go data. The bash rows are judged by the hook's
// own pretool-bash action (hookruntime.ProbeBash); the other tables by native
// rules over that data. Hook source contracts detect changes to the
// dispatchers themselves.
package aihooks

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/hookruntime"
)

const (
	bashRowsExpected          = 92
	writeEditRowsExpected     = 92
	weakeningRowsExpected     = 10
	postWriteEditRowsExpected = 24
)

// Re-sealed 2026-09-25 for spec-le-subject-first-command-tree, Phase 2:
// the bash fixtures name the subject-first commands (`ai rules update index`,
// `ai hooks unit`, `spec wip`, `test unit`, `repo changed`, `test qemu`). Same
// rows, same expected codes.
// Re-sealed 2026-09-25 for the same spec, Phase 3: the seven bash fixtures that
// ran or named the deleted standalone harness binary are gone with the
// rawZeTest guard and the harness alternative of expensivePipe. 82 rows are 75.
// 2026-10-18: the verify-gate rows type `./le verify current`, because the
// `mode full` spelling was removed; the rows and their verdicts are unchanged.
// Re-sealed 2026-10-26 for spec-le-cli-usability D-P4-reseal (owner approved):
// one bash row types `./le ai rules update index`, the Phase 4 spelling of
// `ai rules index-update`. Same rows, same expected codes.
// Re-sealed 2026-10-26 for the Phase 4 retirement of `doc check verify`
// (successor `doc check all`): nine bash rows type the new spelling. Same
// rows, same expected codes.
// Re-sealed 2026-11-11 for spec-agent-economy-1-call-shape phase 4: `go build
// ./...` expects 2 (umbrella D-4, AC-11), and five bash rows join for AC-7,
// AC-12 and AC-16. 75 rows are 80.
// Re-sealed 2026-11-11 for spec-agent-economy-2-scoped-test phase 6 (AC-7):
// three bash rows join for go vet, a refused raw one, a reader naming one and
// an admitted raw one. 80 rows are 83.
// Re-derived 2026-11-11 for spec-agent-economy-6-shell-parser (phases 2-5).
// Phase 5: nine bash rows join, five reader loops whose quoted pattern names a
// guarded command (0, looser) and four heavy or piped expensive commands
// inside a construct (2, stricter); AC-36 (02692d2ed8) had already moved two
// rows on a judge's approval. 83 rows are 92.
var parityCatalogDigest = [sha256.Size]byte{
	0x96, 0x06, 0xcf, 0x4c, 0x0a, 0x8b, 0x74, 0xcd,
	0x70, 0x62, 0x7a, 0xef, 0x9b, 0x7d, 0x31, 0x91,
	0xef, 0x1a, 0x07, 0x06, 0xd5, 0x2d, 0x35, 0x07,
	0xf2, 0x05, 0x0e, 0x37, 0x8c, 0xff, 0x57, 0xcd,
}

// bashGoldenProject is the placeholder a bash row writes for the checkout
// root. The judge puts the probe tree's root in its place, so a row such as
// `echo probe > @PROJECT@/tmp/notes.txt` names that tree's own tmp/.
const bashGoldenProject = "@PROJECT@"

// bashGoldenTree is the checkout the bash rows are judged over: a module with
// one main package, the package two rows test and vet, and a tmp/ directory.
// The build guard reads the tree (the module path, which directory is a main
// package), and the scratch guard resolves tmp/, so the rows need a tree that
// holds what they name. A throwaway tree also keeps the action's writers off
// the session's checkout.
var bashGoldenTree = map[string]string{
	"go.mod":                    "module github.com/ze-software/ze\n\ngo 1.24\n",
	"cmd/ze/main.go":            "package main\n\nfunc main() {}\n",
	"internal/core/env/env.go":  "package env\n",
	"tmp/session/.probe-anchor": "",
}

// parityJudge answers one row's verdict over the probe tree: the exit code,
// what the producer wrote, or why it could not answer.
type parityJudge func(tree, row string) (code int, output string, err error)

type parityFixture struct {
	name         string
	expectedCode int
}

type parityTable struct {
	key      string
	name     string
	expected int
	judge    parityJudge
	fixtures []parityFixture
}

func runParity(_ string) ([]Result, Population, []GoldenCase) {
	results := make([]Result, 0, len(parityCatalog))
	golden := make([]GoldenCase, 0,
		bashRowsExpected+writeEditRowsExpected+weakeningRowsExpected+postWriteEditRowsExpected)
	population := Population{}
	currentDigest := parityDigest(parityCatalog[:])
	catalogOK := currentDigest == parityCatalogDigest

	tree, treeErr := probeTree("bash-golden", bashGoldenTree)
	if tree != "" {
		defer func() { _ = os.RemoveAll(tree) }()
	}

	for index, table := range parityCatalog {
		result := Result{Name: table.name, Passed: catalogOK}
		if !catalogOK {
			result.Code = 2
			var tb textbuf.Buffer
			result.Message = tb.Str("typed dispatcher fixture content changed: got ").
				Str(hex.EncodeToString(currentDigest[:])).Str(", want ").
				Str(hex.EncodeToString(parityCatalogDigest[:])).String()
		} else if len(table.fixtures) != table.expected {
			result.Passed = false
			result.Code = 2
			var tb textbuf.Buffer
			result.Message = tb.Str("population is ").Int(int64(len(table.fixtures))).
				Str(" rows, want ").Int(int64(table.expected)).String()
		}
		for _, fixture := range table.fixtures {
			got, output, err := judgeRow(table, tree, treeErr, fixture.name)
			if err != nil {
				// A row the producer could not judge has no verdict to record,
				// so it joins no golden case and the table fails on it.
				if result.Passed {
					result.Passed = false
					result.Code = 2
					result.Message = fixture.name + ": " + err.Error()
				}
				continue
			}
			golden = append(golden, GoldenCase{
				Table:        table.key,
				Name:         fixture.name,
				ExpectedCode: fixture.expectedCode,
				NativeCode:   got,
				Passed:       got == fixture.expectedCode,
			})
			if got == fixture.expectedCode || !result.Passed {
				continue
			}
			result.Passed = false
			result.Code = 1
			var tb textbuf.Buffer
			tb.Str(fixture.name).Str(": native code ").Int(int64(got)).
				Str(", expected code ").Int(int64(fixture.expectedCode))
			if output != "" {
				tb.Str("\n").Str(output)
			}
			result.Message = tb.String()
		}
		results = append(results, result)
		switch index {
		case 0:
			population.Bash = len(table.fixtures)
		case 1:
			population.WriteEdit = len(table.fixtures)
		case 2:
			population.Weakening = len(table.fixtures)
		case 3:
			population.PostWriteEdit = len(table.fixtures)
		}
	}
	return results, population, golden
}

// judgeRow asks the table's judge for one row over the probe tree. A tree that
// could not be built answers its error for every row, so no row reads as
// judged over a tree that does not exist.
func judgeRow(table parityTable, tree string, treeErr error, row string) (int, string, error) {
	if treeErr != nil {
		return 0, "", fmt.Errorf("build the probe tree: %w", treeErr)
	}
	return table.judge(tree, row)
}

func parityDigest(tables []parityTable) [sha256.Size]byte {
	var tb textbuf.Buffer
	for _, table := range tables {
		for _, fixture := range table.fixtures {
			tb.Str(table.key).Byte(0).Str(fixture.name).Byte(0).
				Int(int64(fixture.expectedCode)).Byte(0)
		}
	}
	return sha256.Sum256([]byte(tb.String()))
}

// bashHookVerdict judges one bash row by the hook itself: the whole
// pretool-bash action, over a main-thread payload, with the probe tree as the
// checkout (hookruntime.ProbeBash). The golden then goes red when the hook's
// verdict moves, which the hand model it replaces never did: the `go build
// ./...` row stayed at 0 after the build guard began refusing it.
func bashHookVerdict(tree, row string) (int, string, error) {
	return hookruntime.ProbeBash(strings.ReplaceAll(row, bashGoldenProject, tree), tree)
}

// handModel adapts a table judged by a hand-written model of its dispatcher,
// which reads no tree and writes nothing.
func handModel(code func(string) int) parityJudge {
	return func(_, row string) (int, string, error) {
		return code(row), "", nil
	}
}

func writeEditCode(name string) int {
	_, label, ok := strings.Cut(name, "|")
	if !ok {
		return 2
	}
	if label == "nested claude not generated" || label == "observer ok ci" ||
		strings.HasPrefix(label, "scratch dated") || strings.HasPrefix(label, "scratch producer") ||
		strings.HasPrefix(label, "scratch shared") {
		return 0
	}
	if label == "observer sysexit ci" {
		return 1
	}
	if label == "md naming bad" {
		if strings.HasPrefix(name, "Write|") {
			return 1
		}
		return 0
	}
	if label == "claude plans" && strings.HasPrefix(name, "Edit|") {
		return 0
	}
	return 2
}

func weakeningCode(name string) int {
	switch name {
	case "skip added", "skip added with a retired relax token", "commented out assertion",
		"build tag ignore added", "delete test func", "write overwrite weakens":
		return 2
	default:
		return 0
	}
}

func postWriteEditCode(name string) int {
	_, label, ok := strings.Cut(name, "|")
	if !ok {
		return 1
	}
	switch label {
	case "big >1000", "journal malformed row", "md deferral":
		return 1
	default:
		return 0
	}
}
