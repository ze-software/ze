// Design: docs/architecture/testing/verify-freshness-scope.md -- native verification certificates
// Related: run.go -- the full verification run that writes this certificate
package verifyengine

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/job"
)

// SkippedSuites reads the suite omission list using the dotted-environment
// equivalence used by native le commands.
func SkippedSuites() string {
	return skippedSuites(os.Environ())
}

func skippedSuites(environ []string) string {
	var equivalent string
	normalizer := strings.NewReplacer(".", "_")
	for _, entry := range environ {
		name, value, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		normalized := normalizer.Replace(strings.ToLower(name))
		if normalized != "ze_skip_suites" {
			continue
		}
		if name == "ZE_SKIP_SUITES" {
			return value
		}
		if equivalent == "" {
			equivalent = value
		}
	}
	return equivalent
}

const (
	// StatusPath is the root-relative verification certificate.
	StatusPath = "tmp/ze-verify.status"
	// ManifestPath is the root-relative per-path fingerprint beside the certificate.
	ManifestPath = "tmp/ze-verify-manifest.txt"

	movedDuringRun = "MOVED-DURING-RUN"
	treeMoved      = "tree-moved-during-run"
)

// Certificate is the last verification verdict recorded for a checkout.
type Certificate struct {
	Exit      int    `json:"exit"`
	Timestamp string `json:"timestamp"`
	Mode      string `json:"mode"`
	Skipped   string `json:"skipped"`
	GitSHA    string `json:"git-sha"`
	TreeHash  string `json:"tree-hash"`
}

// Text renders the status file byte for byte in its established format.
func (c Certificate) Text() string {
	var text textbuf.Buffer
	text.Reset().Str("exit=").Int(int64(c.Exit)).Byte('\n')
	text.Str("timestamp=").Str(c.Timestamp).Byte('\n')
	text.Str("mode=").Str(c.Mode).Byte('\n')
	text.Str("skipped=").Str(c.Skipped).Byte('\n')
	text.Str("git_sha=").Str(c.GitSHA).Byte('\n')
	text.Str("tree_hash=").Str(c.TreeHash).Byte('\n')
	return text.String()
}

// WriteRequest specifies one verification certificate. Start MUST be captured
// before the verified work begins. GitSHA identifies the commit the work read.
type WriteRequest struct {
	Exit    int
	Mode    string
	Skipped string
	GitSHA  string
	Start   job.TreeSnapshot
	At      time.Time
}

// WriteCertificate records the verification result and its per-path manifest.
// It writes each file atomically, so a reader sees the old certificate or the
// complete new certificate.
func WriteCertificate(root string, request WriteRequest) (Certificate, error) {
	if request.Mode == "" {
		request.Mode = Mode
	}
	if request.GitSHA == "" {
		request.GitSHA = job.Head(root)
	}
	if request.At.IsZero() {
		request.At = time.Now()
	}
	if request.Start.Hash == "" {
		request.Start = job.SnapshotTree(root)
	}

	end := job.SnapshotTree(root)
	treeHash := request.Start.Hash
	if end.Hash != request.Start.Hash {
		treeHash = treeMoved
	}
	manifest := manifestText(request.Start.Manifest, end.Manifest)
	if err := atomicWrite(root, ManifestPath, []byte(manifest)); err != nil {
		return Certificate{}, fmt.Errorf("write verify manifest: %w", err)
	}

	certificate := Certificate{
		Exit:      request.Exit,
		Timestamp: request.At.UTC().Format(time.RFC3339),
		Mode:      request.Mode,
		Skipped:   request.Skipped,
		GitSHA:    request.GitSHA,
		TreeHash:  treeHash,
	}
	if err := atomicWrite(root, StatusPath, []byte(certificate.Text())); err != nil {
		return Certificate{}, fmt.Errorf("write verify status: %w", err)
	}
	return certificate, nil
}

func manifestText(start, end map[string]string) string {
	paths := make([]string, 0, len(start)+len(end))
	for rel := range start {
		paths = append(paths, rel)
	}
	for rel := range end {
		if _, exists := start[rel]; !exists {
			paths = append(paths, rel)
		}
	}
	slices.Sort(paths)

	var text textbuf.Buffer
	text.Reset()
	for _, rel := range paths {
		fingerprint := start[rel]
		if end[rel] != fingerprint {
			fingerprint = movedDuringRun
		}
		text.Str(fingerprint).Byte(' ').Str(rel).Byte('\n')
	}
	return text.String()
}

// ReadCertificate reads the status file without evaluating its freshness.
func ReadCertificate(root string) (Certificate, error) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(StatusPath))) //nolint:gosec // the path is a verification artifact under the checkout root
	if err != nil {
		return Certificate{}, err
	}
	fields := make(map[string]string, 6)
	for line := range strings.SplitSeq(strings.TrimSuffix(string(content), "\n"), "\n") {
		key, value, found := strings.Cut(line, "=")
		if found {
			fields[key] = value
		}
	}
	exit, err := strconv.Atoi(fields["exit"])
	if err != nil {
		return Certificate{}, fmt.Errorf("parse verify exit %q: %w", fields["exit"], err)
	}
	return Certificate{
		Exit:      exit,
		Timestamp: fields["timestamp"],
		Mode:      fields["mode"],
		Skipped:   fields["skipped"],
		GitSHA:    fields["git_sha"],
		TreeHash:  fields["tree_hash"],
	}, nil
}

// Freshness is the structured answer to a status check.
type Freshness struct {
	Fresh       bool     `json:"fresh"`
	Mode        string   `json:"mode,omitempty"`
	Timestamp   string   `json:"timestamp,omitempty"`
	GitSHA      string   `json:"git-sha,omitempty"`
	ScopedPaths []string `json:"scoped-paths,omitempty"`
	Reason      string   `json:"reason"`
}

// Text renders the established one-line freshness answer.
func (f Freshness) Text() string { return f.Reason + "\n" }

// CheckCertificate reports whether the current checkout still matches the last
// successful certificate. Paths restrict the question to those files and
// directories.
func CheckCertificate(root string, paths []string) Freshness {
	certificate, err := ReadCertificate(root)
	if errors.Is(err, os.ErrNotExist) {
		return stale("STALE: no status file (never verified)")
	}
	if err != nil {
		return stale("STALE: status file is unreadable (run a full verify to replace it)")
	}
	if certificate.Exit != 0 {
		return stale(fmt.Sprintf("STALE: last verify failed (exit=%d, at %s)", certificate.Exit, certificate.Timestamp))
	}
	if certificate.Skipped != "" {
		return stale(fmt.Sprintf("STALE: last pass skipped suites (%s) at %s", certificate.Skipped, certificate.Timestamp))
	}
	if len(paths) != 0 {
		return checkScoped(root, certificate, paths)
	}
	if job.TreeHash(root) != certificate.TreeHash {
		return stale(fmt.Sprintf("STALE: tree changed since last PASS at %s", certificate.Timestamp))
	}
	mode := certificate.Mode
	if mode == "" {
		mode = Mode
	}
	return Freshness{
		Fresh:     true,
		Mode:      mode,
		Timestamp: certificate.Timestamp,
		GitSHA:    certificate.GitSHA,
		Reason: fmt.Sprintf("FRESH(%s): tree unchanged since PASS at %s (sha %s)",
			mode, certificate.Timestamp, certificate.GitSHA),
	}
}

func checkScoped(root string, certificate Certificate, paths []string) Freshness {
	recorded, err := readManifest(filepath.Join(root, filepath.FromSlash(ManifestPath)))
	if errors.Is(err, os.ErrNotExist) {
		return stale("STALE: no per-path manifest (PASS predates scoped checking)")
	}
	if err != nil {
		return stale("STALE: per-path manifest is unreadable (run a full verify to replace it)")
	}
	if reason := scopedChange(root, certificate, recorded, paths); reason != "" {
		return stale(reason)
	}

	mode := certificate.Mode
	if mode == "" {
		mode = Mode
	}
	var text textbuf.Buffer
	return Freshness{
		Fresh:       true,
		Mode:        mode,
		Timestamp:   certificate.Timestamp,
		GitSHA:      certificate.GitSHA,
		ScopedPaths: append([]string(nil), paths...),
		Reason: text.Reset().Str("FRESH(").Str(mode).Str("): ").Int(int64(len(paths))).
			Str(" scoped path(s) unchanged since PASS at ").Str(certificate.Timestamp).
			Str(" (sha ").Str(certificate.GitSHA).Byte(')').String(),
	}
}

// scopedChange answers why the scoped paths no longer hold what the stages
// read, or the empty string when they still hold it.
//
// The question is asked about CONTENT, and asking it any other way is what made
// a scoped PASS worthless in a checkout that many sessions commit to. The
// manifest is HEAD-relative, so a path the run read as dirty stops being dirty
// the moment a commit absorbs it unchanged. Comparing dirty SETS therefore
// calls a path stale for moving out of the worktree into a commit, and
// comparing the certificate's commit against HEAD calls EVERY scoped path stale
// as soon as any other session commits anything at all. Neither answer is about
// the files the caller asked about.
//
// So a recorded row is compared against the file on disk, and the commits made
// since the PASS are asked only about the scoped paths that carry no row. Those
// paths were identical to the certificate's commit when the run read them, and
// that commit is the only record of what they held.
func scopedChange(root string, certificate Certificate, recorded map[string]string, paths []string) string {
	for rel, fingerprint := range recorded {
		if !underScope(rel, paths) {
			continue
		}
		if fingerprint == movedDuringRun {
			return scopedReason("a scoped path moved while the run was in flight, so no stage judged the content it now holds", certificate)
		}
		if job.Fingerprint(root, rel) != fingerprint {
			return scopedReason("a scoped path no longer holds the content the stages read", certificate)
		}
	}
	for rel := range job.DirtyManifest(root) {
		if _, judged := recorded[rel]; judged || !underScope(rel, paths) {
			continue
		}
		return scopedReason("a scoped path is dirty and no stage judged it", certificate)
	}

	head := job.Head(root)
	if head == certificate.GitSHA {
		return ""
	}
	moved, err := job.PathsChangedBetween(root, certificate.GitSHA, head)
	if err != nil {
		return scopedReason("the commit this PASS read can no longer be compared with HEAD", certificate)
	}
	for _, rel := range moved {
		if _, judged := recorded[rel]; judged || !underScope(rel, paths) {
			continue
		}
		return scopedReason("a commit made since the PASS changed a scoped path", certificate)
	}
	return ""
}

// scopedReason renders one stale answer. Every one of them names the PASS it
// judged, because a checkout can hold several certificates over a morning and
// the reason alone does not say which one the reader is being told about.
func scopedReason(what string, certificate Certificate) string {
	var text textbuf.Buffer
	return text.Reset().Str("STALE: ").Str(what).
		Str(" (PASS at ").Str(certificate.Timestamp).
		Str(", sha ").Str(certificate.GitSHA).Byte(')').String()
}

// underScope answers whether one path IS the scope or sits inside it. An empty
// scope reaches nothing, which is why CheckCertificate asks the whole-tree
// question rather than this one when the caller names no path.
func underScope(rel string, paths []string) bool {
	for _, scope := range paths {
		if rel == scope || strings.HasPrefix(rel, scope+"/") {
			return true
		}
	}
	return false
}

func stale(reason string) Freshness { return Freshness{Reason: reason} }

func readManifest(path string) (map[string]string, error) {
	content, err := os.ReadFile(path) //nolint:gosec // the path is a verification artifact under the checkout root
	if err != nil {
		return nil, err
	}
	manifest := make(map[string]string)
	for row := range strings.SplitSeq(strings.TrimSuffix(string(content), "\n"), "\n") {
		if row == "" {
			continue
		}
		fingerprint, rel, found := strings.Cut(row, " ")
		if !found {
			return nil, fmt.Errorf("manifest row has no path: %q", row)
		}
		manifest[rel] = fingerprint
	}
	return manifest, nil
}

// atomicWrite publishes one verification artifact through a temporary file in
// the same directory. Every artifact is owner-only: the reader is the same
// account that ran the verification.
func atomicWrite(root, rel string, content []byte) error {
	const mode os.FileMode = 0o600
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".le-write-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()

	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return err
	}
	written, err := file.Write(content)
	if err != nil {
		_ = file.Close()
		return err
	}
	if written != len(content) {
		_ = file.Close()
		return io.ErrShortWrite
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
