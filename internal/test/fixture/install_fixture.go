package fixture

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	switch filepath.Base(os.Args[0]) {
	case "docker":
		os.Exit(installDockerStub(os.Args[1:]))
	case "go":
		os.Exit(installGoStub(os.Args[1:]))
	case "grub-mkstandalone":
		os.Exit(installGRUBStub(os.Args[1:]))
	case "xorriso":
		os.Exit(installXorrisoStub(os.Args[1:]))
	}

	Register("appliance/appliance-iso-arm64/prepare", isoARM64Prepare)
	Register("appliance/appliance-iso-default-paths/prepare", isoAMD64Prepare)
	Register("appliance/appliance-iso-default-paths/check-image", isoCheckImage)
	registerPassed("appliance/appliance-build-arm64-goarch", applianceBuildARM64Fixture)
	registerPassed("appliance/appliance-iso-arm64", applianceISOARM64Fixture)
	registerPassed("appliance/appliance-iso-default-paths", applianceISOAMD64Fixture)
	Register("appliance/appliance-push-image-escape", appliancePushEscapeFixture)
	Register("appliance/appliance-replace-cert", applianceReplaceCertFixture)
	registerPassed("appliance/serial-login", applianceSerialLoginFixture)
	registerPassed("install/appliance-kernel-auto-qemu", kernelAutoBuilderFixture)
	registerPassed("install/appliance-kernel-auto-docker", kernelAutoBuilderFixture)
	registerPassed("install/appliance-kernel-docker", kernelBuilderSingleDriverFixture)
	registerPassed("install/appliance-kernel-registry", kernelSharedFragmentFixture)
	registerPassed("install/appliance-kernel-runtime", kernelBuildOwnershipFixture)
	registerPassed("install/appliance-kernel-qemu", kernelQEMUFixture)
	registerPassed("install/kernel-build-output-ownership", kernelBuildOwnershipFixture)
	registerPassed("install/kernel-builder-no-shell", kernelBuilderNativeFixture)
	registerPassed("install/kernel-builder-packages", kernelBuilderPackagesFixture)
	registerPassed("install/kernel-builder-single-driver", kernelBuilderSingleDriverFixture)
	registerPassed("install/kernel-qemu-arch-alias", kernelQEMUArchAliasFixture)
	registerPassed("install/kernel-runtime-deps", kernelRuntimeDepsFixture)
	registerPassed("install/kernel-shared-fragment", kernelSharedFragmentFixture)
	registerPassed("install/kernel-version-provenance", kernelVersionProvenanceFixture)
	registerPassed("install/kernel-arch-mapping-single", kernelAutoBuilderFixture)
	registerPassed("install/kernel-compose", kernelSharedFragmentFixture)
	registerPassed("install/kernel-tarball-dedup", kernelBuilderPackagesFixture)
	registerPassed("install/kernel-version-single-reader", kernelVersionProvenanceFixture)
	registerPassed("install/kernel-wiring", kernelBuilderSingleDriverFixture)
	registerPassed("install/ze-kernel-no-modcache-mutation", kernelBuilderPackagesFixture)
	registerPassed("install/ze-kernel-overlay", kernelBuildOwnershipFixture)
}

func registerPassed(name string, driver Driver) {
	Register(name, func(ctx context.Context, args []string) error {
		if err := driver(ctx, args); err != nil {
			return err
		}
		fmt.Println("OK")
		return nil
	})
}
func installDockerStub(args []string) int {
	if logPath := os.Getenv("ZE_INSTALL_DOCKER_LOG"); logPath != "" {
		record, err := json.Marshal(args)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		_, writeErr := fmt.Fprintln(file, string(record))
		if err := errors.Join(writeErr, file.Close()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
	}
	if len(args) > 0 && args[0] == "run" && containsArg(args, "ze-kernel-builder") {
		output := dockerOutputMount(args)
		if output == "" {
			fmt.Fprintln(os.Stderr, "fake kernel worker has no /out mount")
			return 2
		}
		if err := os.MkdirAll(output, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		artifact := filepath.Join(output, "Image")
		if argumentValue(args, "--modules") == "yes" {
			artifact = filepath.Join(output, "vmlinuz")
			if err := os.MkdirAll(filepath.Join(output, "lib", "modules", "7.1.1-fixture"), 0o755); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 2
			}
		}
		if err := os.WriteFile(artifact, []byte("fixture kernel\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		if err := os.WriteFile(filepath.Join(output, "config"), []byte("CONFIG_FIXTURE=y\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
	}
	if len(args) > 0 && args[0] == "run" && os.Getenv("ZE_INSTALL_DOCKER_FAIL_BUILD") != "" &&
		containsArg(args, "ze-kernel-builder") {
		fmt.Fprintln(os.Stderr, "fake kernel build failure")
		return 1
	}
	return 0
}
func installGoStub(args []string) int {
	arch := os.Getenv("GOARCH")
	if logPath := os.Getenv("ZE_INSTALL_GOARCH_LOG"); logPath != "" {
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		_, writeErr := fmt.Fprintln(file, arch)
		if err := errors.Join(writeErr, file.Close()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
	}
	if want := os.Getenv("ZE_INSTALL_GOARCH"); want != "" && arch != want {
		fmt.Fprintf(os.Stderr, "fake go saw GOARCH=%s, want %s\n", arch, want)
		return 42
	}
	if containsArg(args, "-json") {
		importPath := ""
		if len(args) != 0 {
			importPath = args[len(args)-1]
		}
		encoded, err := json.Marshal(map[string]string{"Name": "main", "ImportPath": importPath})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		fmt.Println(string(encoded))
		return 0
	}
	if containsArg(args, "-f") {
		fmt.Println(os.Getenv("ZE_INSTALL_FAKE_PKGDIR"))
		return 0
	}
	if output := argumentValue(args, "-o"); output != "" {
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		if err := os.WriteFile(output, []byte("fake binary\n"), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
	}
	return 0
}

func installGRUBStub(args []string) int {
	target, output := argumentValue(args, "-O"), argumentValue(args, "-o")
	if target == "" || output == "" {
		fmt.Fprintln(os.Stderr, "grub fixture requires -O TARGET and -o OUTPUT")
		return 2
	}
	if want := os.Getenv("ZE_INSTALL_GRUB_TARGET"); want != "" && target != want {
		fmt.Fprintf(os.Stderr, "grub target %s, want %s\n", target, want)
		return 2
	}
	if logPath := os.Getenv("ZE_INSTALL_GRUB_LOG"); logPath != "" {
		if err := os.WriteFile(logPath, []byte(target+"\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err := os.WriteFile(output, []byte("efi-"+target+"\n"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	return 0
}

func installXorrisoStub(args []string) int {
	output, image := argumentValue(args, "-o"), argumentValue(args, "-e")
	stage := ""
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "-as":
			index++
		case "-o", "-e":
			index++
		case "-R", "-J", "-eltorito-alt-boot", "-no-emul-boot":
		default:
			stage = args[index]
		}
	}
	if output == "" || image == "" || stage == "" {
		fmt.Fprintln(os.Stderr, "xorriso fixture did not receive output, EFI image, and staging directory")
		return 2
	}
	if err := requireRegularFile(filepath.Join(stage, filepath.FromSlash(image))); err != nil {
		fmt.Fprintf(os.Stderr, "xorriso EFI image missing: %v\n", err)
		return 2
	}
	if err := os.WriteFile(output, []byte("fixture iso\n"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	return 0
}

func argumentValue(args []string, name string) string {
	for index, arg := range args {
		if arg == name && index+1 < len(args) {
			return args[index+1]
		}
	}
	return ""
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
func dockerOutputMount(args []string) string {
	for _, arg := range args {
		if strings.HasSuffix(arg, ":/out") {
			return strings.TrimSuffix(arg, ":/out")
		}
	}
	return ""
}

func applianceBuildARM64Fixture(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("appliance-build-arm64-goarch requires REPOSITORY")
	}
	ze, err := exec.LookPath("ze")
	if err != nil {
		return err
	}
	scratch, err := copyKernelFixtureTree(args[0])
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	arm, err := queryKernelCache(ctx, ze, scratch, "installer", "arm64", "qemu")
	if err != nil {
		return err
	}
	amd, err := queryKernelCache(ctx, ze, scratch, "installer", "amd64", "qemu")
	if err != nil {
		return err
	}
	if arm == amd || !strings.Contains(arm, "arm64") || !strings.Contains(amd, "amd64") {
		return fmt.Errorf("arm64 cache identity = %q, amd64 = %q", arm, amd)
	}
	work, err := os.Getwd()
	if err != nil {
		return err
	}
	appliances := filepath.Join(work, "appliances")
	if err := os.Mkdir(appliances, 0o755); err != nil {
		return err
	}
	config := filepath.Join(work, "build-config.json")
	if output, err := runApplianceCapture(ctx, ze, "--dir", appliances, "init", "--config", config, "lab"); err != nil {
		return fmt.Errorf("appliance init: %w: %s", err, output)
	}
	fakebin, err := os.MkdirTemp("", "install-goarch-bin-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(fakebin) }()
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.Symlink(self, filepath.Join(fakebin, "go")); err != nil {
		return err
	}
	logPath := filepath.Join(fakebin, "goarch.log")
	fakePackage := filepath.Join(fakebin, "package")
	if err := os.Mkdir(fakePackage, 0o755); err != nil {
		return err
	}
	restores := []func(){
		setFixtureEnv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH")),
		setFixtureEnv("GOARCH", "amd64"),
		setFixtureEnv("ZE_INSTALL_GOARCH", "arm64"),
		setFixtureEnv("ZE_INSTALL_GOARCH_LOG", logPath),
		setFixtureEnv("ZE_INSTALL_FAKE_PKGDIR", fakePackage),
	}
	defer func() {
		for index := len(restores) - 1; index >= 0; index-- {
			restores[index]()
		}
	}()
	cmd := exec.CommandContext(ctx, ze, "appliance", "--dir", appliances, "build", "lab")
	cmd.Dir = args[0]
	output, buildErr := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if strings.Contains(string(output), "fake go saw GOARCH=") {
		return fmt.Errorf("arm64 build request: %s", output)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		return fmt.Errorf("appliance build did not reach go (build error %s, output %s): %w", fmt.Sprint(buildErr), output, err)
	}
	lines := strings.Fields(string(data))
	if len(lines) == 0 {
		return fmt.Errorf("appliance build did not invoke go (build error %s, output %s)", fmt.Sprint(buildErr), output)
	}
	for _, arch := range lines {
		if arch != "arm64" {
			return fmt.Errorf("gok go subprocess used GOARCH=%s, want arm64 (build error %s, output %s)", arch, fmt.Sprint(buildErr), output)
		}
	}
	return nil
}

func applianceISOARM64Fixture(ctx context.Context, _ []string) error {
	return applianceISOFixture(ctx, "arm64", 56, 0x644d5241, "arm64-efi")
}

func applianceISOAMD64Fixture(ctx context.Context, _ []string) error {
	return applianceISOFixture(ctx, "amd64", 0x202, 0x53726448, "x86_64-efi")
}

func applianceISOFixture(ctx context.Context, arch string, magicOffset int, magic uint32, grubTarget string) error {
	repo := os.Getenv("ZE_REPO_ROOT")
	if repo == "" {
		return errors.New("ZE_REPO_ROOT is not set")
	}
	version, err := os.ReadFile(filepath.Join(repo, "internal", "appliance", "kernel.version"))
	if err != nil {
		return fmt.Errorf("read kernel version: %w", err)
	}
	for _, dir := range []string{
		"appliances",
		filepath.Join("tools", "installer-kernel"),
		filepath.Join("build", "kernel"),
		filepath.Join("build", "initrd"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	for _, name := range []string{"kernel.config", "kernel.require", "qemu.config", "qemu.require"} {
		if err := os.WriteFile(filepath.Join("tools", "installer-kernel", name), nil, 0o644); err != nil {
			return err
		}
	}
	ze, err := exec.LookPath("ze")
	if err != nil {
		return err
	}
	if output, err := runApplianceCapture(ctx, ze, "--dir", "appliances", "init", "--config", "iso-config.json", "lab"); err != nil {
		return fmt.Errorf("appliance init: %w: %s", err, output)
	}
	image := filepath.Join("appliances", "lab", "ze-20260101-000000.img")
	kernel := filepath.Join("build", "kernel", "Image")
	if err := os.WriteFile(image, []byte("image-bytes\n"), 0o644); err != nil {
		return err
	}
	if err := prepareISOInputs([]string{kernel, image}, magicOffset, magic); err != nil {
		return err
	}
	variant := fmt.Sprintf("%s-qemu-%s-test\n", arch, strings.TrimSpace(string(version)))
	if err := os.WriteFile(filepath.Join("build", "kernel", ".variant"), []byte(variant), 0o644); err != nil {
		return err
	}
	initrd := "initrd-" + arch + "\n"
	if arch == "amd64" {
		initrd = "initrd-default\n"
	}
	if err := os.WriteFile(filepath.Join("build", "initrd", "initrd.img.gz"), []byte(initrd), 0o644); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	fakebin, err := os.MkdirTemp("", "install-iso-bin-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(fakebin) }()
	for _, name := range []string{"grub-mkstandalone", "xorriso"} {
		if err := os.Symlink(self, filepath.Join(fakebin, name)); err != nil {
			return err
		}
	}
	restorePath := setFixtureEnv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	defer restorePath()
	restoreTarget := setFixtureEnv("ZE_INSTALL_GRUB_TARGET", grubTarget)
	defer restoreTarget()
	grubLog := filepath.Join(fakebin, "grub.target")
	restoreLog := setFixtureEnv("ZE_INSTALL_GRUB_LOG", grubLog)
	defer restoreLog()

	if output, err := runApplianceCapture(ctx, ze, "--dir", "appliances", "iso", "--keep-staging", "lab"); err != nil {
		return fmt.Errorf("appliance iso: %w: %s", err, output)
	}
	stages, err := filepath.Glob(filepath.Join("appliances", "lab", ".iso-staging-*"))
	if err != nil {
		return err
	}
	if len(stages) != 1 {
		return fmt.Errorf("ISO staging directories = %v, want exactly one", stages)
	}
	stage := stages[0]
	grub, err := os.ReadFile(filepath.Join(stage, "boot", "grub", "grub.cfg"))
	if err != nil {
		return err
	}
	console := "console=ttyAMA0,115200n8 console=tty0"
	bootEFI := "BOOTAA64.EFI"
	if arch == "amd64" {
		console = "console=ttyS0,115200n8 console=tty0"
		bootEFI = "BOOTX64.EFI"
	}
	for _, text := range []string{console, "ze.image=ze-20260101-000000.img.gz"} {
		if !bytes.Contains(grub, []byte(text)) {
			return fmt.Errorf("grub.cfg missing %q", text)
		}
	}
	if arch == "arm64" && !bytes.Contains(grub, []byte("search --no-floppy --file /ze-install/media-id --set=root")) {
		return errors.New("arm64 grub.cfg missing media root search")
	}
	for _, path := range []string{
		filepath.Join(stage, "EFI", "BOOT", bootEFI),
		filepath.Join(stage, "EFI", "BOOT", "efiboot.img"),
		filepath.Join(stage, "ze-install", "images", "ze-20260101-000000.img.gz"),
	} {
		if err := requireRegularFile(path); err != nil {
			return fmt.Errorf("ISO staging artifact: %w", err)
		}
	}
	target, err := os.ReadFile(grubLog)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(target)) != grubTarget {
		return fmt.Errorf("grub target = %q, want %q", strings.TrimSpace(string(target)), grubTarget)
	}
	if arch == "amd64" {
		kernelData, err := os.ReadFile(filepath.Join(stage, "boot", "kernel"))
		if err != nil {
			return err
		}
		if len(kernelData) != 0x206 {
			return fmt.Errorf("staged amd64 kernel length = %d, want %d", len(kernelData), 0x206)
		}
		initrdData, err := os.ReadFile(filepath.Join(stage, "boot", "initrd.img.gz"))
		if err != nil {
			return err
		}
		if string(initrdData) != "initrd-default\n" {
			return fmt.Errorf("staged initrd = %q", initrdData)
		}
		if err := requireRegularFile(filepath.Join(stage, "ze-install", "images", "ze-20260101-000000.img.gz.sha256")); err != nil {
			return fmt.Errorf("staged compressed image checksum: %w", err)
		}
	}
	return isoCheckImage(ctx, []string{filepath.Join(stage, "ze-install", "images", "ze-20260101-000000.img.gz")})
}

func runApplianceCapture(ctx context.Context, ze string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, ze, append([]string{"appliance"}, args...)...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
func requireRegularFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	return nil
}

func appliancePushEscapeFixture(ctx context.Context, _ []string) error {
	if err := os.Mkdir("appliances", 0o755); err != nil {
		return err
	}
	if code := applianceCommand(ctx, "--dir", "appliances", "init", "--config", "push-config.json", "lab"); code != 0 {
		return fmt.Errorf("appliance init exited %d", code)
	}
	if err := os.WriteFile("outside.img", []byte("outside image\n"), 0o644); err != nil {
		return err
	}
	if err := os.Symlink("../../outside.img", filepath.Join("appliances", "lab", "ze-20260101-000000.img")); err != nil {
		return err
	}
	if code := applianceCommand(ctx, "--dir", "appliances", "push", "lab"); code == 0 {
		return errors.New("push unexpectedly accepted an escaping image")
	}
	return nil
}

func applianceReplaceCertFixture(ctx context.Context, _ []string) error {
	if err := os.Mkdir("appliances", 0o755); err != nil {
		return err
	}
	for _, item := range [][2]string{{"lab-config.json", "lab"}, {"other-config.json", "other"}} {
		if code := applianceCommand(ctx, "--dir", "appliances", "init", "--config", item[0], item[1]); code != 0 {
			return fmt.Errorf("appliance init %s exited %d", item[1], code)
		}
	}
	lab := filepath.Join("appliances", "lab", "secrets", "tls")
	other := filepath.Join("appliances", "other", "secrets", "tls")
	cert, err := os.ReadFile(filepath.Join(lab, "cert.pem"))
	if err != nil {
		return err
	}
	key, err := os.ReadFile(filepath.Join(lab, "key.pem"))
	if err != nil {
		return err
	}
	otherKey, err := os.ReadFile(filepath.Join(other, "key.pem"))
	if err != nil {
		return err
	}
	for path, data := range map[string][]byte{"good.pem": cert, "good.key": key, "other.key": otherKey} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return err
		}
	}
	if code := applianceCommand(ctx, "--dir", "appliances", "replace-cert", "--cert", "good.pem", "--key", "good.key", "lab"); code != 0 {
		return fmt.Errorf("valid replacement exited %d", code)
	}
	storedCert, err := os.ReadFile(filepath.Join(lab, "cert.pem"))
	if err != nil {
		return err
	}
	storedKey, err := os.ReadFile(filepath.Join(lab, "key.pem"))
	if err != nil {
		return err
	}
	if !bytes.Equal(storedCert, cert) || !bytes.Equal(storedKey, key) {
		return errors.New("valid replacement did not store both requested files")
	}
	if code := applianceCommand(ctx, "--dir", "appliances", "replace-cert", "--cert", "good.pem", "--key", "other.key", "lab"); code == 0 {
		return errors.New("mismatched certificate and key were accepted")
	}
	afterCert, err := os.ReadFile(filepath.Join(lab, "cert.pem"))
	if err != nil {
		return err
	}
	afterKey, err := os.ReadFile(filepath.Join(lab, "key.pem"))
	if err != nil {
		return err
	}
	if !bytes.Equal(afterCert, cert) || !bytes.Equal(afterKey, key) {
		return errors.New("stored material changed after the refusal")
	}
	entries, err := os.ReadDir(lab)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			return fmt.Errorf("temporary certificate file left behind: %s", entry.Name())
		}
	}
	fmt.Fprintln(os.Stderr, "stored material unchanged after the refusal")
	return nil
}

func applianceCommand(ctx context.Context, args ...string) int {
	cmd := exec.CommandContext(ctx, "ze", append([]string{"appliance"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "cannot run ze appliance: %v\n", err)
		return 127
	}
	return 0
}

func applianceSerialLoginFixture(ctx context.Context, _ []string) error {
	ze, err := exec.LookPath("ze")
	if err != nil {
		return err
	}
	root, err := os.MkdirTemp("", "serial-login-fixture-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(root) }()
	ash := filepath.Join(root, "ash")
	if err := os.Symlink(ze, ash); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, ash)
	var stderr bytes.Buffer
	cmd.Stdin, cmd.Stderr = bytes.NewReader(nil), &stderr
	if err := cmd.Run(); err == nil {
		return errors.New("ash invocation with non-terminal stdin was not rejected")
	}
	if strings.Contains(stderr.String(), "granting access without authentication") {
		return errors.New("serial login reached authentication before terminal rejection")
	}
	if err := exec.CommandContext(ctx, ze, "show", "version").Run(); err != nil {
		return fmt.Errorf("normal ze dispatch: %w", err)
	}
	return nil
}

func isoARM64Prepare(_ context.Context, args []string) error {
	return prepareISOInputs(args, 56, 0x644d5241)
}

func isoAMD64Prepare(_ context.Context, args []string) error {
	return prepareISOInputs(args, 0x202, 0x53726448)
}

func prepareISOInputs(args []string, magicOffset int, magic uint32) error {
	if len(args) != 2 {
		return errors.New("prepare requires KERNEL IMAGE")
	}
	kernel := make([]byte, 0x206)
	binary.LittleEndian.PutUint32(kernel[magicOffset:], magic)
	if err := os.WriteFile(args[0], kernel, 0o644); err != nil {
		return fmt.Errorf("write kernel image: %w", err)
	}
	data, err := os.ReadFile(args[1])
	if err != nil {
		return fmt.Errorf("read appliance image: %w", err)
	}
	sum := sha256.Sum256(data)
	sidecar := fmt.Sprintf("%x  %s\n", sum, filepath.Base(args[1]))
	if err := os.WriteFile(args[1]+".sha256", []byte(sidecar), 0o644); err != nil {
		return fmt.Errorf("write appliance checksum: %w", err)
	}
	return nil
}

func isoCheckImage(_ context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("check-image requires GZIP-PATH")
	}
	file, err := os.Open(args[0])
	if err != nil {
		return err
	}
	decompressed, err := gzip.NewReader(file)
	if err != nil {
		_ = file.Close()
		return err
	}
	data, readErr := io.ReadAll(decompressed)
	return errors.Join(readErr, decompressed.Close(), file.Close(), expectBytes("decompressed image", data, []byte("image-bytes\n")))
}

func kernelAutoBuilderFixture(ctx context.Context, _ []string) error {
	root, req, cleanup, err := syntheticBuildRequest()
	if err != nil {
		return err
	}
	defer cleanup()

	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	noop, err := noopBinary()
	if err != nil {
		return err
	}
	for _, name := range []string{"docker", "qemu-system-aarch64", "go"} {
		if err := os.Symlink(noop, filepath.Join(bin, name)); err != nil {
			return err
		}
	}
	restore := setFixtureEnv("PATH", bin)
	defer restore()

	out, err := runBuildCapture(ctx, &req)
	if err != nil {
		return fmt.Errorf("docker-first selection: %w", err)
	}
	if !strings.Contains(out, "builder=docker") {
		return fmt.Errorf("auto selection did not prefer docker: %s", out)
	}
	req.Arch = "amd64"
	req.OutputDir = "out-docker-amd64"
	out, err = runBuildCapture(ctx, &req)
	if err != nil {
		return fmt.Errorf("amd64 docker selection: %w", err)
	}
	if !strings.Contains(out, "arch=amd64") || !strings.Contains(out, "builder=docker") {
		return fmt.Errorf("amd64 mapping output = %q", out)
	}
	req.Arch = "arm64"

	if err := os.Remove(filepath.Join(bin, "docker")); err != nil {
		return err
	}
	req.OutputDir = "out-qemu"
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	out, err = runBuildCapture(cancelled, &req)
	if err == nil || !errors.Is(err, context.Canceled) || !strings.Contains(out, "builder=qemu") {
		return fmt.Errorf("auto selection did not enter and cancel qemu: output=%q error=%s", out, fmt.Sprint(err))
	}

	if err := os.Remove(filepath.Join(bin, "qemu-system-aarch64")); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(bin, "go")); err != nil {
		return err
	}
	req.OutputDir = "out-none"
	_, err = runBuildCapture(ctx, &req)
	if err == nil || !strings.Contains(err.Error(), "no builder available") {
		return fmt.Errorf("no-builder selection returned %s", fmt.Sprint(err))
	}
	return nil
}

func kernelQEMUFixture(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("appliance-kernel-qemu requires REPOSITORY")
	}
	root, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}
	tmpRoot := filepath.Join(root, "tmp")
	if err := os.MkdirAll(tmpRoot, 0o755); err != nil {
		return err
	}
	out, err := os.MkdirTemp(tmpRoot, "install-fixture-qemu-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(out) }()
	cache, err := os.MkdirTemp("", "install-fixture-qemu-cache-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(cache) }()
	bin := filepath.Join(cache, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	noop, err := noopBinary()
	if err != nil {
		return err
	}
	for _, name := range []string{"qemu-system-aarch64", "go"} {
		if err := os.Symlink(noop, filepath.Join(bin, name)); err != nil {
			return err
		}
	}
	restorePath := setFixtureEnv("PATH", bin)
	defer restorePath()
	restoreCache := setFixtureEnv("XDG_CACHE_HOME", cache)
	defer restoreCache()
	relOut, err := filepath.Rel(root, out)
	if err != nil {
		return err
	}
	fragments := []string{
		"tools/installer-kernel/kernel.config",
		"tools/installer-kernel/hardware.config",
		"tools/kernel-builder/common/efi-console.config",
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	req := kernelbuilder.Request{
		Root: root, Version: "7.1.1", Arch: "arm64", Profile: "hardware", Builder: "qemu",
		Target: "installer", SourceDir: "tools/installer-kernel", OutputDir: relOut,
		BuilderDir: "tools/kernel-builder", CommonDir: "tools/kernel-builder/common",
		Modules: "no", Fragments: fragments,
	}
	progress, err := runBuildCapture(cancelled, &req)
	if err == nil || !errors.Is(err, context.Canceled) || !strings.Contains(progress, "builder=qemu") {
		return fmt.Errorf("native QEMU backend was not selected and canceled: output=%q error=%s", progress, fmt.Sprint(err))
	}
	return nil
}

func kernelBuildOwnershipFixture(ctx context.Context, args []string) error {
	if len(args) == 0 {
		root, req, cleanup, err := syntheticBuildRequest()
		if err != nil {
			return err
		}
		defer cleanup()
		bin := filepath.Join(root, "bin")
		if err := os.MkdirAll(bin, 0o755); err != nil {
			return err
		}
		self, err := os.Executable()
		if err != nil {
			return err
		}
		if err := os.Symlink(self, filepath.Join(bin, "docker")); err != nil {
			return err
		}
		restorePath := setFixtureEnv("PATH", bin)
		defer restorePath()
		logPath := filepath.Join(root, "docker.log")
		restoreLog := setFixtureEnv("ZE_INSTALL_DOCKER_LOG", logPath)
		defer restoreLog()

		req.Builder = "docker"
		req.OutputDir = filepath.Join(root, "isolated-output")
		if err := kernelbuilder.Build(ctx, req); err != nil {
			return fmt.Errorf("successful ownership build: %w", err)
		}
		calls, err := readDockerCalls(logPath)
		if err != nil {
			return err
		}
		if err := assertOwnershipRepairCalls(calls, req.OutputDir); err != nil {
			return fmt.Errorf("successful build: %w", err)
		}
		if err := requireRegularFile(filepath.Join(req.OutputDir, "vmlinuz")); err != nil {
			return fmt.Errorf("successful runtime output: %w", err)
		}
		provenance, err := os.ReadFile(filepath.Join(req.OutputDir, "kernel.version"))
		if err != nil {
			return err
		}
		wantProvenance := "version=7.1.1\ntarget=runtime\nprofile=runtime\narch=arm64\nmodules=yes\nbuilder=docker\n"
		if string(provenance) != wantProvenance {
			return fmt.Errorf("runtime provenance = %q, want %q", provenance, wantProvenance)
		}

		if err := os.WriteFile(logPath, nil, 0o644); err != nil {
			return err
		}
		req.OutputDir = filepath.Join(root, "failed-output")
		restoreFailure := setFixtureEnv("ZE_INSTALL_DOCKER_FAIL_BUILD", "1")
		buildErr := kernelbuilder.Build(ctx, req)
		restoreFailure()
		if buildErr == nil {
			return errors.New("failed container was reported as successful")
		}
		calls, err = readDockerCalls(logPath)
		if err != nil {
			return err
		}
		if err := assertOwnershipRepairCalls(calls, req.OutputDir); err != nil {
			return fmt.Errorf("failed build: %w", err)
		}
		if _, err := os.Stat(filepath.Join(req.OutputDir, "kernel.version")); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed build provenance exists or cannot be checked: %s", fmt.Sprint(err))
		}
		return nil
	}
	if len(args) != 2 {
		return errors.New("kernel-build-output-ownership requires REPOSITORY OUTPUT")
	}
	return kernelbuilder.Build(ctx, kernelbuilder.Request{
		Root: args[0], Version: "7.1.1", Arch: "amd64", Profile: "runtime", Builder: "docker",
		Target: "runtime", SourceDir: "gokrazy/kernel", OutputDir: args[1],
		BuilderDir: "tools/kernel-builder", CommonDir: "tools/kernel-builder/common",
		Modules: "yes", PatchesDir: "gokrazy/kernel/patches",
		Fragments: []string{
			"gokrazy/kernel/kernel.config",
			"gokrazy/kernel/runtime.config",
			"tools/kernel-builder/common/efi-console.config",
		},
	})
}
func readDockerCalls(path string) ([][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	calls := make([][]string, 0, len(lines))
	for _, line := range lines {
		var args []string
		if err := json.Unmarshal([]byte(line), &args); err != nil {
			return nil, fmt.Errorf("decode docker call %q: %w", line, err)
		}
		calls = append(calls, args)
	}
	return calls, nil
}

func assertOwnershipRepairCalls(calls [][]string, outputDir string) error {
	if len(calls) != 3 {
		return fmt.Errorf("docker calls = %v, want build, worker, repair", calls)
	}
	if !containsArgSequence(calls[0], "build", "--platform", "linux/arm64") {
		return fmt.Errorf("builder image argv = %v", calls[0])
	}
	if !containsArgSequence(calls[1], "run", "--rm", "--platform", "linux/arm64") ||
		!containsArg(calls[1], "ze-kernel-builder") ||
		!containsArg(calls[1], outputDir+":/out") {
		return fmt.Errorf("kernel worker argv = %v", calls[1])
	}
	owner := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	if !containsArgSequence(calls[2], "run", "--rm", "--platform", "linux/arm64") ||
		!containsArgSequence(calls[2], "chown", "-R", owner, "/out") ||
		!containsArg(calls[2], outputDir+":/out") {
		return fmt.Errorf("ownership repair argv = %v", calls[2])
	}
	return nil
}

func containsArgSequence(args []string, sequence ...string) bool {
	if len(sequence) == 0 {
		return true
	}
	for start := range len(args) - len(sequence) + 1 {
		match := true
		for offset := range sequence {
			if args[start+offset] != sequence[offset] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func kernelVersionProvenanceFixture(ctx context.Context, args []string) error {
	if len(args) == 0 {
		root, req, cleanup, err := syntheticBuildRequest()
		if err != nil {
			return err
		}
		defer cleanup()
		bin := filepath.Join(root, "bin")
		if err := os.MkdirAll(bin, 0o755); err != nil {
			return err
		}
		self, err := os.Executable()
		if err != nil {
			return err
		}
		if err := os.Symlink(self, filepath.Join(bin, "docker")); err != nil {
			return err
		}
		restorePath := setFixtureEnv("PATH", bin)
		defer restorePath()
		logPath := filepath.Join(root, "docker.log")
		restoreLog := setFixtureEnv("ZE_INSTALL_DOCKER_LOG", logPath)
		defer restoreLog()
		req.Builder = "docker"
		req.Target, req.Profile, req.Modules = "installer", "qemu", "no"
		req.OutputDir = filepath.Join(root, "provenance")
		if err := kernelbuilder.Build(ctx, req); err != nil {
			return err
		}
		data, err := os.ReadFile(filepath.Join(req.OutputDir, "kernel.version"))
		if err != nil {
			return err
		}
		want := "version=7.1.1\ntarget=installer\nprofile=qemu\narch=arm64\nmodules=no\nbuilder=docker\n"
		if string(data) != want {
			return fmt.Errorf("provenance = %q, want %q", data, want)
		}
		if err := os.WriteFile(logPath, nil, 0o644); err != nil {
			return err
		}
		for _, badVersion := range []string{"not-a-version", "6.12.9"} {
			req.Version = badVersion
			if err := kernelbuilder.Build(ctx, req); err == nil {
				return fmt.Errorf("invalid kernel version %q was accepted", badVersion)
			}
		}
		calls, err := readDockerCalls(logPath)
		if err != nil {
			return err
		}
		if len(calls) != 0 {
			return fmt.Errorf("invalid kernel version reached docker: %v", calls)
		}
		return nil
	}
	if len(args) != 3 {
		return errors.New("kernel-version-provenance requires REPOSITORY OUTPUT VERSION")
	}
	return kernelbuilder.Build(ctx, kernelbuilder.Request{
		Root: args[0], Version: args[2], Arch: "amd64", Profile: "qemu", Builder: "docker",
		Target: "installer", SourceDir: "tools/installer-kernel", OutputDir: args[1],
		BuilderDir: "tools/kernel-builder", CommonDir: "tools/kernel-builder/common",
		Modules: "no",
		Fragments: []string{
			"tools/installer-kernel/kernel.config",
			"tools/installer-kernel/qemu.config",
		},
	})
}
func kernelBuilderNativeFixture(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("kernel-builder-no-shell requires REPOSITORY")
	}

	entries, err := os.ReadDir(filepath.Join(args[0], "tools", "kernel-builder"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sh") {
			return fmt.Errorf("unexpected command script in native builder: %s", entry.Name())
		}
	}
	if err := requireRegularFile(filepath.Join(args[0], "tools", "kernel-builder", "main.go")); err != nil {
		return fmt.Errorf("native builder entrypoint: %w", err)
	}

	if _, err := runSyntheticWorker(ctx, "CONFIG_REQUIRED=y\nCONFIG_EQUALS=y\n", "runtime", "yes"); err != nil {
		return fmt.Errorf("valid required symbols: %w", err)
	}
	_, err = runSyntheticWorker(ctx, "CONFIG_REQUIRED=m\nCONFIG_EQUALS=y\n", "runtime", "yes")
	if err == nil || !strings.Contains(err.Error(), "CONFIG_REQUIRED did not resolve to =y") {
		return fmt.Errorf("missing required symbol returned %s", fmt.Sprint(err))
	}
	_, err = runSyntheticWorker(ctx, "CONFIG_REQUIRED=y\nCONFIG_EQUALS=y\n", "hardware-kms", "yes")
	if err == nil {
		return errors.New("hardware-kms without firmware returned <nil>")
	}
	if !strings.Contains(err.Error(), "requires --firmware-dir") {
		return fmt.Errorf("hardware-kms without firmware returned %w", err)
	}
	return nil
}

func kernelBuilderPackagesFixture(ctx context.Context, _ []string) error {
	output, err := runSyntheticWorker(ctx, "CONFIG_REQUIRED=y\nCONFIG_EQUALS=y\n", "runtime", "yes")
	if err != nil {
		return err
	}
	if !strings.Contains(output, "INSTALL_MOD_PATH=") || !strings.Contains(output, "modules_install") {
		return fmt.Errorf("runtime worker did not install modules: %s", output)
	}
	if !strings.Contains(output, "bzImage modules") {
		return fmt.Errorf("runtime worker did not build the compressed kernel and modules: %s", output)
	}
	installer, err := runSyntheticWorker(ctx, "CONFIG_REQUIRED=y\nCONFIG_EQUALS=y\n", "qemu", "no")
	if err != nil {
		return err
	}
	if !strings.Contains(installer, "bzImage") ||
		strings.Contains(installer, "modules_install") ||
		strings.Contains(installer, "bzImage modules") {
		return fmt.Errorf("installer worker did not use the shared module-free kernel path: %s", installer)
	}
	return nil
}

func runSyntheticWorker(ctx context.Context, config, profile, modules string) (string, error) {
	root, err := os.MkdirTemp("", "install-worker-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(root)
	work := filepath.Join(root, "work")
	build := filepath.Join(root, "build")
	out := filepath.Join(root, "out")
	tree := filepath.Join(build, "linux-7.1.1-"+modules)
	fragment := filepath.Join(root, "custom.config")
	for _, dir := range []string{work, filepath.Join(tree, "scripts", "kconfig"), filepath.Join(tree, "scripts"), filepath.Join(tree, "arch", "x86", "boot"), out} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	files := map[string]string{
		filepath.Join(work, "linux-7.1.1.tar.xz"):             "cached",
		filepath.Join(tree, "scripts", "Kbuild.include"):      "present",
		filepath.Join(tree, ".config"):                        config,
		filepath.Join(tree, "arch", "x86", "boot", "bzImage"): "kernel",
		fragment: "CONFIG_REQUIRED=y\n",
		strings.TrimSuffix(fragment, filepath.Ext(fragment)) + ".require": "CONFIG_REQUIRED\nCONFIG_EQUALS=y\n",
		filepath.Join(work, "linux-7.1.1-"+modules+".built.tar.part"):     "",
	}
	for path, data := range files {
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			return "", err
		}
	}
	noop, err := noopBinary()
	if err != nil {
		return "", err
	}
	if err := os.Symlink(noop, filepath.Join(tree, "scripts", "kconfig", "merge_config.sh")); err != nil {
		return "", err
	}
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return "", err
	}
	if err := os.Symlink("/bin/echo", filepath.Join(bin, "make")); err != nil {
		return "", err
	}
	if err := os.Symlink(noop, filepath.Join(bin, "tar")); err != nil {
		return "", err
	}
	restore := setFixtureEnv("PATH", bin)
	defer restore()
	var output bytes.Buffer
	err = kernelbuilder.RunWorker(ctx, kernelbuilder.WorkerRequest{
		Version: "7.1.1", Arch: "amd64", Profile: profile, Modules: modules, Jobs: "1",
		SourceDir: root, OutputDir: out, WorkDir: work, BuildDir: build,
		Fragments: []string{fragment}, Stdout: &output, Stderr: &output,
	})
	return output.String(), err
}

func kernelBuilderSingleDriverFixture(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("kernel-builder-single-driver requires REPOSITORY")
	}
	root, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}
	if err := requireRegularFile(filepath.Join(root, "tools", "kernel-builder", "main.go")); err != nil {
		return fmt.Errorf("shared native entrypoint: %w", err)
	}
	versionData, err := os.ReadFile(filepath.Join(root, "internal", "appliance", "kernel.version"))
	if err != nil {
		return err
	}
	version := strings.TrimSpace(string(versionData))
	work, err := os.MkdirTemp("", "install-single-driver-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	bin := filepath.Join(work, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.Symlink(self, filepath.Join(bin, "docker")); err != nil {
		return err
	}
	restorePath := setFixtureEnv("PATH", bin)
	defer restorePath()
	logPath := filepath.Join(work, "docker.log")
	restoreLog := setFixtureEnv("ZE_INSTALL_DOCKER_LOG", logPath)
	defer restoreLog()

	requests := []struct {
		name     string
		request  kernelbuilder.Request
		worker   []string
		wantProv string
	}{
		{
			name: "installer",
			request: kernelbuilder.Request{
				Root: root, Version: version, Arch: "amd64", Profile: "qemu", Builder: "docker",
				Target: "installer", SourceDir: "tools/installer-kernel", OutputDir: filepath.Join(work, "installer"),
				BuilderDir: "tools/kernel-builder", CommonDir: "tools/kernel-builder/common", Modules: "no",
				Fragments: []string{"tools/installer-kernel/kernel.config", "tools/installer-kernel/qemu.config"},
			},
			worker:   []string{"--modules", "no", "--fragment", "/src/kernel.config", "--fragment", "/src/qemu.config"},
			wantProv: fmt.Sprintf("version=%s\ntarget=installer\nprofile=qemu\narch=amd64\nmodules=no\nbuilder=docker\n", version),
		},
		{
			name: "runtime",
			request: kernelbuilder.Request{
				Root: root, Version: version, Arch: "amd64", Profile: "runtime", Builder: "docker",
				Target: "runtime", SourceDir: "gokrazy/kernel", OutputDir: filepath.Join(work, "runtime"),
				BuilderDir: "tools/kernel-builder", CommonDir: "tools/kernel-builder/common", Modules: "yes",
				PatchesDir: "gokrazy/kernel/patches",
				Fragments: []string{
					"gokrazy/kernel/kernel.config",
					"gokrazy/kernel/runtime.config",
					"tools/kernel-builder/common/efi-console.config",
				},
			},
			worker: []string{
				"--modules", "yes", "--patches-dir", "/src/patches",
				"--fragment", "/src/kernel.config", "--fragment", "/src/runtime.config",
				"--fragment", "/builder/common/efi-console.config",
			},
			wantProv: fmt.Sprintf("version=%s\ntarget=runtime\nprofile=runtime\narch=amd64\nmodules=yes\nbuilder=docker\n", version),
		},
	}
	for _, test := range requests {
		if err := os.WriteFile(logPath, nil, 0o644); err != nil {
			return err
		}
		output, err := runBuildCapture(ctx, &test.request)
		if err != nil {
			return fmt.Errorf("%s shared driver: %w", test.name, err)
		}
		if !strings.Contains(output, "builder=docker") || !strings.Contains(output, test.request.Target+" kernel ready") {
			return fmt.Errorf("%s progress output = %q", test.name, output)
		}
		calls, err := readDockerCalls(logPath)
		if err != nil {
			return err
		}
		if err := assertKernelBuildCalls(calls, test.request.OutputDir, test.worker); err != nil {
			return fmt.Errorf("%s request: %w", test.name, err)
		}
		provenance, err := os.ReadFile(filepath.Join(test.request.OutputDir, "kernel.version"))
		if err != nil {
			return err
		}
		if string(provenance) != test.wantProv {
			return fmt.Errorf("%s provenance = %q, want %q", test.name, provenance, test.wantProv)
		}
		artifact := "Image"
		if test.request.Target == "runtime" {
			artifact = "vmlinuz"
		}
		if err := requireRegularFile(filepath.Join(test.request.OutputDir, artifact)); err != nil {
			return fmt.Errorf("%s output artifact: %w", test.name, err)
		}
		if test.request.Target == "runtime" {
			if info, err := os.Stat(filepath.Join(test.request.OutputDir, "lib", "modules")); err != nil || !info.IsDir() {
				if err == nil {
					err = errors.New("not a directory")
				}
				return fmt.Errorf("runtime module tree: %w", err)
			}
		}
	}
	return nil
}

func assertKernelBuildCalls(calls [][]string, outputDir string, workerSequence []string) error {
	if len(calls) != 3 {
		return fmt.Errorf("docker calls = %v, want build, worker, repair", calls)
	}
	if !containsArgSequence(calls[0], "build", "--platform", "linux/amd64") {
		return fmt.Errorf("builder image argv = %v", calls[0])
	}
	if !containsArgSequence(calls[1], "run", "--rm", "--platform", "linux/amd64") ||
		!containsArg(calls[1], "ze-kernel-builder") ||
		!containsArg(calls[1], outputDir+":/out") {
		return fmt.Errorf("worker argv = %v", calls[1])
	}
	for index := 0; index < len(workerSequence); index += 2 {
		if !containsArgSequence(calls[1], workerSequence[index:index+2]...) {
			return fmt.Errorf("worker argv missing %v: %v", workerSequence[index:index+2], calls[1])
		}
	}
	if !containsArgSequence(calls[2], "chown", "-R", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), "/out") {
		return fmt.Errorf("repair argv = %v", calls[2])
	}
	return nil
}

func kernelQEMUArchAliasFixture(_ context.Context, _ []string) error {
	root, err := os.MkdirTemp("", "install-arch-alias-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	err = kernelbuilder.Build(context.Background(), kernelbuilder.Request{
		Root: root, Version: "7.1.1", Arch: "aarch64", Profile: "runtime", Builder: "qemu",
		Target: "runtime", SourceDir: "../bad", OutputDir: "out", BuilderDir: "builder",
		CommonDir: "common", Modules: "yes", Fragments: []string{"fragment.config"},
	})
	if err == nil {
		return errors.New("expected repository-path validation failure")
	}
	if strings.Contains(err.Error(), "unsupported ARCH") {
		return fmt.Errorf("aarch64 alias was rejected: %w", err)
	}
	if !strings.Contains(err.Error(), "path escapes repository") {
		return fmt.Errorf("aarch64 did not continue to path validation: %w", err)
	}
	return nil
}

func kernelRuntimeDepsFixture(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("kernel-runtime-deps requires REPOSITORY")
	}
	ze, err := exec.LookPath("ze")
	if err != nil {
		return err
	}
	scratch, err := copyKernelFixtureTree(args[0])
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	query := func(target, arch, profile string) (string, error) {
		return queryKernelCache(ctx, ze, scratch, target, arch, profile)
	}
	baseline, err := query("runtime", "amd64", "runtime")
	if err != nil {
		return err
	}
	arm, err := query("runtime", "arm64", "runtime")
	if err != nil {
		return err
	}
	installer, err := query("installer", "amd64", "qemu")
	if err != nil {
		return err
	}
	if baseline == arm || baseline == installer {
		return fmt.Errorf("architecture or target did not change cache identity: runtime=%s arm=%s installer=%s", baseline, arm, installer)
	}
	mutationPaths, err := kernelCacheMutationPaths(scratch)
	if err != nil {
		return err
	}
	for _, rel := range mutationPaths {
		if err := assertCacheMutation(ctx, ze, scratch, rel, baseline); err != nil {
			return err
		}
	}
	probe := filepath.Join(scratch, "internal", "appliance", "kernelbuilder", "zz_runtime_deps_probe.go")
	if err := os.WriteFile(probe, []byte("package kernelbuilder\nconst fixtureProbe = true\n"), 0o644); err != nil {
		return err
	}
	changed, err := query("runtime", "amd64", "runtime")
	if err != nil {
		return err
	}
	if changed == baseline {
		return errors.New("new native builder source did not invalidate cache identity")
	}
	if err := os.Remove(probe); err != nil {
		return err
	}
	restored, err := query("runtime", "amd64", "runtime")
	if err != nil {
		return err
	}
	if restored != baseline {
		return fmt.Errorf("cache identity did not return to steady state: got %s want %s", restored, baseline)
	}
	return nil
}
func kernelCacheMutationPaths(root string) ([]string, error) {
	paths := []string{
		"gokrazy/kernel/kernel.config",
		"gokrazy/kernel/kernel.require",
		"gokrazy/kernel/runtime.config",
		"gokrazy/kernel/runtime.require",
		"tools/kernel-builder/common/efi-console.config",
		"tools/kernel-builder/common/efi-console.require",
	}
	for _, sourceRoot := range []string{"tools/kernel-builder", "internal/appliance/kernelbuilder"} {
		absoluteRoot := filepath.Join(root, filepath.FromSlash(sourceRoot))
		err := filepath.WalkDir(absoluteRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if sourceRoot == "tools/kernel-builder" && path != absoluteRoot {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			paths = append(paths, filepath.ToSlash(relative))
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func assertCacheMutation(ctx context.Context, ze, root, rel, baseline string) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(original, []byte("\n// fixture mutation\n")...), info.Mode().Perm()); err != nil {
		return err
	}
	changed, queryErr := queryKernelCache(ctx, ze, root, "runtime", "amd64", "runtime")
	restoreErr := os.WriteFile(path, original, info.Mode().Perm())
	if queryErr != nil || restoreErr != nil {
		return errors.Join(queryErr, restoreErr)
	}
	if changed == baseline {
		return fmt.Errorf("%s did not invalidate cache identity", rel)
	}
	restored, err := queryKernelCache(ctx, ze, root, "runtime", "amd64", "runtime")
	if err != nil {
		return err
	}
	if restored != baseline {
		return fmt.Errorf("%s did not restore cache identity", rel)
	}
	return nil
}

func kernelSharedFragmentFixture(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("kernel-shared-fragment requires REPOSITORY")
	}
	ze, err := exec.LookPath("ze")
	if err != nil {
		return err
	}
	scratch, err := copyKernelFixtureTree(args[0])
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	common := filepath.Join(scratch, "tools", "kernel-builder", "common", "efi-console.config")
	data, err := os.ReadFile(common)
	if err != nil {
		return err
	}
	symbols := []string{"CONFIG_SERIAL_8250_FINTEK", "CONFIG_DRM_SIMPLEDRM", "CONFIG_X86_SYSFB", "CONFIG_FB", "CONFIG_FB_EFI", "CONFIG_FRAMEBUFFER_CONSOLE"}
	for _, symbol := range symbols {
		if !hasExactLine(data, symbol+"=y") {
			return fmt.Errorf("%s missing from shared console fragment", symbol)
		}
		for _, rel := range []string{"gokrazy/kernel/kernel.config", "tools/installer-kernel/hardware.config"} {
			profile, err := os.ReadFile(filepath.Join(scratch, filepath.FromSlash(rel)))
			if err != nil {
				return err
			}
			if hasLinePrefix(profile, symbol+"=") {
				return fmt.Errorf("shared symbol %s remains inline in %s", symbol, rel)
			}
		}
	}
	if err := assertRuntimeKernelComposition(scratch); err != nil {
		return err
	}
	profiles := [][3]string{{"runtime", "amd64", "runtime"}, {"installer", "amd64", "hardware"}, {"installer", "amd64", "hardware-kms"}, {"installer", "amd64", "qemu"}}
	before := make([]string, len(profiles))
	for i, profile := range profiles {
		before[i], err = queryKernelCache(ctx, ze, scratch, profile[0], profile[1], profile[2])
		if err != nil {
			return err
		}
	}
	if err := os.WriteFile(common, append(data, []byte("\n# fixture mutation\n")...), 0o644); err != nil {
		return err
	}
	for i, profile := range profiles {
		after, err := queryKernelCache(ctx, ze, scratch, profile[0], profile[1], profile[2])
		if err != nil {
			return err
		}
		included := profile[2] != "qemu"
		if included == (after == before[i]) {
			return fmt.Errorf("shared fragment membership wrong for %s: before=%s after=%s", profile[2], before[i], after)
		}
	}
	return nil
}
func assertRuntimeKernelComposition(root string) error {
	var combined []byte
	for _, rel := range []string{
		"gokrazy/kernel/kernel.config",
		"gokrazy/kernel/runtime.config",
		"tools/kernel-builder/common/efi-console.config",
	} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		combined = append(combined, data...)
		combined = append(combined, '\n')
	}
	required := []string{
		"CONFIG_MODULES", "CONFIG_PPP", "CONFIG_PPPOL2TP", "CONFIG_L2TP", "CONFIG_L2TP_V3",
		"CONFIG_DEVTMPFS_MOUNT", "CONFIG_BLK_DEV_INITRD", "CONFIG_VIRTIO_NET",
		"CONFIG_DRM_SIMPLEDRM", "CONFIG_X86_SYSFB", "CONFIG_I40E", "CONFIG_ICE",
		"CONFIG_BNXT", "CONFIG_MLX5_CORE", "CONFIG_IGB", "CONFIG_IGC", "CONFIG_NF_TABLES",
		"CONFIG_WIREGUARD", "CONFIG_BRIDGE", "CONFIG_TUN", "CONFIG_MACVLAN", "CONFIG_IPV6",
		"CONFIG_SQUASHFS", "CONFIG_FUSE_FS", "CONFIG_BPF_SYSCALL", "CONFIG_BPF_JIT", "CONFIG_VETH",
	}
	for _, symbol := range required {
		if !hasExactLine(combined, symbol+"=y") {
			return fmt.Errorf("runtime kernel composition missing %s=y", symbol)
		}
	}
	configs := [][]byte{combined}
	installerConfigs, err := filepath.Glob(filepath.Join(root, "tools", "installer-kernel", "*.config"))
	if err != nil {
		return err
	}
	for _, path := range installerConfigs {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		configs = append(configs, data)
	}
	removed := []string{
		"CONFIG_NF_NAT_IPV4", "CONFIG_NF_NAT_MASQUERADE_IPV4", "CONFIG_NFT_MASQ_IPV4",
		"CONFIG_NFT_CHAIN_NAT_IPV4", "CONFIG_NFT_CHAIN_ROUTE_IPV4", "CONFIG_NFT_CHAIN_ROUTE_IPV6",
		"CONFIG_USB_DEVICEFS", "CONFIG_FB_SIMPLE", "CONFIG_L2TP_NETLINK",
	}
	for _, symbol := range removed {
		for _, data := range configs {
			if hasLinePrefix(data, symbol+"=") || hasLinePrefix(data, symbol+" ") {
				return fmt.Errorf("removed kernel option remains configured: %s", symbol)
			}
		}
	}
	return nil
}

func hasExactLine(data []byte, want string) bool {
	for line := range strings.SplitSeq(string(data), "\n") {
		if line == want {
			return true
		}
	}
	return false
}

func hasLinePrefix(data []byte, prefix string) bool {
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func queryKernelCache(ctx context.Context, ze, root, target, arch, profile string) (string, error) {
	cmd := exec.CommandContext(ctx, ze, "appliance", "kernel", "--print-cache-dir", "--target", target, "--arch", arch, "--profile", profile)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "XDG_CACHE_HOME="+filepath.Join(root, "cache"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("query %s/%s/%s cache: %w: %s", target, arch, profile, err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

func copyKernelFixtureTree(repo string) (string, error) {
	root, err := os.MkdirTemp("", "install-kernel-tree-")
	if err != nil {
		return "", err
	}
	for _, rel := range []string{"gokrazy/kernel", "tools/installer-kernel", "tools/kernel-builder", "internal/appliance/kernelbuilder"} {
		if err := copyFixtureTree(filepath.Join(repo, filepath.FromSlash(rel)), filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			os.RemoveAll(root)
			return "", err
		}
	}
	return root, nil
}

func copyFixtureTree(source, dest string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			in.Close()
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		return errors.Join(copyErr, in.Close(), out.Close())
	})
}

func syntheticBuildRequest() (string, kernelbuilder.Request, func(), error) {
	root, err := os.MkdirTemp("", "install-build-")
	if err != nil {
		return "", kernelbuilder.Request{}, func() {}, err
	}
	cleanup := func() { os.RemoveAll(root) }
	for _, dir := range []string{"src", "common", "builder"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			cleanup()
			return "", kernelbuilder.Request{}, func() {}, err
		}
	}
	if err := os.WriteFile(filepath.Join(root, "src", "runtime.config"), []byte("CONFIG_TEST=y\n"), 0o644); err != nil {
		cleanup()
		return "", kernelbuilder.Request{}, func() {}, err
	}
	if err := os.WriteFile(filepath.Join(root, "builder", "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		cleanup()
		return "", kernelbuilder.Request{}, func() {}, err
	}
	req := kernelbuilder.Request{
		Root: root, Version: "7.1.1", Arch: "arm64", Profile: "runtime", Target: "runtime",
		SourceDir: "src", OutputDir: "out", BuilderDir: "builder", CommonDir: "common",
		Modules: "yes", Fragments: []string{"src/runtime.config"}, Image: "fixture-builder",
	}
	return root, req, cleanup, nil
}

func runBuildCapture(ctx context.Context, req *kernelbuilder.Request) (string, error) {
	file, err := os.CreateTemp("", "install-build-output-")
	if err != nil {
		return "", err
	}
	path := file.Name()
	defer os.Remove(path)
	req.Stdout, req.Stderr = file, file
	buildErr := kernelbuilder.Build(ctx, *req)
	closeErr := file.Close()
	data, readErr := os.ReadFile(path)
	return string(data), errors.Join(buildErr, closeErr, readErr)
}

// noopBinary resolves the no-op command these fixtures symlink their stub
// executables to. It is looked up on PATH rather than hardcoded at /bin/true:
// recent macOS releases ship the binary at /usr/bin/true only, and a symlink to
// an absent target is created without error, so every stub then reads as an
// unavailable command and the fixture fails as though the tool were missing.
func noopBinary() (string, error) {
	path, err := exec.LookPath("true")
	if err != nil {
		return "", fmt.Errorf("resolve the no-op stub target: %w", err)
	}
	return path, nil
}

func setFixtureEnv(key, value string) func() {
	old, present := os.LookupEnv(key)
	_ = os.Setenv(key, value)
	return func() {
		if present {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	}
}

func expectBytes(label string, got, want []byte) error {
	if bytes.Equal(got, want) {
		return nil
	}
	return fmt.Errorf("%s: got %q, want %q", label, got, want)
}
