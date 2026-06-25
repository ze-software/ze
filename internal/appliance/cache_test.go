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
