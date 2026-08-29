//go:build ze_core

package main

import (
	"sort"
	"strings"
	"testing"

	cli "github.com/ze-software/ze/internal/component/cli/client"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
)

// rootHandledCommandPaths answers every path BELOW a verb a root command
// claims that the YANG command tree declares a ze:command at.
//
// It walks the tree and the root registry the way an operator's command
// reaches them rather than asking the dispatcher what it believes, so the two
// answers can disagree and the test can say so. The population is derived: a
// plugin that registers a root handler over a YANG subtree joins it the day it
// lands, and no list here needs editing (ai/rules/plugins.md).
func rootHandledCommandPaths(t *testing.T) [][]string {
	t.Helper()

	tree := cli.YANGCommandTree()
	if tree == nil {
		t.Fatal("the YANG command tree must be built")
	}

	var paths [][]string
	for _, root := range registry.ListRoot() {
		verb := tree.Children[root.Name]
		if verb == nil {
			continue
		}
		collectCommandPaths(verb, []string{root.Name}, &paths)
	}
	sort.Slice(paths, func(i, j int) bool {
		return strings.Join(paths[i], " ") < strings.Join(paths[j], " ")
	})
	return paths
}

// collectCommandPaths appends the path of every node at or below node that
// declares a wire method, and of nothing else. A node with no ze:command
// declares no command, so it has no invocation form to state. A path of one
// word is the verb itself, which the root command owns.
func collectCommandPaths(node *command.Node, path []string, out *[][]string) {
	if node.WireMethod != "" && len(path) > 1 {
		*out = append(*out, path)
	}
	for name, child := range node.Children {
		collectCommandPaths(child, append(append([]string{}, path...), name), out)
	}
}

// VALIDATES: `ze <verb> <subcommand> help` states the generated invocation
// form for every command the tree declares below a root-handled verb.
// PREVENTS: a root command's handler swallowing the help word, which left the
// 21 commands under `plugin` and `resolve` carrying a generated usage line no
// operator could reach.
//
// TestRootHandledCommandHelpStatesGeneratedUsage drives zeDispatch itself, so
// it fails while the root handler answers the help request.
func TestRootHandledCommandHelpStatesGeneratedUsage(t *testing.T) {
	ensureLocalCommandsRegistered()
	dispatchDefaultFlags(t)

	paths := rootHandledCommandPaths(t)
	if len(paths) < 2 {
		t.Fatalf("root-handled command paths = %v, want the population the tree declares", paths)
	}

	for _, path := range paths {
		spelling := strings.Join(path, " ")
		code := 0
		out := captureStderr(t, func() { code = zeDispatch(append(append([]string{}, path...), "help")) })
		if code != 0 {
			t.Errorf("ze %s help = exit %d, want 0", spelling, code)
		}
		if !strings.Contains(out, "  ze "+spelling) {
			t.Errorf("ze %s help states no invocation form, wrote: %s", spelling, strings.TrimSpace(out))
		}
		if strings.Contains(out, "unknown") {
			t.Errorf("ze %s help wrote an unknown-command answer: %s", spelling, strings.TrimSpace(out))
		}
	}
}

// VALIDATES: the generated line names the argument the model declares, not the
// path alone.
// PREVENTS: the operator reaching a page that carries the navigation line by
// itself, which names no argument and offers subcommands a leaf command has
// none of.
func TestRootHandledCommandHelpNamesDeclaredArgument(t *testing.T) {
	ensureLocalCommandsRegistered()
	dispatchDefaultFlags(t)

	const want = "ze resolve dns a <hostname>"
	out := captureStderr(t, func() { zeDispatch([]string{"resolve", "dns", "a", "help"}) })
	if !strings.Contains(out, want) {
		t.Errorf("ze resolve dns a help must state %q, wrote: %s", want, strings.TrimSpace(out))
	}
}

// VALIDATES: a root command keeps its own name, and keeps every path below it
// that the tree declares no command at.
// PREVENTS: the fix routing the whole verb to the tree, which would replace
// `ze resolve help` and `ze resolve dns` with a listing naming neither the
// --server flag nor the operations the handler accepts.
func TestRootCommandStillOwnsItsOwnHelp(t *testing.T) {
	ensureLocalCommandsRegistered()
	dispatchDefaultFlags(t)

	cases := map[string]struct {
		args []string
		want string
	}{
		"the verb itself":        {[]string{"resolve", "help"}, "query resolution services"},
		"a path with no command": {[]string{"resolve", "dns"}, "DNS record queries"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			out := captureStderr(t, func() { zeDispatch(tc.args) })
			if !strings.Contains(out, tc.want) {
				t.Errorf("ze %s must stay with the resolve handler and state %q, wrote: %s",
					strings.Join(tc.args, " "), tc.want, strings.TrimSpace(out))
			}
		})
	}
}
