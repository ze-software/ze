package main

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/test/perfrunner"
)

func main() {
	root, err := perfrunner.FindRoot(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ze-perf-run: %v\n", err)
		os.Exit(1)
	}
	os.Exit(perfrunner.New(root, os.Stdout, os.Stderr).RunCLI(os.Args[1:]))
}
