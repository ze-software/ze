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
	"github.com/ze-software/ze/internal/core/textbuf"
)

func cmdMigrate(args []string) int {
	fs := flag.NewFlagSet("config migrate", flag.ExitOnError)
	outputPath := fs.String("o", "", "write output to file")
	dryRun := fs.Bool("dry-run", false, "show what would be migrated without making changes")
	listTransforms := fs.Bool("list", false, "list available transformations")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze config migrate",
			Summary: "Convert configuration to current format",
			Usage:   []string{"ze config migrate [options] [format <form>] <config-file>"},
			Sections: []helpfmt.HelpSection{
				{Title: helpSectionDescription, Entries: []helpfmt.HelpEntry{
					{Name: "", Desc: "Default output is set format. Use - to read from stdin."},
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
		fmt.Fprintf(os.Stderr, "error: --dry-run cannot be combined with -o\n")
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

	if *outputPath != "" {
		fmt.Fprintf(os.Stderr, "Config migrated: %s\n", *outputPath)
		printMigrateResult(result)
		printMigrateWarnings(warnings)
	} else {
		printMigrateResult(result)
		fmt.Print(output)
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
	transforms := migration.ListTransformations()
	fmt.Println("Available transformations (in order):")
	for _, t := range transforms {
		fmt.Printf("  %-25s %s\n", t.Name, t.Description)
	}
}

func printMigrateResult(result *migration.MigrateResult) {
	if result == nil {
		return
	}

	if len(result.Applied) == 0 && len(result.Skipped) > 0 {
		fmt.Fprintln(os.Stderr, "No transformation needed.")
		return
	}

	fmt.Fprintln(os.Stderr, "Transformations:")
	for _, name := range result.Applied {
		fmt.Fprintf(os.Stderr, "  + %s\n", name)
	}
	for _, name := range result.Skipped {
		fmt.Fprintf(os.Stderr, "  - %s (not needed)\n", name)
	}
	fmt.Fprintf(os.Stderr, "\n%d applied, %d skipped.\n", len(result.Applied), len(result.Skipped))
}

func cmdMigrateDryRun(configPath string) int {
	data, err := cliio.ReadFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	content := string(data)

	schema, err := config.YANGSchema()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	p := config.NewParser(schema)
	tree, err := p.Parse(content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parse error: %v\n", err)
		return exitError
	}

	result, err := migration.DryRun(tree)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	alreadyDone := make(map[string]bool)
	wouldApply := make(map[string]bool)
	for _, name := range result.AlreadyDone {
		alreadyDone[name] = true
	}
	for _, name := range result.WouldApply {
		wouldApply[name] = true
	}

	fmt.Println("Transformation analysis:")
	transforms := migration.ListTransformations()
	for _, t := range transforms {
		if alreadyDone[t.Name] {
			fmt.Printf("  [done] %s\n", t.Name)
		} else if wouldApply[t.Name] {
			if t.Name == result.FailedAt {
				fmt.Printf("  [fail] %s\n", t.Name)
			} else {
				fmt.Printf("  [pending] %s\n", t.Name)
			}
		}
	}

	fmt.Println()
	if !result.WouldSucceed {
		fmt.Printf("Error: %s: %v\n", result.FailedAt, result.Error)
		fmt.Println("\nResult: Transformation would fail.")
		return exitError
	}

	if len(result.WouldApply) == 0 {
		fmt.Println("Result: No transformation needed.")
	} else {
		fmt.Printf("Result: %d transformation(s) would apply. All would succeed.\n", len(result.WouldApply))
	}

	return exitOK
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

func findUnsupportedFeatures(tree *config.Tree) []string {
	var warnings []string

	for _, entry := range tree.GetListOrdered("peer") {
		warnings = append(warnings, checkUnsupportedInPeerTree(entry.Key, entry.Value)...)
	}

	for _, entry := range tree.GetListOrdered("neighbor") {
		warnings = append(warnings, checkUnsupportedInPeerTree(entry.Key, entry.Value)...)
	}

	if bgp := tree.GetContainer("bgp"); bgp != nil {
		for _, entry := range bgp.GetListOrdered("peer") {
			warnings = append(warnings, checkUnsupportedInPeerTree(entry.Key, entry.Value)...)
		}
	}

	if tmpl := tree.GetContainer("template"); tmpl != nil {
		for _, entry := range tmpl.GetListOrdered("group") {
			warnings = append(warnings, checkUnsupportedInPeerTree("template/group/"+entry.Key, entry.Value)...)
		}
		for _, entry := range tmpl.GetListOrdered("match") {
			warnings = append(warnings, checkUnsupportedInPeerTree("template/match/"+entry.Key, entry.Value)...)
		}
		for _, entry := range tmpl.GetListOrdered("neighbor") {
			warnings = append(warnings, checkUnsupportedInPeerTree("template/neighbor/"+entry.Key, entry.Value)...)
		}
		if bgp := tmpl.GetContainer("bgp"); bgp != nil {
			for _, entry := range bgp.GetListOrdered("peer") {
				warnings = append(warnings, checkUnsupportedInPeerTree("template/bgp/peer/"+entry.Key, entry.Value)...)
			}
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

func configMigrateWithWarnings(inputPath, outputPath, outputForm string) (string, *migration.MigrateResult, []string, error) {
	data, err := cliio.ReadFile(inputPath)
	if err != nil {
		return "", nil, nil, err
	}

	content := string(data)

	// Parse any format: auto-detect and use the appropriate parser.
	schema, err := config.YANGSchema()
	if err != nil {
		return "", nil, nil, fmt.Errorf("YANG schema: %w", err)
	}

	sourceFormat := config.DetectFormat(content)
	var tree *config.Tree
	switch sourceFormat {
	case config.FormatSet, config.FormatSetMeta:
		tree, err = config.NewSetParser(schema).Parse(content)
	default:
		tree, err = config.NewParser(schema).Parse(content)
	}
	if err != nil {
		return "", nil, nil, fmt.Errorf("parse error: %w", err)
	}

	result, err := migration.Migrate(tree)
	if err != nil {
		return "", nil, nil, fmt.Errorf("migration failed: %w", err)
	}

	warnings := findUnsupportedFeatures(result.Tree)

	// Serialize in the requested output form (default: set).
	var output string
	if outputForm == formHierarchical {
		output = config.Serialize(result.Tree, schema)
	} else {
		output = config.SerializeSet(result.Tree, schema)
	}

	if outputPath != "" {
		// "-o -" writes the migrated config to stdout (result/warnings go to
		// stderr); an omitted -o already prints to stdout via the caller.
		if err := cliio.WriteFile(outputPath, []byte(output), 0o600); err != nil {
			return "", result, warnings, fmt.Errorf("write output: %w", err)
		}
		return "", result, warnings, nil
	}

	return output, result, warnings, nil
}
