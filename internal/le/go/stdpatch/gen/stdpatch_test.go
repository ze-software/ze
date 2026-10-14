// VALIDATES: AC-20, AC-21, AC-25, AC-26: the generator's refusals, its key,
// its tree shape and the process activation.
// PREVENTS: a tree that silently misses packages, a patch applied over code
// nobody reviewed, and an activation that leaves part of the environment set.

package gen

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// repoRoot answers the checkout this test runs in: the nearest ancestor of
// the package directory holding go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test directory")
		}
		dir = parent
	}
}

// realToolchain resolves the toolchain this checkout pins, as the generator
// does, with cache as the directory whose trees are ignored.
func realToolchain(t *testing.T, cache string) Toolchain {
	t.Helper()
	toolchain, err := Resolve(repoRoot(t), cache, shippedPatches(t))
	if err != nil {
		t.Fatal(err)
	}
	return toolchain
}

// shippedPatches answers the patch set of the checkout.
func shippedPatches(t *testing.T) []Patch {
	t.Helper()
	patches, err := Patches(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	return patches
}

// fixtureGOROOT writes a GOROOT holding only VERSION and the files the
// shipped patches change, copied from the real toolchain, and answers it.
func fixtureGOROOT(t *testing.T, real Toolchain, patches []Patch) string {
	t.Helper()
	goroot := filepath.Join(t.TempDir(), "go")
	for _, patch := range patches {
		for _, path := range patchTargets(patch.Body) {
			body, err := os.ReadFile(filepath.Join(real.GOROOT, filepath.FromSlash(path)))
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(goroot, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, body, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(goroot, "VERSION"), []byte(real.Version+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return goroot
}

// pathWithoutTrees answers path with every patched GOROOT's bin removed.
func pathWithoutTrees(path string) string {
	kept := make([]string, 0, 8)
	for _, entry := range filepath.SplitList(path) {
		if strings.Contains(entry, string(filepath.Separator)+"stdpatch"+string(filepath.Separator)) {
			continue
		}
		kept = append(kept, entry)
	}
	return strings.Join(kept, string(filepath.ListSeparator))
}

// cacheEntries answers the names in a cache directory, so a test can say
// that a refused build left nothing behind.
func cacheEntries(t *testing.T, cache string) []string {
	t.Helper()
	entries, err := os.ReadDir(cache)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// TestStdpatchRefusesAStaleReviewStamp proves step 1a (AC-26): a patch whose
// stamp names another Go release refuses before any tree or temporary
// directory exists, even though its diff applies to the fixture GOROOT (its
// handshake_server.go is the real one); a patch with no stamp refuses the
// same way; a stamp equal to the version passes the check.
func TestStdpatchRefusesAStaleReviewStamp(t *testing.T) {
	patches := shippedPatches(t)
	real := realToolchain(t, t.TempDir())

	t.Run("stale", func(t *testing.T) {
		cache := t.TempDir()
		stale := Toolchain{GOROOT: fixtureGOROOT(t, real, patches), Version: "go1.27.3"}
		_, err := builder{cache: cache, toolchain: stale, patches: patches, layout: layoutVersion}.build()
		if err == nil {
			t.Fatal("a go1.27.3 toolchain built a tree from a patch stamped for another release")
		}
		stamp, stampErr := Stamp(patches[0])
		if stampErr != nil {
			t.Fatal(stampErr)
		}
		for _, want := range []string{stamp, "go1.27.3", patches[0].Path, "When the Go version changes", "src/crypto/tls/handshake_server.go"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal does not name %q:\n%v", want, err)
			}
		}
		if !strings.HasSuffix(err.Error(), "Then run: "+repairCommand) {
			t.Errorf("refusal does not end with the command line to run after the edit:\n%v", err)
		}
		if names := cacheEntries(t, cache); len(names) != 0 {
			t.Errorf("a refused build left %v in the cache", names)
		}
	})

	t.Run("missing", func(t *testing.T) {
		cache := t.TempDir()
		body := strings.Replace(string(patches[0].Body), stampKey, "Reviewed:", 1)
		unstamped := []Patch{{Path: patches[0].Path, Body: []byte(body)}}
		_, err := builder{cache: cache, toolchain: real, patches: unstamped, layout: layoutVersion}.build()
		if !errors.Is(err, ErrNoStamp) {
			t.Fatalf("an unstamped patch answered %v, want ErrNoStamp", err)
		}
		if !strings.HasSuffix(err.Error(), "Then run: "+repairCommand) {
			t.Errorf("refusal does not end with the command line:\n%v", err)
		}
		if names := cacheEntries(t, cache); len(names) != 0 {
			t.Errorf("a refused build left %v in the cache", names)
		}
	})

	t.Run("equal", func(t *testing.T) {
		if err := CheckStamps(patches, real.Version); err != nil {
			t.Fatalf("the shipped patch refuses the pinned toolchain %s: %v", real.Version, err)
		}
	})
}

// TestStdpatchRefusesAPatchThatDoesNotApply proves step 5 (AC-21): over a
// fixture GOROOT whose handshake_server.go differs in a context line, the
// build refuses naming the version, the patch, git's output and the page
// section, writes no tree and leaves no temporary directory.
func TestStdpatchRefusesAPatchThatDoesNotApply(t *testing.T) {
	patches := shippedPatches(t)
	real := realToolchain(t, t.TempDir())
	goroot := fixtureGOROOT(t, real, patches)
	target := filepath.Join(goroot, "src", "crypto", "tls", "handshake_server.go")
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	// The patch's leading context line, which appears once in the file.
	context := "\ths.hello.ticketSupported = hs.clientHello.ticketSupported && !c.config.SessionTicketsDisabled\n"
	if !strings.Contains(string(body), context) {
		t.Fatalf("the context line %q is not in %s", context, target)
	}
	if strings.Count(string(body), context) != 1 {
		t.Fatalf("the context line %q is not unique in %s", context, target)
	}
	changed := strings.Replace(string(body), context, "\ths.hello.ticketSupported = hs.clientHello.ticketSupported // moved upstream\n", 1)
	if err := os.WriteFile(target, []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}

	cache := t.TempDir()
	_, err = builder{cache: cache, toolchain: Toolchain{GOROOT: goroot, Version: real.Version}, patches: patches, layout: layoutVersion}.build()
	if err == nil {
		t.Fatal("a patch whose context line moved built a tree")
	}
	for _, want := range []string{real.Version, patches[0].Path, "git apply said:", "patch failed", "When the Go version changes"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q:\n%v", want, err)
		}
	}
	if names := cacheEntries(t, cache); len(names) != 0 {
		t.Errorf("a refused build left %v in the cache", names)
	}
}

// TestStdpatchTreeIsContentAddressed proves the key (AC-21): the same inputs
// answer the same directory; a changed GOROOT file, GOROOT path or layout
// version each answer a new one.
func TestStdpatchTreeIsContentAddressed(t *testing.T) {
	patches := shippedPatches(t)
	base := builder{toolchain: Toolchain{GOROOT: "/usr/local/go", Version: "go1.27.2"}, patches: patches, layout: layoutVersion}
	targets := map[string][]byte{"src/crypto/tls/handshake_server.go": []byte("package tls\n")}
	key := base.key(targets)
	if again := base.key(map[string][]byte{"src/crypto/tls/handshake_server.go": []byte("package tls\n")}); again != key {
		t.Fatalf("the same inputs answered %s and %s", key, again)
	}

	changedFile := base.key(map[string][]byte{"src/crypto/tls/handshake_server.go": []byte("package tls // changed\n")})
	movedRoot := base
	movedRoot.toolchain.GOROOT = "/home/user/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.2.linux-amd64"
	newLayout := base
	newLayout.layout = layoutVersion + "-next"
	for name, other := range map[string]string{
		"a changed GOROOT file": changedFile,
		"a changed GOROOT path": movedRoot.key(targets),
		"a changed layout":      newLayout.key(targets),
	} {
		if other == key {
			t.Errorf("%s answered the same key %s", name, key)
		}
	}
}

// TestStdpatchConcurrentBuildsAnswerOneTree proves step 4.6 (AC-21): two
// builds racing for one key over the real toolchain answer the same tree,
// and the loser's temporary directory is gone.
func TestStdpatchConcurrentBuildsAnswerOneTree(t *testing.T) {
	cache := t.TempDir()
	build := builder{cache: cache, toolchain: realToolchain(t, cache), patches: shippedPatches(t), layout: layoutVersion}

	var wg sync.WaitGroup
	trees := make([]string, 2)
	errs := make([]error, 2)
	for i := range trees {
		wg.Go(func() { trees[i], errs[i] = build.build() })
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("build %d: %v", i, err)
		}
	}
	if trees[0] != trees[1] {
		t.Fatalf("two builds of one key answered %s and %s", trees[0], trees[1])
	}
	for _, name := range cacheEntries(t, cache) {
		if strings.HasPrefix(name, "tmp-") {
			t.Errorf("a temporary directory %s is left in the cache", name)
		}
	}
}

// TestStdpatchTreeServesTheWholeStdlib proves the tree's shape (AC-25) over
// the real toolchain: `go list -e std cmd` under the tree answers the real
// set with no package error for each target and cgo setting; every src
// directory is real; the embedded, patched and added files are regular and
// every other src file a link into the real GOROOT; `go env` answers the
// tree and the real version; an inherited tree GOROOT does not change what
// Resolve answers.
func TestStdpatchTreeServesTheWholeStdlib(t *testing.T) {
	cache := t.TempDir()
	real := realToolchain(t, cache)
	patches := shippedPatches(t)
	tree, err := builder{cache: cache, toolchain: real, patches: patches, layout: layoutVersion}.build()
	if err != nil {
		t.Fatal(err)
	}

	embedded, err := embedSet(real)
	if err != nil {
		t.Fatal(err)
	}
	regular := make(map[string]bool, len(embedded)+2)
	for path := range embedded {
		regular[path] = true
	}
	for _, patch := range patches {
		for _, path := range patchTargets(patch.Body) {
			regular[path] = true
		}
		for _, path := range AddedFiles(patch) {
			regular[path] = true
		}
	}
	if len(embedded) == 0 {
		t.Fatal("the real GOROOT embeds no file; the embed listing is broken")
	}

	err = filepath.WalkDir(filepath.Join(tree, "src"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(tree, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		switch {
		case entry.IsDir():
			return nil
		case regular[rel]:
			if !entry.Type().IsRegular() {
				t.Errorf("%s is not a regular file", rel)
			}
		case entry.Type()&fs.ModeSymlink == 0:
			t.Errorf("%s is neither a link nor a file the tree must copy", rel)
		default:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if target != filepath.Join(real.GOROOT, filepath.FromSlash(rel)) {
				t.Errorf("%s links to %s, not into the real GOROOT", rel, target)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for path := range regular {
		if _, err := os.Lstat(filepath.Join(tree, filepath.FromSlash(path))); err != nil {
			t.Errorf("%s is missing from the tree: %v", path, err)
		}
	}

	for _, target := range embedTargets {
		goos, goarch, _ := strings.Cut(target, "/")
		for _, cgo := range []string{"0", "1"} {
			environment := []string{"GOOS=" + goos, "GOARCH=" + goarch, "CGO_ENABLED=" + cgo}
			realSet, err := listStdlib(real.GOROOT, cache, environment...)
			if err != nil {
				t.Fatal(err)
			}
			listed, err := listStdlib(tree, cache, environment...)
			if err != nil {
				t.Fatal(err)
			}
			for path := range realSet {
				if _, found := listed[path]; !found {
					t.Errorf("%s cgo=%s: the tree does not list %s", target, cgo, path)
				}
			}
			for path, pkg := range listed {
				if _, found := realSet[path]; !found {
					t.Errorf("%s cgo=%s: the tree lists %s, which the real GOROOT does not", target, cgo, path)
				}
				if pkg.Error != nil {
					t.Errorf("%s cgo=%s: %s: %s", target, cgo, path, pkg.Error.Err)
				}
			}
		}
	}

	cmd := exec.CommandContext(context.Background(), filepath.Join(tree, "bin", "go"), "env", "GOROOT", "GOVERSION")
	cmd.Dir = cache
	cmd.Env = append(without(os.Environ(), "GOROOT", "GOTOOLCHAIN"), "GOROOT="+tree, "GOTOOLCHAIN=local")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if want := tree + "\n" + real.Version + "\n"; string(out) != want {
		t.Errorf("go env under the tree answered %q, want %q", out, want)
	}

	t.Setenv("GOROOT", tree)
	inherited, err := Resolve(repoRoot(t), cache, patches)
	if err != nil {
		t.Fatal(err)
	}
	resolvedReal, err := filepath.EvalSymlinks(real.GOROOT)
	if err != nil {
		t.Fatal(err)
	}
	resolvedInherited, err := filepath.EvalSymlinks(inherited.GOROOT)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedInherited != resolvedReal {
		t.Errorf("with GOROOT=%s inherited, Resolve answered %s, want the real %s", tree, inherited.GOROOT, real.GOROOT)
	}
}

// TestResolveAnswersTheRealRootUnderATreeFirstOnPath proves Resolve answers
// the real GOROOT when the inherited environment is a tree's: GOROOT set to
// the tree and the tree's bin first on PATH, as `go test` arranges under le.
// The fake tree's bin and pkg/tool are links to the real toolchain's, which is
// the shape the builder writes. A go invoked through the link on darwin, where
// os.Executable answers the unresolved path, finds the tree as its root, so a
// Resolve that runs the first go on PATH by name answers the tree.
func TestResolveAnswersTheRealRootUnderATreeFirstOnPath(t *testing.T) {
	t.Setenv("GOROOT", "")
	t.Setenv("PATH", pathWithoutTrees(os.Getenv("PATH")))
	patches := shippedPatches(t)
	real := realToolchain(t, t.TempDir())

	cache := t.TempDir()
	tree := filepath.Join(cache, "0123456789abcdef", "goroot")
	if err := os.MkdirAll(filepath.Join(tree, "pkg"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(real.GOROOT, "bin"), filepath.Join(tree, "bin")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(real.GOROOT, "pkg", "tool"), filepath.Join(tree, "pkg", "tool")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOROOT", tree)
	t.Setenv("PATH", filepath.Join(tree, "bin")+string(filepath.ListSeparator)+os.Getenv("PATH"))

	got, err := Resolve(repoRoot(t), cache, patches)
	if err != nil {
		t.Fatal(err)
	}
	resolvedReal, err := filepath.EvalSymlinks(real.GOROOT)
	if err != nil {
		t.Fatal(err)
	}
	resolvedGot, err := filepath.EvalSymlinks(got.GOROOT)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedGot != resolvedReal {
		t.Errorf("with %s/bin first on PATH, Resolve answered %s, want the real %s", tree, got.GOROOT, real.GOROOT)
	}
}

// TestActivateSetsTheProcessEnvironment proves the activation (AC-20): after
// it, GOROOT is the tree, GOTOOLCHAIN is local and PATH starts with exactly
// one <tree>/bin; a second activation under the inherited tree (the nested
// le) adds none; a refusal sets none of the three and Activate answers it
// again on the next call.
func TestActivateSetsTheProcessEnvironment(t *testing.T) {
	root := repoRoot(t)
	t.Setenv("GOROOT", "")
	t.Setenv("GOTOOLCHAIN", "")
	// `go test` puts $GOROOT/bin first on PATH, and under le that is the
	// tree's: start from a PATH that holds no tree, as a fresh shell does.
	t.Setenv("PATH", pathWithoutTrees(os.Getenv("PATH")))
	if err := os.Unsetenv("GOROOT"); err != nil {
		t.Fatal(err)
	}

	tree, err := activate(root)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := activate(root); err != nil || again != tree {
		t.Fatalf("the nested activation answered %q, %v; want %q", again, err, tree)
	}
	if got := os.Getenv("GOROOT"); got != tree {
		t.Errorf("GOROOT = %q, want %q", got, tree)
	}
	if got := os.Getenv("GOTOOLCHAIN"); got != "local" {
		t.Errorf("GOTOOLCHAIN = %q, want local", got)
	}
	entries := filepath.SplitList(os.Getenv("PATH"))
	bin := filepath.Join(tree, "bin")
	if len(entries) == 0 || entries[0] != bin {
		t.Fatalf("PATH does not start with %s: %v", bin, entries)
	}
	count := 0
	for _, entry := range entries {
		if entry == bin {
			count++
		}
	}
	if count != 1 {
		t.Errorf("PATH holds %s %d times, want once", bin, count)
	}

	t.Run("refusal", func(t *testing.T) {
		fixture := t.TempDir()
		if err := os.WriteFile(filepath.Join(fixture, "go.mod"), []byte("module fixture\n\ngo 1.27.2\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(fixture, filepath.FromSlash(PatchDir)), 0o750); err != nil {
			t.Fatal(err)
		}
		stale := strings.Replace(string(shippedPatches(t)[0].Body), stampKey+" go1.27.2", stampKey+" go1.26.0", 1)
		if err := os.WriteFile(filepath.Join(fixture, filepath.FromSlash(PatchDir), "stale.patch"), []byte(stale), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GOROOT", "")
		t.Setenv("GOTOOLCHAIN", "")
		before := os.Getenv("PATH")
		if err := os.Unsetenv("GOROOT"); err != nil {
			t.Fatal(err)
		}

		activation.Lock()
		activation.done = false
		activation.Unlock()
		t.Cleanup(func() {
			activation.Lock()
			activation.done = false
			activation.Unlock()
		})

		_, err := Activate(fixture)
		if err == nil {
			t.Fatal("a stale stamp activated")
		}
		if _, set := os.LookupEnv("GOROOT"); set {
			t.Errorf("a refusal set GOROOT=%s", os.Getenv("GOROOT"))
		}
		if got := os.Getenv("GOTOOLCHAIN"); got != "" {
			t.Errorf("a refusal set GOTOOLCHAIN=%s", got)
		}
		if got := os.Getenv("PATH"); got != before {
			t.Errorf("a refusal changed PATH to %s", got)
		}
		if _, again := Activate(root); again == nil || again.Error() != err.Error() {
			t.Errorf("the second Activate answered %v, want the kept refusal %v", again, err)
		}
	})
}
