package main

import (
	"os"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/linuxle"
	"github.com/ze-software/ze/internal/test/perfrunner"
)

func main() {
	root, err := perfrunner.FindRoot(".")
	if err != nil {
		fail(err)
	}
	runner := perfrunner.New(root, os.Stdout, os.Stderr)
	// The sender container runs a linux le, built with the tags the launcher
	// builds le with, so `le perf send` links the BGP code it measures with.
	runner.LinuxTags, err = linuxle.Tags(root)
	if err != nil {
		fail(err)
	}
	os.Exit(runner.RunCLI(os.Args[1:]))
}

// fail writes one diagnosis line and exits 1.
func fail(err error) {
	var tb textbuf.Buffer
	tb.Str("ze-perf-run: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // pre-exit diagnostic
	os.Exit(1)
}
