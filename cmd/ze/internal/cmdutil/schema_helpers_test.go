package cmdutil

import (
	"testing"

	cli "github.com/ze-software/ze/internal/component/cli/client"
	"github.com/ze-software/ze/internal/component/command"
)

// The helpers below call the resolution functions whose error is the YANG
// loader's refusal of the schema, and fail the test on it. Every test in this
// package runs over the shipped schema, so an error there is a broken tree,
// never an answer a test may read as "not declared".

func resolveForTest(t testing.TB, args []string, verb string) (Resolution, bool) {
	t.Helper()
	res, ok, err := ResolveCommand(args, verb)
	if err != nil {
		t.Fatalf("ResolveCommand(%q, %q): %v", args, verb, err)
	}
	return res, ok
}

func verbTreeForTest(t testing.TB, verb string) *cli.Command {
	t.Helper()
	tree, err := cli.BuildVerbCommandTree(verb)
	if err != nil {
		t.Fatalf("BuildVerbCommandTree(%q): %v", verb, err)
	}
	return tree
}

func yangTreeForTest(t testing.TB) *cli.Command {
	t.Helper()
	tree, err := cli.YANGCommandTree()
	if err != nil {
		t.Fatalf("YANGCommandTree: %v", err)
	}
	return tree
}

func wireToPathsForTest(t testing.TB) map[string][]string {
	t.Helper()
	paths, err := cli.WireToPaths()
	if err != nil {
		t.Fatalf("WireToPaths: %v", err)
	}
	return paths
}

func absoluteVerbPathForTest(t testing.TB, verb string, rel []string) ([]string, bool) {
	t.Helper()
	words, declared, err := cli.AbsoluteVerbPath(verb, rel)
	if err != nil {
		t.Fatalf("AbsoluteVerbPath(%q, %q): %v", verb, rel, err)
	}
	return words, declared
}

func matchLocalForTest(t testing.TB, words, values []string) (command.LocalHandler, []string) {
	t.Helper()
	handler, args, err := matchLocalHandler(words, values)
	if err != nil {
		t.Fatalf("matchLocalHandler(%q, %q): %v", words, values, err)
	}
	return handler, args
}

func lookupLocalForTest(t testing.TB, words []string) (command.LocalHandler, []string) {
	t.Helper()
	handler, args, err := command.LookupLocal(words, cli.IsDeclaredCommand)
	if err != nil {
		t.Fatalf("LookupLocal(%q): %v", words, err)
	}
	return handler, args
}
