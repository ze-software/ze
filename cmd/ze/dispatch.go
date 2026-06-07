// Design: docs/architecture/system-architecture.md -- unified binary dispatch infrastructure
//
// Shared dispatch infrastructure for all ze binaries. Each binary personality
// (ze, ze-test, ze-chaos, ze-perf, ze-analyze) registers itself via init() in
// build-tagged files; the dispatch loop adapts to what's registered.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/crashlog"
	zeversion "codeberg.org/thomas-mangin/ze/internal/core/version"
)

// binarySetup is called before dispatch to perform personality-specific
// initialization (plugin server version stamp, diagnostics, command registration)
// and to parse personality-specific global flags from the arg list, returning
// the remaining args. When args is nil, the setup handled the request (e.g.
// --version) and the returned int is the exit code. Set by ze_core_dispatch.go
// for the ze personality.
var binarySetup func(args []string) ([]string, int)

// binaryDispatch, when non-nil, replaces the default dispatch loop entirely.
// Used by the ze personality which has YANG verbs, config file detection, and
// global flag state that the default registry-only dispatch cannot handle.
var binaryDispatch func(args []string) int

// binaryUsage, when non-nil, replaces the default usage printer with a
// personality-specific one (e.g., ze's full help page with YANG verbs and
// options sections).
var binaryUsage func()

func flushCrashlog() {
	crashlog.Flush()
}

func isHelpArg(s string) bool {
	return s == "help" || s == "-h" || s == "--help" //nolint:goconst // consistent pattern across cmd files
}

func printUsage() {
	if binaryUsage != nil {
		binaryUsage()
		return
	}
	defaultUsage()
}

func defaultUsage() {
	roots := registry.ListRoot()
	name := binaryName()

	if len(roots) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n", name)
		return
	}

	width := 0
	for _, rc := range roots {
		if len(rc.Name) > width {
			width = len(rc.Name)
		}
	}

	fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n\nCommands:\n", name)
	for _, rc := range roots {
		fmt.Fprintf(os.Stderr, "  %-*s  %s\n", width, rc.Name, rc.Meta.Description)
	}
	fmt.Fprintf(os.Stderr, "\nRun '%s <command> --help' for command-specific help.\n", name)
}

// defaultDispatch handles the common case: look up a registered root command
// and dispatch to it. Used by ze-test, ze-chaos, ze-perf, and ze-analyze where
// all commands are registered as root handlers.
func defaultDispatch(args []string) int {
	if len(args) == 0 {
		printUsage()
		return 1
	}

	arg := args[0]
	if isHelpArg(arg) {
		printUsage()
		return 0
	}

	handler := registry.LookupRoot(arg)
	if handler == nil {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", arg)
		printUsage()
		return 1
	}

	rctx := &registry.RuntimeContext{}
	return handler(rctx, args[1:])
}

// multiCallPrefix extracts the personality prefix from the binary name.
// "ze-test" returns "test", "ze-chaos" returns "chaos", "ze" returns "".
func multiCallPrefix() string {
	base := binaryName()
	if len(base) > 3 && base[:3] == "ze-" {
		return base[3:]
	}
	return ""
}

func binaryName() string {
	base := filepath.Base(os.Args[0])
	base = strings.TrimSuffix(base, ".exe")
	base = strings.TrimSuffix(base, ".test")
	return base
}

// dispatchMain is the unified entry point called by main(). Returns exit code.
func dispatchMain(args []string) int {
	crashlog.Init()
	zeversion.Stamp(version, buildDate)

	// Handle universal flags before multi-call prefix.
	if len(args) > 0 {
		switch args[0] {
		case "--version", "-V":
			fmt.Println(zeversion.Short())
			return 0
		case "--extended-version":
			fmt.Println(zeversion.Extended())
			return 0
		}
	}

	// Multi-call: ze-test foo -> ze test foo.
	if prefix := multiCallPrefix(); prefix != "" {
		args = append([]string{prefix}, args...)
	}

	// Run personality-specific setup (ze: global flag parsing, plugin init).
	if binarySetup != nil {
		var code int
		args, code = binarySetup(args)
		if args == nil {
			return code
		}
	}

	// Personality-specific dispatch or default registry dispatch.
	if binaryDispatch != nil {
		return binaryDispatch(args)
	}
	return defaultDispatch(args)
}
