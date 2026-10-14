// Design: docs/contributing/patching-the-standard-library.md -- the layout of a patched GOROOT
// Overview: stdpatch.go -- the generator and the activation
// RFC: rfc/short/rfc5216.md

package stdpatch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// embedTargets is every GOOS/GOARCH one tree serves. EmbedFiles follows build
// constraints, and one tree serves every target and both cgo settings (a
// guest route cross-compiles under the host tree), so the copied embed set is
// the union over this list and CGO_ENABLED 0 and 1.
var embedTargets = [...]string{"linux/amd64", "linux/arm64", "darwin/arm64", "freebsd/amd64", "windows/amd64"}

// builder builds one tree. Every field is data a test can set.
type builder struct {
	// cache holds one key directory per tree, and the temporary directories
	// a build writes before its rename.
	cache string
	// toolchain is the real toolchain the tree mirrors.
	toolchain Toolchain
	// patches is the set applied, in order.
	patches []Patch
	// layout is layoutVersion outside tests.
	layout string
}

// build answers the tree, building it when its key directory is absent.
//
// RFC 5216 Section 2.1.1: "If the peer's sessionId is null or unrecognized by
// the server, the server MUST choose the sessionId to establish a new
// session." This is the function that applies the patch making crypto/tls
// choose it (docs/contributing/patching-the-standard-library.md).
func (b builder) build() (string, error) {
	// Step 1a: a toolchain nobody reviewed the patch against refuses first.
	if err := checkStamps(b.patches, b.toolchain.Version); err != nil {
		return "", err
	}
	targets, err := b.targetFiles()
	if err != nil {
		return "", err
	}
	final := filepath.Join(b.cache, b.key(targets))
	tree := filepath.Join(final, "goroot")
	if _, err := os.Lstat(tree); err == nil {
		return tree, nil
	}

	staging, err := os.MkdirTemp(b.cache, "tmp-")
	if err != nil {
		return "", fmt.Errorf("stdpatch: %w", err)
	}
	renamed := false
	defer func() {
		if !renamed {
			_ = os.RemoveAll(staging) //nolint:errcheck // the build already failed; its error is the one returned
		}
	}()

	// The patch applies first, over copies of its targets, so a refusal costs
	// no `go list`; the embedded files are copied in after it.
	copies := make(map[string]bool, len(targets))
	for path := range targets {
		copies[path] = true
	}
	staged := filepath.Join(staging, "goroot")
	if err := populate(staged, b.toolchain.GOROOT, copies); err != nil {
		return "", err
	}
	for _, patch := range b.patches {
		if err := apply(staging, staged, patch, b.toolchain); err != nil {
			return "", err
		}
	}
	embedded, err := embedSet(b.toolchain)
	if err != nil {
		return "", err
	}
	if err := copyEmbedded(staged, b.toolchain.GOROOT, embedded); err != nil {
		return "", err
	}
	if err := sameStdlib(b.toolchain, staged, staging); err != nil {
		return "", err
	}

	if err := os.Rename(staging, final); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("stdpatch: %w", err)
		}
		// Another build won the race for this key: answer its tree.
		if _, statErr := os.Stat(filepath.Join(tree, "VERSION")); statErr != nil {
			return "", fmt.Errorf("stdpatch: a concurrent build left %s without a usable VERSION: %w", tree, statErr)
		}
		return tree, nil
	}
	renamed = true
	return tree, nil
}

// targetFiles answers each GOROOT file a patch changes, with its bytes.
func (b builder) targetFiles() (map[string][]byte, error) {
	targets := make(map[string][]byte)
	for _, patch := range b.patches {
		for _, path := range patchTargets(patch.Body) {
			body, err := os.ReadFile(filepath.Join(b.toolchain.GOROOT, filepath.FromSlash(path)))
			if err != nil {
				return nil, fmt.Errorf("stdpatch: %s changes %s, which the %s GOROOT %s does not hold: %w\n%s",
					patch.Path, path, b.toolchain.Version, b.toolchain.GOROOT, err, reviewStep(patch, b.toolchain.Version))
			}
			targets[path] = body
		}
	}
	return targets, nil
}

// key answers the tree's directory name: a SHA-256 over the layout, the
// version, the real GOROOT path, every patch and every target file. The path
// is in the key because every link in the tree names an absolute path into
// that GOROOT, and cache/ is shared by every checkout.
func (b builder) key(targets map[string][]byte) string {
	sum := sha256.New()
	field := func(value []byte) {
		var length [8]byte
		size := uint64(len(value))
		for i := range length {
			length[i] = byte(size >> (8 * i))
		}
		sum.Write(length[:])
		sum.Write(value)
	}
	field([]byte(b.layout))
	field([]byte(b.toolchain.Version))
	field([]byte(b.toolchain.GOROOT))
	for _, patch := range b.patches {
		field([]byte(patch.Path))
		field(patch.Body)
	}
	paths := make([]string, 0, len(targets))
	for path := range targets {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		field([]byte(path))
		field(targets[path])
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// populate writes the link tree at target for the real GOROOT source: every
// top-level entry but src is a link, every directory under src is a REAL
// directory, and every file a link unless copies names it. Directories are
// never linked, because the go command's `...` walk ignores a linked
// directory; an embedded file is a copy, because the go command refuses to
// embed a link ("cannot embed irregular file").
func populate(target, source string, copies map[string]bool) error {
	if err := os.Mkdir(target, 0o750); err != nil {
		return fmt.Errorf("stdpatch: %w", err)
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return fmt.Errorf("stdpatch: %w", err)
	}
	for _, entry := range entries {
		if entry.Name() == "src" {
			continue
		}
		if err := os.Symlink(filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name())); err != nil {
			return fmt.Errorf("stdpatch: %w", err)
		}
	}
	// The walk is bounded by the toolchain's own source tree.
	return filepath.WalkDir(filepath.Join(source, "src"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		link := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.Mkdir(link, 0o750)
		}
		if entry.Type().IsRegular() && copies[filepath.ToSlash(rel)] {
			return copyFile(link, path)
		}
		return os.Symlink(path, link) //nolint:gosec // the walk reads the toolchain's own GOROOT and writes only the staging tree it created
	})
}

// copyEmbedded replaces the link of every embedded file with a copy. A file
// the patch already made real stays as the patch left it.
func copyEmbedded(staged, source string, embedded map[string]bool) error {
	paths := make([]string, 0, len(embedded))
	for path := range embedded {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		link := filepath.Join(staged, filepath.FromSlash(path))
		info, err := os.Lstat(link)
		if err != nil {
			return fmt.Errorf("stdpatch: an embedded file is missing from the tree: %w", err)
		}
		if info.Mode()&fs.ModeSymlink == 0 {
			continue
		}
		if err := os.Remove(link); err != nil {
			return fmt.Errorf("stdpatch: %w", err)
		}
		if err := copyFile(link, filepath.Join(source, filepath.FromSlash(path))); err != nil {
			return fmt.Errorf("stdpatch: %w", err)
		}
	}
	return nil
}

// copyFile writes a regular copy of source at target.
func copyFile(target, source string) error {
	in, err := os.Open(source) //nolint:gosec // the source is a file of the toolchain's own GOROOT
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // read-only; a close error loses nothing

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644) //nolint:gosec // a GOROOT file is world-readable
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close() //nolint:errcheck // the copy error is the one returned
		return err
	}
	return out.Close()
}

// apply runs `git apply` for one patch inside the staged tree. The ceiling
// stops git at the staging directory, so it never finds the ze repository
// and applies the paths relative to the tree. git has no fuzz: a changed
// context line refuses.
func apply(staging, staged string, patch Patch, toolchain Toolchain) error {
	cmd := exec.CommandContext(context.Background(), "git", "apply", "-")
	cmd.Dir = staged
	cmd.Stdin = bytes.NewReader(patch.Body)
	cmd.Env = append(without(os.Environ(), "GIT_CEILING_DIRECTORIES", "GIT_DIR", "GIT_WORK_TREE"), "GIT_CEILING_DIRECTORIES="+staging)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("stdpatch: %s does not apply to the %s standard library at %s.\ngit apply said: %s\n%s",
			patch.Path, toolchain.Version, toolchain.GOROOT, strings.TrimSpace(string(out)), reviewStep(patch, toolchain.Version))
	}
	return nil
}

// listedPackage is the part of `go list -json` the generator reads.
type listedPackage struct {
	ImportPath string
	Dir        string
	EmbedFiles []string
	Error      *struct{ Err string }
}

// listStdlib answers `go list -e std cmd` under goroot, keyed by import path.
func listStdlib(goroot, dir string, extra ...string) (map[string]listedPackage, error) {
	cmd := exec.CommandContext(context.Background(), filepath.Join(goroot, "bin", "go"), "list", "-e", "-json=ImportPath,Dir,EmbedFiles,Error", "std", "cmd") //nolint:gosec // goroot is the real toolchain or the tree built from it
	cmd.Dir = dir
	environment := without(os.Environ(), "GOROOT", "GOTOOLCHAIN", "GOFLAGS", "GOWORK", "GOOS", "GOARCH", "CGO_ENABLED")
	environment = append(environment, "GOROOT="+goroot, "GOTOOLCHAIN=local", "GOFLAGS=", "GOWORK=off")
	environment = append(environment, extra...)
	cmd.Env = environment
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("stdpatch: `go list std cmd` under %s %v: %w%s", goroot, extra, err, stderrOf(err))
	}
	listed := make(map[string]listedPackage)
	decoder := json.NewDecoder(bytes.NewReader(out))
	for decoder.More() {
		var pkg listedPackage
		if err := decoder.Decode(&pkg); err != nil {
			return nil, fmt.Errorf("stdpatch: reading `go list` under %s: %w", goroot, err)
		}
		listed[pkg.ImportPath] = pkg
	}
	return listed, nil
}

// embedSet answers every GOROOT-relative file a package of std or cmd embeds,
// over every target of embedTargets and both cgo settings.
func embedSet(toolchain Toolchain) (map[string]bool, error) {
	copies := make(map[string]bool)
	for _, target := range embedTargets {
		goos, goarch, _ := strings.Cut(target, "/")
		for _, cgo := range [...]string{"0", "1"} {
			listed, err := listStdlib(toolchain.GOROOT, toolchain.GOROOT, "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED="+cgo)
			if err != nil {
				return nil, err
			}
			for _, pkg := range listed {
				for _, file := range pkg.EmbedFiles {
					rel, err := filepath.Rel(toolchain.GOROOT, filepath.Join(pkg.Dir, file))
					if err != nil {
						return nil, fmt.Errorf("stdpatch: %w", err)
					}
					copies[filepath.ToSlash(rel)] = true
				}
			}
		}
	}
	return copies, nil
}

// sameStdlib refuses a staged tree whose `go list -e std cmd` differs from the
// real GOROOT's, or names any package error, naming the first package that
// differs.
func sameStdlib(toolchain Toolchain, staged, dir string) error {
	real, err := listStdlib(toolchain.GOROOT, dir)
	if err != nil {
		return err
	}
	tree, err := listStdlib(staged, dir)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(real)+len(tree))
	for path := range real {
		paths = append(paths, path)
	}
	for path := range tree {
		if _, found := real[path]; !found {
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)
	for _, path := range paths {
		pkg, inTree := tree[path]
		if _, inReal := real[path]; !inReal {
			return fmt.Errorf("stdpatch: the patched GOROOT lists %s, which the real %s does not", path, toolchain.GOROOT)
		}
		if !inTree {
			return fmt.Errorf("stdpatch: the patched GOROOT does not list %s, which the real %s does", path, toolchain.GOROOT)
		}
		if pkg.Error != nil {
			return fmt.Errorf("stdpatch: the patched GOROOT fails to load %s: %s", path, pkg.Error.Err)
		}
	}
	return nil
}
