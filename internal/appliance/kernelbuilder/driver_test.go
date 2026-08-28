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
	"strings"
	"testing"
)

func TestSelectBuilder(t *testing.T) {
	old := commandAvailable
	t.Cleanup(func() { commandAvailable = old })

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
			got, err := selectBuilder(test.requested, "arm64")
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("selectBuilder error = %v, want %q", err, test.wantError)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("selectBuilder = %q, %v, want %q", got, err, test.want)
			}
		})
	}
}

func TestValidateRequestNormalizesAarch64(t *testing.T) {
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
	err := runDocker(ctx, req)
	if err == nil || !strings.Contains(err.Error(), "build failed") {
		t.Fatalf("runDocker error = %v", err)
	}
	if calls != 3 || !repairRanAfterCancel {
		t.Fatalf("ownership repair was skipped after cancellation, calls = %d", calls)
	}
}

func TestWriteProvenanceBytes(t *testing.T) {
	root := t.TempDir()
	req := Request{Root: root, Version: "7.2.3", Arch: "arm64", Profile: "hardware", Target: "runtime", Modules: "yes"}
	if err := writeProvenance("out", req, "qemu"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "out", provenanceName))
	if err != nil {
		t.Fatal(err)
	}
	want := "version=7.2.3\ntarget=runtime\nprofile=hardware\narch=arm64\nmodules=yes\nbuilder=qemu\n"
	if string(data) != want {
		t.Fatalf("provenance = %q, want %q", data, want)
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
	req := Request{}
	path, err := ensureAlpineISO(context.Background(), req, "x86_64")
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
	if _, err := ensureAlpineISO(context.Background(), req, "x86_64"); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("verified cache downloaded again, requests = %d", requests)
	}
	if err := os.WriteFile(path, []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureAlpineISO(context.Background(), req, "x86_64"); err != nil {
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
	args, err := qemuArgs(context.Background(), req, "alpine.iso", 22022, 9216, filepath.Join(root, "ccache"), filepath.Join(root, "build"))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"-accel kvm", "-accel tcg,thread=multi,tb-size=512", "hostfwd=tcp::22022-:22", "mount_tag=workspace", "mount_tag=ccache", "mount_tag=builddir", "mount_tag=output", "mount_tag=firmware", "readonly=on"} {
		if !strings.Contains(joined, want) {
			t.Errorf("QEMU argv missing %q: %s", want, joined)
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
