// Design: docs/architecture/system-architecture.md -- unified binary dispatch infrastructure
//
// Shared dispatch infrastructure for all ze binaries. Build tags control which
// commands register; the dispatch loop adapts to what's registered. The binary
// name is used only for usage/help text, never for dispatch.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/cmd/ze/internal/helpfmt"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/textbuf"
	zeversion "github.com/ze-software/ze/internal/core/version"
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
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n", name) //nolint:errcheck // help output
		return
	}

	entries := make([]helpfmt.HelpEntry, len(roots))
	for i, rc := range roots {
		entries[i] = helpfmt.HelpEntry{Name: rc.Name, Desc: rc.Meta.Description}
	}

	var tb textbuf.Buffer
	tb.Str(name).Str(" <command> [options]")

	p := helpfmt.Page{
		Command: name,
		Usage:   []string{tb.String()},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: entries},
		},
	}
	p.WriteErr()
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

	rctx := &registry.RuntimeContext{}

	if handler := registry.LookupRoot(arg); handler != nil {
		return handler(rctx, args[1:])
	}

	// `ze-analyze density` shorthand: when argv[0] is not itself a root but the
	// binary-name segment after the last '-' names a registered root, dispatch
	// through that suffix root so `ze-analyze density` behaves like
	// `ze analyze density`. The prefix before the '-' is ignored, so a
	// dynamically-named binary (foo-analyze) resolves the same. No-op for
	// multi-root binaries like ze-test whose subcommands are top-level roots,
	// and the explicit `ze-analyze analyze density` form still works (arg is a
	// root then, handled above).
	if suffix := binarySuffixRoot(); suffix != "" && suffix != arg {
		if handler := registry.LookupRoot(suffix); handler != nil {
			return handler(rctx, args)
		}
	}

	fmt.Fprintf(os.Stderr, "unknown command: %s\n", arg)
	printUsage()
	return 1
}

// binarySuffixRoot returns the binary-name segment after the last '-'
// (e.g. "ze-analyze" -> "analyze", "ze-perf" -> "perf") when it names a
// registered root command, else "". The prefix before the '-' is ignored so
// the perf/analyze code can ship in a dynamically-named dedicated binary.
func binarySuffixRoot() string {
	name := binaryName()
	i := strings.LastIndex(name, "-")
	if i < 0 || i+1 >= len(name) {
		return ""
	}
	suffix := name[i+1:]
	if registry.LookupRoot(suffix) != nil {
		return suffix
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

	// Handle universal flags before personality setup.
	if len(args) > 0 {
		switch args[0] {
		case "--version", "-V":
			rw := helpfmt.NewRenderWriter(os.Stdout)
			rw.Line(zeversion.Short())
			return rw.ExitCode()
		case "--extended-version":
			rw := helpfmt.NewRenderWriter(os.Stdout)
			rw.Line(zeversion.Extended())
			return rw.ExitCode()
		}
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
