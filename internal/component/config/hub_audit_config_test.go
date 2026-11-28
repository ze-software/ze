// VALIDATES: ExtractHubConfig returns a zero plugin.HubConfig when bus/hub is
// absent, seeds DefaultHubAuditConfig when bus/hub/server omits audit, parses
// explicit bus/hub/audit max-entries overrides including bounds, and refuses
// invalid or out-of-range values through both YANG loading and raw extraction.
// PREVENTS: Non-zero HubConfig breaking FirstPass with no bus/hub block,
// omitted audit retention collapsing to zero, or invalid audit bounds passing
// config validation.

package config

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

func TestExtractHubConfigAudit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hub-audit.conf")

	t.Run("absent bus hub returns zero HubConfig", func(t *testing.T) {
		hub, err := ExtractHubConfig(nil)
		require.NoError(t, err)
		assert.Equal(t, plugin.HubConfig{}, hub)

		emptyTree := NewTree()
		hub, err = ExtractHubConfig(emptyTree)
		require.NoError(t, err)
		assert.Equal(t, plugin.HubConfig{}, hub)

		busOnly := NewTree()
		busOnly.SetContainer("bus", NewTree())
		hub, err = ExtractHubConfig(busOnly)
		require.NoError(t, err)
		assert.Equal(t, plugin.HubConfig{}, hub)
	})

	t.Run("server without audit block gets defaults", func(t *testing.T) {
		result, err := LoadConfig(hubLabelConfig(""), path, nil)
		require.NoError(t, err)

		hub, err := ExtractHubConfig(result.Tree)
		require.NoError(t, err)
		assert.Equal(t, plugin.DefaultHubAuditConfig(), hub.Audit)
	})

	t.Run("empty audit block preserves default", func(t *testing.T) {
		result, err := LoadConfig("bus { hub { audit { } } }", path, nil)
		require.NoError(t, err)

		hub, err := ExtractHubConfig(result.Tree)
		require.NoError(t, err)
		assert.Equal(t, plugin.DefaultHubAuditConfig(), hub.Audit)
	})

	t.Run("explicit max-entries overrides including boundaries", func(t *testing.T) {
		for _, want := range []int{100, 2500, 100000} {
			cfgText := `bus {
				hub {
					server central {
						client edge-01 { secret ` + labelTestSecret + `; }
					}
					audit {
						max-entries ` + strconv.Itoa(want) + `;
					}
				}
			}`
			result, err := LoadConfig(cfgText, path, nil)
			require.NoError(t, err)

			hub, err := ExtractHubConfig(result.Tree)
			require.NoError(t, err)
			assert.Equal(t, plugin.HubAuditConfig{MaxEntries: want}, hub.Audit)
		}
	})

	t.Run("YANG range bounds enforced", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			leaf string
		}{
			{"below min", "max-entries 99;"},
			{"above max", "max-entries 100001;"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				_, err := LoadConfig("bus { hub { audit { "+tc.leaf+" } } }", path, nil)
				require.Error(t, err)
			})
		}
	})

	t.Run("ExtractHubConfig refuses invalid and out-of-range raw tree values", func(t *testing.T) {
		makeTree := func(val string) *Tree {
			root := NewTree()
			bus := NewTree()
			hub := NewTree()
			audit := NewTree()
			audit.Set("max-entries", val)
			hub.SetContainer("audit", audit)
			bus.SetContainer("hub", hub)
			root.SetContainer("bus", bus)
			return root
		}

		for _, tc := range []struct {
			name    string
			val     string
			wantErr string
		}{
			{"non-integer", "abc", "set bus hub audit max-entries 10000"},
			{"below min", "99", "set bus hub audit max-entries 100"},
			{"above max", "100001", "set bus hub audit max-entries 100000"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				_, err := ExtractHubConfig(makeTree(tc.val))
				require.ErrorContains(t, err, tc.wantErr)
			})
		}
	})
}
