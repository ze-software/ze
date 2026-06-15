// Design: plan/learned/856-install-10-iso-prerequisites.md — installer kernel download/build

package appliance

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"io"
	"net/http"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/helpfmt"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

const (
	defaultKernelVersion     = "7.0.11"
	kernelURLKey             = "ze.appliance.kernel.url"
	kernelBuilderDir         = "tools/kernel-builder"
	kernelInstallerConfigDir = "tools/installer-kernel"
	kernelInstallerOutputDir = "tools/installer-kernel/build"
	kernelBuildTimeout       = 120 * time.Minute
	kernelDockerImage        = "ze-installer-kernel-builder"
	builderDocker            = "docker"
	builderQEMU              = "qemu"
	firmwareCacheDir         = "firmware"
	firmwareBaseURL          = "https://git.kernel.org/pub/scm/linux/kernel/git/firmware/linux-firmware.git/plain"
)

var i915FirmwareBlobs = []string{
	"i915/adlp_dmc.bin",
}

var _ = env.MustRegister(env.EnvEntry{
	Key:         kernelURLKey,
	Type:        "string",
	Description: "Base URL for pre-built installer kernel downloads",
})

var (
	kernelQEMUCheckFn   = defaultQEMUCheck
	kernelQEMUBuildFn   = defaultQEMUBuild
	kernelDockerCheckFn = defaultDockerCheck
	kernelDockerBuildFn = defaultDockerBuild
)

func init() {
	cmdKernel = runKernel
}

func runKernel(args []string) int {
	fs := flag.NewFlagSet("appliance kernel", flag.ContinueOnError)
	archFlag := fs.String("arch", "", "Target architecture: amd64 or arm64 (default: from appliance config or host)")
	profileFlag := fs.String("profile", "", "Kernel profile: qemu, hardware, or hardware-kms (default: from appliance config or qemu)")
	builderFlag := fs.String("builder", "", "Build backend: docker or qemu (default: docker if available, else qemu)")
	versionFlag := fs.String("version", defaultKernelVersion, "Linux kernel version")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze appliance kernel",
			Summary: "Download or build the installer kernel",
			Usage:   []string{"ze appliance kernel [options] [<name>]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--arch <arch>", Desc: "Target architecture: amd64 or arm64 (default: from appliance config or host)"},
					{Name: "--profile <profile>", Desc: "Kernel profile: qemu, hardware, or hardware-kms (default: from appliance config or qemu)"},
					{Name: "--builder <backend>", Desc: "Build backend: docker or qemu (default: docker if available, else qemu)"},
					{Name: "--version <ver>", Desc: func() string {
						var tb textbuf.Buffer
						return tb.Str("Linux kernel version (default: ").Str(defaultKernelVersion).Byte(')').String()
					}()},
				}},
			},
			Examples: []string{
				"ze appliance kernel prod",
				"ze appliance kernel --profile hardware prod",
				"ze appliance kernel --builder docker --arch amd64",
				"ze appliance kernel --version 6.12.9 prod",
			},
		}
		p.Write()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	arch := *archFlag
	profile := *profileFlag
	builder := *builderFlag
	if fs.NArg() > 0 {
		name := fs.Arg(0)
		dir := getBaseDir()
		cfg, err := LoadConfig(ConfigPath(dir, name))
		if err != nil {
			cliErrorf("load appliance %q config: %v", name, err)
			return exitError
		}
		if arch == "" {
			arch = cfg.Image.Arch
		}
		if profile == "" && cfg.Image.KernelProfile != "" {
			profile = cfg.Image.KernelProfile
		}
	}
	if arch == "" {
		arch = runtime.GOARCH
	}
	if profile == "" {
		profile = ProfileQEMU
	}

	if arch != archAMD64 && arch != archARM64 {
		cliErrorf("arch %q must be amd64 or arm64", arch)
		return exitError
	}
	if profile != ProfileQEMU && profile != ProfileHardware && profile != ProfileHardwareKMS {
		cliErrorf("profile %q must be qemu, hardware, or hardware-kms", profile)
		return exitError
	}
	if builder != "" && builder != builderDocker && builder != builderQEMU {
		cliErrorf("builder %q must be docker or qemu", builder)
		return exitError
	}

	path, err := resolveKernel(*versionFlag, arch, profile, builder)
	if err != nil {
		cliErrorf("%v", err)
		return exitError
	}

	fmt.Fprintf(os.Stdout, "kernel ready: %s (profile=%s)\n", path, profile) //nolint:errcheck // CLI output
	return exitOK
}

func resolveKernel(version, arch, profile, builder string) (string, error) {
	cached := kernelCachePath(version, kernelCacheVariant(arch, profile))
	toolsDst := filepath.Join(kernelInstallerOutputDir, kernelFileName)

	if _, err := os.Stat(cached); err == nil {
		if cpErr := copyToToolsPath(cached, toolsDst); cpErr != nil {
			fmt.Fprintf(os.Stdout, "warning: copy to %s: %v\n", toolsDst, cpErr) //nolint:errcheck // CLI warning
		}
		return cached, nil
	}

	if baseURL := env.Get(kernelURLKey); baseURL != "" {
		var tb textbuf.Buffer
		artifactURL := tb.Str(baseURL).Byte('/').Str(version).Byte('-').Str(arch).Byte('-').Str(profile).Byte('/').Str(kernelFileName).String()
		checksumURL := tb.Reset().Str(artifactURL).Str(checksumSuffix).String()
		if err := downloadAndVerify(artifactURL, checksumURL, cached); err == nil {
			if cpErr := copyToToolsPath(cached, toolsDst); cpErr != nil {
				fmt.Fprintf(os.Stdout, "warning: copy to %s: %v\n", toolsDst, cpErr) //nolint:errcheck // CLI warning
			}
			return cached, nil
		} else {
			fmt.Fprintf(os.Stdout, "warning: download from %s failed: %v; falling back to local build\n", baseURL, err) //nolint:errcheck // CLI warning
		}
	}

	buildFn, buildName, err := selectBuilder(builder)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(os.Stdout, "building kernel with %s...\n", buildName) //nolint:errcheck // CLI output

	if err := buildFn(version, arch, profile, cached); err != nil {
		return "", fmt.Errorf("%s kernel build: %w", buildName, err)
	}

	if cpErr := copyToToolsPath(cached, toolsDst); cpErr != nil {
		fmt.Fprintf(os.Stdout, "warning: copy to %s: %v\n", toolsDst, cpErr) //nolint:errcheck // CLI warning
	}

	return cached, nil
}

func selectBuilder(builder string) (func(string, string, string, string) error, string, error) {
	switch builder {
	case builderDocker:
		if err := kernelDockerCheckFn(); err != nil {
			return nil, "", fmt.Errorf("docker builder requested but not available: %w", err)
		}
		return kernelDockerBuildFn, builderDocker, nil
	case builderQEMU:
		if err := kernelQEMUCheckFn(); err != nil {
			return nil, "", fmt.Errorf("qemu builder requested but not available: %w", err)
		}
		return kernelQEMUBuildFn, builderQEMU, nil
	default:
		if kernelDockerCheckFn() == nil {
			return kernelDockerBuildFn, builderDocker, nil
		}
		if kernelQEMUCheckFn() == nil {
			return kernelQEMUBuildFn, builderQEMU, nil
		}
		return nil, "", fmt.Errorf("no builder available; install docker or qemu (brew install qemu)")
	}
}

func defaultDockerCheck() error {
	if _, err := exec.LookPath(builderDocker); err != nil {
		return fmt.Errorf("docker not found")
	}
	return nil
}

func defaultDockerBuild(version, arch, profile, destPath string) error {
	builderDir, err := filepath.Abs(kernelBuilderDir)
	if err != nil {
		return fmt.Errorf("resolve builder directory: %w", err)
	}
	configDir, err := filepath.Abs(kernelInstallerConfigDir)
	if err != nil {
		return fmt.Errorf("resolve installer config directory: %w", err)
	}
	outDir, err := filepath.Abs(kernelInstallerOutputDir)
	if err != nil {
		return fmt.Errorf("resolve installer output directory: %w", err)
	}
	platform, err := dockerPlatform(arch)
	if err != nil {
		return err
	}
	fwDir, err := ensureFirmware(profile)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(destPath), cacheDirPerm); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}
	if err := os.MkdirAll(outDir, cacheDirPerm); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	buildCtx, buildCancel := context.WithTimeout(context.Background(), kernelBuildTimeout)
	defer buildCancel()

	buildCmd := exec.CommandContext(buildCtx, builderDocker, "build", "--platform", platform, "-t", kernelDockerImage, builderDir) //nolint:gosec // controlled args
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stdout
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("docker build: %w", err)
	}

	var tb textbuf.Buffer
	dockerArgs := []string{"run", "--rm", "--platform", platform,
		"-e", tb.Str("LINUX_VERSION=").Str(version).String(),
		"-e", tb.Reset().Str("ARCH=").Str(arch).String(),
		"-e", tb.Reset().Str("PROFILE=").Str(profile).String(),
		"-e", "SRC_DIR=/src",
		"-e", "OUT_DIR=/out",
		"-v", tb.Reset().Str(builderDir).Str(":/builder:ro").String(),
		"-v", tb.Reset().Str(configDir).Str(":/src:ro").String(),
		"-v", tb.Reset().Str(outDir).Str(":/out").String(),
	}
	if fwDir != "" {
		absFW, absErr := filepath.Abs(fwDir)
		if absErr != nil {
			return fmt.Errorf("resolve firmware directory: %w", absErr)
		}
		dockerArgs = append(dockerArgs,
			"-v", tb.Reset().Str(absFW).Str(":/firmware:ro").String(),
			"-e", "FIRMWARE_DIR=/firmware",
		)
	}
	dockerArgs = append(dockerArgs, kernelDockerImage, "sh", "/builder/build.sh")
	runCmd := exec.CommandContext(buildCtx, builderDocker, dockerArgs...) //nolint:gosec // controlled args
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stdout
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("docker run: %w", err)
	}

	builtKernel := filepath.Join(outDir, kernelFileName)
	if _, err := os.Stat(builtKernel); err != nil {
		return fmt.Errorf("kernel not produced at %s", builtKernel)
	}

	return copyToToolsPath(builtKernel, destPath)
}

func dockerPlatform(arch string) (string, error) {
	switch arch {
	case archAMD64:
		return "linux/amd64", nil
	case archARM64:
		return "linux/arm64", nil
	default:
		return "", fmt.Errorf("arch %q must be amd64 or arm64", arch)
	}
}

func defaultQEMUCheck() error {
	if _, err := exec.LookPath("python3"); err != nil {
		return fmt.Errorf("python3 not found; required for QEMU kernel build")
	}
	for _, bin := range []string{"qemu-system-aarch64", "qemu-system-x86_64"} {
		if _, err := exec.LookPath(bin); err == nil {
			return nil
		}
	}
	return fmt.Errorf("QEMU not found; install with: brew install qemu")
}

func defaultQEMUBuild(version, arch, profile, destPath string) error {
	scriptPath, err := filepath.Abs(filepath.Join(kernelBuilderDir, "qemu-build.py"))
	if err != nil {
		return fmt.Errorf("resolve build script path: %w", err)
	}
	fwDir, err := ensureFirmware(profile)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(destPath), cacheDirPerm); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}
	if err := os.MkdirAll(kernelInstallerOutputDir, cacheDirPerm); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	buildCtx, buildCancel := context.WithTimeout(context.Background(), kernelBuildTimeout)
	defer buildCancel()

	qemuArgs := []string{scriptPath,
		"--arch", arch,
		"--profile", profile,
		"--version", version,
		"--builder-dir", kernelBuilderDir,
		"--src-dir", kernelInstallerConfigDir,
		"--out-dir", kernelInstallerOutputDir,
	}
	if fwDir != "" {
		qemuArgs = append(qemuArgs, "--firmware-dir", fwDir)
	}
	cmd := exec.CommandContext(buildCtx, "python3", qemuArgs...) //nolint:gosec // controlled args
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("qemu-build.py: %w", err)
	}

	builtKernel := filepath.Join(kernelInstallerOutputDir, kernelFileName)
	if _, err := os.Stat(builtKernel); err != nil {
		return fmt.Errorf("kernel not produced at %s", builtKernel)
	}

	return copyToToolsPath(builtKernel, destPath)
}

func ensureFirmware(profile string) (string, error) {
	if profile != ProfileHardwareKMS {
		return "", nil
	}
	fwDir := filepath.Join(ResolveCacheDir(), firmwareCacheDir)
	for _, blob := range i915FirmwareBlobs {
		dest := filepath.Join(fwDir, blob)
		if _, err := os.Stat(dest); err == nil {
			continue
		}
		var tb textbuf.Buffer
		url := tb.Str(firmwareBaseURL).Byte('/').Str(blob).String()
		if err := downloadFirmwareBlob(url, dest); err != nil {
			return "", fmt.Errorf("download firmware %s: %w", blob, err)
		}
		fmt.Fprintf(os.Stdout, "  firmware: %s\n", blob) //nolint:errcheck // CLI output
	}
	return fwDir, nil
}

func downloadFirmwareBlob(rawURL, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), cacheDirPerm); err != nil {
		return fmt.Errorf("create firmware directory: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody) //nolint:gosec // URL from constant
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose // closed below
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // HTTP response
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, rawURL)
	}
	tmp, err := os.CreateTemp(filepath.Dir(destPath), ".fw-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		os.Remove(tmpPath) //nolint:errcheck // cleanup
	}()
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close() //nolint:errcheck // cleanup
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, destPath)
}

func cliErrorf(format string, args ...any) {
	fmt.Fprintf(os.Stdout, "error: %s\n", fmt.Sprintf(format, args...)) //nolint:errcheck // CLI output
}
