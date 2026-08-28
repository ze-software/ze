// Design: docs/architecture/core-design.md -- Go builds run under one derived toolchain
// Detail: ../gotoolchain/gotoolchain.go -- cache, compiler, and release identity environment
//
// Package buildartifacts builds the host-side appliance driver and both
// installer initrd binaries. Host and target plans are separate on purpose. A
// target architecture in the invoking environment must never reach ze-host.

package buildartifacts

import (
	"path/filepath"
	"runtime"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const (
	hostAction = "host"
	hostTags   = "ze_core ze_setup"

	installerTags = "ze_installer"
)

// BuildReport is the structured answer from one compiler invocation.
// Environment contains only the toolchain overrides, not inherited variables
// that can include credentials.
type BuildReport struct {
	Action      string   `json:"action"`
	Command     []string `json:"command"`
	Environment []string `json:"environment"`
	Output      string   `json:"output"`
	Writes      bool     `json:"writes"`
	Code        int      `json:"code"`
}

type buildPlan struct {
	action  string
	command []string
	environ gotoolchain.EnvOptions
	output  string
}

type buildRunner func(action string, argv []string, dir string, environ []string) int

func runHost() (any, int) {
	return runAtRoot(hostPlan)
}

// BuildHost builds the native host-side appliance driver in root.
func BuildHost(root string) (BuildReport, int) {
	payload, code := runWithRoot(root, hostPlan, streamBuild)
	report, _ := payload.(BuildReport)
	return report, code
}

func runInstaller(arch string) (any, int) {
	return runAtRoot(func(root string, toolchain gotoolchain.Toolchain) buildPlan {
		return installerPlan(root, toolchain, arch)
	})
}

func runAtRoot(plan func(string, gotoolchain.Toolchain) buildPlan) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runWithRoot(root, plan, streamBuild)
}

func runWithRoot(
	root string,
	plan func(string, gotoolchain.Toolchain) buildPlan,
	run buildRunner,
) (any, int) {
	toolchain, err := gotoolchain.New(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return execute(plan(root, toolchain), toolchain, root, run)
}

func hostPlan(root string, toolchain gotoolchain.Toolchain) buildPlan {
	output := filepath.Join(root, "ze-host")
	return buildPlan{
		action: hostAction,
		command: []string{
			"go", "build",
			"-tags", hostTags,
			"-ldflags", toolchain.LDFlags(),
			"-o", output,
			"./cmd/ze",
		},
		// Pin both host values. Inherited GOOS or GOARCH can name an appliance
		// target, and an executable for that target cannot run on this host.
		environ: gotoolchain.EnvOptions{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH},
		output:  output,
	}
}

func installerPlan(_ string, toolchain gotoolchain.Toolchain, arch string) buildPlan {
	var tb textbuf.Buffer
	output := tb.Str("bin/ze-installer-").Str(arch).String()
	action := tb.Reset().Str("installer-").Str(arch).String()
	return buildPlan{
		action: action,
		command: []string{
			"go", "build",
			"-tags", installerTags,
			"-ldflags", toolchain.LDFlags(),
			"-o", output,
			"./cmd/ze-installer",
		},
		environ: gotoolchain.EnvOptions{GOOS: "linux", GOARCH: arch},
		output:  output,
	}
}

func execute(
	plan buildPlan,
	toolchain gotoolchain.Toolchain,
	root string,
	run buildRunner,
) (BuildReport, int) {
	overrides := toolchain.Overrides(plan.environ)
	code := run(plan.action, plan.command, root, toolchain.Environment(plan.environ))
	report := BuildReport{
		Action:      plan.action,
		Command:     plan.command,
		Environment: overrides,
		Output:      plan.output,
		Writes:      true,
		Code:        code,
	}
	return report, code
}

func streamBuild(action string, argv []string, dir string, environ []string) int {
	_, code := gaterun.Run(action, argv, dir, environ)
	return code
}
