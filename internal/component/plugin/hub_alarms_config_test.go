// VALIDATES: DefaultHubAlarmsConfig populates every alarm delay, resource
// threshold, software-behind threshold, and history retention bound with the
// YANG defaults, and ValidateHubAlarmsConfig accepts valid boundary values
// while refusing every below-min and above-max setting with an operator error
// naming the exact set bus hub alarms fix command.
// PREVENTS: Omitted alarm defaults drifting from ze-plugin-conf.yang or
// out-of-range alarm bounds reaching the hub alarm evaluator.

package plugin

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

func TestDefaultHubAlarmsConfig(t *testing.T) {
	cfg := DefaultHubAlarmsConfig()
	assert.Equal(t, DefaultAlarmOfflineDelaySeconds, cfg.OfflineDelaySeconds)
	assert.Equal(t, DefaultAlarmResourcePercent, cfg.ResourceCPUPercent)
	assert.Equal(t, DefaultAlarmResourcePercent, cfg.ResourceMemoryPercent)
	assert.Equal(t, DefaultAlarmResourcePercent, cfg.ResourceStoragePercent)
	assert.Equal(t, DefaultAlarmResourceDurationSeconds, cfg.ResourceDurationSeconds)
	assert.Equal(t, DefaultAlarmSoftwareBehindVersions, cfg.SoftwareBehindVersions)
	assert.Equal(t, DefaultAlarmRetentionDays, cfg.RetentionDays)
	assert.Equal(t, DefaultAlarmMaxEpisodesPerDevice, cfg.MaxEpisodesPerDevice)
	assert.Equal(t, DefaultAlarmMaxEpisodesHub, cfg.MaxEpisodesHub)
	require.NoError(t, ValidateHubAlarmsConfig(cfg))
}

func TestValidateHubAlarmsConfigBounds(t *testing.T) {
	minCfg := HubAlarmsConfig{
		OfflineDelaySeconds:     MinAlarmOfflineDelaySeconds,
		ResourceCPUPercent:      MinAlarmResourcePercent,
		ResourceMemoryPercent:   MinAlarmResourcePercent,
		ResourceStoragePercent:  MinAlarmResourcePercent,
		ResourceDurationSeconds: MinAlarmResourceDurationSeconds,
		SoftwareBehindVersions:  MinAlarmSoftwareBehindVersions,
		RetentionDays:           MinAlarmRetentionDays,
		MaxEpisodesPerDevice:    MinAlarmMaxEpisodesPerDevice,
		MaxEpisodesHub:          MinAlarmMaxEpisodesHub,
	}
	require.NoError(t, ValidateHubAlarmsConfig(minCfg))

	maxCfg := HubAlarmsConfig{
		OfflineDelaySeconds:     MaxAlarmOfflineDelaySeconds,
		ResourceCPUPercent:      MaxAlarmResourcePercent,
		ResourceMemoryPercent:   MaxAlarmResourcePercent,
		ResourceStoragePercent:  MaxAlarmResourcePercent,
		ResourceDurationSeconds: MaxAlarmResourceDurationSeconds,
		SoftwareBehindVersions:  MaxAlarmSoftwareBehindVersions,
		RetentionDays:           MaxAlarmRetentionDays,
		MaxEpisodesPerDevice:    MaxAlarmMaxEpisodesPerDevice,
		MaxEpisodesHub:          MaxAlarmMaxEpisodesHub,
	}
	require.NoError(t, ValidateHubAlarmsConfig(maxCfg))

	cases := []struct {
		name     string
		mutate   func(*HubAlarmsConfig)
		wantWhat string
		wantFix  string
	}{
		{
			name:     "offline-delay below min",
			mutate:   func(c *HubAlarmsConfig) { c.OfflineDelaySeconds = 0 },
			wantWhat: "bus hub alarms offline-delay 0 is below minimum 1 seconds",
			wantFix:  "set bus hub alarms offline-delay 1",
		},
		{
			name:     "offline-delay above max",
			mutate:   func(c *HubAlarmsConfig) { c.OfflineDelaySeconds = MaxAlarmOfflineDelaySeconds + 1 },
			wantWhat: "bus hub alarms offline-delay 86401 exceeds maximum 86400 seconds",
			wantFix:  "set bus hub alarms offline-delay 86400",
		},
		{
			name:     "resource-cpu-percent below min",
			mutate:   func(c *HubAlarmsConfig) { c.ResourceCPUPercent = 0 },
			wantWhat: "bus hub alarms resource-cpu-percent 0 is below minimum 1 percent",
			wantFix:  "set bus hub alarms resource-cpu-percent 1",
		},
		{
			name:     "resource-cpu-percent above max",
			mutate:   func(c *HubAlarmsConfig) { c.ResourceCPUPercent = 101 },
			wantWhat: "bus hub alarms resource-cpu-percent 101 exceeds maximum 100 percent",
			wantFix:  "set bus hub alarms resource-cpu-percent 100",
		},
		{
			name:     "resource-memory-percent below min",
			mutate:   func(c *HubAlarmsConfig) { c.ResourceMemoryPercent = -5 },
			wantWhat: "bus hub alarms resource-memory-percent -5 is below minimum 1 percent",
			wantFix:  "set bus hub alarms resource-memory-percent 1",
		},
		{
			name:     "resource-memory-percent above max",
			mutate:   func(c *HubAlarmsConfig) { c.ResourceMemoryPercent = 101 },
			wantWhat: "bus hub alarms resource-memory-percent 101 exceeds maximum 100 percent",
			wantFix:  "set bus hub alarms resource-memory-percent 100",
		},
		{
			name:     "resource-storage-percent below min",
			mutate:   func(c *HubAlarmsConfig) { c.ResourceStoragePercent = 0 },
			wantWhat: "bus hub alarms resource-storage-percent 0 is below minimum 1 percent",
			wantFix:  "set bus hub alarms resource-storage-percent 1",
		},
		{
			name:     "resource-storage-percent above max",
			mutate:   func(c *HubAlarmsConfig) { c.ResourceStoragePercent = 101 },
			wantWhat: "bus hub alarms resource-storage-percent 101 exceeds maximum 100 percent",
			wantFix:  "set bus hub alarms resource-storage-percent 100",
		},
		{
			name:     "resource-duration below min",
			mutate:   func(c *HubAlarmsConfig) { c.ResourceDurationSeconds = 0 },
			wantWhat: "bus hub alarms resource-duration 0 is below minimum 1 seconds",
			wantFix:  "set bus hub alarms resource-duration 1",
		},
		{
			name:     "resource-duration above max",
			mutate:   func(c *HubAlarmsConfig) { c.ResourceDurationSeconds = 86401 },
			wantWhat: "bus hub alarms resource-duration 86401 exceeds maximum 86400 seconds",
			wantFix:  "set bus hub alarms resource-duration 86400",
		},
		{
			name:     "software-behind-versions below min",
			mutate:   func(c *HubAlarmsConfig) { c.SoftwareBehindVersions = 0 },
			wantWhat: "bus hub alarms software-behind-versions 0 is below minimum 1",
			wantFix:  "set bus hub alarms software-behind-versions 1",
		},
		{
			name:     "software-behind-versions above max",
			mutate:   func(c *HubAlarmsConfig) { c.SoftwareBehindVersions = 1001 },
			wantWhat: "bus hub alarms software-behind-versions 1001 exceeds maximum 1000",
			wantFix:  "set bus hub alarms software-behind-versions 1000",
		},
		{
			name:     "retention-days below min",
			mutate:   func(c *HubAlarmsConfig) { c.RetentionDays = 0 },
			wantWhat: "bus hub alarms retention-days 0 is below minimum 1 days",
			wantFix:  "set bus hub alarms retention-days 1",
		},
		{
			name:     "retention-days above max",
			mutate:   func(c *HubAlarmsConfig) { c.RetentionDays = 31 },
			wantWhat: "bus hub alarms retention-days 31 exceeds maximum 30 days",
			wantFix:  "set bus hub alarms retention-days 30",
		},
		{
			name:     "max-episodes-per-device below min",
			mutate:   func(c *HubAlarmsConfig) { c.MaxEpisodesPerDevice = 0 },
			wantWhat: "bus hub alarms max-episodes-per-device 0 is below minimum 1",
			wantFix:  "set bus hub alarms max-episodes-per-device 1",
		},
		{
			name:     "max-episodes-per-device above max",
			mutate:   func(c *HubAlarmsConfig) { c.MaxEpisodesPerDevice = 513 },
			wantWhat: "bus hub alarms max-episodes-per-device 513 exceeds maximum 512",
			wantFix:  "set bus hub alarms max-episodes-per-device 512",
		},
		{
			name:     "max-episodes-hub below min",
			mutate:   func(c *HubAlarmsConfig) { c.MaxEpisodesHub = 0 },
			wantWhat: "bus hub alarms max-episodes-hub 0 is below minimum 1",
			wantFix:  "set bus hub alarms max-episodes-hub 1",
		},
		{
			name:     "max-episodes-hub above max",
			mutate:   func(c *HubAlarmsConfig) { c.MaxEpisodesHub = 65537 },
			wantWhat: "bus hub alarms max-episodes-hub 65537 exceeds maximum 65536",
			wantFix:  "set bus hub alarms max-episodes-hub 65536",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultHubAlarmsConfig()
			tc.mutate(&cfg)

			err := ValidateHubAlarmsConfig(cfg)
			require.Error(t, err)

			opErr, ok := errors.AsType[*opererr.Error](err)
			require.True(t, ok, "validation refusal must be an *opererr.Error")
			assert.Equal(t, tc.wantWhat, opErr.Problem())
			assert.NotEmpty(t, opErr.Why())
			assert.Equal(t, tc.wantFix, opErr.Fix())
		})
	}
}
