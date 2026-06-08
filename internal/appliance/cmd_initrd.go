// Design: plan/spec-install-10-iso-prerequisites.md — installer initrd download/build

package appliance

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/helpfmt"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

const (
	defaultInitrdVersion = "v1"
	initrdURLKey         = "ze.appliance.initrd.url"
	initrdToolsDir       = "tools/installer-initrd"
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         initrdURLKey,
	Type:        "string",
	Description: "Base URL for pre-built installer initrd downloads",
})

var (
	initrdMakeBuildFn = defaultInitrdMakeBuild
	initrdLookPathFn  = exec.LookPath
)

func init() {
	cmdInitrd = runInitrd
}

func runInitrd(args []string) int {
	fs := flag.NewFlagSet("appliance initrd", flag.ContinueOnError)

	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze appliance initrd",
			Summary: "Download or build the installer initrd",
			Usage:   []string{"ze appliance initrd"},
			Examples: []string{
				"ze appliance initrd",
			},
		}
		p.Write()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	path, err := resolveInitrd()
	if err != nil {
		cliErrorf("%v", err)
		return exitError
	}

	fmt.Fprintf(os.Stdout, "initrd ready: %s\n", path) //nolint:errcheck // CLI output
	return exitOK
}

func resolveInitrd() (string, error) {
	version := defaultInitrdVersion
	cached := initrdCachePath(version)
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}

	toolsDst := filepath.Join(initrdToolsDir, "build", initrdFileName)

	if baseURL := env.Get(initrdURLKey); baseURL != "" {
		var tb textbuf.Buffer
		artifactURL := tb.Str(baseURL).Byte('/').Str(version).Byte('/').Str(initrdFileName).String()
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

	missing := checkInitrdBuildTools()
	if len(missing) > 0 {
		return "", fmt.Errorf("installer initrd not cached; missing build tools: %v (or set %s for remote download)", missing, initrdURLKey)
	}

	if err := initrdMakeBuildFn(cached); err != nil {
		return "", fmt.Errorf("initrd build: %w", err)
	}

	if cpErr := copyToToolsPath(cached, toolsDst); cpErr != nil {
		fmt.Fprintf(os.Stdout, "warning: copy to %s: %v\n", toolsDst, cpErr) //nolint:errcheck // CLI warning
	}

	return cached, nil
}

func checkInitrdBuildTools() []string {
	var missing []string
	for _, tool := range []string{"busybox", "cpio", "gzip"} {
		if _, err := initrdLookPathFn(tool); err != nil {
			missing = append(missing, tool)
		}
	}
	return missing
}

func defaultInitrdMakeBuild(destPath string) error {
	srcDir, err := filepath.Abs(initrdToolsDir)
	if err != nil {
		return fmt.Errorf("resolve initrd tools dir: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	makeCmd := exec.CommandContext(ctx, "make", "-C", srcDir) //nolint:gosec // controlled args
	makeCmd.Stdout = os.Stdout
	makeCmd.Stderr = os.Stdout
	if err := makeCmd.Run(); err != nil {
		return fmt.Errorf("make initrd: %w", err)
	}

	builtInitrd := filepath.Join(srcDir, "build", initrdFileName)
	if _, err := os.Stat(builtInitrd); err != nil {
		return fmt.Errorf("initrd not produced at %s", builtInitrd)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), cacheDirPerm); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}

	return copyToToolsPath(builtInitrd, destPath)
}
