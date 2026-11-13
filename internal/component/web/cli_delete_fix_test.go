// Design: docs/guide/config-editor.md -- a refused candidate remains editable.

package web

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestWebPrintedDeleteFixKeepsAcceptedMembers replays the commit's actual
// deletion command on both web command surfaces, at the root and in a context.
func TestWebPrintedDeleteFixKeepsAcceptedMembers(t *testing.T) {
	const module = `module ze-web-member-repair-conf {
  yang-version 1.1;
  namespace "urn:ze:web-member-repair";
  prefix probe;
  import ze-extensions { prefix ze; }
  container member-repair {
    leaf sentinel { type string; }
    leaf-list plain { type enumeration { enum blue; enum red; } }
    leaf-list bracket {
      type enumeration { enum blue; enum red; }
      ze:syntax "bracket";
    }
  }
}`
	t.Cleanup(configyang.RegisterModuleForTest("ze-web-member-repair-conf.yang", module))
	installPeerValidator(t, func(*config.Tree) error { return nil })
	for _, memberKind := range []string{"plain", "bracket"} {
		for _, position := range []string{"", "bgp"} {
			for _, surface := range []string{"terminal", "bar"} {
				t.Run(memberKind+"/"+position+"/"+surface, func(t *testing.T) {
					mgr, _ := newCommitTestManager(t)
					terminal := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, mgr.schema, nil, nil, nil, nil)
					us, err := mgr.GetOrCreate("testuser")
					require.NoError(t, err)
					path := []string{"member-repair"}
					require.NoError(t, us.editor.SetValue(path, "sentinel", "keep-me"))
					require.NoError(t, us.editor.SetValue(path, memberKind, "blue"))
					badValue := "bogus"
					if memberKind == "bracket" {
						badValue = "blue bogus"
					}
					require.NoError(t, us.editor.SetValue(path, memberKind, badValue))
					refused := webTerminalLine(t, terminal, position, "commit now")
					var fix string
					for _, line := range strings.Split(refused.Output, "\n") {
						if strings.HasPrefix(strings.TrimSpace(line), "fix: ") {
							fix = strings.TrimPrefix(strings.TrimSpace(line), "fix: ")
						}
					}
					prefix := "delete "
					if position != "" {
						prefix = "top delete "
					}
					require.Equal(t, prefix+"member-repair "+memberKind+" bogus", fix, refused.Output)
					if surface == "terminal" {
						replayed := webTerminalLine(t, terminal, position, fix)
						require.NotContains(t, replayed.Output, "error:")
						assert.Equal(t, "deleted member-repair "+memberKind+" bogus", replayed.Feedback)
						assert.Equal(t, position, replayed.Path)
					} else {
						renderer, err := NewRenderer()
						require.NoError(t, err)
						bar := HandleCLICommandWithAuthorizerAndAudit(mgr, mgr.schema, renderer, nil, nil)
						require.NotContains(t, webBarLine(t, bar, position, fix), "delete error:")
					}
					probe := mgr.Tree("testuser").GetContainer("member-repair")
					require.NotNil(t, probe, "printed fix removed the containing subtree")
					assert.Equal(t, []string{"blue"}, probe.GetMultiValues(memberKind))
					sentinel, ok := probe.Get("sentinel")
					assert.True(t, ok)
					assert.Equal(t, "keep-me", sentinel)
					committed := webTerminalLine(t, terminal, position, "commit now")
					assert.NotContains(t, committed.Output, "blocked")
					assert.NotContains(t, committed.Output, "error:")
					content, err := mgr.committedConfig()
					require.NoError(t, err)
					// The editor persists the original set or hierarchical
					// format; use the same reader as the web's saved views.
					tree, err := parseConfigContent(string(content), mgr.schema)
					require.NoError(t, err)
					stored := tree.GetContainer("member-repair")
					require.NotNil(t, stored)
					assert.Equal(t, []string{"blue"}, stored.GetMultiValues(memberKind))
					storedSentinel, ok := stored.Get("sentinel")
					assert.True(t, ok)
					assert.Equal(t, "keep-me", storedSentinel)
				})
			}
		}
	}
}
