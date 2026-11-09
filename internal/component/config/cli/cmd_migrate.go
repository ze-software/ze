// Design: docs/architecture/config/syntax.md — config migrate command
// Overview: main.go — dispatch and exit codes

package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func cmdMigrate(args []string) int {
	fs := flag.NewFlagSet("config migrate", flag.ExitOnError)
	outputPath := fs.String("o", "", "write output to file")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command:   "ze config migrate",
			ShortHelp: "Rewrite a configuration in the set or hierarchical form",
			Usage:     []string{"ze config migrate [options] [format <form>] <config-file>"},
			Sections: []helpfmt.HelpSection{
				{Title: helpSectionDescription, Entries: []helpfmt.HelpEntry{
					{Name: "", Desc: "Reads a current config in either form and writes it in the form named, " +
						"with plaintext passwords hashed. Default output is set format. Use - to read from stdin."},
				}},
				{Title: "Forms", Entries: []helpfmt.HelpEntry{
					{Name: "format set", Desc: "Write flat set commands (default)"},
					{Name: "format hierarchical", Desc: "Write nested blocks"},
				}},
				{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
					{Name: "-o <file>", Desc: "Write output to file"},
				}},
				{Title: helpSectionExitCodes, Entries: []helpfmt.HelpEntry{
					{Name: "0", Desc: helpDescSuccess},
					{Name: "2", Desc: "Error (file not found, parse error, write error)"},
				}},
			},
			Examples: []string{
				"ze config migrate config.conf                          # Convert to set form (stdout)",
				"ze config migrate -o new.conf config.conf              # Convert to new file",
				"ze config migrate format hierarchical config.conf      # Explicit hierarchical output",
				"cat config.conf | ze config migrate -                  # Read from stdin",
			},
		}
		p.WriteErr()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	outputForm, rest, err := parseOutputForm(fs.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return exitError
	}

	if len(rest) < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file (use - for stdin)\n")
		fs.Usage()
		return exitError
	}

	configPath := rest[0]

	output, warnings, err := configMigrateWithWarnings(configPath, *outputPath, outputForm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if *outputPath == "" {
		fmt.Print(output)
		printMigrateWarnings(warnings)
	} else {
		fmt.Fprintf(os.Stderr, "Config migrated: %s\n", *outputPath)
		printMigrateWarnings(warnings)
	}

	return exitOK
}

// The two forms a migrated configuration can be written in, and the keyword
// that selects one. The form is part of the question this command answers, so
// it is grammar an operator types, never a flag (ai/rules/cli.md).
const (
	formKeyword      = "format"
	formSet          = "set"
	formHierarchical = "hierarchical"
)

// parseOutputForm reads an optional leading `format <form>` from the words
// after the flags, and answers the form plus the words that remain.
//
// The default is the set form, which is what a migration writes when the
// operator names none.
func parseOutputForm(args []string) (string, []string, error) {
	if len(args) == 0 || args[0] != formKeyword {
		return formSet, args, nil
	}
	if len(args) < 2 {
		var b textbuf.Buffer
		return "", nil, errors.New(b.Str(formKeyword).Str(" needs a form: ").
			Str(formSet).Str(" or ").Str(formHierarchical).String())
	}
	form := args[1]
	if form != formSet && form != formHierarchical {
		var b textbuf.Buffer
		return "", nil, errors.New(b.Str(formKeyword).Str(": unknown form ").Quoted(form).
			Str(" (want ").Str(formSet).Str(" or ").Str(formHierarchical).Str(")").String())
	}
	return form, args[2:], nil
}

func printMigrateWarnings(warnings []string) {
	if len(warnings) > 0 {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Unsupported features detected (will be ignored):")
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "  - %s\n", w)
		}
	}
}

// findUnsupportedFeatures names, for each BGP peer, the configured features the
// schema accepts and Ze does not implement.
func findUnsupportedFeatures(tree *config.Tree) []string {
	var warnings []string

	if bgp := tree.GetContainer("bgp"); bgp != nil {
		for _, entry := range bgp.GetListOrdered("peer") {
			warnings = append(warnings, checkUnsupportedInPeerTree(entry.Key, entry.Value)...)
		}
	}

	return warnings
}

func checkUnsupportedInPeerTree(path string, tree *config.Tree) []string {
	var warnings []string

	if cap := tree.GetContainer("capability"); cap != nil {
		if _, ok := cap.GetFlex("multi-session"); ok {
			warnings = append(warnings, fmt.Sprintf("%s: capability.multi-session not supported", path))
		}
		if _, ok := cap.GetFlex("operational"); ok {
			warnings = append(warnings, fmt.Sprintf("%s: capability.operational not supported", path))
		}
	}

	if tree.GetContainer("operational") != nil {
		warnings = append(warnings, fmt.Sprintf("%s: operational block not supported", path))
	}

	return warnings
}

// configMigrateWithWarnings reads the config at inputPath in either form and
// answers it in outputForm, with the unsupported features it holds. With an
// outputPath it writes the file and answers no output.
func configMigrateWithWarnings(inputPath, outputPath, outputForm string) (string, []string, error) {
	data, err := cliio.ReadFile(inputPath)
	if err != nil {
		return "", nil, err
	}

	content := string(data)

	// Parse any format: auto-detect and use the appropriate parser.
	schema, err := config.YANGSchema()
	if err != nil {
		return "", nil, fmt.Errorf("YANG schema: %w", err)
	}

	sourceFormat := config.DetectFormat(content)
	var tree *config.Tree
	switch sourceFormat {
	case config.FormatSet, config.FormatSetMeta:
		tree, err = config.NewSetParser(schema).Parse(content)
	case config.FormatHierarchical:
		tree, err = config.NewParser(schema).Parse(content)
	default:
		panic("BUG: unknown detected config format")
	}
	if err != nil {
		return "", nil, fmt.Errorf("parse error: %w", err)
	}

	warnings := findUnsupportedFeatures(tree)

	// The plaintext-password leaf is hashed into its canonical leaf first:
	// unhashed, the hierarchical form skips the ephemeral leaf and loses the
	// credential, and the set form prints the password in clear.
	weaknesses, err := hashPlaintextPasswords(tree, schema)
	if err != nil {
		return "", nil, err
	}
	printPasswordWeaknesses(weaknesses)

	// Serialize in the requested output form (default: set).
	// Both printers write each ze:sensitive key in the $9$ form, as fmt does.
	var output string
	if outputForm == formHierarchical {
		output = config.Serialize(tree, schema)
	} else {
		output = config.SerializeSet(tree, schema)
	}

	if outputPath != "" {
		// "-o -" writes the migrated config to stdout (warnings go to
		// stderr); an omitted -o already prints to stdout via the caller.
		if err := cliio.WriteFile(outputPath, []byte(output), 0o600); err != nil {
			return "", warnings, fmt.Errorf("write output: %w", err)
		}
		return "", warnings, nil
	}

	return output, warnings, nil
}
