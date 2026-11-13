// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
// Related: runtime.go -- nativeHookActions, the checks whose refusals the table records
// Related: bash_test.go -- guardVerdict, one helper the corpus is collected through
// Related: runtime_test.go -- runHook, the other
package hookruntime

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// verdictsUpdate makes TestGuardVerdictsOfHeadStand append, with the verdicts
// the hook gives now, a row for every corpus line the table lacks. It keeps
// every recorded row exactly as it stands and removes none, so a refusal once
// recorded leaves the table only by a hand edit a reviewer sees in the diff
// (mergeVerdictRows, TestVerdictTableUpdateKeepsEveryRecordedRow).
//
//nolint:gochecknoglobals // a test flag is package state by construction
var verdictsUpdate = flag.Bool("update", false,
	"append to "+verdictsTablePath+" a row for each corpus line it lacks, with the verdicts the hook gives now")

// verdictsTablePath is the recorded table of AC-11 of
// spec-agent-economy-6-shell-parser (closed): each
// line of the corpus, for each caller, with the checks that refused it on the
// tree it was recorded from.
const verdictsTablePath = "testdata/guard-verdicts-head.jsonl"

// The callers a row is judged as. A subagent row is judged without the
// identity line: the line is what the agent typed, and the hook adds nothing.
const (
	callerMain     = "main"
	callerSubagent = "subagent"
	// callerFork is history. The three rows that name it were recorded when
	// the hook read a fork from CLAUDE_CODE_FORK_SUBAGENT and a payload with
	// no agent id. A fork's payload names an agent, so the rows are judged
	// as a subagent's (refusingChecks, rowIdentity) and keep their recorded
	// refusals, and no new fork row is recorded: collectCorpus yields none.
	callerFork = "fork"
)

// verdictRow is one line of the recorded table.
type verdictRow struct {
	Line   string `json:"line"`
	Caller string `json:"caller"`
	// RefusedBy names the checks that refused the line when it was recorded,
	// in the order nativeHookActions runs them. It is never null.
	RefusedBy []string `json:"refused-by"`
	// ReaderLine is set by hand, only on a row the parser replacement loosens,
	// and Proof then names the package test that proves its commands are
	// readers. Such a row may lose a recorded refusal; no other row may.
	ReaderLine bool   `json:"reader-line"`
	Proof      string `json:"proof,omitempty"`
	// Waives names, on a reader-line row only, each recorded refusal the row
	// may lose: a check of RefusedBy that a reader line lifts
	// (readerLiftableChecks). A recorded refusal it does not name still
	// holds, so a row that is a reader line for one check alone (the
	// identity check) keeps every other guard's refusal.
	Waives []string `json:"waives,omitempty"`
}

// refusalProofs are tests of readerLineProofs that assert a refusal: judging
// a line there proves nothing about it being a reader line, so no reader-line
// row may name one as its proof.
//
//nolint:gochecknoglobals // the declared refusal tests
var refusalProofs = map[string]bool{
	"TestReaderLineRefusesWhatCanRunAnythingElse": true,
	"TestAnotherIdentityIsJudgedByTheNode":        true,
	"TestRefusalAdvisesTheParsedCommand":          true,
	"TestSetOptionsThatChangeNoVariablePass":      true,
}

// rowIdentity is the identity line a row's caller types in front of its
// command: the agent's own for a subagent row and for a fork row, which is a
// subagent's, and none for the main thread.
func rowIdentity(row verdictRow) string {
	switch row.Caller {
	case callerSubagent, callerFork:
		return identityLineOfAgent
	}
	return ""
}

// readerRowFaults judges a reader-line row's own claims: it waives at least
// one check, each a recorded refusal a reader line lifts; its proof is no
// test that asserts a refusal; and the predicate answers reader line for it
// now, with the identity its caller types.
func readerRowFaults(t verdictReporter, row verdictRow, liftable map[string]bool) {
	t.Helper()
	if len(row.Waives) == 0 {
		t.Errorf("%q (%s) is marked a reader line and waives no check: name each recorded refusal it loses", row.Line, row.Caller)
	}
	for _, check := range row.Waives {
		if !liftable[check] {
			t.Errorf("%q (%s) waives %s, which no reader line lifts", row.Line, row.Caller, check)
		}
		if !slices.Contains(row.RefusedBy, check) {
			t.Errorf("%q (%s) waives %s, which it never recorded", row.Line, row.Caller, check)
		}
	}
	if refusalProofs[row.Proof] {
		t.Errorf("%q (%s) names the proof %q, which asserts a refusal and proves no reader line", row.Line, row.Caller, row.Proof)
	}
	if _, outside := judgeReaderLine(row.Line, parseCommandLine(row.Line), rowIdentity(row)); outside != "" {
		t.Errorf("%q (%s) is marked a reader line and is no reader line now: it holds %s", row.Line, row.Caller, outside)
	}
}

type verdictKey struct {
	line   string
	caller string
}

func (row verdictRow) key() verdictKey {
	return verdictKey{line: row.Line, caller: row.Caller}
}

// corpusRecorder collects the command lines the corpus source tests judge
// while collectCorpus runs them. Its lines map is nil in every other run, so
// the two helpers that feed it (guardVerdict, runHook) record nothing there.
//
//nolint:gochecknoglobals // test-only collector, active inside collectCorpus alone
var corpusRecorder struct {
	sync.Mutex
	// lines is the set of judged lines.
	lines map[string]bool

	// current is the source test collectCorpus is running, and judgedBy maps
	// each judged line to the tests that judged it: a reader-line row's proof
	// must be one of them.
	current  string
	judgedBy map[string]map[string]bool
	faults   []string
}

// recordCorpusLine adds one judged line to the corpus while collectCorpus runs.
func recordCorpusLine(line string) {
	corpusRecorder.Lock()
	defer corpusRecorder.Unlock()
	if corpusRecorder.lines == nil {
		return
	}
	if line == "" {
		return
	}
	corpusRecorder.lines[line] = true
	if corpusRecorder.judgedBy[line] == nil {
		corpusRecorder.judgedBy[line] = map[string]bool{}
	}
	corpusRecorder.judgedBy[line][corpusRecorder.current] = true
}

// recordCorpusPayload adds the command of one pretool-bash payload runHook
// sends. It decodes the bytes Run reads, so every payload shape a test builds
// is read the way the hook reads it. A payload that does not decode is a fault
// collectCorpus reports, never a line silently left out.
func recordCorpusPayload(kind string, body []byte) {
	if kind != "pretool-bash" {
		return
	}
	var payload Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		corpusRecorder.Lock()
		defer corpusRecorder.Unlock()
		if corpusRecorder.lines != nil {
			corpusRecorder.faults = append(corpusRecorder.faults, "a pretool-bash payload does not decode: "+err.Error())
		}
		return
	}
	recordCorpusLine(stringInput(payload.ToolInput, "command"))
}

// corpusSourceTests are the tests whose judged lines the corpus holds (the
// spec's "Recorded verdicts"): six guard tests and every TestTestDeletion test
// of bash_test.go, and every test of session_identity_test.go.
// TestGuardVerdictsOfHeadStand holds the deletion tests and the identity tests
// complete against their files (holdSourcesComplete). A test listed here that
// judges nothing through guardVerdict or runHook adds no line and costs one run.
//
//nolint:gochecknoglobals // the declared corpus sources
var corpusSourceTests = []func(*testing.T){
	TestTestDeletionFixNamesThePathAndTheRoute,
	TestTestDeletionAsksOnlyForATestPath,
	TestTestDeletionJudgesOnlyTheArgumentsOfAPlainRm,
	TestTestDeletionRefusalNamesTheNextAction,
	TestGuardsKeepTheTextVerdictOfHead,
	TestGuardsKeepTheTextVerdictBesideAReaderItsJudgeRefuses,
	TestGuardsJudgeALineThatCanRunAStringByItsText,
	TestGuardsKeepTheTextVerdictOutsideTheReaderLine,
	TestGuardsKeepTheTextVerdictWhereTheSplitMovesAWord,
	TestGuardsSeeACommandHandedToAShell,
	TestSubagentReaderCommandPassesUnchanged,
	TestSubagentCommandOutsideTheReaderSetNeedsTheIdentityLine,
	TestSubagentCommandCarryingItsIdentityLinePasses,
	TestSubagentIdentityLineOfAnotherAgentIsRefused,
	TestMainThreadCommandNeedsNoIdentityLine,
	TestCallerPredicateAgreesWithIdentityCheck,
	TestSubagentUnparsableCommandNeedsTheIdentityLine,
	TestSubagentReaderOutsideItsJudgeNeedsTheIdentityLine,
	TestSubagentSplitterFaultLinesAreJudgedFromTheParse,
	TestReaderLineNamesEveryConstructOutsideIt,
	TestReaderLineKeepsABackslashBashKeepsInDoubleQuotes,
	TestSubagentRedirectionBeforeAWordNeedsTheIdentityLine,
	TestSubagentEmptyQuotedWordNeedsTheIdentityLine,
	TestSubagentQuotedBackslashTargetIsAFile,
	TestSubagentChangingTheEnvironmentAfterItsLineIsRefused,
	TestSubagentNamingAnIdentityVariableAfterItsLineIsRefused,
	TestEnvironmentToolIsFoundBehindEveryWrapper,
	TestQuotedArgumentAfterTheLineChangesNoVerdict,
}

// readerLineProofs are the tests a reader-line row may name as its proof,
// beside corpusSourceTests. collectCorpus runs them with the recorder on, and
// a row's proof holds only when the test it names judged that exact line,
// through guardVerdict or runHook, and passed. The parser replacement adds
// each test it names in a reader-line row here; none exists before it.
//
//nolint:gochecknoglobals // the declared reader-line proofs
var readerLineProofs = []func(*testing.T){
	TestReaderLineReachesThroughConstructs,
	TestReaderLineRefusesWhatCanRunAnythingElse,
	TestIdentityLineBeforeAReaderLineStillLifts,
	TestSubagentReaderLoopPassesUnchanged,
	TestRefusalAdvisesTheParsedCommand,
	TestRootBuildReadsTheBuildFromTheTree,
	TestSystemTmpJudgesEveryWordOfAReaderLine,
	TestTestDeletionReadsThePlainRmFromTheTree,
	TestAnotherIdentityIsJudgedByTheNode,
	TestSetOptionsThatChangeNoVariablePass,
	TestScratchGuardLiftsOnAReaderLine,
}

// verdictsTableRowsMin is the row count of the table after its last update. A
// table holding fewer rows lost recorded refusals: deleting the file and
// re-running -update records again from the tree in hand, which keeps no
// refusal HEAD gave. Raise it when an update appends rows; never lower it.
const verdictsTableRowsMin = 1761

// readerSetJudgment is the one function that judges whether a command is a
// reader (readers.go). A check whose code reaches it can pass a line because
// its commands are readers, so only such a check may lose a recorded refusal
// on a row marked a reader line (readerLiftableChecks).
const readerSetJudgment = "readerCommand"

// specConstructLines are the construct rows the spec names: its parity table
// and the inputs of its acceptance criteria. The after-line `set` rows of AC-31
// are absent on purpose: the spec records them after their fix, which
// loosens them, and they name that criterion.
//
//nolint:gochecknoglobals // the spec's named rows
var specConstructLines = []string{
	// Parity table, and AC-5, AC-9, AC-10.
	`for f in a b; do grep -n "x\|go vet ./y" "$f"; done`,
	`while read -r f; do grep -c 'x;go test ./y' "$f"; done < list`,
	`for f in a b; do grep -c 'x; go test ./y | tail -1' "$f"; done`,
	`for f in a b; do grep -n 'go build ./cmd/ze' "$f"; done`,
	`for f in a b; do grep -n 'cat /tmp/x' "$f"; done`,
	`for p in a b; do go test ./internal/$p; done`,
	`if true; then go vet ./internal/x; fi`,
	`( go test ./internal/x )`,
	`for p in a b; do ./le test unit all | tail -3; done`,
	// AC-6.
	`if [ -f a ]; then cat a; fi`,
	`while read -r f; do cat "$f"; done < list`,
	`until true; do cat a; done`,
	`case x in a) cat a;; esac`,
	`[[ -f "$f" ]] && cat "$f"`,
	`{ cat a; cat b; }`,
	`( cat a )`,
	`! grep -q x a`,
	`time grep -c x a`,
	`echo "$(cat a)"`,
	"cat <<EOF\nx\nEOF",
	`diff <(sort a) <(sort b)`,
	// AC-7.
	`grep -n '^##' f`,
	`grep -n '{x}' f`,
	`grep -n '!x' f`,
	`grep -n x\ y f`,
	`grep -n 'é' f`,
	`grep -n '' f`,
	`grep -n "foo$" f`,
	`< f grep -n x`,
	`[ -f x ] 2>/dev/null && cat x`,
	`sort f | uniq -c 2>/dev/null`,
	`grep -n $'x' f`,
	`grep -n $"x" f`,
	// AC-8.
	`f() { cat a; }; f`,
	`A=1; cat a`,
	`PATH=/x; cat a`,
	`export A=1; cat a`,
	`declare A=1; cat a`,
	`let a=1; cat a`,
	`echo $((1+2))`,
	`echo "${a[0]}"`,
	`echo "${a:1:2}"`,
	`cat a &`,
	`coproc cat a`,
	`LC_ALL=C sort f`,
	`PATH=. cat f`,
	`[[ a -eq b ]]`,
	`read -n 1 x`,
	// AC-9.
	`for p in a b; do go test ./$p; done`,
	`if true; then go vet ./x; fi`,
	`( go test ./x )`,
	`! golangci-lint run`,
	`x=$(go test ./x)`,
	`./le job run label unit-pkg quiet command go test ./x`,
	`ZE_ADMIT_RAW="reason" go test ./x`,
	// AC-10.
	`for p in a b; do grep -c x a | tail -3; done`,
	// AC-12.
	`grep -n "x f`,
	"cat <<EOF\nx",
	`fi`,
	// AC-14.
	identityLineOfAgent + `for f in a b; do grep -n "x\|go vet ./y" "$f"; done`,
	identityLineOfAgent + `grep -n 'go build ./cmd/ze' f`,
	identityLineOfAgent + `grep -n 'cat /tmp/x' f`,
	// AC-15.
	`grep -n '#' f; go test -run 'TestA|TestB' ./x | tail -5`,
	`{ go test -run 'TestA|TestB' ./x | tail -5; }`,
	// AC-16.
	`cat a; go build -o bin/ze ./cmd/ze`,
	`2>/dev/null go build -o bin/ze ./cmd/ze`,
	`for f in a; do cat "$f"; done; go build -o bin/ze ./cmd/ze`,
	// AC-17.
	`for f in /tmp/a; do cat "$f"; done`,
	`cat <<< /tmp/a`,
	// AC-18.
	`rm -f scratch && grep x internal/a_test.go`,
	// AC-19.
	identityLineOfAgent + `export X=1`,
	identityLineOfAgent + `declare -x X`,
	identityLineOfAgent + `env X=1 ./le x`,
	identityLineOfAgent + `FOO=bar ./le x`,
	identityLineOfAgent + `cd x && ./le x`,
	identityLineOfAgent + `grep ZE_AGENT_ID f`,
	// AC-20.
	`go test -count=1 ./internal/x`,
	`golangci-lint run`,
	// AC-26.
	identityLineOfAgent + `time . ./env.sh; ./le x`,
	identityLineOfAgent + `if true; then . ./env.sh; fi; ./le x`,
	identityLineOfAgent + `command . x`,
	identityLineOfAgent + `builtin . x`,
	identityLineOfAgent + `timeout 5 ./le x`,
	identityLineOfAgent + `timeout 1.5s ./le x`,
	identityLineOfAgent + `nice -n 5 ./le x`,
	// AC-27.
	identityLineOfAgent + `source x`,
	identityLineOfAgent + `. x`,
	identityLineOfAgent + `set -a`,
	identityLineOfAgent + `eval x`,
	identityLineOfAgent + `typeset -x A`,
	identityLineOfAgent + `readonly A`,
	identityLineOfAgent + `local A`,
	identityLineOfAgent + `command export A=1`,
	identityLineOfAgent + `if true; then export A=1; fi`,
	identityLineOfAgent + `timeout 5 env -i le x`,
	identityLineOfAgent + `time . x`,
	// AC-29.
	identityLineOfAgent + `timeout -s KILL 60 env -i ./le x`,
	identityLineOfAgent + `/usr/bin/env -i ./le x`,
	identityLineOfAgent + `nice -n 5 /usr/bin/sudo le x`,
	identityLineOfAgent + `stdbuf -o L env -i le x`,
	identityLineOfAgent + `timeout -s KILL 60 ./le x`,
	// AC-33.
	`bash -o pipefail -c './le test unit all | tail -5'`,
	`bash -O extglob -c 'go test ./x'`,
	`timeout -s KILL 60 go test ./x`,
	`nice -n 5 go test ./x`,
	`sudo -u u go test ./x`,
	// AC-34.
	`go build -toolexec=x -o bin/y ./cmd/y`,
	`go build -overlay=o.json -o bin/y ./cmd/y`,
	`go build -modfile=m -o bin/y ./cmd/y`,
	`go build -toolexec x -o bin/y ./cmd/y`,
	`GOFLAGS=-toolexec=x go build -o bin/y ./cmd/y`,
	`go build -o bin/ze ./cmd/ze`,
	// AC-35.
	`cat ${v}/tmp`,
	`cat ${v}/tmp/x`,
	`ls ${HOME}/tmp`,
	`cat x/tmp/y`,
	`cat tmp/x`,
	// Phase 3's stricter rules (spec, phase 3 Decisions H-1 to H-6).
	identityLineOfAgent + `command time env -i ./le x`,
	identityLineOfAgent + `nice time env -i ./le x`,
	`cat foo$D/tmp/x`,
	"bash <<EOF\nrm a_test.go\nEOF",
	identityLineOfAgent + `n=x; read -r "$n" <<< y; ./le x`,
	identityLineOfAgent + `printf -v "$n" x; ./le x`,
	identityLineOfAgent + `mapfile a <<< x; ./le x`,
	identityLineOfAgent + `readarray a <<< x; ./le x`,
	identityLineOfAgent + `getopts a b; ./le x`,
	identityLineOfAgent + `trap 'x' EXIT; ./le x`,
	identityLineOfAgent + `enable -f lib.so x; ./le x`,
	identityLineOfAgent + `let a=1; ./le x`,
	identityLineOfAgent + `(( a = 1 )); ./le x`,
	identityLineOfAgent + `c=env; $c -i ./le x`,
	identityLineOfAgent + `setsid env -i ./le x`,
	identityLineOfAgent + `flock l.lock env -i ./le x`,
	identityLineOfAgent + `strace -o out env -i ./le x`,
	identityLineOfAgent + `ltrace -o out env -i ./le x`,
	identityLineOfAgent + `BASH_ENV=./x.sh ./le x`,
	identityLineOfAgent + `ENV=./x.sh ./le x`,
	identityLineOfAgent + `SHELLOPTS=x ./le x`,
	identityLineOfAgent + `BASHOPTS=x ./le x`,
	identityLineOfAgent + `PROMPT_COMMAND=x ./le x`,
	identityLineOfAgent + `BASH_FUNC_f=x ./le x`,
	`cat @(a|b)`,
	`a+=x`,
	`echo "${a?x}"`,
	`[[ -o posix ]]`,
	"cat x\x00y",
	`cat notes.md \`,
}

// TestGuardVerdictsOfHeadStand holds the pre-tool Bash hook to the verdicts
// recorded in testdata/guard-verdicts-head.jsonl (AC-11 of
// spec-agent-economy-6-shell-parser).
//
// VALIDATES: every row of the corpus is in the table, for each caller; and
// every check that refused a row when it was recorded refuses it now, unless
// the row is hand-marked a reader line and names the test proving it. A new
// refusal is allowed: stricter is permitted.
// The corpus is collected, never copied: the lines the corpus source tests
// judge (run here as subtests, through the helpers they judge with), the bash
// parity rows and the Bash fixture values of internal/le/ai/hooks (read from
// its source, which this package cannot import), and the spec's construct
// rows. Each check of the pretool-bash action is asked alone, over a fixed
// tree, except the call budget, which counts calls rather than judging one.
// PREVENTS: the parser replacement clearing a line HEAD refused, which a test
// written against the new contract is green over by construction.
func TestGuardVerdictsOfHeadStand(t *testing.T) {
	corpus, judgedBy := collectCorpus(t)
	recorded, err := readVerdictTable(verdictsTablePath)
	if err != nil {
		if !*verdictsUpdate {
			t.Fatalf("%v\nRestore the committed table: an update run only appends to it.", err)
		}
		// The table is committed: an update run over a missing table would
		// record every row from the tree in hand, with none of HEAD's refusals.
		if errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%v\nThe table is committed and an update only appends to it: restore the committed file, never record it again", err)
		}
		t.Fatal(err)
	}
	root := goBuildTree(t)
	t.Setenv("GOFLAGS", "")
	t.Setenv("CDPATH", "")
	if *verdictsUpdate {
		held := map[verdictKey]bool{}
		for _, row := range recorded {
			held[row.key()] = true
		}
		fresh := make([]verdictRow, 0, len(corpus))
		for _, key := range sortedVerdictKeys(corpus) {
			if held[key] {
				continue
			}
			fresh = append(fresh, verdictRow{Line: key.line, Caller: key.caller, RefusedBy: refusingChecks(t, root, key)})
		}
		recorded = mergeVerdictRows(recorded, fresh)
		if err := writeVerdictTable(verdictsTablePath, recorded); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %d rows, %d appended", verdictsTablePath, len(recorded), len(fresh))
	}
	if len(recorded) < verdictsTableRowsMin {
		t.Errorf("%s holds %d rows and its last update left %d (verdictsTableRowsMin): rows were lost, and an update re-records them from the tree in hand rather than from HEAD; restore the committed file",
			verdictsTablePath, len(recorded), verdictsTableRowsMin)
	}
	checkVerdictTable(t, t, root, recorded, corpus, judgedBy)
}

// TestVerdictTableUpdateKeepsEveryRecordedRow drives mergeVerdictRows, the one
// function the update mode writes the table through, with a recorded row the
// corpus no longer holds, a recorded row the hook now judges differently, and
// a fresh row.
//
// VALIDATES: the merge keeps each recorded row byte for byte, adds the fresh
// one, and answers them sorted by line and caller.
// PREVENTS: an update run that drops a row or rewrites its refusals, which
// would clear a lost refusal with no diff line saying so.
func TestVerdictTableUpdateKeepsEveryRecordedRow(t *testing.T) {
	gone := verdictRow{Line: "go test ./gone", Caller: callerMain, RefusedBy: []string{"bashRawHeavy"}}
	kept := verdictRow{Line: "go vet ./x", Caller: callerSubagent, RefusedBy: []string{"bashRawHeavy", "bashAgentIdentity"}}
	added := verdictRow{Line: "cat a", Caller: callerMain, RefusedBy: []string{}}
	relabelled := verdictRow{Line: kept.Line, Caller: kept.Caller, RefusedBy: []string{}}

	merged := mergeVerdictRows([]verdictRow{gone, kept}, []verdictRow{added, relabelled})
	want := []verdictRow{added, gone, kept}
	if !reflect.DeepEqual(merged, want) {
		t.Fatalf("merged = %+v\nwant %+v", merged, want)
	}
}

// TestReaderLineRowWaivesOnlyTheChecksAReaderLineLifts drives checkVerdictTable
// with hand-marked reader-line rows over a fixed tree, and reads what it
// reports.
//
// VALIDATES: a reader-line row keeps every recorded refusal of a check that
// never asks the reader set, and a waiver of one is reported; it waives a
// recorded refusal of a check a reader line lifts only when its Waives names
// it (bashRawHeavy, and bashScratch, over `cat a`: a reader line writes no
// file, so the scratch guard lifts on one), so a liftable refusal it does not
// name still holds, and the same row unmarked waives nothing; a mark that
// waives nothing, a mark on a line the predicate puts outside, a proof that
// asserts a refusal, and a proof that judged another line, or none, are
// refused.
// PREVENTS: a reader-line mark that clears a git, poll or governed write
// refusal, and a proof that names any test of the package.
func TestReaderLineRowWaivesOnlyTheChecksAReaderLineLifts(t *testing.T) {
	root := goBuildTree(t)
	t.Setenv("GOFLAGS", "")
	t.Setenv("CDPATH", "")
	const proof = "TestTestDeletionAsksOnlyForATestPath"
	judge := func(row verdictRow, judgedBy map[string]map[string]bool) verdictFaults {
		var faults verdictFaults
		checkVerdictTable(&faults, t, root, []verdictRow{row}, map[verdictKey]bool{row.key(): true}, judgedBy)
		return faults
	}
	judged := func(line string) map[string]map[string]bool {
		return map[string]map[string]bool{line: {proof: true}}
	}

	holds := func(faults verdictFaults, part string) bool {
		return slices.ContainsFunc(faults, func(fault string) bool { return strings.Contains(fault, part) })
	}

	// A mark on `git merge main`, which bashBranchMove refuses, waives a
	// check no reader line lifts and marks a line that is no reader line:
	// both are reported, and the refusal itself still holds.
	merge := verdictRow{Line: "git merge main", Caller: callerMain, RefusedBy: []string{"bashBranchMove"}, ReaderLine: true, Proof: proof, Waives: []string{"bashBranchMove"}}
	if faults := judge(merge, judged(merge.Line)); !holds(faults, "which no reader line lifts") || !holds(faults, "no reader line now") {
		t.Errorf("a reader-line mark on %q: faults %q, want the waiver and the mark refused", merge.Line, faults)
	}
	for _, check := range []string{"bashBranchMove", "bashDestructiveGit", "bashPollLoop", "bashGovernedWrite"} {
		lost := verdictRow{Line: "cat a", Caller: callerMain, RefusedBy: []string{check}, ReaderLine: true, Proof: proof, Waives: []string{check}}
		if faults := judge(lost, judged(lost.Line)); !holds(faults, check+" refused") {
			t.Errorf("a reader-line row whose %s refusal the hook no longer gives: faults %q, want that refusal held", check, faults)
		}
	}

	lifted := verdictRow{Line: "cat a", Caller: callerMain, RefusedBy: []string{"bashRawHeavy"}, ReaderLine: true, Proof: proof, Waives: []string{"bashRawHeavy"}}
	if faults := judge(lifted, judged(lifted.Line)); len(faults) != 0 {
		t.Errorf("a reader-line row losing a bashRawHeavy refusal: faults %q, want none", faults)
	}
	scratch := verdictRow{Line: "cat a", Caller: callerMain, RefusedBy: []string{"bashScratch"}, ReaderLine: true, Proof: proof, Waives: []string{"bashScratch"}}
	if faults := judge(scratch, judged(scratch.Line)); len(faults) != 0 {
		t.Errorf("a reader-line row losing a bashScratch refusal: faults %q, want none", faults)
	}
	unmarkedScratch := scratch
	unmarkedScratch.ReaderLine, unmarkedScratch.Proof, unmarkedScratch.Waives = false, "", nil
	if faults := judge(unmarkedScratch, judged(unmarkedScratch.Line)); len(faults) != 1 || !strings.Contains(faults[0], "bashScratch refused") {
		t.Errorf("the bashScratch row unmarked: faults %q, want the bashScratch refusal held", faults)
	}

	// The waiver is per check: a liftable refusal the row does not name
	// still holds, and a mark that names none is reported.
	partial := verdictRow{Line: "cat a", Caller: callerMain, RefusedBy: []string{"bashRawHeavy", "bashSystemTmp"}, ReaderLine: true, Proof: proof, Waives: []string{"bashRawHeavy"}}
	if faults := judge(partial, judged(partial.Line)); len(faults) != 1 || !strings.Contains(faults[0], "bashSystemTmp refused") {
		t.Errorf("a reader-line row waiving bashRawHeavy alone: faults %q, want the bashSystemTmp refusal held", faults)
	}
	bare := lifted
	bare.Waives = nil
	if faults := judge(bare, judged(bare.Line)); !holds(faults, "waives no check") || !holds(faults, "bashRawHeavy refused") {
		t.Errorf("a reader-line row with no waives: faults %q, want the mark refused and the refusal held", faults)
	}

	// A proof that asserts a refusal, and a mark on a line the predicate
	// puts outside, are reported.
	const refusing = "TestReaderLineRefusesWhatCanRunAnythingElse"
	cited := lifted
	cited.Proof = refusing
	if faults := judge(cited, map[string]map[string]bool{cited.Line: {refusing: true}}); !holds(faults, "asserts a refusal") {
		t.Errorf("a reader-line row citing %s: faults %q, want the proof refused", refusing, faults)
	}
	outside := verdictRow{Line: "./le x", Caller: callerMain, RefusedBy: []string{"bashRawHeavy"}, ReaderLine: true, Proof: proof, Waives: []string{"bashRawHeavy"}}
	if faults := judge(outside, judged(outside.Line)); !holds(faults, "no reader line now") {
		t.Errorf("a reader-line mark on %q: faults %q, want the mark refused", outside.Line, faults)
	}

	unmarked := lifted
	unmarked.ReaderLine, unmarked.Proof, unmarked.Waives = false, "", nil
	if faults := judge(unmarked, judged(unmarked.Line)); len(faults) != 1 || !strings.Contains(faults[0], "bashRawHeavy refused") {
		t.Errorf("the same row unmarked: faults %q, want the bashRawHeavy refusal held", faults)
	}
	if faults := judge(lifted, judged("cat b")); len(faults) != 1 || !strings.Contains(faults[0], "judged that exact line") {
		t.Errorf("a proof that judged another line: faults %q, want the proof refused", faults)
	}
	if faults := judge(lifted, map[string]map[string]bool{}); len(faults) != 1 || !strings.Contains(faults[0], "judged that exact line") {
		t.Errorf("a proof that judged nothing: faults %q, want the proof refused", faults)
	}
}

// TestReaderLiftableChecksAreTheReaderGuards reads the set readerLiftableChecks
// derives from the source.
//
// VALIDATES: the eight guards that ask the reader set are in it (the
// identity exemption, the lifts of the raw-job, lossy-pipe, /tmp and scratch
// guards, the build guard's reader+build line, the plain-rm narrowing, and
// the subagent heavy-work check, which opens with the same exemption), and
// every other pretool-bash check is not: the git guards, bashGovernedWrite
// and bashPollLoop, which the spec says never lift, and the checks that never
// read the reader judgment (bashDocumentRead judges its text alone). The
// scratch guard refuses only a write at the tmp/ root, and a reader line
// writes no file, so it lifts there (hook defect (b) of AC-36).
// PREVENTS: a derivation that follows a shared helper, or a local whose name
// a method shares, into a git guard and lets a reader-line mark waive its
// refusal.
func TestReaderLiftableChecksAreTheReaderGuards(t *testing.T) {
	liftable := readerLiftableChecks(t)
	readers := []string{"bashAgentIdentity", "bashRawHeavy", "bashLossyPipe", "bashSystemTmp", "bashScratch", "bashRootBuild", "bashTestDeletion", "bashSubagentHeavy"}
	for _, name := range readers {
		if !liftable[name] {
			t.Errorf("%s asks the reader set and is not in the liftable set %v", name, liftable)
		}
	}
	for _, name := range []string{"bashWorktreeCopy", "bashDestructiveGit", "bashBranchMove", "bashGovernedWrite", "bashPollLoop"} {
		if liftable[name] {
			t.Errorf("%s never lifts on a reader line and is in the liftable set %v", name, liftable)
		}
	}
	for _, check := range nativeHookActions["pretool-bash"].checks {
		name := checkName(check)
		if liftable[name] && !slices.Contains(readers, name) {
			t.Errorf("%s does not ask the reader set and is in the liftable set %v", name, liftable)
		}
	}
}

// collectCorpus runs every corpus source test and every reader-line proof as a
// subtest with the recorder on, then adds the parity, fixture and spec lines,
// and answers each line once per caller it is judged as: main and subagent.
// A fork is a subagent, so the corpus holds no fork key (callerFork). It also
// answers, for each line a test judged, the tests that judged it.
func collectCorpus(t *testing.T) (map[verdictKey]bool, map[string]map[string]bool) {
	t.Helper()
	holdSourcesComplete(t)

	corpusRecorder.Lock()
	corpusRecorder.lines = map[string]bool{}
	corpusRecorder.judgedBy = map[string]map[string]bool{}
	corpusRecorder.faults = nil
	corpusRecorder.Unlock()
	defer func() {
		corpusRecorder.Lock()
		corpusRecorder.lines = nil
		corpusRecorder.judgedBy = nil
		corpusRecorder.current = ""
		corpusRecorder.Unlock()
	}()

	for _, source := range slices.Concat(corpusSourceTests, readerLineProofs) {
		name := testFunctionName(source)
		corpusRecorder.Lock()
		corpusRecorder.current = name
		corpusRecorder.Unlock()
		// A proof that fails proves nothing: its name is withheld from the
		// lines it judged, so no row can cite it.
		if !t.Run(name, source) {
			corpusRecorder.Lock()
			for _, tests := range corpusRecorder.judgedBy {
				delete(tests, name)
			}
			corpusRecorder.Unlock()
		}
	}

	corpusRecorder.Lock()
	lines := corpusRecorder.lines
	judgedBy := corpusRecorder.judgedBy
	faults := corpusRecorder.faults
	corpusRecorder.Unlock()
	for _, fault := range faults {
		t.Error(fault)
	}
	for _, line := range append(aihooksCorpusLines(t), specConstructLines...) {
		lines[line] = true
	}

	temporary := filepath.Join(os.TempDir(), "Test")
	corpus := make(map[verdictKey]bool, 2*len(lines))
	for line := range lines {
		if strings.Contains(line, temporary) {
			t.Errorf("corpus line %q holds a test's temporary directory, which differs on every run: judge it under a fixed path", line)
			continue
		}
		corpus[verdictKey{line: line, caller: callerMain}] = true
		corpus[verdictKey{line: line, caller: callerSubagent}] = true
	}
	return corpus, judgedBy
}

// completeSources names, per test file, the prefix of the tests that file
// must list in corpusSourceTests: every test of session_identity_test.go, and
// every deletion test of bash_test.go, whose word list the parser replacement
// rewrites.
//
//nolint:gochecknoglobals // the declared complete sources
var completeSources = map[string]string{
	"session_identity_test.go": "Test",
	"bash_test.go":             "TestTestDeletion",
}

// holdSourcesComplete refuses a test of a completeSources file, under its
// prefix, that corpusSourceTests does not list, so a table added there joins
// the corpus.
func holdSourcesComplete(t *testing.T) {
	t.Helper()
	listed := map[string]bool{}
	for _, source := range corpusSourceTests {
		listed[testFunctionName(source)] = true
	}
	for path, prefix := range completeSources {
		for _, decl := range parseGoFile(t, path).Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if !strings.HasPrefix(function.Name.Name, prefix) {
				continue
			}
			if !listed[function.Name.Name] {
				t.Errorf("%s (%s) is not in corpusSourceTests: the lines it judges are outside the recorded corpus", function.Name.Name, path)
			}
		}
	}
}

// testFunctionName is the Go name of a test function, without its package
// path, read from the function value so a list entry cannot carry a wrong name.
func testFunctionName(test func(*testing.T)) string {
	full := goruntime.FuncForPC(reflect.ValueOf(test).Pointer()).Name()
	if _, name, found := strings.CutLast(full, "."); found {
		return name
	}
	return full
}

// refusingChecks answers the checks of the pretool-bash action that refuse
// one corpus row, each asked alone, in registry order. The call budget is left
// out: it counts calls and would refuse a row for the rows judged before it.
func refusingChecks(t *testing.T, root string, key verdictKey) []string {
	t.Helper()
	payload := Payload{SessionID: "parent-17", ToolName: toolBash, ToolInput: map[string]any{"command": key.line}}
	switch key.caller {
	case callerMain:
	case callerSubagent, callerFork:
		// A fork is a subagent: its payload names an agent, so the two
		// callers are one caller here by construction.
		payload.AgentID = "agent-1"
	default:
		t.Fatalf("row %q names the caller %q: a caller is main, subagent or fork", key.line, key.caller)
	}
	ctx, ok := newContext(payload, root)
	if !ok {
		t.Fatalf("no context for the root %s", root)
	}
	refused := []string{}
	budget := checkName(bashCallBudget)
	for _, check := range nativeHookActions["pretool-bash"].checks {
		name := checkName(check)
		if name == budget {
			continue
		}
		if result := check(ctx); result != nil && result.code >= 2 {
			refused = append(refused, name)
		}
	}
	return refused
}

// verdictReporter is where checkVerdictTable reports a broken rule: the test
// itself, or a collector a test of the judgment reads.
type verdictReporter interface {
	Helper()
	Errorf(format string, args ...any)
}

// verdictFaults collects what checkVerdictTable reports, for a test that
// judges the judgment.
type verdictFaults []string

func (*verdictFaults) Helper() {}

func (faults *verdictFaults) Errorf(format string, args ...any) {
	*faults = append(*faults, fmt.Sprintf(format, args...))
}

// checkVerdictTable is the plain run's judgment of the table, and the update
// run's after it wrote. It reports to t, and asks the checks through run.
// A reader-line row is held to two rules more than its mark: its proof is a
// test of corpusSourceTests or readerLineProofs that judged that exact line
// and passed (judgedBy, from collectCorpus) and asserts no refusal
// (refusalProofs); the predicate answers reader line for it now
// (readerRowFaults); and it waives only the recorded refusals its Waives
// names, each of a check a reader line can lift (readerLiftableChecks).
// Every other check it recorded still refuses it.
func checkVerdictTable(t verdictReporter, run *testing.T, root string, recorded []verdictRow, corpus map[verdictKey]bool, judgedBy map[string]map[string]bool) {
	t.Helper()
	proofs := packageTestNames(run)
	liftable := readerLiftableChecks(run)
	held := map[verdictKey]bool{}
	for _, row := range recorded {
		key := row.key()
		if held[key] {
			t.Errorf("%s holds %q for %s twice", verdictsTablePath, row.Line, row.Caller)
		}
		held[key] = true
		if row.ReaderLine && !proofs[row.Proof] {
			t.Errorf("%q (%s) is marked a reader line, and its proof %q names no test of this package", row.Line, row.Caller, row.Proof)
		}
		if row.ReaderLine && !judgedBy[row.Line][row.Proof] {
			t.Errorf("%q (%s) is marked a reader line, and its proof %q is no test of corpusSourceTests or readerLineProofs that judged that exact line and passed", row.Line, row.Caller, row.Proof)
		}
		if !row.ReaderLine && row.Proof != "" {
			t.Errorf("%q (%s) names the proof %q but is not marked a reader line", row.Line, row.Caller, row.Proof)
		}
		if !row.ReaderLine && len(row.Waives) != 0 {
			t.Errorf("%q (%s) waives %v but is not marked a reader line", row.Line, row.Caller, row.Waives)
		}
		if row.ReaderLine {
			readerRowFaults(t, row, liftable)
		}
	}
	missing := 0
	for _, key := range sortedVerdictKeys(corpus) {
		if !held[key] {
			missing++
			t.Errorf("corpus line %q (%s) has no row in %s", key.line, key.caller, verdictsTablePath)
		}
	}
	if missing > 0 {
		t.Errorf("%d corpus rows are missing: record them with -args -update, which appends and keeps every recorded row", missing)
	}
	for _, row := range recorded {
		now := refusingChecks(run, root, row.key())
		for _, check := range row.RefusedBy {
			if row.ReaderLine && liftable[check] && slices.Contains(row.Waives, check) {
				continue
			}
			if !slices.Contains(now, check) {
				t.Errorf("%s refused %q (%s) when the table was recorded and passes it now (refused by %v)", check, row.Line, row.Caller, now)
			}
		}
	}
}

// packageTestNames answers the Test functions this package's test files
// declare: a reader-line row's proof names one of them.
func packageTestNames(t *testing.T) map[string]bool {
	t.Helper()
	paths, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, path := range paths {
		for _, decl := range parseGoFile(t, path).Decls {
			if function, ok := decl.(*ast.FuncDecl); ok && strings.HasPrefix(function.Name.Name, "Test") {
				names[function.Name.Name] = true
			}
		}
	}
	return names
}

// readerLiftableChecks answers the pretool-bash checks whose code reaches
// readerSetJudgment, read from this package's non-test source
// (packageSource.reaches). TestReaderLiftableChecksAreTheReaderGuards holds
// it apart from the guards the spec says never lift.
func readerLiftableChecks(t *testing.T) map[string]bool {
	t.Helper()
	source := readPackageSource(t)
	if source.functions[readerSetJudgment] == nil {
		t.Fatalf("the package declares no function %s: readerSetJudgment names the function that judges whether a command is a reader", readerSetJudgment)
	}
	liftable := map[string]bool{}
	for _, check := range nativeHookActions["pretool-bash"].checks {
		name := checkName(check)
		if source.functions[name] == nil {
			t.Fatalf("the pretool-bash check %s has no body in this package's source", name)
		}
		if source.reaches(name, readerSetJudgment) {
			liftable[name] = true
		}
	}
	return liftable
}

// packageSource is this package's non-test source as the liftable
// derivation reads it: the bodies of its functions and of its methods by
// name, the field names its struct types declare, and the names its files
// import packages under.
type packageSource struct {
	functions map[string][]*ast.BlockStmt
	methods   map[string][]*ast.BlockStmt
	fields    map[string]bool
	imports   map[string]bool
}

// readPackageSource parses every non-test Go file of the package.
func readPackageSource(t *testing.T) packageSource {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	source := packageSource{
		functions: map[string][]*ast.BlockStmt{},
		methods:   map[string][]*ast.BlockStmt{},
		fields:    map[string]bool{},
		imports:   map[string]bool{},
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file := parseGoFile(t, path)
		for _, spec := range file.Imports {
			source.imports[importedName(spec)] = true
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if structure, ok := node.(*ast.StructType); ok {
				for _, field := range structure.Fields.List {
					for _, name := range field.Names {
						source.fields[name.Name] = true
					}
				}
			}
			return true
		})
		for _, decl := range file.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if function.Body == nil {
				continue
			}
			if function.Recv == nil {
				source.functions[function.Name.Name] = append(source.functions[function.Name.Name], function.Body)
				continue
			}
			source.methods[function.Name.Name] = append(source.methods[function.Name.Name], function.Body)
		}
	}
	return source
}

// importedName is the name a file uses for one import: its alias, or the
// last element of its path, skipping a major-version element (`v3`).
func importedName(spec *ast.ImportSpec) string {
	if spec.Name != nil {
		return spec.Name.Name
	}
	importPath, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return spec.Path.Value
	}
	elements := strings.Split(importPath, "/")
	last := elements[len(elements)-1]
	if len(elements) > 1 && len(last) > 1 && last[0] == 'v' && strings.Trim(last[1:], "0123456789") == "" {
		return elements[len(elements)-2]
	}
	return last
}

// reaches answers whether target, a package function, is start or is named
// in a body start reaches. Without types, a name is resolved as far as the
// syntax shows what it is, and every doubt resolves toward reaching, so the
// set can only be too wide:
//   - a bare name is a package function, and a local that shadows one is
//     followed into it too;
//   - a selector's name is a method of every receiver that declares one, when
//     the selector is called or when no struct field carries the name, so
//     `ctx.call.parse` reads the field and `walk.call(c, p)` the method;
//   - a selector on an imported package (`strings.Join`) and the field key of
//     a composite literal name nothing of this package.
//
// Matching a name against every declaration whatever its place sends a
// guard's local `command` into the walks' command methods, which reach the
// reader judgment (identityWalk.statement names readerAdmit), and so puts
// every guard, the git guards included, in the set.
func (source packageSource) reaches(start, target string) bool {
	if start == target {
		return true
	}
	seen := map[string]bool{"function " + start: true}
	queue := slices.Clone(source.functions[start])
	found := false
	follow := func(kind, name string, bodies map[string][]*ast.BlockStmt) {
		if bodies[name] == nil {
			return
		}
		if seen[kind+" "+name] {
			return
		}
		seen[kind+" "+name] = true
		if kind == "function" && name == target {
			found = true
		}
		queue = append(queue, bodies[name]...)
	}
	called := map[*ast.SelectorExpr]bool{}
	var visit func(node ast.Node) bool
	visit = func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.CallExpr:
			if selector, ok := node.Fun.(*ast.SelectorExpr); ok {
				called[selector] = true
			}
		case *ast.KeyValueExpr:
			if _, field := node.Key.(*ast.Ident); field {
				ast.Inspect(node.Value, visit)
				return false
			}
		case *ast.SelectorExpr:
			if qualifier, ok := node.X.(*ast.Ident); ok && source.imports[qualifier.Name] {
				return false
			}
			if called[node] || !source.fields[node.Sel.Name] {
				follow("method", node.Sel.Name, source.methods)
			}
			ast.Inspect(node.X, visit)
			return false
		case *ast.Ident:
			follow("function", node.Name, source.functions)
		}
		return true
	}
	for len(queue) > 0 && !found {
		body := queue[0]
		queue = queue[1:]
		ast.Inspect(body, visit)
	}
	return found
}

// mergeVerdictRows answers every recorded row unchanged, plus each fresh row
// whose line and caller no recorded row holds, sorted by line then caller. It
// is the only path by which the update mode changes the table.
func mergeVerdictRows(recorded, fresh []verdictRow) []verdictRow {
	merged := slices.Clone(recorded)
	held := map[verdictKey]bool{}
	for _, row := range recorded {
		held[row.key()] = true
	}
	for _, row := range fresh {
		if held[row.key()] {
			continue
		}
		held[row.key()] = true
		merged = append(merged, row)
	}
	slices.SortStableFunc(merged, compareVerdictRows)
	return merged
}

func compareVerdictRows(a, b verdictRow) int {
	if order := strings.Compare(a.Line, b.Line); order != 0 {
		return order
	}
	return strings.Compare(a.Caller, b.Caller)
}

func sortedVerdictKeys(corpus map[verdictKey]bool) []verdictKey {
	keys := make([]verdictKey, 0, len(corpus))
	for key := range corpus {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b verdictKey) int {
		return compareVerdictRows(verdictRow{Line: a.line, Caller: a.caller}, verdictRow{Line: b.line, Caller: b.caller})
	})
	return keys
}

// readVerdictTable decodes the table, one JSON object per line, refusing a
// field the row type does not declare.
func readVerdictTable(path string) ([]verdictRow, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	rows := []verdictRow{}
	for decoder.More() {
		var row verdictRow
		if err := decoder.Decode(&row); err != nil {
			return nil, errors.New(path + " row " + strconv.Itoa(len(rows)+1) + ": " + err.Error())
		}
		if row.RefusedBy == nil {
			return nil, errors.New(path + " row " + strconv.Itoa(len(rows)+1) + " has no refused-by list: an empty list is written []")
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// writeVerdictTable writes the rows one JSON object per line, with no HTML
// escaping, so a `<` or `&` of a command reads as typed in the diff.
func writeVerdictTable(path string, rows []verdictRow) error {
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	encoder.SetEscapeHTML(false)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, body.Bytes(), 0o600)
}

// aihooksCorpusLines reads, from the source of internal/le/ai/hooks, the bash
// parity rows and the Bash values its fixture categories probe. That package
// imports this one, so it is read, not imported. Every shape the reading does
// not model fails the test rather than leaving a line out.
func aihooksCorpusLines(t *testing.T) []string {
	t.Helper()
	lines := bashGoldenLines(t, parseGoFile(t, filepath.Join("..", "ai", "hooks", "parity_catalog.go")))
	return append(lines, bashFixtureValues(t, parseGoFile(t, filepath.Join("..", "ai", "hooks", "fixtures.go")))...)
}

// bashGoldenLines answers the name of every fixture of the BASH_GOLDEN parity
// table, and holds their count to the table's own expected count.
func bashGoldenLines(t *testing.T, file *ast.File) []string {
	t.Helper()
	for _, element := range varLiteral(t, file, "parityCatalog").Elts {
		table, ok := element.(*ast.CompositeLit)
		if !ok {
			continue
		}
		key := literalField(table, "key")
		if key == nil {
			continue
		}
		if stringLiteral(t, key, "a parity table key") != "BASH_GOLDEN" {
			continue
		}
		expected, err := strconv.Atoi(basicLiteral(t, literalField(table, "expected"), token.INT, "BASH_GOLDEN expected"))
		if err != nil {
			t.Fatal(err)
		}
		fixtures, ok := literalField(table, "fixtures").(*ast.CompositeLit)
		if !ok {
			t.Fatal("BASH_GOLDEN fixtures are not a composite literal")
		}
		lines := make([]string, 0, len(fixtures.Elts))
		for _, fixture := range fixtures.Elts {
			row, ok := fixture.(*ast.CompositeLit)
			if !ok {
				t.Fatalf("a BASH_GOLDEN fixture is not a composite literal: %T", fixture)
			}
			lines = append(lines, stringLiteral(t, literalField(row, "name"), "a BASH_GOLDEN fixture name"))
		}
		if len(lines) != expected {
			t.Fatalf("read %d BASH_GOLDEN fixtures, and the table expects %d", len(lines), expected)
		}
		return lines
	}
	t.Fatal("parity_catalog.go declares no BASH_GOLDEN table")
	return nil
}

// bashFixtureValues answers the allow and refuse values of every fixture
// category whose probe sends a Bash command (categoryProbes, tool "Bash"). A
// Bash probe with a wrap would send another string than the value, so it fails.
func bashFixtureValues(t *testing.T, file *ast.File) []string {
	t.Helper()
	bash := map[string]bool{}
	for _, element := range varLiteral(t, file, "categoryProbes").Elts {
		entry, ok := element.(*ast.KeyValueExpr)
		if !ok {
			t.Fatalf("a categoryProbes entry is not a key and value: %T", element)
		}
		probe, ok := entry.Value.(*ast.CompositeLit)
		if !ok {
			t.Fatalf("a categoryProbes value is not a composite literal: %T", entry.Value)
		}
		tool := literalField(probe, "tool")
		if tool == nil {
			continue
		}
		// A tool named by a constant is read from that constant's literal;
		// any other shape may be Bash, so the reading fails rather than leave
		// that probe's values out.
		if name, isName := tool.(*ast.Ident); isName {
			tool = constLiteral(t, file, name.Name)
		}
		if stringLiteral(t, tool, "a categoryProbes tool") != toolBash {
			continue
		}
		category, ok := entry.Key.(*ast.Ident)
		if !ok {
			t.Fatalf("a Bash categoryProbes key is not a name: %T", entry.Key)
		}
		if literalField(probe, "wrap") != nil {
			t.Fatalf("the Bash probe of %s wraps its value, which this reading does not model", category.Name)
		}
		bash[category.Name] = true
	}
	if len(bash) == 0 {
		t.Fatal("categoryProbes holds no probe whose tool is \"Bash\": the reading found none of the Bash fixture values it collects")
	}
	allow, refuse := structFieldIndex(t, file, "fixtureCategory", "allow"), structFieldIndex(t, file, "fixtureCategory", "refuse")
	values := []string{}
	found := map[string]bool{}
	for _, element := range varLiteral(t, file, "fixtureCategories").Elts {
		row, ok := element.(*ast.CompositeLit)
		if !ok {
			t.Fatalf("a fixtureCategories row is not a composite literal: %T", element)
		}
		category, ok := row.Elts[0].(*ast.Ident)
		if !ok {
			continue
		}
		if !bash[category.Name] {
			continue
		}
		found[category.Name] = true
		values = append(values,
			stringLiteral(t, row.Elts[allow], category.Name+" allow"),
			stringLiteral(t, row.Elts[refuse], category.Name+" refuse"))
	}
	for category := range bash {
		if !found[category] {
			t.Errorf("the Bash probe %s has no fixtureCategories row", category)
		}
	}
	return values
}

func parseGoFile(t *testing.T, path string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

// varLiteral answers the composite literal a top-level var of the file holds.
func varLiteral(t *testing.T, file *ast.File, name string) *ast.CompositeLit {
	t.Helper()
	for _, decl := range file.Decls {
		general, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if general.Tok != token.VAR {
			continue
		}
		for _, spec := range general.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			if len(value.Names) != 1 {
				continue
			}
			if value.Names[0].Name != name {
				continue
			}
			if len(value.Values) != 1 {
				t.Fatalf("var %s holds %d values, want one literal", name, len(value.Values))
			}
			literal, ok := value.Values[0].(*ast.CompositeLit)
			if !ok {
				t.Fatalf("var %s is not a composite literal: %T", name, value.Values[0])
			}
			return literal
		}
	}
	t.Fatalf("%s declares no var %s", file.Name.Name, name)
	return nil
}

// constLiteral answers the value expression of a top-level const of the file
// that holds one value per name, and fails for any other declaration.
func constLiteral(t *testing.T, file *ast.File, name string) ast.Expr {
	t.Helper()
	for _, decl := range file.Decls {
		general, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if general.Tok != token.CONST {
			continue
		}
		for _, spec := range general.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, declared := range value.Names {
				if declared.Name != name {
					continue
				}
				if len(value.Values) != len(value.Names) {
					t.Fatalf("const %s takes its value from an earlier line, which this reading does not model", name)
				}
				return value.Values[index]
			}
		}
	}
	t.Fatalf("%s declares no const %s", file.Name.Name, name)
	return nil
}

// structFieldIndex answers the position of one field in a struct type the
// file declares, so a positional literal is read by field name.
func structFieldIndex(t *testing.T, file *ast.File, typeName, field string) int {
	t.Helper()
	for _, decl := range file.Decls {
		general, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range general.Specs {
			named, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if named.Name.Name != typeName {
				continue
			}
			structure, ok := named.Type.(*ast.StructType)
			if !ok {
				t.Fatalf("%s is not a struct", typeName)
			}
			index := 0
			for _, member := range structure.Fields.List {
				for _, name := range member.Names {
					if name.Name == field {
						return index
					}
					index++
				}
			}
			t.Fatalf("struct %s has no field %s", typeName, field)
		}
	}
	t.Fatalf("no type %s", typeName)
	return 0
}

// literalField answers the value of one keyed field of a composite literal,
// or nil when the literal does not set it.
func literalField(literal *ast.CompositeLit, name string) ast.Expr {
	for _, element := range literal.Elts {
		entry, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, isName := entry.Key.(*ast.Ident); isName && key.Name == name {
			return entry.Value
		}
	}
	return nil
}

func basicLiteral(t *testing.T, expr ast.Expr, kind token.Token, what string) string {
	t.Helper()
	literal, ok := expr.(*ast.BasicLit)
	if !ok {
		t.Fatalf("%s is not a literal: %T", what, expr)
	}
	if literal.Kind != kind {
		t.Fatalf("%s is a %s literal, want %s", what, literal.Kind, kind)
	}
	return literal.Value
}

func stringLiteral(t *testing.T, expr ast.Expr, what string) string {
	t.Helper()
	value, err := strconv.Unquote(basicLiteral(t, expr, token.STRING, what))
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	return value
}
