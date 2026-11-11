// Design: docs/architecture/testing/ci-workflows.md -- CodeQL's tracked Go population.
// Detail: context.go -- the actual Go command's complete build context.
package codeql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const cacheDirectory = "gokrazy/modcache"

// Select derives buildable directories from the Git index, never a root list.
// It MUST finish Guard before returning arguments to a traced build. The caller
// MUST preserve the supplied context and checkout until that build completes.
func Select(ctx context.Context, root string, target *build.Context) ([]string, error) {
	if err := validateBuildContext(ctx, root, target); err != nil {
		return nil, err
	}
	boundary, err := newCacheBoundary(root)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "git", "ls-files", "-z", "--", "*.go")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("enumerate tracked Go for CodeQL: %w", err)
	}
	directories := map[string]bool{}
	classifiedDirectories := map[string]bool{}
	for _, raw := range bytes.Split(out, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		file := string(raw)
		excluded, err := classifyTrackedGo(file)
		if err != nil {
			return nil, err
		}
		if excluded {
			continue
		}
		full := filepath.Join(root, filepath.FromSlash(file))
		if err := boundary.check(full); err != nil {
			return nil, fmt.Errorf("tracked Go %s: %w", file, err)
		}
		info, err := os.Stat(full)
		if err != nil {
			return nil, fmt.Errorf("read tracked Go %s: %w", file, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("unclassified tracked Go %s: not a regular source file", file)
		}
		dir := path.Dir(file)
		if !classifiedDirectories[dir] {
			if err := classifyModule(root, dir); err != nil {
				return nil, err
			}
			classifiedDirectories[dir] = true
		}
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		matched, err := target.MatchFile(filepath.Dir(full), filepath.Base(full))
		if err != nil {
			return nil, fmt.Errorf("match tracked Go %s: %w", file, err)
		}
		if !matched {
			continue
		}
		if !target.CgoEnabled {
			usesCgo, err := importsCgo(full)
			if err != nil {
				return nil, err
			}
			if usesCgo {
				continue
			}
		}
		directories[dir] = true
	}
	packages := make([]string, 0, len(directories))
	for dir := range directories {
		if dir == "." {
			packages = append(packages, ".")
			continue
		}
		packages = append(packages, "./"+dir)
	}
	slices.Sort(packages)
	if err := guard(ctx, root, target, packages, boundary); err != nil {
		return nil, err
	}
	return packages, nil
}

// classifyTrackedGo makes exclusions explicit. Every other ordinary authored
// directory is admitted without an enumeration of repository roots.
func classifyTrackedGo(file string) (bool, error) {
	if path.Clean(file) != file {
		return false, fmt.Errorf("unclassified tracked Go %q: non-canonical path", file)
	}
	if strings.ContainsAny(file, "\n\r\x00\\") {
		return false, fmt.Errorf("unclassified tracked Go %q: unsupported path boundary", file)
	}
	for _, prefix := range []string{"vendor/", cacheDirectory + "/"} {
		if strings.HasPrefix(file, prefix) {
			return true, nil
		}
	}
	segments := strings.Split(file, "/")
	if slices.Contains(segments, "testdata") {
		return true, nil
	}
	// These are tool pins, ruleguard input and the documented separate-module
	// example, respectively; none belongs to the main module's traced builds.
	for _, excluded := range []string{"tools.go", ".golangci/ruleguard/modern.go", "examples/plugin/go/main.go"} {
		if file == excluded {
			return true, nil
		}
	}
	for _, segment := range segments {
		if strings.HasPrefix(segment, ".") {
			return false, fmt.Errorf("unclassified tracked Go %s: hidden Go path", file)
		}
		if strings.HasPrefix(segment, "_") {
			return false, fmt.Errorf("unclassified tracked Go %s: ignored Go path", file)
		}
	}
	return false, nil
}

// The ancestor walk is bounded by the tracked directory's segment count.
func classifyModule(root, dir string) error {
	for dir != "." {
		module := filepath.Join(root, filepath.FromSlash(dir), "go.mod")
		_, err := os.Stat(module)
		if err == nil {
			return fmt.Errorf("unclassified tracked Go directory %s: nested module %s", dir, module)
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("classify tracked Go directory %s: %w", dir, err)
		}
		dir = path.Dir(dir)
	}
	return nil
}

func importsCgo(file string) (bool, error) {
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
	if err != nil {
		return false, fmt.Errorf("parse tracked Go imports %s: %w", file, err)
	}
	for _, imported := range parsed.Imports {
		importPath, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			return false, fmt.Errorf("decode tracked Go import in %s: %w", file, err)
		}
		if importPath == "C" {
			return true, nil
		}
	}
	return false, nil
}

// Guard refuses explicit cache requests and any dependency or source resolving
// into the cache. Callers MUST run it before each traced build, with that build's
// complete context and exact package arguments, and preserve both until build.
func Guard(ctx context.Context, root string, target *build.Context, packages []string) error {
	if err := validateBuildContext(ctx, root, target); err != nil {
		return err
	}
	boundary, err := newCacheBoundary(root)
	if err != nil {
		return err
	}
	return guard(ctx, root, target, packages, boundary)
}

func guard(ctx context.Context, root string, target *build.Context, packages []string, boundary *cacheBoundary) error {
	if len(packages) == 0 {
		return errors.New("CodeQL selected no buildable tracked packages; refuse an unbounded build")
	}
	for _, pkg := range packages {
		if strings.Contains(pkg, "...") {
			return fmt.Errorf("CodeQL needs explicit package directories, not %q", pkg)
		}
		if filepath.IsAbs(pkg) {
			return fmt.Errorf("CodeQL needs repository-relative package directories, not %q", pkg)
		}
		full := filepath.Join(root, filepath.FromSlash(pkg))
		rel, err := filepath.Rel(root, full)
		if err != nil {
			return fmt.Errorf("place CodeQL root %q: %w", pkg, err)
		}
		if escapesRoot(rel) {
			return fmt.Errorf("CodeQL package %q escapes the repository", pkg)
		}
		if err := boundary.check(full); err != nil {
			return err
		}
	}
	const fields = "-json=ImportPath,Dir,Incomplete,Error,DepsErrors,GoFiles,CgoFiles,CFiles,CXXFiles,MFiles,HFiles,FFiles,SFiles,SwigFiles,SwigCXXFiles,SysoFiles,EmbedFiles"
	args := []string{"list", "-deps", fields, "-tags", strings.Join(target.BuildTags, ","), "--"}
	args = append(args, packages...)
	out, err := captureGo(ctx, root, contextEnvironment(target), args...)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(out))
	count := 0
	for {
		var record dependency
		if err := decoder.Decode(&record); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("decode CodeQL dependency graph: %w", err)
		}
		if err := record.admit(boundary); err != nil {
			return err
		}
		count++
	}
	if count == 0 {
		return errors.New("go list returned no CodeQL dependency records")
	}
	return nil
}

type dependency struct {
	ImportPath                                                                                                          string
	Dir                                                                                                                 string
	Incomplete                                                                                                          bool
	Error                                                                                                               *struct{ Err string }
	DepsErrors                                                                                                          []struct{ Err string }
	GoFiles, CgoFiles, CFiles, CXXFiles, MFiles, HFiles, FFiles, SFiles, SwigFiles, SwigCXXFiles, SysoFiles, EmbedFiles []string
}

func (record *dependency) admit(boundary *cacheBoundary) error {
	if record.Error != nil {
		return fmt.Errorf("go list %s: %s", record.ImportPath, record.Error.Err)
	}
	if len(record.DepsErrors) != 0 {
		return fmt.Errorf("go list %s dependency: %s", record.ImportPath, record.DepsErrors[0].Err)
	}
	if record.Incomplete {
		return fmt.Errorf("go list %s returned an incomplete dependency", record.ImportPath)
	}
	if record.ImportPath == "" {
		return errors.New("go list returned a dependency without an import path")
	}
	if !filepath.IsAbs(record.Dir) {
		return fmt.Errorf("go list %s returned no absolute dependency directory", record.ImportPath)
	}
	if err := boundary.check(record.Dir); err != nil {
		return fmt.Errorf("go list dependency %s: %w", record.ImportPath, err)
	}
	for _, files := range [][]string{record.GoFiles, record.CgoFiles, record.CFiles, record.CXXFiles, record.MFiles, record.HFiles, record.FFiles, record.SFiles, record.SwigFiles, record.SwigCXXFiles, record.SysoFiles, record.EmbedFiles} {
		for _, file := range files {
			if err := boundary.check(filepath.Join(record.Dir, file)); err != nil {
				return fmt.Errorf("go list dependency %s source %s: %w", record.ImportPath, file, err)
			}
		}
	}
	return nil
}

// cacheBoundary is constructed from canonical checkout paths. A nil receiver is
// not usable; only newCacheBoundary supplies it to the admission routines.
type cacheBoundary struct {
	lexical   string
	canonical string
}

// cachePathError distinguishes a proved cache source from a path whose
// canonical identity could not be read. Both refuse admission.
type cachePathError struct{ cause error }

func (err cachePathError) Error() string { return err.cause.Error() }

func newCacheBoundary(root string) (*cacheBoundary, error) {
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("canonicalize CodeQL repository: %w", err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return nil, fmt.Errorf("resolve CodeQL repository: %w", err)
	}
	cache := filepath.Join(canonical, filepath.FromSlash(cacheDirectory))
	resolved, err := filepath.EvalSymlinks(cache)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("canonicalize %s: %w", cacheDirectory, err)
		}
		resolved = cache
	}
	return &cacheBoundary{lexical: cache, canonical: resolved}, nil
}

func (boundary *cacheBoundary) check(file string) error {
	full, err := filepath.Abs(file)
	if err != nil {
		return fmt.Errorf("resolve CodeQL path %s: %w", file, err)
	}
	if insideDirectory(boundary.lexical, full) {
		return cachePathError{fmt.Errorf("CodeQL refuses %s path %s", cacheDirectory, file)}
	}
	canonical, err := filepath.EvalSymlinks(full)
	if err != nil {
		return fmt.Errorf("canonicalize CodeQL path %s: %w", file, err)
	}
	if insideDirectory(boundary.lexical, canonical) {
		return cachePathError{fmt.Errorf("CodeQL refuses canonical %s path %s -> %s", cacheDirectory, file, canonical)}
	}
	if insideDirectory(boundary.canonical, canonical) {
		return cachePathError{fmt.Errorf("CodeQL refuses canonical %s path %s -> %s", cacheDirectory, file, canonical)}
	}
	return nil
}

func insideDirectory(directory, file string) bool {
	prefix := strings.TrimSuffix(directory, string(filepath.Separator)) + string(filepath.Separator)
	return file == directory || strings.HasPrefix(file, prefix)
}

func escapesRoot(relative string) bool {
	return relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
