// Design: docs/contributing/documentation-testing.md -- a claim a reader reviewed
// Related: checks.go -- checkDocDrift, the check that reads this ledger.
// Related: internal/le/le/path/commitsession.go -- the identity a shard is named after.
//
// reviewed.go reads the reviewed-claim ledger. A change that preserves behavior,
// such as a branch swap or a split guard, leaves every claim about its symbol as
// true as it was. A page edit made only to satisfy doc-drift is banned, so a
// reader who compared the claim with the change records it here instead.
//
// A claim is covered only when every unpushed commit that changed a line of
// its symbol has a row, and the working tree leaves the symbol alone. A row
// the grammar refuses, a row that names no claim anchor, and a row whose
// unpushed commit did not change its symbol are each a finding of their own.

package docwiring

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	docindex "github.com/ze-software/ze/internal/le/doc/index"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// ReviewedDir holds one ledger shard for each commit session, named after its
// eight-hex `./le commit session` id. Every shard counts, so a claim another
// session reviewed stays accepted for this session's run.
const ReviewedDir = "plan/doc-reviewed"

// reviewedGit is the program every ledger query runs.
const reviewedGit = "git"

// reviewedCommitPattern is the only Commit spelling a row accepts: an object
// id. A ref such as HEAD or a branch moves, so a row naming one would cover
// every later commit without anybody reading it.
var reviewedCommitPattern = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// reviewedHeader is the one table header a shard carries.
var reviewedHeader = []string{"Doc", "Source", "Symbol", "Commit", "Reason"}

// reviewedRow is one claim a reader compared with one commit's change.
type reviewedRow struct {
	doc    string
	source string
	symbol string
	commit string
	reason string
	shard  string
	line   int
}

// reviewLedger answers whether a flagged claim is covered by reviewed rows.
// It caches each git answer, because many rows name the same commit. Not safe
// for concurrent use.
type reviewLedger struct {
	root  string
	base  string
	rows  []reviewedRow
	byKey map[string][]reviewedRow

	// problems holds one finding for each row or shard that is wrong on its
	// own, in the order they were found. shards names the files they are in.
	problems []string
	shards   map[string]bool

	resolved     map[string]string
	unpushed     []string
	unpushedRead bool
	paths        map[string]map[string]bool
	renamed      map[string]map[string]bool
	inCommit     map[string]repochanged.ChangedLines
	touchedAt    map[string]map[string]bool
	worktree     repochanged.ChangedLines
}

// reviewedKey is a claim's identity in the ledger. A line number is never part
// of it, because an edit above the anchor moves the line and leaves the claim.
func reviewedKey(doc, source, symbol string) string {
	return doc + "\x00" + source + "\x00" + symbol
}

// readReviewLedger reads every shard under ReviewedDir and resolves each commit
// a row names. An absent directory is an empty ledger. base is the first commit
// outside the unpushed range. An empty base leaves no commit in range, so no
// row covers anything.
func readReviewLedger(root, base string) (*reviewLedger, error) {
	ledger := &reviewLedger{
		root:      root,
		base:      base,
		byKey:     map[string][]reviewedRow{},
		shards:    map[string]bool{},
		resolved:  map[string]string{},
		paths:     map[string]map[string]bool{},
		renamed:   map[string]map[string]bool{},
		inCommit:  map[string]repochanged.ChangedLines{},
		touchedAt: map[string]map[string]bool{},
	}
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(ReviewedDir)))
	if errors.Is(err, os.ErrNotExist) {
		return ledger, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		shard := ReviewedDir + "/" + entry.Name()
		body, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(shard))) //nolint:gosec // a shard under the repository's own ledger directory
		if readErr != nil {
			return nil, readErr
		}
		ledger.parseShard(shard, string(body))
	}
	for _, row := range ledger.rows {
		ledger.resolve(row)
	}
	return ledger, nil
}

// ledgerOnlyResult answers the check when no Go source changed. The ledger is
// still judged, so a malformed shard fails the run now rather than on the next
// Go edit.
func (g *checker) ledgerOnlyResult(ledger *reviewLedger) CheckResult {
	if len(ledger.problems) == 0 {
		return CheckResult{Skipped: true}
	}
	g.declareFailureGroup(checkDocDriftName, sortedKeys(ledger.shards),
		"a reviewed-claim ledger row is wrong on its own", actionRerun)
	return CheckResult{Failed: true, Violations: slices.Clone(ledger.problems)}
}

// empty reports a ledger with no row and no problem, which has nothing to say.
func (l *reviewLedger) empty() bool {
	return len(l.rows) == 0 && len(l.problems) == 0
}

// parseShard reads every table row after the shard's header into the ledger.
// A blank line or prose between rows does not end the table, so no row is
// dropped unread. A shard with no header, and a row the grammar refuses, each
// add a problem naming the shard and line.
func (l *reviewLedger) parseShard(shard, body string) {
	lines := strings.Split(body, "\n")
	start := -1
	for index, line := range lines {
		if slices.Equal(reviewedCells(line), reviewedHeader) {
			start = index
			break
		}
	}
	if start < 0 {
		l.problem(shard, 0, "has no `| Doc | Source | Symbol | Commit | Reason |` table, so no row in it can be read")
		return
	}
	for index := start + 1; index < len(lines); index++ {
		cells := reviewedCells(lines[index])
		if cells == nil {
			continue
		}
		if reviewedSeparator(cells) {
			continue
		}
		l.parseRow(shard, index+1, cells)
	}
}

// parseRow adds one table row, or the problem that keeps it from covering.
func (l *reviewLedger) parseRow(shard string, line int, cells []string) {
	if len(cells) != len(reviewedHeader) {
		var tb textbuf.Buffer
		l.problem(shard, line, tb.Str("has ").Int(int64(len(cells))).
			Str(" cells; a row is `| Doc | Source | Symbol | Commit | Reason |`").String())
		return
	}
	for index, cell := range cells {
		if cell != "" {
			continue
		}
		l.problem(shard, line, "names no "+reviewedHeader[index]+"; a row with an empty cell covers nothing")
		return
	}
	if !reviewedCommitPattern.MatchString(cells[3]) {
		l.problem(shard, line, "names commit "+cells[3]+", which is not a 7 to 40 digit lowercase hex id; a ref moves, so a row names the commit itself")
		return
	}
	row := reviewedRow{doc: cells[0], source: cells[1], symbol: cells[2], commit: cells[3], reason: cells[4], shard: shard, line: line}
	l.rows = append(l.rows, row)
	key := reviewedKey(row.doc, row.source, row.symbol)
	l.byKey[key] = append(l.byKey[key], row)
}

// reviewedCells answers a markdown table line's trimmed cells, with the
// backticks a writer puts around a path or a symbol removed. A line that is
// not a table row answers nil.
func reviewedCells(line string) []string {
	body := strings.TrimSpace(line)
	if !strings.HasPrefix(body, "|") {
		return nil
	}
	parts := strings.Split(strings.Trim(body, "|"), "|")
	for index := range parts {
		parts[index] = strings.Trim(strings.TrimSpace(parts[index]), "`")
	}
	return parts
}

// reviewedSeparator reports the `|---|---|` line under a header.
func reviewedSeparator(cells []string) bool {
	for _, cell := range cells {
		if !strings.Contains(cell, "-") {
			return false
		}
		if strings.Trim(cell, ":-") != "" {
			return false
		}
	}
	return true
}

// problem records one finding about a shard, and the shard as a path the
// failure is about.
func (l *reviewLedger) problem(shard string, line int, text string) {
	var tb textbuf.Buffer
	tb.Str(shard)
	if line > 0 {
		tb.Byte(':').Int(int64(line))
	}
	l.problems = append(l.problems, tb.Byte(' ').Str(text).String())
	l.shards[shard] = true
}

// resolve answers the full id of the commit a row names, and records a
// problem once for a row whose commit this repository does not hold.
func (l *reviewLedger) resolve(row reviewedRow) (string, bool) {
	if full, known := l.resolved[row.commit]; known {
		return full, full != ""
	}
	out, err := repochanged.RunCommand(l.root, []string{reviewedGit, "rev-parse", "--verify", "--quiet", row.commit + "^{commit}"})
	full := strings.TrimSpace(out)
	if err != nil {
		full = ""
	}
	l.resolved[row.commit] = full
	if full == "" {
		l.problem(row.shard, row.line, "names commit "+row.commit+", which this repository does not hold")
	}
	return full, full != ""
}

// validate judges every row on its own, whether or not a claim is flagged. A
// row whose key names no claim anchor is a typo nobody would otherwise see. A
// row naming an unpushed commit that did not change its symbol vouches for a
// change that does not exist. A row naming a pushed commit says nothing about
// the range and is left alone, which is every row once its commits are pushed.
func (l *reviewLedger) validate(claims map[string][]docindex.AnchorClaim) {
	anchored := map[string]bool{}
	for source, list := range claims {
		for _, claim := range list {
			for _, symbol := range claim.Symbols {
				anchored[reviewedKey(claim.Doc, source, symbol)] = true
			}
		}
	}
	for _, row := range l.rows {
		if !anchored[reviewedKey(row.doc, row.source, row.symbol)] {
			l.problem(row.shard, row.line, "names "+row.doc+" "+row.source+" "+row.symbol+", which matches no claim anchor")
			continue
		}
		full, ok := l.resolve(row)
		if !ok {
			continue
		}
		unpushed, err := l.unpushedCommits()
		if err != nil {
			l.problem(row.shard, row.line, "cannot be judged, because the unpushed range could not be read: "+err.Error())
			continue
		}
		if !slices.Contains(unpushed, full) {
			continue
		}
		touched, err := l.symbolsAt(full, row.source)
		if err != nil {
			l.problem(row.shard, row.line, "cannot be judged, because "+row.commit+" could not be read: "+err.Error())
			continue
		}
		if len(claimedSymbolsTouched([]string{row.symbol}, touched)) == 0 {
			l.problem(row.shard, row.line, row.commit+" did not change "+row.symbol)
		}
	}
}

// covers answers whether the rows for one flagged claim symbol cover it: no
// unpushed commit renamed or copied a file into source, every unpushed commit
// that changed a line of the symbol is named by a row, and the working tree
// leaves the symbol alone. When they do not, the reasons name the rename, each
// commit with no row, and the working-tree edit. A claim with no row at all
// answers no reasons, because its finding already says what to do.
func (l *reviewLedger) covers(doc, source, symbol string) ([]string, bool) {
	rows := l.byKey[reviewedKey(doc, source, symbol)]
	if len(rows) == 0 {
		return nil, false
	}
	// The commit walk reads each commit at the path source, so an edit made
	// before a rename into source sits at the old path and no row can name
	// it. A row for a later commit would vouch for a change nobody read, so a
	// rename refuses every row on the source.
	renaming, err := l.renamingCommit(source)
	if err != nil {
		return []string{"the reviewed rows cannot be judged: " + err.Error()}, false
	}
	if renaming != "" {
		return []string{source + " was renamed in the unpushed range (" + shortCommit(renaming) + "): edit the page"}, false
	}
	reviewed := map[string]bool{}
	for _, row := range rows {
		if full, ok := l.resolve(row); ok {
			reviewed[full] = true
		}
	}

	var reasons []string
	commits, err := l.commitsTouching(source, symbol)
	if err != nil {
		return []string{"the reviewed rows cannot be judged: " + err.Error()}, false
	}
	for _, commit := range commits {
		if reviewed[commit] {
			continue
		}
		reasons = append(reasons, shortCommit(commit)+" changed "+symbol+" and has no reviewed row")
	}
	worktree, err := l.worktreeLines()
	if err != nil {
		return append(reasons, "the working tree cannot be read: "+err.Error()), false
	}
	touched, readable := touchedSymbols(l.root, source, worktree)
	if !readable {
		return append(reasons, source+" cannot be parsed"), false
	}
	worktreeTouched := len(claimedSymbolsTouched([]string{symbol}, touched)) != 0
	if worktreeTouched {
		reasons = append(reasons, "the working tree changed "+symbol+" after every reviewed commit")
	}
	// The claim was flagged by the whole range, so a change that neither the
	// commit walk nor the working tree accounts for is one no row can name: a
	// rename git did not pair carried the edited lines to another path. A row
	// covering it would vouch for a change nobody read.
	if len(commits) == 0 && !worktreeTouched {
		reasons = append(reasons, "the change to "+symbol+" is not attributable to a reviewed commit (rename): edit the page")
	}
	return reasons, len(reasons) == 0
}

// shortCommit answers the ten-digit spelling a finding uses for a commit.
func shortCommit(commit string) string {
	if len(commit) > 10 {
		return commit[:10]
	}
	return commit
}

// commitsTouching answers the unpushed commits that changed a line of symbol in
// source, newest first.
func (l *reviewLedger) commitsTouching(source, symbol string) ([]string, error) {
	unpushed, err := l.unpushedCommits()
	if err != nil {
		return nil, err
	}
	var touching []string
	for _, commit := range unpushed {
		paths, err := l.commitPaths(commit)
		if err != nil {
			return nil, err
		}
		if !paths[source] {
			continue
		}
		touched, err := l.symbolsAt(commit, source)
		if err != nil {
			return nil, err
		}
		if len(claimedSymbolsTouched([]string{symbol}, touched)) != 0 {
			touching = append(touching, commit)
		}
	}
	return touching, nil
}

// unpushedCommits answers the commits between the base and HEAD, newest
// first. With no base the list is empty, so no row covers.
func (l *reviewLedger) unpushedCommits() ([]string, error) {
	if l.unpushedRead {
		return l.unpushed, nil
	}
	if l.base == "" {
		l.unpushedRead = true
		return nil, nil
	}
	out, err := repochanged.RunCommand(l.root, []string{reviewedGit, "rev-list", l.base + "..HEAD"})
	if err != nil {
		return nil, err
	}
	l.unpushed = strings.Fields(out)
	l.unpushedRead = true
	return l.unpushed, nil
}

// commitPaths answers the paths one commit changed against its first parent.
// A merge lists its own changes, the same first-parent diff LinesInCommit reads.
func (l *reviewLedger) commitPaths(commit string) (map[string]bool, error) {
	if paths, known := l.paths[commit]; known {
		return paths, nil
	}
	out, err := repochanged.RunCommand(l.root, []string{reviewedGit, "diff-tree", "--no-commit-id", "-r", "--name-only", "--no-renames", "--diff-merges=first-parent", "--root", commit})
	if err != nil {
		return nil, err
	}
	paths := map[string]bool{}
	for path := range strings.FieldsSeq(out) {
		paths[path] = true
	}
	l.paths[commit] = paths
	return paths, nil
}

// renamingCommit answers the newest unpushed commit that renamed or copied a
// file into source, or "" when none did. Each commit is read against its first
// parent, the same diff commitPaths reads.
func (l *reviewLedger) renamingCommit(source string) (string, error) {
	unpushed, err := l.unpushedCommits()
	if err != nil {
		return "", err
	}
	for _, commit := range unpushed {
		targets, err := l.renameTargets(commit)
		if err != nil {
			return "", err
		}
		if targets[source] {
			return commit, nil
		}
	}
	return "", nil
}

// renameTargets answers the paths one commit renamed or copied a file into.
// The -z output is a run of NUL-separated fields: a status, then one path, or
// the old path and the new path for a rename (R) or a copy (C). -z keeps a
// path with an unusual byte unquoted, so it still matches source.
func (l *reviewLedger) renameTargets(commit string) (map[string]bool, error) {
	if targets, known := l.renamed[commit]; known {
		return targets, nil
	}
	out, err := repochanged.RunCommand(l.root, []string{reviewedGit, "diff-tree", "--no-commit-id", "-r", "-z", "--find-renames", "--find-copies", "--name-status", "--diff-merges=first-parent", "--root", commit})
	if err != nil {
		return nil, err
	}
	targets := map[string]bool{}
	fields := strings.Split(out, "\x00")
	for index := 0; index < len(fields); {
		status := fields[index]
		if status == "" {
			index++
			continue
		}
		paired := status[0] == 'R' || status[0] == 'C'
		if !paired {
			index += 2
			continue
		}
		if index+2 >= len(fields) {
			return nil, errors.New("git diff-tree for " + commit + " ends inside a rename record")
		}
		targets[fields[index+2]] = true
		index += 3
	}
	l.renamed[commit] = targets
	return targets, nil
}

// symbolsAt answers the declarations of source that one commit changed, read
// from that commit's own copy of the file.
func (l *reviewLedger) symbolsAt(commit, source string) (map[string]bool, error) {
	key := commit + "\x00" + source
	if touched, known := l.touchedAt[key]; known {
		return touched, nil
	}
	lines, known := l.inCommit[commit]
	if !known {
		read, err := repochanged.LinesInCommit(l.root, commit)
		if err != nil {
			return nil, err
		}
		lines = read
		l.inCommit[commit] = lines
	}
	body, err := repochanged.RunCommand(l.root, []string{reviewedGit, "show", commit + ":" + source})
	if err != nil {
		return nil, err
	}
	touched, readable := touchedSymbolsIn(source, []byte(body), lines)
	if !readable {
		return nil, errors.New(source + " at " + commit + " cannot be parsed")
	}
	l.touchedAt[key] = touched
	return touched, nil
}

// worktreeLines answers the lines the working tree changed since HEAD.
func (l *reviewLedger) worktreeLines() (repochanged.ChangedLines, error) {
	if l.worktree != nil {
		return l.worktree, nil
	}
	lines, err := repochanged.LinesSinceCommit(l.root, "HEAD")
	if err != nil {
		return nil, err
	}
	l.worktree = lines
	return lines, nil
}
