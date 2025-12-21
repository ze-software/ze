package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/exa-networks/zebgp/pkg/config"
	"github.com/exa-networks/zebgp/pkg/config/migration"
)

// Exit codes for config commands.
const (
	exitOK              = 0 // Success
	exitMigrationNeeded = 1 // Config needs migration (check command)
	exitError           = 2 // Error (file not found, parse error, etc.)
)

// cmdConfigCheckCLI is the CLI entry point for "zebgp config check".
func cmdConfigCheckCLI(args []string) int {
	fs := flag.NewFlagSet("config check", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "output as JSON")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: zebgp config check [options] <config-file>

Show config version and deprecated patterns that need migration.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Exit codes:
  0  Config is current (v3)
  1  Config needs migration (v2 or older)
  2  File not found or parse error
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

	result := configCheck(configPath)

	if result.err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", result.err)
		return exitError
	}

	if *jsonOutput {
		return outputCheckJSON(result)
	}

	return outputCheckText(result)
}

// checkResult holds results from config check.
type checkResult struct {
	version    string
	isCurrent  bool
	deprecated []string
	err        error
}

// configCheck analyzes a config file and returns version/deprecation info.
func configCheck(path string) checkResult {
	// Read file
	data, err := os.ReadFile(path) //nolint:gosec // Config path from user
	if err != nil {
		return checkResult{err: err}
	}

	// Parse with schema
	p := config.NewParser(config.BGPSchema())
	tree, err := p.Parse(string(data))
	if err != nil {
		return checkResult{err: fmt.Errorf("parse error: %w", err)}
	}

	// Detect version
	version := migration.DetectVersion(tree)

	// Find deprecated patterns
	deprecated := findDeprecatedPatterns(tree)

	return checkResult{
		version:    version.String(),
		isCurrent:  version == migration.VersionCurrent,
		deprecated: deprecated,
	}
}

// findDeprecatedPatterns returns a list of deprecated patterns found in the tree.
func findDeprecatedPatterns(tree *config.Tree) []string {
	var deprecated []string

	// Check for neighbor at root (v2 pattern)
	if hasListEntries(tree, "neighbor") {
		for _, entry := range tree.GetListOrdered("neighbor") {
			deprecated = append(deprecated, fmt.Sprintf("neighbor %s → peer %s", entry.Key, entry.Key))
		}
	}

	// Check for peer glob at root (v2 pattern - should be template.match)
	for _, entry := range tree.GetListOrdered("peer") {
		if isGlobPattern(entry.Key) {
			deprecated = append(deprecated, fmt.Sprintf("peer %s → template { match %s }", entry.Key, entry.Key))
		}
	}

	// Check for template.neighbor (v2 pattern - should be template.group)
	if tmpl := tree.GetContainer("template"); tmpl != nil {
		for _, entry := range tmpl.GetListOrdered("neighbor") {
			deprecated = append(deprecated, fmt.Sprintf("template.neighbor %s → template.group %s", entry.Key, entry.Key))
		}
	}

	// Check for static blocks in any peer/neighbor (v2 pattern - should be announce)
	for _, entry := range tree.GetListOrdered("neighbor") {
		if entry.Value.GetContainer("static") != nil {
			deprecated = append(deprecated, fmt.Sprintf("neighbor.%s.static → peer.%s.announce.<afi>.<safi>", entry.Key, entry.Key))
		}
	}
	for _, entry := range tree.GetListOrdered("peer") {
		if entry.Value.GetContainer("static") != nil {
			deprecated = append(deprecated, fmt.Sprintf("peer.%s.static → peer.%s.announce.<afi>.<safi>", entry.Key, entry.Key))
		}
	}

	// Check template.group and template.match for static
	if tmpl := tree.GetContainer("template"); tmpl != nil {
		for _, entry := range tmpl.GetListOrdered("group") {
			if entry.Value.GetContainer("static") != nil {
				deprecated = append(deprecated, fmt.Sprintf("template.group.%s.static → template.group.%s.announce.<afi>.<safi>", entry.Key, entry.Key))
			}
		}
		// template.match+static is defensive (doesn't exist in practice)
		for _, entry := range tmpl.GetListOrdered("match") {
			if entry.Value.GetContainer("static") != nil {
				deprecated = append(deprecated, fmt.Sprintf("template.match.%s.static → template.match.%s.announce.<afi>.<safi>", entry.Key, entry.Key))
			}
		}
	}

	return deprecated
}

// hasListEntries returns true if tree has entries for the given list name.
func hasListEntries(tree *config.Tree, name string) bool {
	return len(tree.GetListOrdered(name)) > 0
}

// isGlobPattern returns true if pattern contains wildcards or CIDR notation.
func isGlobPattern(pattern string) bool {
	return strings.Contains(pattern, "*") || strings.Contains(pattern, "/")
}

func outputCheckText(result checkResult) int {
	if result.isCurrent {
		fmt.Printf("✅ Config version: %s (current)\n", result.version)
		fmt.Println("   No migration needed.")
		return exitOK
	}

	fmt.Printf("⚠️  Config version: %s\n", result.version)
	fmt.Printf("   Current version: %s\n", migration.VersionCurrent.String())
	fmt.Println()
	fmt.Println("Deprecated patterns found:")
	for _, d := range result.deprecated {
		fmt.Printf("  • %s\n", d)
	}
	fmt.Println()
	fmt.Println("To migrate, run:")
	fmt.Println("  zebgp config migrate <file> -o <output>")
	fmt.Println("  zebgp config migrate <file> --in-place")

	return exitMigrationNeeded
}

func outputCheckJSON(result checkResult) int {
	// Simple JSON without encoding/json for minimal size
	status := "current"
	exitCode := exitOK
	if !result.isCurrent {
		status = "needs-migration"
		exitCode = exitMigrationNeeded
	}

	fmt.Printf(`{"version":%q,"status":%q,"deprecated":[`, result.version, status)
	for i, d := range result.deprecated {
		if i > 0 {
			fmt.Print(",")
		}
		fmt.Printf("%q", d)
	}
	fmt.Println("]}")

	return exitCode
}
