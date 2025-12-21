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

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: zebgp config migrate [options] <config-file>

Convert configuration to current format (v3).

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
`)
	}

	if err := fs.Parse(args); err != nil {
		return exitError
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

	if *inPlace {
		backupPath, err := configMigrateInPlace(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
		fmt.Fprintf(os.Stderr, "✅ Backup created: %s\n", backupPath)
		fmt.Fprintf(os.Stderr, "✅ Config migrated: %s\n", configPath)
		return exitOK
	}

	output, err := configMigrate(configPath, *outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if *outputPath != "" {
		fmt.Fprintf(os.Stderr, "✅ Config migrated: %s\n", *outputPath)
	} else {
		fmt.Print(output)
	}

	return exitOK
}

// configMigrate reads a config file, migrates it, and returns the output.
// If outputPath is non-empty, writes to that file and returns empty string.
func configMigrate(inputPath, outputPath string) (string, error) {
	// Read file
	data, err := os.ReadFile(inputPath) //nolint:gosec // Config path from user
	if err != nil {
		return "", err
	}

	// Parse with schema
	p := config.NewParser(config.BGPSchema())
	tree, err := p.Parse(string(data))
	if err != nil {
		return "", fmt.Errorf("parse error: %w", err)
	}

	// Detect version
	version := migration.DetectVersion(tree)
	if version == migration.VersionCurrent {
		// Already current - just serialize
		output := config.Serialize(tree, config.BGPSchema())
		if outputPath != "" {
			if err := os.WriteFile(outputPath, []byte(output), 0o600); err != nil {
				return "", fmt.Errorf("write output: %w", err)
			}
			return "", nil
		}
		return output, nil
	}

	// Migrate v2 → v3
	migrated, err := migration.MigrateV2ToV3(tree)
	if err != nil {
		return "", fmt.Errorf("migration failed: %w", err)
	}

	// Serialize output
	output := config.Serialize(migrated, config.BGPSchema())

	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(output), 0o600); err != nil {
			return "", fmt.Errorf("write output: %w", err)
		}
		return "", nil
	}

	return output, nil
}

// configMigrateInPlace migrates a config file in place, creating a backup first.
// Returns the backup path.
func configMigrateInPlace(path string) (string, error) {
	// Create backup
	backupPath, err := createBackup(path)
	if err != nil {
		return "", fmt.Errorf("backup failed: %w", err)
	}

	// Migrate to same path
	_, err = configMigrate(path, path)
	if err != nil {
		return "", err
	}

	return backupPath, nil
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
