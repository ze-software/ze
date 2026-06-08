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
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
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

	cached := initrdCachePath(defaultInitrdVersion)
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

// VALIDATES: cache-hit branch copies initrd to tools/installer-initrd/build/ for downstream ISO.
// PREVENTS: appliance iso failing with "installer initrd not found" after a cache-hit resolve.
func TestInitrdCacheHitCopiesToToolsPath(t *testing.T) {
	t.Chdir(t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	content := []byte("cached-initrd")
	cached := initrdCachePath(defaultInitrdVersion)
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached, content, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := resolveInitrd(); err != nil {
		t.Fatalf("resolveInitrd: %v", err)
	}

	toolsPath := filepath.Join(initrdToolsDir, "build", initrdFileName)
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

func TestInitrdFailsWithoutBuildTools(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	setTestHTTP(t, func(url string) (*http.Response, error) { return nil, errors.New("network error") })

	oldLookPath := initrdLookPathFn
	initrdLookPathFn = func(name string) (string, error) { return "", errors.New("not found") }
	t.Cleanup(func() { initrdLookPathFn = oldLookPath })

	_, err := resolveInitrd()
	if err == nil {
		t.Fatal("expected error when download fails and build tools missing")
	}
	if !strings.Contains(err.Error(), "missing build tools") {
		t.Errorf("error should mention missing build tools, got: %v", err)
	}
}

func TestInitrdInitScriptInvalidatesCache(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	toolsDir := filepath.Join(dir, initrdToolsDir)
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, "init"), []byte("#!/bin/sh\necho boot\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, "Makefile"), []byte("all:\n\tcpio\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cached1 := initrdCachePath(defaultInitrdVersion)
	if err := os.MkdirAll(filepath.Dir(cached1), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached1, []byte("initrd-v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveInitrd()
	if err != nil {
		t.Fatalf("resolveInitrd before init change: %v", err)
	}
	if got != cached1 {
		t.Fatalf("expected cache hit at %q, got %q", cached1, got)
	}

	if err := os.WriteFile(filepath.Join(toolsDir, "init"), []byte("#!/bin/sh\necho boot\nretry_iso_scan\n"), 0o644); err != nil {
		t.Fatal(err)
	}

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
		t.Fatalf("resolveInitrd after init change: %v", err)
	}
	if got2 == cached1 {
		t.Error("init script change did not invalidate initrd cache")
	}

	data, err := os.ReadFile(got2)
	if err != nil {
		t.Fatalf("read rebuilt initrd: %v", err)
	}
	if string(data) != "initrd-v2" {
		t.Errorf("rebuilt initrd content = %q, want initrd-v2", data)
	}
}

func TestInitrdMakefileInvalidatesCache(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	toolsDir := filepath.Join(dir, initrdToolsDir)
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, "init"), []byte("#!/bin/sh\necho boot\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, "Makefile"), []byte("all:\n\tcpio\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	variant1 := initrdCacheVariant(defaultInitrdVersion)

	if err := os.WriteFile(filepath.Join(toolsDir, "Makefile"), []byte("all:\n\tcpio --quiet\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	variant2 := initrdCacheVariant(defaultInitrdVersion)
	if variant1 == variant2 {
		t.Fatal("Makefile change did not change initrd cache variant")
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
