// VALIDATES: ExtractHubConfig returns a zero plugin.HubConfig when bus/hub is
// absent, seeds DefaultHubAlarmsConfig when bus/hub/server omits alarms,
// parses every explicit bus/hub/alarms leaf override, and refuses invalid or
// out-of-range alarm values through both YANG loading and direct extraction.
// PREVENTS: Non-zero HubConfig breaking FirstPass with no bus/hub block,
// omitted alarm leaves collapsing to zero, or invalid alarm bounds passing
// config validation.

package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

func TestExtractHubConfigAlarms(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hub-alarms.conf")

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

	t.Run("server without alarms block gets defaults", func(t *testing.T) {
		result, err := LoadConfig(hubLabelConfig(""), path, nil)
		require.NoError(t, err)

		hub, err := ExtractHubConfig(result.Tree)
		require.NoError(t, err)
		assert.Equal(t, plugin.DefaultHubAlarmsConfig(), hub.Alarms)
	})

	t.Run("partial alarms block preserves defaults for omitted leaves", func(t *testing.T) {
		partialCfg := `bus {
			hub {
				alarms {
					offline-delay 120;
					retention-days 14;
				}
			}
		}`
		result, err := LoadConfig(partialCfg, path, nil)
		require.NoError(t, err)

		hub, err := ExtractHubConfig(result.Tree)
		require.NoError(t, err)

		want := plugin.DefaultHubAlarmsConfig()
		want.OfflineDelaySeconds = 120
		want.RetentionDays = 14
		assert.Equal(t, want, hub.Alarms)
	})

	t.Run("explicit alarms overrides parse every leaf", func(t *testing.T) {
		customCfg := `bus {
			hub {
				server central {
					client edge-01 { secret ` + labelTestSecret + `; }
				}
				alarms {
					offline-delay 45;
					resource-cpu-percent 80;
					resource-memory-percent 85;
					resource-storage-percent 95;
					resource-duration 180;
					software-behind-versions 3;
					retention-days 14;
					max-episodes-per-device 128;
					max-episodes-hub 4096;
				}
			}
		}`
		result, err := LoadConfig(customCfg, path, nil)
		require.NoError(t, err)

		hub, err := ExtractHubConfig(result.Tree)
		require.NoError(t, err)
		assert.Equal(t, plugin.HubAlarmsConfig{
			OfflineDelaySeconds:     45,
			ResourceCPUPercent:      80,
			ResourceMemoryPercent:   85,
			ResourceStoragePercent:  95,
			ResourceDurationSeconds: 180,
			SoftwareBehindVersions:  3,
			RetentionDays:           14,
			MaxEpisodesPerDevice:    128,
			MaxEpisodesHub:          4096,
		}, hub.Alarms)
	})

	t.Run("YANG range bounds enforced", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			leaf string
		}{
			{"offline-delay below min", "offline-delay 0;"},
			{"offline-delay above max", "offline-delay 86401;"},
			{"resource-cpu-percent below min", "resource-cpu-percent 0;"},
			{"resource-cpu-percent above max", "resource-cpu-percent 101;"},
			{"resource-memory-percent below min", "resource-memory-percent 0;"},
			{"resource-memory-percent above max", "resource-memory-percent 101;"},
			{"resource-storage-percent below min", "resource-storage-percent 0;"},
			{"resource-storage-percent above max", "resource-storage-percent 101;"},
			{"resource-duration below min", "resource-duration 0;"},
			{"resource-duration above max", "resource-duration 86401;"},
			{"software-behind-versions below min", "software-behind-versions 0;"},
			{"software-behind-versions above max", "software-behind-versions 1001;"},
			{"retention-days below min", "retention-days 0;"},
			{"retention-days above max", "retention-days 31;"},
			{"max-episodes-per-device below min", "max-episodes-per-device 0;"},
			{"max-episodes-per-device above max", "max-episodes-per-device 513;"},
			{"max-episodes-hub below min", "max-episodes-hub 0;"},
			{"max-episodes-hub above max", "max-episodes-hub 65537;"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				_, err := LoadConfig("bus { hub { alarms { "+tc.leaf+" } } }", path, nil)
				require.Error(t, err)
			})
		}
	})

	t.Run("ExtractHubConfig refuses invalid and out-of-range raw tree values", func(t *testing.T) {
		makeTree := func(leaf, val string) *Tree {
			root := NewTree()
			bus := NewTree()
			hub := NewTree()
			alarms := NewTree()
			alarms.Set(leaf, val)
			hub.SetContainer("alarms", alarms)
			bus.SetContainer("hub", hub)
			root.SetContainer("bus", bus)
			return root
		}

		for _, tc := range []struct {
			name    string
			leaf    string
			val     string
			wantErr string
		}{
			{"non-integer offline-delay", "offline-delay", "abc", "set bus hub alarms offline-delay 300"},
			{"out-of-range offline-delay", "offline-delay", "0", "set bus hub alarms offline-delay 1"},
			{"out-of-range cpu", "resource-cpu-percent", "101", "set bus hub alarms resource-cpu-percent 100"},
			{"out-of-range memory", "resource-memory-percent", "0", "set bus hub alarms resource-memory-percent 1"},
			{"out-of-range storage", "resource-storage-percent", "101", "set bus hub alarms resource-storage-percent 100"},
			{"out-of-range resource-duration", "resource-duration", "86401", "set bus hub alarms resource-duration 86400"},
			{"out-of-range software-behind-versions", "software-behind-versions", "1001", "set bus hub alarms software-behind-versions 1000"},
			{"out-of-range retention-days", "retention-days", "31", "set bus hub alarms retention-days 30"},
			{"out-of-range max-episodes-per-device", "max-episodes-per-device", "513", "set bus hub alarms max-episodes-per-device 512"},
			{"out-of-range max-episodes-hub", "max-episodes-hub", "65537", "set bus hub alarms max-episodes-hub 65536"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				_, err := ExtractHubConfig(makeTree(tc.leaf, tc.val))
				require.ErrorContains(t, err, tc.wantErr)
			})
		}
	})
}
