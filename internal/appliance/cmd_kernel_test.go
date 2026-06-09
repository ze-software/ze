package appliance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
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

func setTestQEMUCheck(t *testing.T, fn func() error) {
	t.Helper()
	old := kernelQEMUCheckFn
	kernelQEMUCheckFn = fn
	t.Cleanup(func() { kernelQEMUCheckFn = old })
}

func setTestQEMUBuild(t *testing.T, fn func(string, string, string, string) error) {
	t.Helper()
	old := kernelQEMUBuildFn
	kernelQEMUBuildFn = fn
	t.Cleanup(func() { kernelQEMUBuildFn = old })
}

func TestRunDispatchesKernel(t *testing.T) {
	table := dispatchTable()
	if _, ok := table["kernel"]; !ok {
		t.Fatal("kernel not registered in dispatch table")
	}
}

func TestKernelResolvesCache(t *testing.T) {
	t.Chdir(t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	version := "7.0.11"
	arch := archAMD64
	cached := kernelCachePath(version, arch+"-"+ProfileQEMU)
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached, []byte("fake-kernel"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveKernel(version, arch, ProfileQEMU, "")
	if err != nil {
		t.Fatalf("resolveKernel: %v", err)
	}
	if got != cached {
		t.Errorf("resolveKernel = %q, want cached path %q", got, cached)
	}
}

func TestKernelCacheHitCopiesToToolsPath(t *testing.T) {
	t.Chdir(t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	version := "7.0.11"
	content := []byte("cached-hardware-kernel")
	cached := kernelCachePath(version, archAMD64+"-"+ProfileHardware)
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached, content, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := resolveKernel(version, archAMD64, ProfileHardware, ""); err != nil {
		t.Fatalf("resolveKernel: %v", err)
	}

	toolsPath := filepath.Join(kernelInstallerOutputDir, kernelFileName)
	got, err := os.ReadFile(toolsPath)
	if err != nil {
		t.Fatalf("tools path %s not written on cache hit: %v", toolsPath, err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("tools path content = %q, want %q", got, content)
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

	got, err := resolveKernel("7.0.11", archAMD64, ProfileQEMU, "")
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

	setTestQEMUCheck(t, func() error { return errors.New("no qemu") })

	_, err := resolveKernel("7.0.11", archAMD64, ProfileQEMU, "")
	if err == nil {
		t.Fatal("expected error from checksum mismatch + no QEMU fallback")
	}
}

func TestKernelFallsBackToQEMU(t *testing.T) {
	t.Chdir(t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	setTestHTTP(t, func(url string) (*http.Response, error) { return nil, errors.New("network error") })
	setTestDockerCheck(t, func() error { return errors.New("no docker") })
	setTestQEMUCheck(t, func() error { return nil })
	setTestQEMUBuild(t, func(version, arch, profile, destPath string) error {
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destPath, []byte("qemu-built-kernel"), 0o644)
	})

	got, err := resolveKernel("7.0.11", archAMD64, ProfileQEMU, "")
	if err != nil {
		t.Fatalf("resolveKernel: %v", err)
	}
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "qemu-built-kernel" {
		t.Errorf("content = %q, want qemu-built-kernel", data)
	}
}

func TestKernelFailsWithoutBuilders(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	setTestHTTP(t, func(url string) (*http.Response, error) { return nil, errors.New("network error") })
	setTestDockerCheck(t, func() error { return errors.New("docker not found") })
	setTestQEMUCheck(t, func() error { return errors.New("qemu not found") })

	_, err := resolveKernel("7.0.11", archAMD64, ProfileQEMU, "")
	if err == nil {
		t.Fatal("expected error when both download and builders fail")
	}
	if !strings.Contains(err.Error(), "no builder available") {
		t.Errorf("error should mention no builder available, got: %v", err)
	}
}

func TestKernelArchFlag(t *testing.T) {
	code := runKernel([]string{"--arch", "x86"})
	if code != exitError {
		t.Errorf("runKernel(--arch x86) = %d, want %d", code, exitError)
	}
}

func TestKernelProfileFlag(t *testing.T) {
	code := runKernel([]string{"--profile", "invalid"})
	if code != exitError {
		t.Errorf("runKernel(--profile invalid) = %d, want %d", code, exitError)
	}
}

func TestKernelVersionFlag(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	version := "6.12.9"
	cached := kernelCachePath(version, archAMD64+"-"+ProfileQEMU)
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
	setTestDockerCheck(t, func() error { return errors.New("no docker") })
	setTestQEMUCheck(t, func() error { return nil })
	setTestQEMUBuild(t, func(version, arch, profile, destPath string) error {
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destPath, []byte("built-kernel"), 0o644)
	})

	got, err := resolveKernel("7.0.11", archAMD64, ProfileQEMU, "")
	if err != nil {
		t.Fatalf("resolveKernel: %v", err)
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("cached kernel not at %q: %v", got, err)
	}
}

func TestKernelReadsArchFromAppliance(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	appDir := t.TempDir()
	oldBase := baseDir
	baseDir = appDir
	defer func() { baseDir = oldBase }()

	cfg := DefaultConfig("testapp")
	cfg.Image.Arch = archAMD64
	appPath := filepath.Join(appDir, "testapp")
	if err := os.MkdirAll(appPath, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgData, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appPath, "appliance.json"), cfgData, 0o644); err != nil {
		t.Fatal(err)
	}

	cached := kernelCachePath(defaultKernelVersion, archAMD64+"-"+ProfileQEMU)
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached, []byte("fake-amd64-kernel"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Pre-cache at the host arch too, so exit code alone can't prove the config was read.
	// If runKernel ignores the config and falls back to runtime.GOARCH, it would find this
	// cache entry and succeed -- but the output path would not contain "amd64".
	hostCached := kernelCachePath(defaultKernelVersion, runtime.GOARCH+"-"+ProfileQEMU)
	if hostCached != cached {
		if err := os.MkdirAll(filepath.Dir(hostCached), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(hostCached, []byte("fake-host-kernel"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := runKernel([]string{"testapp"})

	w.Close() //nolint:errcheck // test pipe
	os.Stdout = old

	if code != exitOK {
		t.Fatalf("runKernel(testapp) = %d, want %d", code, exitOK)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	want := defaultKernelVersion + "-" + archAMD64
	if !strings.Contains(output, want) {
		t.Fatalf("expected output to contain %q, got %q", want, output)
	}
}

func TestKernelReadsProfileFromAppliance(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	appDir := t.TempDir()
	oldBase := baseDir
	baseDir = appDir
	defer func() { baseDir = oldBase }()

	cfg := DefaultConfig("hwapp")
	cfg.Image.KernelProfile = ProfileHardware
	appPath := filepath.Join(appDir, "hwapp")
	if err := os.MkdirAll(appPath, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgData, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appPath, "appliance.json"), cfgData, 0o644); err != nil {
		t.Fatal(err)
	}

	cached := kernelCachePath(defaultKernelVersion, archAMD64+"-"+ProfileHardware)
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached, []byte("fake-hardware-kernel"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := runKernel([]string{"hwapp"})

	w.Close() //nolint:errcheck // test pipe
	os.Stdout = old

	if code != exitOK {
		t.Fatalf("runKernel(hwapp) = %d, want %d", code, exitOK)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "profile=hardware") {
		t.Errorf("expected output to mention profile=hardware, got %q", output)
	}
}

func TestKernelConfigHashInvalidatesCache(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	configDir := filepath.Join(dir, kernelInstallerConfigDir)
	builderDir := filepath.Join(dir, kernelBuilderDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(builderDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "kernel.config"), []byte("CONFIG_A=y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "qemu.config"), []byte("CONFIG_B=y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(builderDir, "build.sh"), []byte("#!/bin/sh\nmake\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	variant1 := kernelCacheVariant(archAMD64, ProfileQEMU)
	cached1 := kernelCachePath("7.0.11", variant1)
	if err := os.MkdirAll(filepath.Dir(cached1), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached1, []byte("kernel-v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveKernel("7.0.11", archAMD64, ProfileQEMU, "")
	if err != nil {
		t.Fatalf("resolveKernel before config change: %v", err)
	}
	if got != cached1 {
		t.Fatalf("expected cache hit at %q, got %q", cached1, got)
	}

	if err := os.WriteFile(filepath.Join(configDir, "kernel.config"), []byte("CONFIG_A=y\nCONFIG_IGC=y\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	variant2 := kernelCacheVariant(archAMD64, ProfileQEMU)
	if variant1 == variant2 {
		t.Fatal("config change did not change cache variant")
	}

	setTestDockerCheck(t, func() error { return errors.New("no docker") })
	setTestQEMUCheck(t, func() error { return nil })
	setTestQEMUBuild(t, func(version, arch, profile, destPath string) error {
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destPath, []byte("kernel-v2"), 0o644)
	})

	got2, err := resolveKernel("7.0.11", archAMD64, ProfileQEMU, "")
	if err != nil {
		t.Fatalf("resolveKernel after config change: %v", err)
	}
	if got2 == cached1 {
		t.Error("config change did not invalidate kernel cache")
	}

	data, err := os.ReadFile(got2)
	if err != nil {
		t.Fatalf("read rebuilt kernel: %v", err)
	}
	if string(data) != "kernel-v2" {
		t.Errorf("rebuilt kernel content = %q, want kernel-v2", data)
	}
}

func TestKernelBuildScriptInvalidatesCache(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	configDir := filepath.Join(dir, kernelInstallerConfigDir)
	builderDir := filepath.Join(dir, kernelBuilderDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(builderDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "kernel.config"), []byte("CONFIG_A=y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "qemu.config"), []byte("CONFIG_B=y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(builderDir, "build.sh"), []byte("#!/bin/sh\nmake\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	variant1 := kernelCacheVariant(archAMD64, ProfileQEMU)

	if err := os.WriteFile(filepath.Join(builderDir, "build.sh"), []byte("#!/bin/sh\nmake -j$(nproc)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	variant2 := kernelCacheVariant(archAMD64, ProfileQEMU)
	if variant1 == variant2 {
		t.Fatal("build.sh change did not change cache variant")
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

	_, err := resolveKernel("7.0.11", archAMD64, ProfileQEMU, "")
	if err != nil {
		t.Fatalf("resolveKernel: %v", err)
	}
	if !strings.Contains(requestedURL, "/custom-base/") {
		t.Errorf("expected custom base URL in request, got %q", requestedURL)
	}
}

func setTestDockerCheck(t *testing.T, fn func() error) {
	t.Helper()
	old := kernelDockerCheckFn
	kernelDockerCheckFn = fn
	t.Cleanup(func() { kernelDockerCheckFn = old })
}

func setTestDockerBuild(t *testing.T, fn func(string, string, string, string) error) {
	t.Helper()
	old := kernelDockerBuildFn
	kernelDockerBuildFn = fn
	t.Cleanup(func() { kernelDockerBuildFn = old })
}

func TestSelectBuilderExplicitDocker(t *testing.T) {
	setTestDockerCheck(t, func() error { return nil })
	setTestDockerBuild(t, func(_, _, _, _ string) error { return nil })

	fn, name, err := selectBuilder(builderDocker)
	if err != nil {
		t.Fatalf("selectBuilder(docker): %v", err)
	}
	if name != builderDocker {
		t.Errorf("name = %q, want %q", name, builderDocker)
	}
	if fn == nil {
		t.Fatal("build function is nil")
	}
}

func TestSelectBuilderExplicitDockerUnavailable(t *testing.T) {
	setTestDockerCheck(t, func() error { return errors.New("docker not found") })

	_, _, err := selectBuilder(builderDocker)
	if err == nil {
		t.Fatal("expected error when docker requested but unavailable")
	}
	if !strings.Contains(err.Error(), "docker builder requested but not available") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSelectBuilderExplicitQEMU(t *testing.T) {
	setTestQEMUCheck(t, func() error { return nil })
	setTestQEMUBuild(t, func(_, _, _, _ string) error { return nil })

	fn, name, err := selectBuilder(builderQEMU)
	if err != nil {
		t.Fatalf("selectBuilder(qemu): %v", err)
	}
	if name != builderQEMU {
		t.Errorf("name = %q, want %q", name, builderQEMU)
	}
	if fn == nil {
		t.Fatal("build function is nil")
	}
}

func TestSelectBuilderExplicitQEMUUnavailable(t *testing.T) {
	setTestQEMUCheck(t, func() error { return errors.New("qemu not found") })

	_, _, err := selectBuilder(builderQEMU)
	if err == nil {
		t.Fatal("expected error when qemu requested but unavailable")
	}
	if !strings.Contains(err.Error(), "qemu builder requested but not available") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSelectBuilderAutoDocker(t *testing.T) {
	setTestDockerCheck(t, func() error { return nil })
	setTestDockerBuild(t, func(_, _, _, _ string) error { return nil })
	setTestQEMUCheck(t, func() error { return nil })

	_, name, err := selectBuilder("")
	if err != nil {
		t.Fatalf("selectBuilder auto: %v", err)
	}
	if name != builderDocker {
		t.Errorf("auto-select should prefer docker, got %q", name)
	}
}

func TestSelectBuilderAutoFallsBackToQEMU(t *testing.T) {
	setTestDockerCheck(t, func() error { return errors.New("no docker") })
	setTestQEMUCheck(t, func() error { return nil })
	setTestQEMUBuild(t, func(_, _, _, _ string) error { return nil })

	_, name, err := selectBuilder("")
	if err != nil {
		t.Fatalf("selectBuilder auto fallback: %v", err)
	}
	if name != builderQEMU {
		t.Errorf("auto-select should fall back to qemu, got %q", name)
	}
}

func TestSelectBuilderAutoNoneAvailable(t *testing.T) {
	setTestDockerCheck(t, func() error { return errors.New("no docker") })
	setTestQEMUCheck(t, func() error { return errors.New("no qemu") })

	_, _, err := selectBuilder("")
	if err == nil {
		t.Fatal("expected error when no builder available")
	}
	if !strings.Contains(err.Error(), "no builder available") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestKernelBuilderFlag(t *testing.T) {
	code := runKernel([]string{"--builder", "invalid"})
	if code != exitError {
		t.Errorf("runKernel(--builder invalid) = %d, want %d", code, exitError)
	}
}

func TestKernelFallsBackToDocker(t *testing.T) {
	t.Chdir(t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	setTestHTTP(t, func(url string) (*http.Response, error) { return nil, errors.New("network error") })
	setTestDockerCheck(t, func() error { return nil })
	setTestDockerBuild(t, func(version, arch, profile, destPath string) error {
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destPath, []byte("docker-built-kernel"), 0o644)
	})
	setTestQEMUCheck(t, func() error { return errors.New("no qemu") })

	got, err := resolveKernel("7.0.11", archAMD64, ProfileQEMU, "")
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

func TestDockerBuildMountsSharedBuilder(t *testing.T) {
	// VALIDATES: AC-15 expects Docker build/run to use the shared builder with
	// target platform and separate builder/config/output mounts.
	// PREVENTS: silently continuing to run tools/installer-kernel/build.sh.
	dir := t.TempDir()
	t.Chdir(dir)

	builderDir := filepath.Join(dir, kernelBuilderDir)
	configDir := filepath.Join(dir, kernelInstallerConfigDir)
	outputDir := filepath.Join(dir, kernelInstallerOutputDir)
	fakeBin := filepath.Join(dir, "fakebin")
	if err := os.MkdirAll(builderDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"Dockerfile":    "FROM scratch\n",
		"build.sh":      "#!/bin/sh\n",
		"kernel.config": "CONFIG_A=y\n",
		"qemu.config":   "CONFIG_B=y\n",
	} {
		base := builderDir
		if strings.HasSuffix(name, ".config") {
			base = configDir
		}
		if err := os.WriteFile(filepath.Join(base, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	logPath := filepath.Join(dir, "docker.log")
	dockerPath := filepath.Join(fakeBin, "docker")
	dockerScript := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$ZE_DOCKER_LOG"
if [ "$1" = run ]; then
	prev=
	for arg in "$@"; do
		if [ "$prev" = "-v" ]; then
			case "$arg" in
				*:/out)
					out=${arg%:/out}
					mkdir -p "$out"
					printf kernel > "$out/Image"
					;;
			esac
		fi
		prev=$arg
	done
fi
`
	if err := os.WriteFile(dockerPath, []byte(dockerScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ZE_DOCKER_LOG", logPath)

	destPath := filepath.Join(dir, "cache", kernelFileName)
	if err := defaultDockerBuild("7.0.11", archAMD64, ProfileQEMU, destPath); err != nil {
		t.Fatalf("defaultDockerBuild: %v", err)
	}
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("cache image not written: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read docker log: %v", err)
	}
	log := string(logData)
	for _, want := range []string{
		"build --platform linux/amd64 -t " + kernelDockerImage + " " + builderDir,
		"run --rm --platform linux/amd64",
		builderDir + ":/builder:ro",
		configDir + ":/src:ro",
		outputDir + ":/out",
		"sh /builder/build.sh",
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("docker log missing %q\nlog:\n%s", want, log)
		}
	}
}

func TestQEMUBuildPassesBuilderDir(t *testing.T) {
	// VALIDATES: AC-15 expects installer QEMU builds to call shared qemu-build.py
	// with explicit builder, source, and output dirs, but no runtime modules flags.
	// PREVENTS: qemu-build.py hardcoding tools/installer-kernel paths.
	dir := t.TempDir()
	t.Chdir(dir)

	builderDir := filepath.Join(dir, kernelBuilderDir)
	outputDir := filepath.Join(dir, kernelInstallerOutputDir)
	fakeBin := filepath.Join(dir, "fakebin")
	if err := os.MkdirAll(builderDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(builderDir, "qemu-build.py"), []byte("#!/usr/bin/env python3\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	logPath := filepath.Join(dir, "python.log")
	pythonPath := filepath.Join(fakeBin, "python3")
	pythonScript := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$ZE_PYTHON_LOG"
out=
prev=
for arg in "$@"; do
	if [ "$prev" = "--out-dir" ]; then
		out=$arg
	fi
	prev=$arg
done
if [ -n "$out" ]; then
	mkdir -p "$out"
	printf kernel > "$out/Image"
fi
`
	if err := os.WriteFile(pythonPath, []byte(pythonScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ZE_PYTHON_LOG", logPath)

	destPath := filepath.Join(dir, "cache", kernelFileName)
	if err := defaultQEMUBuild("7.0.11", archARM64, ProfileHardware, destPath); err != nil {
		t.Fatalf("defaultQEMUBuild: %v", err)
	}
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("cache image not written: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read python log: %v", err)
	}
	log := string(logData)
	for _, want := range []string{
		filepath.Join(builderDir, "qemu-build.py"),
		"--builder-dir " + kernelBuilderDir,
		"--src-dir " + kernelInstallerConfigDir,
		"--out-dir " + kernelInstallerOutputDir,
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("python log missing %q\nlog:\n%s", want, log)
		}
	}
	for _, forbidden := range []string{"--modules", "--patches-dir"} {
		if strings.Contains(log, forbidden) {
			t.Fatalf("python log unexpectedly contains %q\nlog:\n%s", forbidden, log)
		}
	}
	_ = outputDir
}
