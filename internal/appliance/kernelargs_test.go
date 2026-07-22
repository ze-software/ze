package appliance

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

// TestKernelArgsHugepages verifies the assembly emits the three hugepage tokens
// in deterministic order when configured, and nothing when unconfigured (AC-8).
func TestKernelArgsHugepages(t *testing.T) {
	t.Run("unconfigured returns nil", func(t *testing.T) {
		got, err := hugepageKernelArgs(ImageConfig{})
		if err != nil || got != nil {
			t.Errorf("expected (nil, nil), got (%v, %v)", got, err)
		}
	})

	t.Run("1gb of 2mb pages", func(t *testing.T) {
		got, err := hugepageKernelArgs(ImageConfig{Hugepages: &Hugepages{Size: "1gb", PageSize: "2mb"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"default_hugepagesz=2M", "hugepagesz=2M", "hugepages=512"} // 1gb / 2mb = 512
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("arg[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("8gb of 1gb pages", func(t *testing.T) {
		got, err := hugepageKernelArgs(ImageConfig{Hugepages: &Hugepages{Size: "8gb", PageSize: "1gb"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"default_hugepagesz=1G", "hugepagesz=1G", "hugepages=8"} // 8gb / 1gb = 8
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("arg[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("invalid page-size errors", func(t *testing.T) {
		if _, err := hugepageKernelArgs(ImageConfig{Hugepages: &Hugepages{Size: "1gb", PageSize: "4mb"}}); err == nil {
			t.Error("expected an error for an unsupported page-size")
		}
	})
}

// TestDerivedInstanceConfigPreservesFields verifies the JSON patch appends to
// KernelExtraArgs while preserving base args and unknown fields (R-4), and never
// mutates the source bytes.
func TestDerivedInstanceConfigPreservesFields(t *testing.T) {
	src := []byte(`{
    "Hostname": "ze",
    "KernelExtraArgs": ["loglevel=8"],
    "SomeFutureField": {"nested": [1, 2, 3]},
    "Packages": ["a", "b"]
}`)
	srcCopy := append([]byte(nil), src...)

	out, err := deriveInstanceConfigJSON(src, []string{"default_hugepagesz=2M", "hugepages=512"})
	if err != nil {
		t.Fatalf("deriveInstanceConfigJSON: %v", err)
	}
	if !bytes.Equal(src, srcCopy) {
		t.Fatal("source bytes were mutated")
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	// Unknown field survives verbatim.
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

// TestDerivedInstanceConfigNoBaseArgs verifies KernelExtraArgs is created when
// the source has none.
func TestDerivedInstanceConfigNoBaseArgs(t *testing.T) {
	out, err := deriveInstanceConfigJSON([]byte(`{"Hostname":"ze"}`), []string{"hugepages=4"})
	if err != nil {
		t.Fatalf("deriveInstanceConfigJSON: %v", err)
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

// zeModRelative is the shape of the checked-in ze builddir module: it replaces
// ze with a path six levels up, which is the repository root only at the
// builddir's original depth.
const zeModRelative = `module gokrazy/build/codeberg.org/thomas-mangin/ze

go 1.26

require codeberg.org/thomas-mangin/ze v0.0.0

replace codeberg.org/thomas-mangin/ze => ../../../../../../
`

// gokrazyMod is a builddir module with no filesystem-path replace; it must be
// copied through byte-for-byte.
const gokrazyMod = `module gokrazy/build/ze

go 1.26.2

require github.com/gokrazy/gokrazy v0.0.0-20260703061218-a4a45a20149d // indirect
`

// writeDerivedParentFixture lays out a gokrazy parent dir shaped like the
// checked-in one: <root>/gokrazy/ze/{config.json,ze.conf,builddir/...}. It
// returns the repo root and the parent dir.
func writeDerivedParentFixture(t *testing.T, origConfig []byte) (root, srcParent string) {
	t.Helper()
	root = t.TempDir()
	srcParent = filepath.Join(root, "gokrazy")
	zeMod := filepath.Join(srcParent, "ze", "builddir", "codeberg.org", "thomas-mangin", "ze")
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

// TestMaterializeDerivedParent verifies the derived parent dir carries a patched
// config.json, symlinks sibling files, carries a complete builddir whose
// filesystem-path replaces still resolve from the new depth, lives under the
// project tmp/, and leaves the checked-in source untouched (AC-8).
//
// VALIDATES: a derived (hugepage) build resolves the checked-in pins.
// PREVENTS: gok silently falling back to `go get` and building unpinned
// upstream versions, which happened on 2026-07-18 and 2026-07-20 for
// github.com/rtr7/kernel.
func TestMaterializeDerivedParent(t *testing.T) {
	origConfig := []byte(`{"Hostname":"ze","KernelExtraArgs":["loglevel=8"]}`)
	root, srcParent := writeDerivedParentFixture(t, origConfig)
	srcInstance := filepath.Join(srcParent, "ze")

	parent, cleanup, err := materializeDerivedParent(srcParent, []string{"hugepages=512"})
	if err != nil {
		t.Fatalf("materializeDerivedParent: %v", err)
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
		t.Errorf("ze.conf not present in derived dir: %v", err)
	}

	// The derived parent lives under the project tmp/, not the system temp dir.
	wantTmp := filepath.Join(root, "tmp")
	if !strings.HasPrefix(parent, wantTmp+string(filepath.Separator)) {
		t.Errorf("derived parent %s is not under project tmp %s", parent, wantTmp)
	}

	// builddir travels with the instance: every module and its sum.
	derivedZeMod := filepath.Join(parent, "ze", "builddir", "codeberg.org", "thomas-mangin", "ze", "go.mod")
	derivedGokMod := filepath.Join(parent, "ze", "builddir", "github.com", "gokrazy", "gokrazy", "go.mod")
	derivedZeSum := filepath.Join(parent, "ze", "builddir", "codeberg.org", "thomas-mangin", "ze", "go.sum")
	for _, p := range []string{derivedZeMod, derivedGokMod, derivedZeSum} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("builddir entry missing from derived instance: %v", err)
		}
	}

	// The relative self-replace is rewritten to the absolute path it named at the
	// source depth, so it still resolves to the repository root.
	derivedData, err := os.ReadFile(derivedZeMod)
	if err != nil {
		t.Fatalf("read derived ze go.mod: %v", err)
	}
	f, err := modfile.Parse(derivedZeMod, derivedData, nil)
	if err != nil {
		t.Fatalf("parse derived ze go.mod: %v", err)
	}
	if len(f.Replace) != 1 {
		t.Fatalf("derived ze go.mod has %d replaces, want 1:\n%s", len(f.Replace), derivedData)
	}
	if got := f.Replace[0].New.Path; got != root {
		t.Errorf("replace target = %q, want the repo root %q", got, root)
	}
	if len(f.Require) != 1 || f.Require[0].Mod.Path != "codeberg.org/thomas-mangin/ze" {
		t.Errorf("derived ze go.mod lost its require:\n%s", derivedData)
	}

	// A module with no filesystem-path replace is copied byte-for-byte.
	gokData, err := os.ReadFile(derivedGokMod)
	if err != nil {
		t.Fatalf("read derived gokrazy go.mod: %v", err)
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
	srcMod, err := os.ReadFile(filepath.Join(srcInstance, "builddir", "codeberg.org", "thomas-mangin", "ze", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if string(srcMod) != zeModRelative {
		t.Errorf("source ze go.mod was modified:\n%s", srcMod)
	}

	// Cleanup removes the derived dir.
	cleanup()
	if _, err := os.Stat(parent); !os.IsNotExist(err) {
		t.Errorf("cleanup did not remove derived dir: %v", err)
	}
}

// TestMaterializeDerivedParentIsolatesConcurrentBuilds verifies two preparations
// of the same source get distinct directories, so builds in one checkout cannot
// collide.
func TestMaterializeDerivedParentIsolatesConcurrentBuilds(t *testing.T) {
	_, srcParent := writeDerivedParentFixture(t, []byte(`{"Hostname":"ze"}`))

	first, cleanupFirst, err := materializeDerivedParent(srcParent, []string{"hugepages=4"})
	if err != nil {
		t.Fatalf("first materializeDerivedParent: %v", err)
	}
	defer cleanupFirst()
	second, cleanupSecond, err := materializeDerivedParent(srcParent, []string{"hugepages=4"})
	if err != nil {
		t.Fatalf("second materializeDerivedParent: %v", err)
	}
	defer cleanupSecond()

	if first == second {
		t.Errorf("both builds share the derived dir %s", first)
	}
}

// TestCopyBuildDirFailsClosedWithoutModules verifies an empty builddir is a loud
// error, never a quiet copy that would leave gok resolving over the network.
//
// VALIDATES: fail-closed guard on the pin set.
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
