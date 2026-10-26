// Design: docs/architecture/core-design.md -- native documentation verifier actions
// Related: repository.go -- the tracked-file walk this sweep shares with links
// Related: ../../le/retired/retired.go -- the rename map the sweep reads
//
// The retired-name sweep finds every tracked line that still names a form le
// retired (spec-le-subject-first-command-tree, spec-le-cli-usability).
// It reads the one rename map package leretired declares, which dispatch also
// reads to refuse a typed old spelling, and holds no list of its own. The same
// walk judges every le command a line cites against what le holds
// (commandcitation.go, AC-25 of spec-le-cli-usability), so a citation of a
// command le never had, or lost without a rename row, is refused too. It is a
// gate: a line that names an old form, or cites a command le does not hold,
// outside the declared records and exceptions makes it exit 1, and it runs as
// a stage of the full verify (internal/le/verify/engine/stages.go).

package doccheck

import (
	"regexp"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	leretired "github.com/ze-software/ze/internal/le/le/retired"
	specjournal "github.com/ze-software/ze/internal/le/spec/journal"
)

// retiredDeclarations are the files that declare the rename map, the sweep and
// their tests. They name every old form by necessity, so they are not callers.
// The map's own test drives real rows of the map, `le test harness` among them,
// so its messages spell old forms. The two leroot tests name old forms to prove
// each one is refused naming its successor.
var retiredDeclarations = [...]string{
	"internal/le/le/retired/retired.go",
	"internal/le/le/retired/retired_test.go",
	"internal/le/doc/check/retired.go",
	"internal/le/doc/check/retired_test.go",
	"internal/le/le/root/retired_test.go",
	"internal/le/le/root/retired_dispatch_test.go",
}

// retiredRecord is one path the sweep skips because it records history: what
// a command was called on the day the record was written (owner decision,
// 2026-09-24). A path that ends in a slash is a directory and takes every file
// under it.
type retiredRecord struct {
	path string
	why  string
}

// retiredRecords are the historical records AC-16 declares. A record states
// what happened under the names of its day, so a rewrite would falsify it.
var retiredRecords = [...]retiredRecord{
	{path: "plan/journal/", why: "defect rows record the command a session ran when it met the defect"},
	{path: "plan/verification-debt/", why: "debt entries record the command that was not run"},
	{path: "plan/learned/", why: "closed-spec summaries record the commands of their day"},
	{path: "plan/handover/", why: "handovers are frozen session state"},
	{path: "plan/audits/", why: "audits record the commands they ran"},
	{path: "website/blog/", why: "published posts are the owner's authored prose"},
	{path: "website/changes/posts/", why: "published change posts are the owner's authored prose"},
	{path: "website/changes/discord/2026-08-03-weekly.md", why: "a published weekly update"},
	{path: "website/talks/", why: "published talks record the commands of their day"},
	{path: "plan/known-failures/", why: "known-failure shards record the command that failed on the day it failed"},
	{path: "rfc/audit/", why: "RFC audit verdicts record the carrier commands they judged, and the renames those carriers went through"},
	{path: "rfc/discrimination/", why: "discrimination records carry the break the recorder applied on the day it ran, the halt text naming the command of that day included"},
	{path: "plan/records/", why: "historical records retain the commands and paths of their day"},
	{
		path: "internal/le/doc/check/testdata/doc_citation_baseline.txt",
		why:  "the link gate's baseline lists the dead paths that historical records already carried",
	},
	{
		path: "internal/le/verify/engine/testdata/",
		why:  "captured verify logs are parser fixtures that carry the harness output of their day",
	},
	{
		path: "internal/le/completeness_record_test.go",
		why:  "the Makefile migration record names each retired target and the recipe it ran",
	},
	{
		path: "cmd/ze/testdata/godebug-guidance-defect.txt",
		why:  "a verbatim copy of cmd/ze/main.go at a past commit, the defect the GODEBUG guidance test must go red on",
	},
	{
		path: "internal/le/site/testdata/published-health-record.json",
		why:  "a captured copy of a published health record, which the site renderer test replays as published",
	},
	{
		path: "internal/le/site/testdata/published-health-body.html",
		why:  "a captured copy of a published health page, which the site renderer test replays as published",
	},
}

// retiredException is one declared use of an old spelling that names
// something else: a word that happens to be spelled as a retired name.
type retiredException struct {
	file string
	old  string
	// lines are the lines the exception covers, each written as its text with
	// the indentation trimmed. A line is covered when it holds one of them, so
	// a live instruction elsewhere in the same file is still found.
	lines []string
	why   string
}

// retiredExceptions are the declared exceptions of AC-16. Each names the file,
// the old spelling it may keep, the lines that keep it, and why the spelling
// there is not the retired name.
// discordChannelWhy is the reason shared by the files that name the Discord
// channel.
const discordChannelWhy = "the Discord channel named ze-test, not the harness binary"

// retiredHookNegativeWhy is the reason of the hook test that proves a text
// search naming a retired harness binary is not refused as a raw test run.
const retiredHookNegativeWhy = "the hook test feeds the retired name to prove the deleted harness admission refuses nothing"

var retiredExceptions = [...]retiredException{
	{file: "internal/le/hookruntime/bash_test.go", old: retiredZeTest, lines: []string{"\"ze-test bgp plugin 42\","}, why: retiredHookNegativeWhy},
	{file: "internal/le/hookruntime/bash_test.go", old: retiredLeTest, lines: []string{"\"grep -n 'le-test bgp' internal/le/hookruntime/bash.go\","}, why: retiredHookNegativeWhy},
	{
		file:  "internal/le/weekly/answer.go",
		old:   retiredZeTest,
		lines: []string{"Requirement: leaction.Optional, Help: \"the channel to post to; ze-test is the rehearsal channel\"},"},
		why:   "the publication channel named ze-test, not the harness binary",
	},
	{
		file:  "internal/component/firewall/validate_test.go",
		old:   retiredZeTestTag,
		lines: []string{"Name:   \"ze_test\","},
		why:   "the nftables table named ze_test, not the build tag",
	},
	{
		file:  "docs/architecture/testing/qemu-integration.md",
		old:   retiredZeTestTag,
		lines: []string{"| Firewall table | nftables in netns | `nft add table ip ze_test` in an isolated namespace |"},
		why:   "the nftables table named ze_test, not the build tag",
	},
	{
		file:  "internal/component/bgp/reactor/filter_flowspec.go",
		old:   "le rules",
		lines: []string{"// prefix/ge/le rules without discarding the rest of an accepted FlowSpec NLRI."},
		why:   "the FlowSpec prefix operators ge/le, not the le command",
	},
	{
		file:  "website/changes/discord/STYLE.md",
		old:   retiredZeTest,
		lines: []string{"ze-test confirm` for a rehearsal. The command splits at section boundaries"},
		why:   discordChannelWhy,
	},
	{
		file:  "internal/le/le/action/leaction_test.go",
		old:   "le qemu",
		lines: []string{"want := \"le qemu run - boot a guest\\n\\n  boot a guest\\n\\nUsage:\\n\" +", "\"  le qemu run command <command> [timeout <duration>] [share <path>]... [keep-alive]\\n\" +", "if want := \"error: argument keyword \\\"timeout\\\" was provided more than once\\nusage: le qemu run \"; !strings.HasPrefix(refusal, want) {", "if !strings.HasPrefix(page, \"le qemu run - boot a guest\\n\") {"},
		why:   "the usage line of a fixture area named qemu, not the qemu command",
	},
	{
		file:  "internal/le/site/terminaldemo/terminaldemo_test.go",
		old:   "ze-terminal-pty",
		lines: []string{"if strings.Contains(strings.Join(recorder.Args, \" \"), \"ze-terminal-pty\") {", "t.Errorf(\"the demo build still builds ze-terminal-pty: %q\", recorder.Args)", "// PREVENTS: the container entrypoint or a validator still exec'ing ze-terminal-pty, which the demo build no longer writes."},
		why:   "a test that proves the demo build no longer builds or runs the retired recorder",
	},
	{
		file:  retiredGuestLeTestFile,
		old:   retiredLeTestBin,
		lines: []string{retiredGuestKeyLoopLine},
		why:   retiredVariableIgnoredWhy,
	},
	{
		file:  retiredGuestLeTestFile,
		old:   retiredZeTestBin,
		lines: []string{retiredGuestKeyLoopLine},
		why:   retiredVariableIgnoredWhy,
	},
	{
		file:  retiredGuestLeTestFile,
		old:   "le.qemu.test.bin",
		lines: []string{"// le.qemu.test.bin nor ze.qemu.test.bin is registered.", retiredGuestKeyLoopLine},
		why:   retiredVariableIgnoredWhy,
	},
	{
		file:  retiredGuestLeTestFile,
		old:   "ze.qemu.test.bin",
		lines: []string{"// le.qemu.test.bin nor ze.qemu.test.bin is registered.", retiredGuestKeyLoopLine},
		why:   retiredVariableIgnoredWhy,
	},
	{
		file:  retiredQEMUKnobsTestFile,
		old:   retiredLeTestBin,
		lines: []string{"\"ZE_BIN\", \"ZE_STRIPPED_BIN\", \"ZE_TEST_BIN\", \"LE_TEST_BIN\","},
		why:   retiredVariableIgnoredWhy,
	},
	{
		file:  retiredQEMUKnobsTestFile,
		old:   retiredZeTestBin,
		lines: []string{"\"ZE_BIN\", \"ZE_STRIPPED_BIN\", \"ZE_TEST_BIN\", \"LE_TEST_BIN\","},
		why:   retiredVariableIgnoredWhy,
	},
	{
		file:  retiredStressReproTestFile,
		old:   retiredLeTestBin,
		lines: []string{"for _, key := range []string{\"ze.bin\", \"ZE_BIN\", \"ze.test.bin\", \"ZE_TEST_BIN\", \"le.test.bin\", \"LE_TEST_BIN\"} {", "t.Setenv(\"LE_TEST_BIN\", absent)"},
		why:   retiredVariableIgnoredWhy,
	},
	{
		file:  retiredStressReproTestFile,
		old:   retiredZeTestBin,
		lines: []string{"for _, key := range []string{\"ze.bin\", \"ZE_BIN\", \"ze.test.bin\", \"ZE_TEST_BIN\", \"le.test.bin\", \"LE_TEST_BIN\"} {"},
		why:   retiredVariableIgnoredWhy,
	},
	{
		file:  "internal/le/test/functional/exabgp_test.go",
		old:   retiredLeTestBin,
		lines: []string{"if value, present := environment[\"LE_TEST_BIN\"]; present {", "t.Fatalf(\"LE_TEST_BIN = %q, want it unset: the harness is the set's le\", value)"},
		why:   retiredVariableIgnoredWhy,
	},
	{
		file:  retiredHostLoadTestFile,
		old:   retiredLeTest,
		lines: []string{"{\"/usr/bin/le-test ospf\", false},"},
		why:   retiredHarnessRefusedWhy,
	},
	{
		file:  retiredHostLoadTestFile,
		old:   "bin/le-test",
		lines: []string{"{\"/usr/bin/le-test ospf\", false},"},
		why:   retiredHarnessRefusedWhy,
	},
	{
		file:  "cmd/ze/login_test.go",
		old:   retiredLeTest,
		lines: []string{"{\"le-test does not trigger\", \"le-test\", false},"},
		why:   retiredHarnessRefusedWhy,
	},
	{
		file:  retiredPerfTestFile,
		old:   retiredZePerf,
		lines: []string{"// selection of the retired ze-perf-run, and every perf verb answers as the", "// ze-perf run, report and track, with the trailing words and the exit code.", "// track --check keeps the exit code ze-perf track --check gave.", "t.Fatal(\"`perf compare <absent file>` exited 0, so the file never reached ze-perf report\")"},
		why:   retiredParityWhy,
	},
	{
		file:  retiredPerfTestFile,
		old:   "ze-perf-run",
		lines: []string{"// selection of the retired ze-perf-run, and every perf verb answers as the"},
		why:   retiredParityWhy,
	},
	{
		file:  "internal/le/perf/bench_test.go",
		old:   retiredZePerf,
		lines: []string{"// VALIDATES: the retired ze-perf-bench, ze-perf-history-record and", "// PREVENTS: a benchmark action that looks for a host ze-perf, one that measures", "// way the ze-perf run subcommand emits a JSON document.", "// TestHistoryRecordAppendsOneCompactLinePerResult is the ze-perf-history-record"},
		why:   retiredParityWhy,
	},
	{
		file:  "internal/le/mrt/mrt_test.go",
		old:   "ze-analyze",
		lines: []string{"// PREVENTS: a subcommand that ze-analyze ran and le does not, and a listing"},
		why:   retiredParityWhy,
	},
	{file: retiredOwnershipTestFile, old: retiredZeTestTag, lines: []string{"\"a retired ze_test tag\":  {\"//go:build ze_test\\n\\npackage main\\n\\nfunc f() { registry.MustRegisterRootHandler(\\\"bgp\\\", nil, m) }\\n\", 1},"}, why: retiredTagRefusedWhy},
	{file: retiredOwnershipTestFile, old: "ze_chaos", lines: []string{"// is allowlisted. No build tag exempts the file: the ze_chaos variant that once", "\"a retired ze_chaos tag\": {\"//go:build ze_chaos\\n\\npackage main\\n\\nfunc f() { registry.MustRegisterRootHandler(\\\"bgp\\\", nil, m) }\\n\", 1},"}, why: retiredTagRefusedWhy},
	{file: retiredHarnessEnvTestFile, old: retiredLeTestBin, lines: []string{"// Method: NewRunner with a stale LE_TEST_BIN exported, then read lePath and", "t.Setenv(\"LE_TEST_BIN\", \"/stale/harness\")"}, why: retiredVariableIgnoredWhy},
	{file: retiredHarnessEnvTestFile, old: "ze.test.no.build", lines: []string{"if env.IsRegistered(\"ze.test.no.build\") {", "t.Fatal(\"ze.test.no.build is still registered\")", "t.Setenv(\"ZE_TEST_NO_BUILD\", tc.retired)"}, why: retiredVariableIgnoredWhy},
	{file: retiredHarnessEnvTestFile, old: retiredLeTest, lines: []string{"// VALIDATES: AC-37, the head `le`; AC-43, heads `le-test`, `ze-test` and", retiredHarnessHeadsLine, "for _, head := range []string{\"le-test\", \"ze-test\", \"ze-peer\"} {"}, why: retiredHarnessRefusedWhy},
	{file: retiredHarnessEnvTestFile, old: retiredZeTest, lines: []string{"// VALIDATES: AC-37, the head `le`; AC-43, heads `le-test`, `ze-test` and", retiredHarnessHeadsLine, "for _, head := range []string{\"le-test\", \"ze-test\", \"ze-peer\"} {"}, why: retiredHarnessRefusedWhy},
	{file: retiredHarnessEnvTestFile, old: retiredZePeer, lines: []string{"// `ze-peer` are left to a PATH lookup.", retiredHarnessHeadsLine}, why: retiredHarnessRefusedWhy},
	{file: retiredParsingTestFile, old: retiredLeTest, lines: []string{"{\"le-test fixture storage/empty-tree\", \"le-test fixture storage/empty-tree\"},"}, why: retiredHarnessRefusedWhy},
	{file: retiredParsingTestFile, old: retiredZeTest, lines: []string{"{\"ze-test fixture storage/empty-tree\", \"ze-test fixture storage/empty-tree\"},"}, why: retiredHarnessRefusedWhy},
	{file: "internal/test/runner/peer_contract_test.go", old: retiredZePeer, lines: []string{"// that the retired head `ze-peer` no longer takes the peer role (AC-43)."}, why: retiredHarnessRefusedWhy},
	{file: retiredPerfRunnerTestFile, old: retiredZePerf, lines: []string{"// bin/ze-perf path. The Docker argv is read where the wrapper renders it.", "if strings.Contains(word, filepath.Join(\"bin\", \"ze-perf\")) {", "t.Errorf(\"a command names a bin/ze-perf path: %q\", argv)"}, why: retiredPerfPathWhy},
	{file: retiredPerfRunnerTestFile, old: "bin/ze-perf", lines: []string{"// bin/ze-perf path. The Docker argv is read where the wrapper renders it.", "t.Errorf(\"a command names a bin/ze-perf path: %q\", argv)"}, why: retiredPerfPathWhy},
	{file: retiredWikiIndexFile, old: retiredZeTest, lines: []string{"\"summary\": \"ze-test is the test harness for Ze.\""}, why: retiredWikiIndexWhy},
	{file: retiredWikiIndexFile, old: retiredZeChaos, lines: []string{"\"summary\": \"A chaos scenario in Ze is a configuration of ze-chaos: the number of mock peers, the address families they negotiate, the duration, the chaos seed and rate, and any explicit fault triggers.\"", "\"summary\": \"The chaos dashboard is a live web UI for ze-chaos.\"", "\"summary\": \"Property verification is the part of ze-chaos that turns \\\"did the run finish without crashing\\\" into something useful.\""}, why: retiredWikiIndexWhy},
	{file: retiredWikiIndexFile, old: retiredZePerf, lines: []string{"\"summary\": \"ze-perf is a BGP propagation latency benchmark tool.\""}, why: retiredWikiIndexWhy},
	{
		file:  "internal/test/fixture/ui_fixture_le_checks_answers.go",
		old:   "le tracked",
		lines: []string{"trackedResult, err := runLE(nil, checkTestSensitivity, \"tracked\", \"|\", \"json\")"},
		why:   "the tracked action of `le test sensitivity`, passed after the command constant, not the retired command",
	},
	{
		file:  "internal/le/cli/grammar/cligrammar_test.go",
		old:   "le dash-stdio",
		lines: []string{"if namesLeFinding(result, \"dash-stdio\") {"},
		why:   "a synthetic probe root the grammar test feeds its own gate, and namesLeFinding runs no le",
	},
	{
		file:  retiredRunningCommandsFile,
		old:   "le verify lock",
		lines: []string{"`error: le verify lock was renamed: run le job run`"},
		why:   "the page quotes the refusal le prints for the retired spelling, not an instruction to run it",
	},
	{
		file:  retiredRunningCommandsFile,
		old:   "le spec session",
		lines: []string{"so `le spec session claim spec x`"},
		why:   retiredTwoHopWhy,
	},
	{
		file:  retiredRunningCommandsFile,
		old:   "le spec session claim",
		lines: []string{"so `le spec session claim spec x`"},
		why:   retiredTwoHopWhy,
	},
	{
		file:  "internal/test/fixture/ui_fixture_le_launcher_words.go",
		old:   "le rollback newest",
		lines: []string{`tb.Str("refused: ./le rollback newest, ./le rollback newest now,`},
		why:   "the fixture's report names the retired verb it feeds the launcher to prove the refusal",
	},
}

// retiredGuestKeyLoopLine is the guest-le test's loop over the four retired
// harness variables, which four rows declare.
const retiredGuestKeyLoopLine = "for _, key := range []string{\"le.qemu.test.bin\", \"ze.qemu.test.bin\", \"le.test.bin\", \"ze.test.bin\"} {"

// retiredHarnessHeadsLine is the harness-env test's loop over the retired
// heads, which three rows declare.
const retiredHarnessHeadsLine = "for _, head := range []string{\"le-test\", \"ze-test\", \"ze-peer\", \"lean\", \"sh\", \"./le\"} {"

// retiredTwoHopWhy is the reason of the page that shows a retired spelling
// whose successor was itself retired, to say the refusal names the final one.
const retiredTwoHopWhy = "the page quotes a twice-renamed spelling to show the refusal follows the renames to their end"

// The retired names that several exceptions declare.
const (
	retiredZeTest    = "ze-test"
	retiredLeTest    = "le-test"
	retiredZeTestTag = "ze_test"
	retiredZePerf    = "ze-perf"
	retiredZeChaos   = "ze-chaos"
	retiredZePeer    = "ze-peer"
)

// Files that declare more than one exception, and the variable names that
// several of them declare.
const (
	retiredLeTestBin           = "le.test.bin"
	retiredZeTestBin           = "ze.test.bin"
	retiredGuestLeTestFile     = "internal/le/test/qemu/guestle_run_test.go"
	retiredQEMUKnobsTestFile   = "internal/le/test/qemu/alltests_knobs_test.go"
	retiredStressReproTestFile = "internal/le/test/stressrepro/stressrepro_test.go"
	retiredHostLoadTestFile    = "internal/core/hostload/hostload_test.go"
	retiredPerfTestFile        = "internal/le/perf/perf_test.go"
	retiredOwnershipTestFile   = "internal/le/cli/ownership/commandownership_test.go"
	retiredHarnessEnvTestFile  = "internal/test/runner/harness_env_test.go"
	retiredParsingTestFile     = "internal/test/runner/parsing_test.go"
	retiredPerfRunnerTestFile  = "internal/test/perfrunner/run_test.go"
	retiredWikiIndexFile       = "website/data/wiki.json"
	retiredRunningCommandsFile = "docs/contributing/running-commands.md"
)

// retiredWikiIndexWhy is the reason of the wiki search index. `le site wiki
// update` derives it from the separate wiki repository, whose pages still name
// the retired programs, so the fix lands in that repository and the index
// follows on its next refresh.
const retiredWikiIndexWhy = "derived by le site wiki update from the separate wiki repository, which still names the retired programs"

// Its producer, internal/le/test/health render.go, already writes `le test
// health`; the page is stale until regenerated, and another session holds
// uncommitted edits to it.
// retiredTagRefusedWhy is the reason of the ownership test that feeds a
// retired build tag to prove it no longer exempts a root handler.
const retiredTagRefusedWhy = "a test sample that proves a retired build tag no longer exempts a root handler"

// retiredPerfPathWhy is the reason of the perf runner test that proves no
// command it issues names the retired perf binary.
const retiredPerfPathWhy = "a test that proves no perf runner command names the retired perf binary"

// retiredVariableIgnoredWhy is the reason shared by the tests that export a
// retired variable to prove nothing reads it.
const retiredVariableIgnoredWhy = "a test that exports the retired variable to prove nothing reads it"

// retiredHarnessRefusedWhy is the reason shared by the tests that feed the
// retired harness name to a matcher to prove it no longer matches.
const retiredHarnessRefusedWhy = "a test sample that proves the retired harness name no longer matches"

// retiredParityWhy is the reason shared by the tests that prove an le command
// does what the retired program did (AC-25).
const retiredParityWhy = "a parity test that names the retired program the le command replaced"

// retiredWord is one spelling that contains a retired name and names something
// else wherever it appears. The sweep blanks it out of a line before matching.
type retiredWord struct {
	match *regexp.Regexp
	why   string
}

// retiredWords are the declared spellings of AC-16 that are not file-bound.
var retiredWords = [...]retiredWord{
	{
		match: regexp.MustCompile(`(^|[\s` + "`" + `'"(])/ze-test(?:$|[^A-Za-z0-9_-])`),
		why:   "the /ze-test skill, not the harness binary",
	},
	{
		match: regexp.MustCompile(`ze-test-client(?:$|[^A-Za-z0-9_-])`),
		why:   "the EAP identity ze-test-client, not the harness binary",
	},
	{
		match: regexp.MustCompile(`ze-test-all(?:$|[^A-Za-z0-9_-])`),
		why:   "the retired make target ze-test-all, not the harness binary",
	},
	{
		match: regexp.MustCompile(`ze-perf-bench(?:$|[^A-Za-z0-9_-])`),
		why:   "the retired make target ze-perf-bench, not the perf program",
	},
	{
		match: regexp.MustCompile(`ze-chaos-mcp(?:$|[^A-Za-z0-9_-])`),
		why:   "the MCP server name ze-chaos-mcp that the chaos orchestrator reports, not the program",
	},
	{
		match: regexp.MustCompile(`(^|\s)package ze_test(?:$|\s)`),
		why:   "the external test package of pkg/ze, which Go names ze_test, not the build tag",
	},
}

// retiredMasked answers the line with every declared word blanked out, byte
// for byte, so a pattern cannot match inside one.
func retiredMasked(line string) string {
	for _, word := range retiredWords {
		line = word.match.ReplaceAllStringFunc(line, func(found string) string {
			return strings.Repeat(" ", len(found))
		})
	}
	return line
}

// retiredExcluded reports whether a tracked file is outside the sweep: the
// vendored tree, which Ze does not write, the declarations themselves, and the
// historical records.
func retiredExcluded(rel string) bool {
	if strings.HasPrefix(rel, "vendor/") {
		return true
	}
	if slices.Contains(retiredDeclarations[:], rel) {
		return true
	}
	for _, record := range &retiredRecords {
		if strings.HasSuffix(record.path, "/") {
			if strings.HasPrefix(rel, record.path) {
				return true
			}
			continue
		}
		if rel == record.path {
			return true
		}
	}
	return false
}

// retiredOwnsRows reports whether a tracked file is a spec under plan/, the
// only file whose owned journal rows section is a journal ownership table
// (plan/journal/README.md, "Owned rows").
func retiredOwnsRows(rel string) bool {
	if !strings.HasPrefix(rel, "plan/") {
		return false
	}
	base := rel[strings.LastIndexByte(rel, '/')+1:]
	if !strings.HasPrefix(base, "spec-") {
		return false
	}
	return strings.HasSuffix(base, ".md")
}

// retiredOwnedRowsAfter answers whether line sits inside a spec's owned
// journal rows section, given whether the line before it did. The section
// opens at its exact heading and runs to the next heading, as the journal
// owner check reads it. Its rows copy journal text verbatim, and plan/journal/
// is a historical record, so rewriting the copy would make it disagree with
// the row it names.
func retiredOwnedRowsAfter(inside bool, line string) bool {
	trimmed := strings.TrimRight(line, " \t\r")
	if trimmed == specjournal.OwnedRowsHeading {
		return true
	}
	if strings.HasPrefix(trimmed, "#") {
		return false
	}
	return inside
}

// retiredResealNotePrefix opens a reseal note: the comment that records why a
// sealed digest (internal/le/ai/hooks/fixtures.go, parity.go) was re-sealed.
const retiredResealNotePrefix = "// Re-sealed "

// retiredSealedDir is the package whose sources hold the sealed digests and
// their reseal notes. A reseal note is skipped only there: anywhere else a
// comment that happens to open with retiredResealNotePrefix is an ordinary
// comment, swept like every other line.
const retiredSealedDir = "internal/le/ai/hooks/"

// retiredSealed reports whether a tracked file is a source of the sealed
// package: a non-test Go file directly in retiredSealedDir. Its tests are not
// sealed, and neither is a package under it.
func retiredSealed(rel string) bool {
	name, found := strings.CutPrefix(rel, retiredSealedDir)
	if !found {
		return false
	}
	if strings.ContainsRune(name, '/') {
		return false
	}
	if strings.HasSuffix(name, "_test.go") {
		return false
	}
	return strings.HasSuffix(name, ".go")
}

// retiredResealNoteAfter answers whether line sits inside a reseal note in a
// sealed source (retiredSealed), given whether the line before it did. A note opens at a comment line
// that starts with retiredResealNotePrefix and runs through the comment lines
// after it, to a bare `//` line or the first line that is not a comment. A
// note records a past re-seal, so it names the spelling the source carried
// then: a historical record, like the journal. The digest values, the catalog
// rows and every other comment in the same file are still swept.
func retiredResealNoteAfter(inside bool, line string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, retiredResealNotePrefix) {
		return true
	}
	if trimmed == "//" {
		return false
	}
	if strings.HasPrefix(trimmed, "//") {
		return inside
	}
	return false
}

// retiredExempt reports whether one old spelling is declared legitimate on one
// line of one file: an exception of that file and spelling lists the line.
func retiredExempt(rel, old, line string) bool {
	for _, exception := range &retiredExceptions {
		if exception.file != rel {
			continue
		}
		if exception.old != old {
			continue
		}
		for _, covered := range exception.lines {
			if strings.Contains(line, covered) {
				return true
			}
		}
	}
	return false
}

// retiredPattern is one row of the rename map, compiled into what a tracked
// line would carry if it still named the row's old form.
type retiredPattern struct {
	kind        string
	old         string
	replacement string
	// needle is a normalized fragment every match contains. A file whose
	// normalized bytes lack it cannot match, so its lines are never scanned.
	needle string
	match  *regexp.Regexp
}

// normalizeRetired lowers a text and folds the three separators an env key is
// written with into one, so a single needle stands for every spelling. It works
// on bytes, so the answer has the length of the text and a line of one is at
// the same offsets in the other.
func normalizeRetired(text string) string {
	folded := []byte(text)
	for index, char := range folded {
		switch {
		case char == '-' || char == '_':
			folded[index] = '.'
		case char >= 'A' && char <= 'Z':
			folded[index] = char + ('a' - 'A')
		}
	}
	return string(folded)
}

// Word edges. A command word, a program and a file end at anything that cannot
// continue a name. A build tag ends at anything that cannot continue a Go
// identifier, so `ze_chaos_` metric names and the key spelling
// `ze_test_bgp_port` are not the tag.
const (
	edgeBefore     = `(?:^|[^A-Za-z0-9_])`
	edgeBeforeLe   = `(?:^|[^A-Za-z0-9_.-])`
	edgeAfterName  = `(?:$|[^A-Za-z0-9_-])`
	edgeAfterIdent = `(?:$|[^A-Za-z0-9_])`
)

// leCallee is a Go function named for le: `le`, a name that starts with `le`
// and an upper-case letter (`leSTERun`), or a name that carries `Le` or `LE`
// after a lower-case letter (`runLE`, `uiLeDocvalidAnswersRunCommand`). The
// ui fixtures run le through such helpers, and the helper supplies the
// program, so the argv it is handed starts at the command.
const leCallee = `(?:le|[A-Za-z0-9_]*[a-z](?:Le|LE))(?:[A-Z0-9_][A-Za-z0-9_]*)?\(`

// commandPattern compiles the forms a caller names a retired command in:
// `le <old>` after any path or program prefix (`./le`, `ze le`,
// `$CLAUDE_PROJECT_DIR/le`), and a Go argv whose command words are literals
// and whose program is the literal `"le"`, a variable named `le`
// (`run(ctx, le, "<w1>")`), or a helper named for le whose first literal
// argument is the command (`runLE(nil, "<w1>")`). A literal in any other call
// is not read as an argv: `filepath.Join(root, "qemu")` names a directory.
func commandPattern(row leretired.Rename) retiredPattern {
	old := row.Old()
	quoted := make([]string, 0, len(old))
	longest := ""
	for _, word := range old {
		quoted = append(quoted, regexp.QuoteMeta(word))
		if len(word) > len(longest) {
			longest = word
		}
	}

	var tb textbuf.Buffer
	argv := tb.Byte('"').Join(quoted, `",[ \t]*"`).Byte('"').String()
	tb.Reset()
	tb.Str(edgeBeforeLe).Str(`le[ \t]+`).Join(quoted, `[ \t]+`).Str(edgeAfterName).
		Byte('|').Str(edgeBefore).Str(`(?:"le"|le),[ \t]*`).Str(argv).
		Byte('|').Str(edgeBefore).Str(leCallee).Str(`(?:[^()"]*,[ \t]*)?`).Str(argv)
	expression := tb.String()
	tb.Reset()

	return retiredPattern{
		kind:        "command",
		old:         tb.Str("le ").Join(old, " ").String(),
		replacement: strings.Join(append([]string{"le"}, row.New()...), " "),
		needle:      normalizeRetired(longest),
		match:       regexp.MustCompile(expression),
	}
}

// namePattern compiles the form a retired program, build tag, file or
// variable is written in.
func namePattern(retired leretired.Retirement) retiredPattern {
	var tb textbuf.Buffer
	switch retired.Kind {
	case leretired.RetiredTag:
		tb.Str(edgeBefore).Str(regexp.QuoteMeta(retired.Old)).Str(edgeAfterIdent)
	case leretired.RetiredVariable:
		// A key is read as `ze.test.bin`, `ZE_TEST_BIN` or `ze-test-bin`, in
		// any case: the env registry treats them as one variable.
		parts := strings.Split(retired.Old, ".")
		for index, part := range parts {
			parts[index] = regexp.QuoteMeta(part)
		}
		tb.Str(`(?i)(?:^|[^a-z0-9])`).Join(parts, `[._-]`).Str(`(?:$|[^a-z0-9])`)
	case leretired.RetiredFile:
		// A name that ends in a separator is a prefix: `bin/ze-test-linux-`
		// takes every architecture after it.
		tb.Str(edgeBefore).Str(regexp.QuoteMeta(retired.Old))
		if !strings.HasSuffix(retired.Old, "-") {
			tb.Str(edgeAfterName)
		}
	case leretired.RetiredProgram:
		if retired.ProgramPositionOnly {
			programPosition(&tb, regexp.QuoteMeta(retired.Old))
			break
		}
		// A program name continues into its file names, `ze-test-linux-amd64`
		// among them, so only a letter or digit ends the match.
		tb.Str(edgeBefore).Str(regexp.QuoteMeta(retired.Old)).Str(edgeAfterIdent)
	case leretired.RetiredKindUnspecified:
		panic("BUG: doccheck.namePattern: a retired name declares no kind; see leretired.Retirements")
	default:
		panic("BUG: retired name has an unknown kind")
	}
	return retiredPattern{
		kind:        retired.Kind.String(),
		old:         retired.Old,
		replacement: retired.Replacement,
		needle:      normalizeRetired(retired.Old),
		match:       regexp.MustCompile(tb.String()),
	}
}

// programPosition writes the expression for a program name that is matched
// only where it is a program:
//
//   - run as the first word of a command: at the line start, after a shell
//     separator, after `$(` or a backtick, or as a `.ci` `exec=` value, with
//     any path before it;
//   - the first word of a quoted command string followed by an argument, as a
//     plugin `run "<name> fixture ..."` or a usage line carries it;
//   - a Go argv element followed by another literal argv element, as
//     `"<name>", "peer"` is passed to a process starter;
//   - built as `cmd/<name>`, shipped as `bin/<name>` or `<name>-linux-<arch>`,
//     or copied or run by a Dockerfile line;
//   - named in prose as the program: `<name>` followed by binary, harness,
//     personality, executable or runner.
//
// The same word as a whole string literal, a hostname in a config string, a
// NAS id or a module name is not the program. The variables the program reads
// are rows of their own.
func programPosition(tb *textbuf.Buffer, name string) {
	const (
		command = `(?:^[ \t]*(?:\$[ \t]+)?|[;|&(` + "`" + `][ \t]*|\$\([ \t]*|exec=)`
		path    = `(?:[^\s"'` + "`" + `;|&()]*/)?`
		argvEnd = `(?:$|[ \t"'` + "`" + `)])`
		quoted  = `["'][ \t]*`
		argWord = `[ \t]+[A-Za-z]`
		argvGo  = `",[ \t]*"`
		shipped = "(?:cmd|bin)/"
		docker  = `(?:^[ \t]*(?:COPY|RUN|ADD)[ \t](?:.*[^A-Za-z0-9_.-])?)`
		prose   = `[ \t]+(?:binary|binaries|harness|personality|executable|runner)\b`
	)
	tb.Str(command).Str(path).Str(name).Str(argvEnd).
		Byte('|').Str(quoted).Str(path).Str(name).Str(argWord).
		Byte('|').Byte('"').Str(name).Str(argvGo).
		Byte('|').Str(edgeBefore).Str(shipped).Str(name).Str(edgeAfterIdent).
		Byte('|').Str(edgeBefore).Str(name).Str("-linux-").
		Byte('|').Str(docker).Str(name).Str(edgeAfterIdent).
		Byte('|').Str(edgeBefore).Str(name).Str(prose)
}

// retiredPatterns compiles the whole rename map, commands first, in the order
// package leretired declares it.
func retiredPatterns() []retiredPattern {
	rows := leretired.Renames()
	names := leretired.Retirements()
	patterns := make([]retiredPattern, 0, len(rows)+len(names))
	for _, row := range rows {
		patterns = append(patterns, commandPattern(row))
	}
	for _, name := range names {
		patterns = append(patterns, namePattern(name))
	}
	return patterns
}

// RetiredMatch is one tracked line that names a retired form.
type RetiredMatch struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

// RetiredRow is one row of the rename map and every line that still names it.
type RetiredRow struct {
	Kind        string         `json:"kind"`
	Old         string         `json:"old"`
	Replacement string         `json:"replacement"`
	Matches     []RetiredMatch `json:"matches"`
}

// retiredReport is the answer of `le doc check retired-commands`: every
// row of the rename map, with the lines that still name it, and every cited
// le command le does not hold.
type retiredReport struct {
	Rows       []RetiredRow     `json:"rows"`
	Lines      int              `json:"lines"`
	Unheld     []UnheldCitation `json:"unheld"`
	Unreadable []string         `json:"unreadable,omitempty"`
}

// Text renders the rows that still have callers, then the count.
func (r retiredReport) Text() string {
	var out textbuf.Buffer
	for _, row := range r.Rows {
		if len(row.Matches) == 0 {
			continue
		}
		out.Str(row.Kind).Byte(' ').Str(row.Old).Str(" -> ").Str(row.Replacement).
			Str(": ").Int(int64(len(row.Matches))).Str(" line(s)\n")
		for _, match := range row.Matches {
			out.Str("  ").Str(match.File).Byte(':').Int(int64(match.Line)).Byte('\n')
		}
	}
	for _, unheld := range r.Unheld {
		out.Str("unheld ").Str(unheld.File).Byte(':').Int(int64(unheld.Line)).Str(": ").
			Str(unheld.Cited).Str(": ").Str(unheld.Reason).Byte('\n')
	}
	for _, rel := range r.Unreadable {
		out.Str("unreadable: ").Str(rel).Byte('\n')
	}
	out.Int(int64(r.Lines)).Str(" tracked line(s) name a retired form\n")
	out.Int(int64(len(r.Unheld))).Str(" cited le command(s) le does not hold\n")
	return out.String()
}

// sweepRetired reads every tracked file and answers, per rename-map row, the
// lines that name the row's old form, and every cited le command le does not
// hold. Both read the same lines, so the history records, the owned journal
// rows and the reseal notes skip a citation exactly as they skip an old form.
func sweepRetired(root string) (retiredReport, error) {
	files, err := trackedFiles(root)
	if err != nil {
		return retiredReport{}, err
	}
	patterns := retiredPatterns()
	report := retiredReport{Rows: make([]RetiredRow, len(patterns)), Unheld: []UnheldCitation{}}
	for index, pattern := range patterns {
		report.Rows[index] = RetiredRow{
			Kind: pattern.kind, Old: pattern.old, Replacement: pattern.replacement,
			Matches: []RetiredMatch{},
		}
	}

	candidates := make([]int, 0, len(patterns))
	unreadable, err := walkTracked(root, files, retiredExcluded, func(rel string, raw []byte) error {
		if slices.Contains(raw, 0) {
			return nil
		}
		text := string(raw)
		normalized := normalizeRetired(text)
		candidates = candidates[:0]
		for index, pattern := range patterns {
			if strings.Contains(normalized, pattern.needle) {
				candidates = append(candidates, index)
			}
		}
		// A needle is checked on the line before its expression runs: most
		// lines of a file that names an old form somewhere name none. Every
		// line is still read for a citation (commandCitationCandidate).
		ownsRows := retiredOwnsRows(rel)
		inOwnedRows := false
		sealed := retiredSealed(rel)
		inResealNote := false
		lineNo := 0
		for start := 0; start <= len(text); {
			end := strings.IndexByte(text[start:], '\n')
			if end < 0 {
				end = len(text)
			} else {
				end += start
			}
			lineNo++
			line, folded := text[start:end], normalized[start:end]
			start = end + 1
			if ownsRows {
				inOwnedRows = retiredOwnedRowsAfter(inOwnedRows, line)
				if inOwnedRows {
					continue
				}
			}
			if sealed {
				inResealNote = retiredResealNoteAfter(inResealNote, line)
				if inResealNote {
					continue
				}
			}
			report.Unheld = append(report.Unheld, unheldCitations(rel, lineNo, line)...)
			for _, index := range candidates {
				pattern := patterns[index]
				if !strings.Contains(folded, pattern.needle) {
					continue
				}
				if !pattern.match.MatchString(retiredMasked(line)) {
					continue
				}
				if retiredExempt(rel, pattern.old, line) {
					continue
				}
				row := &report.Rows[index]
				row.Matches = append(row.Matches, RetiredMatch{File: rel, Line: lineNo})
				report.Lines++
			}
		}
		return nil
	})
	if err != nil {
		return retiredReport{}, err
	}
	for _, file := range unreadable {
		var tb textbuf.Buffer
		report.Unreadable = append(report.Unreadable, tb.Str(file.rel).Str(": ").Err(file.err).String())
	}
	return report, nil
}

// runRetiredCheck is `le doc check retired-commands`, the gate AC-16 of
// spec-le-subject-first-command-tree and AC-25 of spec-le-cli-usability name.
// Every line that still names a retired form, or cites an le command le does
// not hold, makes it answer 1, and the report names each file and line. A file it could not read makes the answer incomplete, and that is said
// with 2 rather than hidden behind a verdict the sweep cannot support.
func runRetiredCheck(root string) (any, int) {
	report, err := sweepRetired(root)
	if err != nil {
		return errorReport{Error: err.Error()}, 2
	}
	if len(report.Unreadable) != 0 {
		return report, 2
	}
	if report.Lines != 0 {
		return report, 1
	}
	if len(report.Unheld) != 0 {
		return report, 1
	}
	return report, 0
}
