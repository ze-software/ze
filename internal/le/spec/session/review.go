// Design: docs/architecture/core-design.md -- native spec lifecycle support
// Related: model.go -- transcript model enforcement
// Related: review_report.go -- structured review answers

package specsession

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/lepath"
)

const (
	reviewDir           = "tmp/review"
	reviewRoundCap      = 5
	reviewOwnerRoundCap = 5
)

var (
	reviewHeaderPattern = regexp.MustCompile(`<!--\s*ze-review\s+spec=(\S+)\s+verdict=(\S+).*?-->`)
	reviewFilePattern   = regexp.MustCompile(`^\s{2}([0-9a-f]{64}|DELETED)\s+(.+)$`)
)

var reviewCodeSuffixes = [...]string{".go", ".ci", ".et", ".py", ".sh", ".yang", ".wb", ".mk", ".tmpl", ".html", ".c", ".rs", ".s", ".rego", ".tac"}

// reviewRecord selects the evidence recorded for one independent review.
type reviewRecord struct {
	Spec            string
	Verdict         string
	Files           []string
	Reviewers       string
	FindingsFile    string
	Rounds          int
	RoundsReason    string
	OwnerAuthorised string
	ModelOverride   string
	Model           string
	Now             time.Time
	SessionID       string
}

// recordReview validates and atomically records one review artifact.
func recordReview(root string, request reviewRecord) (reviewArtifact, error) {
	files := uniqueSorted(request.Files)
	if len(files) == 0 {
		return reviewArtifact{}, errors.New("review_gate: record needs files")
	}
	verdict := strings.ToLower(request.Verdict)
	if !isReviewVerdict(verdict) {
		return reviewArtifact{}, errors.New("review_gate: verdict must be clean or findings")
	}
	if request.Rounds < 1 {
		return reviewArtifact{}, errors.New("review_gate: rounds must be at least 1; an artifact claiming zero passes is a review that never ran")
	}
	reason := strings.TrimSpace(request.RoundsReason)
	owner := strings.TrimSpace(request.OwnerAuthorised)
	if request.Rounds > reviewRoundCap {
		if reason == "" {
			return reviewArtifact{}, fmt.Errorf("review_gate: %d review rounds needs a rounds reason that names the product defect found after round %d", request.Rounds, reviewRoundCap)
		}
	}
	if request.Rounds > reviewOwnerRoundCap {
		if owner == "" {
			return reviewArtifact{}, fmt.Errorf("review_gate: %d review rounds needs Thomas's authorization; a session must not authorize itself", request.Rounds)
		}
	}

	model := request.Model
	if model == "" {
		model = CurrentModel(root)
	}
	var warnings []string
	if model == "" {
		warnings = append(warnings, "review_gate: WARNING could not determine the running model; the review-model boundary is UNCHECKED (ai/rules/planning.md)")
	} else if !IsReviewTier(model) {
		if request.ModelOverride == "" {
			return reviewArtifact{}, fmt.Errorf(
				"review_gate: BLOCKED this session is on %s. Review runs on Opus 5 (ai/rules/planning.md).\n"+
					"  A review performed on the implementation model is the author grading their own work,\n"+
					"  which is the failure the independent-review rule exists to prevent (ai/rules/planning.md).\n"+
					"  Switch to Opus 5 and re-run the review, or provide a model override with the operator's reason",
				model,
			)
		}
		warnings = append(warnings, fmt.Sprintf("review_gate: WARNING recording a review made on %s, not the review model. Operator reason: %s", model, request.ModelOverride))
	}

	findings := ""
	if request.FindingsFile != "" {
		body, err := os.ReadFile(reviewInputPath(root, request.FindingsFile))
		if err != nil {
			return reviewArtifact{}, fmt.Errorf("review_gate: findings file %s cannot be read: %w", request.FindingsFile, err)
		}
		findings = strings.TrimSpace(string(body))
	}
	if verdict == "findings" {
		if findings == "" {
			return reviewArtifact{}, errors.New("review_gate: a findings verdict needs a non-empty findings file")
		}
	}
	if findings != "" {
		if !findingsNameReviewedFile(findings, files) {
			return reviewArtifact{}, fmt.Errorf("review_gate: the findings name none of the reviewed files: %s", strings.Join(files, ", "))
		}
	}

	sid := request.SessionID
	if sid == "" {
		session, err := lepath.ResolveSession(root, false)
		if err != nil {
			return reviewArtifact{}, err
		}
		sid = session.ID
	}
	if safeSessionID(sid) == "" {
		return reviewArtifact{}, fmt.Errorf("review_gate: invalid session ID %q", sid)
	}
	now := request.Now
	if now.IsZero() {
		now = time.Now()
	}
	artifact := reviewArtifact{
		Spec:            specStem(request.Spec),
		Verdict:         verdict,
		Rounds:          request.Rounds,
		Reviewers:       request.Reviewers,
		Model:           model,
		Timestamp:       now.UTC().Format(time.RFC3339),
		Files:           make([]ReviewedFile, 0, len(files)),
		Findings:        findings,
		RoundsReason:    reason,
		OwnerAuthorised: owner,
		ModelOverride:   request.ModelOverride,
		Warnings:        warnings,
	}
	if artifact.Spec == "" {
		return reviewArtifact{}, errors.New("review_gate: record needs a spec")
	}
	if artifact.Reviewers == "" {
		artifact.Reviewers = "unspecified"
	}
	for _, file := range files {
		hash, err := reviewFileHash(root, file)
		if err != nil {
			return reviewArtifact{}, err
		}
		artifact.Files = append(artifact.Files, ReviewedFile{Path: file, Hash: hash})
	}
	outPath := reviewArtifactPath(root, artifact.Spec, sid)
	if err := writeAtomic(outPath, []byte(artifact.Document()), 0o600); err != nil {
		return reviewArtifact{}, err
	}
	artifact.Path = relativeReviewPath(root, outPath)
	return artifact, nil
}

// CheckReview verifies that this session's clean artifact covers every named
// code file and that no covered file changed after the review.
func CheckReview(root, spec, sessionID string, files []string) (ReviewCheck, error) {
	if sessionID == "" {
		session, err := lepath.ResolveSession(root, false)
		if err != nil {
			return ReviewCheck{}, err
		}
		sessionID = session.ID
	}
	if safeSessionID(sessionID) == "" {
		return ReviewCheck{}, fmt.Errorf("review_gate: invalid session ID %q", sessionID)
	}
	path := reviewArtifactPath(root, specStem(spec), sessionID)
	artifact, err := parseReviewArtifact(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ReviewCheck{Spec: specStem(spec), Path: relativeReviewPath(root, path), Blocked: true, Reason: reasonMissing}, nil
		}
		if errors.Is(err, errMalformedArtifact) {
			return ReviewCheck{Spec: specStem(spec), Path: relativeReviewPath(root, path), Blocked: true, Reason: reasonMissing}, nil
		}
		return ReviewCheck{}, err
	}
	check := ReviewCheck{Spec: specStem(spec), Path: relativeReviewPath(root, path), Verdict: artifact.Verdict}
	if artifact.Model != "" {
		if !IsReviewTier(artifact.Model) {
			check.Warnings = append(check.Warnings, fmt.Sprintf("review_gate: NOTE this artifact was recorded on %s, not the review model (ai/rules/planning.md)", artifact.Model))
		}
	}
	if artifact.Verdict != "clean" {
		check.Blocked = true
		check.Reason = keywordVerdict
		return check, nil
	}
	hashes := make(map[string]string, len(artifact.Files))
	for _, file := range artifact.Files {
		hashes[file.Path] = file.Hash
	}
	for _, file := range uniqueSorted(files) {
		if !isReviewCode(file) {
			continue
		}
		check.CodeFiles++
		recorded, ok := hashes[file]
		if !ok {
			check.Unreviewed = append(check.Unreviewed, file)
			continue
		}
		current, err := reviewFileHash(root, file)
		if err != nil {
			return ReviewCheck{}, err
		}
		if recorded != current {
			check.Stale = append(check.Stale, file)
		}
	}
	if len(check.Unreviewed) > 0 {
		check.Blocked = true
		check.Reason = "unreviewed"
	} else if len(check.Stale) > 0 {
		check.Blocked = true
		check.Reason = "stale"
	}
	return check, nil
}

func reviewFileHash(root, path string) (string, error) {
	body, err := os.ReadFile(reviewInputPath(root, path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "DELETED", nil
		}
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

// specStem reduces a path, file name, or bare spec name to its stem.
func specStem(spec string) string {
	base := filepath.Base(filepath.ToSlash(spec))
	return strings.TrimSuffix(strings.TrimPrefix(base, "spec-"), ".md")
}

func reviewInputPath(root, path string) string {
	path = filepath.FromSlash(path)
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func reviewArtifactPath(root, spec, sid string) string {
	return filepath.Join(root, filepath.FromSlash(reviewDir), spec+"-"+sid+".md")
}

var errMalformedArtifact = errors.New("malformed review artifact")

func parseReviewArtifact(path string) (reviewArtifact, error) {
	body, err := os.ReadFile(path) //nolint:gosec // the path is a spec or session artifact under the checkout root
	if err != nil {
		return reviewArtifact{}, err
	}
	text := string(body)
	header := reviewHeaderPattern.FindStringSubmatch(text)
	if header == nil {
		return reviewArtifact{}, errMalformedArtifact
	}
	artifact := reviewArtifact{Path: path, Spec: header[1], Verdict: strings.ToLower(header[2])}
	for line := range strings.SplitSeq(text, "\n") {
		if match := reviewFilePattern.FindStringSubmatch(line); match != nil {
			artifact.Files = append(artifact.Files, ReviewedFile{Hash: match[1], Path: match[2]})
		}
	}
	if model := regexp.MustCompile(`\bmodel=(\S+)`).FindStringSubmatch(header[0]); model != nil {
		artifact.Model = model[1]
	}
	return artifact, nil
}

func writeAtomic(path string, body []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".review-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) //nolint:errcheck // rename removes it on success
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close() //nolint:errcheck // chmod error is primary
		return err
	}
	if _, err := tmp.Write(body); err != nil {
		tmp.Close() //nolint:errcheck // write error is primary
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close() //nolint:errcheck // sync error is primary
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func uniqueSorted(values []string) []string {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func isReviewVerdict(verdict string) bool {
	switch verdict {
	case "clean", "findings":
		return true
	default:
		return false
	}
}
func findingsNameReviewedFile(findings string, files []string) bool {
	for _, file := range files {
		if strings.Contains(findings, file) {
			return true
		}
		if strings.Contains(findings, filepath.Base(file)) {
			return true
		}
	}
	return false
}

func isReviewCode(path string) bool {
	if filepath.Base(path) == "Makefile" {
		return true
	}
	for _, suffix := range reviewCodeSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func relativeReviewPath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}
