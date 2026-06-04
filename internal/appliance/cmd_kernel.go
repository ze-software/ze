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

	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/helpfmt"
)

const (
	defaultKernelVersion = "7.0.11"
	kernelURLKey         = "ze.appliance.kernel.url"
	defaultKernelBaseURL = "https://codeberg.org/thomas-mangin/ze/releases/download/installer-kernel"
	kernelDockerImage    = "ze-installer-kernel-builder"
	kernelToolsDir       = "tools/installer-kernel"
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         kernelURLKey,
	Type:        "string",
	Description: "Base URL for pre-built installer kernel downloads",
})

var (
	kernelDockerCheckFn = defaultDockerCheck
	kernelDockerBuildFn = defaultDockerBuild
)

func init() {
	cmdKernel = runKernel
}

func runKernel(args []string) int {
	fs := flag.NewFlagSet("appliance kernel", flag.ContinueOnError)
	archFlag := fs.String("arch", runtime.GOARCH, "Target architecture (amd64 or arm64)")
	versionFlag := fs.String("version", defaultKernelVersion, "Linux kernel version")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze appliance kernel",
			Summary: "Download or build the installer kernel",
			Usage:   []string{"ze appliance kernel [--arch <arch>] [--version <version>]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--arch <arch>", Desc: "Target architecture: amd64 or arm64 (default: host)"},
					{Name: "--version <ver>", Desc: "Linux kernel version (default: " + defaultKernelVersion + ")"},
				}},
			},
			Examples: []string{
				"ze appliance kernel",
				"ze appliance kernel --arch amd64",
				"ze appliance kernel --version 6.12.9 --arch arm64",
			},
		}
		p.Write()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	arch := *archFlag
	if arch != archAMD64 && arch != archARM64 {
		cliErrorf("arch %q must be amd64 or arm64", arch)
		return exitError
	}

	path, err := resolveKernel(*versionFlag, arch)
	if err != nil {
		cliErrorf("%v", err)
		return exitError
	}

	fmt.Fprintf(os.Stdout, "kernel ready: %s\n", path) //nolint:errcheck // CLI output
	return exitOK
}

func resolveKernel(version, arch string) (string, error) {
	cached := kernelCachePath(version, arch)
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}

	baseURL := env.Get(kernelURLKey)
	if baseURL == "" {
		baseURL = defaultKernelBaseURL
	}
	artifactURL := baseURL + "/" + version + "-" + arch + "/" + kernelFileName
	checksumURL := artifactURL + checksumSuffix

	toolsDst := filepath.Join(kernelToolsDir, "build", kernelFileName)

	if err := downloadAndVerify(artifactURL, checksumURL, cached); err == nil {
		if cpErr := copyToToolsPath(cached, toolsDst); cpErr != nil {
			fmt.Fprintf(os.Stdout, "warning: copy to %s: %v\n", toolsDst, cpErr) //nolint:errcheck // CLI warning
		}
		return cached, nil
	}

	if err := kernelDockerCheckFn(); err != nil {
		return "", fmt.Errorf("installer kernel not cached and download failed; install Docker to build locally or set %s", kernelURLKey)
	}

	if err := kernelDockerBuildFn(version, arch, cached); err != nil {
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

func defaultDockerBuild(version, arch, destPath string) error {
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

	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	buildCmd := exec.CommandContext(ctx, "docker", "build", "-t", kernelDockerImage, ".") //nolint:gosec // controlled args
	buildCmd.Dir = srcDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stdout
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("docker build: %w", err)
	}

	runCmd := exec.CommandContext(ctx, "docker", "run", "--rm", //nolint:gosec // controlled args
		"-e", "LINUX_VERSION="+version,
		"-e", "ARCH="+arch,
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
