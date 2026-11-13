// Design: docs/guide/web-interface.md -- activation refusals keep the command's position.

package web

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

const webActivationProbe = `module ze-web-activation-probe-conf {
  yang-version 1.1;
  namespace "urn:ze:web-activation-probe";
  prefix probe;
  container activation-probe {
    list lane {
      key "kind";
      leaf kind { type enumeration { enum blue; enum red; } }
      leaf-list member { type string; ordered-by user; }
    }
  }
}`

// TestWebTerminalActivationFramesKeyRefusal proves that an absent list entry
// cannot be created under a refused enum key by activating one of its members.
// VALIDATES: AC-31 on the web -- the complete activation command is the fix,
// behind top when typed there, and no refused entry or inactive marker is saved.
func TestWebTerminalActivationFramesKeyRefusal(t *testing.T) {
	t.Cleanup(configyang.RegisterModuleForTest("ze-web-activation-probe-conf.yang", webActivationProbe))
	for _, verb := range []string{"activate", "deactivate"} {
		for _, behindTop := range []bool{false, true} {
			t.Run(verb+"/top="+strconv.FormatBool(behindTop), func(t *testing.T) {
				mgr, terminal := newYANGTerminal(t)
				_, err := mgr.GetOrCreate("testuser")
				require.NoError(t, err)
				before := config.Serialize(mgr.Tree("testuser"), mgr.schema)
				line := verb + " activation-probe lane bogus member one"
				position := ""
				if behindTop {
					position = "bgp"
					line = "top " + line
				}
				refused := webTerminalLine(t, terminal, position, line)
				assert.Contains(t, refused.Output, `activation-probe lane bogus: "bogus" is not an accepted value`)
				assert.Equal(t, "fix: "+strings.Replace(line, "bogus", "<blue|red>", 1), lastLine(refused.Output))
				assert.Equal(t, position, refused.Path)
				assert.Equal(t, before, config.Serialize(mgr.Tree("testuser"), mgr.schema))

				set := webTerminalLine(t, terminal, "", "set activation-probe lane blue member one")
				require.False(t, strings.HasPrefix(set.Output, "error:"), "%s", set.Output)
				if verb == "activate" {
					deactivated := webTerminalLine(t, terminal, "", "deactivate activation-probe lane blue member one")
					require.False(t, strings.HasPrefix(deactivated.Output, "error:"), "%s", deactivated.Output)
				}
				accepted := webTerminalLine(t, terminal, position, strings.Replace(line, "bogus", "blue", 1))
				require.False(t, strings.HasPrefix(accepted.Output, "error:"), "%s", accepted.Output)
				assert.Equal(t, position, accepted.Path)
				entry := mgr.Tree("testuser").GetContainer("activation-probe").GetList("lane")["blue"]
				require.NotNil(t, entry)
				assert.Equal(t, []config.MemberState{{Value: "one", Inactive: verb == "deactivate"}}, entry.GetMultiValuesState("member"))
			})
		}
	}
}
