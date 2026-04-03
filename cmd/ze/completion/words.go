// Design: (none -- new feature, shell completion generation)
// Overview: main.go -- completion dispatch
// Related: bash.go -- bash completion uses words at tab time
// Related: zsh.go -- zsh completion uses words at tab time
// Related: fish.go -- fish completion uses words at tab time
// Related: nushell.go -- nushell completion uses words at tab time
// Related: peers.go -- dynamic peer selector completion from running daemon

package completion

import (
	"fmt"
	"io"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/command"
)

// words outputs tab-separated "word\tdescription" pairs for shell completion.
// Called by shell completion scripts at tab time to get contextual completions.
// Delegates to command.TreeCompleter so CLI interactive and shell completion
// share the same walker and ValueHints.
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
	tc := command.NewTreeCompleter(tree)

	// Build input string from path args. Trailing space signals "list all
	// children" (no prefix filter) -- the shell handles its own filtering.
	input := strings.Join(args[1:], " ")
	if len(args) > 1 {
		input += " "
	}

	suggestions := tc.Complete(input)
	for _, s := range suggestions {
		// Skip pipe operators -- not relevant for shell completion.
		if s.Type == "pipe" {
			continue
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\n", s.Text, s.Description); err != nil {
			return 1
		}
	}
	return 0
}
