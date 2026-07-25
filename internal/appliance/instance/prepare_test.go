package instance

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/mod/modfile"
)

// TestDeriveConfigJSONPreservesFields verifies the JSON patch appends to
// KernelExtraArgs while preserving base args and unknown fields, and never
// mutates the source bytes.
//
// VALIDATES: a prepared instance keeps every config field the build does not
// understand.
// PREVENTS: a gokrazy config field being silently dropped when the instance is
// relocated.
func TestDeriveConfigJSONPreservesFields(t *testing.T) {
	src := []byte(`{
    "Hostname": "ze",
    "KernelExtraArgs": ["loglevel=8"],
    "SomeFutureField": {"nested": [1, 2, 3]},
    "Packages": ["a", "b"]
}`)
	srcCopy := append([]byte(nil), src...)

	out, err := deriveConfigJSON(src, []string{"default_hugepagesz=2M", "hugepages=512"})
	if err != nil {
		t.Fatalf("deriveConfigJSON: %v", err)
	}
	if !bytes.Equal(src, srcCopy) {
		t.Fatal("source bytes were mutated")
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	if _, ok := obj["SomeFutureField"]; !ok {
		t.Error("unknown field SomeFutureField dropped")
	}
	if _, ok := obj["Hostname"]; !ok {
		t.Error("Hostname dropped")
	}

	var args []string
	if err := json.Unmarshal(obj["KernelExtraArgs"], &args); err != nil {
		t.Fatalf("KernelExtraArgs not an array: %v", err)
	}
	want := []string{"loglevel=8", "default_hugepagesz=2M", "hugepages=512"}
	if strings.Join(args, ",") != strings.Join(want, ",") {
		t.Errorf("KernelExtraArgs = %v, want %v (base args must precede appended args)", args, want)
	}
}

// TestDeriveConfigJSONNoBaseArgs verifies KernelExtraArgs is created when the
// source has none.
func TestDeriveConfigJSONNoBaseArgs(t *testing.T) {
	out, err := deriveConfigJSON([]byte(`{"Hostname":"ze"}`), []string{"hugepages=4"})
	if err != nil {
		t.Fatalf("deriveConfigJSON: %v", err)
	}
	var obj struct {
		KernelExtraArgs []string
	}
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if len(obj.KernelExtraArgs) != 1 || obj.KernelExtraArgs[0] != "hugepages=4" {
		t.Errorf("KernelExtraArgs = %v, want [hugepages=4]", obj.KernelExtraArgs)
	}
}

// TestDeriveConfigJSONNoExtraArgs verifies a preparation with no extra kernel
// arguments still produces a valid config and adds no arguments. Preparation is
// unconditional (AC-1), so the empty case is the common one.
//
// VALIDATES: AC-1 -- preparing a plain build changes nothing about its cmdline.
// PREVENTS: unconditional preparation silently altering non-hugepage images.
func TestDeriveConfigJSONNoExtraArgs(t *testing.T) {
	out, err := deriveConfigJSON([]byte(`{"Hostname":"ze","KernelExtraArgs":["loglevel=8"]}`), nil)
	if err != nil {
		t.Fatalf("deriveConfigJSON: %v", err)
	}
	var obj struct {
		KernelExtraArgs []string
	}
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if len(obj.KernelExtraArgs) != 1 || obj.KernelExtraArgs[0] != "loglevel=8" {
		t.Errorf("KernelExtraArgs = %v, want the base args unchanged", obj.KernelExtraArgs)
	}
}

// zeModRelative is the shape of the checked-in ze builddir module: it replaces
// ze with a path six levels up, which is the repository root only at the
// builddir's original depth.
const zeModRelative = `module gokrazy/build/github.com/ze-software/ze

go 1.26

require github.com/ze-software/ze v0.0.0

replace github.com/ze-software/ze => ../../../../../../
`

// gokrazyMod is a builddir module with no filesystem-path replace; it must be
// copied through byte-for-byte.
const gokrazyMod = `module gokrazy/build/ze

go 1.26.2

require github.com/gokrazy/gokrazy v0.0.0-20260703061218-a4a45a20149d // indirect
`

// writePreparedParentFixture lays out a gokrazy parent dir shaped like the
// checked-in one: <root>/gokrazy/ze/{config.json,ze.conf,builddir/...}. It
// returns the repo root and the parent dir.
func writePreparedParentFixture(t *testing.T, origConfig []byte) (root, srcParent string) {
	t.Helper()
	root = t.TempDir()
	srcParent = filepath.Join(root, "gokrazy")
	zeMod := filepath.Join(srcParent, "ze", "builddir", "github.com", "ze-software", "ze")
	gokMod := filepath.Join(srcParent, "ze", "builddir", "github.com", "gokrazy", "gokrazy")
	for _, d := range []string{zeMod, gokMod} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path string, data []byte) {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(zeMod, "go.mod"), []byte(zeModRelative))
	write(filepath.Join(zeMod, "go.sum"), []byte("codeberg.org/example v0.0.0 h1:deadbeef=\n"))
	write(filepath.Join(gokMod, "go.mod"), []byte(gokrazyMod))
	write(filepath.Join(srcParent, "ze", "config.json"), origConfig)
	write(filepath.Join(srcParent, "ze", "ze.conf"), []byte("seed"))
	return root, srcParent
}

// TestPrepare verifies the prepared parent dir carries a patched config.json,
// symlinks sibling files, carries a complete builddir whose filesystem-path
// replaces still resolve from the new depth, lives under the project tmp/, and
// leaves the checked-in source untouched.
//
// VALIDATES: AC-3, AC-4 -- a prepared build resolves the checked-in pins.
// PREVENTS: gok silently falling back to `go get` and building unpinned
// upstream versions, which happened on 2026-07-18 and 2026-07-20 for
// github.com/rtr7/kernel.
func TestPrepare(t *testing.T) {
	origConfig := []byte(`{"Hostname":"ze","KernelExtraArgs":["loglevel=8"]}`)
	root, srcParent := writePreparedParentFixture(t, origConfig)
	srcInstance := filepath.Join(srcParent, "ze")

	parent, cleanup, err := Prepare(srcParent, Options{ExtraKernelArgs: []string{"hugepages=512"}})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	defer cleanup()

	// Patched config.json carries the extra arg.
	patched, err := os.ReadFile(filepath.Join(parent, "ze", "config.json"))
	if err != nil {
		t.Fatalf("read patched config: %v", err)
	}
	if !strings.Contains(string(patched), "hugepages=512") || !strings.Contains(string(patched), "loglevel=8") {
		t.Errorf("patched config missing args:\n%s", patched)
	}

	// ze.conf symlinked in.
	if _, err := os.Stat(filepath.Join(parent, "ze", "ze.conf")); err != nil {
		t.Errorf("ze.conf not present in prepared dir: %v", err)
	}

	// The prepared parent lives under the project tmp/, not the system temp dir.
	wantTmp := filepath.Join(root, "tmp")
	if !strings.HasPrefix(parent, wantTmp+string(filepath.Separator)) {
		t.Errorf("prepared parent %s is not under project tmp %s", parent, wantTmp)
	}

	// builddir travels with the instance: every module and its sum.
	preparedZeMod := filepath.Join(parent, "ze", "builddir", "github.com", "ze-software", "ze", "go.mod")
	preparedGokMod := filepath.Join(parent, "ze", "builddir", "github.com", "gokrazy", "gokrazy", "go.mod")
	preparedZeSum := filepath.Join(parent, "ze", "builddir", "github.com", "ze-software", "ze", "go.sum")
	for _, p := range []string{preparedZeMod, preparedGokMod, preparedZeSum} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("builddir entry missing from prepared instance: %v", err)
		}
	}

	// The relative self-replace is rewritten to the absolute path it named at the
	// source depth, so it still resolves to the repository root.
	preparedData, err := os.ReadFile(preparedZeMod)
	if err != nil {
		t.Fatalf("read prepared ze go.mod: %v", err)
	}
	f, err := modfile.Parse(preparedZeMod, preparedData, nil)
	if err != nil {
		t.Fatalf("parse prepared ze go.mod: %v", err)
	}
	if len(f.Replace) != 1 {
		t.Fatalf("prepared ze go.mod has %d replaces, want 1:\n%s", len(f.Replace), preparedData)
	}
	// Compare resolved paths, not spellings: copyBuildDir resolves the builddir
	// source through EvalSymlinks, and on darwin the tempdir prefix /var is a
	// symlink to /private/var, so the written target is the resolved form of
	// the same directory. The property under test is "the replace resolves to
	// the repository root", not a particular spelling of it.
	got := f.Replace[0].New.Path
	gotResolved, gotErr := filepath.EvalSymlinks(got)
	wantResolved, wantErr := filepath.EvalSymlinks(root)
	if gotErr != nil || wantErr != nil || gotResolved != wantResolved {
		t.Errorf("replace target = %q (resolved %q, %v), want the repo root %q (resolved %q, %v)",
			got, gotResolved, gotErr, root, wantResolved, wantErr)
	}
	if len(f.Require) != 1 || f.Require[0].Mod.Path != "github.com/ze-software/ze" {
		t.Errorf("prepared ze go.mod lost its require:\n%s", preparedData)
	}

	// A module with no filesystem-path replace is copied byte-for-byte.
	gokData, err := os.ReadFile(preparedGokMod)
	if err != nil {
		t.Fatalf("read prepared gokrazy go.mod: %v", err)
	}
	if string(gokData) != gokrazyMod {
		t.Errorf("gokrazy go.mod was rewritten:\n%s", gokData)
	}

	// Source config.json and source go.mod untouched.
	after, err := os.ReadFile(filepath.Join(srcInstance, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, origConfig) {
		t.Errorf("source config.json was modified:\n%s", after)
	}
	srcMod, err := os.ReadFile(filepath.Join(srcInstance, "builddir", "github.com", "ze-software", "ze", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if string(srcMod) != zeModRelative {
		t.Errorf("source ze go.mod was modified:\n%s", srcMod)
	}

	// Cleanup removes the prepared dir.
	cleanup()
	if _, err := os.Stat(parent); !os.IsNotExist(err) {
		t.Errorf("cleanup did not remove prepared dir: %v", err)
	}
}

// kernelMod is the shape of the checked-in rtr7/kernel builddir module: it pins
// the kernel by version and carries a module-to-module version replace.
const kernelMod = `module gokrazy/build/ze

go 1.26.2

require github.com/rtr7/kernel v0.0.0-20260403073601-5a996da3a37b // indirect

replace github.com/gokrazy/gokrazy v0.0.0-20200501080617-f3445e01a904 => github.com/gokrazy/gokrazy v0.0.0-20260703061218-a4a45a20149d
`

// writeKernelModule adds a checked-in rtr7/kernel builddir module to a fixture
// parent, so kernel-package injection has the module it must rewrite.
func writeKernelModule(t *testing.T, srcParent string) string {
	t.Helper()
	dir := filepath.Join(srcParent, "ze", "builddir", "github.com", "rtr7", "kernel")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(kernelMod), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestPrepareInjectsKernelReplace verifies an out-of-tree kernel package is
// injected into the PREPARED copy only, as an absolute path, leaving the tracked
// module byte-identical.
//
// This replaces `make ze-kernel` editing gokrazy/ze/builddir/github.com/rtr7/
// kernel/go.mod in place, which made a build step write to a tracked file and
// left the tree dirty until ze-kernel-clean was run.
//
// VALIDATES: AC-7 -- the replace reaches the prepared copy, no tracked file
// changes.
// PREVENTS: a build mutating tracked state, and a stale custom-kernel replace
// surviving into an unrelated build or commit.
func TestPrepareInjectsKernelReplace(t *testing.T) {
	_, srcParent := writePreparedParentFixture(t, []byte(`{"Hostname":"ze"}`))
	srcKernelDir := writeKernelModule(t, srcParent)
	pkg := filepath.Join(t.TempDir(), "kernelpkg")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}

	prepared, cleanup, err := Prepare(srcParent, Options{KernelPackage: pkg})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	defer cleanup()

	preparedKernel := filepath.Join(prepared, "ze", "builddir", "github.com", "rtr7", "kernel", GoModName)
	data, err := os.ReadFile(preparedKernel)
	if err != nil {
		t.Fatalf("read prepared kernel go.mod: %v", err)
	}
	f, err := modfile.Parse(preparedKernel, data, nil)
	if err != nil {
		t.Fatalf("parse prepared kernel go.mod: %v", err)
	}

	var got string
	for _, r := range f.Replace {
		if r.Old.Path == KernelModule {
			got = r.New.Path
		}
	}
	if got == "" {
		t.Fatalf("no replace for %s in the prepared module:\n%s", KernelModule, data)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("kernel replace %q is not absolute; it would resolve against the prepared depth", got)
	}
	if got != pkg {
		t.Errorf("kernel replace = %q, want %q", got, pkg)
	}

	// The pre-existing version replace must survive alongside the new one.
	if len(f.Replace) != 2 {
		t.Errorf("prepared kernel go.mod has %d replaces, want 2 (the existing version replace plus the kernel package):\n%s", len(f.Replace), data)
	}

	// The tracked module is untouched.
	srcData, err := os.ReadFile(filepath.Join(srcKernelDir, GoModName))
	if err != nil {
		t.Fatal(err)
	}
	if string(srcData) != kernelMod {
		t.Errorf("the tracked kernel go.mod was modified by a build:\n%s", srcData)
	}
}

// TestPrepareNoKernelPackageUsesPin verifies that with no kernel package the
// prepared module keeps the pinned kernel and gains no replace, so a build
// cannot inherit a custom kernel from leftover state.
//
// VALIDATES: AC-8.
// PREVENTS: the old failure mode where a custom kernel persisted in the tracked
// go.mod until the operator remembered ze-kernel-clean.
func TestPrepareNoKernelPackageUsesPin(t *testing.T) {
	_, srcParent := writePreparedParentFixture(t, []byte(`{"Hostname":"ze"}`))
	writeKernelModule(t, srcParent)

	prepared, cleanup, err := Prepare(srcParent, Options{})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	defer cleanup()

	preparedKernel := filepath.Join(prepared, "ze", "builddir", "github.com", "rtr7", "kernel", GoModName)
	data, err := os.ReadFile(preparedKernel)
	if err != nil {
		t.Fatalf("read prepared kernel go.mod: %v", err)
	}
	if string(data) != kernelMod {
		t.Errorf("the kernel module was rewritten though no kernel package was given:\n%s", data)
	}
}

// TestPrepareRejectsMissingKernelPackage verifies a kernel package path that does
// not exist is an error, not a silently ignored request that would hand the
// operator a pinned-kernel image while they believe they are testing their own.
//
// VALIDATES: fail-closed on an explicit parameter.
func TestPrepareRejectsMissingKernelPackage(t *testing.T) {
	_, srcParent := writePreparedParentFixture(t, []byte(`{"Hostname":"ze"}`))
	writeKernelModule(t, srcParent)
	missing := filepath.Join(t.TempDir(), "nope")

	_, cleanup, err := Prepare(srcParent, Options{KernelPackage: missing})
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		t.Fatal("Prepare accepted a kernel package that does not exist")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error does not name the missing path: %v", err)
	}
}

// TestReapStalePreparedDirs verifies that a prepared dir left behind by a build
// that died without cleanup (gok's pack.Main os.Exit skips both the deferred and
// the explicit cleanup) is removed by a later preparation, while a fresh sibling
// from a concurrent build is left alone.
//
// VALIDATES: the leak on gok's os.Exit build-failure path is bounded.
// PREVENTS: unbounded accumulation of orphaned tmp/appliance-build-* dirs.
func TestReapStalePreparedDirs(t *testing.T) {
	_, srcParent := writePreparedParentFixture(t, []byte(`{"Hostname":"ze"}`))
	tmpRoot := filepath.Join(filepath.Dir(srcParent), "tmp")
	if err := os.MkdirAll(tmpRoot, 0o750); err != nil {
		t.Fatal(err)
	}

	// A leaked dir from a long-dead build: old mtime.
	stale := filepath.Join(tmpRoot, "appliance-build-STALE")
	if err := os.MkdirAll(stale, 0o750); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-3 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	// A fresh dir from a concurrent build: recent mtime. Must survive.
	fresh := filepath.Join(tmpRoot, "appliance-build-FRESH")
	if err := os.MkdirAll(fresh, 0o750); err != nil {
		t.Fatal(err)
	}

	// A sibling that is not ours: must never be touched.
	bystander := filepath.Join(tmpRoot, "something-else")
	if err := os.MkdirAll(bystander, 0o750); err != nil {
		t.Fatal(err)
	}

	_, cleanup, err := Prepare(srcParent, Options{})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	defer cleanup()

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale prepared dir was not reaped: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("a fresh concurrent build's dir was wrongly reaped: %v", err)
	}
	if _, err := os.Stat(bystander); err != nil {
		t.Errorf("an unrelated sibling was reaped: %v", err)
	}
}

// TestPrepareIsolatesConcurrentBuilds verifies two preparations of the same
// source get distinct directories, so builds in one checkout cannot collide.
//
// VALIDATES: AC-11.
// PREVENTS: two concurrent builds in one checkout corrupting each other.
func TestPrepareIsolatesConcurrentBuilds(t *testing.T) {
	_, srcParent := writePreparedParentFixture(t, []byte(`{"Hostname":"ze"}`))

	first, cleanupFirst, err := Prepare(srcParent, Options{ExtraKernelArgs: []string{"hugepages=4"}})
	if err != nil {
		t.Fatalf("first Prepare: %v", err)
	}
	defer cleanupFirst()
	second, cleanupSecond, err := Prepare(srcParent, Options{ExtraKernelArgs: []string{"hugepages=4"}})
	if err != nil {
		t.Fatalf("second Prepare: %v", err)
	}
	defer cleanupSecond()

	if first == second {
		t.Errorf("both builds share the prepared dir %s", first)
	}
}

// TestPrepareFailsClosedWithoutBuildDir verifies a source instance with NO
// builddir is refused. copyBuildDir guards an empty builddir, but an absent one
// would simply never be copied, and gok would resolve everything over the
// network from a prepared instance that looked fine.
//
// VALIDATES: AC-6 -- the fail-closed guard covers absence, not just emptiness.
// PREVENTS: a caller that assembles its own parent dir (the L2TP evidence
// script) silently producing an unpinned image by forgetting the builddir.
func TestPrepareFailsClosedWithoutBuildDir(t *testing.T) {
	root := t.TempDir()
	srcParent := filepath.Join(root, "gokrazy")
	if err := os.MkdirAll(filepath.Join(srcParent, "ze"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcParent, "ze", "config.json"), []byte(`{"Hostname":"ze"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, cleanup, err := Prepare(srcParent, Options{})
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		t.Fatal("Prepare accepted an instance with no builddir")
	}
	if !strings.Contains(err.Error(), buildDirName) {
		t.Errorf("error does not name the missing directory: %v", err)
	}
}

// TestPrepareAcceptsSymlinkedBuildDir verifies a builddir that is a SYMLINK to
// the real one is copied through. This lets a caller assemble a parent dir with
// a patched config.json without duplicating the builddir first, which is how the
// L2TP evidence script avoids carrying its own copy of this logic.
//
// VALIDATES: one preparer, not several.
// PREVENTS: callers reimplementing the copy because a symlink was rejected.
func TestPrepareAcceptsSymlinkedBuildDir(t *testing.T) {
	_, realParent := writePreparedParentFixture(t, []byte(`{"Hostname":"ze"}`))

	// A second parent whose builddir is a symlink to the real one.
	linkRoot := t.TempDir()
	linkParent := filepath.Join(linkRoot, "gokrazy")
	linkInstance := filepath.Join(linkParent, "ze")
	if err := os.MkdirAll(linkInstance, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(linkInstance, "config.json"), []byte(`{"Hostname":"ze"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(realParent, "ze", "builddir"), filepath.Join(linkInstance, "builddir")); err != nil {
		t.Fatal(err)
	}

	prepared, cleanup, err := Prepare(linkParent, Options{})
	if err != nil {
		t.Fatalf("Prepare rejected a symlinked builddir: %v", err)
	}
	defer cleanup()

	zeMod := filepath.Join(prepared, "ze", "builddir", "github.com", "ze-software", "ze", GoModName)
	data, err := os.ReadFile(zeMod)
	if err != nil {
		t.Fatalf("prepared instance is missing the ze module: %v", err)
	}
	f, err := modfile.Parse(zeMod, data, nil)
	if err != nil {
		t.Fatalf("parse prepared ze go.mod: %v", err)
	}
	if len(f.Replace) != 1 || !filepath.IsAbs(f.Replace[0].New.Path) {
		t.Errorf("the self-replace was not absolutized through the symlink:\n%s", data)
	}
}

// TestCopyBuildDirFailsClosedWithoutModules verifies an empty builddir is a loud
// error, never a quiet copy that would leave gok resolving over the network.
//
// VALIDATES: AC-6 -- fail-closed guard on the pin set.
// PREVENTS: a silent unpinned build after a copy or layout regression.
func TestCopyBuildDirFailsClosedWithoutModules(t *testing.T) {
	src := filepath.Join(t.TempDir(), "builddir")
	if err := os.MkdirAll(filepath.Join(src, "github.com", "gokrazy"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := copyBuildDir(src, filepath.Join(t.TempDir(), "dst"))
	if err == nil {
		t.Fatal("copyBuildDir accepted a builddir with no go.mod")
	}
	if !strings.Contains(err.Error(), "no go.mod found") {
		t.Errorf("error does not name the cause: %v", err)
	}
}

// TestAbsolutizeReplacesLeavesVersionReplaces verifies a module-to-module version
// replace (serial-busybox, rtr7/kernel) is not touched, and the file is returned
// unchanged when there is nothing to rewrite.
func TestAbsolutizeReplacesLeavesVersionReplaces(t *testing.T) {
	const versionReplace = `module gokrazy/build/ze

go 1.26.2

require github.com/gokrazy/serial-busybox v0.0.0-20250119153030-ac58ba7574e7 // indirect

replace github.com/gokrazy/gokrazy v0.0.0-20200501080617-f3445e01a904 => github.com/gokrazy/gokrazy v0.0.0-20260703061218-a4a45a20149d
`
	out, err := absolutizeReplaces(filepath.Join(t.TempDir(), "go.mod"), []byte(versionReplace))
	if err != nil {
		t.Fatalf("absolutizeReplaces: %v", err)
	}
	if string(out) != versionReplace {
		t.Errorf("version replace was rewritten:\n%s", out)
	}
}
