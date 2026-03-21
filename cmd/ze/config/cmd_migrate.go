// Design: docs/architecture/config/syntax.md — config migrate command
// Overview: main.go — dispatch and exit codes

package config

import (
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/migration"
)

func cmdMigrate(args []string) int {
	fs := flag.NewFlagSet("config migrate", flag.ExitOnError)
	outputPath := fs.String("o", "", "write output to file")
	outputFormat := fs.String("format", "set", "output format: set (default) or hierarchical")
	dryRun := fs.Bool("dry-run", false, "show what would be migrated without making changes")
	listTransforms := fs.Bool("list", false, "list available transformations")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze config migrate [options] <config-file>

Convert configuration to current format. Default output is set format.
Use - to read from stdin.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Exit codes:
  0  Success
  2  Error (file not found, parse error, write error)

Examples:
  ze config migrate config.conf                          # Convert to set format (stdout)
  ze config migrate config.conf -o new.conf              # Convert to new file
  ze config migrate --format hierarchical config.conf    # Explicit hierarchical output
  ze config migrate config.conf --dry-run                # Preview transformations
  ze config migrate --list                               # List available transformations
  cat config.conf | ze config migrate -                  # Read from stdin
`)
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if *listTransforms {
		printTransformationList()
		return exitOK
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file (use - for stdin)\n")
		fs.Usage()
		return exitError
	}

	configPath := fs.Arg(0)

	if *dryRun && *outputPath != "" {
		fmt.Fprintf(os.Stderr, "error: --dry-run cannot be combined with -o\n")
		return exitError
	}

	if *dryRun {
		return cmdMigrateDryRun(configPath)
	}

	if *outputFormat != "set" && *outputFormat != "hierarchical" {
		fmt.Fprintf(os.Stderr, "error: --format must be 'set' or 'hierarchical'\n")
		return exitError
	}

	output, result, warnings, err := configMigrateWithWarnings(configPath, *outputPath, *outputFormat)
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
	data, err := loadConfigData(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	content := string(data)

	p := config.NewParser(config.YANGSchema())
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
			warnings = append(warnings, checkUnsupportedInPeerTree("template.group."+entry.Key, entry.Value)...)
		}
		for _, entry := range tmpl.GetListOrdered("match") {
			warnings = append(warnings, checkUnsupportedInPeerTree("template.match."+entry.Key, entry.Value)...)
		}
		for _, entry := range tmpl.GetListOrdered("neighbor") {
			warnings = append(warnings, checkUnsupportedInPeerTree("template.neighbor."+entry.Key, entry.Value)...)
		}
		if bgp := tmpl.GetContainer("bgp"); bgp != nil {
			for _, entry := range bgp.GetListOrdered("peer") {
				warnings = append(warnings, checkUnsupportedInPeerTree("template.bgp.peer."+entry.Key, entry.Value)...)
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

func configMigrateWithWarnings(inputPath, outputPath, outputFormat string) (string, *migration.MigrateResult, []string, error) {
	data, err := loadConfigData(inputPath)
	if err != nil {
		return "", nil, nil, err
	}

	content := string(data)

	// Parse any format: auto-detect and use the appropriate parser.
	schema := config.YANGSchema()
	if schema == nil {
		return "", nil, nil, fmt.Errorf("failed to load YANG schema")
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

	// Serialize in the requested output format (default: set).
	var output string
	if outputFormat == "hierarchical" {
		output = config.Serialize(result.Tree, schema)
	} else {
		output = config.SerializeSet(result.Tree, schema)
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(output), 0o600); err != nil {
			return "", result, warnings, fmt.Errorf("write output: %w", err)
		}
		return "", result, warnings, nil
	}

	return output, result, warnings, nil
}
