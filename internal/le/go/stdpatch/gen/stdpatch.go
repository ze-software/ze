// Design: docs/contributing/patching-the-standard-library.md -- the generator and the activation
// Related: stamp.go -- the review stamp every patch carries
// Related: tree.go -- the link tree a patched GOROOT is built as
// RFC: rfc/short/rfc5216.md

// Package gen builds the patched GOROOT Ze compiles with, and selects it
// for every Go command le starts.
//
// Ze changes the Go standard library at build time, from a diff applied to the
// toolchain in use: no fork, no copy in the repository. The one patch today is
// patches/crypto-tls-server-session-id.patch. RFC 5216 Section 2.1.1: "If the
// peer's sessionId is null or unrecognized by the server, the server MUST
// choose the sessionId to establish a new session." crypto/tls never chooses
// one on a TLS 1.2 full handshake, so the patch makes it.
//
// The patched source has to exist on disk, because golangci-lint,
// staticcheck, govulncheck and gopls read it from disk. So the package builds
// a GOROOT that is the toolchain's own, linked file by file, with the patched
// files and the files the patch adds real, and every compile site selects it
// with GOROOT. The page above is the contributor's account and the place a Go
// upgrade starts from.
//
// The package imports only the standard library. It MUST stay that way: it
// compiles under the unpatched GOROOT, before any tree exists, when the
// launcher, a QEMU guest, a Dockerfile or CI runs `go run ./internal/le/go/stdpatch/goroot`.
package gen

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// PatchDir is where the standard-library patches live, relative to the
// checkout. The directory is the set: every *.patch file in it is applied.
const PatchDir = "internal/le/go/stdpatch/patches"

// layoutVersion keys every tree with the shape of the tree. It MUST be bumped
// whenever the shape changes (what is linked, what is copied, what is added),
// so a tree built under an older rule is never answered again.
const layoutVersion = "1"

// pageSection is where every refusal sends the contributor.
const pageSection = `docs/contributing/patching-the-standard-library.md, section "When the Go version changes"`

// repairCommand is the command line every refusal ends with.
const repairCommand = "./le go stdpatch goroot"

// Toolchain is the real Go toolchain a tree is built from.
type Toolchain struct {
	// GOROOT is the real root `go env GOROOT` answers, never a tree.
	GOROOT string
	// Version is `go env GOVERSION`, for example go1.27.2.
	Version string
}

// Tree answers the patched GOROOT for the toolchain this checkout pins,
// building it on first use. Every tree lives under CacheDir(root).
//
// RFC 5216 Section 2.1.1: the tree is what carries the session_id patch.
func Tree(root string) (string, error) {
	cache, err := CacheDir(root)
	if err != nil {
		return "", err
	}
	patches, err := Patches(root)
	if err != nil {
		return "", err
	}
	toolchain, err := Resolve(root, cache, patches)
	if err != nil {
		return "", err
	}
	return builder{cache: cache, toolchain: toolchain, patches: patches, layout: layoutVersion}.build()
}

// CacheDirs answers every directory a tree can live in, in the order CacheDir
// tries them, without creating either. `le scratch cache-clean` reads it to
// report the trees it keeps.
func CacheDirs(root string) []string {
	return []string{filepath.Join(root, "cache", "stdpatch"), filepath.Join(root, "tmp", "stdpatch")}
}

// CacheDir answers the directory the trees live in: cache/stdpatch under the
// checkout, or tmp/stdpatch when cache/ is not usable here. cache/ is a link
// to ~/.cache/ze on the development hosts and dangles anywhere the checkout is
// shared without it, the same fallback the launcher's build_le takes for
// GOCACHE. A QEMU guest mounts a tmpfs of its own over cache/stdpatch before
// it calls here (guestPatchedTree in internal/le/test/qemu/run.go), because a
// tree written through the shared checkout would link the host to the guest's
// GOROOT.
//
// The answer is spelled where a link carries it (Landing), so a verify
// worktree, whose cache/ links to ~/.cache/ze, names the same GOROOT, and
// puts the same bin first on PATH, in every run.
func CacheDir(root string) (string, error) {
	dirs := CacheDirs(root)
	cache, fallback := dirs[0], dirs[1]
	if err := os.MkdirAll(cache, 0o750); err == nil {
		return Landing(root, cache), nil
	}
	if err := os.MkdirAll(fallback, 0o750); err != nil {
		return "", fmt.Errorf("stdpatch: neither %s nor %s can hold the patched GOROOT: %w", cache, fallback, err)
	}
	return Landing(root, fallback), nil
}

// Resolve answers the real GOROOT and version of the toolchain the checkout
// pins: `go env GOROOT GOVERSION` run in root, with GOTOOLCHAIN at go.mod's
// pin. An inherited GOROOT that is a tree is dropped: one under cache, or one
// that already holds a file the patches add (a tree of another cache, such as
// a test's, or a developer's export). So the generator never depends on its
// own output and a le started by le resolves the same toolchain. Go's own
// `go test` puts $GOROOT/bin first on PATH, so an inherited PATH may start
// with a tree's bin, a link to the real toolchain's bin. A go run through that
// link finds its root from its own path, and on darwin os.Executable answers
// the unresolved path, so it would answer the tree. Resolve therefore runs the
// go on PATH by its resolved path (realGo), which lies in the real toolchain on
// every platform, and keeps the PATH name as argv[0]. When the pin names a version other than the go on PATH, the
// go command runs the pinned toolchain, and the real GOROOT is then that
// toolchain's directory under GOMODCACHE.
func Resolve(root, cache string, patches []Patch) (Toolchain, error) {
	pin, err := Pin(root)
	if err != nil {
		return Toolchain{}, err
	}
	onPath, goBinary, err := realGo()
	if err != nil {
		return Toolchain{}, err
	}
	environment := os.Environ()
	if inherited := os.Getenv("GOROOT"); inherited != "" && isTree(inherited, cache, patches) {
		environment = without(environment, "GOROOT")
	}
	if pin != "" {
		environment = append(without(environment, "GOTOOLCHAIN"), "GOTOOLCHAIN="+pin)
	}
	cmd := exec.CommandContext(context.Background(), goBinary, "env", "GOROOT", "GOVERSION") //nolint:gosec // goBinary is the go on PATH, resolved through its links
	// argv[0] stays the PATH name: a version manager's shim (mise) links to one
	// dispatcher that picks the tool from argv[0], while go finds its root from the
	// executed path (darwin) or /proc/self/exe (Linux), never from argv[0].
	cmd.Args[0] = onPath
	cmd.Dir = root
	cmd.Env = environment
	out, err := cmd.Output()
	if err != nil {
		return Toolchain{}, fmt.Errorf("stdpatch: `go env GOROOT GOVERSION` in %s: %w%s", root, err, stderrOf(err))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		return Toolchain{}, fmt.Errorf("stdpatch: `go env GOROOT GOVERSION` answered %d lines, want 2: %q", len(lines), out)
	}
	toolchain := Toolchain{GOROOT: strings.TrimSpace(lines[0]), Version: strings.TrimSpace(lines[1])}
	if toolchain.GOROOT == "" {
		return Toolchain{}, errors.New("stdpatch: `go env GOROOT` answered nothing")
	}
	if toolchain.Version == "" {
		return Toolchain{}, errors.New("stdpatch: `go env GOVERSION` answered nothing")
	}
	return toolchain, nil
}

// realGo answers the go command on PATH twice: as PATH names it, and with every
// symbolic link resolved. The resolved path finds the real toolchain as its
// root whichever link PATH reached it through: a tree's bin, or a package
// manager's bin such as Homebrew's /opt/homebrew/bin/go. The PATH name is the
// argv[0] a dispatcher behind that link reads to choose the tool.
func realGo() (onPath, resolved string, err error) {
	onPath, err = exec.LookPath("go")
	if err != nil {
		return "", "", fmt.Errorf("stdpatch: no go command on PATH, so the toolchain to patch cannot be found: %w\n"+
			"install Go, or put its bin directory first on PATH, then run the command again", err)
	}
	resolved, err = filepath.EvalSymlinks(onPath)
	if err != nil {
		return "", "", fmt.Errorf("stdpatch: the go command on PATH, %s, could not be resolved to the file it links to, so the toolchain to patch cannot be found: %w\n"+
			"check that every directory on the way to it is readable and that it was not replaced while le ran, then run the command again", onPath, err)
	}
	return onPath, resolved, nil
}

// Pin answers the toolchain go.mod pins: the toolchain line when there is
// one, else the go directive when it names a patch release (`go 1.27.2` pins
// go1.27.2). It answers "" when go.mod pins nothing, because Go refuses a
// language version such as go1.27 as a GOTOOLCHAIN value. An unreadable
// go.mod is an error, never "", because "" would silently run the ambient
// toolchain.
func Pin(root string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(root, "go.mod")) //nolint:gosec // a build tool reads the checkout it was pointed at
	if err != nil {
		return "", err
	}
	version := ""
	for line := range strings.SplitSeq(string(raw), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		if parts[0] == "toolchain" {
			return parts[1], nil
		}
		if parts[0] == "go" && version == "" {
			version = parts[1]
		}
	}
	if strings.Count(version, ".") != 2 {
		return "", nil
	}
	return "go" + version, nil
}

// activation is what the first Activate call in this process answered.
// Safe for concurrent use: the mutex guards every field.
var activation struct {
	sync.Mutex
	done bool
	tree string
	err  error
	// caller is the environment Activate found before it changed anything
	// (CallerEnvironment).
	caller callerValues
}

// Activate answers the patched GOROOT for root's pinned toolchain and selects
// it for this process and every child it starts: it sets GOROOT to the tree,
// GOTOOLCHAIN to local, and puts the tree's bin first on PATH. A bare `go`
// then resolves to the tree's go through exec.LookPath, and every child built
// from os.Environ() inherits the three values.
//
// The first call does the work and every later call answers the same tree or
// the same error, so le's root handler and gotoolchain.New share one answer.
// A refusal sets none of the three values. Activate MUST be called while the
// process runs one goroutine or before any goroutine reads the environment:
// le's root handler calls it before it dispatches.
//
// RFC 5216 Section 2.1.1: the tree carries the patch that makes crypto/tls
// choose the session_id the requirement asks for.
func Activate(root string) (string, error) {
	activation.Lock()
	defer activation.Unlock()

	if !activation.done {
		activation.caller = captureCaller()
		activation.tree, activation.err = activate(root)
		activation.done = true
	}
	return activation.tree, activation.err
}

// activate is one activation, with no memory of an earlier one: the nested
// le case, where the environment already carries a tree.
func activate(root string) (string, error) {
	tree, err := Tree(root)
	if err != nil {
		return "", err
	}
	if err := os.Setenv("GOROOT", tree); err != nil {
		return "", err
	}
	if err := os.Setenv("GOTOOLCHAIN", "local"); err != nil {
		return "", err
	}
	bin := filepath.Join(tree, "bin")
	path := os.Getenv("PATH")
	entries := filepath.SplitList(path)
	if len(entries) > 0 && entries[0] == bin {
		return tree, nil
	}
	if path == "" {
		return tree, os.Setenv("PATH", bin)
	}
	return tree, os.Setenv("PATH", bin+string(filepath.ListSeparator)+path)
}

// isTree answers whether goroot is a patched tree: inside cache, or holding
// a file one of the patches adds, which no real GOROOT holds.
func isTree(goroot, cache string, patches []Patch) bool {
	if isUnder(goroot, cache) {
		return true
	}
	for _, patch := range patches {
		for _, added := range AddedFiles(patch) {
			if _, err := os.Lstat(filepath.Join(goroot, filepath.FromSlash(added))); err == nil {
				return true
			}
		}
	}
	return false
}

// isUnder answers whether path lies inside dir, both resolved through their
// symbolic links: cache/ is a link to ~/.cache/ze, so a tree reached through
// it and one reached through its target are the same tree.
func isUnder(path, dir string) bool {
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolvedPath = filepath.Clean(path)
	}
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		resolvedDir = filepath.Clean(dir)
	}
	rel, err := filepath.Rel(resolvedDir, resolvedPath)
	if err != nil {
		return false
	}
	if rel == "." {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Landing answers where path lands when a link inside root carries it out of
// root, and path itself when it stays inside root or root cannot be resolved.
//
// A checkout and every verify worktree reach the per-user caches through one
// link: cache/ is a link to ~/.cache/ze, and the Go, golangci-lint and
// Staticcheck caches all sit under it. Spelled through the worktree, every
// cache path differs between two runs of one commit, and a Go test that reads
// such a variable through os.Environ (internal/core/env does) is blind to the
// change, because cmd/go keys its test cache only on what os.Getenv logged
// (computeTestInputsID, cmd/go/internal/test/test.go). Naming where the link
// lands gives two runs one spelling. A path that stays inside root keeps its spelling, so a
// checkout with no links and a test fixture under a linked temp directory
// (/var on macOS) answer what they did.
//
// path need not exist: the longest prefix that does is resolved and the rest
// joined back, the way a go command later creates its GOCACHE. The walk is
// bounded by the number of elements in path.
func Landing(root, path string) string {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		// No root on disk holds a link, so nothing carries path out of it.
		return path
	}
	resolved := resolveExisting(filepath.Clean(path))
	if resolved == resolvedRoot || isUnder(resolved, resolvedRoot) {
		return path
	}
	return resolved
}

// resolveExisting resolves the longest existing prefix of path through its
// links and joins the rest back. It answers path when not even its first
// element resolves.
func resolveExisting(path string) string {
	rest := ""
	for prefix := path; ; prefix = filepath.Dir(prefix) {
		if resolved, err := filepath.EvalSymlinks(prefix); err == nil {
			return filepath.Join(resolved, rest)
		}
		if filepath.Dir(prefix) == prefix {
			return path
		}
		rest = filepath.Join(filepath.Base(prefix), rest)
	}
}

// without answers environment with every entry for each name removed.
func without(environment []string, names ...string) []string {
	kept := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if !slices.Contains(names, name) {
			kept = append(kept, entry)
		}
	}
	return kept
}

// stderrOf answers the standard error an exec failure carries, prefixed for
// an error message, or "" when it carries none.
func stderrOf(err error) string {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return ""
	}
	text := strings.TrimSpace(string(exit.Stderr))
	if text == "" {
		return ""
	}
	return ": " + text
}
