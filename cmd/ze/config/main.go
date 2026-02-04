// Package config provides the ze config subcommand.
package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/config"
	"codeberg.org/thomas-mangin/ze/internal/config/editor"
	"codeberg.org/thomas-mangin/ze/internal/config/migration"
	tea "github.com/charmbracelet/bubbletea"
)

// Exit codes for config commands.
const (
	exitOK              = 0 // Success
	exitMigrationNeeded = 1 // Config needs migration (check command)
	exitError           = 2 // Error (file not found, parse error, etc.)
)

// subcommandHandlers maps subcommand names to their handler functions.
// Using a map avoids both if-else chains (gocritic lint) and switch default
// (hook false positive for /config/ path).
var subcommandHandlers = map[string]func([]string) int{
	"edit":    cmdEdit,
	"check":   cmdCheck,
	"migrate": cmdMigrate,
	"fmt":     cmdFmt,
	"dump":    cmdDump,
}

// Run executes the config subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}

	subcmd := args[0]
	subArgs := args[1:]

	// Check for help first
	if subcmd == "help" || subcmd == "-h" || subcmd == "--help" {
		usage()
		return 0
	}

	// Look up handler in map
	if handler, ok := subcommandHandlers[subcmd]; ok {
		return handler(subArgs)
	}

	// Unknown subcommand
	fmt.Fprintf(os.Stderr, "unknown config subcommand: %s\n", subcmd)
	usage()
	return 1
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: ze config <command> [options]

Configuration management commands.

Commands:
  edit <file>    Interactive configuration editor
  check <file>   Check config status and deprecated patterns
  migrate <file> Convert configuration to current format
  fmt <file>     Format and normalize configuration file
  dump <file>    Dump parsed configuration

Examples:
  ze config edit config.conf
  ze config check config.conf
  ze config migrate config.conf -o new.conf
  ze config fmt config.conf
  ze config dump config.conf
`)
}

// ============================================================================
// edit command
// ============================================================================

func cmdEdit(args []string) int {
	fs := flag.NewFlagSet("config edit", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze config edit [options] <config-file>

Interactive configuration editor with VyOS-like set commands.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Commands:
  set <path> <value>    Set a configuration value
  delete <path>         Delete a configuration value
  edit <path>           Enter a subsection (narrowed context)
  edit <list> *         Edit template for all entries (inheritance)
  top                   Return to root context
  up                    Go up one level
  show [section]        Display current configuration
  compare               Show diff vs original
  commit                Save changes (creates backup)
  discard               Revert all changes
  history               List backup files
  rollback <N>          Restore backup N
  exit/quit             Exit (prompts if unsaved changes)

Tab completion:
  Type partial text + Tab for completion
  Multiple matches show dropdown, Tab cycles through
  Ghost text shows best match in gray

Examples:
  ze config edit /etc/ze/config.conf
  ze config edit ./myconfig.conf
`)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file\n")
		fs.Usage()
		return 1
	}

	configPath := fs.Arg(0)

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: config file not found: %s\n", configPath)
		return 1
	}

	// Create editor
	ed, err := editor.NewEditor(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	defer ed.Close() //nolint:errcheck // Best effort cleanup

	// Check for pending edit file from previous session
	if ed.HasPendingEdit() {
		switch ed.PromptPendingEdit() {
		case editor.PendingEditContinue:
			if err := ed.LoadPendingEdit(); err != nil {
				fmt.Fprintf(os.Stderr, "error loading edit file: %v\n", err)
				return 1
			}
		case editor.PendingEditDiscard:
			if err := ed.Discard(); err != nil {
				fmt.Fprintf(os.Stderr, "error discarding edit file: %v\n", err)
				return 1
			}
		case editor.PendingEditQuit:
			return 0
		}
	}

	// Create model
	m, err := editor.NewModel(ed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Run Bubble Tea program
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}

// ============================================================================
// check command
// ============================================================================

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

// ============================================================================
// migrate command
// ============================================================================

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

// ============================================================================
// fmt command
// ============================================================================

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
			fmt.Fprintf(os.Stderr, " %s\n", origLine)
			if i-hunkStart > 6 {
				inHunk = false
			}
		}
	}
}

// ============================================================================
// dump command
// ============================================================================

func cmdDump(args []string) int {
	fs := flag.NewFlagSet("config dump", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "output as JSON")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze config dump [options] <config>

Dump parsed configuration in human-readable or JSON format.
Useful for debugging config parsing issues.
Use - to read from stdin.

Options:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file (use - for stdin)\n")
		fs.Usage()
		return 1
	}

	configPath := fs.Arg(0)

	var data []byte
	var err error
	if configPath == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(configPath) //nolint:gosec // Path from CLI
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		return 1
	}

	schema := config.YANGSchema()
	if schema == nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load YANG schema\n")
		return 1
	}
	p := config.NewParser(schema)
	tree, err := p.Parse(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing config: %v\n", err)
		return 1
	}

	if warnings := p.Warnings(); len(warnings) > 0 {
		fmt.Fprintf(os.Stderr, "Warnings:\n")
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "  %s\n", w)
		}
		fmt.Fprintln(os.Stderr)
	}

	cfg, err := config.TreeToConfig(tree)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error converting config: %v\n", err)
		return 1
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	printConfig(cfg)
	return 0
}

func printConfig(cfg *config.BGPConfig) {
	fmt.Printf("router-id: %s\n", uint32ToIP(cfg.RouterID))
	fmt.Printf("local-as: %d\n", cfg.LocalAS)
	if cfg.Listen != "" {
		fmt.Printf("listen: %s\n", cfg.Listen)
	}
	fmt.Println()

	for _, n := range cfg.Peers {
		fmt.Printf("peer %s:\n", n.Address)
		if n.Description != "" {
			fmt.Printf("  description: %s\n", n.Description)
		}
		if n.RouterID != 0 {
			fmt.Printf("  router-id: %s\n", uint32ToIP(n.RouterID))
		}
		if n.LocalAddress.IsValid() {
			fmt.Printf("  local-address: %s\n", n.LocalAddress)
		}
		if n.LocalAS != 0 {
			fmt.Printf("  local-as: %d\n", n.LocalAS)
		}
		if n.PeerAS != 0 {
			fmt.Printf("  peer-as: %d\n", n.PeerAS)
		}
		if n.HoldTime != 0 {
			fmt.Printf("  hold-time: %d\n", n.HoldTime)
		}
		if n.Passive {
			fmt.Printf("  passive: true\n")
		}

		if len(n.Families) > 0 {
			fmt.Printf("  families:\n")
			for _, f := range n.Families {
				fmt.Printf("    - %s\n", f)
			}
		}

		cap := n.Capabilities
		if cap.ASN4 || cap.RouteRefresh || cap.GracefulRestart || cap.AddPathSend || cap.AddPathReceive || cap.SoftwareVersion {
			fmt.Printf("  capabilities:\n")
			if cap.ASN4 {
				fmt.Printf("    asn4: true\n")
			}
			if cap.RouteRefresh {
				fmt.Printf("    route-refresh: true\n")
			}
			if cap.GracefulRestart {
				fmt.Printf("    graceful-restart: true (restart-time: %d)\n", cap.RestartTime)
			}
			if cap.AddPathSend {
				fmt.Printf("    add-path-send: true\n")
			}
			if cap.AddPathReceive {
				fmt.Printf("    add-path-receive: true\n")
			}
			if cap.SoftwareVersion {
				fmt.Printf("    software-version: true\n")
			}
		}

		if len(n.StaticRoutes) > 0 {
			fmt.Printf("  static-routes:\n")
			for _, sr := range n.StaticRoutes {
				fmt.Printf("    - prefix: %s\n", sr.Prefix)
				if sr.NextHop != "" {
					fmt.Printf("      next-hop: %s\n", sr.NextHop)
				}
				if sr.LocalPreference != 0 {
					fmt.Printf("      local-preference: %d\n", sr.LocalPreference)
				}
				if sr.MED != 0 {
					fmt.Printf("      med: %d\n", sr.MED)
				}
				if sr.Community != "" {
					fmt.Printf("      community: %s\n", sr.Community)
				}
				if sr.ASPath != "" {
					fmt.Printf("      as-path: %s\n", sr.ASPath)
				}
			}
		}

		fmt.Println()
	}

	if len(cfg.Plugins) > 0 {
		fmt.Printf("plugins:\n")
		for _, pl := range cfg.Plugins {
			fmt.Printf("  - name: %s\n", pl.Name)
			if pl.Run != "" {
				fmt.Printf("    run: %s\n", pl.Run)
			}
			if pl.Encoder != "" {
				fmt.Printf("    encoder: %s\n", pl.Encoder)
			}
		}
	}
}

func uint32ToIP(n uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		(n>>24)&0xFF, (n>>16)&0xFF, (n>>8)&0xFF, n&0xFF)
}
