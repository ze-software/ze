// Design: docs/architecture/testing/test-health.md -- immutable baseline/selected-content comparison and pairing
// Related: ledger.go -- the fail-closed approval row reader; approve.go -- the approval writer.
// Related: detector.go, scope.go -- verdict production and unit naming.
package testweakened

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/commit/hunks"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/repo/objects"
)

const cannotRunPrefix = "check could not run: "

// RenamePair is one old/new test path pair from the prospective commit's
// isolated index. Score is Git's similarity percentage.
type RenamePair struct {
	OldPath string `json:"old-path"`
	NewPath string `json:"new-path"`
	Score   int    `json:"score"`
}

// Request is the exact change population the check judges. Paths and Removed
// come from one commit, not from a scan of the shared worktree.
type Request struct {
	Root string `json:"root"`
	// ApprovalSession is the commit namespace whose owner approvals admit this
	// request's weakenings: the main thread's, because Thomas answers the main
	// thread and a subagent's edit is the main thread's work
	// (lepath.SessionCommitSession). Empty resolves the running harness's main
	// thread; a test names it to drive a fixture.
	ApprovalSession string       `json:"approval-session,omitempty"`
	Paths           []string     `json:"paths,omitempty"`
	Removed         []string     `json:"removed,omitempty"`
	RenamePairs     []RenamePair `json:"rename-pairs,omitempty"`
	// Anchor is the immutable baseline commit or tree. Native preparation uses
	// the retained tree after earlier script blocks, never a second HEAD lookup.
	Anchor string `json:"anchor,omitempty"`
	// Selected is the immutable content carried for selected paths, including
	// whole-file snapshots and hunks. Native comparisons never read their new
	// side from concurrent working-tree bytes.
	Selected hunks.Selection `json:"-"`
}

// Finding is one weakened unit and the ordered detector verdicts for it.
type Finding struct {
	Path    string   `json:"path"`
	Package string   `json:"package"`
	Name    string   `json:"name"`
	Details []string `json:"details"`
}

// Unit names the finding the way an owner approval and its trailer do,
// `<package>.<TestName>`.
func (f Finding) Unit() string {
	return f.Package + "." + f.Name
}

// Approvals is what the check learned from one owner approval file: the
// trailer line for each row the commit uses, and the exact file lines the
// generated commit script drops once the commit lands. Dropped holds the used
// rows and every row whose trailer git already carries: a row a landed commit
// used approves nothing further, even when the prune that should have removed
// it did not run.
type Approvals struct {
	Path     string   `json:"path"`
	Trailers []string `json:"trailers,omitempty"`
	Dropped  []string `json:"dropped,omitempty"`
}

// Result is the structured answer from Check.
type Result struct {
	// Contract is the weakening approval file the check read.
	Contract    string    `json:"contract"`
	Anchor      string    `json:"anchor,omitempty"`
	Rows        int       `json:"rows"`
	PathsGiven  int       `json:"paths-given"`
	PathsJudged int       `json:"paths-judged"`
	Findings    []Finding `json:"findings,omitempty"`
	Problems    []string  `json:"problems,omitempty"`
	Comparison  bool      `json:"comparison"`
	// Approvals is what the commit carries from the weakening approval file.
	Approvals Approvals `json:"approvals"`
	// RFCApprovals holds RFC rows that admitted a weakening or matched a
	// changed existing unit, carrying each row's `RFC-approved:` trailer.
	RFCApprovals Approvals `json:"rfc-approvals"`
	// ApprovalSites includes changed units whose live approval is
	// carried even when the structural detector reports no finding.
	ApprovalSites []UnitSite `json:"approval-sites,omitempty"`
}

// Check judges one exact HEAD/worktree comparison: every unit the change
// weakens must be named by a live owner approval row, in the session's
// weakening approval file or its RFC approval file. A request that names no
// path compares nothing. The selftest, the fixtures and the commit gate use
// this function; the edit hook reads the same files through Proposed.
// A live weakening or RFC approval of a changed selected unit is carried even when
// the structural detector finds nothing, so a semantic verdict is not lost.
func Check(request Request) Result {
	var result Result
	if request.Root == "" {
		var text textbuf.Buffer
		result.Problems = []string{text.Str(cannotRunPrefix).
			Str("the checkout root is empty").String()}
		return result
	}
	if err := normalizeRequestPaths(&request); err != nil {
		var text textbuf.Buffer
		result.Problems = []string{text.Str(cannotRunPrefix).Err(err).String()}
		return result
	}
	session, problem := approvalSession(request)
	if problem != "" {
		result.Problems = []string{problem}
		return result
	}
	result.Contract = ApprovalPath(session)
	if len(request.Paths) == 0 && len(request.Removed) == 0 {
		return result
	}

	result.Comparison = true
	result.Anchor = request.Anchor
	if result.Anchor == "" {
		result.Anchor = headRevision
	}
	given := uniquePaths(request.Paths, request.Removed)
	result.PathsGiven = len(given)
	for _, path := range given {
		if isTestPath(path) {
			result.PathsJudged++
		}
	}
	findings, changed, errors := weakenedTests(
		request.Root, request.Paths, request.Removed, request.RenamePairs, result.Anchor, request.Selected,
	)
	result.Findings = findings
	if len(errors) != 0 {
		result.Problems = errors
		return result
	}
	if len(findings) == 0 && len(changed) == 0 {
		return result
	}

	weakening, problems := loadApprovalFile(request.Root, WeakeningApprovals(session))
	if len(problems) != 0 {
		result.Problems = problems
		return result
	}
	owner, problems := loadApprovalFile(request.Root, RFCApprovals(session))
	if len(problems) != 0 {
		result.Problems = problems
		return result
	}
	result.Rows = len(weakening.rows)
	result.Approvals.Path = weakening.path
	result.RFCApprovals.Path = owner.path
	// Every landed weakening row leaves with this commit: it already did its
	// work. The RFC approval file's landed rows are its own gate's to drop.
	for _, row := range weakening.rows {
		if row.Carrier != "" {
			result.Approvals.Dropped = append(result.Approvals.Dropped, row.Text)
		}
	}
	for _, finding := range findings {
		claimed, why := claimFinding(&result.Approvals, &weakening, finding)
		if !claimed {
			var fromOwner string
			claimed, fromOwner = claimFinding(&result.RFCApprovals, &owner, finding)
			if why == "" {
				why = fromOwner
			}
		}
		if !claimed {
			result.Problems = append(result.Problems, unapprovedProblem(finding, why))
		}
	}
	// A semantic migration can keep or add assertions. Its judge's verdict
	// still belongs in the commit, but an unchanged or unselected unit cannot
	// spend it. The same row matcher preserves single-use and session scope.
	// A unit an unlanded row names is still an approval site when the row is
	// stale and claims nothing: the commit's shared-unit guard must see every
	// unit a verdict names, so one row cannot be spent across two carriers.
	for _, unit := range changed {
		claimed, _ := claimFinding(&result.Approvals, &weakening, unit)
		if !claimed {
			claimed, _ = claimFinding(&result.RFCApprovals, &owner, unit)
		}
		site := claimed
		if !site {
			site = namesUnlanded(&weakening, unit)
		}
		if !site {
			site = namesUnlanded(&owner, unit)
		}
		if site {
			result.ApprovalSites = append(result.ApprovalSites, UnitSite{Unit: unit.Unit(), Path: unit.Path})
		}
	}
	return result
}

// namesUnlanded reports whether a row of file that no commit carried and
// that targets no commit names unit, stale or not. It claims nothing: a
// stale row stays unspent, and only claimFinding records a trailer.
func namesUnlanded(file *approvalFile, unit Finding) bool {
	for _, row := range file.rows {
		if row.Target != "" {
			continue
		}
		if row.Carrier != "" {
			continue
		}
		if rowMatches(row.Name, unit) {
			return true
		}
	}
	return false
}

// approvalSession answers the namespace whose approval files admit request,
// as Request.ApprovalSession describes, refusing one that is not a namespace
// before it names a file under tmp/.
func approvalSession(request Request) (string, string) {
	var text textbuf.Buffer
	if request.ApprovalSession != "" {
		if !lepath.ValidCommitSession(request.ApprovalSession) {
			return "", text.Str(cannotRunPrefix).Str("approval session ").Str(request.ApprovalSession).
				Str(" is not a commit namespace (8 lowercase hexadecimal characters)").String()
		}
		return request.ApprovalSession, ""
	}
	session, err := lepath.SessionCommitSession(request.Root)
	if err != nil {
		return "", text.Str(cannotRunPrefix).
			Str("cannot resolve this session's approval namespace: ").Err(err).String()
	}
	return session, ""
}

// approvalFile is one owner approval file as the check reads it. used marks
// the rows this commit claimed, so each row's trailer is carried once.
type approvalFile struct {
	path string
	rows []ApprovalRow
	used map[int]bool
}

// loadApprovalFile reads one approval file through LoadApprovals, the loader
// the edit hook and the audit share.
func loadApprovalFile(root string, source ApprovalSource) (approvalFile, []string) {
	rows, problems := LoadApprovals(root, source)
	return approvalFile{path: source.Path, rows: rows, used: make(map[int]bool)}, problems
}

// claimFinding marks the first live row of file that names finding as used,
// recording its trailer and file line in approvals once. It answers whether a
// live row claimed the finding and, when none did, why a row naming it
// approves nothing: the commit that already used it, or the change after the
// verdict that made it stale.
func claimFinding(approvals *Approvals, file *approvalFile, finding Finding) (bool, string) {
	why := ""
	for index, row := range file.rows {
		if row.Target != "" {
			continue
		}
		if !rowMatches(row.Name, finding) {
			continue
		}
		if row.Carrier != "" {
			why = "the approval naming it was already used by commit " + row.Carrier + ", so it approves nothing further"
			continue
		}
		if row.Stale != "" {
			why = "the approval naming it is stale (" + row.Stale + "), so it approves nothing"
			continue
		}
		if !file.used[index] {
			file.used[index] = true
			approvals.Trailers = append(approvals.Trailers, row.Trailer)
			approvals.Dropped = append(approvals.Dropped, row.Text)
		}
		return true, ""
	}
	return false, why
}

// unapprovedProblem is the refusal for one weakened unit no live approval
// names. It says who decides, and the exact command that records his answer.
func unapprovedProblem(finding Finding, why string) string {
	var text textbuf.Buffer
	text.Str(finding.Path).Str(" weakens ").Str(finding.Unit())
	if why == "" {
		text.Str(", and no ze-test-judge approval names it:\n")
	} else {
		text.Str(", and ").Str(why).Str(":\n")
	}
	for _, detail := range finding.Details {
		text.Str("    - ").Str(detail).Byte('\n')
	}
	text.Str("    Fix the code by default. Only an independent ze-test-judge agent approves a test weakening, BEFORE the edit.\n").
		Str("    On approval the judge records its verdict, then retry:\n    ").Str(ApproveCommand(finding.Unit()))
	return text.String()
}

func normalizeRequestPaths(request *Request) error {
	for _, paths := range []*[]string{&request.Paths, &request.Removed} {
		copied := false
		for index, raw := range *paths {
			path, err := repositoryPath(request.Root, raw)
			if err != nil {
				return err
			}
			if path != raw {
				if !copied {
					*paths = slices.Clone(*paths)
					copied = true
				}
				(*paths)[index] = path
			}
		}
	}
	copied := false
	for index, pair := range request.RenamePairs {
		oldPath, err := repositoryPath(request.Root, pair.OldPath)
		if err != nil {
			return err
		}
		newPath, err := repositoryPath(request.Root, pair.NewPath)
		if err != nil {
			return err
		}
		if oldPath != pair.OldPath || newPath != pair.NewPath {
			if !copied {
				request.RenamePairs = slices.Clone(request.RenamePairs)
				copied = true
			}
			request.RenamePairs[index].OldPath = oldPath
			request.RenamePairs[index].NewPath = newPath
		}
	}
	return nil
}

// ExitCode preserves the producer's three-way verdict: clean, problem, or no
// comparison. A cannot-run result is never interpreted as a clean population.
func (r Result) ExitCode() int {
	for _, problem := range r.Problems {
		if strings.HasPrefix(problem, cannotRunPrefix) {
			return 2
		}
	}
	if len(r.Problems) != 0 {
		return 1
	}
	return 0
}

func (r Result) Text() string {
	var page textbuf.Buffer
	page.SetColor(slogutil.UseColor(os.Stdout))
	color := textbuf.C
	switch r.ExitCode() {
	case 0:
		if !r.Comparison {
			page.Colored(color.BrightGreen).Str("Weakened-test check: no path given, nothing compared.").
				Colored(color.Reset).Byte('\n')
			return page.String()
		}
		page.Colored(color.BrightGreen).Str("Weakened-test check: clean (").
			Int(int64(r.PathsJudged)).Str(" of ").Int(int64(r.PathsGiven)).
			Str(" path(s) are tests, judged against ").Str(r.Anchor).Str(").").
			Colored(color.Reset).Byte('\n')
	case 2:
		page.Colored(color.BoldRed).Str("Weakened-test check: CANNOT RUN.").
			Colored(color.Reset).Byte('\n')
		for _, problem := range r.Problems {
			page.Str("  ").Str(problem).Byte('\n')
		}
	default:
		page.Colored(color.BoldRed).Str("Weakened-test check: ").Int(int64(len(r.Problems))).
			Str(" problem(s).").Colored(color.Reset).Str("\n\n")
		for _, problem := range r.Problems {
			page.Str("  ").Str(problem).Byte('\n')
		}
	}
	return page.String()
}

func weakenedTests(
	root string,
	paths, removed []string,
	renamePairs []RenamePair,
	anchor string,
	selection hunks.Selection,
) ([]Finding, []Finding, []string) {
	findings := make([]Finding, 0)
	changed := make([]Finding, 0)
	problems := make([]string, 0)
	seen := make(map[string]bool, len(paths)+len(removed))
	removedSet := make(map[string]bool, len(removed))
	var text textbuf.Buffer
	for _, path := range removed {
		removedSet[path] = true
	}
	for _, pair := range renamePairs {
		if seen[pair.OldPath] || seen[pair.NewPath] {
			problems = append(problems, text.Reset().Str(cannotRunPrefix).
				Str("rename path appears in more than one pair, so no rename was compared").String())
			continue
		}
		seen[pair.OldPath] = true
		seen[pair.NewPath] = true
		pairFindings, pairChanges, problem := comparePath(root, pair.OldPath, pair.NewPath, anchor, selection)
		findings = append(findings, pairFindings...)
		if problem != "" {
			problems = append(problems, text.Reset().Str(cannotRunPrefix).Str(problem).String())
			continue
		}
		changed = append(changed, pairChanges...)
	}
	for _, path := range slices.Concat(paths, removed) {
		if seen[path] || !isTestPath(path) {
			continue
		}
		seen[path] = true
		newPath := path
		if removedSet[path] {
			newPath = ""
		}
		pathFindings, pathChanges, problem := comparePath(root, path, newPath, anchor, selection)
		findings = append(findings, pathFindings...)
		if problem != "" {
			problems = append(problems, text.Reset().Str(cannotRunPrefix).Str(problem).String())
			continue
		}
		changed = append(changed, pathChanges...)
	}
	return findings, changed, problems
}

func comparePath(root, oldPath, newPath, anchor string, selection hunks.Selection) ([]Finding, []Finding, string) {
	oldText, problem := baselineText(root, oldPath, anchor)
	if problem != "" {
		return nil, nil, problem
	}
	if strings.TrimSpace(oldText) == "" {
		return nil, nil, ""
	}
	newText := ""
	path := oldPath
	if newPath != "" {
		path = newPath
		text, problem := worktreeText(root, newPath, selection)
		if problem != "" {
			return nil, nil, problem
		}
		newText = text
	}
	packageDir := filepath.Dir(path)
	packageName := filepath.Base(packageDir)
	if packageDir == "." {
		packageName = ""
	}
	verdicts := weakenedUnits(path, oldText, newText)
	findings := make([]Finding, 0, len(verdicts))
	for _, verdict := range verdicts {
		findings = append(findings, Finding{
			Path: path, Package: packageName, Name: verdict.name, Details: verdict.details,
		})
	}
	changed := make([]Finding, 0)
	names, err := changedUnitNames(path, oldText, newText)
	if err != nil {
		return findings, nil, err.Error()
	}
	for _, name := range names {
		changed = append(changed, Finding{Path: path, Package: packageName, Name: name})
	}
	return findings, changed, ""
}

func baselineText(root, path, anchor string) (string, string) {
	tree, err := objects.ResolveTree(context.Background(), root, anchor)
	if err != nil {
		if exit, failed := errors.AsType[*exec.ExitError](err); failed {
			if exit.Exited() {
				var text textbuf.Buffer
				return "", text.Str(anchor).
					Str(" does not resolve to a commit, so nothing was compared").String()
			}
		}
		return "", err.Error()
	}
	content, _, err := objects.ReadPath(context.Background(), root, tree, path)
	if err != nil {
		return "", err.Error()
	}
	return string(content), ""
}

// worktreeText answers the text the commit carries for path, or why it cannot
// be read. A path the commit carries that cannot be read is never compared as
// empty: the comparison would judge a test file the commit does not hold.
func worktreeText(root, path string, selection hunks.Selection) (string, string) {
	content, err := selection.Read(root, path)
	if err != nil {
		return "", "read " + path + ": " + err.Error()
	}
	return string(content), ""
}

func readRepositoryFile(root, path string) ([]byte, error) {
	repository, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	content, readErr := repository.ReadFile(filepath.FromSlash(path))
	closeErr := repository.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return content, nil
}

// rowMatches reports whether an approval row's name covers one finding: the
// row names exactly `<package>.<TestName>`. LoadApprovals refuses a row of
// any other shape, so no bare name reaches this comparison.
func rowMatches(rowName string, finding Finding) bool {
	return rowName == finding.Unit()
}

func isTestPath(path string) bool {
	if strings.HasSuffix(path, "_test.go") {
		return true
	}
	if (strings.HasSuffix(path, ".ci") || strings.HasSuffix(path, ".et")) &&
		(strings.HasPrefix(path, "test/") || strings.Contains(path, "/test/")) {
		return true
	}
	return isPythonTest(path)
}

func uniquePaths(first, second []string) []string {
	paths := make([]string, 0, len(first)+len(second))
	seen := make(map[string]bool, len(first)+len(second))
	for _, path := range slices.Concat(first, second) {
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	return paths
}

func gitCapture(root string, args ...string) (string, string, int, bool) {
	// #nosec G204 -- Git is the fixed executable and every call site supplies one of this package's closed verbs; path and ref operands remain distinct argv, never shell code.
	command := exec.CommandContext(context.Background(), "git", args...)
	command.Dir = root
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0, true
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return stdout.String(), stderr.String(), exit.ExitCode(), true
	}
	return stdout.String(), err.Error(), -1, false
}

// The revision this detector compares against, and the two reasons it reports
// when git answers nothing it can compare.
const (
	headRevision        = "HEAD"
	gitRenameUnreadable = "Git returned malformed or ambiguous rename status, so no rename was compared"
)
