// Design: docs/guide/appliance.md -- gokrazy build tool wrapper
// Related: register.go -- the registration of this command

// Package buildgokrazy is `le build gokrazy`, the gokrazy build tool run
// in-process with a project-local module cache and a prepared instance.
// It replaced a standalone gokrazy program, whose tests moved here.
package buildgokrazy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gokrazy/tools/gok"

	"github.com/ze-software/ze/internal/appliance/instance"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var _ = env.MustRegister(env.EnvEntry{Key: "ze.gok.debug", Type: "bool", Description: "Print `le build gokrazy` debug output (resolved GOMODCACHE path and arguments)"})

// The out-of-tree kernel is selected per build, never by leftover state. It is
// an env var rather than a YANG leaf because it is a developer knob for testing
// a locally built kernel, not something an operator sets on an appliance
// (ai/rules/config.md). `./ze appliance kernel --target runtime` prints the exact command.
var _ = env.MustRegister(env.EnvEntry{Key: "ze.gok.kernel-package", Type: "string", Description: "Path to an out-of-tree kernel package to build the appliance image against (default: the pinned github.com/rtr7/kernel)"})

const (
	// parentDirFlag is the gok flag naming the directory that contains one
	// subdirectory per instance (vendor/github.com/gokrazy/internal/instanceflag).
	parentDirFlag = "--parent_dir"
	// parentDirFlagEq is the inline-value spelling pflag also accepts.
	parentDirFlagEq = "--parent_dir="
)

// buildingSubcommands are the gok subcommands that BUILD an image and never
// write back into the instance directory, so redirecting them to a prepared copy
// is safe and correct.
//
// Deliberately an allowlist, not a denylist. The mutating subcommands (new, add,
// edit, get) write the operator's change into the instance dir; preparing one of
// those would land the edit in a temp copy that is deleted moments later, losing
// it silently. An unknown or future subcommand therefore passes through
// unprepared, which is the safe default. Before adding a verb here, confirm it
// does not write to the instance directory.
var buildingSubcommands = map[string]bool{
	"overwrite": true,
}

// globalValueFlags are gok's global flags that take a SEPARATE value, so the
// token after them is a value and never the subcommand.
var globalValueFlags = map[string]bool{
	parentDirFlag: true,
	"--instance":  true,
	"-i":          true,
}

// subcommandOf returns the first bare (non-flag) token that is not the value of
// a global value-taking flag, which is gok's subcommand.
//
// Scanning for a known verb anywhere in the argument list would be wrong in a
// way that loses data: `gok edit --instance overwrite` would then look like a
// build, and preparing an `edit` writes the operator's change into a temp copy
// that is deleted moments later.
func subcommandOf(args []string) string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if globalValueFlags[a] {
			i++ // skip the value
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue // a flag, or a flag with an inline =value
		}
		return a
	}
	return ""
}

// prepareArgs rewrites gok's --parent_dir to a prepared copy under the project
// tmp/ for image-building subcommands, so a build never runs from, or writes to,
// the tracked gokrazy directory. It returns the rewritten arguments and a
// cleanup func (always non-nil, safe to defer).
//
// The prepared copy carries the instance's builddir. Without it gok synthesizes
// an empty module and resolves every package over the network, discarding the
// pins (see internal/appliance/instance). Preparation failure is therefore an
// error, never a fallthrough to the tracked dir: falling through would produce a
// silently unpinned image.
//
// With no --parent_dir there is no repo-local instance to prepare and gok uses
// its own default instance directory, so the arguments pass through unchanged.
func prepareArgs(args []string) ([]string, func(), error) {
	noop := func() {}

	if !buildingSubcommands[subcommandOf(args)] {
		return args, noop, nil
	}

	for i, a := range args {
		var src string
		switch {
		case a == parentDirFlag && i+1 < len(args):
			src = args[i+1]
		case strings.HasPrefix(a, parentDirFlagEq):
			src = strings.TrimPrefix(a, parentDirFlagEq)
		default:
			continue
		}

		abs, err := filepath.Abs(src)
		if err != nil {
			return nil, noop, fmt.Errorf("resolve %s %s: %w", parentDirFlag, src, err)
		}
		prepared, cleanup, err := instance.Prepare(abs, instance.Options{KernelPackage: env.Get("ze.gok.kernel-package")})
		if err != nil {
			return nil, noop, fmt.Errorf("prepare gokrazy instance from %s: %w", abs, err)
		}

		out := make([]string, len(args))
		copy(out, args)
		if a == parentDirFlag {
			out[i+1] = prepared
		} else {
			var tb textbuf.Buffer
			out[i] = tb.Str(parentDirFlagEq).Str(prepared).String()
		}
		return out, cleanup, nil
	}

	return args, noop, nil
}

// name is the command as a developer types it after `le`.
const name = "build gokrazy"

// Run runs gok with args and answers the exit code. It sets GOMODCACHE,
// CGO_ENABLED, GOFLAGS and GOPROXY in this process's environment for the Go
// subprocesses gok spawns, so the caller MUST be a process that runs one gok
// command and exits, as le does.
func Run(args []string) int {
	modcache := os.Getenv("GOMODCACHE")
	if modcache == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fail(err)
		}
		modcache = filepath.Join(wd, "gokrazy", "modcache")
	}
	if err := os.MkdirAll(modcache, 0o750); err != nil {
		return fail(err)
	}
	if err := os.Setenv("GOMODCACHE", modcache); err != nil {
		return fail(fmt.Errorf("setenv: %w", err))
	}
	// gok spawns Go build and list subprocesses. Keep their target binaries
	// CGO-free and their checked-in module cache user-writable.
	// Mirrors appliance.runGokBuild (not imported: too heavy here).
	if err := os.Setenv("CGO_ENABLED", "0"); err != nil {
		return fail(fmt.Errorf("setenv: %w", err))
	}

	if goflags := os.Getenv("GOFLAGS"); !strings.Contains(goflags, "-modcacherw") {
		var tb textbuf.Buffer
		if goflags != "" {
			tb.Str(goflags).Byte(' ')
		}
		tb.Str("-modcacherw")
		if err := os.Setenv("GOFLAGS", tb.String()); err != nil {
			return fail(fmt.Errorf("setenv: %w", err))
		}
	}

	// Resolve the module graph strictly from the checked-in modcache. gok reads
	// the ambient GOPROXY (vendor/.../packer/gotool.go getIncomplete) and does NOT
	// force offline, so a module missing from the builddir/modcache would silently
	// resolve over the network to a NEWER version than the pins choose -- the exact
	// ship-a-different-kernel failure the prepared instance exists to prevent. off
	// turns that into a loud "module lookup disabled" error, enforcing the offline
	// build contract (internal/appliance/cmd_build.go header). Explicit GOPROXY wins, so
	// ./le setup install (a separate target that needs the network) is unaffected.
	if os.Getenv("GOPROXY") == "" {
		if err := os.Setenv("GOPROXY", "off"); err != nil {
			return fail(fmt.Errorf("setenv: %w", err))
		}
	}

	if env.IsEnabled("ze.gok.debug") {
		debug("GOMODCACHE=", modcache)
	}

	// Build from a prepared copy of the instance under project tmp/, so an image
	// build never runs from, or writes to, the tracked gokrazy dir.
	//
	// The deferred cleanup does NOT run across gok's own os.Exit: gok's
	// pack.Main calls os.Exit(1) on a build failure
	// (vendor/.../internal/packer/packer.go) from inside Execute below, so on a
	// failed build control never returns here and the prepared dir is left
	// behind. instance.Prepare reaps such stale dirs on the next run, which
	// bounds the leak; there is no way to run a cleanup across a callee's os.Exit.
	prepared, cleanup, err := prepareArgs(args)
	if err != nil {
		return fail(err)
	}
	defer cleanup()

	if env.IsEnabled("ze.gok.debug") {
		debug("args=", strings.Join(prepared, " "))
	}

	if err := (gok.Context{Args: prepared}).Execute(context.Background()); err != nil {
		return fail(err)
	}
	return 0
}

// fail reports err on stderr, prefixed with the command name, and answers the
// exit code of a failed build.
func fail(err error) int {
	var buffer textbuf.Buffer
	buffer.Str(name).Str(": ").Err(err).Byte('\n').StdErr() //nolint:errcheck // CLI output
	return 1
}

// debug writes one ze.gok.debug line on stderr.
func debug(label, value string) {
	var buffer textbuf.Buffer
	buffer.Str(name).Str(": ").Str(label).Str(value).Byte('\n').StdErr() //nolint:errcheck // CLI output
}
