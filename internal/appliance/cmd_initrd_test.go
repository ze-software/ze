package appliance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

func TestRunDispatchesInitrd(t *testing.T) {
	table := dispatchTable()
	if _, ok := table["initrd"]; !ok {
		t.Fatal("initrd not registered in dispatch table")
	}
}

func TestInitrdResolvesCache(t *testing.T) {
	t.Chdir(t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	cached := initrdCachePath(defaultInitrdVersion, runtime.GOARCH)
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached, []byte("fake-initrd"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveInitrd()
	if err != nil {
		t.Fatalf("resolveInitrd: %v", err)
	}
	if got != cached {
		t.Errorf("resolveInitrd = %q, want %q", got, cached)
	}
}

// VALIDATES: cache-hit branch copies initrd to build/initrd/ for downstream ISO.
// PREVENTS: appliance iso failing with "installer initrd not found" after a cache-hit resolve.
func TestInitrdCacheHitCopiesToToolsPath(t *testing.T) {
	t.Chdir(t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	content := []byte("cached-initrd")
	cached := initrdCachePath(defaultInitrdVersion, runtime.GOARCH)
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached, content, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := resolveInitrd(); err != nil {
		t.Fatalf("resolveInitrd: %v", err)
	}

	toolsPath := filepath.Join(initrdToolsDir, initrdFileName)
	got, err := os.ReadFile(toolsPath)
	if err != nil {
		t.Fatalf("tools path %s not written on cache hit: %v", toolsPath, err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("tools path content = %q, want %q", got, content)
	}
}

func TestInitrdDownloadsAndCaches(t *testing.T) {
	t.Chdir(t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	content := []byte("test-initrd-content")
	sum := sha256.Sum256(content)
	checksumHex := hex.EncodeToString(sum[:])

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/initrd.img.gz"):
			w.Write(content) //nolint:errcheck // test fixture
		case strings.HasSuffix(r.URL.Path, "/initrd.img.gz.sha256"):
			fmt.Fprintf(w, "%s  initrd.img.gz\n", checksumHex) //nolint:errcheck // test fixture
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	setTestHTTP(t, srv.Client().Get)
	t.Setenv("ZE_APPLIANCE_INITRD_URL", srv.URL)
	env.ResetCache()

	got, err := resolveInitrd()
	if err != nil {
		t.Fatalf("resolveInitrd: %v", err)
	}
	if !strings.Contains(got, cacheDir) {
		t.Errorf("expected path under cache dir, got %q", got)
	}
}

func TestInitrdFallsBackToLocalBuild(t *testing.T) {
	t.Chdir(t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	setTestHTTP(t, func(url string) (*http.Response, error) { return nil, errors.New("network error") })

	oldLookPath := initrdLookPathFn
	initrdLookPathFn = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	t.Cleanup(func() { initrdLookPathFn = oldLookPath })

	oldMake := initrdMakeBuildFn
	initrdMakeBuildFn = func(destPath string) error {
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destPath, []byte("make-built-initrd"), 0o644)
	}
	t.Cleanup(func() { initrdMakeBuildFn = oldMake })

	got, err := resolveInitrd()
	if err != nil {
		t.Fatalf("resolveInitrd: %v", err)
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("cached initrd not at %q: %v", got, err)
	}
}

// the old "missing build tools" path tested cpio/gzip lookup failure.
// The initrd build no longer uses external cpio/gzip (pure Go cpio+compress/gzip),
// so that path is unreachable. This test verifies the build fails clearly when
// the Go source is absent (e.g., running from a detached temp dir).
func TestInitrdFailsWhenBuildFails(t *testing.T) {
	t.Chdir(t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	setTestHTTP(t, func(url string) (*http.Response, error) { return nil, errors.New("network error") })

	_, err := resolveInitrd()
	if err == nil {
		t.Fatal("expected error when download fails and go build has no source")
	}
	if !strings.Contains(err.Error(), "initrd build") {
		t.Errorf("error should mention initrd build failure, got: %v", err)
	}
}

// cache inputs changed from shell files (tools/installer-initrd/{init,Makefile})
// to Go sources (cmd/ze-installer/main.go, internal/install/disk/*_linux.go) because the
// initrd build was rewritten from busybox+shell to a pure-Go binary. The old
// TestInitrdInitScriptInvalidatesCache and TestInitrdMakefileInvalidatesCache are replaced
// by TestInitrdSourceChangeInvalidatesCache, TestInitrdCacheVariantIncludesArch, and
// TestInitrdCacheVariantChangesOnSourceEdit which test the same property (source change
// invalidates cache) against the new inputs.

func TestInitrdSourceChangeInvalidatesCache(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	createInitrdCacheInputs(t, dir, "v1")

	cached1 := initrdCachePath(defaultInitrdVersion, runtime.GOARCH)
	if err := os.MkdirAll(filepath.Dir(cached1), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached1, []byte("initrd-v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveInitrd()
	if err != nil {
		t.Fatalf("resolveInitrd before source change: %v", err)
	}
	if got != cached1 {
		t.Fatalf("expected cache hit at %q, got %q", cached1, got)
	}

	createInitrdCacheInputs(t, dir, "v2")

	oldLookPath := initrdLookPathFn
	initrdLookPathFn = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	t.Cleanup(func() { initrdLookPathFn = oldLookPath })

	oldMake := initrdMakeBuildFn
	initrdMakeBuildFn = func(destPath string) error {
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destPath, []byte("initrd-v2"), 0o644)
	}
	t.Cleanup(func() { initrdMakeBuildFn = oldMake })

	got2, err := resolveInitrd()
	if err != nil {
		t.Fatalf("resolveInitrd after source change: %v", err)
	}
	if got2 == cached1 {
		t.Error("source change did not invalidate initrd cache")
	}

	data, err := os.ReadFile(got2)
	if err != nil {
		t.Fatalf("read rebuilt initrd: %v", err)
	}
	if string(data) != "initrd-v2" {
		t.Errorf("rebuilt initrd content = %q, want initrd-v2", data)
	}
}

func TestInitrdCacheVariantIncludesArch(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	createInitrdCacheInputs(t, dir, "v1")

	v1 := initrdCacheVariant(defaultInitrdVersion, "amd64")
	v2 := initrdCacheVariant(defaultInitrdVersion, "arm64")
	if v1 == v2 {
		t.Fatal("cache variant should differ between amd64 and arm64")
	}
}

func TestInitrdCacheVariantChangesOnSourceEdit(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	createInitrdCacheInputs(t, dir, "v1")

	variant1 := initrdCacheVariant(defaultInitrdVersion, runtime.GOARCH)

	createInitrdCacheInputs(t, dir, "v2")

	variant2 := initrdCacheVariant(defaultInitrdVersion, runtime.GOARCH)
	if variant1 == variant2 {
		t.Fatal("source change did not change initrd cache variant")
	}
}

// TestInitrdCacheVariantHashesAllDiskSources guards the regression where the
// cache key hashed only 4 hand-picked files, so an edit to any other installer
// source (e.g. internal/install/disk/dhcp_linux.go) did not invalidate the
// cache and a stale initrd was served. The variant must change when any .go
// file under the installer source dirs changes.
func TestInitrdCacheVariantHashesAllDiskSources(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	createInitrdCacheInputs(t, dir, "v1")

	before := initrdCacheVariant(defaultInitrdVersion, runtime.GOARCH)

	// dhcp_linux.go was NOT in the old 4-file hash. Adding/editing it must move
	// the variant now that the whole disk package is hashed.
	extra := filepath.Join(dir, "internal", "install", "disk", "dhcp_linux.go")
	if err := os.WriteFile(extra, []byte("package disk // fix"), 0o644); err != nil {
		t.Fatal(err)
	}

	after := initrdCacheVariant(defaultInitrdVersion, runtime.GOARCH)
	if before == after {
		t.Fatal("editing internal/install/disk/dhcp_linux.go must change the initrd cache variant (was masked by the old 4-file hash)")
	}

	// _test.go files are build-irrelevant and must NOT move the variant.
	testFile := filepath.Join(dir, "internal", "install", "disk", "dhcp_linux_test.go")
	if err := os.WriteFile(testFile, []byte("package disk // test only"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := initrdCacheVariant(defaultInitrdVersion, runtime.GOARCH); got != after {
		t.Fatal("a _test.go edit must not change the initrd cache variant")
	}
}

func createInitrdCacheInputs(t *testing.T, root, version string) {
	t.Helper()
	for _, rel := range []string{
		"cmd/ze-installer/main.go",
		"internal/install/disk/initrd_linux.go",
		"internal/install/disk/bootstrap_linux.go",
		"internal/install/disk/rescue_linux.go",
	} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(version), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInitrdEnvURL(t *testing.T) {
	t.Chdir(t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	content := []byte("custom-url-initrd")
	sum := sha256.Sum256(content)
	checksumHex := hex.EncodeToString(sum[:])

	var requestedURL string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedURL = r.URL.Path
		switch {
		case strings.HasSuffix(r.URL.Path, "/initrd.img.gz"):
			w.Write(content) //nolint:errcheck // test fixture
		case strings.HasSuffix(r.URL.Path, "/initrd.img.gz.sha256"):
			fmt.Fprintf(w, "%s  initrd.img.gz\n", checksumHex) //nolint:errcheck // test fixture
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	setTestHTTP(t, srv.Client().Get)
	t.Setenv("ZE_APPLIANCE_INITRD_URL", srv.URL+"/custom-initrd")
	env.ResetCache()

	_, err := resolveInitrd()
	if err != nil {
		t.Fatalf("resolveInitrd: %v", err)
	}
	if !strings.Contains(requestedURL, "/custom-initrd/") {
		t.Errorf("expected custom base URL in request, got %q", requestedURL)
	}
}
