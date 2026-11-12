// Design: docs/contributing/preview-artifacts.md -- exact source identity.
package buildpreview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	buildcompile "github.com/ze-software/ze/internal/le/build/compile"
	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
)

// Input records content, never arbitrary local file bodies.
type Input struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Source identifies the current selected build inputs, including new files.
type Source struct {
	Commit    string   `json:"commit"`
	Dirty     bool     `json:"dirty"`
	Digest    string   `json:"digest"`
	BuildDate string   `json:"build-date"`
	Toolchain string   `json:"toolchain"`
	Tags      []string `json:"tags"`
	Inputs    []Input  `json:"inputs"`
}

func captureSource(ctx context.Context, root string) (Source, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	paths, tags, pin, err := buildcompile.DaemonInputs(ctx, root)
	if err != nil {
		return Source{}, err
	}
	paths = append(paths, "go.mod", "go.sum", "vendor/modules.txt", "feature-gates.txt", "feature-tags", "LICENSE", "docker/Dockerfile", "docker/Dockerfile.lab", "docker/Dockerfile.runtime", ".dockerignore")
	patches, err := gen.Patches(root)
	if err != nil {
		return Source{}, err
	}
	for _, patch := range patches {
		paths = append(paths, patch.Path)
	}
	// These producers build the patched compiler before daemon compilation.
	for _, dir := range []string{"internal/le/go/stdpatch/gen", "internal/le/go/stdpatch/goroot"} {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			return Source{}, err
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
				paths = append(paths, dir+"/"+entry.Name())
			}
		}
	}
	inputs, digest, err := fingerprintInputs(root, paths)
	if err != nil {
		return Source{}, err
	}
	git := func(args ...string) (string, error) {
		command := exec.CommandContext(ctx, "git", args...) //nolint:gosec // G204: every call passes a fixed git query from this function, run in the checkout the action located
		command.Dir = root
		body, err := command.Output()
		if err != nil {
			return "", fmt.Errorf("preview git %s: %w", args[0], err)
		}
		return strings.TrimSpace(string(body)), nil
	}
	commit, err := git("rev-parse", "HEAD")
	if err != nil {
		return Source{}, err
	}
	date, err := git("show", "-s", "--format=%cI", "HEAD")
	if err != nil {
		return Source{}, err
	}
	stamp, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return Source{}, err
	}
	dirty, err := git("status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return Source{}, err
	}
	return Source{Commit: commit, Dirty: dirty != "", Digest: sourceDigest(digest, pin, tags), BuildDate: stamp.UTC().Format(time.RFC3339), Toolchain: pin, Tags: tags, Inputs: inputs}, nil
}

// Pin and tags influence selection and belong to the same source identity.
func sourceDigest(files, pin string, tags []string) string {
	identity, _ := json.Marshal(struct{ Files, Pin, Tags string }{files, pin, strings.Join(tags, " ")})
	sum := sha256.Sum256(identity)
	return hex.EncodeToString(sum[:])
}

func inputDigest(inputs []Input) string {
	body, _ := json.Marshal(inputs)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func fingerprintInputs(root string, paths []string) ([]Input, string, error) {
	paths = slices.Clone(paths)
	slices.Sort(paths)
	paths = slices.Compact(paths)
	inputs := make([]Input, 0, len(paths))
	for _, path := range paths {
		if filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path || path == ".." || strings.HasPrefix(path, "../") {
			return nil, "", fmt.Errorf("unsafe preview source path %q", path)
		}
		full := filepath.Join(root, filepath.FromSlash(path))
		canonical, err := filepath.EvalSymlinks(full)
		if err != nil {
			return nil, "", err
		}
		canonicalRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			return nil, "", err
		}
		if canonical != filepath.Join(canonicalRoot, filepath.FromSlash(path)) {
			return nil, "", fmt.Errorf("preview input %s follows a symbolic link", path)
		}
		info, err := os.Lstat(full)
		if err != nil {
			return nil, "", err
		}
		if !info.Mode().IsRegular() {
			return nil, "", fmt.Errorf("preview input %s is not regular", path)
		}
		sum, err := fileSum(full)
		if err != nil {
			return nil, "", err
		}
		inputs = append(inputs, Input{Path: path, SHA256: sum})
	}
	return inputs, inputDigest(inputs), nil
}
