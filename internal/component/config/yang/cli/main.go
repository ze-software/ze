// Design: docs/architecture/config/yang-config-design.md -- ze yang CLI entry point
// Detail: prefix.go -- prefix collision analysis
// Detail: tree.go -- unified analysis tree
// Detail: format.go -- output formatting
// Detail: doc.go -- command documentation

package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/suggest"
	"github.com/ze-software/ze/internal/core/textbuf"
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
		if suggestion := suggest.Command(args[0], append(yangCommands, "help")); suggestion != "" {
			fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", suggestion)
		}
		usage()
		return 1
	}
}

func usage() {
	p := helpfmt.Page{
		Command: "ze yang",
		Summary: "YANG analysis and documentation",
		Usage:   []string{"ze yang <command> [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: []helpfmt.HelpEntry{
				{Name: "completion", Desc: "Detect prefix collisions in config and command trees"},
				{Name: "tree", Desc: "Print unified config + command tree"},
				{Name: "doc", Desc: "Command documentation"},
				{Name: "help", Desc: "Show this help"},
			}},
		},
		Examples: []string{
			"ze yang completion",
			"ze yang completion --json",
			"ze yang completion --min-prefix 3",
			"ze yang tree",
			"ze yang tree --commands",
			"ze yang tree --config",
			"ze yang tree --json",
			"ze yang doc --list",
			`ze yang doc "show bgp peer list"`,
		},
	}
	p.WriteErr()
}

func cmdCompletion(args []string) int {
	options, err := parseCompletionOptions(args)
	if err != nil {
		return writeOptionError(err)
	}

	root, err := buildUnifiedTree()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	groups := collectCollisions(root, options.minPrefix)
	if options.jsonOutput {
		if err := formatCollisionsJSON(os.Stdout, groups); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	if err := formatCollisionsText(os.Stdout, groups); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

type completionOptions struct {
	jsonOutput bool
	minPrefix  int
}

func parseCompletionOptions(args []string) (completionOptions, error) {
	options := completionOptions{minPrefix: 1}
	fs := flag.NewFlagSet("yang completion", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&options.jsonOutput, "json", false, "output as JSON")
	fs.IntVar(&options.minPrefix, "min-prefix", 1,
		"minimum disambiguation depth to report (1-10)")
	fs.Usage = completionUsage
	if err := fs.Parse(args); err != nil {
		return completionOptions{}, err
	}
	if fs.NArg() > 0 {
		return completionOptions{}, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if options.minPrefix < 1 {
		return completionOptions{},
			fmt.Errorf("--min-prefix must be 1-10, got %d", options.minPrefix)
	}
	if options.minPrefix > 10 {
		return completionOptions{},
			fmt.Errorf("--min-prefix must be 1-10, got %d", options.minPrefix)
	}
	return options, nil
}

func completionUsage() {
	p := helpfmt.Page{
		Command: "ze yang completion",
		Summary: "Detect prefix collisions in config and command trees",
		Usage:   []string{"ze yang completion [--json] [--min-prefix N]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: "--json", Desc: "Output as JSON"},
				{Name: "--min-prefix N", Desc: "Minimum disambiguation depth to report (1-10)"},
			}},
		},
	}
	p.WriteErr()
}

func cmdTree(args []string) int {
	options, err := parseTreeOptions(args)
	if err != nil {
		return writeOptionError(err)
	}

	root, err := buildUnifiedTree()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if options.jsonOutput {
		if err := formatTreeJSON(os.Stdout, root, options.filter); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	if err := formatTreeText(os.Stdout, root, options.filter); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

type treeOptions struct {
	filter     string
	jsonOutput bool
}

func parseTreeOptions(args []string) (treeOptions, error) {
	var options treeOptions
	var commands bool
	var config bool

	fs := flag.NewFlagSet("yang tree", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&options.jsonOutput, "json", false, "output as JSON")
	fs.BoolVar(&commands, FilterCommands, false, "show command nodes only")
	fs.BoolVar(&config, "config", false, "show config nodes only")
	fs.Usage = treeUsage
	if err := fs.Parse(args); err != nil {
		return treeOptions{}, err
	}
	if fs.NArg() > 0 {
		return treeOptions{}, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}

	if commands {
		if config {
			return treeOptions{},
				errors.New("--commands and --config are mutually exclusive")
		}
		options.filter = FilterCommands
	}
	if config {
		options.filter = SourceConfig
	}
	return options, nil
}

func treeUsage() {
	p := helpfmt.Page{
		Command: "ze yang tree",
		Summary: "Print unified config + command tree",
		Usage:   []string{"ze yang tree [--json] [--commands] [--config]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: "--json", Desc: "Output as JSON"},
				{Name: "--commands", Desc: "Show command nodes only"},
				{Name: "--config", Desc: "Show config nodes only"},
			}},
		},
	}
	p.WriteErr()
}

func writeOptionError(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	return 1
}

func cmdDoc(args []string) int {
	fs := flag.NewFlagSet("yang doc", flag.ExitOnError)
	list := fs.Bool("list", false, "list all commands")
	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze yang doc",
			Summary: "Command documentation",
			Usage:   []string{"ze yang doc [--list] [<command>]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--list", Desc: "List all commands"},
				}},
			},
		}
		p.WriteErr()
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *list {
		if err := formatDocList(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: ze yang doc <command> or ze yang doc --list\n")
		return 1
	}

	cliCommand := textbuf.Join(fs.Args(), " ")
	if err := formatDocCommand(os.Stdout, cliCommand); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}
