// Design: docs/architecture/config/syntax.md — config diff command
// Overview: main.go — dispatch and exit codes

package cli

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/ze-software/ze/internal/component/config/infra"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
)

func cmdDiffWithStorage(store storage.Storage, args []string) int {
	return cmdDiffImpl(store, args)
}

func cmdDiff(args []string) int {
	return cmdDiffImpl(storage.NewFilesystem(), args)
}

func cmdDiffImpl(store storage.Storage, args []string) int {
	fs := flag.NewFlagSet("config diff", flag.ExitOnError)
	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze config diff",
			Summary: "Compare two configuration files and show differences",
			Usage: []string{
				"ze config diff <file1> <file2>",
				"ze config diff <N> <file>",
			},
			Sections: []helpfmt.HelpSection{
				{Title: helpSectionExitCodes, Entries: []helpfmt.HelpEntry{
					{Name: "0", Desc: "Success (differences shown, or no differences)"},
					{Name: "2", Desc: "File not found or parse error"},
				}},
			},
		}
		p.WriteErr()
		fmt.Fprintf(os.Stderr, "\nWhen first argument is a number, compares current config against rollback revision N.\n")
		fmt.Fprintf(os.Stderr, "Operates on resolved config (after template expansion).\n")
		fmt.Fprintf(os.Stderr, "Use - for stdin (only one file can be stdin).\n")
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	diff, code := resolveDiff(store, fs.Args())
	if code != exitOK {
		if fs.NArg() < 2 {
			fs.Usage()
		}
		return code
	}
	return outputDiffText(diff)
}

// resolveDiff compares the two configurations named in args, with every secret
// value masked, and answers the diff both spellings of this command render.
//
// It is the payload half of cmdDiffImpl, lifted so `show config diff` answers
// with DATA (dataDiff, config_data.go) and the two spellings cannot drift.
func resolveDiff(store storage.Storage, args []string) (*config.ConfigDiff, int) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "error: requires two config files, or revision number and config file\n")
		return nil, exitError
	}

	// Check if first arg is a revision number (diff against rollback)
	file1 := args[0]
	file2 := args[1]
	if n, err := strconv.Atoi(file1); err == nil {
		resolved, err := resolveRollbackPath(store, file2, n)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return nil, exitError
		}
		file1 = resolved
	}

	schema, err := config.YANGSchema()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: YANG schema: %v\n", err)
		return nil, exitError
	}

	tree1, err := loadAndResolve(store, schema, file1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", file1, err)
		return nil, exitError
	}

	tree2, err := loadAndResolve(store, schema, file2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", file2, err)
		return nil, exitError
	}

	diff := config.DiffMaps(tree1, tree2)
	maskDiffSecrets(diff, config.SecretKeys(schema))
	return diff, exitOK
}

// resolveRollbackPath resolves a revision number to a rollback file path.
func resolveRollbackPath(store storage.Storage, configPath string, n int) (string, error) {
	ed, err := cli.NewEditorWithStorage(store, configPath)
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

// loadAndResolve loads a config file via storage, parses it, and resolves the BGP tree.
// Supports "-" for stdin. The caller owns the schema, so one diff builds it once.
func loadAndResolve(store storage.Storage, schema *config.Schema, path string) (map[string]any, error) {
	// "-" reads stdin (claiming it once); a real path goes through the storage
	// abstraction, which may be a blob store where path is a key, not a file.
	var data []byte
	var err error
	if cliio.IsStdin(path) {
		data, err = cliio.ReadFile(path)
	} else {
		data, err = store.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}

	p := config.NewParser(schema)
	tree, err := p.Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	// Resolve the bgp{} section through the always-on seam; on a binary built
	// without the BGP engine (//go:build ze_bgp) a bgp-free config resolves to
	// an empty tree and the diff runs on the plain tree, while a config that
	// does carry bgp{} is an error rather than a diff missing its BGP half.
	bgpTree, err := infra.ResolveBGPTree(tree)
	if err != nil {
		return nil, fmt.Errorf("resolve: %w", err)
	}

	return bgpTree, nil
}

// maskDiffSecrets replaces every secret value the diff carries with the display
// placeholder, in the text shape and in the JSON shape alike.
//
// The diff runs on the REAL values first, so a rotated credential still reports
// as changed. Masking the two trees before the diff would give both sides the
// same placeholder, and the command would answer that nothing moved. The
// operator learns which leaf changed, and neither value. That is the answer
// Editor.Diff (internal/component/cli/editor.go) and the web commit review
// (changedSecretLines, internal/component/web/cli_terminal.go) already write.
//
// The parser decodes a $9$ value into the tree, so this command printed in
// cleartext what the file holds encoded.
func maskDiffSecrets(diff *config.ConfigDiff, secretKeys map[string]bool) {
	if diff == nil || len(secretKeys) == 0 {
		return
	}
	for key, value := range diff.Added {
		diff.Added[key] = maskDiffValue(key, value, secretKeys)
	}
	for key, value := range diff.Removed {
		diff.Removed[key] = maskDiffValue(key, value, secretKeys)
	}
	for key, pair := range diff.Changed {
		diff.Changed[key] = config.DiffPair{
			Old: maskDiffValue(key, pair.Old, secretKeys),
			New: maskDiffValue(key, pair.New, secretKeys),
		}
	}
}

// maskDiffValue masks one diff entry. A whole subtree is walked, because an
// added peer carries its own md5 password. A scalar is masked when the last
// segment of its path names a secret leaf.
//
// The name is what identifies the leaf here. ResolveBGPTree flattens group and
// peer inheritance, so a path in the resolved map addresses no schema node.
// `ze config dump` reads a name for the same reason.
func maskDiffValue(path string, value any, secretKeys map[string]bool) any {
	if subtree, ok := value.(map[string]any); ok {
		maskMapValues(subtree, secretKeys, config.DisplayStrip)
		return subtree
	}
	segments := config.SplitPath(path)
	if secretKeys[segments[len(segments)-1]] {
		return config.SecretDataPlaceholder
	}
	return value
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
