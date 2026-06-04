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

func kernelTestServer(t *testing.T, content []byte, checksumHex string) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/Image"):
			w.Write(content) //nolint:errcheck // test fixture
		case strings.HasSuffix(r.URL.Path, "/Image.sha256"):
			fmt.Fprintf(w, "%s  Image\n", checksumHex) //nolint:errcheck // test fixture
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func setTestHTTP(t *testing.T, fn func(string) (*http.Response, error)) {
	t.Helper()
	old := httpGetFn
	httpGetFn = fn
	t.Cleanup(func() { httpGetFn = old })
}

func setTestDockerCheck(t *testing.T, fn func() error) {
	t.Helper()
	old := kernelDockerCheckFn
	kernelDockerCheckFn = fn
	t.Cleanup(func() { kernelDockerCheckFn = old })
}

func setTestDockerBuild(t *testing.T, fn func(string, string, string) error) {
	t.Helper()
	old := kernelDockerBuildFn
	kernelDockerBuildFn = fn
	t.Cleanup(func() { kernelDockerBuildFn = old })
}

func TestRunDispatchesKernel(t *testing.T) {
	table := dispatchTable()
	if _, ok := table["kernel"]; !ok {
		t.Fatal("kernel not registered in dispatch table")
	}
}

func TestKernelResolvesCache(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	version := "7.0.11"
	arch := archAMD64
	cached := kernelCachePath(version, arch)
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached, []byte("fake-kernel"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveKernel(version, arch)
	if err != nil {
		t.Fatalf("resolveKernel: %v", err)
	}
	if got != cached {
		t.Errorf("resolveKernel = %q, want cached path %q", got, cached)
	}
}

func TestKernelDownloadsAndCaches(t *testing.T) {
	t.Chdir(t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	content := []byte("test-kernel-image-content")
	sum := sha256.Sum256(content)
	checksumHex := hex.EncodeToString(sum[:])

	srv := kernelTestServer(t, content, checksumHex)
	setTestHTTP(t, srv.Client().Get)
	t.Setenv("ZE_APPLIANCE_KERNEL_URL", srv.URL)
	env.ResetCache()

	got, err := resolveKernel("7.0.11", archAMD64)
	if err != nil {
		t.Fatalf("resolveKernel: %v", err)
	}
	if !strings.Contains(got, cacheDir) {
		t.Errorf("expected path under cache dir, got %q", got)
	}
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("read cached kernel: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Errorf("cached content mismatch")
	}
}

func TestKernelDownloadChecksumMismatch(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	content := []byte("test-kernel")
	wrongChecksum := strings.Repeat("ab", 32)

	srv := kernelTestServer(t, content, wrongChecksum)
	setTestHTTP(t, srv.Client().Get)
	t.Setenv("ZE_APPLIANCE_KERNEL_URL", srv.URL)

	setTestDockerCheck(t, func() error { return errors.New("no docker") })

	_, err := resolveKernel("7.0.11", archAMD64)
	if err == nil {
		t.Fatal("expected error from checksum mismatch + no docker fallback")
	}
}

func TestKernelFallsBackToDocker(t *testing.T) {
	t.Chdir(t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	setTestHTTP(t, func(url string) (*http.Response, error) { return nil, errors.New("network error") })
	setTestDockerCheck(t, func() error { return nil })
	setTestDockerBuild(t, func(version, arch, destPath string) error {
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destPath, []byte("docker-built-kernel"), 0o644)
	})

	got, err := resolveKernel("7.0.11", archAMD64)
	if err != nil {
		t.Fatalf("resolveKernel: %v", err)
	}
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "docker-built-kernel" {
		t.Errorf("content = %q, want docker-built-kernel", data)
	}
}

func TestKernelFailsWithoutDocker(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	setTestHTTP(t, func(url string) (*http.Response, error) { return nil, errors.New("network error") })
	setTestDockerCheck(t, func() error { return errors.New("docker not found") })

	_, err := resolveKernel("7.0.11", archAMD64)
	if err == nil {
		t.Fatal("expected error when both download and docker fail")
	}
	if !strings.Contains(err.Error(), "Docker") {
		t.Errorf("error should mention Docker, got: %v", err)
	}
}

func TestKernelArchFlag(t *testing.T) {
	code := runKernel([]string{"--arch", "x86"})
	if code != exitError {
		t.Errorf("runKernel(--arch x86) = %d, want %d", code, exitError)
	}
}

func TestKernelVersionFlag(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	version := "6.12.9"
	cached := kernelCachePath(version, archAMD64)
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached, []byte("kernel-6.12.9"), 0o644); err != nil {
		t.Fatal(err)
	}

	code := runKernel([]string{"--version", version, "--arch", archAMD64})
	if code != exitOK {
		t.Errorf("runKernel(--version 6.12.9) = %d, want %d", code, exitOK)
	}
}

func TestKernelCopiesToToolsPath(t *testing.T) {
	t.Chdir(t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	setTestHTTP(t, func(url string) (*http.Response, error) { return nil, errors.New("no network") })
	setTestDockerCheck(t, func() error { return nil })
	setTestDockerBuild(t, func(version, arch, destPath string) error {
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destPath, []byte("built-kernel"), 0o644)
	})

	got, err := resolveKernel("7.0.11", archAMD64)
	if err != nil {
		t.Fatalf("resolveKernel: %v", err)
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("cached kernel not at %q: %v", got, err)
	}
}

func TestKernelEnvURL(t *testing.T) {
	t.Chdir(t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	content := []byte("custom-url-kernel")
	sum := sha256.Sum256(content)
	checksumHex := hex.EncodeToString(sum[:])

	var requestedURL string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedURL = r.URL.Path
		switch {
		case strings.HasSuffix(r.URL.Path, "/Image"):
			w.Write(content) //nolint:errcheck // test fixture
		case strings.HasSuffix(r.URL.Path, "/Image.sha256"):
			fmt.Fprintf(w, "%s  Image\n", checksumHex) //nolint:errcheck // test fixture
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	setTestHTTP(t, srv.Client().Get)
	t.Setenv("ZE_APPLIANCE_KERNEL_URL", srv.URL+"/custom-base")
	env.ResetCache()

	_, err := resolveKernel("7.0.11", archAMD64)
	if err != nil {
		t.Fatalf("resolveKernel: %v", err)
	}
	if !strings.Contains(requestedURL, "/custom-base/") {
		t.Errorf("expected custom base URL in request, got %q", requestedURL)
	}
}
