package appliance

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestKernelArgsHugepages verifies the assembly emits the three hugepage tokens
// in deterministic order when configured, and nothing when unconfigured (AC-8).
func TestKernelArgsHugepages(t *testing.T) {
	t.Run("unconfigured returns nil", func(t *testing.T) {
		if got := hugepageKernelArgs(ImageConfig{}); got != nil {
			t.Errorf("expected nil args, got %v", got)
		}
	})

	t.Run("2M reservation", func(t *testing.T) {
		got := hugepageKernelArgs(ImageConfig{Hugepages: &Hugepages{PageSize: "2M", Count: 512}})
		want := []string{"default_hugepagesz=2M", "hugepagesz=2M", "hugepages=512"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("arg[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("1G reservation", func(t *testing.T) {
		got := hugepageKernelArgs(ImageConfig{Hugepages: &Hugepages{PageSize: "1G", Count: 8}})
		want := []string{"default_hugepagesz=1G", "hugepagesz=1G", "hugepages=8"}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("arg[%d] = %q, want %q", i, got[i], want[i])
			}
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

// TestMaterializeDerivedParent verifies the derived parent dir carries a patched
// config.json, symlinks sibling files, excludes builddir (cold rebuild), and
// leaves the checked-in source untouched (AC-8).
func TestMaterializeDerivedParent(t *testing.T) {
	srcParent := t.TempDir()
	srcInstance := filepath.Join(srcParent, "ze")
	if err := os.MkdirAll(filepath.Join(srcInstance, "builddir"), 0o755); err != nil {
		t.Fatal(err)
	}
	origConfig := []byte(`{"Hostname":"ze","KernelExtraArgs":["loglevel=8"]}`)
	if err := os.WriteFile(filepath.Join(srcInstance, "config.json"), origConfig, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcInstance, "ze.conf"), []byte("seed"), 0o644); err != nil {
		t.Fatal(err)
	}

	parent, cleanup, err := materializeDerivedParent(srcParent, "ze", []string{"hugepages=512"})
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

	// builddir deliberately excluded (cold rebuild).
	if _, err := os.Lstat(filepath.Join(parent, "ze", "builddir")); !os.IsNotExist(err) {
		t.Errorf("builddir should be excluded from derived dir, got err=%v", err)
	}

	// Source config.json untouched.
	after, err := os.ReadFile(filepath.Join(srcInstance, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, origConfig) {
		t.Errorf("source config.json was modified:\n%s", after)
	}

	// Cleanup removes the temp dir.
	cleanup()
	if _, err := os.Stat(parent); !os.IsNotExist(err) {
		t.Errorf("cleanup did not remove temp dir: %v", err)
	}
}
