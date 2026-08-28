// Design: docs/architecture/core-design.md -- component-group unit actions.
// Detail: actions.go -- command dispatch and subprocess execution.
//
// Package testunit runs five race-instrumented Go test groups.
package testunit

import "github.com/ze-software/ze/internal/le/gotoolchain"

// Area is the root command that owns the five component groups.
const Area = "test-unit"

// Group is one component-group Go test. Pattern is its package population.
type Group struct {
	Verb    string
	Pattern string
	Why     string
}

var groups = [...]Group{
	{
		Verb:    "bgp",
		Pattern: "./internal/component/bgp/...",
		Why:     "the BGP component group: reactor, fsm, wire, message, attribute (~1:30)",
	},
	{
		Verb:    "core",
		Pattern: "./internal/core/...",
		Why:     "the core leaf libraries every tier above depends on (~30s)",
	},
	{
		Verb:    "plugins",
		Pattern: "./internal/plugins/...",
		Why:     "the system plugins: DHCP, NTP, static, firewall, the CLI verb providers (~40s)",
	},
	{
		Verb:    "config",
		Pattern: "./internal/component/config/...",
		Why:     "the YANG-modeled config pipeline: file, tree, resolve (~20s)",
	},
	{
		Verb:    "cli",
		Pattern: "./internal/component/cli/...",
		Why:     "the CLI: modes, completion, diff, commit, dashboard (~10s)",
	},
}

// Table returns the component groups in execution order.
func Table() []Group {
	rows := make([]Group, len(groups))
	copy(rows, groups[:])
	return rows
}

// Argv returns the exact race-instrumented Go test command for this group.
func (g Group) Argv(tc gotoolchain.Toolchain) []string {
	return tc.GoTest(gotoolchain.TestOptions{Race: true}, g.Pattern)
}

// EnvOptions returns the toolchain environment required by a race test. The
// race detector requires cgo, and GOMAXPROCS caps package and test concurrency.
func (Group) EnvOptions() gotoolchain.EnvOptions {
	return gotoolchain.EnvOptions{CGO: true, Procs: true}
}
