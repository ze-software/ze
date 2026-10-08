// Design: (none -- new feature, shell completion generation)
// Overview: main.go -- completion dispatch
// Related: bash.go -- bash completion uses words at tab time
// Related: zsh.go -- zsh completion uses words at tab time
// Related: fish.go -- fish completion uses words at tab time
// Related: nushell.go -- nushell completion uses words at tab time
// Related: peers.go -- dynamic peer selector completion from running daemon

package completion

import (
	"io"
	"os"
	"strings"

	cli "github.com/ze-software/ze/internal/component/cli/client"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// words outputs tab-separated "word\tdescription" pairs for shell completion.
// Called by shell completion scripts at tab time to get contextual completions.
// Delegates to command.TreeCompleter so CLI interactive and shell completion
// share the same walker and ValueHints.
//
// Usage:
//
//	ze completion words show [path...]       — read-only command tree under `ze show`
//	ze completion words run [command path...] — daemon command tree used by `ze run`
func words(args []string) int {
	return writeWords(os.Stdout, args)
}

// writeWords writes completion pairs to w. Separated for testability.
func writeWords(w io.Writer, args []string) int {
	if len(args) == 0 {
		return 0
	}

	tree, path, err := completionTree(args)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // one-shot error to stderr
		return 1
	}
	if tree == nil {
		return 0
	}

	tc := command.NewTreeCompleter(tree)

	// Build input string from path args. Trailing space signals "list all
	// children" (no prefix filter) -- the shell handles its own filtering.
	var tb textbuf.Buffer
	input := tb.Join(path, " ").String()
	if len(path) > 0 {
		input = tb.Reset().Str(input).Byte(' ').String()
	}

	suggestions := tc.Complete(input)
	for _, s := range suggestions {
		// Skip pipe operators -- not relevant for shell completion.
		if s.Type == "pipe" {
			continue
		}
		if err := writeCompletionRecord(w, s.Text, s.Description); err != nil {
			return 1
		}
	}
	return 0
}

// writeCompletionRecord writes one shell-completion record. The record is the
// candidate, a tab, its summary, a newline. Every shell generator reads that
// shape. A tab or a newline inside the summary makes the shell read one
// candidate as two, or read half the summary as a third field.
//
// A summary carrying either one is FOLDED to single spaces, never cut. Folding
// loses no word and decides nothing. It answers the format's one-line
// constraint, the same answer the TUI completion pane gives the same problem
// (internal/component/cli/model_render.go). The cut that stood here until this
// spec published the first line as if the author had chosen it as the summary.
// It dropped the rest and logged nothing. A summary is declared one line long,
// so on a converted node this folds nothing.
func writeCompletionRecord(w io.Writer, candidate, summary string) error {
	if strings.ContainsAny(summary, "\n\r\t") {
		summary = textbuf.Join(strings.Fields(summary), " ")
	}
	var record textbuf.Buffer
	record.Str(candidate).Byte('\t').Str(summary).Byte('\n')
	_, err := io.WriteString(w, record.String())
	return err
}

// nameRIB is the BGP RIB that the "rib" shorthand expands to. The verb this
// file names more than once is command.VerbShow, the canonical spelling.
const nameRIB = "rib"

// completionTree answers the tree args complete against and the path left to
// walk in it. A nil tree with no error means args name nothing completable.
// The error is the YANG loader's refusal of the schema.
func completionTree(args []string) (*command.Node, []string, error) {
	if len(args) == 0 {
		return nil, nil, nil
	}
	if args[0] == "run" {
		return runCompletionTree(args[1:])
	}
	if args[0] == command.VerbShow {
		tree, err := cli.BuildVerbCommandTree(command.VerbShow)
		return tree, args[1:], err
	}
	tree, found, err := rootCommandTree(args[0])
	if err != nil {
		return nil, nil, err
	}
	if found {
		return tree, args[1:], nil
	}
	return nil, nil, nil
}

// rootCommandTree builds the tree of the registered root command name; found
// is false when none is registered. The error is the YANG loader's refusal of
// the schema, met while merging the `show` descriptions.
func rootCommandTree(name string) (*command.Node, bool, error) {
	for _, cmd := range registry.ListRoot() {
		if cmd.Name != name {
			continue
		}
		root := &command.Node{Name: name, ShortHelp: cmd.Meta.ShortHelp}
		subs := cmd.Meta.ResolveSubs()
		if subs != "" {
			root.Children = make(map[string]*command.Node)
			for sub := range strings.SplitSeq(subs, ",") {
				sub = strings.TrimSpace(sub)
				if sub == "" {
					continue
				}
				cmdName, hint, _ := strings.Cut(sub, " ")
				if cmdName[0] == '-' || cmdName[0] == '[' {
					continue
				}
				root.Children[cmdName] = &command.Node{Name: cmdName, ShortHelp: hint}
			}
		}
		if err := mergeShowDescriptions(name, root); err != nil {
			return nil, false, err
		}
		if name == "env" {
			wireEnvKeyHints(root)
		}
		return root, true, nil
	}
	return nil, false, nil
}

func mergeShowDescriptions(name string, root *command.Node) error {
	if root.Children == nil {
		return nil
	}
	showTree, err := cli.BuildVerbCommandTree(command.VerbShow)
	if err != nil {
		return err
	}
	src := showTree.Children[name]
	if src == nil || src.Children == nil {
		return nil
	}
	for childName, child := range root.Children {
		if child.ShortHelp == "" {
			if srcChild, ok := src.Children[childName]; ok && srcChild.ShortHelp != "" {
				child.ShortHelp = srcChild.ShortHelp
			}
		}
	}
	return nil
}

func wireEnvKeyHints(root *command.Node) {
	if root.Children == nil {
		return
	}
	for _, leaf := range []string{"get", "registered"} {
		if node, ok := root.Children[leaf]; ok {
			node.ValueHints = command.EnvValueHints
		}
	}
}

// runCompletionTree answers the daemon command tree `ze run` completes
// against and the path left to walk in it. The error is the YANG loader's
// refusal of the schema.
func runCompletionTree(path []string) (*command.Node, []string, error) {
	if len(path) == 0 {
		tree, err := cli.BuildCommandTree(false)
		return tree, nil, err
	}
	if path[0] == nameRIB {
		tree, err := cli.BuildVerbCommandTree(command.VerbShow)
		if err != nil {
			return nil, nil, err
		}
		addRIBRoutesAlias(tree)
		return tree, append([]string{"bgp", nameRIB}, path[1:]...), nil
	}
	// A canonical verb roots a tree of its own, so completion walks that tree
	// relative to the verb. The set is command.Verbs rather than a list written
	// here, so a verb added to the vocabulary completes without a second edit.
	if command.IsVerb(path[0]) {
		tree, err := cli.BuildVerbCommandTree(path[0])
		return tree, path[1:], err
	}
	tree, err := cli.BuildCommandTree(false)
	return tree, path, err
}

func addRIBRoutesAlias(tree *command.Node) {
	if tree == nil {
		return
	}
	current := tree
	for _, name := range []string{"bgp", nameRIB} {
		if current.Children == nil {
			return
		}
		child := current.Children[name]
		if child == nil {
			return
		}
		current = child
	}
	if current.Children == nil {
		current.Children = make(map[string]*command.Node, 1)
	}
	if current.Children["routes"] == nil {
		current.Children["routes"] = &command.Node{
			Name:      "routes",
			ShortHelp: "Query routes in the BGP RIB",
		}
	}
}
