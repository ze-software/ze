// Design: docs/architecture/testing/test-health.md -- branch and worktree review audit
package testweakened

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/rfc"
)

// AuditRequest selects the comparison base. HEAD audits uncommitted changes.
type AuditRequest struct {
	Root string `json:"root"`
	Base string `json:"base"`
}

// AuditFinding is one unexplained weakening in one parent-to-child comparison.
type AuditFinding struct {
	Kind    string   `json:"kind"`
	Path    string   `json:"path"`
	Package string   `json:"package"`
	Name    string   `json:"name"`
	Details []string `json:"details"`
}

// AuditReport is the complete branch/worktree comparison.
type AuditReport struct {
	Base     string         `json:"base"`
	Anchor   string         `json:"anchor,omitempty"`
	Examined int            `json:"examined"`
	Findings []AuditFinding `json:"findings,omitempty"`
	Problem  string         `json:"problem,omitempty"`
}

// ExitCode preserves clean, finding, and cannot-run as 0, 1, and 2.
func (r AuditReport) ExitCode() int {
	if r.Problem != "" {
		return 2
	}
	if len(r.Findings) != 0 {
		return 1
	}
	return 0
}

// Text renders the review-facing audit verdict.
func (r AuditReport) Text() string {
	var text textbuf.Buffer
	if r.Problem != "" {
		return text.Str("Test-relaxation audit: CANNOT RUN.\n  ").Str(r.Problem).Byte('\n').String()
	}
	if len(r.Findings) == 0 {
		return text.Str("Test-relaxation audit: clean (no unexplained test weakening).\n  base ").
			Str(r.Base).Str(", range ").Str(shortRevision(r.Anchor)).Str("..worktree, ").
			Int(int64(r.Examined)).Str(" changed test file(s) examined.\n").String()
	}
	text.Str("Test-relaxation audit (base ").Str(r.Base).Str(", range ").
		Str(shortRevision(r.Anchor)).Str("..worktree)\n\n")
	deleted := 0
	for _, finding := range r.Findings {
		if finding.Kind == "DELETED" {
			deleted++
		}
		text.Str("  [").Str(finding.Kind).Str("] ").Str(finding.Path).Byte('\n').
			Str("      - test: ").Str(finding.Name).Byte('\n')
		for _, detail := range finding.Details {
			text.Str("      - ").Str(detail).Byte('\n')
		}
	}
	return text.Byte('\n').Int(int64(len(r.Findings))).Str(" unexplained finding(s): ").
		Int(int64(deleted)).Str(" deleted, ").Int(int64(len(r.Findings) - deleted)).Str(" weakened.\n").String()
}

// Audit examines each commit in the selected range separately, then HEAD
// against the worktree. A ledger row explains only the commit carrying it.
func Audit(request AuditRequest) AuditReport {
	base := strings.TrimSpace(request.Base)
	if base == "" {
		base = headRevision
	}
	report := AuditReport{Base: base}
	head, problem := resolveCommit(request.Root, headRevision)
	if problem != "" {
		report.Problem = problem
		return report
	}
	if base == headRevision {
		report.Anchor = head
	} else {
		resolved, problem := resolveCommit(request.Root, base)
		if problem != "" {
			report.Problem = problem
			return report
		}
		merge, stderr, code, started := gitCapture(request.Root, "merge-base", resolved, head)
		if !started || code != 0 || strings.TrimSpace(merge) == "" {
			report.Problem = "base shares no common ancestor with HEAD: " + strings.TrimSpace(stderr)
			return report
		}
		report.Anchor = strings.TrimSpace(merge)
		if report.Anchor == head {
			report.Problem = "base resolves to the same commit as HEAD, so the range holds no commits; use HEAD for uncommitted changes"
			return report
		}
	}
	commits, stderr, code, started := gitCapture(request.Root, "rev-list", "--reverse", report.Anchor+"..HEAD")
	if !started || code != 0 {
		report.Problem = "git rev-list failed, so branch history cannot be examined: " + strings.TrimSpace(stderr)
		return report
	}
	for commit := range strings.SplitSeq(strings.TrimSpace(commits), "\n") {
		if commit == "" {
			continue
		}
		rows, problem := acceptedRows(request.Root, commit)
		if problem != "" {
			report.Problem = problem
			return report
		}
		findings, examined, problem := auditDiff(request.Root, commit+"^", commit, rows)
		if problem != "" {
			report.Problem = problem
			return report
		}
		report.Findings = append(report.Findings, findings...)
		report.Examined += examined
	}
	findings, examined, problem := auditDiff(request.Root, headRevision, "", nil)
	if problem != "" {
		report.Problem = problem
		return report
	}
	report.Findings = append(report.Findings, findings...)
	report.Examined += examined
	return report
}

func resolveCommit(root, revision string) (string, string) {
	stdout, stderr, code, started := gitCapture(root, "rev-parse", "--verify", revision+"^{commit}")
	if !started {
		return "", "git could not start"
	}
	if code != 0 {
		return "", "base " + revision + " does not resolve to a commit: " + strings.TrimSpace(stderr)
	}
	return strings.TrimSpace(stdout), ""
}

type changedPath struct {
	status  string
	oldPath string
	newPath string
}

func auditDiff(root, oldRevision, newRevision string, accepted []Row) ([]AuditFinding, int, string) {
	args := []string{"diff", "--name-status", "-M", oldRevision}
	if newRevision != "" {
		args = append(args, newRevision)
	}
	args = append(args, "--")
	stdout, stderr, code, started := gitCapture(root, args...)
	if !started || code != 0 {
		return nil, 0, "git diff failed, so nothing was compared: " + strings.TrimSpace(stderr)
	}
	changes := parseChangedPaths(stdout)
	findings := make([]AuditFinding, 0)
	examined := 0
	for _, change := range changes {
		if change.status == "A" {
			continue
		}
		if !isTestPath(change.oldPath) && !isTestPath(change.newPath) {
			continue
		}
		examined++
		oldText, problem := revisionText(root, oldRevision, change.oldPath)
		if problem != "" {
			return nil, 0, problem
		}
		newText := ""
		if change.status != "D" {
			if newRevision == "" {
				content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(change.newPath))) //nolint:gosec // the path is a tracked file under the checkout root
				if err != nil {
					return nil, 0, "cannot read worktree file " + change.newPath + ": " + err.Error()
				}
				newText = string(content)
			} else {
				newText, problem = revisionText(root, newRevision, change.newPath)
				if problem != "" {
					return nil, 0, problem
				}
			}
		}
		path := change.newPath
		if change.status == "D" {
			path = change.oldPath
		}
		packageName := filepath.Base(filepath.Dir(path))
		if filepath.Dir(path) == "." {
			packageName = ""
		}
		rfcTags := rfc.ChangedTags(path, oldText, newText)
		rfcDetails := make([]string, 0, 2)
		if len(rfcTags) != 0 {
			rfcDetails = append(rfcDetails,
				"RFC-TAGGED test changed: "+strings.Join(rfcTags, ", "),
				"only the OWNER approves this, and the approval is a row in test/rfc-changed.md in the commit that carries the change",
			)
		}
		rfcReported := false
		for _, verdict := range weakenedUnits(path, oldText, newText) {
			if acceptedBy(accepted, path, packageName, verdict.name) {
				continue
			}
			details := slices.Clone(verdict.details)
			if strings.HasPrefix(change.status, "R") {
				details = append([]string{"renamed file"}, details...)
			}
			if !rfcReported && len(rfcDetails) != 0 {
				details = append(details, rfcDetails...)
				rfcReported = true
			}
			kind := "WEAKENED"
			if change.status == "D" {
				kind = "DELETED"
			}
			findings = append(findings, AuditFinding{
				Kind: kind, Path: path, Package: packageName, Name: verdict.name, Details: details,
			})
		}
		if len(rfcDetails) != 0 && !rfcReported {
			details := slices.Clone(rfcDetails)
			if strings.HasPrefix(change.status, "R") {
				details = append([]string{"renamed file"}, details...)
			}
			findings = append(findings, AuditFinding{
				Kind: "WEAKENED", Path: path, Package: packageName,
				Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Details: details,
			})
		}
	}
	return findings, examined, ""
}

func parseChangedPaths(output string) []changedPath {
	changes := make([]changedPath, 0)
	for line := range strings.SplitSeq(output, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		if strings.HasPrefix(parts[0], "R") && len(parts) >= 3 {
			changes = append(changes, changedPath{status: parts[0], oldPath: parts[1], newPath: parts[2]})
			continue
		}
		changes = append(changes, changedPath{status: parts[0], oldPath: parts[1], newPath: parts[1]})
	}
	return changes
}

func acceptedRows(root, commit string) ([]Row, string) {
	_, _, code, started := gitCapture(root, "diff", "--quiet", commit+"^", commit, "--", ContractPath)
	if !started {
		return nil, "git could not inspect accepted weakening rows"
	}
	if code == 0 {
		return nil, ""
	}
	if code != 1 {
		return nil, "git could not scope accepted weakening rows to commit " + shortRevision(commit)
	}
	_, stderr, code, started := gitCapture(root, "cat-file", "-e", commit+":"+ContractPath)
	if !started {
		return nil, "git could not inspect accepted weakening rows"
	}
	if code != 0 {
		if code == 1 {
			return nil, ""
		}
		return nil, "git could not read accepted weakening rows from commit " +
			shortRevision(commit) + ": " + strings.TrimSpace(stderr)
	}
	content, problem := revisionText(root, commit, ContractPath)
	if problem != "" {
		return nil, problem
	}
	if content == "" {
		return nil, ""
	}
	rows, problems := parseLedger(content, ContractPath)
	if len(problems) != 0 {
		return nil, "cannot read accepted rows from commit " + shortRevision(commit) + ": " + strings.Join(problems, "; ")
	}
	return rows, ""
}

func revisionText(root, revision, path string) (string, string) {
	stdout, stderr, code, started := gitCapture(root, "show", revision+":"+path)
	if !started {
		return "", "git could not start"
	}
	if code != 0 {
		return "", "git show " + revision + ":" + path + " failed: " + strings.TrimSpace(stderr)
	}
	return stdout, ""
}

// acceptedBy checks path-scoped rows too, unlike the exported RowMatches: a
// migration commit that retires a whole tree with one scoped row (see
// scopedRowMatches in testweakened.go) must still read as explained when a later
// audit walks that commit's own diff.
func acceptedBy(rows []Row, path, packageName, name string) bool {
	finding := Finding{Path: path, Package: packageName, Name: name}
	for _, row := range rows {
		if rowMatches(row.Name, finding) {
			return true
		}
	}
	return false
}

func shortRevision(revision string) string {
	if len(revision) > 12 {
		return revision[:12]
	}
	return revision
}
