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

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/suggest"
)

// helpSectionOptions is the section title every usage page in this file carries.
const helpSectionOptions = "Options"

// Run executes the ze yang subcommand. Returns exit code.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	// The cases stay written out. TestDispatchParity reads them from this
	// file's AST and compares them with yangCommands, and it can only see a
	// string literal.
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
		Command:   "ze yang",
		ShortHelp: "YANG analysis and documentation",
		Usage:     []string{"ze yang <command> [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: []helpfmt.HelpEntry{
				{Name: subCompletion, Desc: "Detect prefix collisions in config and command trees"},
				{Name: subTree, Desc: "Print unified config + command tree"},
				{Name: subDoc, Desc: "Command documentation"},
				{Name: "help", Desc: "Show this help"},
			}},
		},
		Examples: []string{
			"ze yang completion",
			"ze yang completion --min-prefix 3",
			`ze cli -c "show yang completion | json"`,
			"ze yang tree",
			"ze yang tree --commands",
			"ze yang tree --config",
			`ze cli -c "show yang tree --config | json"`,
			"ze yang doc --list",
			`ze yang doc "show bgp peer list"`,
		},
	}
	p.WriteErr()
}

func cmdCompletion(args []string) int {
	options, err := parseCompletionOptions(args)
	if err != nil {
		return writeOptionError(err, completionHelpPage())
	}

	root, err := buildUnifiedTree()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	groups := collectCollisions(root, options.minPrefix)
	if err := formatCollisionsText(os.Stdout, groups); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

type completionOptions struct {
	minPrefix int
}

func parseCompletionOptions(args []string) (completionOptions, error) {
	options := completionOptions{minPrefix: 1}
	fs := flag.NewFlagSet("yang completion", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVar(&options.minPrefix, "min-prefix", 1,
		"minimum disambiguation depth to report (1-10)")
	// A help request comes back as flag.ErrHelp and the caller prints the
	// page: the parser also serves a data handler, which writes nothing.
	fs.Usage = func() {}
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

// completionHelpPage is the help page of `ze yang completion`.
func completionHelpPage() *helpfmt.Page {
	return &helpfmt.Page{
		Command:   "ze yang completion",
		ShortHelp: "Detect prefix collisions in config and command trees",
		Usage:     []string{"ze yang completion [--min-prefix N]"},
		Sections: []helpfmt.HelpSection{
			{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
				{Name: "--min-prefix N", Desc: "Minimum disambiguation depth to report (1-10)"},
			}},
		},
	}
}

func cmdTree(args []string) int {
	options, err := parseTreeOptions(args)
	if err != nil {
		return writeOptionError(err, treeHelpPage())
	}

	root, err := buildUnifiedTree()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if err := formatTreeText(os.Stdout, root, options.filter); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

type treeOptions struct {
	filter string
}

func parseTreeOptions(args []string) (treeOptions, error) {
	var options treeOptions
	var commands bool
	var config bool

	fs := flag.NewFlagSet("yang tree", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&commands, FilterCommands, false, "show command nodes only")
	fs.BoolVar(&config, "config", false, "show config nodes only")
	// A help request comes back as flag.ErrHelp and the caller prints the
	// page: the parser also serves a data handler, which writes nothing.
	fs.Usage = func() {}
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

// treeHelpPage is the help page of `ze yang tree`.
func treeHelpPage() *helpfmt.Page {
	return &helpfmt.Page{
		Command:   "ze yang tree",
		ShortHelp: "Print unified config + command tree",
		Usage:     []string{"ze yang tree [--commands] [--config]"},
		Sections: []helpfmt.HelpSection{
			{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
				{Name: "--commands", Desc: "Show command nodes only"},
				{Name: "--config", Desc: "Show config nodes only"},
			}},
		},
	}
}

// writeOptionError reports an option the parser refused, for the printing
// form of a command. A help request prints the command's help page and exits
// 0, which is not a failure.
func writeOptionError(err error, page *helpfmt.Page) int {
	if errors.Is(err, flag.ErrHelp) {
		page.WriteErr()
		return 0
	}
	os.Stderr.WriteString(command.LocalErrorLine(err)) //nolint:errcheck // CLI diagnostic
	return 1
}

// cmdDoc is `ze yang doc`: the answer of `show yang doc`, printed.
func cmdDoc(args []string) int {
	return command.PrintLocalAnswer("show yang doc", docAnswer(args))
}

// docHelpPage is the help page of `ze yang doc`.
func docHelpPage() *helpfmt.Page {
	return &helpfmt.Page{
		Command:   "ze yang doc",
		ShortHelp: "Command documentation",
		Usage:     []string{"ze yang doc [--list] [<command>]"},
		Sections: []helpfmt.HelpSection{
			{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
				{Name: "--list", Desc: "List all commands"},
			}},
		},
	}
}
