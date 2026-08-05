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

	"github.com/ze-software/ze/internal/core/env"
)

// test-relax: kernel-build-consolidation collapsed Go-side builder selection
// (selectBuilder, defaultDockerBuild, defaultQEMUBuild, dockerPlatform, the
// docker/qemu *CheckFn/*BuildFn seams) into the single tools/kernel-builder/run.py
// driver. The tests that exercised that removed Go logic (TestSelectBuilder*,
// TestKernelFallsBackToDocker/QEMU, TestKernelFailsWithoutBuilders,
// TestDockerBuildMountsSharedBuilder, TestQEMUBuildPassesBuilderDir) are gone;
// builder selection is now covered by run.py and the appliance-kernel-*.ci
// functional tests, and the Go->run.py argv contract by TestRunBuilderArgvDocker
// and TestRunBuilderArgvRuntime below.

// fakeInstallerConfig covers every symbol in the test installer registry
// manifests plus the universal floor, so a fake build's emitted config passes
// enforcement for any installer profile.
const fakeInstallerConfig = "CONFIG_IP_PNP_DHCP=y\nCONFIG_EXT4_FS=y\nCONFIG_BLK_DEV_INITRD=y\nCONFIG_DEVTMPFS_MOUNT=y\n" +
	"CONFIG_VIRTIO_NET=y\nCONFIG_VIRTIO_BLK=y\n" +
	"CONFIG_EFI=y\nCONFIG_EFI_STUB=y\nCONFIG_FB_EFI=y\nCONFIG_FRAMEBUFFER_CONSOLE=y\n" +
	"CONFIG_E1000E=y\nCONFIG_IGB=y\nCONFIG_IGC=y\nCONFIG_R8169=y\nCONFIG_SATA_AHCI=y\n" +
	"CONFIG_BLK_DEV_NVME=y\nCONFIG_BLK_DEV_LOOP=y\nCONFIG_VFAT_FS=y\nCONFIG_EXFAT_FS=y\n"

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

// setTestKernelBuild substitutes the shared-driver invocation so unit tests
// exercise resolveKernel without docker or qemu.
func setTestKernelBuild(t *testing.T, fn func(kernelBuildSpec) error) {
	t.Helper()
	old := kernelBuildFn
	kernelBuildFn = fn
	t.Cleanup(func() { kernelBuildFn = old })
}

// fakeInstallerBuild writes an installer Image + config that passes enforcement.
func fakeInstallerBuild(content string) func(kernelBuildSpec) error {
	return func(spec kernelBuildSpec) error {
		if err := os.MkdirAll(spec.outDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(spec.outDir, "Image"), []byte(content), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(spec.outDir, "config"), []byte(fakeInstallerConfig), 0o644)
	}
}

func writeInstallerKernelRegistry(t *testing.T) {
	t.Helper()
	configDir := kernelInstallerConfigDir
	builderDir := kernelBuilderDir
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(builderDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(configDir, "kernel.config"):    "CONFIG_IP_PNP_DHCP=y\nCONFIG_EXT4_FS=y\nCONFIG_BLK_DEV_INITRD=y\nCONFIG_DEVTMPFS_MOUNT=y\n",
		filepath.Join(configDir, "kernel.require"):   "CONFIG_IP_PNP_DHCP\nCONFIG_EXT4_FS\nCONFIG_BLK_DEV_INITRD\nCONFIG_DEVTMPFS_MOUNT\n",
		filepath.Join(configDir, "qemu.config"):      "CONFIG_VIRTIO_NET=y\nCONFIG_VIRTIO_BLK=y\n",
		filepath.Join(configDir, "qemu.require"):     "CONFIG_VIRTIO_NET\nCONFIG_VIRTIO_BLK\n",
		filepath.Join(configDir, "hardware.config"):  "CONFIG_EFI=y\nCONFIG_EFI_STUB=y\nCONFIG_FB_EFI=y\nCONFIG_FRAMEBUFFER_CONSOLE=y\nCONFIG_E1000E=y\nCONFIG_IGB=y\nCONFIG_IGC=y\nCONFIG_R8169=y\nCONFIG_SATA_AHCI=y\nCONFIG_BLK_DEV_NVME=y\nCONFIG_BLK_DEV_LOOP=y\nCONFIG_VFAT_FS=y\nCONFIG_EXFAT_FS=y\n",
		filepath.Join(configDir, "hardware.require"): "CONFIG_EFI\nCONFIG_EFI_STUB\nCONFIG_FB_EFI\nCONFIG_FRAMEBUFFER_CONSOLE\nCONFIG_E1000E\nCONFIG_IGB\nCONFIG_IGC\nCONFIG_R8169\nCONFIG_SATA_AHCI\nCONFIG_BLK_DEV_NVME\nCONFIG_BLK_DEV_LOOP\nCONFIG_VFAT_FS\nCONFIG_EXFAT_FS\n",
		filepath.Join(builderDir, "build.py"):        "#!/usr/bin/env python3\n",
		filepath.Join(builderDir, "run.py"):          "#!/usr/bin/env python3\n",
		filepath.Join(builderDir, "ksource.py"):      "#!/usr/bin/env python3\n",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// writeRuntimeKernelRegistry lays down a minimal runtime registry under
// gokrazy/kernel so resolveKernel(runtime) resolves and enforces.
func writeRuntimeKernelRegistry(t *testing.T) {
	t.Helper()
	dir := runtimeKernelConfigDir
	if err := os.MkdirAll(filepath.Join(dir, "patches"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(kernelBuilderDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(dir, "kernel.config"):         "CONFIG_IP_PNP_DHCP=y\nCONFIG_EXT4_FS=y\nCONFIG_BLK_DEV_INITRD=y\nCONFIG_DEVTMPFS_MOUNT=y\n",
		filepath.Join(dir, "kernel.require"):        "CONFIG_IP_PNP_DHCP\nCONFIG_EXT4_FS\nCONFIG_BLK_DEV_INITRD\nCONFIG_DEVTMPFS_MOUNT\n",
		filepath.Join(dir, "runtime.config"):        "CONFIG_MODULES=y\nCONFIG_PPP=y\nCONFIG_PPPOE=y\nCONFIG_L2TP=y\nCONFIG_PPPOL2TP=y\nCONFIG_L2TP_V3=y\nCONFIG_VETH=y\nCONFIG_INET_ESP=y\nCONFIG_INET6_ESP=y\nCONFIG_XFRM_STATISTICS=y\n",
		filepath.Join(dir, "runtime.require"):       "CONFIG_MODULES\nCONFIG_VETH\n",
		filepath.Join(kernelBuilderDir, "build.py"): "#!/usr/bin/env python3\n",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

const fakeRuntimeConfig = "CONFIG_IP_PNP_DHCP=y\nCONFIG_EXT4_FS=y\nCONFIG_BLK_DEV_INITRD=y\nCONFIG_DEVTMPFS_MOUNT=y\n" +
	"CONFIG_MODULES=y\nCONFIG_PPP=y\nCONFIG_PPPOE=y\nCONFIG_L2TP=y\nCONFIG_PPPOL2TP=y\nCONFIG_L2TP_V3=y\nCONFIG_VETH=y\n" +
	// The IPsec dataplane floor (runtimeKernelRequirements, kernelreq.go). The real
	// fragment sets these in gokrazy/kernel/runtime.config.
	"CONFIG_INET_ESP=y\nCONFIG_INET6_ESP=y\nCONFIG_XFRM_STATISTICS=y\n"

func fakeRuntimeBuild(spec kernelBuildSpec) error {
	if err := os.MkdirAll(filepath.Join(spec.outDir, "lib", "modules", "7.1.1-ze"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(spec.outDir, "vmlinuz"), []byte("runtime-vmlinuz"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(spec.outDir, "lib", "modules", "7.1.1-ze", "modules.dep"), []byte(""), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(spec.outDir, "config"), []byte(fakeRuntimeConfig), 0o644)
}

func TestRunDispatchesKernel(t *testing.T) {
	table := dispatchTable()
	if _, ok := table["kernel"]; !ok {
		t.Fatal("kernel not registered in dispatch table")
	}
}

func TestKernelResolvesCache(t *testing.T) {
	t.Chdir(t.TempDir())
	writeInstallerKernelRegistry(t)
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	version := "7.1.1"
	arch := archAMD64
	cached := kernelCachePath(version, kernelCacheVariant(arch, defaultKernelProfile))
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached, []byte("fake-kernel"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveKernel(version, arch, defaultKernelProfile, "", kernelTargetInstaller)
	if err != nil {
		t.Fatalf("resolveKernel: %v", err)
	}
	if got != cached {
		t.Errorf("resolveKernel = %q, want cached path %q", got, cached)
	}
}

func TestKernelCacheHitCopiesToToolsPath(t *testing.T) {
	t.Chdir(t.TempDir())
	writeInstallerKernelRegistry(t)
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	version := "7.1.1"
	content := []byte("cached-hardware-kernel")
	cached := kernelCachePath(version, kernelCacheVariant(archAMD64, "hardware"))
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached, content, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := resolveKernel(version, archAMD64, "hardware", "", kernelTargetInstaller); err != nil {
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
	writeInstallerKernelRegistry(t)
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	content := []byte("test-kernel-image-content")
	sum := sha256.Sum256(content)
	checksumHex := hex.EncodeToString(sum[:])

	srv := kernelTestServer(t, content, checksumHex)
	setTestHTTP(t, srv.Client().Get)
	t.Setenv("ZE_APPLIANCE_KERNEL_URL", srv.URL)
	env.ResetCache()

	got, err := resolveKernel("7.1.1", archAMD64, defaultKernelProfile, "", kernelTargetInstaller)
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
	t.Chdir(t.TempDir())
	writeInstallerKernelRegistry(t)
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	content := []byte("test-kernel")
	wrongChecksum := strings.Repeat("ab", 32)

	srv := kernelTestServer(t, content, wrongChecksum)
	setTestHTTP(t, srv.Client().Get)
	t.Setenv("ZE_APPLIANCE_KERNEL_URL", srv.URL)

	setTestKernelBuild(t, func(kernelBuildSpec) error { return errors.New("no builder available") })

	_, err := resolveKernel("7.1.1", archAMD64, defaultKernelProfile, "", kernelTargetInstaller)
	if err == nil {
		t.Fatal("expected error from checksum mismatch + failed build fallback")
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

func TestKernelBuilderFlag(t *testing.T) {
	code := runKernel([]string{"--builder", "invalid"})
	if code != exitError {
		t.Errorf("runKernel(--builder invalid) = %d, want %d", code, exitError)
	}
}

func TestKernelTargetFlag(t *testing.T) {
	code := runKernel([]string{"--target", "bogus"})
	if code != exitError {
		t.Errorf("runKernel(--target bogus) = %d, want %d", code, exitError)
	}
}

func TestKernelVersionFlag(t *testing.T) {
	t.Chdir(t.TempDir())
	writeInstallerKernelRegistry(t)
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	version := "7.0.5"
	cached := kernelCachePath(version, kernelCacheVariant(archAMD64, defaultKernelProfile))
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached, []byte("kernel-7.0.5"), 0o644); err != nil {
		t.Fatal(err)
	}

	code := runKernel([]string{"--version", version, "--arch", archAMD64})
	if code != exitOK {
		t.Errorf("runKernel(--version 7.0.5) = %d, want %d", code, exitOK)
	}
}

func TestKernelCopiesToToolsPath(t *testing.T) {
	t.Chdir(t.TempDir())
	writeInstallerKernelRegistry(t)
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	setTestHTTP(t, func(url string) (*http.Response, error) { return nil, errors.New("no network") })
	setTestKernelBuild(t, fakeInstallerBuild("built-kernel"))

	got, err := resolveKernel("7.1.1", archAMD64, defaultKernelProfile, "", kernelTargetInstaller)
	if err != nil {
		t.Fatalf("resolveKernel: %v", err)
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("cached kernel not at %q: %v", got, err)
	}
	toolsPath := filepath.Join(kernelInstallerOutputDir, kernelFileName)
	if _, err := os.Stat(toolsPath); err != nil {
		t.Errorf("tools path %q not written: %v", toolsPath, err)
	}
}

func TestKernelReadsArchFromAppliance(t *testing.T) {
	t.Chdir(t.TempDir())
	writeInstallerKernelRegistry(t)
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

	cached := kernelCachePath(defaultKernelVersion, kernelCacheVariant(archAMD64, defaultKernelProfile))
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached, []byte("fake-amd64-kernel"), 0o644); err != nil {
		t.Fatal(err)
	}

	hostCached := kernelCachePath(defaultKernelVersion, kernelCacheVariant(runtime.GOARCH, defaultKernelProfile))
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

	// The cache variant embeds the target then the arch; amd64 (from the
	// appliance config) must win over the host arch.
	want := kernelTargetInstaller + "-" + archAMD64
	if !strings.Contains(output, want) {
		t.Fatalf("expected output to contain %q, got %q", want, output)
	}
}

func TestKernelReadsProfileFromAppliance(t *testing.T) {
	t.Chdir(t.TempDir())
	writeInstallerKernelRegistry(t)
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	appDir := t.TempDir()
	oldBase := baseDir
	baseDir = appDir
	defer func() { baseDir = oldBase }()

	cfg := DefaultConfig("hwapp")
	cfg.Image.KernelProfile = "hardware"
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

	cached := kernelCachePath(defaultKernelVersion, kernelCacheVariant(archAMD64, "hardware"))
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
	if !strings.Contains(output, "target=installer") {
		t.Errorf("expected output to mention target=installer, got %q", output)
	}
}

func TestKernelConfigHashInvalidatesCache(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeInstallerKernelRegistry(t)
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	configDir := filepath.Join(dir, kernelInstallerConfigDir)
	if err := os.WriteFile(filepath.Join(configDir, "kernel.config"), []byte("CONFIG_IP_PNP_DHCP=y\nCONFIG_EXT4_FS=y\nCONFIG_BLK_DEV_INITRD=y\nCONFIG_DEVTMPFS_MOUNT=y\nCONFIG_IGC=y\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	variant1 := kernelCacheVariant(archAMD64, defaultKernelProfile)
	cached1 := kernelCachePath("7.1.1", variant1)
	if err := os.MkdirAll(filepath.Dir(cached1), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached1, []byte("kernel-v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveKernel("7.1.1", archAMD64, defaultKernelProfile, "", kernelTargetInstaller)
	if err != nil {
		t.Fatalf("resolveKernel before config change: %v", err)
	}
	if got != cached1 {
		t.Fatalf("expected cache hit at %q, got %q", cached1, got)
	}

	if err := os.WriteFile(filepath.Join(configDir, "kernel.config"), []byte("CONFIG_IP_PNP_DHCP=y\nCONFIG_EXT4_FS=y\nCONFIG_BLK_DEV_INITRD=y\nCONFIG_DEVTMPFS_MOUNT=y\nCONFIG_IGC=y\nCONFIG_ICE=y\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	variant2 := kernelCacheVariant(archAMD64, defaultKernelProfile)
	if variant1 == variant2 {
		t.Fatal("config change did not change cache variant")
	}

	setTestHTTP(t, func(url string) (*http.Response, error) { return nil, errors.New("no network") })
	setTestKernelBuild(t, fakeInstallerBuild("kernel-v2"))

	got2, err := resolveKernel("7.1.1", archAMD64, defaultKernelProfile, "", kernelTargetInstaller)
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

func TestKernelBuildPyInvalidatesCache(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	writeInstallerKernelRegistry(t)

	builderDir := filepath.Join(dir, kernelBuilderDir)
	if err := os.WriteFile(filepath.Join(builderDir, "build.py"), []byte("#!/usr/bin/env python3\nprint('v1')\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	variant1 := kernelCacheVariant(archAMD64, defaultKernelProfile)

	if err := os.WriteFile(filepath.Join(builderDir, "build.py"), []byte("#!/usr/bin/env python3\nprint('v2')\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	variant2 := kernelCacheVariant(archAMD64, defaultKernelProfile)
	if variant1 == variant2 {
		t.Fatal("build.py change did not change cache variant")
	}
}

func TestKernelEnvURL(t *testing.T) {
	t.Chdir(t.TempDir())
	writeInstallerKernelRegistry(t)
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

	_, err := resolveKernel("7.1.1", archAMD64, defaultKernelProfile, "", kernelTargetInstaller)
	if err != nil {
		t.Fatalf("resolveKernel: %v", err)
	}
	if !strings.Contains(requestedURL, "/custom-base/") {
		t.Errorf("expected custom base URL in request, got %q", requestedURL)
	}
}

func TestTargetSelectsRegistryDir(t *testing.T) {
	// VALIDATES: --target runtime resolves gokrazy/kernel, default installer resolves tools/installer-kernel.
	// PREVENTS: the runtime verified path silently building from the installer registry.
	installer, err := kernelTargetFor(kernelTargetInstaller)
	if err != nil {
		t.Fatal(err)
	}
	if installer.configDir != kernelInstallerConfigDir || installer.isTree || installer.modules != "no" {
		t.Errorf("installer target = %+v", installer)
	}
	rt, err := kernelTargetFor(kernelTargetRuntime)
	if err != nil {
		t.Fatal(err)
	}
	if rt.configDir != runtimeKernelConfigDir || !rt.isTree || rt.modules != "yes" || rt.defaultProfile != runtimeKernelProfile {
		t.Errorf("runtime target = %+v", rt)
	}
	if _, err := kernelTargetFor("bogus"); err == nil {
		t.Error("kernelTargetFor(bogus) should error")
	}
}

func TestCacheVariantIncludesTarget(t *testing.T) {
	// VALIDATES: AC-6/R-6 expect installer and runtime artifacts to get distinct cache variants.
	// PREVENTS: a runtime tree overwriting an installer Image (or vice versa) in the cache.
	resolved := kernelProfileResolution{Name: "qemu"}
	installer := kernelCacheVariantFor(kernelTargetInstaller, archAMD64, resolved)
	rt := kernelCacheVariantFor(kernelTargetRuntime, archAMD64, resolved)
	if installer == rt {
		t.Fatalf("installer and runtime variants collide: %q", installer)
	}
	if !strings.HasPrefix(installer, kernelTargetInstaller+"-") {
		t.Errorf("installer variant = %q, want %s- prefix", installer, kernelTargetInstaller)
	}
	if !strings.HasPrefix(rt, kernelTargetRuntime+"-") {
		t.Errorf("runtime variant = %q, want %s- prefix", rt, kernelTargetRuntime)
	}
}

func TestVersionValidatedAtEmbed(t *testing.T) {
	// VALIDATES: AC-16 expects the embedded kernel.version to be format-validated in the command path.
	// PREVENTS: a malformed or pre-7 kernel.version reaching a download or build.
	if err := validateKernelVersionString(defaultKernelVersion); err != nil {
		t.Fatalf("embedded kernel.version %q is invalid: %v", defaultKernelVersion, err)
	}
	// Note: "7..1" (empty middle component) is accepted by build.py's
	// validate_version too, so the two readers stay consistent by accepting it.
	for _, bad := range []string{"", "7.x", "6.12.9", ".1.1", "abc"} {
		if err := validateKernelVersionString(bad); err == nil {
			t.Errorf("validateKernelVersionString(%q) = nil, want error", bad)
		}
	}
	// The command path rejects a malformed --version before any build.
	if code := runKernel([]string{"--version", "6.0.0"}); code != exitError {
		t.Errorf("runKernel(--version 6.0.0) = %d, want %d", code, exitError)
	}
}

func TestRunBuilderArgvDocker(t *testing.T) {
	// VALIDATES: AC-1 expects Go to invoke the shared run.py driver (no inline docker/qemu argv).
	// PREVENTS: silently reintroducing inline docker build/run or the arch->platform map in Go.
	dir := t.TempDir()
	t.Chdir(dir)
	writeInstallerKernelRegistry(t)

	fakeBin := filepath.Join(dir, "fakebin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "python.log")
	pythonScript := "#!/bin/sh\nprintf '%s ' \"$@\" >> \"$ZE_PYTHON_LOG\"\nprintf '\\n' >> \"$ZE_PYTHON_LOG\"\n"
	if err := os.WriteFile(filepath.Join(fakeBin, "python3"), []byte(pythonScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ZE_PYTHON_LOG", logPath)

	err := defaultKernelBuild(kernelBuildSpec{
		version: "7.1.1", arch: archAMD64, profile: defaultKernelProfile, builder: builderDocker,
		target: kernelTargetInstaller, srcDir: kernelInstallerConfigDir, outDir: kernelInstallerOutputDir,
		modules: "no",
	})
	if err != nil {
		t.Fatalf("defaultKernelBuild: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read python log: %v", err)
	}
	log := string(logData)
	for _, want := range []string{
		filepath.Join(kernelBuilderDir, runPyName),
		"--target installer",
		"--arch amd64",
		"--profile qemu",
		"--src-dir tools/installer-kernel",
		"--out-dir build/kernel",
		"--builder-dir tools/kernel-builder",
		"--common-dir tools/kernel-builder/common",
		"--modules no",
		"--version 7.1.1",
		"--builder docker",
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("run.py argv missing %q\nlog:\n%s", want, log)
		}
	}
	for _, forbidden := range []string{"docker run", "docker build", "--platform", "run --rm", "build.sh"} {
		if strings.Contains(log, forbidden) {
			t.Fatalf("run.py argv unexpectedly contains inline-builder token %q\nlog:\n%s", forbidden, log)
		}
	}
}

func TestRunBuilderArgvRuntime(t *testing.T) {
	// VALIDATES: AC-6 expects the runtime target to invoke run.py with --target runtime,
	// --modules yes, and the gokrazy patches dir.
	dir := t.TempDir()
	t.Chdir(dir)
	writeRuntimeKernelRegistry(t)

	fakeBin := filepath.Join(dir, "fakebin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "python.log")
	pythonScript := "#!/bin/sh\nprintf '%s ' \"$@\" >> \"$ZE_PYTHON_LOG\"\nprintf '\\n' >> \"$ZE_PYTHON_LOG\"\n"
	if err := os.WriteFile(filepath.Join(fakeBin, "python3"), []byte(pythonScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ZE_PYTHON_LOG", logPath)

	err := defaultKernelBuild(kernelBuildSpec{
		version: "7.1.1", arch: archAMD64, profile: runtimeKernelProfile, builder: builderQEMU,
		target: kernelTargetRuntime, srcDir: runtimeKernelConfigDir, outDir: runtimeKernelOutputDir,
		modules: "yes", patches: runtimeKernelPatchesDir,
	})
	if err != nil {
		t.Fatalf("defaultKernelBuild runtime: %v", err)
	}
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read python log: %v", err)
	}
	log := string(logData)
	for _, want := range []string{
		"--target runtime",
		"--modules yes",
		"--src-dir gokrazy/kernel",
		"--out-dir tmp/kernel/build",
		"--patches-dir gokrazy/kernel/patches",
		"--builder qemu",
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("runtime run.py argv missing %q\nlog:\n%s", want, log)
		}
	}
}

func TestRuntimeKernelBuildsTree(t *testing.T) {
	// VALIDATES: AC-6 expects --target runtime to enforce the runtime floor and cache the
	// vmlinuz tree under a target=runtime cache dir, distinct from the installer Image cache.
	dir := t.TempDir()
	t.Chdir(dir)
	writeRuntimeKernelRegistry(t)
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	setTestKernelBuild(t, fakeRuntimeBuild)

	got, err := resolveKernel("7.1.1", archAMD64, runtimeKernelProfile, "", kernelTargetRuntime)
	if err != nil {
		t.Fatalf("resolveKernel runtime: %v", err)
	}
	info, err := os.Stat(got)
	if err != nil || !info.IsDir() {
		t.Fatalf("runtime artifact %q is not a directory: %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(got, "vmlinuz")); err != nil {
		t.Errorf("cached runtime tree missing vmlinuz: %v", err)
	}
	if _, err := os.Stat(filepath.Join(got, "lib", "modules")); err != nil {
		t.Errorf("cached runtime tree missing lib/modules: %v", err)
	}
	if !strings.Contains(got, runtimeKernelCacheDir) {
		t.Errorf("runtime cache path %q not under %s", got, runtimeKernelCacheDir)
	}
}

func TestRuntimeTargetIgnoresApplianceProfile(t *testing.T) {
	// VALIDATES: ze appliance kernel --target runtime <name> must NOT inherit the
	// appliance's installer KernelProfile (which defaults to "qemu"); the runtime
	// kernel is a single global "runtime" profile. Without the target gate this
	// resolves gokrazy/kernel/qemu.config (absent) and fails confusingly.
	t.Chdir(t.TempDir())
	writeRuntimeKernelRegistry(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	appDir := t.TempDir()
	oldBase := baseDir
	baseDir = appDir
	defer func() { baseDir = oldBase }()

	cfg := DefaultConfig("rtapp") // DefaultConfig sets KernelProfile = "qemu"
	cfg.Image.Arch = archAMD64
	appPath := filepath.Join(appDir, "rtapp")
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

	setTestKernelBuild(t, fakeRuntimeBuild)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := runKernel([]string{"--target", "runtime", "rtapp"})
	w.Close() //nolint:errcheck // test pipe
	os.Stdout = old

	if code != exitOK {
		t.Fatalf("runKernel(--target runtime rtapp) = %d, want %d (runtime must ignore the appliance qemu profile)", code, exitOK)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])
	if !strings.Contains(output, "profile=runtime") {
		t.Errorf("expected profile=runtime, got %q", output)
	}
	if !strings.Contains(output, "target=runtime") {
		t.Errorf("expected target=runtime, got %q", output)
	}
}

func TestRuntimeKernelFloorRejectsMissing(t *testing.T) {
	// VALIDATES: AC-8 expects a runtime build missing a floor symbol to fail enforcement.
	dir := t.TempDir()
	t.Chdir(dir)
	writeRuntimeKernelRegistry(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	setTestKernelBuild(t, func(spec kernelBuildSpec) error {
		if err := os.MkdirAll(filepath.Join(spec.outDir, "lib", "modules"), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(spec.outDir, "vmlinuz"), []byte("x"), 0o644); err != nil {
			return err
		}
		// Missing CONFIG_PPPOE: floor must reject.
		cfg := "CONFIG_IP_PNP_DHCP=y\nCONFIG_EXT4_FS=y\nCONFIG_BLK_DEV_INITRD=y\nCONFIG_DEVTMPFS_MOUNT=y\nCONFIG_MODULES=y\nCONFIG_PPP=y\nCONFIG_L2TP=y\nCONFIG_PPPOL2TP=y\nCONFIG_L2TP_V3=y\nCONFIG_VETH=y\n"
		return os.WriteFile(filepath.Join(spec.outDir, "config"), []byte(cfg), 0o644)
	})

	_, err := resolveKernel("7.1.1", archAMD64, runtimeKernelProfile, "", kernelTargetRuntime)
	if err == nil || !strings.Contains(err.Error(), "CONFIG_PPPOE") {
		t.Fatalf("runtime floor error = %v, want CONFIG_PPPOE", err)
	}
}
