// Design: docs/architecture/cli/error-surface.md -- printed fixes retain token boundaries.

package web

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestWebPrintedEnumFixPreservesListKey executes the terminal's actual printed
// fix, including when top runs a root command from inside a peer context.
func TestWebPrintedEnumFixPreservesListKey(t *testing.T) {
	for _, key := range []string{"edge routers", `edge"routers`, "edge[routers]", "<edge routers>", "|", "edge#3", "edge#3#4"} {
		for _, position := range []string{"", "bgp/peer/peer1"} {
			t.Run(key+"/"+position, func(t *testing.T) {
				mgr, terminal := newYANGTerminal(t)
				words := []string{"set", "bgp", "group", key, "session", "capability", "route-refresh", "requre"}
				if position != "" {
					words = append([]string{"top"}, words...)
				}
				keyAt := 3
				if position != "" {
					keyAt++
				}
				line := config.FormatEditPath(words[:keyAt]) + " " + strconv.Quote(key) + " " + config.FormatEditPath(words[keyAt+1:])
				refused := webTerminalLine(t, terminal, position, line)
				require.Contains(t, refused.Output, `"requre" is not an accepted value`)
				fix := strings.TrimPrefix(lastLine(refused.Output), "fix: ")
				accepted := webTerminalLine(t, terminal, position, fix)
				require.False(t, strings.HasPrefix(accepted.Output, "error:"), "printed fix %q: %s", fix, accepted.Output)
				assert.Equal(t, position, accepted.Path)
				group := mgr.Tree("testuser").GetContainer("bgp").GetList("group")[key]
				require.NotNil(t, group)
				value, ok := group.GetContainer("session").GetContainer("capability").Get("route-refresh")
				assert.True(t, ok)
				assert.Equal(t, "require", value)
				assert.Nil(t, mgr.Tree("testuser").GetContainer("bgp").GetList("group")["edge"])
			})
		}
	}
}
