// Design: website/AI.md -- a build stages sources into an isolated Pages artifact
package sitebuild

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/docvalid"
)

const sourceListTimeout = 30 * time.Second

// BuildOptions controls one deterministic site build.
type BuildOptions struct {
	Repository string
	Output     string
	Partial    bool
}

// BuildReport identifies the inputs and artifact produced by Build.
type BuildReport struct {
	SourceDigest string `json:"source-digest"`
	Output       string `json:"output"`
	Files        int    `json:"files"`
}

// Build stages the website source tree, preserves the last complete artifact as
// an incremental seed, refreshes native generated surfaces, and removes every
// source-only input from deployment.
func Build(options BuildOptions) (BuildReport, error) {
	paths, err := resolvePaths(options.Repository, options.Output)
	if err != nil {
		return BuildReport{}, err
	}
	files, err := trackedAndUntrackedSourceFiles(paths)
	if err != nil {
		return BuildReport{}, err
	}
	digest, err := sourceDigest(paths.Source, files)
	if err != nil {
		return BuildReport{}, err
	}
	if options.Partial {
		if info, statErr := os.Stat(paths.Output); statErr != nil || !info.IsDir() {
			return BuildReport{}, fmt.Errorf("partial build requires an existing full artifact: %s", paths.Output)
		}
	} else if err := seedOrCleanArtifact(paths); err != nil {
		return BuildReport{}, err
	}
	if err := stageSources(paths.Source, paths.Output, files); err != nil {
		return BuildReport{}, err
	}
	if err := refreshNativeSurfaces(paths); err != nil {
		return BuildReport{}, err
	}
	if err := removeSourceOnly(paths.Output); err != nil {
		return BuildReport{}, err
	}
	return BuildReport{SourceDigest: digest, Output: paths.Output, Files: len(files)}, nil
}

func trackedAndUntrackedSourceFiles(paths Paths) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), sourceListTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	command.Dir = paths.Source
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("list website sources: %w", err)
	}
	var files []string
	for _, raw := range bytes.Split(output, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		name := filepath.Clean(string(raw))
		info, statErr := os.Lstat(filepath.Join(paths.Source, name))
		if statErr == nil && (info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	return files, nil
}

// sourceDigest computes the build-boundary digest used to detect source races.
func sourceDigest(source string, files []string) (string, error) {
	hash := sha256.New()
	names := append([]string(nil), files...)
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(source, name)
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		hash.Write([]byte(filepath.ToSlash(name)))
		hash.Write([]byte{0})
		if info.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(path)
			if readErr != nil {
				return "", readErr
			}
			hash.Write([]byte(filepath.ToSlash(target)))
		} else {
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return "", readErr
			}
			hash.Write(content)
		}
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func seedOrCleanArtifact(paths Paths) error {
	seed := filepath.Join(filepath.Dir(paths.Repository), "gh-pages")
	keep := func(name string) bool {
		return name != ".git" && !strings.HasPrefix(name, ".git/") && !isSourceOnly(name)
	}
	if filepath.Clean(seed) == filepath.Clean(paths.Output) {
		info, err := os.Stat(seed)
		if os.IsNotExist(err) {
			return cleanArtifact(paths.Output)
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("site artifact is not a directory: %s", seed)
		}
		snapshot, err := os.MkdirTemp(filepath.Dir(seed), ".site-artifact-")
		if err != nil {
			return fmt.Errorf("create site artifact snapshot: %w", err)
		}
		defer os.RemoveAll(snapshot)
		if err := copyTree(seed, snapshot, keep); err != nil {
			return fmt.Errorf("snapshot site artifact: %w", err)
		}
		if err := cleanArtifact(paths.Output); err != nil {
			return err
		}
		return copyTree(snapshot, paths.Output, keep)
	}
	if err := cleanArtifact(paths.Output); err != nil {
		return err
	}
	if info, err := os.Stat(seed); err == nil && info.IsDir() {
		return copyTree(seed, paths.Output, keep)
	}
	return nil
}

func cleanArtifact(root string) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func stageSources(source, output string, files []string) error {
	for _, name := range files {
		if err := copyPath(filepath.Join(source, name), filepath.Join(output, name)); err != nil {
			return fmt.Errorf("stage %s: %w", name, err)
		}
	}
	return nil
}

func copyTree(source, target string, keep func(string) bool) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return os.MkdirAll(target, 0o755)
		}
		name := filepath.ToSlash(relative)
		if !keep(name) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(target, relative), 0o755)
		}
		return copyPath(path, filepath.Join(target, relative))
	})
}

func copyPath(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	_ = os.RemoveAll(target)
	if info.Mode()&os.ModeSymlink != 0 {
		value, readErr := os.Readlink(source)
		if readErr != nil {
			return readErr
		}
		return os.Symlink(value, target)
	}
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(target, content, info.Mode().Perm())
}

func refreshNativeSurfaces(paths Paths) error {
	catalog := filepath.Join(paths.Output, "data", "cli-commands.json")
	primaryCommandPage := filepath.Join(paths.Output, "reference", "cli", "index.html")
	if _, err := os.Stat(primaryCommandPage); os.IsNotExist(err) {
		raw, readErr := os.ReadFile(catalog)
		if readErr == nil {
			if err := docvalid.RenderCommandSurfaces(paths.Output, raw); err != nil {
				return err
			}
		} else if !os.IsNotExist(readErr) {
			return readErr
		}
	} else if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(paths.Output, "assets", "site.css")); os.IsNotExist(err) {
		if err := renderCSS(paths.Source, paths.Output); err != nil {
			return err
		}
	}
	if _, err := os.Stat(filepath.Join(paths.Output, "assets", "site.js")); os.IsNotExist(err) {
		if err := renderJS(paths.Source, paths.Output); err != nil {
			return err
		}
	}
	if _, err := refreshTalks(paths.Repository, filepath.Join(paths.Output, "talks")); err != nil {
		return err
	}
	return nil
}

func removeSourceOnly(root string) error {
	var removals []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || strings.Contains(filepath.ToSlash(path), "/.git/") {
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if isSourceOnly(relative) {
			removals = append(removals, path)
			if entry.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(removals, func(i, j int) bool { return len(removals[i]) > len(removals[j]) })
	for _, path := range removals {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}
