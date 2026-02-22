// Design: docs/architecture/config/syntax.md — config check command
// Related: main.go — dispatch and exit codes

package config

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/config"
	"codeberg.org/thomas-mangin/ze/internal/config/migration"
)

// checkResult holds results from config check.
type checkResult struct {
	needsMigration bool
	deprecated     []string
	unsupported    []string
	err            error
}

func cmdCheck(args []string) int {
	fs := flag.NewFlagSet("config check", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "output as JSON")
	envOnly := fs.Bool("env", false, "validate environment variables only (no config file needed)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze config check [options] <config-file>

Show config status and deprecated patterns that need migration.
Use --env to validate environment variables without a config file.
Use - to read from stdin.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Exit codes:
  0  Config is current / environment valid
  1  Config needs migration (old syntax)
  2  File not found, parse error, or invalid environment
`)
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if *envOnly {
		return checkEnvironment(*jsonOutput)
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file (use - for stdin)\n")
		fs.Usage()
		return exitError
	}

	configPath := fs.Arg(0)
	result := configCheckData(configPath)

	if result.err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", result.err)
		return exitError
	}

	if *jsonOutput {
		return outputCheckJSON(result)
	}

	return outputCheckText(result)
}

func checkEnvironment(jsonOutput bool) int {
	_, err := config.LoadEnvironment()
	if err != nil {
		if jsonOutput {
			fmt.Printf(`{"status":"invalid","error":%q}`+"\n", err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "error: environment validation failed: %v\n", err)
		}
		return exitError
	}

	if jsonOutput {
		fmt.Println(`{"status":"valid"}`)
	} else {
		fmt.Println("Environment variables valid")
	}
	return exitOK
}

func configCheckData(path string) checkResult {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path) //nolint:gosec // Config path from user
	}
	if err != nil {
		return checkResult{err: err}
	}

	p := config.NewParser(config.YANGSchema())
	tree, err := p.Parse(string(data))
	if err != nil {
		return checkResult{err: fmt.Errorf("parse error: %w", err)}
	}

	needsMigration := migration.NeedsMigration(tree)
	deprecated := findDeprecatedPatterns(tree)
	unsupported := findUnsupportedFeatures(tree)

	return checkResult{
		needsMigration: needsMigration,
		deprecated:     deprecated,
		unsupported:    unsupported,
	}
}

func findDeprecatedPatterns(tree *config.Tree) []string {
	var deprecated []string

	if hasListEntries(tree, "neighbor") {
		for _, entry := range tree.GetListOrdered("neighbor") {
			deprecated = append(deprecated, fmt.Sprintf("neighbor %s -> peer %s", entry.Key, entry.Key))
		}
	}

	for _, entry := range tree.GetListOrdered("peer") {
		if isGlobPattern(entry.Key) {
			deprecated = append(deprecated, fmt.Sprintf("peer %s -> template { match %s }", entry.Key, entry.Key))
		}
	}

	if tmpl := tree.GetContainer("template"); tmpl != nil {
		for _, entry := range tmpl.GetListOrdered("neighbor") {
			deprecated = append(deprecated, fmt.Sprintf("template.neighbor %s -> template.group %s", entry.Key, entry.Key))
		}
	}

	for _, entry := range tree.GetListOrdered("neighbor") {
		if entry.Value.GetContainer("static") != nil {
			deprecated = append(deprecated, fmt.Sprintf("neighbor.%s.static -> peer.%s.announce.<afi>.<safi>", entry.Key, entry.Key))
		}
	}
	for _, entry := range tree.GetListOrdered("peer") {
		if entry.Value.GetContainer("static") != nil {
			deprecated = append(deprecated, fmt.Sprintf("peer.%s.static -> peer.%s.announce.<afi>.<safi>", entry.Key, entry.Key))
		}
	}

	if tmpl := tree.GetContainer("template"); tmpl != nil {
		for _, entry := range tmpl.GetListOrdered("group") {
			if entry.Value.GetContainer("static") != nil {
				deprecated = append(deprecated, fmt.Sprintf("template.group.%s.static -> template.group.%s.announce.<afi>.<safi>", entry.Key, entry.Key))
			}
		}
		for _, entry := range tmpl.GetListOrdered("match") {
			if entry.Value.GetContainer("static") != nil {
				deprecated = append(deprecated, fmt.Sprintf("template.match.%s.static -> template.match.%s.announce.<afi>.<safi>", entry.Key, entry.Key))
			}
		}
	}

	return deprecated
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

func hasListEntries(tree *config.Tree, name string) bool {
	return len(tree.GetListOrdered(name)) > 0
}

func isGlobPattern(pattern string) bool {
	return strings.Contains(pattern, "*") || strings.Contains(pattern, "/")
}

func outputCheckText(result checkResult) int {
	if !result.needsMigration {
		fmt.Println("Config is current - no migration needed")
	} else {
		fmt.Println("Config needs migration")
		fmt.Println()
		fmt.Println("Deprecated patterns found:")
		for _, d := range result.deprecated {
			fmt.Printf("  - %s\n", d)
		}
		fmt.Println()
		fmt.Println("To migrate, run:")
		fmt.Println("  ze config migrate <file> -o <output>")
	}

	if len(result.unsupported) > 0 {
		fmt.Println()
		fmt.Println("Unsupported features detected (will be ignored):")
		for _, w := range result.unsupported {
			fmt.Printf("  - %s\n", w)
		}
	}

	if result.needsMigration {
		return exitMigrationNeeded
	}
	return exitOK
}

func outputCheckJSON(result checkResult) int {
	status := "current"
	exitCode := exitOK
	if result.needsMigration {
		status = "needs-migration"
		exitCode = exitMigrationNeeded
	}

	fmt.Printf(`{"status":%q,"deprecated":[`, status)
	for i, d := range result.deprecated {
		if i > 0 {
			fmt.Print(",")
		}
		fmt.Printf("%q", d)
	}
	fmt.Print(`],"unsupported":[`)
	for i, w := range result.unsupported {
		if i > 0 {
			fmt.Print(",")
		}
		fmt.Printf("%q", w)
	}
	fmt.Println("]}")

	return exitCode
}
