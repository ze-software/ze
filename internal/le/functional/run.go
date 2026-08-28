// Design: docs/architecture/testing/ci-format.md -- what a gating run is
// Detail: budget.go -- the cap each suite runs under
// Overview: binaries.go -- the set every suite in one run executes
//
// run.go builds one isolated set and runs each gating suite under its budget.
// It then evaluates the results and returns one report.
//
// Progress goes to stderr as it happens, while the report payload goes to stdout.
// The child inherits this process's streams for its own output.
// A reader can therefore watch a 600s suite while it runs.

package functional

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/lepath"
)

// coverageReduceTimeout bounds the go tool invocation for one suite.
const coverageReduceTimeout = 30 * time.Second

// killedByBudget is what `timeout` answers when it killed the command.
const killedByBudget = 124

// sessionScratch resolves this session's root-relative scratch path without a
// subprocess. Path lookup creates nothing.
func sessionScratch(root string) (string, error) {
	paths, err := lepath.ResolveSession(root, false)
	if err != nil {
		return "", err
	}
	return paths.Scratch, nil
}

// commandLine is the full argv one suite runs: the cap, then the binary, then
// the suite.
//
// `timeout` runs the suite in its own process group and signals the whole group
// on expiry, so leaked grandchildren (ze daemons, tacacs mocks) die with it.
func commandLine(suite Suite, set BinarySet) []string {
	argv := suite.Command()
	argv[0] = set.zeTestPath()

	var tb textbuf.Buffer
	full := make([]string, 0, len(argv)+3)
	full = append(full, "timeout", tb.Str("--kill-after=").Str(KillAfter()).String(), suite.Budget())
	return append(full, argv...)
}

// Execute runs one suite and answers its exit status and how long it took.
func Execute(tc gotoolchain.Toolchain, suite Suite, set BinarySet, cover string) (code, seconds int) {
	environ := set.Environment(tc)
	if cover != "" {
		if err := os.MkdirAll(cover, 0o750); err != nil {
			var tb textbuf.Buffer
			gaterun.Note(tb.Str("could not create ").Str(cover).Str(": ").Err(err).String())
		}
		var cov textbuf.Buffer
		environ = append(environ, cov.Str("GOCOVERDIR=").Str(cover).String())
	}
	started := time.Now()
	code = gaterun.Stream(commandLine(suite, set), tc.Root, environ)
	return code, int(time.Since(started).Seconds())
}

// failureGroupLine is the declared failure group a cap expiry publishes.
//
// A budget expiry has already consumed the full run budget.
// Its failure group therefore includes the same rerun command as an ordinary suite failure.
// functionalSuiteRerun defines that command (internal/le/verify/run.go).
// The failure kind is timeout rather than a failed test.
func failureGroupLine(suite Suite, summary string) string {
	group := map[string]any{
		"group-id": groupID(suite.Name),
		"kind":     "timeout",
		"related":  []string{suite.Name},
		"summary":  summary,
		"rerun":    suite.Rerun(),
		"parallel": "stage",
	}
	raw, err := json.Marshal(group)
	if err != nil {
		// Unreachable: every value above is a string or a []string. Reporting
		// it rather than dropping the line keeps a producer that declared a
		// group from silently declaring none.
		var tb textbuf.Buffer
		return tb.Str("{\"stage\":").Quoted(suite.Name).Str(",\"kind\":\"timeout\"}").String()
	}
	return string(raw)
}

// groupID names the failure group a budget expiry belongs to.
func groupID(suite string) string {
	var tb textbuf.Buffer
	return tb.Str("suite-budget:").Str(suite).String()
}

// Skipped answers the suites ZE_SKIP_SUITES asks the gating run to leave out.
func Skipped() map[string]bool {
	out := map[string]bool{}
	for name := range strings.SplitSeq(env.Get("ze.skip.suites"), ",") {
		if name != "" {
			out[name] = true
		}
	}
	return out
}

var _ = env.MustRegister(env.EnvEntry{
	Key:         "ze.skip.suites",
	Type:        envString,
	Default:     "",
	Description: "comma-separated suite names the gating run leaves out, for a host missing their tooling",
	Private:     true,
})

// Run is what a gating run has seen so far, and the report it prints as it
// goes.
//
// A test can supply a status and duration to this value, then inspect the resulting report.
type Run struct {
	report GatingReport
	index  int
}

// NewRun starts an accounting over total suites, which is the denominator every
// progress line reads.
func NewRun(total int) *Run {
	return &Run{report: GatingReport{
		SuiteTotal:    total,
		DefaultBudget: sharedBudget(),
		WarnPercent:   WarnPercent(),
		Runtimes:      []string{},
		FailedNames:   []string{},
		ExpiredNames:  []string{},
		WarnedNames:   []string{},
		SkippedNames:  []string{},
	}}
}

// sharedBudget is the cap the closing report names, which is the one that
// governs every suite with no budget of its own.
func sharedBudget() string {
	if set := env.Get(envKey(sharedBudgetVar)); set != "" {
		return set
	}
	return DefaultBudget
}

// Report answers what the run has judged so far.
func (r *Run) Report() GatingReport { return r.report }

// Skip records a suite ZE_SKIP_SUITES left out.
func (r *Run) Skip(suite Suite) {
	r.report.SkippedNames = append(r.report.SkippedNames, suite.Name)
}

// Announce prints the progress line, before the suite runs.
func (r *Run) Announce(suite Suite) {
	r.report.Ran++
	r.index++
	var tb textbuf.Buffer
	gaterun.Note("")
	gaterun.Note(tb.Byte('[').Int(int64(r.index)).Byte('/').Int(int64(r.report.SuiteTotal)).
		Str("] suite ").Str(suite.Name).String())
}

// Record judges one finished suite: its runtime, its budget, and its verdict.
//
// A kill is reported only as a kill.
// Every killed suite consumes 100% of its budget, so a separate warning would hide the kill line.
func (r *Run) Record(suite Suite, seconds, status int) {
	budget := suite.Budget()
	allowed := DurationSeconds(budget)
	percent := 0

	var tb textbuf.Buffer
	if allowed > 0 {
		percent = seconds * 100 / allowed
		gaterun.Note(tb.Str("      suite ").Str(suite.Name).Str(" took ").Int(int64(seconds)).
			Str("s of its ").Str(budget).Str(" budget (").Int(int64(percent)).Str("%)").String())
		tb.Reset()
		r.report.Runtimes = append(r.report.Runtimes, tb.Str("  ").Str(suite.Name).Byte(' ').
			Int(int64(seconds)).Str("s of ").Str(budget).Str(" (").Int(int64(percent)).Str("%)").String())
	} else {
		gaterun.Note(tb.Str("      suite ").Str(suite.Name).Str(" took ").Int(int64(seconds)).
			Str("s (budget ").Str(budget).
			Str(" is not a duration this report can measure against)").String())
		tb.Reset()
		r.report.Runtimes = append(r.report.Runtimes, tb.Str("  ").Str(suite.Name).Byte(' ').
			Int(int64(seconds)).Str("s of ").Str(budget).Str(" (unmeasurable budget)").String())
	}

	switch {
	case status == killedByBudget:
		summary := expirySummary(suite, budget)
		tb.Reset()
		tb.SetColor(true)
		gaterun.Note(tb.Colored(textbuf.C.BoldRed).Str("BUDGET EXPIRED  ").Str(summary).
			Str(". The test failures above are that kill, not the product.").
			Colored(textbuf.C.Reset).String())
		tb.Reset()
		gaterun.Note(tb.Str("VERIFY FAILURE GROUP: ").Str(failureGroupLine(suite, summary)).String())
		r.report.ExpiredNames = append(r.report.ExpiredNames, suite.Name)
	case allowed > 0 && percent >= r.report.WarnPercent:
		tb.Reset()
		tb.SetColor(true)
		gaterun.Note(tb.Colored(textbuf.C.BrightYellow).Str("BUDGET WARNING  suite ").Str(suite.Name).
			Str(" used ").Int(int64(percent)).Str("% of its ").Str(budget).
			Str(" budget, and the warning level is ").Int(int64(r.report.WarnPercent)).
			Str("%. Make the suite faster or raise ").Str(suite.BudgetVar()).
			Str(" before it becomes a kill.").Colored(textbuf.C.Reset).String())
		r.report.WarnedNames = append(r.report.WarnedNames, suite.Name)
	}

	if status != 0 {
		r.report.FailedNames = append(r.report.FailedNames, suite.Name)
	}
}

// expirySummary is the sentence a kill is reported with, in the report and in
// the declared failure group alike, so the two cannot disagree.
func expirySummary(suite Suite, budget string) string {
	var tb textbuf.Buffer
	return tb.Str("suite ").Str(suite.Name).Str(" reached its ").Str(budget).
		Str(" wall-clock budget (").Str(suite.BudgetVar()).Str(") and was killed").String()
}

// ciGoTestPackages derives every Go package that a .ci command compiles inside
// its own deadline. The retired Make prerequisite derived the same set from
// `cmd=...:exec=go test ...` lines. Keeping the derivation here prevents a new
// fixture from gaining a cold compile without joining the warmup.
func ciGoTestPackages(root string) ([]string, error) {
	packages := map[string]bool{}
	testRoot := filepath.Join(root, "test")
	err := filepath.WalkDir(testRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".ci" {
			return nil
		}
		raw, err := os.ReadFile(path) //nolint:gosec // the repository tool reads its own test corpus
		if err != nil {
			return err
		}
		for line := range strings.SplitSeq(string(raw), "\n") {
			command, found := strings.CutPrefix(strings.TrimSpace(line), "cmd=")
			if !found {
				continue
			}
			_, command, found = strings.Cut(command, "exec=go test ")
			if !found {
				continue
			}
			for _, token := range strings.Fields(command) {
				token, _, _ = strings.Cut(strings.Trim(token, `"'`), ":")
				if strings.HasPrefix(token, "./") {
					packages[token] = true
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(packages) == 0 {
		return nil, errors.New("no .ci-invoked Go package was found")
	}
	out := make([]string, 0, len(packages))
	for name := range packages {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

// warmCITestPackages removes compilation from each fixture's behavioral
// deadline. It intentionally uses the bare tag set that the .ci commands use;
// a tagged build would populate a different Go cache entry.
func warmCITestPackages(tc gotoolchain.Toolchain) error {
	packages, err := ciGoTestPackages(tc.Root)
	if err != nil {
		return fmt.Errorf("derive .ci Go packages: %w", err)
	}
	var tb textbuf.Buffer
	gaterun.Note(tb.Str("Warming build cache for ").Int(int64(len(packages))).
		Str(" .ci-invoked package(s)...").String())
	argv := append([]string{"go", "test", "-run", "^$", "-count=1"}, packages...)
	if code := gaterun.Stream(argv, tc.Root, tc.Environment(gotoolchain.EnvOptions{Procs: true})); code != 0 {
		return fmt.Errorf("warm .ci Go packages: exit %d", code)
	}
	return nil
}

// runGating runs every gating suite, in order, under its own budget.
//
// One isolated binary set serves the whole run, built once and removed however
// the run ends.
//
// The payload is nil when the run never started.
// A zero GatingReport is not an empty run because its Text renders "PASS all 0 suites" in green.
// Returning it after a refused run list or failed build would falsely report a pass.
func runGating(tc gotoolchain.Toolchain) (any, int) {
	suites, err := GatingSuites(Gating, Suites)
	if err != nil {
		gaterun.Note(reportLine(err))
		return nil, 1
	}

	skip := Skipped()
	total := 0
	for _, suite := range suites {
		if !skip[suite.Name] {
			total++
		}
	}
	run := NewRun(total)

	if err := warmCITestPackages(tc); err != nil {
		gaterun.Note(reportLine(err))
		return nil, 1
	}
	set, err := Prepare(tc, "functional", true)
	if err != nil {
		gaterun.Note(reportLine(err))
		return nil, 1
	}

	coverRoot, err := coverRoot(tc.Root)
	if err != nil {
		gaterun.Note(reportLine(err))
		return nil, 1
	}
	for _, suite := range suites {
		if skip[suite.Name] {
			continue
		}
		run.Announce(suite)
		cover := ""
		if coverRoot != "" {
			cover = filepath.Join(coverRoot, suite.Name)
			removeTree(cover)
		}
		code, seconds := Execute(tc, suite, set, cover)
		run.Record(suite, seconds, code)
		if cover != "" {
			reduceCoverage(tc, suite, cover, coverRoot)
		}
	}

	report := run.Report()
	if len(report.FailedNames) > 0 {
		return report, 1
	}
	return report, 0
}

// reportLine spells one failure the way every ported le tool spells it.
func reportLine(err error) string {
	var tb textbuf.Buffer
	return tb.Str("error: ").Err(err).String()
}

// reduceCoverage reduces one suite's raw coverage directory to the packages it
// reached.
func reduceCoverage(tc gotoolchain.Toolchain, suite Suite, cover, root string) {
	files, bytes := treeSize(cover)
	var tb textbuf.Buffer
	appendLine(filepath.Join(root, "raw-size.txt"),
		tb.Str(suite.Name).Byte(' ').Int(int64(files)).Byte(' ').Int(bytes/1024).Byte('\n').String())

	ctx, cancel := context.WithTimeout(context.Background(), coverageReduceTimeout)
	defer cancel()
	tb.Reset()
	//nolint:gosec // the go tool, over a directory this run created
	cmd := exec.CommandContext(ctx, "go", "tool", "covdata", "percent", tb.Str("-i=").Str(cover).String())
	cmd.Dir = tc.Root
	cmd.Env = tc.Environment(gotoolchain.EnvOptions{})
	out, err := cmd.CombinedOutput()

	tb.Reset()
	writeFile(filepath.Join(root, tb.Str(suite.Name).Str(".percent").String()), out)
	if err != nil {
		tb.Reset()
		gaterun.Note(tb.Str("covdata percent failed for suite ").Str(suite.Name).String())
	}
	removeTree(cover)
}

// treeSize answers how many files a directory holds and how many bytes they
// take, which is the coverage run's own size record.
func treeSize(dir string) (files int, bytes int64) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0
	}
	for _, entry := range entries {
		if entry.IsDir() {
			subFiles, subBytes := treeSize(filepath.Join(dir, entry.Name()))
			files += subFiles
			bytes += subBytes
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files++
		bytes += info.Size()
	}
	return files, bytes
}

// appendLine adds one line to a record file, and says so when it cannot.
func appendLine(path, line string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // a path this run derived
	if err != nil {
		gaterun.Note(reportLine(err))
		return
	}
	defer f.Close() //nolint:errcheck // the write below is what matters
	if _, err := f.WriteString(line); err != nil {
		gaterun.Note(reportLine(err))
	}
}

// writeFile records one coverage answer, and says so when it cannot.
func writeFile(path string, content []byte) {
	if err := os.WriteFile(path, content, 0o600); err != nil {
		gaterun.Note(reportLine(err))
	}
}
