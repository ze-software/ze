// Design: docs/architecture/config/yang-config-design.md -- ze yang CLI entry point
// Detail: prefix.go -- prefix collision analysis
// Detail: tree.go -- unified analysis tree
// Detail: format.go -- output formatting
// Detail: doc.go -- command documentation

package yang

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
)

// Run executes the ze yang subcommand. Returns exit code.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	switch args[0] {
	case "completion":
		return cmdCompletion(args[1:])
	case "tree":
		return cmdTree(args[1:])
	case "doc":
		return cmdDoc(args[1:])
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown yang command: %s\n", args[0])
		commands := []string{"completion", "tree", "doc", "help"}
		if suggestion := suggest.Command(args[0], commands); suggestion != "" {
			fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", suggestion)
		}
		usage()
		return 1
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `ze yang - YANG analysis and documentation

Usage:
  ze yang <command> [options]

Commands:
  completion          Detect prefix collisions in config and command trees
  tree                Print unified config + command tree
  doc                 Command documentation
  help                Show this help

Examples:
  ze yang completion
  ze yang completion --json
  ze yang completion --min-prefix 3
  ze yang tree
  ze yang tree --commands
  ze yang tree --config
  ze yang tree --json
  ze yang doc --list
  ze yang doc "bgp peer list"
`)
}

func cmdCompletion(args []string) int {
	fs := flag.NewFlagSet("yang completion", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "output as JSON")
	minPrefix := fs.Int("min-prefix", 1, "minimum disambiguation depth to report (1-10)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze yang completion [--json] [--min-prefix N]\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *minPrefix < 1 || *minPrefix > 10 {
		fmt.Fprintf(os.Stderr, "error: --min-prefix must be 1-10, got %d\n", *minPrefix)
		return 1
	}

	root, err := BuildUnifiedTree()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	groups := CollectCollisions(root, *minPrefix)

	if *jsonOutput {
		if err := FormatCollisionsJSON(os.Stdout, groups); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	if err := FormatCollisionsText(os.Stdout, groups); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func cmdTree(args []string) int {
	fs := flag.NewFlagSet("yang tree", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "output as JSON")
	commands := fs.Bool(FilterCommands, false, "show command nodes only")
	config := fs.Bool("config", false, "show config nodes only")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze yang tree [--json] [--commands] [--config]\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *commands && *config {
		fmt.Fprintf(os.Stderr, "error: --commands and --config are mutually exclusive\n")
		return 1
	}

	filter := ""
	if *commands {
		filter = FilterCommands
	}
	if *config {
		filter = SourceConfig
	}

	root, err := BuildUnifiedTree()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if *jsonOutput {
		if err := FormatTreeJSON(os.Stdout, root, filter); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	if err := FormatTreeText(os.Stdout, root, filter); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func cmdDoc(args []string) int {
	fs := flag.NewFlagSet("yang doc", flag.ExitOnError)
	list := fs.Bool("list", false, "list all commands")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze yang doc [--list] [<command>]\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *list {
		if err := FormatDocList(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: ze yang doc <command> or ze yang doc --list\n")
		return 1
	}

	cliCommand := strings.Join(fs.Args(), " ")
	if err := FormatDocCommand(os.Stdout, cliCommand); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}
