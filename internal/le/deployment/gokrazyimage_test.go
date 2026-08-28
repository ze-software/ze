package deployment

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAssembleKernelPackageCombinesPinAndRuntimeTree(t *testing.T) {
	root := t.TempDir()
	version := "v0.0.0-test"
	writeImageFixture(t, root, "gokrazy/ze/builddir/github.com/rtr7/kernel/go.mod",
		"module gokrazy/build/ze\n\nrequire github.com/rtr7/kernel "+version+"\n", 0o644)
	module := "gokrazy/modcache/github.com/rtr7/kernel@" + version
	writeImageFixture(t, root, module+"/kernel.go", "package kernel\n", 0o444)
	writeImageFixture(t, root, module+"/vmlinuz", "old", 0o444)
	writeImageFixture(t, root, module+"/lib/modules/old/modules.builtin", "old", 0o444)

	runtimeTree := filepath.Join(root, "runtime")
	writeImageFixture(t, root, "runtime/vmlinuz", "new-kernel", 0o644)
	writeImageFixture(t, root, "runtime/lib/modules/6.12/modules.builtin", "kernel/net/l2tp/l2tp_ppp.ko\n", 0o644)
	writeImageFixture(t, root, "runtime/board.dtb", "dtb", 0o644)
	writeImageFixture(t, root, "runtime/overlays/board.dtbo", "overlay", 0o644)

	destination := filepath.Join(root, "pkg")
	if err := assembleKernelPackage(root, runtimeTree, destination); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		"kernel.go":                        "package kernel\n",
		"vmlinuz":                          "new-kernel",
		"lib/modules/6.12/modules.builtin": "kernel/net/l2tp/l2tp_ppp.ko\n",
		"board.dtb":                        "dtb",
		"overlays/board.dtbo":              "overlay",
	} {
		body, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(path)))
		if err != nil || string(body) != want {
			t.Errorf("%s = %q, %v; want %q", path, body, err, want)
		}
	}
	if _, err := os.Stat(filepath.Join(destination, "lib", "modules", "old")); !os.IsNotExist(err) {
		t.Errorf("old module tree remains: %v", err)
	}
	info, err := os.Stat(filepath.Join(destination, "kernel.go"))
	if err != nil || info.Mode().Perm()&0o200 == 0 {
		t.Errorf("copied module is not owner-writable: %v, %v", info, err)
	}
}

func TestKernelModuleVersionFailsClosed(t *testing.T) {
	root := t.TempDir()
	writeImageFixture(t, root, "gokrazy/ze/builddir/github.com/rtr7/kernel/go.mod",
		"module gokrazy/build/ze\n", 0o644)
	if _, err := kernelModuleVersion(root); err == nil {
		t.Fatal("missing kernel requirement was accepted")
	}
}

func writeImageFixture(t *testing.T, root, relative, content string, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}
