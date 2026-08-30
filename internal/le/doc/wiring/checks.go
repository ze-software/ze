// Design: docs/architecture/core-design.md -- the checks this gate runs itself
// Overview: docwiring.go -- the gate that runs them
//
// checks.go holds the checks no other native action owns. Each check reads a
// few files and answers in milliseconds. Thus, the router runs them directly.
//
// Every check REPORTS an unreadable file instead of ignoring it. A ratchet that
// counts only readable files reports a lower value for an incomplete tree. A
// lower value looks like a pass.

package docwiring

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/docstocode"
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

	// gitDiff is the one git subcommand this package's queries share.
	gitDiff = "diff"
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

// parseSleepBaseline answers the ceiling in the committed delta ledger. It sums
// signed-integer lines and ignores comments and blanks. The second result is
// false when no integer line exists, which leaves the ratchet inactive.
//
// The delta form replaces one absolute integer. Two independent sleep removals
// can append separate `-N` lines instead of conflicting on one number. A plain
// integer still parses as one line and one summand. A `+N` line explicitly
// raises the ceiling.
func parseSleepBaseline(text string) (int, bool) {
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
func (g *checker) checkSleepRatchet() CheckResult {
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
	ceiling, active := parseSleepBaseline(string(raw))
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
				Str(" time.sleep( calls against a ceiling of ").Int(int64(ceiling)).String(), actionRerun)
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
func (g *checker) checkSleepJustification() CheckResult {
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
			"a changed .ci test holds a time.sleep( with no comment saying why", actionRerun)
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
func (g *checker) checkLoadExcuses() CheckResult {
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
			"a changed known-failures shard blames host load for its red", actionRerun)
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
func (g *checker) checkLogSubsystemKeys() CheckResult {
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
			"a .ci test sets a ze.log.<subsystem> key that matches no slog subsystem", actionRerun)
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
// It is unconditional in a repository checkout: closure debt is non-local,
// because deleting or closing a spec orphans references in any source file. A
// minimal fixture with no go.mod has no Go repository population and skips.
func (g *checker) checkDesignRefs() CheckResult {
	if _, err := os.Stat(filepath.Join(g.root, "go.mod")); os.IsNotExist(err) {
		return CheckResult{Skipped: true}
	} else if err != nil {
		return g.readFailure(checkDesignRefsName, err)
	}

	findings, err := designReferenceFindings(g.root)
	if err != nil {
		return g.readFailure(checkDesignRefsName, err)
	}
	if len(findings) == 0 {
		return CheckResult{Output: "every path reference resolves\n"}
	}

	g.declareFailureGroup(checkDesignRefsName, findingPaths(g.root, findings),
		"a `// Design:` reference does not resolve to a durable document", actionRerun)
	var tb textbuf.Buffer
	return CheckResult{Failed: true, Output: tb.Join(findings, "\n").Byte('\n').String()}
}

// checkDocDrift reports a changed Go file whose documenting page did not change
// with it.
//
// A page carries a `<!-- source: <path> -- <symbol> -->` anchor over each claim
// it makes about code, so a changed file an anchor names is a file whose
// documented behavior can have moved. The page edit belongs in the same work as
// the code edit (`ai/rules/documentation.md`), and this check asks only whether
// the page moved at all. Whether the claim is still true is a reader's judgment
// and stays with `/ze-review`.
//
// One page of a file's several counts for that page alone. A file cited by
// three pages that changed one of them still leaves two claims unread.
//
// The check is SYMBOL-level, and that is what lets it block. An anchor naming
// `Sym` claims something about `Sym`, so an edit to a different declaration in
// the same file leaves the claim true and reports nothing. Measured on
// 2026-08-30 over the recent commits carrying non-test Go, each replayed
// against its own parent: a file-level reading reported nine of twelve, and
// this reading reports one of nine. That one names six claims whose symbols the
// commit rewrote while its pages stood still.
//
// A claim naming no symbol, and a file the parser cannot read, are both
// unanswerable rather than clean. Neither blocks, and the trailer counts them,
// so a silent check and a check with nothing to say never read alike.
func (g *checker) checkDocDrift() CheckResult {
	sources := changedGoSources(g.report.Changed)
	if len(sources) == 0 {
		return CheckResult{Skipped: true}
	}

	claims, err := docstocode.ClaimsByPath(g.root)
	if err != nil {
		return g.readFailure(checkDocDriftName, err)
	}

	pages := changedPages(g.report.Changed)
	related := map[string]bool{}
	var findings []string
	unanswerable := 0
	for _, source := range sources {
		if len(claims[source]) == 0 {
			continue
		}
		touched, ok := touchedSymbols(g.root, source)
		if !ok {
			unanswerable++
			continue
		}
		for _, claim := range claims[source] {
			if pages[claim.Doc] {
				continue
			}
			named := claimedSymbolsTouched(claim.Symbols, touched)
			if len(named) == 0 {
				if len(claim.Symbols) == 0 {
					unanswerable++
				}
				continue
			}
			var tb textbuf.Buffer
			findings = append(findings, tb.Str(claim.Doc).Byte(':').Int(int64(claim.Line)).Str(": ").
				Str(source).Byte(' ').Str(strings.Join(named, ", ")).
				Str(" changed under this claim and the page did not").String())
			related[claim.Doc] = true
			related[source] = true
		}
	}

	var tb textbuf.Buffer
	if len(findings) == 0 {
		tb.Str("every claim about a changed symbol changed with it")
		if unanswerable > 0 {
			tb.Str(" (").Int(int64(unanswerable)).Str(" claim(s) name no symbol this check can resolve)")
		}
		return CheckResult{Output: tb.Byte('\n').String()}
	}

	sort.Strings(findings)
	g.declareFailureGroup(checkDocDriftName, sortedKeys(related),
		"a changed symbol's documentation did not change with it", actionRerun)
	return CheckResult{Failed: true, Violations: findings}
}

// touchedSymbols answers the declarations of one changed file that the diff
// reached. The second result reports a file this check cannot judge: git or the
// Go parser could not read it, and an unreadable file is never a clean one.
func touchedSymbols(root, rel string) (map[string]bool, bool) {
	lines, ok := changedLines(root, rel)
	if !ok {
		return nil, false
	}

	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) //nolint:gosec // a repository path the caller named
	if err != nil {
		return nil, false
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rel, body, parser.SkipObjectResolution)
	if err != nil {
		return nil, false
	}

	touched := map[string]bool{}
	for _, decl := range file.Decls {
		from := fset.Position(decl.Pos()).Line
		to := fset.Position(decl.End()).Line
		for _, name := range declaredNames(decl) {
			if lines.overlaps(from, to) {
				touched[name] = true
			}
		}
	}
	return touched, true
}

// declaredNames answers what one top-level declaration declares. A method
// answers both its bare name and its Recv.Member spelling, because an anchor
// claims it either way.
func declaredNames(decl ast.Decl) []string {
	switch typed := decl.(type) {
	case *ast.FuncDecl:
		name := typed.Name.Name
		if typed.Recv == nil || len(typed.Recv.List) == 0 {
			return []string{name}
		}
		var tb textbuf.Buffer
		return []string{name, tb.Str(receiverName(typed.Recv.List[0].Type)).Byte('.').Str(name).String()}
	case *ast.GenDecl:
		var names []string
		for _, spec := range typed.Specs {
			switch declared := spec.(type) {
			case *ast.TypeSpec:
				names = append(names, declared.Name.Name)
			case *ast.ValueSpec:
				for _, ident := range declared.Names {
					names = append(names, ident.Name)
				}
			}
		}
		return names
	default:
		return nil
	}
}

// receiverName answers a method receiver's type name, pointer or value.
func receiverName(expr ast.Expr) string {
	if star, pointer := expr.(*ast.StarExpr); pointer {
		expr = star.X
	}
	if index, generic := expr.(*ast.IndexExpr); generic {
		expr = index.X
	}
	if ident, named := expr.(*ast.Ident); named {
		return ident.Name
	}
	return ""
}

// claimedSymbolsTouched answers the claim's symbols that the diff reached. A
// dotted claim matches on its member as well, so `Peer.Stop` answers for a
// change to `Stop`.
func claimedSymbolsTouched(symbols []string, touched map[string]bool) []string {
	var named []string
	for _, symbol := range symbols {
		bare := symbol
		if _, member, dotted := strings.Cut(symbol, "."); dotted {
			bare = member
		}
		if touched[symbol] || touched[bare] {
			named = append(named, symbol)
		}
	}
	return named
}

// lineRange is one contiguous run of changed lines.
type lineRange struct{ from, to int }

// lineRanges is one file's changed lines, as the hunks git reported.
type lineRanges []lineRange

// overlaps reports whether a declaration spanning from..to holds a changed line.
func (r lineRanges) overlaps(from, to int) bool {
	for _, one := range r {
		if one.from <= to && from <= one.to {
			return true
		}
	}
	return false
}

// changedLines answers the lines of one file the diff touched, reading the
// unstaged and staged hunks together. The second result reports a file git
// could not diff.
//
// A file with no hunk in either diff is untracked, so every line of it is new.
// That is the one case where a whole-file answer is the right one.
func changedLines(root, rel string) (lineRanges, bool) {
	var ranges lineRanges
	for _, argv := range [][]string{
		{gitDiff, "-U0", "--", rel},
		{gitDiff, "--cached", "-U0", "--", rel},
	} {
		out, err := gitLines(root, argv)
		if err != nil {
			return nil, false
		}
		ranges = append(ranges, hunkRanges(out)...)
	}
	if len(ranges) == 0 {
		return lineRanges{{from: 1, to: math.MaxInt32}}, true
	}
	return ranges, true
}

// hunkRe reads the new-side span of a unified diff hunk header.
var hunkRe = regexp.MustCompile(`^@@ -\S+ \+(\d+)(?:,(\d+))? @@`)

// hunkRanges answers the new-side line spans of one diff's hunk headers. A hunk
// of zero new lines is a deletion, and it takes the line it deleted from, so a
// declaration losing its body still counts as touched.
func hunkRanges(lines []string) lineRanges {
	var ranges lineRanges
	for _, line := range lines {
		match := hunkRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		from, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		count := 1
		if match[2] != "" {
			if parsed, err := strconv.Atoi(match[2]); err == nil {
				count = parsed
			}
		}
		if count == 0 {
			ranges = append(ranges, lineRange{from: from, to: from})
			continue
		}
		ranges = append(ranges, lineRange{from: from, to: from + count - 1})
	}
	return ranges
}

// changedGoSources answers the changed paths a page can carry a claim about.
//
// A test file is excluded: it states no behavior a page describes, and a page
// anchoring one is anchoring the test rather than the product.
func changedGoSources(changed []string) []string {
	var out []string
	for _, path := range changed {
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			continue
		}
		out = append(out, path)
	}
	return out
}

// changedPages answers the changed documentation pages as a set.
func changedPages(changed []string) map[string]bool {
	pages := map[string]bool{}
	for _, path := range changed {
		if !strings.HasSuffix(path, ".md") {
			continue
		}
		pages[path] = true
	}
	return pages
}

// readFailure answers the result when a check cannot read its judged tree. It
// also declares the failure group.
func (g *checker) readFailure(check string, err error) CheckResult {
	var tb textbuf.Buffer
	summary := tb.Str(check).Str(" could not read the tree it judges").String()
	g.declareFailureGroup(check, nil, summary, actionRerun)
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

// functionalTestAdvisory warns when user-facing code changed and no functional
// test did. It is advisory rather than blocking: a session that changed
// user-facing behavior with no test change gets a named pointer.
func functionalTestAdvisory(changed []string) string {
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
