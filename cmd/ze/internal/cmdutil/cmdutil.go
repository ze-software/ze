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

// RunCommand extracts flags, validates command words against the tree,
// and delegates to cli.Run with --run. The readOnly flag controls whether only
// read-only commands are accepted. The cmdName is used in error/hint messages.
func RunCommand(args []string, readOnly bool, cmdName string) int {
	var username string
	var cmdWords []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--user":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --user requires a username argument\n")
				return 1
			}
			username = args[i+1]
			i++ // skip value
		default:
			cmdWords = append(cmdWords, args[i])
		}
	}

	if len(cmdWords) == 0 {
		return -1 // signal caller to show usage
	}

	// Extract output format keyword (yaml/json/table) from end of command.
	cmdWords, format := ExtractOutputFormat(cmdWords)

	tree := cli.BuildCommandTree(readOnly)

	// Extract peer selector (IP/glob) from command words.
	// User types "peer 127.0.0.2 show" but the tree has peer → show.
	// The selector is passed to the CLI client as a trailing argument.
	treeWords, selector := ExtractSelector(cmdWords, tree)

	if !IsValidCommand(treeWords, tree) {
		fmt.Fprintf(os.Stderr, "error: unknown command: %s\n", strings.Join(cmdWords, " "))
		if suggestion := SuggestFromTree(cmdWords[0], tree); suggestion != "" {
			fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", suggestion)
		}
		fmt.Fprintf(os.Stderr, "hint: run 'ze %s help' for available commands\n", cmdName)
		return 1
	}

	// Group command (has children but no handler) — show subcommands.
	if node := FindNode(treeWords, tree); node != nil && node.Description == "" && len(node.Children) > 0 {
		fmt.Fprintf(os.Stderr, "%s subcommands:\n", strings.Join(treeWords, " "))
		PrintChildren(node)
		return 0
	}

	// Build the run command: tree words + selector as trailing arg.
	// CLI client's resolveCommand("peer detail 127.0.0.2") matches "bgp peer detail"
	// and passes "127.0.0.2" as args to the handler.
	runCmd := strings.Join(treeWords, " ")
	if selector != "" {
		runCmd += " " + selector
	}

	var cliArgs []string
	if username != "" {
		cliArgs = append(cliArgs, "--user", username)
	}
	if format != "" {
		cliArgs = append(cliArgs, "--format", format)
	}
	cliArgs = append(cliArgs, "--run", runCmd)

	return cli.Run(cliArgs)
}

// ExtractOutputFormat removes a trailing format keyword (yaml/json/table) from command words.
func ExtractOutputFormat(words []string) ([]string, string) {
	if len(words) == 0 {
		return words, ""
	}
	last := words[len(words)-1]
	switch last {
	case "yaml", "json", "table":
		return words[:len(words)-1], last
	}
	return words, ""
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

// ExtractSelector detects and removes a peer selector (IP/glob) from command words.
// Pattern: "peer 127.0.0.2 show" → treeWords=["peer","show"], selector="127.0.0.2".
// Only extracts words that look like IP addresses or glob patterns (contain dots or *).
func ExtractSelector(words []string, tree *cli.Command) (treeWords []string, selector string) {
	if len(words) < 2 {
		return words, ""
	}

	current := tree
	for i, word := range words {
		if current.Children == nil {
			return words, ""
		}
		if _, ok := current.Children[word]; ok {
			current = current.Children[word]
			continue
		}
		// Word doesn't match tree — only treat as selector if it looks like an IP or glob.
		if !looksLikeSelector(word) {
			return words, ""
		}
		treeWords = make([]string, 0, len(words)-1)
		treeWords = append(treeWords, words[:i]...)
		treeWords = append(treeWords, words[i+1:]...)
		return treeWords, word
	}
	return words, ""
}

// looksLikeSelector returns true if the word looks like an IP address or glob pattern.
// Matches: "127.0.0.1", "192.168.*.*", "10.0.0.0/24", "*", "::1", "2001:db8::1".
func looksLikeSelector(s string) bool {
	if s == "*" {
		return true
	}
	// Contains dot (IPv4) or colon (IPv6)
	return strings.ContainsAny(s, ".:")
}

// FindNode returns the command node at the given path, or nil if not found.
func FindNode(words []string, tree *cli.Command) *cli.Command {
	current := tree
	for _, word := range words {
		if current.Children == nil {
			return nil
		}
		child, ok := current.Children[word]
		if !ok {
			return nil
		}
		current = child
	}
	return current
}

// PrintChildren prints the children of a command node to stderr.
func PrintChildren(node *cli.Command) {
	entries := CommandList(node)
	for _, e := range entries {
		fmt.Fprintf(os.Stderr, "  %-20s %s\n", e.Name, e.Desc)
	}
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
