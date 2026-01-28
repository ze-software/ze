package bgp

import (
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/config"
	"codeberg.org/thomas-mangin/ze/internal/config/migration"
	"codeberg.org/thomas-mangin/ze/internal/exabgp"
)

// cmdConfigMigrateCLI is the CLI entry point for "ze bgp config migrate".
func cmdConfigMigrateCLI(args []string) int {
	fs := flag.NewFlagSet("config migrate", flag.ExitOnError)
	outputPath := fs.String("o", "", "write output to file")
	inPlace := fs.Bool("in-place", false, "modify file in place (creates backup)")
	dryRun := fs.Bool("dry-run", false, "show what would be migrated without making changes")
	listTransforms := fs.Bool("list", false, "list available transformations")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze bgp config migrate [options] <config-file>

Convert configuration to current format.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Exit codes:
  0  Success
  2  Error (file not found, parse error, write error)

Examples:
  ze bgp config migrate config.conf              # Output to stdout
  ze bgp config migrate config.conf -o new.conf  # Output to file
  ze bgp config migrate config.conf --in-place   # Modify in place (with backup)
  ze bgp config migrate config.conf --dry-run    # Preview transformations
  ze bgp config migrate --list                   # List available transformations
`)
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	// Handle --list (doesn't need a config file)
	if *listTransforms {
		printTransformationList()
		return exitOK
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file\n")
		fs.Usage()
		return exitError
	}

	configPath := fs.Arg(0)

	// Validate flags
	if *inPlace && *outputPath != "" {
		fmt.Fprintf(os.Stderr, "error: --in-place and -o are mutually exclusive\n")
		return exitError
	}
	if *dryRun && *outputPath != "" {
		fmt.Fprintf(os.Stderr, "error: --dry-run cannot be combined with -o\n")
		return exitError
	}

	if *inPlace {
		fmt.Fprintf(os.Stderr, "error: --in-place is no longer supported\n")
		fmt.Fprintf(os.Stderr, "Use: ze bgp config migrate input.conf -o output.conf\n")
		return exitError
	}

	// Handle --dry-run
	if *dryRun {
		return cmdConfigMigrateDryRun(configPath)
	}

	output, result, warnings, err := configMigrateWithWarnings(configPath, *outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if *outputPath != "" {
		fmt.Fprintf(os.Stderr, "✅ Config migrated: %s\n", *outputPath)
		printMigrateResult(result)
		printMigrateWarnings(warnings)
	} else {
		// Print progress to stderr, config to stdout
		printMigrateResult(result)
		fmt.Print(output)
		printMigrateWarnings(warnings)
	}

	return exitOK
}

// printTransformationList prints available transformations.
func printTransformationList() {
	transforms := migration.ListTransformations()
	fmt.Println("Available transformations (in order):")
	for _, t := range transforms {
		fmt.Printf("  %-25s %s\n", t.Name, t.Description)
	}
}

// printMigrateResult prints applied/skipped transformations.
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
		fmt.Fprintf(os.Stderr, "  ✅ %s\n", name)
	}
	for _, name := range result.Skipped {
		fmt.Fprintf(os.Stderr, "  ⏭️  %s (not needed)\n", name)
	}
	fmt.Fprintf(os.Stderr, "\n%d applied, %d skipped.\n", len(result.Applied), len(result.Skipped))
}

// cmdConfigMigrateDryRun shows what would be migrated without making changes.
func cmdConfigMigrateDryRun(configPath string) int {
	// Read file
	data, err := os.ReadFile(configPath) //nolint:gosec // Config path from user
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	content := string(data)

	// Try parsing with native Ze schema first
	p := config.NewParser(config.YANGSchema())
	tree, nativeErr := p.Parse(content)

	if nativeErr != nil {
		// Native parsing failed - check if it's valid ExaBGP syntax
		return cmdConfigMigrateDryRunExaBGP(content)
	}

	// Native parsing succeeded - run structural migration dry-run
	result, err := migration.DryRun(tree)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	// Build lookup maps for status
	alreadyDone := make(map[string]bool)
	wouldApply := make(map[string]bool)
	for _, name := range result.AlreadyDone {
		alreadyDone[name] = true
	}
	for _, name := range result.WouldApply {
		wouldApply[name] = true
	}

	// Print analysis in transformation order
	fmt.Println("Transformation analysis:")
	transforms := migration.ListTransformations()
	for _, t := range transforms {
		if alreadyDone[t.Name] {
			fmt.Printf("  ✅ %s (done)\n", t.Name)
		} else if wouldApply[t.Name] {
			if t.Name == result.FailedAt {
				fmt.Printf("  ❌ %s (would fail)\n", t.Name)
			} else {
				fmt.Printf("  ⏳ %s (pending)\n", t.Name)
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

// cmdConfigMigrateDryRunExaBGP handles dry-run for ExaBGP-format configs.
func cmdConfigMigrateDryRunExaBGP(content string) int {
	// Try parsing with ExaBGP schema
	exaTree, exaErr := exabgp.ParseExaBGPConfig(content)
	if exaErr != nil {
		fmt.Fprintf(os.Stderr, "error: config cannot be parsed\n")
		fmt.Fprintf(os.Stderr, "  parse error: %v\n", exaErr)
		return exitError
	}

	fmt.Println("ExaBGP config detected. Transformations that would apply:")
	fmt.Printf("  ⏳ exabgp->ze (neighbor→peer, announce→update, api→process)\n")

	// Check if RIB plugin would be injected
	if exabgp.NeedsRIBPlugin(exaTree) {
		fmt.Printf("  ⏳ rib-plugin-injected (GR or route-refresh detected)\n")
	}

	fmt.Println("\nResult: ExaBGP migration would succeed.")
	return exitOK
}

// printMigrateWarnings prints unsupported feature warnings to stderr.
func printMigrateWarnings(warnings []string) {
	if len(warnings) > 0 {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "⚠️  Unsupported features detected (will be ignored):")
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "  • %s\n", w)
		}
	}
}

// configMigrate reads a config file, migrates it, and returns the output.
// If outputPath is non-empty, writes to that file and returns empty string.
func configMigrate(inputPath, outputPath string) (string, error) {
	output, _, _, err := configMigrateWithWarnings(inputPath, outputPath)
	return output, err
}

// configMigrateWithWarnings is like configMigrate but also returns migration result and warnings.
func configMigrateWithWarnings(inputPath, outputPath string) (string, *migration.MigrateResult, []string, error) {
	// Read file
	data, err := os.ReadFile(inputPath) //nolint:gosec // Config path from user
	if err != nil {
		return "", nil, nil, err
	}

	content := string(data)

	// Try parsing with native Ze schema first
	p := config.NewParser(config.YANGSchema())
	tree, nativeErr := p.Parse(content)

	if nativeErr != nil {
		// Native parsing failed - try ExaBGP schema
		return migrateExaBGPConfig(content, outputPath)
	}

	// Native parsing succeeded - apply structural migrations
	result, err := migration.Migrate(tree)
	if err != nil {
		return "", nil, nil, fmt.Errorf("migration failed: %w", err)
	}

	// Detect unsupported features in the migrated tree
	warnings := findUnsupportedFeatures(result.Tree)

	// Serialize output using YANG-derived schema
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

// migrateExaBGPConfig handles migration of ExaBGP-format configs to native Ze format.
func migrateExaBGPConfig(content, outputPath string) (string, *migration.MigrateResult, []string, error) {
	// Parse with ExaBGP schema
	exaTree, exaErr := exabgp.ParseExaBGPConfig(content)
	if exaErr != nil {
		return "", nil, nil, fmt.Errorf("failed to parse config (tried both Ze and ExaBGP schemas):\n  ExaBGP parse error: %w", exaErr)
	}

	// Convert ExaBGP tree to Ze format
	exaResult, err := exabgp.MigrateFromExaBGP(exaTree)
	if err != nil {
		return "", nil, nil, fmt.Errorf("ExaBGP migration failed: %w", err)
	}

	// Build result for display
	result := &migration.MigrateResult{
		Tree:    exaResult.Tree,
		Applied: []string{"exabgp->ze (neighbor→peer, announce→update, api→process)"},
	}
	if exaResult.RIBInjected {
		result.Applied = append(result.Applied, "rib-plugin-injected")
	}

	// Collect warnings
	warnings := exaResult.Warnings

	// Serialize output
	output := exabgp.SerializeTree(exaResult.Tree)

	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(output), 0o600); err != nil {
			return "", result, warnings, fmt.Errorf("write output: %w", err)
		}
		return "", result, warnings, nil
	}

	return output, result, warnings, nil
}
