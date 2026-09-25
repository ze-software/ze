// Design: docs/architecture/core-design.md -- build artifacts are le actions
// Related: ../compile/build.go -- the plan, runner and report both actions use
//
// Package buildinstaller is the `le build installer` command. Its action table
// names one architecture per row, and each row cross-builds the installer
// initrd PID 1 for that architecture. The gate name, reason, and writes marker
// stay with the action, so the listing cannot drift from what runs.

package buildinstaller

import (
	buildcompile "github.com/ze-software/ze/internal/le/build/compile"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

const area = "build installer"

var actions = leaction.New(area,
	leaction.Action{Verb: "amd64", Why: "the installer initrd PID 1 for amd64, at bin/ze-installer-amd64",
		Writes: true,
		Answer: func() (any, int) { return buildcompile.Installer("amd64") }},
	leaction.Action{Verb: "arm64", Why: "the installer initrd PID 1 for arm64, at bin/ze-installer-arm64",
		Writes: true,
		Answer: func() (any, int) { return buildcompile.Installer("arm64") }},
)

// Actions answers the command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the action summary that help renders.
func Subs() string { return actions.Subs() }

// Answer is the `le build installer` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }
