package help_test

import (
	"slices"
	"strings"
	"testing"

	cli "github.com/ze-software/ze/internal/component/cli/client"
	_ "github.com/ze-software/ze/internal/component/config/yang/cli" // the show yang commands cmd/ze links by hand
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	_ "github.com/ze-software/ze/internal/plugins/completion" // hand-wired in cmd/ze, as help is
	"github.com/ze-software/ze/internal/plugins/help"
)

// The helpers below call the functions whose error is the YANG loader's
// refusal of the schema, and fail the test on it. Every test in this package
// runs over the shipped schema, so an error there is a broken tree, never an
// answer a test may read as empty.

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

func collectCommandsForTest(t testing.TB) []help.CommandEntry {
	t.Helper()
	entries, err := help.CollectCommands()
	if err != nil {
		t.Fatalf("collectCommands: %v", err)
	}
	return entries
}

// namesOperator answers whether the text names the operator as a WORD. A
// substring test is not sound here: "json" sits inside "ndjson", so a page
// naming only ndjson would report json as published too. It is the helper of
// the same name in cmd/ze/pipe_surfaces_test.go, which the pipe tests there
// still use: a test package cannot import another's test helpers.
func namesOperator(text, name string) bool {
	return slices.Contains(strings.FieldsFunc(text, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-'
	}), name)
}
