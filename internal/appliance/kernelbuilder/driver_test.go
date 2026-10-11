package kernelbuilder

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/tmplink"
)

func TestSelectBuilder(t *testing.T) {
	old := commandAvailable
	oldCheck := acceleratorCheck
	t.Cleanup(func() { commandAvailable, acceleratorCheck = old, oldCheck })
	acceleratorCheck = func(context.Context, string) (string, error) { return "kvm", nil }

	tests := []struct {
		name      string
		requested string
		available map[string]bool
		want      string
		wantError string
	}{
		{name: "explicit docker", requested: "docker", available: map[string]bool{"docker": true}, want: "docker"},
		{name: "explicit qemu", requested: "qemu", available: map[string]bool{"qemu-system-aarch64": true, "go": true}, want: "qemu"},
		{name: "auto prefers docker", available: map[string]bool{"docker": true, "qemu-system-aarch64": true, "go": true}, want: "docker"},
		{name: "auto falls back qemu", available: map[string]bool{"qemu-system-aarch64": true, "go": true}, want: "qemu"},
		{name: "docker unavailable", requested: "docker", wantError: "docker builder requested"},
		{name: "qemu unavailable", requested: "qemu", wantError: "qemu builder requested"},
		{name: "none available", wantError: "no builder available"},
		{name: "bad token", requested: "podman", wantError: "unsupported builder"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			commandAvailable = func(name string) bool { return test.available[name] }
			got, err := SelectBuilder(context.Background(), test.requested, "arm64")
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("SelectBuilder error = %v, want %q", err, test.wantError)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("SelectBuilder = %q, %v, want %q", got, err, test.want)
			}
		})
	}
}

func TestValidateRequestNormalizesAarch64(t *testing.T) {
	hostIs(t, "arm64")
	root := t.TempDir()
	writeFixture(t, root, "configs/kernel.config", "CONFIG_A=y\n")
	if err := os.MkdirAll(filepath.Join(root, "out"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"tools/kernel-builder", "common"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	req := Request{
		Root: root, Version: "7.1.1", Arch: "aarch64", Profile: "qemu",
		SourceDir: "configs", OutputDir: "out", BuilderDir: "tools/kernel-builder",
		CommonDir: "common", Modules: "no", Fragments: []string{"configs/kernel.config"},
	}
	if err := validateRequest(&req); err != nil {
		t.Fatal(err)
	}
	if req.Arch != "arm64" {
		t.Fatalf("normalized arch = %q, want arm64", req.Arch)
	}
}

func TestValidateRequestAcceptsAbsoluteOutputAndRejectsUnsafePaths(t *testing.T) {
	hostIs(t, "amd64")
	root := t.TempDir()
	writeFixture(t, root, "configs/kernel.config", "CONFIG_A=y\n")
	for _, dir := range []string{"tools/kernel-builder", "common"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	absolute := filepath.Join(t.TempDir(), "nested", "..", "out")
	req := Request{
		Root: root, Version: "7.1.1", Arch: "amd64", Profile: "qemu",
		SourceDir: "configs", OutputDir: absolute, BuilderDir: "tools/kernel-builder",
		CommonDir: "common", Modules: "no", Fragments: []string{"configs/kernel.config"},
	}
	if err := validateRequest(&req); err != nil {
		t.Fatalf("absolute output rejected: %v", err)
	}
	if req.OutputDir != filepath.Clean(absolute) {
		t.Fatalf("cleaned output = %q, want %q", req.OutputDir, filepath.Clean(absolute))
	}
	if got := hostOutputPath(req); got != filepath.Clean(absolute) {
		t.Fatalf("host output = %q, want %q", got, filepath.Clean(absolute))
	}

	for _, output := range []string{"", ".", string(filepath.Separator), "../out"} {
		t.Run(output, func(t *testing.T) {
			candidate := req
			candidate.OutputDir = output
			if err := validateRequest(&candidate); err == nil {
				t.Fatalf("unsafe output %q accepted", output)
			}
		})
	}
	candidate := req
	candidate.SourceDir = "../configs"
	candidate.OutputDir = "out"
	if err := validateRequest(&candidate); err == nil || !strings.Contains(err.Error(), "path escapes repository") {
		t.Fatalf("escaping source path returned %v", err)
	}
}

func TestRunDockerArgvAndOwnershipRepair(t *testing.T) {
	hostIs(t, "amd64")
	root := t.TempDir()
	writeFixture(t, root, "configs/kernel.config", "CONFIG_A=y\n")
	writeFixture(t, root, "configs/kernel.require", "CONFIG_A\n")
	writeFixture(t, root, "common/console.config", "CONFIG_B=y\n")
	writeFixture(t, root, "common/console.require", "CONFIG_B\n")
	writeFixture(t, root, "tools/kernel-builder/Dockerfile", "FROM scratch\n")
	req := Request{Root: root, Version: "7.1.1", Arch: "amd64", Profile: "qemu", Builder: "docker", Target: "installer", SourceDir: "configs", OutputDir: "out", BuilderDir: "tools/kernel-builder", CommonDir: "common", Modules: "no", Fragments: []string{"configs/kernel.config", "common/console.config"}, Image: "fixture", Stdout: os.Stdout, Stderr: os.Stderr}
	if err := validateRequest(&req); err != nil {
		t.Fatal(err)
	}
	// No container store on this host, so the space guard reports that it did
	// not run and the reclaim step is skipped. Both are covered on their own in
	// space_test.go; pinning them here keeps this test's call count a fact about
	// runDocker rather than about the machine it runs on.
	pinHostWithoutAContainerStore(t)
	old := runCommand
	t.Cleanup(func() { runCommand = old })
	var calls [][]string
	runCommand = func(_ context.Context, _ Request, name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}
	if err := runDocker(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 {
		t.Fatalf("docker calls = %v", calls)
	}
	// runDocker runs the recipe's argv and nothing it built itself: the
	// recipe is what the cache key hashes (recipe.go, BuildSources).
	runArgs, err := containerRunArgs(req)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(calls[0], append([]string{"docker"}, containerBuildArgs(req)...)) {
		t.Errorf("docker build argv = %v, want containerBuildArgs %v", calls[0], containerBuildArgs(req))
	}
	if !slices.Equal(calls[1], append([]string{"docker"}, runArgs...)) {
		t.Errorf("docker run argv = %v, want containerRunArgs %v", calls[1], runArgs)
	}
	build := strings.Join(calls[0], " ")
	if !strings.Contains(build, "docker build --platform linux/amd64 -t fixture -f tools/kernel-builder/Dockerfile "+root) {
		t.Fatalf("docker build argv = %s", build)
	}
	run := strings.Join(calls[1], " ")
	for _, want := range []string{"ze-kernel-builder --version 7.1.1", "--fragment /src/kernel.config", "--fragment /builder/common/console.config", "ze-kernel-build:/tmp/kbuild", "ze-kernel-work:/build"} {
		if !strings.Contains(run, want) {
			t.Errorf("docker run missing %q: %s", want, run)
		}
	}
	repair := strings.Join(calls[2], " ")
	if !strings.Contains(repair, "chown -R") || !strings.HasSuffix(repair, " /out") {
		t.Fatalf("ownership repair argv = %s", repair)
	}
}

func TestRunDockerRepairsOwnershipAfterFailure(t *testing.T) {
	root := t.TempDir()
	req := Request{Root: root, Version: "7.1.1", Arch: "amd64", Profile: "qemu", Target: "installer", SourceDir: "configs", OutputDir: "out", BuilderDir: "tools/kernel-builder", CommonDir: "common", Modules: "no", Fragments: []string{"configs/kernel.config"}, Image: "fixture", Stdout: os.Stdout, Stderr: os.Stderr}
	pinHostWithoutAContainerStore(t)
	old := runCommand
	t.Cleanup(func() { runCommand = old })
	calls := 0
	ctx, cancel := context.WithCancel(context.Background())
	repairRanAfterCancel := false
	runCommand = func(callCtx context.Context, _ Request, _ string, args ...string) error {
		calls++
		if calls == 2 {
			cancel()
			return errors.New("build failed")
		}
		if calls == 3 {
			repairRanAfterCancel = callCtx.Err() == nil
		}
		return nil
	}
	// The build was canceled while docker run failed, so the stop is the
	// answer, not the step's own error (round-8 ISSUE 1).
	err := runDocker(ctx, req)
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "the kernel container was stopped") {
		t.Fatalf("runDocker error = %v", err)
	}
	if calls != 3 || !repairRanAfterCancel {
		t.Fatalf("ownership repair was skipped after cancellation, calls = %d", calls)
	}
}

func TestWriteProvenanceBytes(t *testing.T) {
	root := t.TempDir()
	req := Request{Root: root, Version: "7.2.9", Arch: "arm64", Profile: "hardware", Target: "runtime", Modules: "yes"}
	if err := writeProvenance("out", req, "qemu"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "out", ProvenanceName))
	if err != nil {
		t.Fatal(err)
	}
	want := "version=7.2.9\ntarget=runtime\nprofile=hardware\narch=arm64\nmodules=yes\nbuilder=qemu\n" +
		"source-url=https://cdn.kernel.org/pub/linux/kernel/v7.x/linux-7.2.9.tar.xz\n" +
		"source-sha256=b4c5dfbe51a364a6c7f03869200f88c8e1f77403539005f14b7fc6bc91b8d8ba\n"
	if string(data) != want {
		t.Fatalf("provenance = %q, want %q", data, want)
	}
}

// VALIDATES: the provenance names the source the worker verified: the URL
// downloadKernelSource fetches and the tracked digest verifyKernelSource checks.
// PREVENTS: a provenance whose source lines are typed by hand and drift from the
// declaration the build uses.
func TestWriteProvenanceSourceIsTheVerifiedOne(t *testing.T) {
	root := t.TempDir()
	req := Request{Root: root, Version: "7.2.9", Arch: "amd64", Profile: "runtime", Target: "runtime", Modules: "yes"}
	if err := writeProvenance("out", req, "docker"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "out", ProvenanceName))
	if err != nil {
		t.Fatal(err)
	}
	digest, tracked := SourceDigest(req.Version)
	if !tracked {
		t.Fatalf("7.2.9 has no tracked digest")
	}
	for _, line := range []string{"source-url=" + kernelTarballURL(req.Version) + "\n", "source-sha256=" + digest + "\n"} {
		if !strings.Contains(string(data), line) {
			t.Errorf("provenance %q does not carry %q", data, line)
		}
	}
}

// VALIDATES: a version with no tracked digest gets no provenance.
// PREVENTS: a provenance that records an empty digest, which reads as a source
// nobody verified.
func TestWriteProvenanceRefusesUntrackedSource(t *testing.T) {
	root := t.TempDir()
	req := Request{Root: root, Version: "7.2.3", Arch: "arm64", Profile: "hardware", Target: "runtime", Modules: "yes"}
	err := writeProvenance("out", req, "qemu")
	if err == nil {
		t.Fatal("provenance written for 7.2.3, which has no tracked SHA-256")
	}
	if !strings.Contains(err.Error(), "7.2.3") {
		t.Errorf("refusal %q does not name the version", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "out", ProvenanceName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("a refused provenance left a file behind: %v", statErr)
	}
}

func TestEnsureAlpineISOChecksumAndCacheInvalidation(t *testing.T) {
	payload := []byte("verified alpine iso")
	sum := sha256.Sum256(payload)
	checksum := hex.EncodeToString(sum[:])
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests++
		if strings.HasSuffix(request.URL.Path, ".sha256") {
			_, _ = fmt.Fprintf(w, "%s  image.iso\n", checksum)
			return
		}
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	old := alpineReleaseBaseURL
	alpineReleaseBaseURL = server.URL
	t.Cleanup(func() { alpineReleaseBaseURL = old })
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	path, err := ensureAlpineISO(context.Background(), "x86_64")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, payload) {
		t.Fatalf("ISO = %q, %v", data, err)
	}
	if requests != 2 {
		t.Fatalf("download requests = %d, want 2", requests)
	}
	if _, err := ensureAlpineISO(context.Background(), "x86_64"); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("verified cache downloaded again, requests = %d", requests)
	}
	if err := os.WriteFile(path, []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureAlpineISO(context.Background(), "x86_64"); err != nil {
		t.Fatal(err)
	}
	if requests != 4 {
		t.Fatalf("corrupt cache was not invalidated, requests = %d", requests)
	}
}

func TestQEMUArgsLifecycleMounts(t *testing.T) {
	root := t.TempDir()
	fakeBin := filepath.Join(root, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	qemu := filepath.Join(fakeBin, "qemu-system-x86_64")
	if err := os.WriteFile(qemu, []byte("#!/bin/sh\nprintf 'kvm\\ntcg\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	req := Request{Root: root, Arch: "amd64", OutputDir: filepath.Join(root, "external-output"), FirmwareDir: filepath.Join(root, "firmware")}
	args, err := qemuArgs(req, "alpine.iso", "kvm", 22022, 9216, filepath.Join(root, "ccache"), filepath.Join(root, "build"), nil)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"-accel kvm", "hostfwd=tcp::22022-:22", "mount_tag=workspace", "mount_tag=ccache", "mount_tag=builddir", "mount_tag=output", "mount_tag=firmware", "readonly=on"} {
		if !strings.Contains(joined, want) {
			t.Errorf("QEMU argv missing %q: %s", want, joined)
		}
	}
	if strings.Contains(joined, "tcg") {
		t.Errorf("QEMU argv offers the tcg emulator beside kvm: %s", joined)
	}
}

// hostIs makes the build host report arch for the rest of the test.
func hostIs(t *testing.T, arch string) {
	t.Helper()
	old := hostGOARCH
	hostGOARCH = arch
	t.Cleanup(func() { hostGOARCH = old })
}

// TestValidateRequestRefusesForeignArch verifies a kernel for an arch other
// than the host's is refused before anything runs, whatever spelling the
// request used, naming the host arch, the requested arch and where to build it.
//
// VALIDATES: AC-13, a kernel is built only natively.
// PREVENTS: a kernel built under Docker --platform emulation or QEMU tcg, which
// takes hours and fails late.
func TestValidateRequestRefusesForeignArch(t *testing.T) {
	for _, tc := range []struct{ host, requested, want string }{
		{host: "amd64", requested: "arm64", want: "arm64"},
		{host: "amd64", requested: "aarch64", want: "arm64"},
		{host: "arm64", requested: "amd64", want: "amd64"},
	} {
		t.Run(tc.host+"-builds-"+tc.requested, func(t *testing.T) {
			hostIs(t, tc.host)
			req := Request{Root: t.TempDir(), Version: "7.1.1", Arch: tc.requested, Profile: "qemu", Builder: "docker"}
			err := validateRequest(&req)
			if err == nil {
				t.Fatalf("a %s kernel was accepted on a %s host", tc.requested, tc.host)
			}
			for _, want := range []string{"the host is " + tc.host, "kernel for " + tc.want, "build it on an " + tc.want + " host"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal lacks %q: %v", want, err)
				}
			}
		})
	}
}

// TestQEMUArgsRefusesWithoutHardwareAccelerator verifies the QEMU backend
// refuses when neither hvf nor kvm serves, rather than emulating under tcg.
//
// VALIDATES: AC-13, never QEMU under tcg.
// PREVENTS: a kernel build that runs for hours in an emulator.
func TestQEMUArgsRefusesWithoutHardwareAccelerator(t *testing.T) {
	root := t.TempDir()
	fakeBin := filepath.Join(root, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	qemu := filepath.Join(fakeBin, "qemu-system-x86_64")
	if err := os.WriteFile(qemu, []byte("#!/bin/sh\nprintf 'tcg\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, err := hardwareAccelerator(context.Background(), "qemu-system-x86_64")
	if err == nil {
		t.Fatal("hardwareAccelerator accepted a QEMU that offers only tcg")
	}
	for _, want := range []string{"hvf", "kvm", "tcg"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q: %v", want, err)
		}
	}
}

// TestQEMUArgsNamesABrokenQEMU verifies that a QEMU binary which cannot list
// its accelerators is reported as that failure, with its exit status and its
// stderr, and not as a host that lacks hvf and kvm.
//
// VALIDATES: availableAccelerators returns the exec error; hardwareAccelerator names it.
// PREVENTS: a missing or crashing QEMU read as "offers neither hvf nor kvm",
// sending the operator to enable an accelerator the host already has.
func TestQEMUArgsNamesABrokenQEMU(t *testing.T) {
	root := t.TempDir()
	fakeBin := filepath.Join(root, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	qemu := filepath.Join(fakeBin, "qemu-system-x86_64")
	if err := os.WriteFile(qemu, []byte("#!/bin/sh\necho 'qemu: library not loaded: libglib' >&2\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, err := hardwareAccelerator(context.Background(), "qemu-system-x86_64")
	if err == nil {
		t.Fatal("hardwareAccelerator accepted a QEMU that cannot list its accelerators")
	}
	if strings.Contains(err.Error(), "offers neither") {
		t.Errorf("a broken QEMU is reported as a missing accelerator: %v", err)
	}
	for _, want := range []string{"qemu-system-x86_64 -accel help", "exit status 3", "library not loaded: libglib"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not name %q: %v", want, err)
		}
	}
}

// VALIDATES: a real checkout tmp whose qemu child is a symlink out of the tree
// gets that child's target exported and mounted at its own absolute path
// before the worker under /workspace/tmp/qemu runs, and the kernel child the
// output lands under gets the same.
// PREVENTS: `sh: /workspace/tmp/qemu/bin/amd64/ze-kernel-builder: not found`,
// observed on 2026-09-15 after `le scratch migrate` relocated tmp/qemu.
func TestARealTmpWithSymlinkedChildrenIsSharedWithTheGuest(t *testing.T) {
	root := t.TempDir()
	scratch := t.TempDir()
	tmp := filepath.Join(root, "tmp")
	if err := os.MkdirAll(tmp, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"qemu", "kernel"} {
		if err := os.MkdirAll(filepath.Join(scratch, name), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(scratch, name), filepath.Join(tmp, name)); err != nil {
			t.Fatal(err)
		}
	}
	shares, err := tmplink.Shares(root, "/workspace")
	if err != nil {
		t.Fatal(err)
	}
	fakeBin := filepath.Join(root, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	qemu := filepath.Join(fakeBin, "qemu-system-x86_64")
	if err := os.WriteFile(qemu, []byte("#!/bin/sh\nprintf 'kvm\\ntcg\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	req := Request{Root: root, Arch: "amd64", OutputDir: filepath.Join("tmp", "kernel", "build")}
	args, err := qemuArgs(req, "alpine.iso", "kvm", 22022, 9216, filepath.Join(root, "ccache"), filepath.Join(root, "build"), shares)
	if err != nil {
		t.Fatal(err)
	}
	setup, guestOutput := guestSetup(req, shares)
	if guestOutput != "/workspace/tmp/kernel/build" {
		t.Errorf("guest output = %q", guestOutput)
	}
	for _, name := range []string{"qemu", "kernel"} {
		resolved, err := filepath.EvalSymlinks(filepath.Join(scratch, name))
		if err != nil {
			t.Fatal(err)
		}
		export := "local,path=" + resolved + ",mount_tag=zescratch-" + name + ",security_model=none,id=zescratch-" + name + ",readonly=off"
		if !slices.Contains(args, export) {
			t.Errorf("QEMU argv misses %q:\n%s", export, strings.Join(args, "\n"))
		}
		guest := filepath.Join(scratch, name)
		mount := "mkdir -p " + shellQuote(guest) + " && mount -t 9p -o trans=virtio,version=9p2000.L,msize=1048576 zescratch-" + name + " " + shellQuote(guest)
		if !strings.Contains(setup, mount) {
			t.Errorf("guest setup misses %q:\n%s", mount, setup)
		}
	}
}

func TestWorkerValidationAndRequiredSymbols(t *testing.T) {
	for _, version := range []string{"", ".7", "7.", "7.x", "6.12.9"} {
		if validateVersion(version) == nil {
			t.Errorf("ValidateVersion(%q) accepted", version)
		}
	}
	if err := validateVersion("7..1"); err != nil {
		t.Fatalf("compatible version rejected: %v", err)
	}
	for _, profile := range []string{"", "UPPER", "../bad", "bad_name"} {
		if validateProfile(profile) == nil {
			t.Errorf("ValidateProfile(%q) accepted", profile)
		}
	}
	if err := validateProfile("7series"); err != nil {
		t.Fatalf("digit-leading profile rejected: %v", err)
	}
	if validateJobs("12x") == nil {
		t.Fatal("nonnumeric jobs accepted")
	}

	root := t.TempDir()
	fragment := filepath.Join(root, "qemu.config")
	writeFixture(t, root, "qemu.config", "CONFIG_A=y\n")
	writeFixture(t, root, "qemu.require", "CONFIG_A\nCONFIG_B=y\n")
	build := filepath.Join(root, "build")
	writeFixture(t, root, "build/.config", "CONFIG_A=y\nCONFIG_B=y\n")
	req := WorkerRequest{Profile: "qemu", Fragments: []string{fragment}}
	if err := enforceRequiredSymbols(req, build); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "build/.config", "CONFIG_A=y\n")
	if err := enforceRequiredSymbols(req, build); err == nil || !strings.Contains(err.Error(), "CONFIG_B") {
		t.Fatalf("missing symbol error = %v", err)
	}
}

func TestExtractTarRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "bad.tar")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := tar.NewWriter(file)
	if err := writer.WriteHeader(&tar.Header{Name: "../escape", Mode: 0o644, Size: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(writer.Close(), file.Close()); err != nil {
		t.Fatal(err)
	}
	req := WorkerRequest{Stdout: io.Discard, Stderr: io.Discard}
	err = extractTar(context.Background(), req, archive, filepath.Join(root, "dest"), false)
	if err == nil || !strings.Contains(err.Error(), "unsafe path") {
		t.Fatalf("extractTar error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "escape")); !os.IsNotExist(err) {
		t.Fatalf("traversal path created: %v", err)
	}
}

func TestContainerFragmentPathRejectsEscape(t *testing.T) {
	req := Request{SourceDir: "src", CommonDir: "common"}

	if got, err := containerFragmentPath(req, "src/base.config"); err != nil || got != "/src/base.config" {
		t.Fatalf("source mapping = %q, %v", got, err)
	}
	if got, err := containerFragmentPath(req, "common/shared.config"); err != nil || got != "/builder/common/shared.config" {
		t.Fatalf("common mapping = %q, %v", got, err)
	}
	if _, err := containerFragmentPath(req, "elsewhere/bad.config"); err == nil {
		t.Fatal("outside fragment accepted")
	}
}
func TestCopyRuntimeOutputs(t *testing.T) {
	root := t.TempDir()
	build := filepath.Join(root, "build")
	output := filepath.Join(root, "out")
	writeFixture(t, root, "build/arch/x86/boot/bzImage", "kernel")
	writeFixture(t, root, "build/overlays/console.dtbo", "overlay")
	old := runWorkerCommand
	t.Cleanup(func() { runWorkerCommand = old })
	runWorkerCommand = func(_ context.Context, _ WorkerRequest, _ string, name string, args ...string) error {
		if name != "make" || !strings.Contains(strings.Join(args, " "), "modules_install") {
			return fmt.Errorf("unexpected worker command: %s %v", name, args)
		}
		modules := filepath.Join(output, "lib", "modules", "7.1.1")
		if err := os.MkdirAll(modules, 0o755); err != nil {
			return err
		}
		if err := os.Symlink(build, filepath.Join(modules, "build")); err != nil {
			return err
		}
		return os.Symlink(build, filepath.Join(modules, "source"))
	}
	req := WorkerRequest{OutputDir: output, Stdout: io.Discard, Stderr: io.Discard}
	if err := copyRuntimeOutputs(context.Background(), req, build, "x86_64", "arch/x86/boot/bzImage"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(output, "vmlinuz"), filepath.Join(output, "overlays", "console.dtbo")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing runtime output %s: %v", path, err)
		}
	}
	for _, name := range []string{"build", "source"} {
		if _, err := os.Lstat(filepath.Join(output, "lib", "modules", "7.1.1", name)); !os.IsNotExist(err) {
			t.Fatalf("module link %s survived: %v", name, err)
		}
	}
}

func TestKernelSourceURL(t *testing.T) {
	if got, want := kernelTarballURL("7.4.2"), "https://cdn.kernel.org/pub/linux/kernel/v7.x/linux-7.4.2.tar.xz"; got != want {
		t.Fatalf("kernelTarballURL = %q, want %q", got, want)
	}
	if got := workerGOARCH("x86_64"); !reflect.DeepEqual(got, "amd64") {
		t.Fatalf("worker GOARCH = %q", got)
	}
}

func writeFixture(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fakeQEMU puts a qemu-system-x86_64 on PATH that runs script.
func fakeQEMU(t *testing.T, script string) {
	t.Helper()
	fakeBin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeBin, "qemu-system-x86_64"), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TestHardwareAcceleratorRefusesAListedButUnusableKVM verifies that a QEMU
// which lists kvm on a host with no usable /dev/kvm is refused, naming the
// device and the fix, rather than being started with -accel kvm.
//
// VALIDATES: round-1 ISSUE 1, the guard probes usability, not the listing.
// PREVENTS: QEMU exiting at once and the operator reading "timeout waiting for
// VM login prompt" two minutes later, with the KVM cause discarded.
func TestHardwareAcceleratorRefusesAListedButUnusableKVM(t *testing.T) {
	fakeQEMU(t, "printf 'kvm\\ntcg\\n'\n")
	old := kvmDevice
	t.Cleanup(func() { kvmDevice = old })
	kvmDevice = filepath.Join(t.TempDir(), "kvm")

	accel, err := hardwareAccelerator(context.Background(), "qemu-system-x86_64")
	if err == nil {
		t.Fatalf("hardwareAccelerator chose %q on a host with no %s", accel, kvmDevice)
	}
	for _, want := range []string{"lists kvm", kvmDevice, "does not exist", "modprobe kvm"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q: %v", want, err)
		}
	}

	if err := os.WriteFile(kvmDevice, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	accel, err = hardwareAccelerator(context.Background(), "qemu-system-x86_64")
	if err != nil || accel != "kvm" {
		t.Fatalf("with an openable %s: accel = %q, err = %v, want kvm", kvmDevice, accel, err)
	}
}

// TestSelectBuilderRefusesQEMUThatCannotRun verifies that with no Docker, a
// QEMU whose accelerator cannot run is refused when the builder is chosen,
// which is also the answer `ze doctor` reports.
//
// VALIDATES: round-1 ISSUE 1 and NOTE 5, the refusal comes before any download.
// PREVENTS: a doctor that passes, and a build that downloads the Alpine ISO,
// on a host where QEMU cannot start a VM.
func TestSelectBuilderRefusesQEMUThatCannotRun(t *testing.T) {
	old := commandAvailable
	oldProbes := acceleratorProbes
	t.Cleanup(func() { commandAvailable, acceleratorProbes = old, oldProbes })
	commandAvailable = func(name string) bool { return name == "qemu-system-x86_64" || name == "go" }
	fakeQEMU(t, "printf 'kvm\\ntcg\\n'\n")
	acceleratorProbes = map[string]func(context.Context) error{
		"kvm": func(context.Context) error { return errors.New("/dev/kvm cannot be opened") },
	}
	for _, requested := range []string{"", "qemu"} {
		_, err := SelectBuilder(context.Background(), requested, "amd64")
		if err == nil {
			t.Fatalf("builder %q chosen on a host whose kvm cannot be opened", requested)
		}
		if !strings.Contains(err.Error(), "/dev/kvm cannot be opened") {
			t.Errorf("builder %q refusal does not carry the accelerator cause: %v", requested, err)
		}
	}
}

// TestBootFailureCarriesQEMUStderr verifies that a QEMU which exits before the
// guest boots is reported with what it wrote to stderr.
//
// VALIDATES: round-1 ISSUE 1, QEMU stderr is kept on an early exit.
// PREVENTS: "timeout waiting for VM login prompt" with QEMU's reason discarded.
func TestBootFailureCarriesQEMUStderr(t *testing.T) {
	fakeQEMU(t, "echo 'qemu-system-x86_64: failed to initialize kvm: Permission denied' >&2\nexit 1\n")
	root := t.TempDir()
	req := Request{Root: root, Arch: "amd64", OutputDir: "out", Stderr: os.Stderr}
	err := runQEMUBuild(context.Background(), req, "alpine.iso", "worker", "kvm")
	if err == nil {
		t.Fatal("a QEMU that exited at once was reported as a build")
	}
	if !strings.Contains(err.Error(), "failed to initialize kvm: Permission denied") {
		t.Errorf("error does not carry QEMU's stderr: %v", err)
	}
}

// TestBootFailureReadsStderrQEMUWritesAsItDies verifies that the reason a QEMU
// writes as it exits reaches the refusal even when it closes its console first:
// stderr is read only after Wait, which is when exec has copied all of it.
//
// VALIDATES: round-2 ISSUE 1, the stderr tail is read after the process ends.
// PREVENTS: "wrote nothing to stderr" for a QEMU that said exactly why it died.
func TestBootFailureReadsStderrQEMUWritesAsItDies(t *testing.T) {
	fakeQEMU(t, "exec 1>&-\nsleep 0.3\necho 'qemu-system-x86_64: Could not access KVM kernel module' >&2\nexit 1\n")
	req := Request{Root: t.TempDir(), Arch: "amd64", OutputDir: "out", Stderr: os.Stderr}
	err := runQEMUBuild(context.Background(), req, "alpine.iso", "worker", "kvm")
	if err == nil {
		t.Fatal("a QEMU that exited at once was reported as a build")
	}
	if !strings.Contains(err.Error(), "Could not access KVM kernel module") {
		t.Errorf("error lost what QEMU wrote as it died: %v", err)
	}
}

// TestBootFailureWithSilentQEMUNamesTheCommand verifies that a QEMU which says
// nothing is refused with the exact QEMU command line on its own line.
//
// VALIDATES: round-2 ISSUE 3, the refusal carries a command, not "run again".
func TestBootFailureWithSilentQEMUNamesTheCommand(t *testing.T) {
	fakeQEMU(t, "exec 1>&-\nexit 0\n")
	req := Request{Root: t.TempDir(), Arch: "amd64", OutputDir: "out", Stderr: os.Stderr}
	err := runQEMUBuild(context.Background(), req, "alpine.iso", "worker", "kvm")
	if err == nil {
		t.Fatal("a QEMU that exited at once was reported as a build")
	}
	lines := strings.Split(err.Error(), "\n")
	last := lines[len(lines)-1]
	if len(lines) < 2 || !strings.HasPrefix(last, "qemu-system-x86_64 ") || !strings.Contains(last, "'-accel' 'kvm'") || !strings.Contains(last, "'alpine.iso'") {
		t.Errorf("refusal does not end with the QEMU command on its own line:\n%v", err)
	}
	if strings.Contains(err.Error(), "run the build again") {
		t.Errorf("refusal still says to run the build again:\n%v", err)
	}
}

// TestProbeKVMGivesTheGroupFixOnlyForPermission verifies the kvm-group fix is
// offered for a permission refusal and not for another open failure.
//
// VALIDATES: round-2 NOTE 4.
func TestProbeKVMGivesTheGroupFixOnlyForPermission(t *testing.T) {
	old := kvmDevice
	t.Cleanup(func() { kvmDevice = old })

	kvmDevice = t.TempDir() // a directory: opening it read-write fails, not for permission
	err := probeKVM(context.Background())
	if err == nil || strings.Contains(err.Error(), "usermod") || !strings.Contains(err.Error(), "dmesg") {
		t.Errorf("non-permission failure: want the kernel log fix and no usermod, got %v", err)
	}

	if os.Geteuid() == 0 {
		t.Skip("root opens a mode 000 file, so the permission case cannot be played")
	}
	kvmDevice = filepath.Join(t.TempDir(), "kvm")
	if err := os.WriteFile(kvmDevice, nil, 0o000); err != nil {
		t.Fatal(err)
	}
	err = probeKVM(context.Background())
	if err == nil || !strings.Contains(err.Error(), "usermod -aG kvm") {
		t.Errorf("permission failure: want the kvm group fix, got %v", err)
	}
}

// TestHardwareAcceleratorBoundsAHungQEMU verifies `-accel help` is bounded, so
// ze doctor and a build report a hung QEMU instead of hanging on it.
//
// VALIDATES: round-2 NOTE 3.
func TestHardwareAcceleratorBoundsAHungQEMU(t *testing.T) {
	fakeQEMU(t, "exec sleep 30\n")
	old := hostProbeTimeout
	t.Cleanup(func() { hostProbeTimeout = old })
	hostProbeTimeout = 200 * time.Millisecond

	start := time.Now()
	_, err := hardwareAccelerator(context.Background(), "qemu-system-x86_64")
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("hardwareAccelerator waited %s on a hung QEMU", elapsed)
	}
	if err == nil || !strings.Contains(err.Error(), "did not answer within") || !strings.Contains(err.Error(), "\nqemu-system-x86_64 -accel help") {
		t.Errorf("a hung QEMU is not named as such, with the command on its own line: %v", err)
	}
}

// TestSupportedArchesAreTheBuildableOnes proves the arch list a refusal offers
// is derived from the backend's platforms: the Go names, once each.
//
// VALIDATES: round-4 NOTE 1.
func TestSupportedArchesAreTheBuildableOnes(t *testing.T) {
	if got := strings.Join(SupportedArches(), ","); got != "amd64,arm64" {
		t.Errorf("SupportedArches() = %s, want amd64,arm64", got)
	}
	cases := map[string]bool{"amd64": true, "x86_64": true, "arm64": true, "aarch64": true, "riscv64": false, "": false}
	for arch, want := range cases {
		if got := ArchSupported(arch); got != want {
			t.Errorf("ArchSupported(%q) = %v, want %v", arch, got, want)
		}
	}
}
