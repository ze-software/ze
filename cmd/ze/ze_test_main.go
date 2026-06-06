// Design: docs/architecture/testing/ci-format.md — ze-test minimal entry point

//go:build ze_test

package main

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
	zeversion "codeberg.org/thomas-mangin/ze/internal/core/version"
)

var (
	version   = "dev"
	buildDate = "unknown"
)

func main() {
	zeversion.Stamp(version, buildDate)
	zeTestRegisterAll()

	if len(os.Args) < 2 {
		zeTestUsage()
		os.Exit(1)
	}

	arg := os.Args[1]
	if isHelpArg(arg) {
		zeTestUsage()
		return
	}

	if arg == "--version" || arg == "-V" {
		fmt.Println(zeversion.Short())
		return
	}

	handler := registry.LookupRoot(arg)
	if handler == nil {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", arg)
		zeTestUsage()
		os.Exit(1)
	}

	rctx := &registry.RuntimeContext{}
	os.Exit(handler(rctx, os.Args[2:]))
}

func isHelpArg(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "help"
}

func zeTestUsage() {
	roots := registry.ListRoot()

	width := 0
	for _, rc := range roots {
		if len(rc.Name) > width {
			width = len(rc.Name)
		}
	}

	fmt.Fprintf(os.Stderr, "Usage: ze-test <command> [options]\n\nCommands:\n")
	for _, rc := range roots {
		fmt.Fprintf(os.Stderr, "  %-*s  %s\n", width, rc.Name, rc.Meta.Description)
	}
	fmt.Fprintf(os.Stderr, "\nRun 'ze-test <command> --help' for command-specific help.\n")
}
