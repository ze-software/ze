// Design: docs/architecture/core-design.md -- native documentation verifier actions
// Related: repository.go -- the tracked-file walk this sweep shares with links
// Related: retirednames.go -- the rename map the sweep reads
//
// The retired-name sweep finds every tracked line that still names a form the
// subject-first rename retires (plan/spec-le-subject-first-command-tree.md).
// It reads the one rename map retirednames.go declares and holds no list of its
// own. It is a gate: a line that names an old form outside the declared
// records and exceptions makes it exit 1, and it runs as a stage of the full
// verify (internal/le/verify/engine/stages.go).

package doccheck

import (
	"regexp"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// retiredDeclarations are the files that declare the rename map, the sweep and
// their tests. They name every old form by necessity, so they are not callers.
// The two leroot tests name old forms to prove each one answers `unknown
// command`.
var retiredDeclarations = [...]string{
	"internal/le/doc/check/retirednames.go",
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
	{path: "test/weakened/", why: "weakening records quote the commit and the command they judged"},
	{path: "website/blog/", why: "published posts are the owner's authored prose"},
	{path: "website/changes/posts/", why: "published change posts are the owner's authored prose"},
	{path: "website/changes/discord/2026-08-03-weekly.md", why: "a published weekly update"},
	{path: "website/talks/", why: "published talks record the commands of their day"},
	{path: "plan/known-failures/", why: "known-failure shards record the command that failed on the day it failed"},
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
		path: "plan/spec-le-subject-first-command-tree.md",
		why:  "the spec of this rename names every old form in order to retire it",
	},
}

// retiredException is one declared use of an old spelling that names
// something else: a word that happens to be spelled as a retired name.
type retiredException struct {
	file string
	old  string
	why  string
}

// retiredExceptions are the declared exceptions of AC-16. Each names the file
// and the old spelling it may keep, and why the spelling is not the retired
// name.
// discordChannelWhy is the reason shared by the files that name the Discord
// channel.
const discordChannelWhy = "the Discord channel named ze-test, not the harness binary"

// retiredHookNegativeWhy is the reason of the hook test that proves a text
// search naming a retired harness binary is not refused as a raw test run.
const retiredHookNegativeWhy = "the hook test feeds the retired name to prove the deleted harness admission refuses nothing"

var retiredExceptions = [...]retiredException{
	{file: "internal/le/hookruntime/bash_test.go", old: "ze-test", why: retiredHookNegativeWhy},
	{file: "internal/le/hookruntime/bash_test.go", old: "le-test", why: retiredHookNegativeWhy},
	{
		file: "internal/le/weekly/answer.go",
		old:  "ze-test",
		why:  "the publication channel named ze-test, not the harness binary",
	},
	{
		file: "internal/component/firewall/validate_test.go",
		old:  "ze_test",
		why:  "the nftables table named ze_test, not the build tag",
	},
	{
		file: "docs/architecture/testing/qemu-integration.md",
		old:  "ze_test",
		why:  "the nftables table named ze_test, not the build tag",
	},
	{
		file: "internal/component/bgp/reactor/filter_flowspec.go",
		old:  "le rules",
		why:  "the FlowSpec prefix operators ge/le, not the le command",
	},
	{
		file: "ai/skills/ze-weekly-update.md",
		old:  "ze-test",
		why:  discordChannelWhy,
	},
	{
		file: ".claude/memory/reference_discord_bot.md",
		old:  "ze-test",
		why:  discordChannelWhy,
	},
	{
		file: "website/changes/discord/STYLE.md",
		old:  "ze-test",
		why:  discordChannelWhy,
	},
	{
		file: "ai/skills/ze-test.md",
		old:  "ze-test",
		why:  "the skill named ze-test, not the harness binary",
	},
	{
		file: retiredDebtFile,
		old:  "le repository tracked-build",
		why:  "the command text of debt rows written before the rename, which the ledger still reads",
	},
	{
		file: retiredDebtFile,
		old:  "le repository",
		why:  "the command text of debt rows written before the rename, which the ledger still reads",
	},
	{
		file: retiredLedgerTestFile,
		old:  "le repository tracked-build",
		why:  "a debt row written before the rename, to prove the ledger still reads it",
	},
	{
		file: retiredLedgerTestFile,
		old:  "le repository",
		why:  "a debt row written before the rename, to prove the ledger still reads it",
	},
	{
		file: "internal/le/le/action/leaction_test.go",
		old:  "le qemu",
		why:  "the usage line of a fixture area named qemu, not the qemu command",
	},
	{
		file: "internal/le/site/terminaldemo/terminaldemo_test.go",
		old:  "ze-terminal-pty",
		why:  "a test that proves the demo build no longer builds or runs the retired recorder",
	},
	{
		file: retiredGuestLeTestFile,
		old:  retiredLeTestBin,
		why:  retiredVariableIgnoredWhy,
	},
	{
		file: retiredGuestLeTestFile,
		old:  retiredZeTestBin,
		why:  retiredVariableIgnoredWhy,
	},
	{
		file: retiredGuestLeTestFile,
		old:  "le.qemu.test.bin",
		why:  retiredVariableIgnoredWhy,
	},
	{
		file: retiredGuestLeTestFile,
		old:  "ze.qemu.test.bin",
		why:  retiredVariableIgnoredWhy,
	},
	{
		file: retiredQEMUKnobsTestFile,
		old:  retiredLeTestBin,
		why:  retiredVariableIgnoredWhy,
	},
	{
		file: retiredQEMUKnobsTestFile,
		old:  retiredZeTestBin,
		why:  retiredVariableIgnoredWhy,
	},
	{
		file: retiredStressReproTestFile,
		old:  retiredLeTestBin,
		why:  retiredVariableIgnoredWhy,
	},
	{
		file: retiredStressReproTestFile,
		old:  retiredZeTestBin,
		why:  retiredVariableIgnoredWhy,
	},
	{
		file: "internal/le/test/functional/exabgp_test.go",
		old:  retiredLeTestBin,
		why:  retiredVariableIgnoredWhy,
	},
	{
		file: retiredHostLoadTestFile,
		old:  "le-test",
		why:  retiredHarnessRefusedWhy,
	},
	{
		file: retiredHostLoadTestFile,
		old:  "bin/le-test",
		why:  retiredHarnessRefusedWhy,
	},
	{
		file: "cmd/ze/login_test.go",
		old:  "le-test",
		why:  retiredHarnessRefusedWhy,
	},
	{
		file: retiredPerfTestFile,
		old:  "ze-perf",
		why:  retiredParityWhy,
	},
	{
		file: retiredPerfTestFile,
		old:  "ze-perf-run",
		why:  retiredParityWhy,
	},
	{
		file: "internal/le/perf/bench_test.go",
		old:  "ze-perf",
		why:  retiredParityWhy,
	},
	{
		file: "internal/le/mrt/mrt_test.go",
		old:  "ze-analyze",
		why:  retiredParityWhy,
	},
	{
		file: "internal/le/chaos/run/run_test.go",
		old:  "ze-chaos",
		why:  retiredParityWhy,
	},
}

// Files that declare more than one exception, and the variable names that
// several of them declare.
const (
	retiredLeTestBin           = "le.test.bin"
	retiredZeTestBin           = "ze.test.bin"
	retiredDebtFile            = "internal/le/commit/debt.go"
	retiredLedgerTestFile      = "internal/le/commit/ledger_test.go"
	retiredGuestLeTestFile     = "internal/le/test/qemu/guestle_run_test.go"
	retiredQEMUKnobsTestFile   = "internal/le/test/qemu/alltests_knobs_test.go"
	retiredStressReproTestFile = "internal/le/test/stressrepro/stressrepro_test.go"
	retiredHostLoadTestFile    = "internal/core/hostload/hostload_test.go"
	retiredPerfTestFile        = "internal/le/perf/perf_test.go"
)

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
	for _, record := range retiredRecords {
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

// retiredExempt reports whether one old spelling is declared legitimate in one
// file.
func retiredExempt(rel, old string) bool {
	for _, exception := range retiredExceptions {
		if exception.file != rel {
			continue
		}
		if exception.old == old {
			return true
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

// commandPattern compiles the forms a caller names a retired command in:
// `le <old>` after any path or program prefix (`./le`, `ze le`,
// `$CLAUDE_PROJECT_DIR/le`), and a Go argv built from literals, `"le", "<w1>"`.
func commandPattern(row Rename) retiredPattern {
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
	tb.Str(edgeBeforeLe).Str(`le[ \t]+`).Join(quoted, `[ \t]+`).Str(edgeAfterName).
		Byte('|').Str(`"le",[ \t]*"`).Join(quoted, `",[ \t]*"`).Byte('"')
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
func namePattern(retired Retirement) retiredPattern {
	var tb textbuf.Buffer
	switch retired.Kind {
	case RetiredTag:
		tb.Str(edgeBefore).Str(regexp.QuoteMeta(retired.Old)).Str(edgeAfterIdent)
	case RetiredVariable:
		// A key is read as `ze.test.bin`, `ZE_TEST_BIN` or `ze-test-bin`, in
		// any case: the env registry treats them as one variable.
		parts := strings.Split(retired.Old, ".")
		for index, part := range parts {
			parts[index] = regexp.QuoteMeta(part)
		}
		tb.Str(`(?i)(?:^|[^a-z0-9])`).Join(parts, `[._-]`).Str(`(?:$|[^a-z0-9])`)
	case RetiredFile:
		// A name that ends in a separator is a prefix: `bin/ze-test-linux-`
		// takes every architecture after it.
		tb.Str(edgeBefore).Str(regexp.QuoteMeta(retired.Old))
		if !strings.HasSuffix(retired.Old, "-") {
			tb.Str(edgeAfterName)
		}
	case RetiredProgram:
		if retired.ProgramPositionOnly {
			programPosition(&tb, regexp.QuoteMeta(retired.Old))
			break
		}
		// A program name continues into its file names, `ze-test-linux-amd64`
		// among them, so only a letter or digit ends the match.
		tb.Str(edgeBefore).Str(regexp.QuoteMeta(retired.Old)).Str(edgeAfterIdent)
	case RetiredKindUnspecified:
		panic("BUG: doccheck.namePattern: a retired name declares no kind; see retirements")
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
// retirednames.go declares it.
func retiredPatterns() []retiredPattern {
	rows := Renames()
	names := Retirements()
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

// RetiredReport is the answer of `le doc check retired-commands`: every
// row of the rename map, with the lines that still name it.
type RetiredReport struct {
	Rows       []RetiredRow `json:"rows"`
	Lines      int          `json:"lines"`
	Unreadable []string     `json:"unreadable,omitempty"`
}

// Text renders the rows that still have callers, then the count.
func (r RetiredReport) Text() string {
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
	for _, rel := range r.Unreadable {
		out.Str("unreadable: ").Str(rel).Byte('\n')
	}
	out.Int(int64(r.Lines)).Str(" tracked line(s) name a retired form\n")
	return out.String()
}

// sweepRetired reads every tracked file and answers, per rename-map row, the
// lines that name the row's old form.
func sweepRetired(root string) (RetiredReport, error) {
	files, err := trackedFiles(root)
	if err != nil {
		return RetiredReport{}, err
	}
	patterns := retiredPatterns()
	report := RetiredReport{Rows: make([]RetiredRow, len(patterns))}
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
		if len(candidates) == 0 {
			return nil
		}
		// A needle is checked on the line before its expression runs: most
		// lines of a file that names an old form somewhere name none.
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
			for _, index := range candidates {
				pattern := patterns[index]
				if !strings.Contains(folded, pattern.needle) {
					continue
				}
				if !pattern.match.MatchString(retiredMasked(line)) {
					continue
				}
				if retiredExempt(rel, pattern.old) {
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
		return RetiredReport{}, err
	}
	for _, file := range unreadable {
		var tb textbuf.Buffer
		report.Unreadable = append(report.Unreadable, tb.Str(file.rel).Str(": ").Err(file.err).String())
	}
	return report, nil
}

// runRetiredCheck is `le doc check retired-commands`, the gate AC-16 of
// plan/spec-le-subject-first-command-tree.md names. Every line that still
// names a retired form makes it answer 1, and the report names each file and
// line. A file it could not read makes the answer incomplete, and that is said
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
	return report, 0
}
