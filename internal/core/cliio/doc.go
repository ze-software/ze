// Package cliio resolves the conventional "-" CLI token to stdin (when reading)
// or stdout (when writing), so every filename-accepting command supports pipes
// through one shared, tier-legal helper instead of ad-hoc `if path == "-"`
// branches or none at all.
//
// The convention ("`-` for stdin", ai/rules/cli.md) predates this
// package and was stated twice but enforced nowhere; the package-private
// normaliser that implemented it (config/cli.loadConfigData) could not be
// reached from other tiers, so most commands drifted. This leaf lives under
// internal/core so every consumer -- component, plugin, analyze, perf, test,
// appliance -- can route through it, and a build gate
// (scripts/checks/cli_dash_stdio.go) fails any command that reads or writes a
// user-supplied path with a raw os call instead of this helper.
//
// stdin is consumable exactly once. ReadFile("-") and OpenReader("-") claim it
// with a fail-closed guard (ai/rules/evidence.md): a second claim in
// the same process returns ErrStdinClaimed rather than a silent empty read.
// stdout is not consumable, so writes to "-" are unguarded. Only the exact token
// "-" is special: no /dev/* handling, no shell expansion.
package cliio
