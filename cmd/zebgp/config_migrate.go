package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/exa-networks/zebgp/pkg/config"
	"github.com/exa-networks/zebgp/pkg/config/migration"
)

// cmdConfigMigrateCLI is the CLI entry point for "zebgp config migrate".
func cmdConfigMigrateCLI(args []string) int {
	fs := flag.NewFlagSet("config migrate", flag.ExitOnError)
	outputPath := fs.String("o", "", "write output to file")
	inPlace := fs.Bool("in-place", false, "modify file in place (creates backup)")
	dryRun := fs.Bool("dry-run", false, "show what would be migrated without making changes")
	listTransforms := fs.Bool("list", false, "list available transformations")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: zebgp config migrate [options] <config-file>

Convert configuration to current format.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Exit codes:
  0  Success
  2  Error (file not found, parse error, write error)

Examples:
  zebgp config migrate config.conf              # Output to stdout
  zebgp config migrate config.conf -o new.conf  # Output to file
  zebgp config migrate config.conf --in-place   # Modify in place (with backup)
  zebgp config migrate config.conf --dry-run    # Preview transformations
  zebgp config migrate --list                   # List available transformations
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
	if *dryRun && (*inPlace || *outputPath != "") {
		fmt.Fprintf(os.Stderr, "error: --dry-run cannot be combined with --in-place or -o\n")
		return exitError
	}

	// Handle --dry-run
	if *dryRun {
		return cmdConfigMigrateDryRun(configPath)
	}

	if *inPlace {
		result, backupPath, warnings, err := configMigrateInPlaceWithWarnings(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
		fmt.Fprintf(os.Stderr, "✅ Backup created: %s\n", backupPath)
		fmt.Fprintf(os.Stderr, "✅ Config migrated: %s\n", configPath)
		printMigrateResult(result)
		printMigrateWarnings(warnings)
		return exitOK
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

	// Parse with schema
	p := config.NewParser(config.LegacyBGPSchema())
	tree, err := p.Parse(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parse error: %v\n", err)
		return exitError
	}

	// Run dry-run
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

	// Parse with schema
	p := config.NewParser(config.LegacyBGPSchema())
	tree, err := p.Parse(string(data))
	if err != nil {
		return "", nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Migrate using new API
	result, err := migration.Migrate(tree)
	if err != nil {
		return "", nil, nil, fmt.Errorf("migration failed: %w", err)
	}

	// Detect unsupported features in the migrated tree
	warnings := findUnsupportedFeatures(result.Tree)

	// Serialize output
	output := config.Serialize(result.Tree, config.BGPSchema())

	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(output), 0o600); err != nil {
			return "", result, warnings, fmt.Errorf("write output: %w", err)
		}
		return "", result, warnings, nil
	}

	return output, result, warnings, nil
}

// configMigrateInPlace migrates a config file in place, creating a backup first.
// Returns the backup path.
func configMigrateInPlace(path string) (string, error) {
	_, backupPath, _, err := configMigrateInPlaceWithWarnings(path)
	return backupPath, err
}

// configMigrateInPlaceWithWarnings is like configMigrateInPlace but also returns migration result and warnings.
func configMigrateInPlaceWithWarnings(path string) (*migration.MigrateResult, string, []string, error) {
	// Create backup
	backupPath, err := createBackup(path)
	if err != nil {
		return nil, "", nil, fmt.Errorf("backup failed: %w", err)
	}

	// Migrate to same path
	_, result, warnings, err := configMigrateWithWarnings(path, path)
	if err != nil {
		return nil, backupPath, warnings, err
	}

	return result, backupPath, warnings, nil
}

// createBackup creates a timestamped backup of the file.
// Returns the backup path.
func createBackup(path string) (string, error) {
	// Read original
	data, err := os.ReadFile(path) //nolint:gosec // Config path from user
	if err != nil {
		return "", err
	}

	// Generate backup filename
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("%s.%s.bak", base, timestamp)
	backupPath := filepath.Join(dir, backupName)

	// Write backup
	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		return "", err
	}

	return backupPath, nil
}
