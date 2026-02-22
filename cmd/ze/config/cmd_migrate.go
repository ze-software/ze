// Design: docs/architecture/config/syntax.md — config migrate command
// Related: main.go — dispatch and exit codes
// Related: cmd_check.go — findUnsupportedFeatures used by migrate

package config

import (
	"flag"
	"fmt"
	"io"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/config"
	"codeberg.org/thomas-mangin/ze/internal/config/migration"
)

func cmdMigrate(args []string) int {
	fs := flag.NewFlagSet("config migrate", flag.ExitOnError)
	outputPath := fs.String("o", "", "write output to file")
	dryRun := fs.Bool("dry-run", false, "show what would be migrated without making changes")
	listTransforms := fs.Bool("list", false, "list available transformations")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze config migrate [options] <config-file>

Convert configuration to current format.
Use - to read from stdin.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Exit codes:
  0  Success
  2  Error (file not found, parse error, write error)

Examples:
  ze config migrate config.conf              # Output to stdout
  ze config migrate config.conf -o new.conf  # Output to file
  ze config migrate config.conf --dry-run    # Preview transformations
  ze config migrate --list                   # List available transformations
  cat config.conf | ze config migrate -      # Read from stdin
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

	output, result, warnings, err := configMigrateWithWarnings(configPath, *outputPath)
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
	var data []byte
	var err error
	if configPath == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(configPath) //nolint:gosec // Config path from user
	}
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

func configMigrateWithWarnings(inputPath, outputPath string) (string, *migration.MigrateResult, []string, error) {
	var data []byte
	var err error
	if inputPath == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(inputPath) //nolint:gosec // Config path from user
	}
	if err != nil {
		return "", nil, nil, err
	}

	content := string(data)

	p := config.NewParser(config.YANGSchema())
	tree, err := p.Parse(content)
	if err != nil {
		return "", nil, nil, fmt.Errorf("parse error: %w", err)
	}

	result, err := migration.Migrate(tree)
	if err != nil {
		return "", nil, nil, fmt.Errorf("migration failed: %w", err)
	}

	warnings := findUnsupportedFeatures(result.Tree)

	schema := config.YANGSchema()
	if schema == nil {
		return "", nil, nil, fmt.Errorf("failed to load YANG schema")
	}
	output := config.Serialize(result.Tree, schema)

	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(output), 0o600); err != nil {
			return "", result, warnings, fmt.Errorf("write output: %w", err)
		}
		return "", result, warnings, nil
	}

	return output, result, warnings, nil
}
