// Design: docs/architecture/core-design.md -- component-group unit gates as one le area
// Detail: actions.go -- command dispatch and subprocess execution
//
// Package testunit runs the five race-instrumented Go test groups that
// scripts/le/application/unit.py serves during the duplicate-then-swap migration.
package testunit

import "github.com/ze-software/ze/internal/le/gotoolchain"

// Area is the root command that owns the five component-group unit gates.
const Area = "test-unit"

// Gate is one component-group Go test. Pattern is the package population that
// the gate covers.
type Gate struct {
	Name    string
	Pattern string
	Why     string
}

var gates = [...]Gate{
	{
		Name:    "ze-unit-bgp-test",
		Pattern: "./internal/component/bgp/...",
		Why:     "the BGP component group: reactor, fsm, wire, message, attribute (~1:30)",
	},
	{
		Name:    "ze-unit-core-test",
		Pattern: "./internal/core/...",
		Why:     "the core leaf libraries every tier above depends on (~30s)",
	},
	{
		Name:    "ze-unit-plugins-test",
		Pattern: "./internal/plugins/...",
		Why:     "the system plugins: DHCP, NTP, static, firewall, the CLI verb providers (~40s)",
	},
	{
		Name:    "ze-unit-config-test",
		Pattern: "./internal/component/config/...",
		Why:     "the YANG-modeled config pipeline: file, tree, resolve (~20s)",
	},
	{
		Name:    "ze-unit-cli-test",
		Pattern: "./internal/component/cli/...",
		Why:     "the CLI: modes, completion, diff, commit, dashboard (~10s)",
	},
}

// Table returns the component groups in execution order.
func Table() []Gate {
	rows := make([]Gate, len(gates))
	copy(rows, gates[:])
	return rows
}

// Argv returns the exact race-instrumented Go test command for this group.
func (g Gate) Argv(tc gotoolchain.Toolchain) []string {
	return tc.GoTest(gotoolchain.TestOptions{Race: true}, g.Pattern)
}

// EnvOptions returns the toolchain environment required by a race test. The
// race detector requires cgo, and GOMAXPROCS caps package and test concurrency.
func (Gate) EnvOptions() gotoolchain.EnvOptions {
	return gotoolchain.EnvOptions{CGO: true, Procs: true}
}
