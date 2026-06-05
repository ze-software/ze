// Design: plan/spec-install-10-iso-prerequisites.md — installer kernel download/build

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

	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/helpfmt"
)

const (
	defaultKernelVersion = "7.0.11"
	kernelURLKey         = "ze.appliance.kernel.url"
	kernelDockerImage    = "ze-installer-kernel-builder"
	kernelToolsDir       = "tools/installer-kernel"
	kernelBuildTimeout   = 60 * time.Minute
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         kernelURLKey,
	Type:        "string",
	Description: "Base URL for pre-built installer kernel downloads",
})

var (
	kernelDockerCheckFn = defaultDockerCheck
	kernelDockerBuildFn = defaultDockerBuild // func(version, arch, profile, destPath string) error
)

func init() {
	cmdKernel = runKernel
}

func runKernel(args []string) int {
	fs := flag.NewFlagSet("appliance kernel", flag.ContinueOnError)
	archFlag := fs.String("arch", "", "Target architecture: amd64 or arm64 (default: from appliance config or host)")
	profileFlag := fs.String("profile", "", "Kernel profile: qemu or hardware (default: from appliance config or qemu)")
	versionFlag := fs.String("version", defaultKernelVersion, "Linux kernel version")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze appliance kernel",
			Summary: "Download or build the installer kernel",
			Usage:   []string{"ze appliance kernel [options] [<name>]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--arch <arch>", Desc: "Target architecture: amd64 or arm64 (default: from appliance config or host)"},
					{Name: "--profile <profile>", Desc: "Kernel profile: qemu or hardware (default: from appliance config or qemu)"},
					{Name: "--version <ver>", Desc: "Linux kernel version (default: " + defaultKernelVersion + ")"},
				}},
			},
			Examples: []string{
				"ze appliance kernel prod",
				"ze appliance kernel --profile hardware prod",
				"ze appliance kernel --arch amd64",
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
	if profile != ProfileQEMU && profile != ProfileHardware {
		cliErrorf("profile %q must be qemu or hardware", profile)
		return exitError
	}

	path, err := resolveKernel(*versionFlag, arch, profile)
	if err != nil {
		cliErrorf("%v", err)
		return exitError
	}

	fmt.Fprintf(os.Stdout, "kernel ready: %s (profile=%s)\n", path, profile) //nolint:errcheck // CLI output
	return exitOK
}

func resolveKernel(version, arch, profile string) (string, error) {
	cached := kernelCachePath(version, arch+"-"+profile)
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}

	toolsDst := filepath.Join(kernelToolsDir, "build", kernelFileName)

	if baseURL := env.Get(kernelURLKey); baseURL != "" {
		artifactURL := baseURL + "/" + version + "-" + arch + "-" + profile + "/" + kernelFileName
		checksumURL := artifactURL + checksumSuffix
		if err := downloadAndVerify(artifactURL, checksumURL, cached); err == nil {
			if cpErr := copyToToolsPath(cached, toolsDst); cpErr != nil {
				fmt.Fprintf(os.Stdout, "warning: copy to %s: %v\n", toolsDst, cpErr) //nolint:errcheck // CLI warning
			}
			return cached, nil
		} else {
			fmt.Fprintf(os.Stdout, "warning: download from %s failed: %v; falling back to local build\n", baseURL, err) //nolint:errcheck // CLI warning
		}
	}

	if err := kernelDockerCheckFn(); err != nil {
		return "", fmt.Errorf("installer kernel not cached; install Docker to build locally or set %s for remote download", kernelURLKey)
	}

	if err := kernelDockerBuildFn(version, arch, profile, cached); err != nil {
		return "", fmt.Errorf("docker kernel build: %w", err)
	}

	if cpErr := copyToToolsPath(cached, toolsDst); cpErr != nil {
		fmt.Fprintf(os.Stdout, "warning: copy to %s: %v\n", toolsDst, cpErr) //nolint:errcheck // CLI warning
	}

	return cached, nil
}

func defaultDockerCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "info") //nolint:gosec // fixed command
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func defaultDockerBuild(version, arch, profile, destPath string) error {
	srcDir, err := filepath.Abs(kernelToolsDir)
	if err != nil {
		return fmt.Errorf("resolve kernel tools dir: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), cacheDirPerm); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}

	outDir, err := os.MkdirTemp("", "ze-kernel-build-*")
	if err != nil {
		return fmt.Errorf("create temp output dir: %w", err)
	}
	defer os.RemoveAll(outDir) //nolint:errcheck // cleanup

	buildCtx, buildCancel := context.WithTimeout(context.Background(), kernelBuildTimeout)
	defer buildCancel()

	buildCmd := exec.CommandContext(buildCtx, "docker", "build", "-t", kernelDockerImage, ".") //nolint:gosec // controlled args
	buildCmd.Dir = srcDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stdout
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("docker build: %w", err)
	}

	runCtx, runCancel := context.WithTimeout(context.Background(), kernelBuildTimeout)
	defer runCancel()

	runCmd := exec.CommandContext(runCtx, "docker", "run", "--rm", //nolint:gosec // controlled args
		"-e", "LINUX_VERSION="+version,
		"-e", "ARCH="+arch,
		"-e", "PROFILE="+profile,
		"-v", srcDir+":/src:ro",
		"-v", outDir+":/out",
		kernelDockerImage, "sh", "/src/build.sh",
	)
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

func cliErrorf(format string, args ...any) {
	fmt.Fprintf(os.Stdout, "error: "+format+"\n", args...) //nolint:errcheck // CLI output
}
