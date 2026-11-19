// Design: docs/architecture/config/syntax.md -- the printer's list entry order

package web

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
)

// TestTerminalAuthCommandReadsSharedMapping proves AC-5 for the web terminal:
// every verb of the terminal's vocabulary is authorized with the permission the
// editor's own table (cli.ConfigPermission) declares, plain and behind `top`,
// so the web terminal and the SSH session editor cannot judge a verb two ways.
// It also pins the strings the terminal sent before the table moved, byte for
// byte, because operator profiles and TACACS+ servers match them.
//
// Method: run terminalAuthCommand over each web verb constant with an
// argument, alone and under `top`, and compare with the CLI table; then compare
// each mutating verb with the literal string it was authorized with.
func TestTerminalAuthCommandReadsSharedMapping(t *testing.T) {
	vocabulary := []string{
		verbClear, verbEdit, verbSet, verbDelete, verbShow, verbTop, verbUp,
		verbCommit, verbDiscard, verbHelp, verbWho, verbCompare, verbSave,
		verbHistory, verbRollback, verbRename, verbCopy, verbInsert,
		verbDeactivate, verbActivate, verbErrors, verbDisconnect, verbRun,
		verbConfigure, verbExit, verbQuit, verbLoad, verbOption,
	}
	for _, verb := range vocabulary {
		permission, gated := cli.ConfigPermission(verb)
		want := ""
		if gated {
			want = permission
		}
		line := verb + " probe"
		assert.Equal(t, want, terminalAuthCommand(parseCLICommand(line)), "verb %q", verb)
		if verb == verbTop {
			continue
		}
		assert.Equal(t, want, terminalAuthCommand(parseCLICommand("top "+line)), "top %q", verb)
	}

	before := map[string]string{
		verbSet:        "config set",
		verbActivate:   "config set",
		verbDelete:     "config delete",
		verbDeactivate: "config delete",
		verbCommit:     "config commit",
		verbDiscard:    "config discard",
		verbDisconnect: "config discard",
		verbRollback:   "config rollback",
		verbRename:     "config rename",
		verbCopy:       "config add",
		verbInsert:     "config add",
		verbSave:       "config save",
		verbLoad:       "config save",
	}
	for verb, want := range before {
		assert.Equal(t, want, terminalAuthCommand(parseCLICommand(verb+" probe")), "verb %q", verb)
	}
	for _, verb := range []string{verbEdit, verbUp, verbTop, verbShow, verbWho, verbOption, verbHelp} {
		assert.Empty(t, terminalAuthCommand(parseCLICommand(verb)), "navigation verb %q is gated", verb)
	}
}

// TestWebCLIShowKeepsOperatorOrder proves AC-13 for the web CLI terminal:
// `show` prints the peers in the order the operator wrote them (zulu, alpha,
// mike), not sorted by key. The method parses a config holding the three
// peers in that order, runs the terminal's `show` verb over it, and compares
// where each peer prints.
func TestWebCLIShowKeepsOperatorOrder(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	peer := func(name, ip string) string {
		return "\tpeer " + name + " {\n\t\tconnection {\n\t\t\tremote {\n\t\t\t\tip " + ip + "\n\t\t\t}\n\t\t}\n\t}\n"
	}
	source := "bgp {\n" + peer("zulu", "192.0.2.26") + peer("alpha", "192.0.2.1") + peer("mike", "192.0.2.13") + "}\n"
	tree, err := config.NewParser(schema).Parse(source)
	require.NoError(t, err)

	mgr, _ := newPromotingEditorManagerOver(t, source)
	output := executeTerminalNav(schema, tree, mgr, "alice", nil,
		cliCommand{Verb: verbShow})

	names := []string{"peer zulu", "peer alpha", "peer mike"}
	positions := make([]int, len(names))
	for i, name := range names {
		positions[i] = strings.Index(output, name)
		require.GreaterOrEqual(t, positions[i], 0, "show lacks %s:\n%s", name, output)
	}
	for i := 1; i < len(names); i++ {
		if positions[i-1] > positions[i] {
			t.Errorf("show prints %s before %s, want the order written (zulu, alpha, mike):\n%s",
				names[i], names[i-1], output)
		}
	}
}
