// Design: docs/architecture/core-design.md -- build artifacts are one le action area
//
// This table is the command surface for the three release-artifact builds. The
// gate name, reason, and writes marker stay with the function that performs each
// build, so the listing and parity claim cannot drift from execution.

package buildartifacts

import "github.com/ze-software/ze/internal/le/leaction"

const area = "build-artifacts"

var actions = leaction.New(area,
	leaction.Action{
		Gate: "ze-host-build",
		Why: "ze-host, the `ze appliance ...` driver that runs on the BUILD machine. " +
			"It owns the kernel cache key, so every QEMU target declares it as a " +
			"prerequisite before taking the staged-kernel guard",
		Writes: true,
		Answer: runHost,
	},
	leaction.Action{
		Gate:   "ze-installer-build-amd64",
		Why:    "the installer initrd PID 1 for amd64, at bin/ze-installer-amd64",
		Writes: true,
		Answer: func() (any, int) { return runInstaller("amd64") },
	},
	leaction.Action{
		Gate:   "ze-installer-build-arm64",
		Why:    "the installer initrd PID 1 for arm64, at bin/ze-installer-arm64",
		Writes: true,
		Answer: func() (any, int) { return runInstaller("arm64") },
	},
)

// Actions answers the command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the action summary that help renders.
func Subs() string { return actions.Subs() }

// Answer is the `le build-artifacts` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }
