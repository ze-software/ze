// Design: docs/architecture/config/syntax.md — config migrate command
// Overview: main.go — dispatch and exit codes

package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/migration"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func cmdMigrate(args []string) int {
	fs := flag.NewFlagSet("config migrate", flag.ExitOnError)
	outputPath := fs.String("o", "", "write output to file")
	dryRun := fs.Bool("dry-run", false, "show what would be migrated without making changes")
	listTransforms := fs.Bool("list", false, "list available transformations")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command:   "ze config migrate",
			ShortHelp: "Rewrite a configuration in the set or hierarchical form",
			Usage:     []string{"ze config migrate [options] [format <form>] <config-file>"},
			Sections: []helpfmt.HelpSection{
				{Title: helpSectionDescription, Entries: []helpfmt.HelpEntry{
					{Name: "", Desc: "Reads a config in either form, applies each format conversion --list names, " +
						"and writes it in the form named, with plaintext passwords hashed. " +
						"Default output is set format. Use - to read from stdin."},
				}},
				{Title: "Forms", Entries: []helpfmt.HelpEntry{
					{Name: "format set", Desc: "Write flat set commands (default)"},
					{Name: "format hierarchical", Desc: "Write nested blocks"},
				}},
				{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
					{Name: "-o <file>", Desc: "Write output to file"},
					{Name: helpFlagDryRun, Desc: "Show what would be migrated without making changes"},
					{Name: "--list", Desc: "List available transformations"},
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
				"ze config migrate --dry-run config.conf                # Preview transformations",
				"ze config migrate --list                               # List available transformations",
				"cat config.conf | ze config migrate -                  # Read from stdin",
			},
		}
		p.WriteErr()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if *listTransforms {
		printTransformationList()
		return exitOK
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

	if *dryRun && *outputPath != "" {
		var b textbuf.Buffer
		b.Str("error: ").Str(opererr.New("--dry-run cannot be combined with -o",
			"a dry run writes nothing, so it has no output file",
			"drop -o to preview, or drop --dry-run to write "+*outputPath).Error()).Byte('\n')
		b.StdErr() //nolint:errcheck // terminal output, advisory
		return exitError
	}

	if *dryRun {
		return cmdMigrateDryRun(configPath)
	}

	output, result, warnings, err := configMigrateWithWarnings(configPath, *outputPath, outputForm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if *outputPath == "" {
		printMigrateResult(result)
		fmt.Print(output)
		printMigrateWarnings(warnings)
	} else {
		fmt.Fprintf(os.Stderr, "Config migrated: %s\n", *outputPath)
		printMigrateResult(result)
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

func printTransformationList() {
	var b textbuf.Buffer
	b.Str("Available transformations (in order):\n")
	for _, t := range migration.ListTransformations() {
		b.Str("  ").PadRight(t.Name, 25).Byte(' ').Str(t.Description).Byte('\n')
	}
	b.StdOut() //nolint:errcheck // terminal output, advisory
}

func printMigrateResult(result *migration.MigrateResult) {
	if result == nil {
		return
	}

	var b textbuf.Buffer
	if len(result.Applied) == 0 && len(result.Skipped) > 0 {
		b.Str("No transformation needed.\n")
		b.StdErr() //nolint:errcheck // terminal output, advisory
		return
	}

	b.Str("Transformations:\n")
	for _, name := range result.Applied {
		b.Str("  + ").Str(name).Byte('\n')
	}
	for _, name := range result.Skipped {
		b.Str("  - ").Str(name).Str(" (not needed)\n")
	}
	b.Byte('\n').Int(int64(len(result.Applied))).Str(" applied, ").
		Int(int64(len(result.Skipped))).Str(" skipped.\n")
	b.StdErr() //nolint:errcheck // terminal output, advisory
}

func cmdMigrateDryRun(configPath string) int {
	data, err := cliio.ReadFile(configPath)
	if err != nil {
		return dryRunRefusal(opererr.Wrap(err, "the config file could not be read",
			"the dry run analyses the file's content", "check the path and its permissions: "+configPath))
	}

	// A file written before a block moved to a top-level root still holds it at
	// the old position, which only the migration schema parses.
	schema, err := config.MigrationSchema()
	if err != nil {
		return dryRunRefusal(opererr.Wrap(err, "the migration schema could not be built",
			"this binary's YANG modules failed to load", "rebuild ze; run ze config validate for the module error"))
	}

	tree, err := config.NewParser(schema).Parse(string(data))
	if err != nil {
		return dryRunRefusal(opererr.Wrap(err, "the config could not be parsed",
			"a dry run analyses a parsed tree", "correct the line the parse error names, then rerun"))
	}

	result, err := migration.DryRun(tree)
	if err != nil {
		return dryRunRefusal(opererr.Wrap(err, "the transformations could not be analysed",
			"the dry run stopped before every transformation was checked", "run ze config migrate --list and report the failing one"))
	}

	alreadyDone := make(map[string]bool)
	wouldApply := make(map[string]bool)
	for _, name := range result.AlreadyDone {
		alreadyDone[name] = true
	}
	for _, name := range result.WouldApply {
		wouldApply[name] = true
	}

	var b textbuf.Buffer
	b.Str("Transformation analysis:\n")
	for _, t := range migration.ListTransformations() {
		switch {
		case alreadyDone[t.Name]:
			b.Str("  [done] ").Str(t.Name).Byte('\n')
		case !wouldApply[t.Name]:
		case t.Name == result.FailedAt:
			b.Str("  [fail] ").Str(t.Name).Byte('\n')
		default:
			b.Str("  [pending] ").Str(t.Name).Byte('\n')
		}
	}

	b.Byte('\n')
	if !result.WouldSucceed {
		b.Str("Error: ").Str(result.FailedAt).Str(": ").Str(result.Error.Error()).Byte('\n')
		b.Str("\nResult: Transformation would fail.\n")
		b.StdOut() //nolint:errcheck // terminal output, advisory
		return exitError
	}

	if len(result.WouldApply) == 0 {
		b.Str("Result: No transformation needed.\n")
	} else {
		b.Str("Result: ").Int(int64(len(result.WouldApply))).Str(" transformation(s) would apply. All would succeed.\n")
	}
	b.StdOut() //nolint:errcheck // terminal output, advisory
	return exitOK
}

// dryRunRefusal prints a dry-run refusal on stderr and answers the exit code.
func dryRunRefusal(refusal *opererr.Error) int {
	var b textbuf.Buffer
	b.Str("error: ").Str(refusal.Error()).Byte('\n')
	b.StdErr() //nolint:errcheck // terminal output, advisory
	return exitError
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

// configMigrateWithWarnings reads the config at inputPath in either form,
// applies each registered format conversion (migration.Migrate), and answers
// it in outputForm with the conversions' outcome and the unsupported features
// it holds. With an outputPath it writes the file and answers no output.
func configMigrateWithWarnings(inputPath, outputPath, outputForm string) (string, *migration.MigrateResult, []string, error) {
	data, err := cliio.ReadFile(inputPath)
	if err != nil {
		return "", nil, nil, err
	}
	converted, err := configMigrateBytes(data, inputPath, outputForm)
	if err != nil {
		return "", nil, nil, err
	}
	printPasswordWeaknesses(converted.weaknesses)
	if outputPath != "" {
		// "-o -" writes the migrated config to stdout (result/warnings go to
		// stderr); an omitted -o already prints to stdout via the caller.
		if err := cliio.WriteFile(outputPath, []byte(converted.output), 0o600); err != nil {
			return "", converted.result, converted.warnings, fmt.Errorf("write output: %w", err)
		}
		return "", converted.result, converted.warnings, nil
	}
	return converted.output, converted.result, converted.warnings, nil
}

// migratedConfig is the conversion result before a caller chooses its output
// destination. Warnings stay with the answer and never reach daemon stderr.
type migratedConfig struct {
	output     string
	result     *migration.MigrateResult
	warnings   []string
	weaknesses []string
}

func configMigrateBytes(data []byte, inputPath, outputForm string) (migratedConfig, error) {
	if outputForm == "" {
		outputForm = formSet
	}
	if outputForm != formSet {
		if outputForm != formHierarchical {
			return migratedConfig{}, fmt.Errorf("unknown output form %q: use set or hierarchical", outputForm)
		}
	}

	content := string(data)

	// Parse any format: auto-detect and use the appropriate parser.
	// The external plugin sections this config declares are published first,
	// so a section of an external plugin is refused with the command that
	// reaches a running ze (config.PublishDeclaredSchemas). The migration
	// schema also accepts a block at the position it moved from.
	declared, err := config.PublishDeclaredSchemas(config.ConfigSource{Text: content, Path: inputPath})
	if err != nil {
		return migratedConfig{}, fmt.Errorf("YANG schema: %w", err)
	}
	schema, err := config.MigrationSchema()
	if err != nil {
		return migratedConfig{}, fmt.Errorf("YANG schema: %w", err)
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
		return migratedConfig{}, fmt.Errorf("parse error: %w", declared.Explain(err))
	}

	result, err := migration.Migrate(tree)
	if err != nil {
		return migratedConfig{}, fmt.Errorf("migration failed: %w", err)
	}

	warnings := findUnsupportedFeatures(result.Tree)

	// The plaintext-password leaf is hashed into its canonical leaf first:
	// unhashed, the hierarchical form skips the ephemeral leaf and loses the
	// credential, and the set form prints the password in clear.
	weaknesses, err := hashPlaintextPasswords(result.Tree, schema)
	if err != nil {
		return migratedConfig{}, err
	}

	// Serialize in the requested output form (default: set).
	// Both printers write each ze:sensitive key in the $9$ form, as fmt does.
	var output string
	if outputForm == formHierarchical {
		output = config.Serialize(result.Tree, schema)
	} else {
		output = config.SerializeSet(result.Tree, schema)
	}

	return migratedConfig{output: output, result: result, warnings: warnings, weaknesses: weaknesses}, nil
}
