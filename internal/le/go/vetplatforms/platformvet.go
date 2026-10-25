// Design: docs/architecture/core-design.md -- native Go checks run through le
// Detail: ../gotoolchain/gotoolchain.go -- the environment for the Go command
//
// Package govetplatforms type-checks the shipped tree under the non-Linux
// targets that the normal host build does not compile.
//
// The lint flavors cannot stand in for it. A lint flavor claims a package
// only when that flavor selects a file no earlier pass selected, so a package
// whose files are all shared (internal/test/runner, internal/core/diskspace) is
// linted once, under the host GOOS, and never type-checked against FreeBSD's
// signed Rlimit or OpenBSD's F_-prefixed Statfs_t. This action vets every
// package under every target it names, so such a break fails here.
package govetplatforms

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/le/gaterun"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// Platform identifies one cross-target vet action. Zero is not a valid target.
type Platform uint8

const (
	// PlatformDarwin selects the Darwin stubs.
	PlatformDarwin Platform = iota + 1
	// PlatformFreeBSD selects the FreeBSD stubs.
	PlatformFreeBSD
	// PlatformOpenBSD selects the OpenBSD stubs.
	PlatformOpenBSD
)

// packagePatterns is the complete population each platform action vets: the
// shipped tree, with its test files.
var packagePatterns = [...]string{
	"./cmd/...",
	"./internal/...",
	"./pkg/...",
}

// personalityTags are added to the manifest's feature tags, so the files every
// shipped personality selects are type-checked together, as the lint
// "personalities" flavor does for Linux.
var personalityTags = []string{"ze_distro", "ze_appliance", "ze_setup"}

// vetFlags turns off the one analyzer the tree is known to trip on purpose:
// internal/core/textbuf hides a pointer from escape analysis with a uintptr
// round trip, and golangci-lint's govet carries the reviewed nolint for it.
// The action exists for compile correctness; the full analyzer set is lint's.
var vetFlags = []string{"-unsafeptr=false"}

type platformSpec struct {
	platform Platform
	goos     string
	verb     string
	summary  string
	why      string
}

var platformSpecs = [...]platformSpec{
	{
		platform: PlatformDarwin,
		goos:     "darwin",
		verb:     "darwin",
		summary:  "type-check the shipped tree under GOOS=darwin",
		why: "the shipped tree still compiles under GOOS=darwin. A Linux host build never " +
			"selects default_other.go, backend_other.go or host/platform_other.go, so a stub " +
			"that stops compiling rots silently",
	},
	{
		platform: PlatformFreeBSD,
		goos:     "freebsd",
		verb:     "freebsd",
		summary:  "type-check the shipped tree under GOOS=freebsd",
		why: "the shipped tree under GOOS=freebsd. An int64-versus-uint64 syscall.Rlimit " +
			"drift is the shape of break this catches",
	},
	{
		platform: PlatformOpenBSD,
		goos:     "openbsd",
		verb:     "openbsd",
		summary:  "type-check the shipped tree under GOOS=openbsd",
		why: "the shipped tree under GOOS=openbsd, whose Statfs_t names its fields F_bavail " +
			"and F_bsize, so code reading Bavail compiles everywhere else",
	},
}

// Plan is one fully derived vet invocation. Environment is the complete child
// environment. It is excluded from JSON because inherited variables can contain
// credentials, and the action report needs only the declared command contract.
type Plan struct {
	Action      string   `json:"action"`
	Platform    string   `json:"platform"`
	Packages    []string `json:"packages"`
	Command     []string `json:"command"`
	Environment []string `json:"-"`
}

// Report is the structured answer for one platform. The child output streams
// unchanged while this report records what ran and its exit code.
type Report struct {
	Action   string   `json:"action"`
	Platform string   `json:"platform"`
	Packages []string `json:"packages"`
	Command  []string `json:"command"`
	Code     int      `json:"code"`
	Error    string   `json:"error,omitempty"`
}

type actionExecutor func(string, []string, string, []string) (gaterun.ActionReport, int)

// Runner owns the derived toolchain for every platform action. It reads the
// checkout once when a sweep names several platforms together.
type Runner struct {
	root    string
	chain   gotoolchain.Toolchain
	execute actionExecutor
}

// NewRunner derives the toolchain data for root. Missing manifest or go.mod
// data is an error instead of a reduced or ambient build.
func NewRunner(root string) (Runner, error) {
	return newRunner(root, gaterun.Run)
}

func newRunner(root string, execute actionExecutor) (Runner, error) {
	if root == "" {
		return Runner{}, fmt.Errorf("platform vet checkout root is empty")
	}
	chain, err := gotoolchain.New(root)
	if err != nil {
		return Runner{}, fmt.Errorf("derive platform vet toolchain: %w", err)
	}
	if execute == nil {
		return Runner{}, fmt.Errorf("platform vet command executor is missing")
	}
	return Runner{root: root, chain: chain, execute: execute}, nil
}

// packagePatternsForVet returns the exact package patterns the action vets.
func packagePatternsForVet() []string { return slices.Clone(packagePatterns[:]) }

// vetCommand answers the argv one platform runs: the manifest's tags plus the
// personality tags, the vet flags, then the packages.
func (r Runner) vetCommand(packages []string) []string {
	args := make([]string, 0, len(vetFlags)+len(packages))
	args = append(args, vetFlags...)
	args = append(args, packages...)
	return r.chain.GoVet(gotoolchain.TestOptions{Tags: personalityTags}, args...)
}

// Plan derives one invocation. GOOS is the only cross-target override. GOARCH
// remains inherited and CGO remains disabled.
func (r Runner) Plan(platform Platform) (Plan, error) {
	spec, err := specFor(platform)
	if err != nil {
		return Plan{}, err
	}
	if err := validatePackages(r.root); err != nil {
		return Plan{}, err
	}

	packages := packagePatternsForVet()
	command := r.vetCommand(packages)

	return Plan{
		Action:      spec.verb,
		Platform:    spec.goos,
		Packages:    packages,
		Command:     command,
		Environment: r.chain.Environment(gotoolchain.EnvOptions{GOOS: spec.goos}),
	}, nil
}

// Run executes one plan with direct stdin, stdout, and stderr access. The child
// exit code is returned unchanged.
func (r Runner) Run(platform Platform) (Report, int) {
	plan, err := r.Plan(platform)
	if err != nil {
		report := failedReport(platform, err)
		leaction.ReportError(err)
		return report, report.Code
	}

	_, code := r.execute(plan.Action, plan.Command, r.root, plan.Environment)
	return Report{
		Action:   plan.Action,
		Platform: plan.Platform,
		Packages: slices.Clone(plan.Packages),
		Command:  slices.Clone(plan.Command),
		Code:     code,
	}, code
}

func failedReport(platform Platform, err error) Report {
	spec, specErr := specFor(platform)
	if specErr != nil {
		return Report{Code: 2, Error: err.Error()}
	}
	// No command ran, and none is reported: the toolchain that would have
	// derived its tags is what failed.
	return Report{
		Action:   spec.verb,
		Platform: spec.goos,
		Packages: packagePatternsForVet(),
		Code:     2,
		Error:    err.Error(),
	}
}

func specFor(platform Platform) (platformSpec, error) {
	for _, spec := range platformSpecs {
		if spec.platform == platform {
			return spec, nil
		}
	}
	return platformSpec{}, fmt.Errorf("unknown platform vet target %d", platform)
}

// validatePackages refuses a missing pattern before it can be mistaken for a
// successful vet over a smaller population.
func validatePackages(root string) error {
	for _, pattern := range packagePatterns {
		relative := strings.TrimSuffix(strings.TrimPrefix(pattern, "./"), "/...")
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("read platform vet package %s: %w", pattern, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("platform vet package %s is not a directory", pattern)
		}
	}
	return nil
}
