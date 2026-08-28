// Design: docs/architecture/testing/verify-freshness-scope.md -- native verification certificates
// Related: run.go -- the full verification run that writes this certificate
package verify

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/lejob"
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
	var text strings.Builder
	_, _ = fmt.Fprintf(&text, "exit=%d\n", c.Exit)
	_, _ = fmt.Fprintf(&text, "timestamp=%s\n", c.Timestamp)
	_, _ = fmt.Fprintf(&text, "mode=%s\n", c.Mode)
	_, _ = fmt.Fprintf(&text, "skipped=%s\n", c.Skipped)
	_, _ = fmt.Fprintf(&text, "git_sha=%s\n", c.GitSHA)
	_, _ = fmt.Fprintf(&text, "tree_hash=%s\n", c.TreeHash)
	return text.String()
}

// WriteRequest specifies one verification certificate. Start MUST be captured
// before the verified work begins. GitSHA identifies the commit the work read.
type WriteRequest struct {
	Exit    int
	Mode    string
	Skipped string
	GitSHA  string
	Start   lejob.TreeSnapshot
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
		request.GitSHA = lejob.Head(root)
	}
	if request.At.IsZero() {
		request.At = time.Now()
	}
	if request.Start.Hash == "" {
		request.Start = lejob.SnapshotTree(root)
	}

	end := lejob.SnapshotTree(root)
	treeHash := request.Start.Hash
	if end.Hash != request.Start.Hash {
		treeHash = treeMoved
	}
	manifest := manifestText(request.Start.Manifest, end.Manifest)
	if err := atomicWrite(root, ManifestPath, []byte(manifest), 0o600); err != nil {
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
	if err := atomicWrite(root, StatusPath, []byte(certificate.Text()), 0o600); err != nil {
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
	sort.Strings(paths)

	var text strings.Builder
	for _, rel := range paths {
		fingerprint := start[rel]
		if end[rel] != fingerprint {
			fingerprint = movedDuringRun
		}
		_, _ = fmt.Fprintf(&text, "%s %s\n", fingerprint, rel)
	}
	return text.String()
}

// ReadCertificate reads the status file without evaluating its freshness.
func ReadCertificate(root string) (Certificate, error) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(StatusPath)))
	if err != nil {
		return Certificate{}, err
	}
	fields := make(map[string]string, 6)
	for _, line := range strings.Split(strings.TrimSuffix(string(content), "\n"), "\n") {
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
	if lejob.TreeHash(root) != certificate.TreeHash {
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
	head := lejob.Head(root)
	if head != certificate.GitSHA {
		return stale(fmt.Sprintf("STALE: HEAD moved since PASS at %s (%s -> %s)",
			certificate.Timestamp, certificate.GitSHA, head))
	}
	recorded, err := readManifest(filepath.Join(root, filepath.FromSlash(ManifestPath)))
	if errors.Is(err, os.ErrNotExist) {
		return stale("STALE: no per-path manifest (PASS predates scoped checking)")
	}
	if err != nil {
		return stale("STALE: per-path manifest is unreadable (run a full verify to replace it)")
	}
	live := lejob.DirtyManifest(root)
	recordedRows := scopedRows(recorded, paths)
	if strings.Join(recordedRows, "\n") == strings.Join(scopedRows(live, paths), "\n") {
		mode := certificate.Mode
		if mode == "" {
			mode = Mode
		}
		return Freshness{
			Fresh:       true,
			Mode:        mode,
			Timestamp:   certificate.Timestamp,
			GitSHA:      certificate.GitSHA,
			ScopedPaths: append([]string(nil), paths...),
			Reason: fmt.Sprintf("FRESH(%s): %d scoped path(s) unchanged since PASS at %s (sha %s)",
				mode, len(paths), certificate.Timestamp, certificate.GitSHA),
		}
	}
	for _, row := range recordedRows {
		if strings.HasPrefix(row, movedDuringRun+" ") {
			return stale(fmt.Sprintf("STALE: a scoped path moved while the run was in flight, so no stage judged the content it now holds (PASS at %s)", certificate.Timestamp))
		}
	}
	return stale(fmt.Sprintf("STALE: a scoped path changed since last PASS at %s", certificate.Timestamp))
}

func stale(reason string) Freshness { return Freshness{Reason: reason} }

func readManifest(path string) (map[string]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	manifest := make(map[string]string)
	for _, row := range strings.Split(strings.TrimSuffix(string(content), "\n"), "\n") {
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

func scopedRows(manifest map[string]string, paths []string) []string {
	rows := make([]string, 0, len(manifest))
	for rel, fingerprint := range manifest {
		for _, scope := range paths {
			if rel == scope {
				rows = append(rows, fingerprint+" "+rel)
				break
			}
			if strings.HasPrefix(rel, scope+"/") {
				rows = append(rows, fingerprint+" "+rel)
				break
			}
		}
	}
	sort.Strings(rows)
	return rows
}

func atomicWrite(root, rel string, content []byte, mode os.FileMode) error {
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
