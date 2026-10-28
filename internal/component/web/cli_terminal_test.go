// Design: docs/architecture/config/syntax.md -- the printer's list entry order

package web

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

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

	_, output := executeTerminalNav(schema, tree, newTestEditorManager(t), "alice", nil,
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
