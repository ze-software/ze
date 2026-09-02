package qemu

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	zeenv "github.com/ze-software/ze/internal/core/env"
)

func installTestEnv(t *testing.T, values map[string]string) {
	t.Helper()
	defaults := map[string]string{
		"ZE_INSTALL_ARCH": "", "ZE_INSTALL_KERNEL": "", "ZE_INSTALL_IMAGE": "",
		"ZE_INSTALL_ZEFS": "", "ZE_INSTALL_KEEP": "", "ZE_INSTALL_IMAGE_SIZE": "",
		"ZE_INSTALL_SSH_USER": "admin", "ZE_INSTALL_SSH_PASS": "secret",
		"ZE_INSTALL_SSH_PORT": "", "ZE_INSTALL_QEMU_ACCEL": "",
		"ZE_INSTALL_NIC": "virtio-net-pci", "ZE_INSTALL_AARCH64_BIOS": "",
		"ZE_INSTALL_X86_UEFI_BIOS": "", "ZE_INSTALL_BOOT_TIMEOUT": "300s",
		"ZE_INSTALL_FAULT_TIMEOUT": "90s", "ZE_INSTALL_RESCUE_STEP_TIMEOUT": "120s",
		"ZE_INSTALL_RESCUE_TIMEOUT": "120s",
	}
	maps.Copy(defaults, values)
	for key, value := range defaults {
		t.Setenv(key, value)
	}
	zeenv.ResetCache()
	t.Cleanup(zeenv.ResetCache)
}

// VALIDATES: the ISO and Ventoy import chain keeps amd64 as its validated default while HTTP and scenarios follow the host.
// PREVENTS: an arm64 host silently moving the ISO gate onto the unproven architecture.
func TestInstallDefaultsKeepTheProducerArchitectureRules(t *testing.T) {
	installTestEnv(t, map[string]string{"ZE_INSTALL_ARCH": ""})
	for _, kind := range []InstallKind{InstallKindISO, InstallKindVentoy} {
		options, err := DefaultInstallOptions(kind)
		if err != nil {
			t.Fatal(err)
		}
		if options.Arch != ArchAMD64 {
			t.Fatalf("%s arch=%q want amd64", kind, options.Arch)
		}
	}
	options, err := DefaultInstallOptions(InstallKindHTTP)
	if err != nil {
		t.Fatal(err)
	}
	if options.Arch != installHostArch(runtimeArch()) {
		t.Fatalf("HTTP arch=%q", options.Arch)
	}
}

func runtimeArch() string { return runtime.GOARCH }

// VALIDATES: every numeric environment boundary rejects zero, overflow, fractions, and non-numbers.
// PREVENTS: a zero deadline or invalid port becoming an unbounded or misdirected wait.
func TestInstallOptionBoundaries(t *testing.T) {
	for _, test := range []struct{ name, key, value string }{
		{"port-zero", "ZE_INSTALL_SSH_PORT", "0"}, {"port-high", "ZE_INSTALL_SSH_PORT", "65536"},
		{"port-text", "ZE_INSTALL_SSH_PORT", "ssh"}, {"bytes-zero", "ZE_INSTALL_IMAGE_SIZE", "0"},
		{"bytes-text", "ZE_INSTALL_IMAGE_SIZE", "large"}, {"timeout-zero", "ZE_INSTALL_BOOT_TIMEOUT", "0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			installTestEnv(t, map[string]string{"ZE_INSTALL_ARCH": "amd64", test.key: test.value})
			if _, err := DefaultInstallOptions(InstallKindHTTP); err == nil {
				t.Fatal("invalid option was accepted")
			}
		})
	}
	installTestEnv(t, map[string]string{"ZE_INSTALL_ARCH": "arm64", "ZE_INSTALL_SSH_PORT": "65535", "ZE_INSTALL_IMAGE_SIZE": "1", "ZE_INSTALL_BOOT_TIMEOUT": "1s"})
	options, err := DefaultInstallOptions(InstallKindHTTP)
	if err != nil {
		t.Fatal(err)
	}
	if options.SSHPort != 65535 || options.ImageSize != 1 || options.BootTimeout != time.Second {
		t.Fatalf("boundary options=%+v", options)
	}
}

// VALIDATES: Darwin selects HVF, Linux selects KVM only with read-write access, and the explicit accelerator wins.
// PREVENTS: /dev/kvm existence without access producing a QEMU timeout.
func TestInstallAcceleratorSelection(t *testing.T) {
	installer := installTestInstaller(t, InstallKindHTTP)
	installer.Options.Accelerator = ""
	installer.ops.GOOS = "darwin"
	if got := installer.accelerator(); got != "hvf" {
		t.Fatal(got)
	}
	installer.ops.GOOS = "linux"
	installer.ops.Access = func(string, uint32) bool { return false }
	if got := installer.accelerator(); got != "tcg" {
		t.Fatal(got)
	}
	installer.ops.Access = func(string, uint32) bool { return true }
	if got := installer.accelerator(); got != "kvm" {
		t.Fatal(got)
	}
	installer.Options.Accelerator = "whpx"
	if got := installer.accelerator(); got != "whpx" {
		t.Fatal(got)
	}
}

// VALIDATES: the host ze build carries no GOOS or GOARCH while the initrd invocation carries Linux target GOARCH.
// PREVENTS: target architecture leaking into ze-host and causing exec format errors.
func TestInstallBuildSeparatesHostAndTargetArchitecture(t *testing.T) {
	installer := installTestInstaller(t, InstallKindHTTP)
	installer.Options.Arch = ArchARM64
	installer.ops.Environ = func() []string {
		return []string{"PATH=/bin", "GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=1", "ZE_SENTINEL=yes"}
	}
	calls := make([]commandSpec, 0, 2)
	work := t.TempDir()
	initrd := filepath.Join(work, "initrd.img.gz")
	installer.ops.Run = func(_ context.Context, spec commandSpec) (commandResult, error) {
		calls = append(calls, spec)
		if spec.Name != "go" {
			if err := os.WriteFile(initrd, []byte("initrd"), 0o600); err != nil {
				return commandResult{}, err
			}
			return commandResult{Stdout: "initrd ready: " + initrd + "\n"}, nil
		}
		return commandResult{}, nil
	}
	if _, err := installer.buildInitrd(context.Background(), work, "XDG_CACHE_HOME=/cache-fault", "ZE_INITRD_FAULT=1"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("calls=%d want 2", len(calls))
	}
	if calls[0].Name != "go" || !reflect.DeepEqual(calls[0].Args, []string{"build", "-tags", "ze_core,ze_setup", "-o", filepath.Join(work, "ze-host"), "./cmd/ze"}) {
		t.Fatalf("host build=%#v", calls[0])
	}
	for _, entry := range calls[0].Env {
		if strings.HasPrefix(entry, "GOARCH=") || strings.HasPrefix(entry, "GOOS=") {
			t.Fatalf("target leaked into host environment: %q", entry)
		}
	}
	if !envHas(calls[0].Env, "CGO_ENABLED=0") {
		t.Fatalf("host env=%q", calls[0].Env)
	}
	if !envHas(calls[0].Env, "XDG_CACHE_HOME=/cache-fault") || !envHas(calls[0].Env, "ZE_INITRD_FAULT=1") {
		t.Fatalf("fault host env=%q", calls[0].Env)
	}
	if !envHas(calls[1].Env, "GOOS=linux") || !envHas(calls[1].Env, "GOARCH=arm64") || !envHas(calls[1].Env, "CGO_ENABLED=0") {
		t.Fatalf("target env=%q", calls[1].Env)
	}
	if !envHas(calls[1].Env, "XDG_CACHE_HOME=/cache-fault") || !envHas(calls[1].Env, "ZE_INITRD_FAULT=1") {
		t.Fatalf("fault target env=%q", calls[1].Env)
	}
}

func envHas(environ []string, want string) bool {
	return slices.Contains(environ, want)
}

// VALIDATES: complete base, installer, ISO, and Ventoy argv retain every disk, network, firmware, target, and serial token.
// PREVENTS: a missing artifact or scenario hidden by a partial argv comparison.
func TestInstallQEMUArgvPlansAreComplete(t *testing.T) {
	installer := installTestInstaller(t, InstallKindHTTP)
	installer.Options.Arch, installer.Options.Accelerator, installer.Options.NIC = ArchAMD64, "tcg", "virtio-net-pci"
	httpArgv, err := installer.HTTPArgv("kernel", "initrd", "disk", 8080)
	if err != nil {
		t.Fatal(err)
	}
	wantHTTP := []string{"qemu-system-x86_64", "-smp", "2", "-m", "1024", "-nographic", "-serial", "mon:stdio", "-machine", "accel=tcg", "-kernel", "kernel", "-initrd", "initrd", "-append", "console=ttyS0 ze.server=10.0.2.2 ze.port=8080 ze.image=ze-test.img ip=dhcp panic=-1", "-drive", "file=disk,format=raw,if=virtio", "-netdev", "user,id=net0", "-device", "virtio-net-pci,netdev=net0"}
	if !reflect.DeepEqual(httpArgv, wantHTTP) {
		t.Fatalf("HTTP argv\n got=%q\nwant=%q", httpArgv, wantHTTP)
	}
	isoArgv, err := installer.ISOArgv("install.iso", "target", "extra", "OVMF.fd")
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"-bios", "OVMF.fd", "file=install.iso,media=cdrom,readonly=on,if=ide", "file=target,format=raw,if=virtio", "file=extra,format=raw,if=virtio"} {
		if !slices.Contains(isoArgv, token) {
			t.Fatalf("ISO argv misses %q: %q", token, isoArgv)
		}
	}
	installer.Options.Kernel = "kernel"
	ventoy, err := installer.VentoyArgv("initrd", "target", "fat", "media", "image.gz")
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"-no-reboot", "file=target,format=raw,if=virtio", "file=fat,format=raw,if=virtio", "console=ttyS0 ze.source=iso ze.media-id=media ze.image=image.gz ze.target=/dev/vda panic=-1"} {
		if !slices.Contains(ventoy, token) {
			t.Fatalf("Ventoy argv misses %q: %q", token, ventoy)
		}
	}
}

// VALIDATES: arm64 uses virt, max CPU, ttyAMA0, and virtio-scsi removable media.
// PREVENTS: x86 firmware or IDE assumptions leaking into arm64.
func TestInstallArm64QEMUPlan(t *testing.T) {
	installer := installTestInstaller(t, InstallKindISO)
	installer.Options.Arch, installer.Options.Accelerator = ArchARM64, "hvf"
	base := installer.qemuBase(false)
	for _, token := range []string{"qemu-system-aarch64", "virt,highmem=off,accel=hvf", "max"} {
		if !slices.Contains(base, token) {
			t.Fatalf("arm base misses %q: %q", token, base)
		}
	}
	want := []string{"-drive", "file=install.iso,if=none,id=cdrom,media=cdrom,readonly=on", "-device", "virtio-scsi-pci,id=scsi0", "-device", "scsi-cd,drive=cdrom,bus=scsi0.0"}
	if got := installer.isoCDROMArgs("install.iso"); !reflect.DeepEqual(got, want) {
		t.Fatalf("cdrom=%q want=%q", got, want)
	}
}

// VALIDATES: image bytes and the checksum sidecar are produced from the served copy.
// PREVENTS: hashing one path and serving different bytes.
func TestInstallChecksumArtifactBytes(t *testing.T) {
	installer := installTestInstaller(t, InstallKindHTTP)
	root := t.TempDir()
	source := filepath.Join(root, "source.img")
	served := filepath.Join(root, "served")
	if err := os.Mkdir(served, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("image-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	target, err := installer.writeChecksum(source, served)
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(target); string(data) != "image-bytes" {
		t.Fatalf("served=%q", data)
	}
	digest, err := installSHA256(target)
	if err != nil {
		t.Fatal(err)
	}
	sidecar, _ := os.ReadFile(target + ".sha256")
	if string(sidecar) != digest+"  ze-test.img\n" {
		t.Fatalf("sidecar=%q", sidecar)
	}
}

// VALIDATES: the slirp-facing HTTP service serves all three exact paths, bytes, and lengths, and refuses every other path.
// PREVENTS: a one-connection guestfwd substitute or a missing checksum/ZeFS endpoint.
func TestInstallHTTPServicePayloadsAndPaths(t *testing.T) {
	served := t.TempDir()
	payloads := map[string][]byte{
		InstallImageName:             []byte("image"),
		InstallImageName + ".sha256": []byte("digest"),
		"database.zefs":              []byte("zefs"),
	}
	for name, payload := range payloads {
		if err := os.WriteFile(filepath.Join(served, name), payload, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	server, port, err := startInstallHTTP(t.Context(), served)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Stop(); err != nil {
			t.Errorf("stop installer HTTP server: %v", err)
		}
	})
	paths := map[string][]byte{
		"/install/image/" + InstallImageName:             payloads[InstallImageName],
		"/install/image/" + InstallImageName + ".sha256": payloads[InstallImageName+".sha256"],
		"/install/database.zefs":                         payloads["database.zefs"],
	}
	client := &http.Client{}
	for path, want := range paths {
		request, requestErr := http.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"http://127.0.0.1:"+strconv.Itoa(port)+path,
			http.NoBody,
		)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		response, requestErr := client.Do(request)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		data, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		if response.StatusCode != http.StatusOK || !bytes.Equal(data, want) {
			t.Fatalf("%s status=%d data=%q want=%q", path, response.StatusCode, data, want)
		}
		if response.ContentLength != int64(len(want)) {
			t.Fatalf("%s length=%d want=%d", path, response.ContentLength, len(want))
		}
	}
	request, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"http://127.0.0.1:"+strconv.Itoa(port)+"/not-an-install-path",
		http.NoBody,
	)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown path status=%d", response.StatusCode)
	}
}

// VALIDATES: gzip extraction preserves in-bound bytes and refuses output one byte over the configured image size.
// PREVENTS: a compressed ISO artifact expanding past its disk-write ceiling while still reporting success.
func TestInstallISOImageGzipByteCeiling(t *testing.T) {
	payload := []byte("installer-image")
	for _, test := range []struct {
		name      string
		ceiling   int64
		wantError bool
	}{
		{name: "at-ceiling", ceiling: int64(len(payload))},
		{name: "over-ceiling", ceiling: int64(len(payload) - 1), wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "image.gz")
			target := filepath.Join(root, "image")
			var compressed bytes.Buffer
			archive := gzip.NewWriter(&compressed)
			if _, err := archive.Write(payload); err != nil {
				t.Fatal(err)
			}
			if err := archive.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(source, compressed.Bytes(), 0o600); err != nil {
				t.Fatal(err)
			}
			installer := installTestInstaller(t, InstallKindISO)
			installer.Options.ImageSize = test.ceiling
			err := installer.extractISOImageGzip(source, target)
			if test.wantError {
				if err == nil {
					t.Fatal("over-limit extraction succeeded")
				}
				want := "output exceeds " + strconv.FormatInt(test.ceiling, 10) + "-byte ceiling"
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("over-limit extraction error=%v, want %q", err, want)
				}
				if _, statErr := os.Stat(target); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("over-limit extraction left target: %v", statErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			extracted, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(extracted, payload) {
				t.Fatalf("extracted=%q want=%q", extracted, payload)
			}
		})
	}
}

// VALIDATES: FAT32 sizing includes 64 MiB slack, a 96 MiB floor, and rounds upward to MiB.
// PREVENTS: a one-byte-over boundary truncating the Ventoy ISO.
func TestVentoyDiskByteBoundaries(t *testing.T) {
	const mib = int64(1 << 20)
	for _, test := range []struct{ iso, want int64 }{{0, 96 * mib}, {32 * mib, 96 * mib}, {32*mib + 1, 97 * mib}, {100*mib + 1, 165 * mib}} {
		if got := ventoyDiskBytes(test.iso); got != test.want {
			t.Fatalf("iso=%d got=%d want=%d", test.iso, got, test.want)
		}
	}
}

// VALIDATES: GPT parsing rejects missing headers and compares the first four absolute entries.
// PREVENTS: partition-count-only proof accepting changed offsets or type GUIDs.
func TestInstallGPTLayoutUsesCompleteEntries(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.img")
	installed := filepath.Join(root, "installed.img")
	writeGPTFixture(t, source, 0)
	writeGPTFixture(t, installed, 0)
	if err := assertInstallGPT(source, installed); err != nil {
		t.Fatal(err)
	}
	writeGPTFixture(t, installed, 1)
	if err := assertInstallGPT(source, installed); err == nil {
		t.Fatal("changed GPT entry was accepted")
	}
	bad := filepath.Join(root, "bad.img")
	if err := os.WriteFile(bad, make([]byte, 4096), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installGPTEntries(bad); err == nil {
		t.Fatal("missing GPT was accepted")
	}
}

func writeGPTFixture(t *testing.T, path string, delta uint64) {
	t.Helper()
	data := make([]byte, 34*512)
	copy(data[512:], "EFI PART")
	binary.LittleEndian.PutUint64(data[512+72:], 2)
	binary.LittleEndian.PutUint32(data[512+80:], 4)
	binary.LittleEndian.PutUint32(data[512+84:], 128)
	for index := range 4 {
		off := 1024 + index*128
		data[off] = byte(index + 1)
		binary.LittleEndian.PutUint64(data[off+32:], uint64(2048+index*100)+delta)
		binary.LittleEndian.PutUint64(data[off+40:], uint64(2147+index*100))
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: serial matching handles prompts without newlines, EOF, and bounded retained output.
// PREVENTS: line-oriented waits hanging on the rescue token prompt.
func TestInstallSerialStateTransitions(t *testing.T) {
	serial := newInstallSerial(strings.NewReader("boot\nrescue token:"))
	ok, err := serial.expect(context.Background(), "rescue token:", time.Second)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v text=%q", ok, err, serial.snapshot())
	}
	missing, err := serial.expect(context.Background(), "never", time.Second)
	if err != nil || missing {
		t.Fatalf("missing=%v err=%v", missing, err)
	}
}

// VALIDATES: every report and action uses explicit pass, skip, fail, and zero-invalid verdict mapping.
// PREVENTS: an unexecuted report exiting successfully.
func TestInstallReportAndExitMapping(t *testing.T) {
	if code := installExitCode(&InstallReport{}); code != 1 {
		t.Fatalf("zero verdict code=%d", code)
	}
	for verdict, want := range map[InstallVerdict]int{InstallVerdictPass: 0, InstallVerdictSkip: 0, InstallVerdictFail: 1} {
		report := InstallReport{Verdict: verdict}
		if got := installExitCode(&report); got != want {
			t.Fatalf("%s code=%d want=%d", verdict, got, want)
		}
	}
	report := InstallReport{Action: "install-test", Verdict: InstallVerdictPass}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"verdict":"pass"`)) {
		t.Fatalf("json=%s", encoded)
	}
}

// VALIDATES: all four installer actions are native and present.
// PREVENTS: one installer workflow disappearing from the action table.
func TestInstallerActionsAreNativeAndComplete(t *testing.T) {
	wanted := map[string]bool{"install-test": false, "install-iso-test": false, "install-scenarios-test": false, "install-ventoy-test": false}
	for _, action := range Actions().Actions {
		if _, ok := wanted[action.Verb]; !ok {
			continue
		}
		wanted[action.Verb] = true
	}
	for verb, found := range wanted {
		if !found {
			t.Errorf("missing %s", verb)
		}
	}
}

// VALIDATES: applianceEnv names the Colima socket on darwin when the client's
// default endpoint is absent, and keeps a DOCKER_HOST the operator already set.
// PREVENTS: `ze appliance build` running Docker with no reachable endpoint, and
// the selection overwriting an operator's remote daemon. The selection's own
// conditions are tested in internal/le/dockerhost.
func TestApplianceEnvNamesTheDockerSocket(t *testing.T) {
	ops := productionInstallOps()
	ops.GOOS = "darwin"
	ops.Home = func() (string, error) { return "/home/me", nil }
	ops.Environ = func() []string { return []string{"PATH=/bin"} }
	// Colima serves its own socket and the client's default is absent, which is
	// the only shape that selects.
	ops.Socket = func(path string) bool { return path == "/home/me/.colima/default/docker.sock" }
	installer := &Installer{ops: ops}
	got := installer.applianceEnv("/work/appliances")
	if !envHas(got, "DOCKER_HOST=unix:///home/me/.colima/default/docker.sock") {
		t.Fatalf("env=%q, want the Colima socket named for `ze appliance build`", got)
	}
	if !envHas(got, "ZE_APPLIANCE_DIR=/work/appliances") {
		t.Fatalf("appliance directory missing: %q", got)
	}

	ops.Environ = func() []string { return []string{"DOCKER_HOST=tcp://docker", "PATH=/bin"} }
	installer = &Installer{ops: ops}
	if got := installer.applianceEnv("/work/appliances"); !envHas(got, "DOCKER_HOST=tcp://docker") {
		t.Fatalf("an operator's daemon was overwritten: %q", got)
	}
}

// VALIDATES: fault and pin serial markers map to the exact scenario branch verdict.
// PREVENTS: presence-only assertions accepting a kernel panic, a touched foreign NIC, or an incomplete install.
func TestInstallScenarioBranches(t *testing.T) {
	faultPass := installFaultScenario(InstallFaultRecoverMark + " " + InstallFatalPolicyMark)
	if faultPass.Verdict != InstallVerdictPass {
		t.Fatalf("fault pass=%+v", faultPass)
	}
	if got := installFaultScenario(InstallKernelPanicMark + " " + InstallFaultRecoverMark + " " + InstallFatalPolicyMark); got.Verdict != InstallVerdictFail {
		t.Fatalf("kernel panic=%+v", got)
	}
	if got := installFaultScenario(InstallFaultRecoverMark); got.Detail != "fatal policy marker absent" {
		t.Fatalf("fault missing fatal=%+v", got)
	}
	ac4 := InstallPinMark + " " + InstallPinnedMAC + " " + InstallPinReachableMark + " " +
		InstallMarkWritten + " " + InstallMarkDone
	if got := installPinScenario("pin-ac4", ac4); got.Verdict != InstallVerdictPass {
		t.Fatalf("pin ac4=%+v", got)
	}
	if got := installPinScenario("pin-ac4", ac4+" "+InstallFallbackMark); got.Detail != "foreign NIC was touched" {
		t.Fatalf("pin ac4 fallback=%+v", got)
	}
	ac5 := InstallPinMark + " " + InstallPinnedMAC + " " + InstallPinFlushMark + " " +
		InstallFallbackMark + " " + InstallMarkWritten + " " + InstallMarkDone
	if got := installPinScenario("pin-ac5", ac5); got.Verdict != InstallVerdictPass {
		t.Fatalf("pin ac5=%+v", got)
	}
	if got := installPinScenario("pin-ac5", strings.ReplaceAll(ac5, InstallPinFlushMark, "")); got.Detail != "pinned NIC was not flushed" {
		t.Fatalf("pin ac5 no flush=%+v", got)
	}

	// The ambiguous-target branch. Both halves of its verdict are asserted: the
	// refusal alone is not enough, because an installer that printed the refusal
	// and then wrote a disk anyway would satisfy a presence-only check.
	if got := installAmbiguousScenario(InstallAmbiguousTargetMark + " (vda vdb)"); got.Verdict != InstallVerdictPass {
		t.Fatalf("ambiguous pass=%+v", got)
	}
	if got := installAmbiguousScenario("installing to /dev/vda"); got.Detail != "installer did not refuse two fixed disks" {
		t.Fatalf("ambiguous silent=%+v", got)
	}
	if got := installAmbiguousScenario(InstallAmbiguousTargetMark + " (vda vdb) " + InstallStreamMark); got.Detail != "installer wrote a disk it had refused to choose" {
		t.Fatalf("ambiguous wrote anyway=%+v", got)
	}
}

// VALIDATES: the base artifact chain executes six build commands and produces exactly initrd, image, and ZeFS payloads.
// PREVENTS: a missing assemble step, duplicate host harness, or absent credential database.
func TestInstallBaseArtifactPlanHasAbsoluteCounts(t *testing.T) {
	installer := installTestInstaller(t, InstallKindHTTP)
	installer.Options.Arch = ArchARM64
	installer.Options.ImageSize = 123456
	work := t.TempDir()
	calls := make([]commandSpec, 0, 6)
	installer.ops.Run = func(_ context.Context, spec commandSpec) (commandResult, error) {
		calls = append(calls, spec)
		if spec.Name == "go" {
			return commandResult{}, nil
		}
		if reflect.DeepEqual(spec.Args, []string{"appliance", "initrd"}) {
			initrd := filepath.Join(work, "initrd.img.gz")
			if err := os.WriteFile(initrd, []byte("initrd"), 0o600); err != nil {
				return commandResult{}, err
			}
			return commandResult{Stdout: "initrd ready: " + initrd + "\n"}, nil
		}
		appDir := filepath.Join(work, "appliances", InstallApplianceName)
		switch {
		case reflect.DeepEqual(spec.Args, []string{"appliance", "init", InstallApplianceName}):
			if err := os.MkdirAll(appDir, 0o750); err != nil {
				return commandResult{}, err
			}
			if err := os.WriteFile(filepath.Join(appDir, "appliance.json"), []byte(`{}`), 0o600); err != nil {
				return commandResult{}, err
			}
		case reflect.DeepEqual(spec.Args, []string{"appliance", "build", InstallApplianceName}):
			if err := os.WriteFile(filepath.Join(appDir, "ze-proof.img"), []byte("image"), 0o600); err != nil {
				return commandResult{}, err
			}
		case reflect.DeepEqual(spec.Args, []string{"appliance", "assemble", "--keep", InstallApplianceName}):
			if err := os.WriteFile(filepath.Join(appDir, "database.zefs"), []byte("zefs"), 0o600); err != nil {
				return commandResult{}, err
			}
		}
		return commandResult{}, nil
	}
	initrd, err := installer.buildInitrd(context.Background(), work)
	if err != nil {
		t.Fatal(err)
	}
	image, err := installer.buildImage(context.Background(), work)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 6 {
		t.Fatalf("commands=%d want 6: %#v", len(calls), calls)
	}
	for _, artifact := range []string{initrd, image.Path, image.ZeFS} {
		if info, statErr := os.Stat(artifact); statErr != nil || !info.Mode().IsRegular() {
			t.Fatalf("artifact %q: %v", artifact, statErr)
		}
	}
	configData, err := os.ReadFile(filepath.Join(work, "appliances", InstallApplianceName, "appliance.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Image struct {
			Arch string `json:"arch"`
			Size int64  `json:"size-bytes"`
		} `json:"image"`
	}
	if err := json.Unmarshal(configData, &config); err != nil {
		t.Fatal(err)
	}
	if config.Image.Arch != ArchARM64 || config.Image.Size != 123456 {
		t.Fatalf("target config=%+v", config.Image)
	}
}

type installCleanupFS struct {
	osRunFS
	made    string
	removed string
}

func (filesystem *installCleanupFS) MkdirTemp(dir, pattern string) (string, error) {
	path, err := os.MkdirTemp(dir, pattern)
	filesystem.made = path
	return path, err
}

func (filesystem *installCleanupFS) RemoveAll(path string) error {
	filesystem.removed = path
	return os.RemoveAll(path)
}

// VALIDATES: an operating failure after work-directory acquisition removes the whole fault tree.
// PREVENTS: failed builds leaking cached credentials, images, or live process artifacts.
func TestInstallOperatingFailureCleansUp(t *testing.T) {
	installer := installTestInstaller(t, InstallKindHTTP)
	kernel := filepath.Join(installer.Tree, "kernel")
	if err := os.WriteFile(kernel, []byte("kernel"), 0o600); err != nil {
		t.Fatal(err)
	}
	installer.Options.Kernel = kernel
	filesystem := &installCleanupFS{}
	installer.ops.FS = filesystem
	installer.ops.Run = func(context.Context, commandSpec) (commandResult, error) {
		return commandResult{Code: 1, Stderr: "build failed"}, nil
	}
	_, err := installer.Execute(context.Background())
	if err == nil {
		t.Fatal("operating failure returned no error")
	}
	if filesystem.made == "" || filesystem.removed != filesystem.made {
		t.Fatalf("made=%q removed=%q", filesystem.made, filesystem.removed)
	}
	if _, statErr := os.Stat(filesystem.made); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("work directory remains: %v", statErr)
	}
}

// VALIDATES: ZE_INSTALL_KEEP retains the same work tree that cleanup otherwise removes.
// PREVENTS: a failure report naming artifacts that cleanup already deleted.
func TestInstallKeepRetainsFailureArtifacts(t *testing.T) {
	installer := installTestInstaller(t, InstallKindHTTP)
	kernel := filepath.Join(installer.Tree, "kernel")
	if err := os.WriteFile(kernel, []byte("kernel"), 0o600); err != nil {
		t.Fatal(err)
	}
	installer.Options.Kernel = kernel
	installer.Options.Keep = true
	filesystem := &installCleanupFS{}
	installer.ops.FS = filesystem
	installer.ops.Run = func(context.Context, commandSpec) (commandResult, error) {
		return commandResult{Code: 1, Stderr: "build failed"}, nil
	}
	report, err := installer.Execute(context.Background())
	if err == nil {
		t.Fatal("operating failure returned no error")
	}
	if report.Retained != filesystem.made || filesystem.removed != "" {
		t.Fatalf("retained=%q made=%q removed=%q", report.Retained, filesystem.made, filesystem.removed)
	}
	if err := os.RemoveAll(filesystem.made); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: rescue branches carry the pinned vector, source-specific failure, bounded wait, and no-reboot policy.
// PREVENTS: copying rescue logic away from the base runner or dropping one branch input.
func TestInstallRescueScenarioArgv(t *testing.T) {
	installer := installTestInstaller(t, InstallKindScenarios)
	installer.Options.Arch, installer.Options.Kernel = ArchAMD64, "kernel"
	withAuth, err := installer.RescueArgv("initrd", "http", true)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(withAuth, " ")
	for _, want := range []string{"-no-reboot", "ze.source=http", "ze.server=10.0.2.99", "ze.wait=3", "ze.rescue-auth=" + InstallRescueAuth} {
		if !strings.Contains(joined, want) {
			t.Fatalf("argv misses %q: %s", want, joined)
		}
	}
	withoutAuth, err := installer.RescueArgv("initrd", "iso", false)
	if err != nil {
		t.Fatal(err)
	}
	joined = strings.Join(withoutAuth, " ")
	if !strings.Contains(joined, "ze.media-id="+InstallDummyMediaID) || strings.Contains(joined, "ze.rescue-auth") {
		t.Fatalf("ISO rescue argv=%s", joined)
	}
}

// VALIDATES: a missing optional host prerequisite returns the exact self-skip line and exit-zero verdict.
// PREVENTS: missing QEMU becoming an operating failure.
func TestInstallSelfSkip(t *testing.T) {
	installer := installTestInstaller(t, InstallKindHTTP)
	installer.ops.Look = func(name string) (string, error) {
		if strings.HasPrefix(name, "qemu-system-") {
			return "", errors.New("missing")
		}
		return name, nil
	}
	report, err := installer.Execute(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != InstallVerdictSkip || report.Text() != "INSTALL-QEMU: SKIP qemu-system-x86_64 not found\n" {
		t.Fatalf("report=%+v text=%q", report, report.Text())
	}
}

func installTestInstaller(t *testing.T, kind InstallKind) *Installer {
	t.Helper()
	options := InstallOptions{Kind: kind, Arch: ArchAMD64, Accelerator: "tcg", Kernel: "kernel", SSHUser: installDefaultSSHUser, SSHPassword: installDefaultSSHPass, NIC: installDefaultNIC, BootTimeout: time.Second, FaultTimeout: time.Second, RescueStepTimeout: time.Second, RescueTimeout: time.Second}
	installer := NewInstaller(t.TempDir(), options)
	installer.ops.GOOS, installer.ops.GOARCH = "linux", ArchAMD64
	installer.ops.Access = func(string, uint32) bool { return false }
	installer.ops.Environ = func() []string { return []string{"PATH=/bin"} }
	installer.ops.Look = func(name string) (string, error) { return name, nil }
	return installer
}
