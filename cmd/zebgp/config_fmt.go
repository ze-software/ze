package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/zebgp/pkg/config"
	"codeberg.org/thomas-mangin/zebgp/pkg/config/migration"
)

// ErrOldConfig is returned when fmt is called on an old ExaBGP config.
var ErrOldConfig = errors.New("config needs migration")

// cmdConfigFmtCLI is the CLI entry point for "zebgp config fmt".
func cmdConfigFmtCLI(args []string) int {
	fs := flag.NewFlagSet("config fmt", flag.ExitOnError)
	write := fs.Bool("w", false, "write result to source file")
	check := fs.Bool("check", false, "check if formatting needed (exit 1 if changes)")
	diff := fs.Bool("diff", false, "show unified diff of changes")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: zebgp config fmt [options] <config-file>

Format and normalize configuration file.

Formats current config files only. For old ExaBGP configs, run "zebgp config migrate" first.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Exit codes:
  0  Success (or no changes needed with --check)
  1  Changes needed (with --check)
  2  Error (file not found, parse error, old config detected)

Examples:
  zebgp config fmt config.conf          # Print formatted config to stdout
  zebgp config fmt -w config.conf       # Write back to file
  zebgp config fmt --check config.conf  # Check if formatting needed (for CI)
  zebgp config fmt --diff config.conf   # Show what would change
  zebgp config fmt -                    # Read from stdin, write to stdout
`)
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file (use - for stdin)\n")
		fs.Usage()
		return exitError
	}

	configPath := fs.Arg(0)

	// Read input
	var input []byte
	var err error
	if configPath == "-" {
		input, err = io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: reading stdin: %v\n", err)
			return exitError
		}
	} else {
		input, err = os.ReadFile(configPath) //nolint:gosec // Config path from user
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
	}

	// Parse with current schema
	p := config.NewParser(config.BGPSchema())
	tree, err := p.Parse(string(input))
	if err != nil {
		// Try legacy schema to detect old ExaBGP syntax
		pLegacy := config.NewParser(config.LegacyBGPSchema())
		treeLegacy, errLegacy := pLegacy.Parse(string(input))
		if errLegacy == nil {
			if migration.NeedsMigration(treeLegacy) {
				fmt.Fprintf(os.Stderr, "error: config needs migration, run 'zebgp config migrate' first\n")
				return exitError
			}
		}
		fmt.Fprintf(os.Stderr, "error: parse error: %v\n", err)
		return exitError
	}

	// Reject configs that need migration
	if migration.NeedsMigration(tree) {
		fmt.Fprintf(os.Stderr, "error: config needs migration, run 'zebgp config migrate' first\n")
		return exitError
	}

	// Serialize (formats the output)
	formatted := config.Serialize(tree, config.BGPSchema())

	// Compare with original
	hasChanges := string(input) != formatted

	if *check {
		if hasChanges {
			if configPath == "-" {
				fmt.Fprintf(os.Stderr, "stdin needs formatting\n")
			} else {
				fmt.Fprintf(os.Stderr, "%s needs formatting\n", configPath)
			}
			return 1
		}
		return exitOK
	}

	if *diff {
		if hasChanges {
			printDiff(configPath, string(input), formatted)
		}
		return exitOK
	}

	if *write {
		if configPath == "-" {
			fmt.Fprintf(os.Stderr, "error: cannot use -w with stdin\n")
			return exitError
		}
		if hasChanges {
			if err := os.WriteFile(configPath, []byte(formatted), 0o600); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return exitError
			}
			fmt.Fprintf(os.Stderr, "✅ Formatted: %s\n", configPath)
		}
		return exitOK
	}

	// Default: print to stdout
	fmt.Print(formatted)
	return exitOK
}

// configFmtBytes formats config bytes and returns formatted output and whether changes were made.
// Returns ErrOldConfig if the config needs migration.
func configFmtBytes(input []byte) (string, bool, error) {
	// Parse with current schema
	p := config.NewParser(config.BGPSchema())
	tree, err := p.Parse(string(input))
	if err != nil {
		// Try legacy schema to detect old ExaBGP syntax
		pLegacy := config.NewParser(config.LegacyBGPSchema())
		treeLegacy, errLegacy := pLegacy.Parse(string(input))
		if errLegacy == nil {
			if migration.NeedsMigration(treeLegacy) {
				return "", false, ErrOldConfig
			}
		}
		return "", false, fmt.Errorf("parse error: %w", err)
	}

	// Reject configs that need migration
	if migration.NeedsMigration(tree) {
		return "", false, ErrOldConfig
	}

	// Serialize (formats the output)
	formatted := config.Serialize(tree, config.BGPSchema())
	hasChanges := string(input) != formatted

	return formatted, hasChanges, nil
}

// printDiff prints a unified diff between original and formatted content.
func printDiff(path, original, formatted string) {
	origLines := strings.Split(original, "\n")
	fmtLines := strings.Split(formatted, "\n")

	// Simple line-by-line diff (not a true unified diff, but useful)
	fmt.Fprintf(os.Stderr, "--- %s (original)\n", path)
	fmt.Fprintf(os.Stderr, "+++ %s (formatted)\n", path)

	// Find differences
	maxLines := len(origLines)
	if len(fmtLines) > maxLines {
		maxLines = len(fmtLines)
	}

	inHunk := false
	hunkStart := 0

	for i := 0; i < maxLines; i++ {
		origLine := ""
		fmtLine := ""
		if i < len(origLines) {
			origLine = origLines[i]
		}
		if i < len(fmtLines) {
			fmtLine = fmtLines[i]
		}

		if origLine != fmtLine {
			if !inHunk {
				inHunk = true
				hunkStart = i
				// Print context (up to 3 lines before)
				start := i - 3
				if start < 0 {
					start = 0
				}
				fmt.Fprintf(os.Stderr, "@@ -%d,%d +%d,%d @@\n", start+1, len(origLines)-start, start+1, len(fmtLines)-start)
				for j := start; j < i; j++ {
					if j < len(origLines) {
						fmt.Fprintf(os.Stderr, " %s\n", origLines[j])
					}
				}
			}
			if i < len(origLines) && origLine != "" {
				fmt.Fprintf(os.Stderr, "-%s\n", origLine)
			}
			if i < len(fmtLines) && fmtLine != "" {
				fmt.Fprintf(os.Stderr, "+%s\n", fmtLine)
			}
		} else if inHunk {
			// Print some context after
			fmt.Fprintf(os.Stderr, " %s\n", origLine)
			if i-hunkStart > 6 {
				inHunk = false
			}
		}
	}
}
