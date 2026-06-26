package appliance

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveCacheDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	got := resolveCacheDir()
	want := filepath.Join(home, ".cache", cacheSubdir)
	if got != want {
		t.Errorf("resolveCacheDir() = %q, want %q", got, want)
	}
}

func TestResolveCacheDirCustom(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom-cache")
	t.Setenv("XDG_CACHE_HOME", custom)
	got := resolveCacheDir()
	want := filepath.Join(custom, cacheSubdir)
	if got != want {
		t.Errorf("resolveCacheDir() = %q, want %q", got, want)
	}
}

func TestCopyTreePreservesSymlinkAndFiles(t *testing.T) {
	// VALIDATES: copyTree preserves regular files, nested dirs, AND symlinks
	// (matching the `cp -R` Make path), rather than silently dropping symlinks.
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "lib", "modules", "7.1.1-ze"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "vmlinuz"), []byte("kernel"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "lib", "modules", "7.1.1-ze", "modules.dep"), []byte("dep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("modules.dep", filepath.Join(src, "lib", "modules", "7.1.1-ze", "modules.alias")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "cached")
	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copyTree: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(dst, "vmlinuz")); err != nil || string(data) != "kernel" {
		t.Errorf("vmlinuz not copied: %v %q", err, data)
	}
	if _, err := os.Stat(filepath.Join(dst, "lib", "modules", "7.1.1-ze", "modules.dep")); err != nil {
		t.Errorf("nested file not copied: %v", err)
	}
	linkPath := filepath.Join(dst, "lib", "modules", "7.1.1-ze", "modules.alias")
	info, err := os.Lstat(linkPath)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink not preserved: %v mode=%v", err, info.Mode())
	}
	if target, err := os.Readlink(linkPath); err != nil || target != "modules.dep" {
		t.Errorf("symlink target = %q (err %v), want modules.dep", target, err)
	}
}
