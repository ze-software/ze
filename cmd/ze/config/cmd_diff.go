// Design: docs/architecture/config/syntax.md — config diff command
// Overview: main.go — dispatch and exit codes

package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"

	bgpconfig "codeberg.org/thomas-mangin/ze/internal/component/bgp/config"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

func cmdDiff(args []string) int {
	fs := flag.NewFlagSet("config diff", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "output as JSON")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze config diff [options] <file1> <file2>
       ze config diff [options] <N> <file>

Compare two configuration files and show differences.
When first argument is a number, compares current config against rollback revision N.
Operates on resolved config (after template expansion).
Use - for stdin (only one file can be stdin).

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Exit codes:
  0  Success (differences shown, or no differences)
  2  File not found or parse error
`)
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if fs.NArg() < 2 {
		fmt.Fprintf(os.Stderr, "error: requires two config files, or revision number and config file\n")
		fs.Usage()
		return exitError
	}

	// Check if first arg is a revision number (diff against rollback)
	file1 := fs.Arg(0)
	file2 := fs.Arg(1)
	if n, err := strconv.Atoi(file1); err == nil {
		resolved, err := resolveRollbackPath(file2, n)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
		file1 = resolved
	}

	tree1, err := loadAndResolve(file1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", file1, err)
		return exitError
	}

	tree2, err := loadAndResolve(file2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", file2, err)
		return exitError
	}

	diff := config.DiffMaps(tree1, tree2)

	if *jsonOutput {
		return outputDiffJSON(diff)
	}
	return outputDiffText(diff)
}

// resolveRollbackPath resolves a revision number to a rollback file path.
func resolveRollbackPath(configPath string, n int) (string, error) {
	ed, err := cli.NewEditor(configPath)
	if err != nil {
		return "", err
	}
	defer ed.Close() //nolint:errcheck // best effort cleanup

	backups, err := ed.ListBackups()
	if err != nil {
		return "", err
	}

	if n < 1 || n > len(backups) {
		return "", fmt.Errorf("revision %d not found (have %d revisions)", n, len(backups))
	}

	return backups[n-1].Path, nil
}

// loadAndResolve loads a config file, parses it, and resolves the BGP tree.
func loadAndResolve(path string) (map[string]any, error) {
	data, err := loadConfigData(path)
	if err != nil {
		return nil, err
	}

	schema := config.YANGSchema()
	if schema == nil {
		return nil, fmt.Errorf("failed to load YANG schema")
	}

	p := config.NewParser(schema)
	tree, err := p.Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	bgpTree, err := bgpconfig.ResolveBGPTree(tree)
	if err != nil {
		return nil, fmt.Errorf("resolve: %w", err)
	}

	return bgpTree, nil
}

func outputDiffJSON(diff *config.ConfigDiff) int {
	out := map[string]any{
		"added":   diff.Added,
		"removed": diff.Removed,
		"changed": diff.Changed,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "error: encoding JSON: %v\n", err)
		return exitError
	}
	return exitOK
}

func outputDiffText(diff *config.ConfigDiff) int {
	if len(diff.Added) == 0 && len(diff.Removed) == 0 && len(diff.Changed) == 0 {
		fmt.Println("No differences")
		return exitOK
	}

	if len(diff.Added) > 0 {
		keys := sortedKeys(diff.Added)
		for _, k := range keys {
			fmt.Printf("+ %s: %v\n", k, diff.Added[k])
		}
	}

	if len(diff.Removed) > 0 {
		keys := sortedKeys(diff.Removed)
		for _, k := range keys {
			fmt.Printf("- %s: %v\n", k, diff.Removed[k])
		}
	}

	if len(diff.Changed) > 0 {
		keys := make([]string, 0, len(diff.Changed))
		for k := range diff.Changed {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			pair := diff.Changed[k]
			fmt.Printf("~ %s: %v -> %v\n", k, pair.Old, pair.New)
		}
	}

	return exitOK
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
