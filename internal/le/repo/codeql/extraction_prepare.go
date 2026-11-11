// Design: docs/architecture/testing/ci-workflows.md -- exact inputs for CodeQL's extraction proof.
package codeql

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const authoredSentinel = "test/codeql/authored_sentinel.go"

type extractionManifest struct {
	SourceRoot   string `json:"source-root"`
	AuthoredFile string `json:"authored-file"`
	AuthoredHash string `json:"authored-sha256"`
	CacheFile    string `json:"cache-file"`
	CacheHash    string `json:"cache-sha256"`
	Tags         string `json:"tags"`
	CacheRefusal string `json:"cache-refusal"`
}

// ExtractionPreparationReport names the task-owned inputs retained until the
// corresponding CI database check and its evidence have been reviewed.
type ExtractionPreparationReport struct {
	ManifestFile string `json:"manifest-file"`
	CacheFile    string `json:"cache-file"`
	CacheRefusal string `json:"cache-refusal"`
}

// Text reports where the exact input identities and actual refusal were saved.
func (report ExtractionPreparationReport) Text() string {
	return "prepared CodeQL extraction manifest: " + report.ManifestFile +
		"\nexplicit cache request refused: " + report.CacheRefusal + "\n"
}

// PrepareExtraction reserves a new manifest before creating cache inputs. The
// caller MUST preserve the sources and Go context until the traced builds and
// CheckExtraction finish. Only the cache directory created here is task-owned.
func PrepareExtraction(ctx context.Context, root, file, tags string) (report ExtractionPreparationReport, err error) {
	file, err = filepath.Abs(file)
	if err != nil {
		return report, fmt.Errorf("resolve CodeQL manifest: %w", err)
	}
	manifest, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return report, fmt.Errorf("CodeQL manifest already exists: %s; choose a new task-owned evidence file", file)
		}
		return report, fmt.Errorf("create CodeQL manifest: %w", err)
	}
	completed, closed := false, false
	cacheOwned := ""
	defer func() {
		if completed {
			return
		}
		if !closed {
			err = errors.Join(err, manifest.Close())
		}
		err = errors.Join(err, os.Remove(file))
		if cacheOwned != "" {
			err = errors.Join(err, os.RemoveAll(cacheOwned))
		}
	}()
	root, err = filepath.Abs(root)
	if err != nil {
		return report, fmt.Errorf("resolve CodeQL source root: %w", err)
	}
	target, err := BuildContext(ctx, root, tags)
	if err != nil {
		return report, err
	}
	authored := filepath.Join(root, filepath.FromSlash(authoredSentinel))
	if err := requireAuthoredSentinel(ctx, root, authored); err != nil {
		return report, err
	}
	matched, err := target.MatchFile(filepath.Dir(authored), filepath.Base(authored))
	if err != nil {
		return report, fmt.Errorf("match authored CodeQL sentinel: %w", err)
	}
	if !matched {
		return report, errors.New("authored CodeQL sentinel is excluded by the supplied build context")
	}
	cache := filepath.Join(root, filepath.FromSlash(cacheDirectory))
	if err := os.MkdirAll(cache, 0o750); err != nil {
		return report, fmt.Errorf("create CodeQL cache population: %w", err)
	}
	cacheOwned, err = os.MkdirTemp(cache, "codeql-extraction-")
	if err != nil {
		return report, fmt.Errorf("create owned CodeQL cache sentinel directory: %w", err)
	}
	cacheFile := filepath.Join(cacheOwned, "sentinel.go")
	if err := os.WriteFile(cacheFile, []byte("package codeqlcache\n\nfunc ExcludedSentinel() int { return 23 }\n"), 0o600); err != nil {
		return report, fmt.Errorf("write owned CodeQL cache sentinel: %w", err)
	}
	relative, err := filepath.Rel(root, cacheOwned)
	if err != nil {
		return report, fmt.Errorf("place CodeQL cache request: %w", err)
	}
	refusal := Guard(ctx, root, target, []string{"./" + filepath.ToSlash(relative)})
	if refusal == nil {
		return report, errors.New("CodeQL admitted its explicit cache sentinel; no extraction proof may be recorded")
	}
	if _, cachePath := errors.AsType[cachePathError](refusal); !cachePath {
		return report, fmt.Errorf("explicit cache request failed before its cache admission check: %w", refusal)
	}
	proof := extractionManifest{
		SourceRoot: root, AuthoredFile: authored, CacheFile: cacheFile, Tags: tags, CacheRefusal: refusal.Error(),
	}
	proof.AuthoredHash, err = extractionSourceHash(authored)
	if err != nil {
		return report, err
	}
	proof.CacheHash, err = extractionSourceHash(cacheFile)
	if err != nil {
		return report, err
	}
	encoder := json.NewEncoder(manifest)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(proof); err != nil {
		return report, fmt.Errorf("save CodeQL manifest: %w", err)
	}
	if err := manifest.Sync(); err != nil {
		return report, fmt.Errorf("sync CodeQL manifest: %w", err)
	}
	err = manifest.Close()
	closed = true
	if err != nil {
		return report, fmt.Errorf("close CodeQL manifest: %w", err)
	}
	completed = true
	return ExtractionPreparationReport{ManifestFile: file, CacheFile: cacheFile, CacheRefusal: refusal.Error()}, nil
}

func requireAuthoredSentinel(ctx context.Context, root, file string) error {
	if file != filepath.Join(root, filepath.FromSlash(authoredSentinel)) {
		return errors.New("manifest does not name the tracked authored sentinel")
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve authored sentinel root: %w", err)
	}
	canonicalFile, err := filepath.EvalSymlinks(file)
	if err != nil {
		return fmt.Errorf("resolve authored sentinel: %w", err)
	}
	if canonicalFile != filepath.Join(canonicalRoot, filepath.FromSlash(authoredSentinel)) {
		return errors.New("manifest does not name the tracked authored sentinel: source resolves to another path")
	}
	info, err := os.Lstat(file)
	if err != nil {
		return fmt.Errorf("read authored sentinel identity: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("tracked authored sentinel is not a regular source file")
	}
	command := exec.CommandContext(ctx, "git", "ls-files", "--error-unmatch", "-z", "--", authoredSentinel)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("identify tracked authored sentinel: %w", err)
	}
	if !bytes.Equal(output, append([]byte(authoredSentinel), 0)) {
		return errors.New("manifest does not name the tracked authored sentinel: Git returned another identity")
	}
	return nil
}

func extractionSourceHash(file string) (digest string, err error) {
	source, err := os.Open(file)
	if err != nil {
		return "", fmt.Errorf("open CodeQL sentinel: %w", err)
	}
	defer func() { err = errors.Join(err, source.Close()) }()
	hash := sha256.New()
	if _, err := io.Copy(hash, source); err != nil {
		return "", fmt.Errorf("hash CodeQL sentinel: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
