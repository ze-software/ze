// Design: docs/architecture/testing/ci-workflows.md -- database evidence, not a package selector's answer.
package codeql

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// GoFile requires an extracted Go AST, unlike ExtractedOrExternalFile's
// diagnostic-only references. A missing relative path must not remove an
// outside-source GoFile from the absolute-path cache check.
const extractedGoFilesQuery = `import go

from GoFile file, string relative
where
  relative = file.getRelativePath()
  or (not exists(file.getRelativePath()) and relative = "")
select relative, file.getAbsolutePath()
`

const extractionQueryPack = `name: ze/extraction-proof
version: 0.0.0
dependencies:
  codeql/go-all: "*"
`

// ExtractionReport identifies the complete decoded database answer and the
// sources whose identities were checked before invoking CodeQL.
type ExtractionReport struct {
	ManifestFile   string `json:"manifest-file"`
	Database       string `json:"database"`
	Executable     string `json:"executable"`
	ProofDirectory string `json:"proof-directory"`
	AuthoredFile   string `json:"authored-file"`
	AuthoredHash   string `json:"authored-sha256"`
	CacheFile      string `json:"cache-file"`
	CacheHash      string `json:"cache-sha256"`
	ExtractedFiles int    `json:"extracted-go-files"`
}

// Text states only the database predicates established by this check.
func (report ExtractionReport) Text() string {
	return "authored sentinel extracted; cache sources excluded\nCodeQL proof: " + report.ProofDirectory + "\n"
}

// CheckExtraction queries the finalized database produced by the normal
// analysis. The caller MUST preserve prepared inputs until this returns.
// Query sources, decoded rows and the report stay beside the manifest; owned
// compilation caches and the intermediate BQRS are removed before returning.
func CheckExtraction(ctx context.Context, root, file, database, executable string) (report ExtractionReport, err error) {
	root, err = filepath.Abs(root)
	if err != nil {
		return report, fmt.Errorf("resolve CodeQL source root: %w", err)
	}
	file, err = filepath.Abs(file)
	if err != nil {
		return report, fmt.Errorf("resolve CodeQL manifest: %w", err)
	}
	proof, err := readExtractionManifest(file)
	if err != nil {
		return report, err
	}
	boundary, err := newCacheBoundary(root)
	if err != nil {
		return report, err
	}
	if err := validateExtractionManifest(ctx, root, &proof, boundary); err != nil {
		return report, err
	}
	database, err = filepath.Abs(database)
	if err != nil {
		return report, fmt.Errorf("resolve CodeQL database: %w", err)
	}
	executable, err = exec.LookPath(executable)
	if err != nil {
		return report, fmt.Errorf("locate CodeQL executable: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return report, fmt.Errorf("resolve CodeQL executable: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return report, fmt.Errorf("place CodeQL executable: %w", err)
	}
	packs, err := extractionBundlePacks(executable)
	if err != nil {
		return report, fmt.Errorf("CodeQL query failed before execution: %w", err)
	}
	directory, err := os.MkdirTemp(filepath.Dir(file), "codeql-proof-")
	if err != nil {
		return report, fmt.Errorf("create owned CodeQL proof directory: %w", err)
	}
	compiled := filepath.Join(directory, "compilation")
	common := filepath.Join(directory, "common-caches")
	bqrs := filepath.Join(directory, "extracted-go-files.bqrs")
	defer func() {
		err = errors.Join(err, os.RemoveAll(compiled))
		err = errors.Join(err, os.RemoveAll(common))
		if remove := os.Remove(bqrs); remove != nil {
			if !errors.Is(remove, os.ErrNotExist) {
				err = errors.Join(err, remove)
			}
		}
	}()
	query := filepath.Join(directory, "extracted-go-files.ql")
	if err := os.WriteFile(query, []byte(extractedGoFilesQuery), 0o600); err != nil {
		return report, fmt.Errorf("write owned CodeQL query: %w", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "qlpack.yml"), []byte(extractionQueryPack), 0o600); err != nil {
		return report, fmt.Errorf("write owned CodeQL query pack: %w", err)
	}
	// Only libraries in the already selected CodeQL bundle are searched. This
	// invokes no package installation or download. An empty owned common-cache
	// directory keeps the user's downloaded packs out of dependency resolution.
	if err := runExtractionCodeQL(ctx, root, executable, "query", "run", "--database="+database,
		"--search-path="+packs, "--no-default-compilation-cache", "--compilation-cache="+compiled,
		"--common-caches="+common, "--threads=1", "--output="+bqrs, query); err != nil {
		return report, fmt.Errorf("CodeQL query failed; inspect %s: %w", query, err)
	}
	rows := filepath.Join(directory, "extracted-go-files.csv")
	if err := runExtractionCodeQL(ctx, root, executable, "bqrs", "decode", "--format=csv", "--no-titles", "--output="+rows, bqrs); err != nil {
		return report, fmt.Errorf("CodeQL decode failed; inspect %s: %w", query, err)
	}
	count, err := checkExtractedGoFiles(root, rows, &proof, boundary)
	if err != nil {
		return report, fmt.Errorf("CodeQL database proof in %s: %w", directory, err)
	}
	if err := validateExtractionManifest(ctx, root, &proof, boundary); err != nil {
		return report, err
	}
	report = ExtractionReport{
		ManifestFile: file, Database: database, Executable: executable, ProofDirectory: directory,
		AuthoredFile: proof.AuthoredFile, AuthoredHash: proof.AuthoredHash,
		CacheFile: proof.CacheFile, CacheHash: proof.CacheHash, ExtractedFiles: count,
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return report, fmt.Errorf("encode CodeQL extraction report: %w", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "report.json"), append(data, '\n'), 0o600); err != nil {
		return report, fmt.Errorf("save CodeQL extraction report: %w", err)
	}
	return report, nil
}

// extractionBundlePacks uses the selected bundle's actual layout:
// <bundle>/codeql/codeql and <bundle>/qlpacks. A missing library refuses before
// CodeQL can fall back to a different installation or a downloaded pack.
func extractionBundlePacks(executable string) (string, error) {
	bundle := filepath.Dir(filepath.Dir(executable))
	packs, err := filepath.EvalSymlinks(filepath.Join(bundle, "qlpacks"))
	if err != nil {
		return "", fmt.Errorf("bundled Go library is unavailable: %w", err)
	}
	if !insideDirectory(bundle, packs) {
		return "", errors.New("bundled Go library is unavailable: qlpacks resolves outside the selected bundle")
	}
	library, err := filepath.EvalSymlinks(filepath.Join(packs, "codeql", "go-all"))
	if err != nil {
		return "", fmt.Errorf("bundled Go library is unavailable: %w", err)
	}
	if !insideDirectory(packs, library) {
		return "", errors.New("bundled Go library is unavailable: codeql/go-all resolves outside the selected bundle")
	}
	versions, err := os.ReadDir(library)
	if err != nil {
		return "", fmt.Errorf("bundled Go library is unavailable: %w", err)
	}
	found := false
	for _, version := range versions {
		if !version.IsDir() {
			continue
		}
		for _, name := range []string{"qlpack.yml", "go.qll"} {
			file := filepath.Join(library, version.Name(), name)
			canonical, err := filepath.EvalSymlinks(file)
			if err != nil {
				return "", fmt.Errorf("bundled Go library is unavailable: %w", err)
			}
			if !insideDirectory(packs, canonical) {
				return "", errors.New("bundled Go library is unavailable: its source resolves outside the selected bundle")
			}
			info, err := os.Stat(canonical)
			if err != nil {
				return "", fmt.Errorf("bundled Go library is unavailable: %w", err)
			}
			if !info.Mode().IsRegular() {
				return "", errors.New("bundled Go library is unavailable: its pack or module is not a regular file")
			}
		}
		found = true
	}
	if !found {
		return "", errors.New("bundled Go library is unavailable: codeql/go-all contains no complete version")
	}
	return packs, nil
}

func readExtractionManifest(file string) (proof extractionManifest, err error) {
	source, err := os.Open(file)
	if err != nil {
		return proof, fmt.Errorf("open CodeQL manifest: %w", err)
	}
	defer func() { err = errors.Join(err, source.Close()) }()
	info, err := source.Stat()
	if err != nil {
		return proof, fmt.Errorf("read CodeQL manifest identity: %w", err)
	}
	if !info.Mode().IsRegular() {
		return proof, errors.New("CodeQL manifest is not a regular file")
	}
	if info.Size() > 64*1024 {
		return proof, errors.New("CodeQL manifest exceeds its 64 KiB bound")
	}
	decoder := json.NewDecoder(source)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&proof); err != nil {
		return proof, fmt.Errorf("read CodeQL manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != nil {
		if !errors.Is(err, io.EOF) {
			return proof, fmt.Errorf("read CodeQL manifest trailing data: %w", err)
		}
	} else {
		return proof, errors.New("CodeQL manifest has trailing data")
	}
	return proof, nil
}

func validateExtractionManifest(ctx context.Context, root string, proof *extractionManifest, boundary *cacheBoundary) error {
	if proof.SourceRoot != root {
		return errors.New("manifest belongs to another checkout")
	}
	if proof.AuthoredFile != filepath.Join(root, filepath.FromSlash(authoredSentinel)) {
		return errors.New("manifest does not name the tracked authored sentinel")
	}
	cache := filepath.Join(root, filepath.FromSlash(cacheDirectory))
	if !filepath.IsAbs(proof.CacheFile) {
		return errors.New("manifest cache sentinel is outside gokrazy/modcache")
	}
	if proof.CacheFile != filepath.Clean(proof.CacheFile) {
		return errors.New("manifest cache sentinel is outside gokrazy/modcache")
	}
	if !insideDirectory(cache, proof.CacheFile) {
		return errors.New("manifest cache sentinel is outside gokrazy/modcache")
	}
	if err := requireAuthoredSentinel(ctx, root, proof.AuthoredFile); err != nil {
		return err
	}
	canonical, err := filepath.EvalSymlinks(proof.CacheFile)
	if err != nil {
		return fmt.Errorf("resolve manifest cache sentinel: %w", err)
	}
	if !insideDirectory(boundary.canonical, canonical) {
		return errors.New("manifest cache sentinel is outside gokrazy/modcache after resolving symlinks")
	}
	info, err := os.Lstat(proof.CacheFile)
	if err != nil {
		return fmt.Errorf("read manifest cache sentinel identity: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("manifest cache sentinel is not a regular source file")
	}
	if proof.CacheRefusal == "" {
		return errors.New("manifest holds no explicit cache request refusal")
	}
	for _, source := range []struct{ name, file, hash string }{
		{"authored", proof.AuthoredFile, proof.AuthoredHash},
		{"cache", proof.CacheFile, proof.CacheHash},
	} {
		digest, err := extractionSourceHash(source.file)
		if err != nil {
			return err
		}
		if digest != source.hash {
			return fmt.Errorf("%s sentinel changed since preparation", source.name)
		}
	}
	return nil
}

func runExtractionCodeQL(ctx context.Context, root, executable string, arguments ...string) error {
	command := exec.CommandContext(ctx, executable, arguments...) //nolint:gosec // the caller explicitly selects CodeQL; every argument is passed without a shell.
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", errors.Join(err, ctx.Err()), strings.TrimSpace(string(output)))
	}
	return nil
}

func checkExtractedGoFiles(root, file string, proof *extractionManifest, boundary *cacheBoundary) (count int, err error) {
	source, err := os.Open(file)
	if err != nil {
		return 0, fmt.Errorf("read extracted Go files: %w", err)
	}
	defer func() { err = errors.Join(err, source.Close()) }()
	reader := csv.NewReader(source)
	reader.FieldsPerRecord = -1
	found := false
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return count, fmt.Errorf("read extracted Go files: %w", err)
		}
		if len(row) != 2 {
			return count, errors.New("extracted Go file result must have exactly two path columns")
		}
		if !filepath.IsAbs(row[1]) {
			return count, errors.New("extracted Go file result has no absolute source path")
		}
		if err := boundary.check(row[1]); err != nil {
			if _, cachePath := errors.AsType[cachePathError](err); cachePath {
				return count, fmt.Errorf("database contains cache source: %w", err)
			}
			return count, fmt.Errorf("cannot establish extracted source identity: %w", err)
		}
		if row[0] != "" {
			if path.IsAbs(row[0]) {
				return count, errors.New("extracted Go file relative path is absolute")
			}
			if path.Clean(row[0]) != row[0] {
				return count, errors.New("extracted Go file relative path is not canonical")
			}
			if escapesRoot(filepath.FromSlash(row[0])) {
				return count, errors.New("extracted Go file relative path escapes the checkout")
			}
			canonical, err := filepath.EvalSymlinks(row[1])
			if err != nil {
				return count, fmt.Errorf("resolve extracted Go source: %w", err)
			}
			relative, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(row[0])))
			if err != nil {
				return count, fmt.Errorf("resolve extracted Go relative source: %w", err)
			}
			if relative != canonical {
				return count, errors.New("extracted Go file path columns name different sources")
			}
		}
		if row[0] == authoredSentinel && row[1] == proof.AuthoredFile {
			found = true
		}
		count++
	}
	if count == 0 {
		return 0, errors.New("database contains no extracted Go files")
	}
	if !found {
		return count, errors.New("authored sentinel was not extracted")
	}
	return count, nil
}
