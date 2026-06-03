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

	cli "codeberg.org/thomas-mangin/ze/internal/component/cli/client"
	"codeberg.org/thomas-mangin/ze/internal/component/command"
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

	tree, path := completionTree(args)
	if tree == nil {
		return 0
	}

	tc := command.NewTreeCompleter(tree)

	// Build input string from path args. Trailing space signals "list all
	// children" (no prefix filter) -- the shell handles its own filtering.
	input := strings.Join(path, " ")
	if len(path) > 0 {
		input += " "
	}

	suggestions := tc.Complete(input)
	for _, s := range suggestions {
		// Skip pipe operators -- not relevant for shell completion.
		if s.Type == "pipe" {
			continue
		}
		// Shell completion is one row per candidate: collapse multi-line YANG
		// descriptions to their first line (the synopsis). The remaining lines
		// carry grammar/action detail that belongs in `ze yang doc`, and emitting
		// them raw would break the word\tdescription contract.
		desc := s.Description
		if before, _, ok := strings.Cut(desc, "\n"); ok {
			desc = before
		}
		if _, err := io.WriteString(w, s.Text+"\t"+desc+"\n"); err != nil {
			return 1
		}
	}
	return 0
}

func completionTree(args []string) (*command.Node, []string) {
	if len(args) == 0 {
		return nil, nil
	}
	if args[0] == "run" {
		return runCompletionTree(args[1:])
	}
	if args[0] == "show" {
		return cli.BuildVerbCommandTree("show"), args[1:]
	}
	return nil, nil
}

func runCompletionTree(path []string) (*command.Node, []string) {
	if len(path) == 0 {
		return cli.BuildCommandTree(false), nil
	}
	switch path[0] {
	case "show", "set", "delete", "clear", "request", "monitor", "resolve", "validate":
		return cli.BuildVerbCommandTree(path[0]), path[1:]
	case "rib":
		tree := cli.BuildVerbCommandTree("show")
		addRIBRoutesAlias(tree)
		return tree, append([]string{"bgp", "rib"}, path[1:]...)
	default:
		return cli.BuildCommandTree(false), path
	}
}

func addRIBRoutesAlias(tree *command.Node) {
	if tree == nil {
		return
	}
	current := tree
	for _, name := range []string{"bgp", "rib"} {
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
			Name:        "routes",
			Description: "Query routes in the BGP RIB",
		}
	}
}
