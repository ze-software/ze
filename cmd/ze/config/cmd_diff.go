// Design: docs/architecture/config/syntax.md — config diff command
// Overview: main.go — dispatch and exit codes

package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	bgpconfig "codeberg.org/thomas-mangin/ze/internal/component/bgp/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

func cmdDiff(args []string) int {
	fs := flag.NewFlagSet("config diff", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "output as JSON")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze config diff [options] <file1> <file2>

Compare two configuration files and show differences.
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
		fmt.Fprintf(os.Stderr, "error: requires two config files\n")
		fs.Usage()
		return exitError
	}

	tree1, err := loadAndResolve(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", fs.Arg(0), err)
		return exitError
	}

	tree2, err := loadAndResolve(fs.Arg(1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", fs.Arg(1), err)
		return exitError
	}

	diff := config.DiffMaps(tree1, tree2)

	if *jsonOutput {
		return outputDiffJSON(diff)
	}
	return outputDiffText(diff)
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
