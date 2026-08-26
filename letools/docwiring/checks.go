// Design: docs/architecture/core-design.md -- the checks this gate runs itself
// Overview: docwiring.go -- the gate that runs them
//
// checks.go holds the checks that no other Make target owns. Each check reads a
// few files and answers in milliseconds. Thus, the router runs them directly.
//
// Every check REPORTS an unreadable file instead of ignoring it. A ratchet that
// counts only readable files reports a lower value for an incomplete tree. A
// lower value looks like a pass.

package docwiring

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// SleepBaseline is the committed delta ledger the sleep ratchet reads.
	SleepBaseline = "test/.ci-sleep-baseline"

	// DraftDir holds functional tests under development. It is gitignored and
	// invisible to every repository-wide gate, so a draft cannot move a ratchet
	// or redden any other check.
	DraftDir = "draft"

	// knownFailuresDir holds the shards that cannot contain a load excuse.
	knownFailuresDir = "plan/known-failures/"

	// designRefTimeout bounds the tree-wide design-reference checker. It walks
	// every source file, which is seconds on this tree.
	designRefTimeout = 10 * time.Minute
)

// knownFailuresExempt are the two shard files this check does not read.
// The first states the policy and the second is a verbatim archive of history
// that must not be edited to satisfy a present-day gate.
var knownFailuresExempt = map[string]bool{"README.md": true, "RESOLVED.md": true}

var (
	sleepRe      = regexp.MustCompile(`time\.sleep\(`)
	signedIntRe  = regexp.MustCompile(`^[+-]?\d+$`)
	loadExcuseRe = regexp.MustCompile(`(?i)under load|loaded host|load average|load[- ]sensitive` +
		`|pass(?:es|ed)? in isolation|resource contention|contended host`)
	ciLogKeyRe = regexp.MustCompile(`ze\.log\.([A-Za-z0-9._-]+)`)
	// hyphenLiteralRe reads every hyphen-bearing double-quoted Go string
	// literal, which is the only form a subsystem name reaches the code in.
	hyphenLiteralRe = regexp.MustCompile(`"([a-z0-9]{1,}(?:[.-][a-z0-9]{1,})*-[a-z0-9]{1,}(?:[.-][a-z0-9]{1,})*)"`)
)

// goSourceRoots are the trees a subsystem name can be declared in.
var goSourceRoots = [...]string{"internal", "pkg", "cmd"}

// ParseSleepBaseline answers the ceiling in the committed delta ledger. It sums
// signed-integer lines and ignores comments and blanks. The second result is
// false when no integer line exists, which leaves the ratchet inactive.
//
// The delta form replaces one absolute integer. Two independent sleep removals
// can append separate `-N` lines instead of conflicting on one number. A plain
// integer still parses as one line and one summand. A `+N` line explicitly
// raises the ceiling.
func ParseSleepBaseline(text string) (int, bool) {
	total := 0
	seen := false
	for line := range strings.SplitSeq(text, "\n") {
		stripped := strings.TrimSpace(line)
		if stripped == "" || strings.HasPrefix(stripped, "#") {
			continue
		}
		if !signedIntRe.MatchString(stripped) {
			continue
		}
		value, err := strconv.Atoi(stripped)
		if err != nil {
			continue
		}
		total += value
		seen = true
	}
	return total, seen
}

// realCIFiles answers every .ci test under test/ except the draft incubator. A
// draft with an unjustified sleep would otherwise exceed the committed ceiling.
// The next verify run would then fail, even for an unrelated session.
func realCIFiles(root string) ([]string, error) {
	base := filepath.Join(root, "test")
	if !isDir(base) {
		// A tree with no functional tests holds no sleep to count. That is a
		// fact about the tree rather than a walk that fell short.
		return nil, nil
	}

	var files []string
	err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walking %s: %w", path, err)
		}
		if entry.IsDir() {
			if path != base && entry.Name() == DraftDir && filepath.Dir(path) == base {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".ci") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// checkSleepRatchet caps how MANY sleeps the functional tests hold.
//
// Sleeps in embedded observers hide real races, and the test API provides
// deterministic waits. Legacy sleeps are tolerated at the committed baseline;
// new ones fail the gate.
func (g *gate) checkSleepRatchet() CheckResult {
	if !anyChangedCI(g.report.Changed) {
		return CheckResult{Skipped: true}
	}

	raw, err := os.ReadFile(filepath.Join(g.root, filepath.FromSlash(SleepBaseline))) //nolint:gosec // the baseline of the tree the caller named
	switch {
	case os.IsNotExist(err):
		// No baseline committed: the ratchet is not active for this tree, which
		// is a fact about the tree.
		return CheckResult{Skipped: true}
	case err != nil:
		// A present unreadable baseline creates the same fail-open state as an
		// unreadable test. The ratchet stops, and any number of new sleeps can
		// pass.
		return g.readFailure(checkSleepRatchetName, err)
	}
	ceiling, active := ParseSleepBaseline(string(raw))
	if !active {
		return CheckResult{Skipped: true}
	}

	files, err := realCIFiles(g.root)
	if err != nil {
		return g.readFailure(checkSleepRatchetName, err)
	}
	count := 0
	for _, path := range files {
		text, err := os.ReadFile(path) //nolint:gosec // a .ci test of the tree the caller named
		if err != nil {
			// A file this ratchet cannot read contributes no sleep, and fewer
			// sleeps is what passing looks like. Reporting it is the whole
			// difference between a ratchet and a count of what happened to open.
			return g.readFailure(checkSleepRatchetName, err)
		}
		count += len(sleepRe.FindAllString(string(text), -1))
	}

	var tb textbuf.Buffer
	switch {
	case count > ceiling:
		// No single file is the offender. The count is a sum over all .ci files
		// in the tree against one committed ceiling. The group names no file, so
		// the commit gate charges it to the committing session.
		g.declareFailureGroup(checkSleepRatchetName, nil,
			tb.Str("test/**/*.ci holds ").Int(int64(count)).
				Str(" time.sleep( calls against a ceiling of ").Int(int64(ceiling)).String(), gateRerun)
		tb.Reset()
		return CheckResult{
			Failed:  true,
			Message: "ci-sleep ratchet FAILED:",
			Violations: []string{tb.Str("test/**/*.ci now contains ").Int(int64(count)).
				Str(" time.sleep( calls; the committed delta baseline (").Str(SleepBaseline).
				Str(") allows ").Int(int64(ceiling)).Byte('.').String()},
		}
	case count < ceiling:
		return CheckResult{Message: tb.Str("ci-sleep ratchet: count dropped to ").Int(int64(count)).
			Str(" (ceiling ").Int(int64(ceiling)).Str("); append a -").Int(int64(ceiling - count)).
			Str(" delta line to ").Str(SleepBaseline).Str(" in this change to tighten it.").String()}
	default:
		return CheckResult{Message: tb.Str("ci-sleep ratchet OK (").Int(int64(count)).
			Str(" <= ceiling ").Int(int64(ceiling)).Byte(')').String()}
	}
}

// checkSleepJustification caps how many sleeps are UNEXPLAINED.
//
// The ratchet caps the total sleeps. This check caps sleeps without a reason.
// A blind sleep hides why it remains. A comment on or above each sleep makes
// that reason auditable. The check covers changed files because a session owns
// the tests that it changes.
func (g *gate) checkSleepJustification() CheckResult {
	changed := changedCI(g.report.Changed)
	if len(changed) == 0 {
		return CheckResult{Skipped: true}
	}

	var violations []string
	offenders := make(map[string]bool)
	checked := 0
	var tb textbuf.Buffer
	for _, rel := range changed {
		lines, missing, err := readLines(g.root, rel)
		if err != nil {
			return g.readFailure(checkSleepJustificationName, err)
		}
		if missing {
			continue // deleting a test is the intended outcome, not a violation
		}
		for i, line := range lines {
			if !sleepRe.MatchString(line) {
				continue
			}
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue // the sleep is itself commented out; nothing to justify
			}
			checked++
			if sleepIsJustified(lines, i) {
				continue
			}
			violations = append(violations, tb.Reset().Str(rel).Byte(':').Int(int64(i+1)).
				Str(": ").Str(strings.TrimSpace(line)).String())
			offenders[rel] = true
		}
	}

	if len(violations) > 0 {
		g.declareFailureGroup(checkSleepJustificationName, sortedKeys(offenders),
			"a changed .ci test holds a time.sleep( with no comment saying why", gateRerun)
		return CheckResult{Failed: true, Message: "ci-sleep justification FAILED:", Violations: violations}
	}
	if checked == 0 {
		return CheckResult{Skipped: true}
	}
	return CheckResult{Message: tb.Reset().Str("ci-sleep justification OK (").Int(int64(checked)).
		Str(" sleeps, all commented)").String()}
}

// sleepIsJustified reports whether line idx has an explanatory sleep comment.
// The comment can trail the call or be the nearest preceding non-blank line.
func sleepIsJustified(lines []string, idx int) bool {
	_, after, found := strings.Cut(lines[idx], "time.sleep(")
	if found && strings.Contains(after, "#") {
		return true
	}
	for j := idx - 1; j >= 0; j-- {
		stripped := strings.TrimSpace(lines[j])
		if stripped == "" {
			continue
		}
		return strings.HasPrefix(stripped, "#")
	}
	return false
}

// checkLoadExcuses refuses a CHANGED known-failures shard that blames host load.
//
// Load is a mechanism, not a mystery. If a shard fails on a busy machine, the
// test asserts elapsed time instead of state. The deliverable is that test's
// fix. Shards remain available when the mechanism is unknown. Thus, this is a
// phrase check, not a shard ban.
func (g *gate) checkLoadExcuses() CheckResult {
	var shards []string
	for _, path := range g.report.Changed {
		if !strings.HasPrefix(path, knownFailuresDir) || !strings.HasSuffix(path, ".md") {
			continue
		}
		if knownFailuresExempt[filepath.Base(path)] {
			continue
		}
		shards = append(shards, path)
	}
	if len(shards) == 0 {
		return CheckResult{Skipped: true}
	}

	var violations []string
	offenders := make(map[string]bool)
	var tb textbuf.Buffer
	for _, rel := range shards {
		lines, missing, err := readLines(g.root, rel)
		if err != nil {
			return g.readFailure(checkLoadExcuseName, err)
		}
		if missing {
			continue // deleting a shard is the intended outcome, not a violation
		}
		for i, line := range lines {
			if !loadExcuseRe.MatchString(line) {
				continue
			}
			violations = append(violations, tb.Reset().Str(rel).Byte(':').Int(int64(i+1)).
				Str(": ").Str(strings.TrimSpace(line)).String())
			offenders[rel] = true
		}
	}

	if len(violations) > 0 {
		g.declareFailureGroup(checkLoadExcuseName, sortedKeys(offenders),
			"a changed known-failures shard blames host load for its red", gateRerun)
		return CheckResult{Failed: true, Message: "known-failure load excuse FAILED:", Violations: violations}
	}
	return CheckResult{Message: tb.Reset().Str("known-failure load excuse OK (").Int(int64(len(shards))).
		Str(" shard(s) checked)").String()}
}

// checkLogSubsystemKeys refuses a log-level key in a .ci test that names no
// real subsystem.
//
// The subsystem lookup splits only on dots. An internal plugin logger replaces
// each registry-name hyphen with a dot. Therefore, a hyphenated key matches no
// lookup and changes no level. The test then loses the log lines that it was
// written to observe, with no error.
//
// The check examines only hyphen-bearing subsystems because that is the failure
// mode. A legitimate hyphenated subsystem has a literal declaration in Go
// source. An absent literal proves that the key is inert. The check ignores
// comment lines because one test documents the wrong form.
//
// It covers changed .ci files, like the sleep gates. The source scan is
// tree-wide so an unrelated edit cannot legitimize an inert key elsewhere.
func (g *gate) checkLogSubsystemKeys() CheckResult {
	if !anyChangedCI(g.report.Changed) {
		return CheckResult{Skipped: true}
	}
	if _, err := os.Stat(filepath.Join(g.root, "test")); err != nil {
		return CheckResult{Skipped: true}
	}

	type suspect struct {
		rel       string
		line      int
		subsystem string
		text      string
	}

	files, err := realCIFilesIncludingDrafts(g.root)
	if err != nil {
		return g.readFailure(checkLogSubsystemName, err)
	}

	var suspects []suspect
	for _, path := range files {
		raw, err := os.ReadFile(path) //nolint:gosec // a .ci test of the tree the caller named
		if err != nil {
			return g.readFailure(checkLogSubsystemName, err)
		}
		rel, err := filepath.Rel(g.root, path)
		if err != nil {
			return g.readFailure(checkLogSubsystemName, err)
		}
		rel = filepath.ToSlash(rel)
		for i, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
				continue
			}
			for _, m := range ciLogKeyRe.FindAllStringSubmatch(line, -1) {
				subsystem := strings.Trim(m[1], ".")
				if !strings.Contains(subsystem, "-") {
					continue
				}
				suspects = append(suspects, suspect{rel: rel, line: i + 1, subsystem: subsystem, text: strings.TrimSpace(line)})
			}
		}
	}
	if len(suspects) == 0 {
		return CheckResult{Skipped: true}
	}

	declared, err := hyphenatedSubsystemsInGo(g.root)
	if err != nil {
		return g.readFailure(checkLogSubsystemName, err)
	}

	var violations []string
	offenders := make(map[string]bool)
	var tb textbuf.Buffer
	for _, s := range suspects {
		if declared[s.subsystem] {
			continue
		}
		violations = append(violations,
			tb.Reset().Str(s.rel).Byte(':').Int(int64(s.line)).
				Str(": ze.log.").Str(s.subsystem).Str("  (did you mean ").
				Str(strings.ReplaceAll(s.subsystem, "-", ".")).Str("?)").String(),
			tb.Reset().Str("  ").Str(s.text).String())
		offenders[s.rel] = true
	}

	if len(violations) > 0 {
		g.declareFailureGroup(checkLogSubsystemName, sortedKeys(offenders),
			"a .ci test sets a ze.log.<subsystem> key that matches no slog subsystem", gateRerun)
		return CheckResult{Failed: true, Message: "ci log-subsystem key FAILED:", Violations: violations}
	}
	return CheckResult{Message: tb.Reset().Str("ci log-subsystem key OK (").Int(int64(len(suspects))).
		Str(" hyphenated key(s) declared)").String()}
}

// realCIFilesIncludingDrafts answers every .ci under test/, the draft incubator
// included. The key check reads the whole tree, because an inert key in a draft
// is still an inert key.
func realCIFilesIncludingDrafts(root string) ([]string, error) {
	base := filepath.Join(root, "test")
	var files []string
	err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walking %s: %w", path, err)
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".ci") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// hyphenatedSubsystemsInGo answers every hyphen-bearing double-quoted Go string
// literal under the source roots.
//
// A subsystem name reaches the code in one of two literal forms. It is an
// engine logger name or the plugin registry Name that the forked path uses verbatim.
// Quoted literals cover both forms without a call-shape parser. Text absent
// from source cannot be a subsystem name.
func hyphenatedSubsystemsInGo(root string) (map[string]bool, error) {
	found := make(map[string]bool)
	for _, sub := range goSourceRoots {
		base := filepath.Join(root, sub)
		if _, err := os.Stat(base); err != nil {
			continue
		}
		err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("walking %s: %w", path, err)
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			raw, err := os.ReadFile(path) //nolint:gosec // a source file of the tree the caller named
			if err != nil {
				// A source file this scan cannot read declares no subsystem, so
				// a key naming one it declares would be reported as inert. That
				// is a FALSE RED rather than a fail-open, and it is still wrong.
				return fmt.Errorf("reading %s: %w", path, err)
			}
			for _, m := range hyphenLiteralRe.FindAllStringSubmatch(string(raw), -1) {
				found[m[1]] = true
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return found, nil
}

// checkDesignRefs runs the design-reference existence gate over the whole tree.
//
// It is unconditional: closure debt is non-local, because deleting or closing a
// spec orphans design references in any source file. So this scans the whole
// tree on every verify rather than only when related files change.
func (g *gate) checkDesignRefs() CheckResult {
	checker := filepath.Join(g.root, "scripts", "dev", "check_doc_links.py")
	if _, err := os.Stat(checker); err != nil {
		// An isolated test root or a minimal checkout without the checker: skip
		// rather than fail. The real verify always runs with the repository as
		// its root.
		return CheckResult{Skipped: true}
	}

	ctx, cancel := context.WithTimeout(context.Background(), designRefTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "scripts/dev/check_doc_links.py", "--design-only")
	cmd.Dir = g.root
	var out, errOut strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()

	if errOut.Len() > 0 {
		fmt.Fprint(os.Stderr, errOut.String()) //nolint:errcheck // CLI output
	}
	if err == nil {
		return CheckResult{Output: out.String()}
	}

	// Captured, not inherited. The child's output contains the paths that this
	// check reports. This unconditional tree-wide check is likely to fail in a
	// shared checkout. A failure without a file would be charged to the
	// committing session.
	g.declareFailureGroup(checkDesignRefsName, findingPaths(g.root, strings.Split(out.String(), "\n")),
		"a `// Design:` reference does not resolve to a durable document",
		"python3 scripts/dev/check_doc_links.py --design-only")
	return CheckResult{Failed: true, Output: out.String()}
}

// readFailure answers the result when a check cannot read its judged tree. It
// also declares the failure group.
func (g *gate) readFailure(check string, err error) CheckResult {
	var tb textbuf.Buffer
	summary := tb.Str(check).Str(" could not read the tree it judges").String()
	g.declareFailureGroup(check, nil, summary, gateRerun)
	tb.Reset()
	return CheckResult{
		Failed:  true,
		Code:    2,
		Message: tb.Str(check).Str(" FAILED: ").Err(err).String(),
	}
}

// readLines answers a repository file's lines. The second result reports a file
// that is absent, which several checks read as an intended deletion rather than
// as a failure.
func readLines(root, rel string) ([]string, bool, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) //nolint:gosec // a repository path the caller named
	if os.IsNotExist(err) {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("reading %s: %w", rel, err)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n"), false, nil
}

// changedCI answers the changed paths that are functional tests.
func changedCI(changed []string) []string {
	var out []string
	for _, path := range changed {
		if strings.HasPrefix(path, "test/") && strings.HasSuffix(path, ".ci") {
			out = append(out, path)
		}
	}
	return out
}

func anyChangedCI(changed []string) bool { return len(changedCI(changed)) > 0 }

// sortedKeys answers a set's members in order, which is what a group's related
// list owes its reader.
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// FunctionalTestAdvisory warns when user-facing code changed and no functional
// test did. It is advisory rather than blocking: a session that changed
// user-facing behavior with no test change gets a named pointer.
func FunctionalTestAdvisory(changed []string) string {
	for _, path := range changed {
		if strings.HasPrefix(path, "test/") {
			return ""
		}
	}

	suites := make(map[string][]string)
	for _, path := range changed {
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			continue
		}
		for _, area := range functionalSuiteByArea {
			if strings.HasPrefix(path, area.prefix) {
				suites[area.suite] = append(suites[area.suite], path)
				break
			}
		}
	}
	if len(suites) == 0 {
		return ""
	}

	var tb textbuf.Buffer
	tb.Str("ADVISORY: user-facing code changed without a functional-test change")
	for _, suite := range sortedKeys(setOf(suites)) {
		paths := suites[suite]
		sort.Strings(paths)
		tb.Str("\n  expected coverage in ").Str(suite).Str(" for: ").Join(paths, ", ")
	}
	return tb.Str("\n  see ai/rules/testing.md").String()
}

// setOf answers a set of a map's keys, so the ordering helper reads one shape.
func setOf(suites map[string][]string) map[string]bool {
	set := make(map[string]bool, len(suites))
	for key := range suites {
		set[key] = true
	}
	return set
}

// functionalSuiteByArea maps a user-facing area to the functional suite
// expected to change with it. It is ORDERED and the first match wins, so a
// nested area is named before the tree that contains it.
var functionalSuiteByArea = [...]struct {
	prefix string
	suite  string
}{
	{"internal/component/cli/", "test/ui/ or test/editor/"},
	{"internal/component/web/", "test/web/"},
	{"internal/component/config/", "test/parse/"},
	{"internal/component/cmd/", "test/ui/"},
}

// isDir reports a path that is a directory this process can stat.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// FunctionalSuite is one row of the advisory table: a user-facing area and the
// functional suite expected to change with it.
type FunctionalSuite struct {
	Prefix string `json:"prefix"`
	Suite  string `json:"suite"`
}

// FunctionalSuites answers the advisory table, in the order the advisory reads
// it. First match wins, so a nested area is named before the tree containing it.
func FunctionalSuites() []FunctionalSuite {
	out := make([]FunctionalSuite, 0, len(functionalSuiteByArea))
	for _, area := range functionalSuiteByArea {
		out = append(out, FunctionalSuite{Prefix: area.prefix, Suite: area.suite})
	}
	return out
}
