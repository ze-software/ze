// Design: docs/architecture/core-design.md -- build artifacts are le actions
// Related: ../compile/build.go -- the plan, runner and report this command uses
//
// Package buildhostdriver is the `le build host-driver` command. It builds
// ze-host, the `ze appliance ...` driver that runs on the BUILD machine, at the
// checkout root. The command owns the kernel cache key, so every QEMU target
// declares it as a prerequisite before taking the staged-kernel guard.

package buildhostdriver

import (
	buildcompile "github.com/ze-software/ze/internal/le/build/compile"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

const name = "build host-driver"

// Answer is the `le build host-driver` command. There is one host and one
// output path, so there is nothing to type after the name.
func Answer(args []string) (any, int) {
	if len(args) > 0 {
		return nil, leroot.RefuseArgument(name, args[0])
	}
	return buildcompile.Host()
}
