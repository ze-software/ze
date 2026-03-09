// Design: (none -- new feature, shell completion generation)
// Overview: main.go -- completion dispatch
// Related: bash.go -- bash completion uses words at tab time
// Related: zsh.go -- zsh completion uses words at tab time
// Related: fish.go -- fish completion uses words at tab time

package completion

import (
	"fmt"
	"io"
	"os"
	"sort"

	"codeberg.org/thomas-mangin/ze/cmd/ze/cli"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdutil"
)

// words outputs tab-separated "word\tdescription" pairs for shell completion.
// Called by shell completion scripts at tab time to get contextual completions.
//
// Usage:
//
//	ze completion words show [path...]   — read-only command tree
//	ze completion words run [path...]    — full command tree
func words(args []string) int {
	return writeWords(os.Stdout, args)
}

// writeWords writes completion pairs to w. Separated for testability.
func writeWords(w io.Writer, args []string) int {
	if len(args) == 0 {
		return 0
	}

	var readOnly bool
	switch args[0] {
	case "show":
		readOnly = true
	case "run":
		readOnly = false
	default:
		return 0
	}

	tree := cli.BuildCommandTree(readOnly)

	// Walk to the target node.
	current := tree
	for _, word := range args[1:] {
		if current.Children == nil {
			return 0
		}
		child, ok := current.Children[word]
		if !ok {
			return 0
		}
		current = child
	}

	if current.Children == nil {
		return 0
	}

	keys := make([]string, 0, len(current.Children))
	for k := range current.Children {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, name := range keys {
		child := current.Children[name]
		desc := cmdutil.DescribeCommand(child)
		if _, err := fmt.Fprintf(w, "%s\t%s\n", name, desc); err != nil {
			return 1
		}
	}
	return 0
}
