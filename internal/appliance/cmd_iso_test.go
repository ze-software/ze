package appliance

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	fatimg "github.com/ze-software/ze/internal/thirdparty/fat"
)

const isoTestApplianceName = "lab"

func setupIsoTestAppliance(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	baseDir = dir
	appDir := filepath.Join(dir, isoTestApplianceName)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("mkdir appliance: %v", err)
	}
	cfg := DefaultConfig(isoTestApplianceName)
	data, err := json.MarshalIndent(&cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, configFileName), append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return appDir
}

func rewriteIsoTestConfig(t *testing.T, appDir string, modify func(*applianceConfig)) {
	t.Helper()
	cfg := DefaultConfig(isoTestApplianceName)
	modify(&cfg)
	data, err := json.MarshalIndent(&cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, configFileName), append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func writeIsoTestFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// writeIsoTestKernelAny writes a fake kernel that passes verifyKernelArch for
// both amd64 and arm64 (the magic offsets don't overlap).
func writeIsoTestKernelAny(t *testing.T, path string) {
	t.Helper()
	buf := make([]byte, 0x206)
	binary.LittleEndian.PutUint32(buf[0x202:], 0x53726448) // x86 "HdrS"
	binary.LittleEndian.PutUint32(buf[56:], 0x644d5241)    // arm64 "ARM\x64"
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write test kernel %s: %v", path, err)
	}
}

func writeIsoTestKernel(t *testing.T, path, arch string) {
	t.Helper()
	buf := make([]byte, 0x206)
	switch arch {
	case archAMD64:
		binary.LittleEndian.PutUint32(buf[0x202:], 0x53726448) // "HdrS"
	case archARM64:
		binary.LittleEndian.PutUint32(buf[56:], 0x644d5241) // "ARM\x64"
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write test kernel %s: %v", path, err)
	}
}

func writeIsoTestChecksum(t *testing.T, imgPath string) string {
	t.Helper()
	data, err := os.ReadFile(imgPath) //nolint:gosec // test fixture
	if err != nil {
		t.Fatalf("read image: %v", err)
	}
	sum := sha256.Sum256(data)
	hexSum := hex.EncodeToString(sum[:])
	if err := os.WriteFile(imgPath+".sha256", []byte(hexSum+"  "+filepath.Base(imgPath)+"\n"), 0o644); err != nil {
		t.Fatalf("write checksum: %v", err)
	}
	return hexSum
}

func setupIsoBuilderTest(t *testing.T) (kernel, initrd string, calls *[]isoBuilderCall) {
	t.Helper()
	kernel = filepath.Join(t.TempDir(), "Image")
	initrd = filepath.Join(t.TempDir(), "initrd.img.gz")
	writeIsoTestKernelAny(t, kernel)
	writeIsoTestFile(t, initrd, "initrd")

	oldLookPath := isoLookPathFn
	oldBuilder := isoBuilderFn
	var got []isoBuilderCall
	isoLookPathFn = func(file string) (string, error) {
		if file == "grub-mkstandalone" || file == "xorriso" {
			return "/tool/" + file, nil
		}
		return "", errors.New("missing")
	}
	isoBuilderFn = func(call isoBuilderCall) error {
		got = append(got, call)
		return os.WriteFile(call.OutputPath, []byte("iso"), 0o644)
	}
	t.Cleanup(func() {
		isoLookPathFn = oldLookPath
		isoBuilderFn = oldBuilder
	})
	return kernel, initrd, &got
}

func runIsoWithTestBuilder(t *testing.T, appDir string, args ...string) []isoBuilderCall {
	t.Helper()
	kernel, initrd, calls := setupIsoBuilderTest(t)
	fullArgs := []string{"--kernel", kernel, "--initrd", initrd}
	fullArgs = append(fullArgs, args...)
	fullArgs = append(fullArgs, isoTestApplianceName)
	code := runIso(fullArgs)
	if code != exitOK {
		t.Fatalf("runIso returned %d, want 0", code)
	}
	if len(*calls) != 1 {
		t.Fatalf("builder calls = %d, want 1", len(*calls))
	}
	if (*calls)[0].StagingDir == "" || !strings.HasPrefix((*calls)[0].StagingDir, appDir) {
		t.Fatalf("staging dir %q is not inside appliance dir %q", (*calls)[0].StagingDir, appDir)
	}
	return *calls
}

func readIsoManifestFromStage(t *testing.T, staging string) isoManifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(staging, "ze-install", "manifest.json")) //nolint:gosec // test fixture
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m isoManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return m
}

func TestRunISOBuilderCreatesExpectedEFIImage(t *testing.T) {
	staging := t.TempDir()
	if err := os.MkdirAll(filepath.Join(staging, "boot", "grub"), 0o755); err != nil {
		t.Fatalf("mkdir grub dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staging, "boot", "grub", "grub.cfg"), []byte("menuentry 'test' {}\n"), 0o644); err != nil {
		t.Fatalf("write grub.cfg: %v", err)
	}
	scripts := t.TempDir()
	grubLog := filepath.Join(scripts, "grub.log")
	grubPath := filepath.Join(scripts, "grub-mkstandalone")
	grubScript := "#!/bin/sh\nset -eu\nout=\ntarget=\nwhile [ $# -gt 0 ]; do\n  case \"$1\" in\n    -O) target=\"$2\"; shift 2 ;;\n    -o) out=\"$2\"; shift 2 ;;\n    *) shift ;;\n  esac\ndone\nprintf '%s' \"$target\" > \"" + grubLog + "\"\nprintf 'efi' > \"$out\"\n"
	if err := os.WriteFile(grubPath, []byte(grubScript), 0o755); err != nil {
		t.Fatalf("write fake grub: %v", err)
	}
	xorrisoPath := filepath.Join(scripts, "xorriso")
	xorrisoScript := "#!/bin/sh\nset -eu\nout=\nbootimg=\nstage=\nwhile [ $# -gt 0 ]; do\n  case \"$1\" in\n    -o) out=\"$2\"; shift 2 ;;\n    -e) bootimg=\"$2\"; shift 2 ;;\n    *) stage=\"$1\"; shift ;;\n  esac\ndone\ntest -f \"$stage/$bootimg\"\nprintf 'iso' > \"$out\"\n"
	if err := os.WriteFile(xorrisoPath, []byte(xorrisoScript), 0o755); err != nil {
		t.Fatalf("write fake xorriso: %v", err)
	}
	out := filepath.Join(t.TempDir(), "out.iso")

	err := runISOBuilder(isoBuilderCall{
		GRUBPath:    grubPath,
		XorrisoPath: xorrisoPath,
		OutputPath:  out,
		StagingDir:  staging,
		GRUBTarget:  "arm64-efi",
		EFIBootFile: "BOOTAA64.EFI",
	})
	if err != nil {
		t.Fatalf("runISOBuilder() error = %v", err)
	}
	if string(mustReadFile(t, out)) != "iso" {
		t.Fatalf("output ISO payload = %q, want iso", string(mustReadFile(t, out)))
	}
	efiImg := filepath.Join(staging, "EFI", "BOOT", "efiboot.img")
	f, err := os.Open(efiImg) //nolint:gosec // test artifact
	if err != nil {
		t.Fatalf("open efiboot image: %v", err)
	}
	defer func() { _ = f.Close() }()
	rd, err := fatimg.NewReader(f)
	if err != nil {
		t.Fatalf("fat reader: %v", err)
	}
	offset, length, err := rd.Extents("/EFI/BOOT/BOOTAA64.EFI")
	if err != nil {
		t.Fatalf("locate EFI boot file: %v", err)
	}
	buf := make([]byte, length)
	if _, err := f.ReadAt(buf, offset); err != nil {
		t.Fatalf("read EFI boot file: %v", err)
	}
	if string(buf) != "efi" {
		t.Fatalf("EFI boot file = %q, want efi", string(buf))
	}
	if got := strings.TrimSpace(string(mustReadFile(t, grubLog))); got != "arm64-efi" {
		t.Fatalf("GRUB target log = %q, want arm64-efi", got)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // test artifact
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func TestRunDispatchesIso(t *testing.T) {
	// VALIDATES: `ze appliance iso <name>` reaches the ISO command handler.
	// PREVENTS: adding the command to help without wiring it into dispatch.
	handler, ok := dispatchTable()["iso"]
	if !ok {
		t.Fatal("dispatchTable missing iso")
	}
	if handler == nil {
		t.Fatal("iso handler is nil")
	}
	if got, stubPtr := reflect.ValueOf(handler).Pointer(), reflect.ValueOf(stub).Pointer(); got == stubPtr {
		t.Fatal("iso handler still points at stub")
	}
}

func TestIsoRequiresName(t *testing.T) {
	// VALIDATES: AC-1 requires an appliance name before filesystem or builder work.
	// PREVENTS: creating an ISO artifact without a selected appliance.
	kernel, initrd, calls := setupIsoBuilderTest(t)
	code := runIso([]string{"--kernel", kernel, "--initrd", initrd})
	if code == exitOK {
		t.Fatal("runIso without name succeeded")
	}
	if len(*calls) != 0 {
		t.Fatalf("builder called %d times without a name", len(*calls))
	}
}

func TestIsoRejectsInvalidName(t *testing.T) {
	// VALIDATES: appliance name validation is enforced before filesystem use.
	// PREVENTS: command-line names with traversal or metacharacters flowing into paths.
	kernel, initrd, calls := setupIsoBuilderTest(t)
	code := runIso([]string{"--kernel", kernel, "--initrd", initrd, "../bad"})
	if code == exitOK {
		t.Fatal("runIso accepted invalid appliance name")
	}
	if len(*calls) != 0 {
		t.Fatalf("builder called %d times for invalid name", len(*calls))
	}
}

func TestIsoBuildsArm64BootArtifacts(t *testing.T) {
	// VALIDATES: appliance ISO supports arm64 images by selecting the arm64 UEFI GRUB target.
	// PREVENTS: emitting x86 boot assets for arm64 appliances built on macOS hosts.
	appDir := setupIsoTestAppliance(t)
	rewriteIsoTestConfig(t, appDir, func(cfg *applianceConfig) {
		cfg.Image.Arch = archARM64
	})
	img := filepath.Join(appDir, "ze-20260101-000000.img")
	writeIsoTestFile(t, img, "image")
	writeIsoTestChecksum(t, img)

	calls := runIsoWithTestBuilder(t, appDir, "--keep-staging")
	if calls[0].GRUBTarget != "arm64-efi" {
		t.Fatalf("GRUB target = %q, want arm64-efi", calls[0].GRUBTarget)
	}
	if calls[0].EFIBootFile != "BOOTAA64.EFI" {
		t.Fatalf("EFI boot file = %q, want BOOTAA64.EFI", calls[0].EFIBootFile)
	}
	manifest := readIsoManifestFromStage(t, calls[0].StagingDir)
	if manifest.Arch != archARM64 {
		t.Fatalf("manifest arch = %q, want %q", manifest.Arch, archARM64)
	}
	grub, err := os.ReadFile(filepath.Join(calls[0].StagingDir, "boot", "grub", "grub.cfg")) //nolint:gosec // test fixture
	if err != nil {
		t.Fatalf("read grub.cfg: %v", err)
	}
	if !strings.Contains(string(grub), "console=ttyAMA0,115200n8 console=tty0") {
		t.Fatalf("grub.cfg missing arm64 console: %s", string(grub))
	}
}

func TestIsoRejectsKernelArchMismatch(t *testing.T) {
	cases := []struct {
		name       string
		kernelArch string
		configArch string
	}{
		{"arm64 kernel for amd64 config", archARM64, archAMD64},
		{"amd64 kernel for arm64 config", archAMD64, archARM64},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			appDir := setupIsoTestAppliance(t)
			if tc.configArch != archAMD64 {
				rewriteIsoTestConfig(t, appDir, func(cfg *applianceConfig) {
					cfg.Image.Arch = tc.configArch
				})
			}
			img := filepath.Join(appDir, "ze-20260101-000000.img")
			writeIsoTestFile(t, img, "image")
			writeIsoTestChecksum(t, img)

			kernel := filepath.Join(t.TempDir(), "Image")
			writeIsoTestKernel(t, kernel, tc.kernelArch)
			initrd := filepath.Join(t.TempDir(), "initrd.img.gz")
			writeIsoTestFile(t, initrd, "initrd")

			oldLookPath := isoLookPathFn
			oldBuilder := isoBuilderFn
			isoLookPathFn = func(file string) (string, error) { return "/tool/" + file, nil }
			isoBuilderFn = func(call isoBuilderCall) error { return os.WriteFile(call.OutputPath, []byte("iso"), 0o644) }
			t.Cleanup(func() {
				isoLookPathFn = oldLookPath
				isoBuilderFn = oldBuilder
			})

			code := runIso([]string{"--kernel", kernel, "--initrd", initrd, isoTestApplianceName})
			if code == exitOK {
				t.Fatalf("runIso accepted %s kernel for %s appliance", tc.kernelArch, tc.configArch)
			}
		})
	}
}

func TestIsoUsesDefaultKernelAndInitrdArtifactPaths(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	writeInstallerKernelRegistry(t)
	kernelPath := filepath.Join(root, "build", "kernel", "Image")
	initrdPath := filepath.Join(root, "build", "initrd", "initrd.img.gz")
	if err := os.MkdirAll(filepath.Dir(kernelPath), 0o755); err != nil {
		t.Fatalf("mkdir kernel dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(initrdPath), 0o755); err != nil {
		t.Fatalf("mkdir initrd dir: %v", err)
	}
	writeIsoTestKernelAny(t, kernelPath)
	writeIsoTestFile(t, filepath.Join(root, "build", "kernel", ".variant"), archAMD64+"-"+defaultKernelProfile+"-"+defaultKernelVersion+"-docker")
	writeIsoTestFile(t, initrdPath, "default-initrd")

	appDir := setupIsoTestAppliance(t)
	img := filepath.Join(appDir, "ze-20260101-000000.img")
	writeIsoTestFile(t, img, "image")
	writeIsoTestChecksum(t, img)
	_, _, calls := setupIsoBuilderTest(t)

	code := runIso([]string{"--keep-staging", isoTestApplianceName})
	if code != exitOK {
		t.Fatalf("runIso returned %d, want 0", code)
	}
	if len(*calls) != 1 {
		t.Fatalf("builder calls = %d, want 1", len(*calls))
	}
	stage := (*calls)[0].StagingDir
	if len(mustReadFile(t, filepath.Join(stage, "boot", "kernel"))) != 0x206 {
		t.Fatal("staged kernel size does not match default kernel stub")
	}
	if got := string(mustReadFile(t, filepath.Join(stage, "boot", "initrd.img.gz"))); got != "default-initrd" {
		t.Fatalf("staged initrd = %q, want default-initrd", got)
	}
}

func TestIsoRejectsStaleFallbackForCustomProfile(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	writeInstallerKernelRegistry(t)
	if err := os.WriteFile(filepath.Join(root, kernelInstallerConfigDir, "custom.config"), []byte("CONFIG_CUSTOM=y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, kernelInstallerConfigDir, "custom.require"), []byte("CONFIG_CUSTOM\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	kernelPath := filepath.Join(root, "build", "kernel", "Image")
	initrdPath := filepath.Join(root, "build", "initrd", "initrd.img.gz")
	if err := os.MkdirAll(filepath.Dir(kernelPath), 0o755); err != nil {
		t.Fatalf("mkdir kernel dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(initrdPath), 0o755); err != nil {
		t.Fatalf("mkdir initrd dir: %v", err)
	}
	writeIsoTestKernelAny(t, kernelPath)
	writeIsoTestFile(t, filepath.Join(root, "build", "kernel", ".variant"), archAMD64+"-"+defaultKernelProfile+"-"+defaultKernelVersion+"-docker")
	writeIsoTestFile(t, initrdPath, "default-initrd")

	appDir := setupIsoTestAppliance(t)
	rewriteIsoTestConfig(t, appDir, func(cfg *applianceConfig) {
		cfg.Image.KernelProfile = "custom"
	})
	img := filepath.Join(appDir, "ze-20260101-000000.img")
	writeIsoTestFile(t, img, "image")
	writeIsoTestChecksum(t, img)
	_, _, calls := setupIsoBuilderTest(t)

	code := runIso([]string{"--keep-staging", isoTestApplianceName})
	if code == exitOK {
		t.Fatal("runIso accepted stale fallback kernel for custom profile")
	}
	if len(*calls) != 0 {
		t.Fatalf("builder calls = %d, want 0", len(*calls))
	}
}
func TestIsoUsesLatestImageByDefault(t *testing.T) {
	// VALIDATES: AC-1 selects the latest built image when --image is omitted.
	// PREVENTS: stale appliance images being wrapped into installer ISOs by default.
	appDir := setupIsoTestAppliance(t)
	oldImg := filepath.Join(appDir, "ze-20260101-000000.img")
	newImg := filepath.Join(appDir, "ze-20260201-000000.img")
	writeIsoTestFile(t, oldImg, "older")
	writeIsoTestChecksum(t, oldImg)
	writeIsoTestFile(t, newImg, "newer")
	newSHA := writeIsoTestChecksum(t, newImg)

	calls := runIsoWithTestBuilder(t, appDir, "--keep-staging")
	manifest := readIsoManifestFromStage(t, calls[0].StagingDir)
	wantImage := filepath.Base(newImg) + ".gz"
	if manifest.Image != wantImage {
		t.Fatalf("manifest image = %q, want %q", manifest.Image, wantImage)
	}
	if manifest.ImageSHA256 != newSHA {
		t.Fatalf("manifest sha = %q, want %q", manifest.ImageSHA256, newSHA)
	}
	if manifest.ImageCompression != "gzip" {
		t.Fatalf("manifest compression = %q, want gzip", manifest.ImageCompression)
	}
	compressed, err := os.ReadFile(filepath.Join(calls[0].StagingDir, "ze-install", "images", wantImage)) //nolint:gosec // test fixture
	if err != nil {
		t.Fatalf("read staged compressed image: %v", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("open gzip: %v", err)
	}
	decompressed, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("decompress staged image: %v", err)
	}
	if string(decompressed) != "newer" {
		t.Fatalf("staged image content = %q, want newer", string(decompressed))
	}
}

func TestIsoUsesExplicitImage(t *testing.T) {
	// VALIDATES: AC-3 allows selecting an image by name inside the appliance directory.
	// PREVENTS: --image being ignored in favor of the latest image.
	appDir := setupIsoTestAppliance(t)
	oldImg := filepath.Join(appDir, "ze-20260101-000000.img")
	newImg := filepath.Join(appDir, "ze-20260201-000000.img")
	writeIsoTestFile(t, oldImg, "older")
	writeIsoTestChecksum(t, oldImg)
	writeIsoTestFile(t, newImg, "newer")
	writeIsoTestChecksum(t, newImg)

	calls := runIsoWithTestBuilder(t, appDir, "--image", filepath.Base(oldImg), "--keep-staging")
	manifest := readIsoManifestFromStage(t, calls[0].StagingDir)
	wantImage := filepath.Base(oldImg) + ".gz"
	if manifest.Image != wantImage {
		t.Fatalf("manifest image = %q, want %q", manifest.Image, wantImage)
	}
}

func TestIsoRejectsImageNameUnsafeForKernelCmdline(t *testing.T) {
	// VALIDATES: the selected image name matches the initrd's accepted ze.image charset.
	// PREVENTS: generating an ISO that cannot boot, or that injects extra kernel arguments.
	appDir := setupIsoTestAppliance(t)
	img := filepath.Join(appDir, "ze-unsafe extra.img")
	writeIsoTestFile(t, img, "image")
	writeIsoTestChecksum(t, img)
	kernel, initrd, calls := setupIsoBuilderTest(t)

	code := runIso([]string{"--kernel", kernel, "--initrd", initrd, "--image", filepath.Base(img), "lab"})
	if code == exitOK {
		t.Fatal("runIso accepted image name that is unsafe for ISO boot")
	}
	if len(*calls) != 0 {
		t.Fatalf("builder called %d times for unsafe image name", len(*calls))
	}
}

func TestIsoRejectsOutputThatOverwritesImage(t *testing.T) {
	// VALIDATES: --output cannot point at the selected source image.
	// PREVENTS: successful ISO creation from destroying the appliance image artifact.
	appDir := setupIsoTestAppliance(t)
	img := filepath.Join(appDir, "ze-20260101-000000.img")
	writeIsoTestFile(t, img, "image")
	writeIsoTestChecksum(t, img)
	kernel, initrd, calls := setupIsoBuilderTest(t)

	code := runIso([]string{"--kernel", kernel, "--initrd", initrd, "--output", img, "lab"})
	if code == exitOK {
		t.Fatal("runIso accepted output path that overwrites the image")
	}
	if len(*calls) != 0 {
		t.Fatalf("builder called %d times for output overwrite", len(*calls))
	}
}

func TestIsoRejectsImagePathEscape(t *testing.T) {
	// VALIDATES: AC-3 rejects path traversal and absolute-path escape for --image.
	// PREVENTS: wrapping arbitrary host files into an appliance ISO.
	appDir := setupIsoTestAppliance(t)
	outside := filepath.Join(filepath.Dir(appDir), "outside.img")
	writeIsoTestFile(t, outside, "outside")
	writeIsoTestChecksum(t, outside)
	kernel, initrd, calls := setupIsoBuilderTest(t)

	code := runIso([]string{"--kernel", kernel, "--initrd", initrd, "--image", "../outside.img", "lab"})
	if code == exitOK {
		t.Fatal("runIso accepted path escape")
	}
	if len(*calls) != 0 {
		t.Fatalf("builder called %d times for path escape", len(*calls))
	}
}

func TestIsoRejectsImageSymlinkEscape(t *testing.T) {
	// VALIDATES: AC-3 rejects symlinks that resolve outside the appliance directory.
	// PREVENTS: path containment checks that only compare lexical path prefixes.
	appDir := setupIsoTestAppliance(t)
	outside := filepath.Join(filepath.Dir(appDir), "outside.img")
	writeIsoTestFile(t, outside, "outside")
	writeIsoTestChecksum(t, outside)
	link := filepath.Join(appDir, "ze-20260101-000000.img")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	kernel, initrd, calls := setupIsoBuilderTest(t)

	code := runIso([]string{"--kernel", kernel, "--initrd", initrd, "lab"})
	if code == exitOK {
		t.Fatal("runIso accepted symlink escape")
	}
	if len(*calls) != 0 {
		t.Fatalf("builder called %d times for symlink escape", len(*calls))
	}
}

func TestIsoRejectsChecksumMismatch(t *testing.T) {
	// VALIDATES: AC-4 rejects selected images whose .sha256 sidecar does not match.
	// PREVENTS: producing a trusted offline installer from corrupt image bytes.
	appDir := setupIsoTestAppliance(t)
	img := filepath.Join(appDir, "ze-20260101-000000.img")
	writeIsoTestFile(t, img, "image")
	writeIsoTestFile(t, img+".sha256", strings.Repeat("0", 64)+"  "+filepath.Base(img)+"\n")
	kernel, initrd, calls := setupIsoBuilderTest(t)

	code := runIso([]string{"--kernel", kernel, "--initrd", initrd, "lab"})
	if code == exitOK {
		t.Fatal("runIso accepted checksum mismatch")
	}
	if len(*calls) != 0 {
		t.Fatalf("builder called %d times after checksum mismatch", len(*calls))
	}
}

func TestIsoMissingChecksumPolicy(t *testing.T) {
	// VALIDATES: AC-5 missing checksum is a hard error for durable ISO artifacts.
	// PREVENTS: silently creating an installer whose payload was never verified.
	appDir := setupIsoTestAppliance(t)
	img := filepath.Join(appDir, "ze-20260101-000000.img")
	writeIsoTestFile(t, img, "image")
	kernel, initrd, calls := setupIsoBuilderTest(t)

	code := runIso([]string{"--kernel", kernel, "--initrd", initrd, "lab"})
	if code == exitOK {
		t.Fatal("runIso accepted image with no checksum")
	}
	if len(*calls) != 0 {
		t.Fatalf("builder called %d times without checksum", len(*calls))
	}
}

func TestIsoBuildsExpectedStagingPlan(t *testing.T) {
	// VALIDATES: AC-6 stages boot artifacts, image, checksum, metadata, and boot config.
	// PREVENTS: creating an ISO missing one of the installer inputs.
	appDir := setupIsoTestAppliance(t)
	img := filepath.Join(appDir, "ze-20260101-000000.img")
	writeIsoTestFile(t, img, "image")
	writeIsoTestChecksum(t, img)

	calls := runIsoWithTestBuilder(t, appDir, "--target", "/dev/vda", "--keep-staging")
	stage := calls[0].StagingDir
	for _, rel := range []string{
		"boot/kernel",
		"boot/initrd.img.gz",
		"boot/grub/grub.cfg",
		"ze-install/manifest.json",
		"ze-install/media-id",
		"ze-install/images/ze-20260101-000000.img.gz",
		"ze-install/images/ze-20260101-000000.img.gz.sha256",
	} {
		if _, err := os.Stat(filepath.Join(stage, rel)); err != nil {
			t.Fatalf("staged %s missing: %v", rel, err)
		}
	}
	grub, err := os.ReadFile(filepath.Join(stage, "boot", "grub", "grub.cfg")) //nolint:gosec // test fixture
	if err != nil {
		t.Fatalf("read grub.cfg: %v", err)
	}
	mediaIDData, err := os.ReadFile(filepath.Join(stage, "ze-install", "media-id")) //nolint:gosec // test fixture
	if err != nil {
		t.Fatalf("read media-id: %v", err)
	}
	manifest := readIsoManifestFromStage(t, stage)
	if manifest.MediaID == "" {
		t.Fatal("manifest media id is empty")
	}
	if strings.TrimSpace(string(mediaIDData)) != manifest.MediaID {
		t.Fatalf("media-id file = %q, manifest = %q", strings.TrimSpace(string(mediaIDData)), manifest.MediaID)
	}
	if !strings.Contains(string(grub), "search --no-floppy --file /ze-install/media-id --set=root") || !strings.Contains(string(grub), "ze.source=iso") || !strings.Contains(string(grub), "ze.target=/dev/vda") || !strings.Contains(string(grub), "ze.media-id="+manifest.MediaID) || !strings.Contains(string(grub), "console=ttyS0,115200n8 console=tty0") || !strings.Contains(string(grub), "ze.image=ze-20260101-000000.img.gz") {
		t.Fatalf("grub.cfg does not contain ISO source, media search, target, media id, compressed image name, and amd64 console: %s", string(grub))
	}
}

func TestIsoInvokesBuilderWithArgumentVector(t *testing.T) {
	// VALIDATES: AC-17 invokes the external ISO tool through an argument vector.
	// PREVENTS: shell interpolation of output or staging paths.
	appDir := setupIsoTestAppliance(t)
	img := filepath.Join(appDir, "ze-20260101-000000.img")
	out := filepath.Join(appDir, "custom.iso")
	writeIsoTestFile(t, img, "image")
	writeIsoTestChecksum(t, img)

	calls := runIsoWithTestBuilder(t, appDir, "--output", out)
	call := calls[0]
	if call.GRUBPath != "/tool/grub-mkstandalone" {
		t.Fatalf("GRUB builder = %q", call.GRUBPath)
	}
	if call.XorrisoPath != "/tool/xorriso" {
		t.Fatalf("xorriso = %q", call.XorrisoPath)
	}
	if call.OutputPath == out {
		t.Fatalf("builder output path %q should be a temporary file, not final %q", call.OutputPath, out)
	}
	if filepath.Dir(call.OutputPath) != filepath.Dir(out) {
		t.Fatalf("builder output dir = %q, want %q", filepath.Dir(call.OutputPath), filepath.Dir(out))
	}
	isoData, err := os.ReadFile(out) //nolint:gosec // test fixture
	if err != nil {
		t.Fatalf("read final ISO: %v", err)
	}
	if string(isoData) != "iso" {
		t.Fatalf("final ISO = %q, want iso", string(isoData))
	}
	if !strings.HasPrefix(call.StagingDir, appDir) {
		t.Fatalf("staging = %q, want inside %q", call.StagingDir, appDir)
	}
}

func TestIsoCleansStagingOnFailure(t *testing.T) {
	// VALIDATES: failed ISO builds remove temporary staging unless --keep-staging is set.
	// PREVENTS: stale image copies and secrets accumulating in appliance directories.
	appDir := setupIsoTestAppliance(t)
	img := filepath.Join(appDir, "ze-20260101-000000.img")
	writeIsoTestFile(t, img, "image")
	writeIsoTestChecksum(t, img)
	kernel, initrd, _ := setupIsoBuilderTest(t)

	oldBuilder := isoBuilderFn
	var stage string
	isoBuilderFn = func(call isoBuilderCall) error {
		stage = call.StagingDir
		return errors.New("builder failed")
	}
	defer func() { isoBuilderFn = oldBuilder }()

	code := runIso([]string{"--kernel", kernel, "--initrd", initrd, "lab"})
	if code == exitOK {
		t.Fatal("runIso succeeded after builder failure")
	}
	if stage == "" {
		t.Fatal("builder was not called")
	}
	if _, err := os.Stat(stage); !os.IsNotExist(err) {
		t.Fatalf("staging dir still exists after failure: %v", err)
	}
}

func TestIsoKeepsStagingOnFailureWhenRequested(t *testing.T) {
	// VALIDATES: --keep-staging preserves the staging tree even on build failure.
	// PREVENTS: losing the only inspection artifacts for a failed ISO build.
	appDir := setupIsoTestAppliance(t)
	img := filepath.Join(appDir, "ze-20260101-000000.img")
	writeIsoTestFile(t, img, "image")
	writeIsoTestChecksum(t, img)
	kernel, initrd, _ := setupIsoBuilderTest(t)

	oldBuilder := isoBuilderFn
	var stage string
	isoBuilderFn = func(call isoBuilderCall) error {
		stage = call.StagingDir
		return errors.New("builder failed")
	}
	defer func() { isoBuilderFn = oldBuilder }()

	code := runIso([]string{"--kernel", kernel, "--initrd", initrd, "--keep-staging", "lab"})
	if code == exitOK {
		t.Fatal("runIso succeeded after builder failure")
	}
	if stage == "" {
		t.Fatal("builder was not called")
	}
	if _, err := os.Stat(stage); err != nil {
		t.Fatalf("staging dir missing with --keep-staging: %v", err)
	}
}

func TestIsoDoesNotDeleteExistingOutputOnFailure(t *testing.T) {
	// VALIDATES: builder failures never delete an existing caller-supplied output file.
	// PREVENTS: losing unrelated artifacts after a failed ISO build.
	appDir := setupIsoTestAppliance(t)
	img := filepath.Join(appDir, "ze-20260101-000000.img")
	out := filepath.Join(appDir, "existing.iso")
	writeIsoTestFile(t, img, "image")
	writeIsoTestChecksum(t, img)
	writeIsoTestFile(t, out, "keep-me")
	kernel, initrd, _ := setupIsoBuilderTest(t)

	oldBuilder := isoBuilderFn
	isoBuilderFn = func(call isoBuilderCall) error {
		return errors.New("builder failed")
	}
	defer func() { isoBuilderFn = oldBuilder }()

	code := runIso([]string{"--kernel", kernel, "--initrd", initrd, "--output", out, "lab"})
	if code == exitOK {
		t.Fatal("runIso succeeded after builder failure")
	}
	data, err := os.ReadFile(out) //nolint:gosec // test fixture
	if err != nil {
		t.Fatalf("read existing output: %v", err)
	}
	if string(data) != "keep-me" {
		t.Fatalf("existing output = %q, want keep-me", string(data))
	}
}

func TestIsoBuilderDependencyMissing(t *testing.T) {
	// VALIDATES: AC-17 fails before partial output when the ISO builder is absent.
	// PREVENTS: leaving a misleading output artifact after dependency preflight failure.
	appDir := setupIsoTestAppliance(t)
	img := filepath.Join(appDir, "ze-20260101-000000.img")
	out := filepath.Join(appDir, "missing-builder.iso")
	writeIsoTestFile(t, img, "image")
	writeIsoTestChecksum(t, img)
	kernel := filepath.Join(t.TempDir(), "Image")
	initrd := filepath.Join(t.TempDir(), "initrd.img.gz")
	writeIsoTestFile(t, kernel, "kernel")
	writeIsoTestFile(t, initrd, "initrd")

	oldLookPath := isoLookPathFn
	oldBuilder := isoBuilderFn
	isoLookPathFn = func(string) (string, error) { return "", errors.New("missing") }
	isoBuilderFn = func(call isoBuilderCall) error {
		t.Fatalf("builder should not be called: %+v", call)
		return nil
	}
	defer func() {
		isoLookPathFn = oldLookPath
		isoBuilderFn = oldBuilder
	}()

	code := runIso([]string{"--kernel", kernel, "--initrd", initrd, "--output", out, "lab"})
	if code == exitOK {
		t.Fatal("runIso succeeded without builder")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("output exists after preflight failure: %v", err)
	}
}

func TestIsoExplicitGrubBuilderRequiresXorriso(t *testing.T) {
	// VALIDATES: explicit grub-mkstandalone builders still fast-fail when xorriso is missing.
	// PREVENTS: staging a full appliance image before dependency failure.
	appDir := setupIsoTestAppliance(t)
	img := filepath.Join(appDir, "ze-20260101-000000.img")
	out := filepath.Join(appDir, "explicit-builder.iso")
	writeIsoTestFile(t, img, "image")
	writeIsoTestChecksum(t, img)
	kernel, initrd, _ := setupIsoBuilderTest(t)

	oldLookPath := isoLookPathFn
	oldBuilder := isoBuilderFn
	isoLookPathFn = func(file string) (string, error) {
		if file == "grub-mkstandalone" {
			return "/tool/grub-mkstandalone", nil
		}
		return "", errors.New("missing")
	}
	isoBuilderFn = func(call isoBuilderCall) error {
		t.Fatalf("builder should not be called: %+v", call)
		return nil
	}
	defer func() {
		isoLookPathFn = oldLookPath
		isoBuilderFn = oldBuilder
	}()

	code := runIso([]string{"--kernel", kernel, "--initrd", initrd, "--builder", "grub-mkstandalone", "--output", out, isoTestApplianceName})
	if code == exitOK {
		t.Fatal("runIso succeeded without xorriso for explicit grub builder")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("output exists after missing xorriso preflight: %v", err)
	}
}
