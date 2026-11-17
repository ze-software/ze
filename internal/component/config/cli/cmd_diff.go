// Design: docs/architecture/config/syntax.md — config diff command
// Overview: main.go — dispatch and exit codes

package cli

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/infra"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/configorder"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func cmdDiffWithStorage(store storage.Storage, args []string) int {
	return cmdDiffImpl(store, args)
}

func cmdDiff(args []string) int {
	return cmdDiffImpl(nil, args)
}

func cmdDiffImpl(store storage.Storage, args []string) int {
	fs := flag.NewFlagSet("config diff", flag.ExitOnError)
	backupPath := fs.String(flagBackup, "", "Compare configs inside a backup artifact")
	user := fs.String("user", "", "account that asks the running ze about a config that declares an external plugin")
	fs.Usage = func() {
		p := helpfmt.Page{
			Command:   "ze config diff",
			ShortHelp: "Compare two configuration files and show differences",
			Usage: []string{
				"ze config diff <file1> <file2>",
				"ze config diff <N> <file>",
				"ze config diff --backup <artifact> <name1> <name2>",
				"ze config diff --backup <artifact> <N> <name>",
			},
			Sections: []helpfmt.HelpSection{
				{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
					accountOptionHelp(),
				}},
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

	if *backupPath != "" {
		backup, err := openBackup(*backupPath, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
		defer backup.Close() //nolint:errcheck // Read-only comparison.
		store = backup
	}

	// typed is the operator's words after `ze config diff`, for the command a
	// failed ask of the running ze names (diffAsking).
	var typed textbuf.Buffer
	if *backupPath != "" {
		typed.Str("--").Str(flagBackup).Byte(' ').Str(*backupPath).Byte(' ')
	}
	typed.Str(strings.Join(fs.Args(), " "))

	var diag textbuf.Buffer
	diff, code := diffAsking(store, fs.Args(), typed.String(), command.RouteLocal, *user, routeAsk, &diag)
	diag.StdErr() //nolint:errcheck // CLI diagnostic, nothing to report a stderr failure to.
	if code != exitOK {
		if fs.NArg() < 2 {
			fs.Usage()
		}
		return code
	}
	return outputDiffText(diff)
}

// diffInputs is the two sides of one comparison, each read once.
type diffInputs struct {
	// first and second are the bytes of the two sides.
	first, second []byte
	// firstName and secondName name the sides in a refusal: the file as it
	// was given, or the history entry of a revision.
	firstName, secondName string
	// fromRevision is true when the first side is a rollback revision.
	fromRevision bool
	// secondErr is why the second side could not be read. compareDiffInputs
	// reports it after any refusal of the first side.
	secondErr error
}

// readDiffInputs reads the two sides args name, each once: two files, or a
// revision number and the file whose history holds it. It is the reading half
// of a comparison, apart from compareDiffInputs so the caller that asks the
// running ze about the two sides (diffAsking) reads them once and sends the
// same bytes the local comparison would parse; standard input cannot be read
// twice.
//
// A refusal goes into diag as whole lines, with a code that is not exitOK.
func readDiffInputs(store storage.Storage, args []string, diag *textbuf.Buffer) (diffInputs, int) {
	if len(args) != 2 {
		diag.Str("error: requires exactly two config files, or revision number and config file\n")
		return diffInputs{}, exitError
	}

	// Check if first arg is a revision number (diff against rollback)
	file1 := args[0]
	file2 := args[1]
	var revisionStore storage.Storage
	// fromRevision names the revision branch: an empty revision reads back
	// as zero bytes, so revisionData alone cannot say whether one was read.
	fromRevision := false
	var revisionData []byte
	if n, err := strconv.Atoi(file1); err == nil {
		if store == nil {
			revisionStore, err = storage.OpenReadOnly(resolve.StoreDir(file2), resolve.StoreDir(file2))
			if err != nil {
				diag.Str("error: config diff history: ").Err(err).Byte('\n')
				return diffInputs{}, exitError
			}
			defer revisionStore.Close() //nolint:errcheck // Read-only history.
		} else {
			revisionStore = store
		}
		resolved, data, err := resolveRollbackVersion(revisionStore, file2, n)
		if err != nil {
			diag.Str("error: ").Err(err).Byte('\n')
			return diffInputs{}, exitError
		}
		file1 = resolved
		revisionData = data
		fromRevision = true
	}

	firstStore := store
	if revisionStore != nil {
		firstStore = revisionStore
	}
	firstData := revisionData
	if !fromRevision {
		var readErr error
		firstData, readErr = readDiffSource(firstStore, file1)
		if readErr != nil {
			diag.Str("error: ").Str(file1).Str(": ").Err(readErr).Byte('\n')
			return diffInputs{}, exitError
		}
	}
	// Read each source once before selecting the schema. A failed second read
	// contributes no declaration; compareDiffInputs reports it after any
	// refusal of the first source, preserving the existing precedence over a
	// later stdin error.
	secondData, secondReadErr := readDiffSource(store, file2)
	return diffInputs{
		first:        firstData,
		second:       secondData,
		firstName:    file1,
		secondName:   file2,
		fromRevision: fromRevision,
		secondErr:    secondReadErr,
	}, exitOK
}

// compareDiffInputs compares the two sides readDiffInputs read, with every
// secret value masked, and answers the diff both spellings of the command
// render (`ze config diff` and `show config diff`, through diffAsking).
//
// It writes nothing itself: a refusal goes into diag as whole lines, with a
// nil diff, and each caller prints diag where its route puts diagnostics.
func compareDiffInputs(inputs diffInputs, diag *textbuf.Buffer) (*config.ConfigDiff, int) {
	file1, file2 := inputs.firstName, inputs.secondName
	firstData, secondData, secondReadErr := inputs.first, inputs.second, inputs.secondErr
	sources := make([]config.ConfigSource, 0, 2)
	sources = append(sources, config.ConfigSource{Text: string(firstData), Path: file1})
	if secondReadErr == nil {
		sources = append(sources, config.ConfigSource{Text: string(secondData), Path: file2})
	}
	schema, declared, err := config.SchemaWithDeclaredPlugins(sources...)
	if err != nil {
		diag.Str("error: YANG schema: ").Err(err).Byte('\n')
		return nil, exitError
	}

	tree1, err := parseAndResolve(schema, firstData)
	if err != nil {
		diag.Str("error: ").Str(file1).Str(": ").Err(declared.Explain(err)).Byte('\n')
		return nil, exitError
	}
	if secondReadErr != nil {
		diag.Str("error: ").Str(file2).Str(": ").Err(secondReadErr).Byte('\n')
		return nil, exitError
	}
	tree2, err := parseAndResolve(schema, secondData)
	if err != nil {
		diag.Str("error: ").Str(file2).Str(": ").Err(declared.Explain(err)).Byte('\n')
		return nil, exitError
	}

	return diffSources(schema, tree1, tree2), exitOK
}

// diffSources compares parsed and resolved sources from either input route.
// Secret masking and schema-declared list ordering have one producer.
func diffSources(schema *config.Schema, tree1, tree2 *diffSource) *config.ConfigDiff {
	diff := config.DiffMaps(tree1.resolved, tree2.resolved)
	diffListOrders(tree1.parsed, tree2.parsed, schema, diff)
	maskDiffSecrets(diff, config.SecretKeys(schema))
	return diff
}

// resolveRollbackVersion resolves a revision number to its history entry key
// and its bytes, read through the entry's object with the hash verified.
func resolveRollbackVersion(store storage.Storage, configPath string, n int) (string, []byte, error) {
	backups, err := store.ListVersions(configPath)
	if err != nil {
		return "", nil, err
	}
	if n < 1 || n > len(backups) {
		return "", nil, fmt.Errorf("revision %d not found (have %d revisions)", n, len(backups))
	}
	data, err := store.ReadVersion(configPath, backups[n-1].Stamp)
	if err != nil {
		return "", nil, err
	}
	return backups[n-1].Path, data, nil
}

// diffSource keeps parsed order beside resolved BGP values: lowering to a map
// alone cannot answer whether a schema-declared ordered list moved.
type diffSource struct {
	parsed   *config.Tree
	resolved map[string]any
}

// readDiffSource reads one input exactly once, before selecting the schema.
// A nil store means an explicit path. Stdin remains a stream even when the
// other source comes from storage.
func readDiffSource(store storage.Storage, path string) ([]byte, error) {
	if store == nil {
		return cliio.ReadFile(path)
	}
	if cliio.IsStdin(path) {
		return cliio.ReadFile(path)
	}
	return store.ReadFile(path)
}

// parseAndResolve retains the parsed tree for schema-driven order comparison.
func parseAndResolve(schema *config.Schema, data []byte) (*diffSource, error) {
	p := config.NewParser(schema)
	tree, err := p.Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	source := &diffSource{parsed: tree}
	// Resolving an absent BGP block asks the engine to validate configuration
	// that is not there. Non-BGP inputs still participate in list-order diff.
	if tree.GetContainer("bgp") != nil {
		source.resolved, err = infra.ResolveBGPTree(tree)
		if err != nil {
			return nil, fmt.Errorf("resolve: %w", err)
		}
	}
	return source, nil
}

// diffListOrders adds only schema-declared ordered-list sequences, leaving the
// existing resolved BGP value paths and non-BGP value-comparison scope intact.
func diffListOrders(before, after *config.Tree, schema *config.Schema, diff *config.ConfigDiff) {
	oldOrders, newOrders := make(map[string][]string), make(map[string][]string)
	secretPaths := make(map[string]bool)
	collectDiffListOrders(before, schema, "", oldOrders, secretPaths)
	collectDiffListOrders(after, schema, "", newOrders, secretPaths)
	for path, old := range oldOrders {
		if _, present := newOrders[path]; !present {
			diff.Removed[path] = maskDiffOrder(old, secretPaths[path])
		}
	}
	for path, keys := range newOrders {
		old, present := oldOrders[path]
		if !present {
			diff.Added[path] = maskDiffOrder(keys, secretPaths[path])
			continue
		}
		// Compare real keys first; masking before comparison would hide a move.
		if !slices.Equal(old, keys) {
			diff.Changed[path] = config.DiffPair{
				Old: maskDiffOrder(old, secretPaths[path]),
				New: maskDiffOrder(keys, secretPaths[path]),
			}
		}
	}
}

// collectDiffListOrders walks the finite parsed tree, including containers and
// entries of plain lists: an ordered list can be nested inside either.
func collectDiffListOrders(tree *config.Tree, schema config.ChildProvider, prefix string, orders map[string][]string, secrets map[string]bool) {
	for _, name := range schema.Children() {
		node := schema.Get(name)
		path := config.AppendPath(prefix, name)
		entries := tree.GetList(name)
		if list, ok := node.(*config.ListNode); ok && list.UserOrdered && len(entries) > 0 {
			recorded := tree.GetListOrdered(name)
			keys := make([]string, 0, len(recorded))
			for _, entry := range recorded {
				keys = append(keys, entry.Key)
			}
			orderPath := config.AppendPath(prefix, configorder.OrderKey(name))
			orders[orderPath] = config.ListEntryKeys(entries, keys)
			if list.KeyLeaf != nil && (list.KeyLeaf.Sensitive || list.KeyLeaf.Bcrypt) {
				secrets[orderPath] = true
			}
		}
		children, ok := node.(config.ChildProvider)
		if !ok {
			continue
		}
		if sub := tree.GetContainer(name); sub != nil {
			collectDiffListOrders(sub, children, path, orders, secrets)
		}
		for key, entry := range entries {
			collectDiffListOrders(entry, children, config.AppendPath(path, key), orders, secrets)
		}
	}
}

func maskDiffOrder(keys []string, secret bool) []string {
	if !secret {
		return keys
	}
	for i := range keys {
		keys[i] = config.SecretDataPlaceholder
	}
	return keys
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
		slices.Sort(keys)
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
	slices.Sort(keys)
	return keys
}
