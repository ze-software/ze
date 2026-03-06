// Design: docs/architecture/api/commands.md — shared CLI command utilities
// Related: ../../run/main.go — ze run consumer
// Related: ../../show/main.go — ze show consumer
// Related: ../../cli/main.go — CLI client and BuildCommandTree
//
// Package cmdutil provides shared logic for ze show and ze run subcommands.
// Both commands discover commands dynamically from RPC registrations and delegate
// execution to cli.Run. This package holds the common tree walking, validation,
// flag extraction, and help formatting used by both.
package cmdutil

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/cli"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
)

// RunCommand extracts --socket flag, validates command words against the tree,
// and delegates to cli.Run with --run. The readOnly flag controls whether only
// read-only commands are accepted. The cmdName is used in error/hint messages.
func RunCommand(args []string, readOnly bool, cmdName string) int {
	var socketPath string
	var cmdWords []string

	for i := 0; i < len(args); i++ {
		if args[i] == "--socket" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --socket requires a path argument\n")
				return 1
			}
			socketPath = args[i+1]
			i++ // skip value
		} else {
			cmdWords = append(cmdWords, args[i])
		}
	}

	if len(cmdWords) == 0 {
		return -1 // signal caller to show usage
	}

	tree := cli.BuildCommandTree(readOnly)

	if !IsValidCommand(cmdWords, tree) {
		fmt.Fprintf(os.Stderr, "error: unknown command: %s\n", strings.Join(cmdWords, " "))
		if suggestion := SuggestFromTree(cmdWords[0], tree); suggestion != "" {
			fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", suggestion)
		}
		fmt.Fprintf(os.Stderr, "hint: run 'ze %s help' for available commands\n", cmdName)
		return 1
	}

	var cliArgs []string
	if socketPath != "" {
		cliArgs = append(cliArgs, "--socket", socketPath)
	}
	cliArgs = append(cliArgs, "--run", strings.Join(cmdWords, " "))

	return cli.Run(cliArgs)
}

// IsValidCommand checks if the command words match a path in the given tree.
func IsValidCommand(words []string, tree *cli.Command) bool {
	if len(words) == 0 {
		return false
	}
	current := tree

	for _, word := range words {
		if current.Children == nil {
			return false
		}
		child, ok := current.Children[word]
		if !ok {
			return false
		}
		current = child
	}

	return current.Description != "" || len(current.Children) > 0
}

// SuggestFromTree returns a "did you mean?" suggestion for the first command word.
func SuggestFromTree(word string, tree *cli.Command) string {
	if tree.Children == nil {
		return ""
	}
	candidates := make([]string, 0, len(tree.Children))
	for k := range tree.Children {
		candidates = append(candidates, k)
	}
	return suggest.Command(word, candidates)
}

// CommandEntry holds a top-level command name and description for help display.
type CommandEntry struct {
	Name string
	Desc string
}

// CommandList returns sorted top-level commands from the tree.
func CommandList(tree *cli.Command) []CommandEntry {
	if tree.Children == nil {
		return nil
	}

	keys := make([]string, 0, len(tree.Children))
	for k := range tree.Children {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	entries := make([]CommandEntry, 0, len(keys))
	for _, name := range keys {
		child := tree.Children[name]
		entries = append(entries, CommandEntry{
			Name: name,
			Desc: DescribeCommand(child),
		})
	}
	return entries
}

// DescribeCommand returns a description for a command node.
// Uses the node's own description if it's a leaf, or summarizes children.
func DescribeCommand(cmd *cli.Command) string {
	if cmd.Description != "" {
		return cmd.Description
	}
	if len(cmd.Children) == 0 {
		return ""
	}
	subs := make([]string, 0, len(cmd.Children))
	for k := range cmd.Children {
		subs = append(subs, k)
	}
	sort.Strings(subs)
	return "subcommands: " + strings.Join(subs, ", ")
}

// PrintCommandList writes the formatted command list to stderr.
func PrintCommandList(tree *cli.Command) {
	entries := CommandList(tree)
	for _, e := range entries {
		fmt.Fprintf(os.Stderr, "  %-16s %s\n", e.Name, e.Desc)
	}
}
