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
	// ApprovalSession is the namespace whose live approval files admit the
	// worktree pass, as Request.ApprovalSession describes. Empty resolves the
	// running harness's main thread.
	ApprovalSession string `json:"approval-session,omitempty"`
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
	// The worktree is not a commit, so no message carries its approvals: the
	// live approval files do, as they do for the commit that will carry it.
	live, problem := liveApprovalRows(request.Root, request.ApprovalSession)
	if problem != "" {
		report.Problem = problem
		return report
	}
	findings, examined, problem := auditDiff(request.Root, headRevision, "", live)
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

// auditApprovals is what admits a change in one audited diff. Weakening is
// every unit whose weakening the owner approved, under either key: an RFC
// approval of a unit approves its weakening too. RFC is only the units whose
// RFC-tagged change he approved, so a Weakening-approved line never admits a
// change to a tagged unit's requirement.
type auditApprovals struct {
	weakening []Row
	rfc       []Row
}

func auditDiff(root, oldRevision, newRevision string, accepted auditApprovals) ([]AuditFinding, int, string) {
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
	files := make([]auditFile, 0, len(changes))
	sites := make([]UnitSite, 0)
	for _, change := range changes {
		if change.status == "A" {
			continue
		}
		if !isTestPath(change.oldPath) && !isTestPath(change.newPath) {
			continue
		}
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
		file := auditFile{
			status: change.status, path: path, packageName: packageName,
			verdicts: weakenedUnits(path, oldText, newText),
			// Each tagged unit the change touched is judged on its own, with
			// the splitter the commit gate uses, so an approved unit does not
			// hide an unapproved one beside it in the same file.
			rfcUnits: rfc.ChangedUnits(path, oldText, newText),
		}
		for _, verdict := range file.verdicts {
			sites = append(sites, UnitSite{Unit: packageName + "." + verdict.name, Path: path})
		}
		for _, unit := range file.rfcUnits {
			sites = append(sites, UnitSite{Unit: packageName + "." + unit.Name, Path: path})
		}
		files = append(files, file)
	}
	// A unit the diff carries from two directories with one base name is
	// named by no approval: the line cannot say which test Thomas approved.
	shared := SharedUnits(sites)
	findings := make([]AuditFinding, 0)
	for _, file := range files {
		findings = append(findings, file.judge(accepted, shared)...)
	}
	return findings, len(files), ""
}

// auditFile is one changed test file of an audited diff: what the detector
// and the RFC splitter found in it, before any approval is applied.
type auditFile struct {
	status      string
	path        string
	packageName string
	verdicts    []namedVerdict
	rfcUnits    []rfc.UnitChange
}

// judge answers the findings of one file no approval admits. An approval of a
// unit in shared admits nothing, and the finding says why.
func (file *auditFile) judge(accepted auditApprovals, shared map[string][]string) []AuditFinding {
	findings := make([]AuditFinding, 0)
	admits := func(rows []Row, name string) bool {
		if shared[file.packageName+"."+name] != nil {
			return false
		}
		return acceptedBy(rows, file.packageName, name)
	}
	sharedDetail := func(details []string, name string) []string {
		unit := file.packageName + "." + name
		if directories := shared[unit]; directories != nil {
			return append(details, SharedUnitProblem(unit, directories))
		}
		return details
	}
	status, path, packageName := file.status, file.path, file.packageName
	rfcTags := make(map[string][]string)
	for _, unit := range file.rfcUnits {
		rfcTags[unit.Name] = unit.Tags
	}
	rfcUnits := file.rfcUnits
	reported := make(map[string]bool)
	for _, verdict := range file.verdicts {
		if admits(accepted.weakening, verdict.name) {
			continue
		}
		details := slices.Clone(verdict.details)
		if strings.HasPrefix(status, "R") {
			details = append([]string{"renamed file"}, details...)
		}
		if tags, tagged := rfcTags[verdict.name]; tagged {
			details = append(details, rfcDetails(tags)...)
			reported[verdict.name] = true
		}
		kind := "WEAKENED"
		if status == "D" {
			kind = "DELETED"
		}
		findings = append(findings, AuditFinding{
			Kind: kind, Path: path, Package: packageName, Name: verdict.name,
			Details: sharedDetail(details, verdict.name),
		})
	}
	for _, unit := range rfcUnits {
		if reported[unit.Name] || admits(accepted.rfc, unit.Name) {
			continue
		}
		details := rfcDetails(unit.Tags)
		if strings.HasPrefix(status, "R") {
			details = append([]string{"renamed file"}, details...)
		}
		findings = append(findings, AuditFinding{
			Kind: "WEAKENED", Path: path, Package: packageName, Name: unit.Name,
			Details: sharedDetail(details, unit.Name),
		})
	}
	return findings
}

// rfcDetails is what the audit says about one changed tagged unit no approval
// names.
func rfcDetails(tags []string) []string {
	return []string{
		"RFC-TAGGED test changed: " + strings.Join(tags, ", "),
		"only the OWNER approves this, and the approval is recorded with `./le rfc approve unit <package>.<TestName> reason \"<the owner's words>\"`, which the commit carries as an RFC-approved: trailer",
	}
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

// acceptedRows answers the units one commit's owner approvals name: one row
// per `Weakening-approved:` or `RFC-approved:` line of its message. The commit
// is the record of what Thomas approved, so a later audit reads it there. A
// line whose unit is not `<package>.<TestName>` approves nothing and is
// skipped: it is never read as a bare test name, and the findings it might
// have named are still reported. It cannot refuse the commit, because a
// pushed message is history, and a prose sentence that opens with a trailer
// key would then block every later audit over that commit.
func acceptedRows(root, commit string) (auditApprovals, string) {
	accepted := auditApprovals{}
	message, stderr, code, started := gitCapture(root, "show", "-s", "--format=%B", commit)
	if !started || code != 0 {
		return accepted, "git could not read the message of commit " +
			shortRevision(commit) + ": " + strings.TrimSpace(stderr)
	}
	for line := range strings.SplitSeq(message, "\n") {
		for _, key := range []string{ApprovalTrailer, rfc.ApprovalTrailer} {
			rest, found := strings.CutPrefix(strings.TrimSpace(line), key)
			if !found {
				continue
			}
			unit, reason, named := strings.Cut(rest, ": ")
			if !named {
				continue
			}
			if !rfc.ValidApprovalUnit(unit) {
				continue
			}
			row := Row{Name: unit, Reason: reason}
			accepted.weakening = append(accepted.weakening, row)
			if key == rfc.ApprovalTrailer {
				accepted.rfc = append(accepted.rfc, row)
			}
		}
	}
	return accepted, ""
}

// liveApprovalRows answers the live rows of the session's two approval files,
// which admit what the worktree weakens, split as acceptedRows splits a
// commit's lines. LoadApprovals refuses a malformed row, and a row whose
// trailer already landed approves nothing further, so it is left out.
func liveApprovalRows(root, namespace string) (auditApprovals, string) {
	accepted := auditApprovals{}
	session, problem := approvalSession(Request{Root: root, ApprovalSession: namespace})
	if problem != "" {
		return accepted, problem
	}
	for _, source := range []ApprovalSource{WeakeningApprovals(session), RFCApprovals(session)} {
		rows, problems := LoadApprovals(root, source)
		if len(problems) != 0 {
			return auditApprovals{}, strings.Join(problems, "; ")
		}
		for _, row := range rows {
			if row.Carrier != "" {
				continue
			}
			accepted.weakening = append(accepted.weakening, row.Row)
			if source.Path == rfc.ApprovalPath(session) {
				accepted.rfc = append(accepted.rfc, row.Row)
			}
		}
	}
	return accepted, ""
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

// acceptedBy reports whether an approval row names the unit.
func acceptedBy(rows []Row, packageName, name string) bool {
	finding := Finding{Package: packageName, Name: name}
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
