// Design: docs/architecture/config/yang-config-design.md — config completion command
// Overview: main.go — dispatch and exit codes

package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The column widths of the completion text form. A candidate wider than its
// column pushes the next one right rather than being cut: the operator needs
// the whole token to type it.
const (
	completionTypeWidth = 12
	completionTextWidth = 24
)

// completionRequest is one parsed `config completion` query: which
// configuration to load, what the operator has typed, and where in the tree.
type completionRequest struct {
	configPath string
	input      string
	context    []string
	ghost      bool
}

// parseCompletionRequest reads the query an operator typed.
//
// usage is the help page to print when the query is unusable; a caller with no
// page of its own passes nil.
func parseCompletionRequest(args []string, usage func()) (completionRequest, int) {
	fs := flag.NewFlagSet("config completion", flag.ContinueOnError)
	context := fs.String("context", "", "context path with / separator (e.g., bgp/peer/1.1.1.1)")
	inputFlag := fs.String("input", "", "input text to complete (e.g., \"set \", \"set local\")")
	ghost := fs.Bool("ghost", false, "show ghost text instead of completions")
	if usage != nil {
		fs.Usage = usage
	}

	if err := fs.Parse(args); err != nil {
		return completionRequest{}, 1
	}

	if fs.NArg() < 1 {
		helpfmt.WriteError(os.Stderr, false, "missing config file (use - for stdin)")
		fs.Usage()
		return completionRequest{}, 1
	}

	return completionRequest{
		configPath: fs.Arg(0),
		// + stands for a space, so the input survives an unquoted argument in
		// a shell and in the functional test runner.
		input: strings.ReplaceAll(*inputFlag, "+", " "),
		// The context path is /-separated for the same reason.
		context: splitContextPath(*context),
		ghost:   *ghost,
	}, exitOK
}

// splitContextPath answers the context path tokens, and nil for an unset
// context, which is the whole tree.
func splitContextPath(context string) []string {
	if context == "" {
		return nil
	}
	return strings.Split(context, "/")
}

// completerFor loads a configuration and answers a completer with the parsed
// tree, which each completion call is given, so completion is data-aware.
func completerFor(configPath string) (*cli.Completer, *config.Tree, int) {
	data, err := cliio.ReadFile(configPath)
	if err != nil {
		helpfmt.WriteError(os.Stderr, false, "reading config: %v", err)
		return nil, nil, exitError
	}

	completer, tree, err := completerForBytes(data, configPath)
	if err != nil {
		helpfmt.WriteError(os.Stderr, false, "%v", err)
		return nil, nil, exitError
	}
	return completer, tree, exitOK
}

// completerForBytes selects the available schema and parses caller-held bytes.
// It starts no declared program and never renders private source fragments in
// a refusal. Local files, stdin and supplied requests use this same policy.
func completerForBytes(data []byte, label string) (*cli.Completer, *config.Tree, error) {
	// Before the completer is built: it takes the module set it finds, and
	// this step fixes that set. With no daemon it is the compiled-in one: no
	// plugin is started, so an external plugin's section is not completed.
	schema, _, err := config.SchemaWithDeclaredPlugins(config.ConfigSource{Text: string(data), Path: label})
	if err != nil {
		return nil, nil, completionError(err, "the configuration schema is unavailable")
	}

	p := config.NewParser(schema)
	tree, err := p.Parse(string(data))
	if err != nil {
		return nil, nil, completionError(err, "the configuration does not match the available schema")
	}

	completer, err := cli.NewCompleter()
	if err != nil {
		return nil, nil, completionError(err, "the completion schema is unavailable")
	}
	return completer, tree, nil
}

// completionError retains typed location and schema constraints, never the
// parser's source text, keyed path, supplied value or declared-program text.
func completionError(err error, problem string) error {
	var message textbuf.Buffer
	message.Str("configuration completion: ").Str(problem)
	line, _ := config.SplitLine(err)
	if line > 0 {
		message.Str(" at line ").Int(int64(line))
	}
	var validation *configyang.ValidationError
	if errors.As(err, &validation) {
		// Only these kinds carry a schema-derived constraint safe to show.
		//exhaustive:ignore
		switch validation.Type {
		case configyang.ErrTypeEnum, configyang.ErrTypeRange, configyang.ErrTypeLength, configyang.ErrTypeType:
			message.Str("; expected ").Str(validation.Expected)
		}
	}
	return opererr.New(message.String(),
		"Check the configuration syntax and the schema available to Ze; completion starts no external plugin.",
		"ze config validate <file>")
}

func cmdCompletion(args []string) int {
	request, code := parseCompletionRequest(args, completionUsage)
	if code != exitOK {
		return code
	}

	completer, tree, code := completerFor(request.configPath)
	if code != exitOK {
		return code
	}

	if request.ghost {
		fmt.Println(completer.GhostText(tree, request.input, request.context))
		return exitOK
	}
	return printCompletions(completionCandidates(completer.Complete(tree, request.input, request.context)))
}

func completionUsage() {
	p := helpfmt.Page{
		Command:   "ze config completion",
		ShortHelp: "Query the YANG-driven completion engine non-interactively",
		Usage:     []string{"ze config completion [options] <config-file>"},
		Sections: []helpfmt.HelpSection{
			{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
				{Name: "--context <path>", Desc: "Context path with / separator (e.g., bgp/peer/1.1.1.1)"},
				{Name: "--input <text>", Desc: "Input text to complete (e.g., \"set \", \"set local\")"},
				{Name: "--ghost", Desc: "Show ghost text instead of completions"},
			}},
		},
		Examples: []string{
			"ze config completion --input set+ config.conf",
			"ze config completion --context bgp --input set+ config.conf",
			"ze config completion --context bgp --input set+local config.conf",
			"ze config completion --ghost --context bgp --input set+router config.conf",
		},
	}
	p.WriteErr()
	fmt.Fprintln(os.Stderr, "\nUseful for testing and debugging config editor completions.")
	fmt.Fprintln(os.Stderr, "Use - to read config from stdin.")
	fmt.Fprintln(os.Stderr, "\nInput uses + for spaces (unquoted), or regular spaces (quoted):")
	fmt.Fprintln(os.Stderr, "  --input set+           equivalent to \"set \"")
	fmt.Fprintln(os.Stderr, "  --input set+local      equivalent to \"set local\"")
}

// printCompletions writes the completion candidates for a reader.
func printCompletions(completions []completionCandidate) int {
	var line textbuf.Buffer
	for _, comp := range completions {
		line.Reset()
		line.PadRight(comp.Type, completionTypeWidth).Byte(' ')
		if comp.ShortHelp == "" {
			line.Str(comp.Text)
		} else {
			line.PadRight(comp.Text, completionTextWidth).Byte(' ').Str(comp.ShortHelp)
		}
		line.Byte('\n').StdOut() //nolint:errcheck // CLI output
	}
	return exitOK
}
