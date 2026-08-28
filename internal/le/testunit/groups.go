// Design: docs/architecture/core-design.md -- component-group unit actions.
// Detail: actions.go -- command dispatch and subprocess execution.
//
// Package testunit runs the six Go test groups: five race-instrumented
// component groups, and the installer initrd behind the ze_installer tag.
package testunit

import (
	"runtime"

	"github.com/ze-software/ze/internal/le/gotoolchain"
)

// Area is the root command that owns the six groups.
const Area = "test-unit"

// Group is one component-group Go test. Pattern is its package population.
type Group struct {
	Verb    string
	Pattern string
	Why     string

	// Tags are build tags the group needs beyond the feature manifest's set.
	// A group that names any takes the bare core tag set as its base, because a
	// tag outside the manifest belongs to a personality rather than a feature.
	Tags []string

	// GOOS is the platform the group's files are written for. Empty means the
	// host, which is every group whose files carry no platform constraint.
	GOOS string

	// Race turns the detector on. Every host group runs under it; a
	// cross-targeted group cannot, because the detector needs cgo.
	Race bool
}

var groups = [...]Group{
	{
		Verb:    "bgp",
		Pattern: "./internal/component/bgp/...",
		Why:     "the BGP component group: reactor, fsm, wire, message, attribute (~1:30)",
		Race:    true,
	},
	{
		Verb:    "core",
		Pattern: "./internal/core/...",
		Why:     "the core leaf libraries every tier above depends on (~30s)",
		Race:    true,
	},
	{
		Verb:    "plugins",
		Pattern: "./internal/plugins/...",
		Why:     "the system plugins: DHCP, NTP, static, firewall, the CLI verb providers (~40s)",
		Race:    true,
	},
	{
		Verb:    "config",
		Pattern: "./internal/component/config/...",
		Why:     "the YANG-modeled config pipeline: file, tree, resolve (~20s)",
		Race:    true,
	},
	{
		Verb:    "cli",
		Pattern: "./internal/component/cli/...",
		Why:     "the CLI: modes, completion, diff, commit, dashboard (~10s)",
		Race:    true,
	},
	{
		// The installer initrd is built with ze_installer, so every _test.go
		// guarded by that tag is invisible to every other group: `go test`
		// without it compiles the package with those files excluded and says
		// nothing. Five files sit in that state, the rescue console's
		// fatal-branch policy among them, which is the code that decides
		// whether a failed install opens a shell, opens nothing, or reboots.
		// They compile and pass; nothing was asking them to run.
		Verb:    "installer",
		Pattern: "./internal/install/...",
		Why:     "the installer initrd's own logic behind the ze_installer tag: bootstrap, console, fault, rescue, initrd (~10s)",
		Tags:    []string{"ze_installer"},
		GOOS:    "linux",
	},
}

// Table returns the component groups in execution order.
func Table() []Group {
	rows := make([]Group, len(groups))
	copy(rows, groups[:])
	return rows
}

// options answers the tag selection this group's commands carry. A group naming
// its own tags takes the bare core set as the base, which is what the retired
// build spelled as -tags 'ze_core ze_installer'.
func (g Group) options() gotoolchain.TestOptions {
	return gotoolchain.TestOptions{Core: len(g.Tags) != 0, Race: g.Race, Tags: g.Tags}
}

// Argv returns the exact Go command for this group.
//
// A group targeting a platform this host is not runs `go vet` instead of
// `go test`, because `go test` cross-compiles the test binary and then tries to
// exec it, which on darwin fails with "fork/exec .../disk.test: exec format
// error". Vet type-checks the tag-guarded files without running them. The real
// execution happens on Linux, and in the Alpine VM through `le qemu all-tests`
// when the host is not Linux (ai/rules/platform-linux.md).
func (g Group) Argv(tc gotoolchain.Toolchain) []string {
	if g.crossTargeted() {
		return tc.GoVet(g.options(), g.Pattern)
	}
	return tc.GoTest(g.options(), g.Pattern)
}

// crossTargeted answers whether this group's platform is not the host's.
func (g Group) crossTargeted() bool {
	return g.GOOS != "" && g.GOOS != runtime.GOOS
}

// EnvOptions returns the toolchain environment this group's command needs. The
// race detector requires cgo, and GOMAXPROCS caps package and test
// concurrency. A group naming a platform pins GOOS, so its files are selected
// by that platform's build constraints rather than by the host's.
func (g Group) EnvOptions() gotoolchain.EnvOptions {
	return gotoolchain.EnvOptions{CGO: g.Race, Procs: true, GOOS: g.GOOS}
}
