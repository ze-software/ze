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

// RuntimeKernelBuild answers the commands, one per line and in order, that
// build Ze's runtime kernel for arch from this checkout: ze-host first, because
// `appliance` is a ze_setup command and the ./ze wrapper builds without it
// ("unknown command: appliance"), then its kernel build. Every le refusal that
// sends an operator to build the runtime kernel prints this, so the command is
// declared once. builder is the --builder backend (docker or qemu) the
// refusing command resolved the cache for; empty leaves ze-host's own choice,
// which names a different cache entry when the backend differs.
func RuntimeKernelBuild(arch, builder string) string {
	build := "  ./le " + name + "\n  ./" + buildcompile.HostBinary +
		" appliance kernel --target runtime --arch " + arch
	if builder == "" {
		return build
	}
	return build + " --builder " + builder
}

// Answer is the `le build host-driver` command. There is one host and one
// output path, so there is nothing to type after the name.
func Answer(args []string) (any, int) {
	if len(args) > 0 {
		return nil, leroot.RefuseArgument(name, args[0])
	}
	return buildcompile.Host()
}
