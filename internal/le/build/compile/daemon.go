// Design: docs/contributing/preview-artifacts.md -- stamped preview targets.
package buildcompile

import (
	"context"
	"fmt"
	"strings"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// BuildDaemon builds a static Linux distro daemon in the current checkout.
// Identity and output belong to the caller; compiler and feature policy stay
// with the same producers every other native build uses.
func BuildDaemon(ctx context.Context, root, arch, output, version, date string) error {
	if arch != "amd64" && arch != "arm64" {
		return fmt.Errorf("unsupported preview architecture %q", arch)
	}
	tc, err := gotoolchain.New(root)
	if err != nil {
		return err
	}
	tc.Version, tc.BuildDate = version, date
	tags, err := repofeaturetags.DaemonBuildTags(root, repofeaturetags.DaemonBase)
	if err != nil {
		return err
	}
	plan := daemonPlan(tc, arch, output, tags)
	_, code := execute(ctx, plan, tc, root, func(action string, argv []string, dir string, environ []string) int {
		return runHostBuild(ctx, action, argv, dir, previewEnvironment(environ))
	})
	if code != 0 {
		return fmt.Errorf("preview Linux %s build failed with exit %d", arch, code)
	}
	return nil
}

func daemonPlan(tc gotoolchain.Toolchain, arch, output, tags string) buildPlan {
	return buildPlan{action: "preview-" + arch, output: output,
		environ: gotoolchain.EnvOptions{GOOS: "linux", GOARCH: arch},
		command: []string{"go", "build", "-mod=vendor", "-trimpath", "-buildvcs=false", "-tags", tags, "-ldflags", tc.LDFlags(), "-o", output, "./cmd/ze"}}
}

// A preview must not silently read a user's overlay/workspace, experiment or
// machine-specific instruction-set setting. Both listing and compilation use
// this exact environment after the shared toolchain selects its cache and Go.
func previewEnvironment(environ []string) []string {
	overrides := []string{"GOFLAGS=-trimpath", "GOWORK=off", "GOENV=off", "GOEXPERIMENT=", "GOAMD64=v1", "GOARM64=v8.0"}
	result := make([]string, 0, len(environ)+len(overrides))
	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		replaced := false
		for _, override := range overrides {
			if strings.HasPrefix(override, key+"=") {
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, entry)
		}
	}
	return append(result, overrides...)
}
