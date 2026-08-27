// Design: docs/architecture/core-design.md -- native Go gates run through le
// Detail: ../gotoolchain/gotoolchain.go -- the environment for the Go command
//
// Package platformvet checks the host and interface package trees against the
// non-Linux implementations that the normal host build does not compile.
package platformvet

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
)

// Platform identifies one cross-target vet action. Zero is not a valid target.
type Platform uint8

const (
	// PlatformDarwin selects the Darwin stubs.
	PlatformDarwin Platform = iota + 1
	// PlatformFreeBSD selects the FreeBSD stubs.
	PlatformFreeBSD
)

// packagePatterns is the complete population the former ze-platform-vet recipe
// judged. No feature tags are added because the producer ran plain `go vet`.
var packagePatterns = [...]string{
	"./internal/component/host/...",
	"./internal/component/iface/...",
	"./internal/plugins/iface/...",
}

type platformSpec struct {
	platform Platform
	goos     string
	gate     string
	why      string
}

var platformSpecs = [...]platformSpec{
	{
		platform: PlatformDarwin,
		goos:     "darwin",
		gate:     "ze-platform-vet-darwin",
		why: "the iface and host trees still compile under GOOS=darwin. Nothing in the " +
			"default host-GOOS build exercises default_other.go, backend_other.go or " +
			"host/platform_other.go, so a stub that stops compiling rots silently",
	},
	{
		platform: PlatformFreeBSD,
		goos:     "freebsd",
		gate:     "ze-platform-vet-freebsd",
		why: "the same trees under GOOS=freebsd. An int64-versus-uint64 syscall.Rlimit " +
			"drift is the shape of break this catches",
	},
}

// Plan is one fully derived vet invocation. Environment is the complete child
// environment. It is excluded from JSON because inherited variables can contain
// credentials, and the action report needs only the declared command contract.
type Plan struct {
	Gate        string   `json:"gate"`
	Platform    string   `json:"platform"`
	Packages    []string `json:"packages"`
	Command     []string `json:"command"`
	Environment []string `json:"-"`
}

// Report is the structured answer for one platform. The child output streams
// unchanged while this report records what ran and its exit code.
type Report struct {
	Gate     string   `json:"gate"`
	Platform string   `json:"platform"`
	Packages []string `json:"packages"`
	Command  []string `json:"command"`
	Code     int      `json:"code"`
	Error    string   `json:"error,omitempty"`
}

type gateExecutor func(string, []string, string, []string) (gaterun.GateReport, int)

// Runner owns the derived toolchain for both platform actions. It reads the
// checkout once when a sweep names Darwin and FreeBSD together.
type Runner struct {
	root    string
	chain   gotoolchain.Toolchain
	execute gateExecutor
}

// NewRunner derives the toolchain data for root. Missing manifest or go.mod
// data is an error instead of a reduced or ambient build.
func NewRunner(root string) (Runner, error) {
	return newRunner(root, gaterun.Run)
}

func newRunner(root string, execute gateExecutor) (Runner, error) {
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

// PackagePatterns returns the exact three package patterns the gate vets.
func PackagePatterns() []string { return slices.Clone(packagePatterns[:]) }

// Plan derives one invocation. GOOS is the only cross-target override. GOARCH
// remains inherited, CGO remains disabled, and no tag flag is added.
func (r Runner) Plan(platform Platform) (Plan, error) {
	spec, err := specFor(platform)
	if err != nil {
		return Plan{}, err
	}
	if err := validatePackages(r.root); err != nil {
		return Plan{}, err
	}

	packages := PackagePatterns()
	command := make([]string, 0, 2+len(packages))
	command = append(command, "go", "vet")
	command = append(command, packages...)

	return Plan{
		Gate:        spec.gate,
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

	_, code := r.execute(plan.Gate, plan.Command, r.root, plan.Environment)
	return Report{
		Gate:     plan.Gate,
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
	packages := PackagePatterns()
	command := make([]string, 0, 2+len(packages))
	command = append(command, "go", "vet")
	command = append(command, packages...)
	return Report{
		Gate:     spec.gate,
		Platform: spec.goos,
		Packages: packages,
		Command:  command,
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
