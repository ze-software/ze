// Design: docs/architecture/core-design.md -- native spec lifecycle support
// Related: model.go -- transcript model metadata
// Related: review_report.go -- structured review answers

package spec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/commit/hunks"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/repo/objects"
)

const (
	reviewDir           = "tmp/review"
	reviewRoundCap      = 5
	reviewOwnerRoundCap = 5
)

var (
	reviewHeaderPattern = regexp.MustCompile(`<!--\s*ze-review\s+spec=(\S+)\s+verdict=(\S+).*?-->`)
	reviewFilePattern   = regexp.MustCompile(`^  ([0-9a-f]{64}|DELETED)  (100644|100755|120000|0)  (.+)$`)
)

// reviewCodeSuffixes is the one list of what a review pins, read only through
// IsReviewCode.
var reviewCodeSuffixes = [...]string{".go", ".ci", ".et", ".py", ".sh", ".yang", ".wb", ".mk", ".tmpl", ".html", ".c", ".rs", ".s", ".rego", ".tac"}

// reviewRecord selects the evidence recorded for one independent review.
type reviewRecord struct {
	Spec    string
	Verdict string
	Files   []string
	// Hunks pins a file a commit will carry through `hunks <path> <patch>`:
	// the review covers HEAD's blob with that patch applied, which is what the
	// commit lands, not the working-tree file, which holds hunks it leaves out.
	Hunks           []ReviewHunk
	Blobs           []ReviewBlob
	Removed         []string
	Modes           []ReviewMode
	Reviewers       string
	FindingsFile    string
	Rounds          int
	RoundsReason    string
	OwnerAuthorised string
	Model           string
	Now             time.Time
	SessionID       string
}

// ReviewHunk is one `hunks <path> <patch>` pair of a review record.
type ReviewHunk struct {
	Path  string
	Patch string
}

// ReviewBlob names an immutable blob the reviewer examined.
type ReviewBlob struct {
	Path   string
	Object string
}

// ReviewMode names the Git mode the reviewer examined for a carried path.
type ReviewMode struct {
	Path string
	Mode string
}

// recordReview validates and atomically records one review artifact.
func recordReview(root string, request reviewRecord) (reviewArtifact, error) {
	selected := make(map[string]string, len(request.Hunks))
	named := slices.Clone(request.Files)
	for _, pair := range request.Hunks {
		if _, twice := selected[pair.Path]; twice {
			return reviewArtifact{}, fmt.Errorf("review_gate: hunks %s is named twice; put every hunk the commit carries for it in one patch", pair.Path)
		}
		selected[pair.Path] = pair.Patch
		named = append(named, pair.Path)
	}
	blobs := make(map[string]string, len(request.Blobs))
	for _, source := range request.Blobs {
		if _, exists := blobs[source.Path]; exists {
			return reviewArtifact{}, fmt.Errorf("review_gate: blob %s is named twice", source.Path)
		}
		if _, exists := selected[source.Path]; exists {
			return reviewArtifact{}, fmt.Errorf("review_gate: %s names both blob and hunks sources", source.Path)
		}
		if slices.Contains(request.Files, source.Path) {
			return reviewArtifact{}, fmt.Errorf("review_gate: %s names both blob and file sources", source.Path)
		}
		blobs[source.Path] = source.Object
		named = append(named, source.Path)
	}
	removed := make(map[string]bool, len(request.Removed))
	for _, path := range request.Removed {
		if slices.Contains(named, path) {
			return reviewArtifact{}, fmt.Errorf("review_gate: %s names both removal and content sources", path)
		}
		removed[path] = true
		named = append(named, path)
	}
	modes := make(map[string]string, len(request.Modes))
	for _, source := range request.Modes {
		if _, exists := modes[source.Path]; exists {
			return reviewArtifact{}, fmt.Errorf("review_gate: mode %s is named twice", source.Path)
		}
		if !slices.Contains(named, source.Path) {
			return reviewArtifact{}, fmt.Errorf("review_gate: mode %s has no content source", source.Path)
		}
		switch source.Mode {
		case "100644", "100755", "120000":
		default:
			return reviewArtifact{}, fmt.Errorf("review_gate: invalid Git mode %q", source.Mode)
		}
		if removed[source.Path] {
			return reviewArtifact{}, fmt.Errorf("review_gate: removed path %s cannot carry a file mode", source.Path)
		}
		modes[source.Path] = source.Mode
	}
	files := uniqueSorted(named)
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
	// Even below the cap, supplied owner words issue an authorization.
	// Admission MUST precede every artifact or session-directory write.
	if err := requireOwnerReviewTerminal(root, request); err != nil {
		return reviewArtifact{}, err
	}
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
	}
	if artifact.Spec == "" {
		return reviewArtifact{}, errors.New("review_gate: record needs a spec")
	}
	if artifact.Reviewers == "" {
		artifact.Reviewers = "unspecified"
	}
	head := ""
	if len(request.Hunks) != 0 {
		var err error
		head, err = objects.Resolve(context.Background(), root, "HEAD")
		if err != nil {
			return reviewArtifact{}, err
		}
	}
	for _, file := range files {
		if strings.ContainsAny(file, "\x00\t\r\n") {
			return reviewArtifact{}, fmt.Errorf("review_gate: path contains unsupported controls: %q", file)
		}
		pin, err := recordedFile(root, head, file, selected, blobs, removed, modes)
		if err != nil {
			return reviewArtifact{}, err
		}
		artifact.Files = append(artifact.Files, pin)
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
	if artifact.Spec != specStem(spec) {
		return ReviewCheck{}, fmt.Errorf("review_gate: artifact names spec %s, not %s", artifact.Spec, specStem(spec))
	}
	if artifact.Verdict != "clean" {
		check.Blocked = true
		check.Reason = keywordVerdict
		return check, nil
	}
	hashes := make(map[string]ReviewedFile, len(artifact.Files))
	for _, file := range artifact.Files {
		if _, duplicate := hashes[file.Path]; duplicate {
			return ReviewCheck{}, fmt.Errorf("review_gate: artifact repeats path %s", file.Path)
		}
		hashes[file.Path] = file
	}
	for _, file := range uniqueSorted(files) {
		if !IsReviewCode(file) {
			continue
		}
		check.CodeFiles++
		recorded, ok := hashes[file]
		if !ok {
			check.Unreviewed = append(check.Unreviewed, file)
			continue
		}
		current, err := ReviewFile(root, file)
		if err != nil {
			return ReviewCheck{}, err
		}
		if recorded.Hash != current.Hash || recorded.Mode != current.Mode {
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

// recordedFile pins exactly one declared source, including its Git mode.
func recordedFile(root, head, path string, selected, blobs map[string]string, removed map[string]bool, modes map[string]string) (ReviewedFile, error) {
	if removed[path] {
		return ReviewedFile{Path: path, Hash: "DELETED", Mode: "0"}, nil
	}
	pin := ReviewedFile{Path: path}
	if patch, found := selected[path]; found {
		entry, err := hunks.Apply(root, head, path, reviewInputPath(root, patch))
		if err != nil {
			return pin, err
		}
		selection := hunks.Selection{path: {Entry: entry, Patch: patch}}
		body, err := selection.Read(root, path)
		if err != nil {
			return pin, err
		}
		metadata, _, _ := strings.Cut(entry, "\t")
		pin.Mode = strings.Fields(metadata)[0]
		sum := sha256.Sum256(body)
		pin.Hash = hex.EncodeToString(sum[:])
	} else if object, found := blobs[path]; found {
		mode, found := modes[path]
		if !found {
			return pin, fmt.Errorf("review_gate: blob %s needs an explicit mode source", path)
		}
		body, err := objects.Read(context.Background(), root, []string{object})
		if err != nil {
			return pin, err
		}
		sum := sha256.Sum256(body[0])
		pin.Hash = hex.EncodeToString(sum[:])
		pin.Mode = mode
	} else {
		var err error
		pin, err = ReviewFile(root, path)
		if err != nil {
			return pin, err
		}
	}
	if mode, found := modes[path]; found {
		if pin.Hash == "DELETED" {
			return pin, fmt.Errorf("review_gate: removed path %s cannot carry a file mode", path)
		}
		pin.Mode = mode
	}
	return pin, nil
}

// ReviewFile reads a checkout review pin, including the Git executable or link mode.
func ReviewFile(root, path string) (ReviewedFile, error) {
	pin := ReviewedFile{Path: path}
	name := reviewInputPath(root, path)
	info, err := os.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		pin.Hash, pin.Mode = "DELETED", "0"
		return pin, nil
	}
	if err != nil {
		return pin, err
	}
	pin.Mode = "100644"
	if info.Mode()&0o100 != 0 {
		pin.Mode = "100755"
	}
	var body []byte
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(name)
		if err != nil {
			return pin, err
		}
		pin.Mode, body = "120000", []byte(target)
	} else {
		if !info.Mode().IsRegular() {
			return pin, fmt.Errorf("review_gate: %s is not a regular file or symbolic link", path)
		}
		body, err = os.ReadFile(name)
		if err != nil {
			return pin, err
		}
	}
	sum := sha256.Sum256(body)
	pin.Hash = hex.EncodeToString(sum[:])
	return pin, nil
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

// ReviewArtifactPath answers, repository-relative, where THIS session's review
// artifact for spec lives. A reader outside this package calls it instead of
// spelling the name again.
//
// The identity in that name is the HARNESS session, which is what recordReview
// writes with. It is not the eight-hex commit namespace that names a session's
// commit scripts and message files. The two were one identity until the native
// port: scripts/dev/review_gate.py wrote the artifact and scripts/dev/
// commit_helper.py read it back through that script, so neither side could name
// it differently. The port gave the reader a name of its own and it stopped
// finding the file, which blocks every spec closure rather than one.
func ReviewArtifactPath(root, spec string) (string, error) {
	session, err := lepath.ResolveSession(root, false)
	if err != nil {
		return "", err
	}

	return relativeReviewPath(root, reviewArtifactPath(root, specStem(spec), session.ID)), nil
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
			artifact.Files = append(artifact.Files, ReviewedFile{Hash: match[1], Mode: match[2], Path: match[3]})
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
	slices.Sort(out)
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

// IsReviewCode answers whether a review pins path: a Makefile, or a file with
// one of reviewCodeSuffixes. It is the one predicate the recorder's check here
// and the commit gate (internal/le/commit, CheckReview) both read.
//
// Markdown is not reviewable, and that is the exemption a closure needs: a spec
// and its closure sections under plan/ change after the clean review (a NOTE
// fixed, the Review Gate and the learned summary filled), and a NOTE never
// re-opens a round. Pinning them would turn each of those edits into a stale
// review and a fresh round over unchanged code.
func IsReviewCode(path string) bool {
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
