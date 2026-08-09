// Design: docs/architecture/appliance/remote-operations.md -- config preview (merged base + overlay)

package appliance

import (
	"flag"
	"fmt"
	"os"
)

func init() {
	cmdConfig = runConfig
}

func runConfig(args []string) int {
	fs := flag.NewFlagSet("appliance config", flag.ContinueOnError)
	mergedFlag := fs.Bool("merged", false, "Show effective config after base + overlay merge")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze appliance config [options] <name>\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  ze appliance config lab --merged\n")
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: requires <name>\n")
		fs.Usage()
		return exitError
	}

	name := fs.Arg(0)

	if !*mergedFlag {
		fmt.Fprintf(os.Stderr, "error: specify --merged to see effective config\n")
		fs.Usage()
		return exitError
	}

	dir := getBaseDir()

	cfg, err := LoadConfig(ConfigPath(dir, name))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	merged, err := resolveSeedConfig(dir, name, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if merged == "" {
		fmt.Fprintf(os.Stderr, "warning: no config found (no base, no overlay)\n")
		return exitOK
	}

	fmt.Print(merged)
	if merged[len(merged)-1] != '\n' {
		fmt.Println()
	}
	return exitOK
}
