// Design: docs/architecture/config/syntax.md — config fmt command
// Overview: main.go — dispatch and exit codes

package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/config"
)

// ErrOldConfig is returned when fmt is called on an old config.
var ErrOldConfig = errors.New("config needs migration")

// ConfigFmtBytes formats config bytes and returns formatted output and whether changes were made.
// Exported for testing.
func ConfigFmtBytes(input []byte) (string, bool, error) {
	schema := config.YANGSchema()
	if schema == nil {
		return "", false, fmt.Errorf("failed to load YANG schema")
	}
	p := config.NewParser(schema)
	tree, err := p.Parse(string(input))
	if err != nil {
		return "", false, fmt.Errorf("parse error: %w", err)
	}

	formatted := config.Serialize(tree, schema)
	hasChanges := string(input) != formatted

	return formatted, hasChanges, nil
}

func cmdFmt(args []string) int {
	fs := flag.NewFlagSet("config fmt", flag.ExitOnError)
	write := fs.Bool("w", false, "write result to source file")
	check := fs.Bool("check", false, "check if formatting needed (exit 1 if changes)")
	diff := fs.Bool("diff", false, "show unified diff of changes")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze config fmt [options] <config-file>

Format and normalize configuration file.

Formats current config files only. For old configs, run "ze config migrate" first.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Exit codes:
  0  Success (or no changes needed with --check)
  1  Changes needed (with --check)
  2  Error (file not found, parse error, old config detected)

Examples:
  ze config fmt config.conf          # Print formatted config to stdout
  ze config fmt -w config.conf       # Write back to file
  ze config fmt --check config.conf  # Check if formatting needed (for CI)
  ze config fmt --diff config.conf   # Show what would change
  ze config fmt -                    # Read from stdin, write to stdout
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

	schema := config.YANGSchema()
	if schema == nil {
		fmt.Fprintf(os.Stderr, "error: failed to load YANG schema\n")
		return exitError
	}
	p := config.NewParser(schema)
	tree, err := p.Parse(string(input))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parse error: %v\n", err)
		return exitError
	}

	formatted := config.Serialize(tree, schema)
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
			fmt.Fprintf(os.Stderr, "Formatted: %s\n", configPath)
		}
		return exitOK
	}

	fmt.Print(formatted)
	return exitOK
}

func printDiff(path, original, formatted string) {
	origLines := strings.Split(original, "\n")
	fmtLines := strings.Split(formatted, "\n")

	fmt.Fprintf(os.Stderr, "--- %s (original)\n", path)
	fmt.Fprintf(os.Stderr, "+++ %s (formatted)\n", path)

	maxLines := max(len(fmtLines), len(origLines))

	inHunk := false
	hunkStart := 0

	for i := range maxLines {
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
				start := max(i-3, 0)
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
			fmt.Fprintf(os.Stderr, " %s\n", origLine)
			if i-hunkStart > 6 {
				inHunk = false
			}
		}
	}
}
