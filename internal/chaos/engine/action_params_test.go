// Design: docs/architecture/chaos-web-dashboard.md — parameterized chaos action types

package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseClockDriftParams_Defaults(t *testing.T) {
	p, err := ParseClockDriftParams(nil, 90*time.Second)
	require.NoError(t, err)
	assert.Equal(t, DefaultClockDrift, p.Drift)
}

func TestParseClockDriftParams_CustomDrift(t *testing.T) {
	p, err := ParseClockDriftParams(map[string]string{"drift": "5s"}, 90*time.Second)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, p.Drift)
}

func TestParseClockDriftParams_NegativeDrift(t *testing.T) {
	p, err := ParseClockDriftParams(map[string]string{"drift": "-3s"}, 90*time.Second)
	require.NoError(t, err)
	assert.Equal(t, -3*time.Second, p.Drift)
}

func TestParseClockDriftParams_DriftExceedsHoldTime(t *testing.T) {
	_, err := ParseClockDriftParams(map[string]string{"drift": "90s"}, 90*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "less than hold time")
}

func TestParseClockDriftParams_InvalidDuration(t *testing.T) {
	_, err := ParseClockDriftParams(map[string]string{"drift": "notaduration"}, 90*time.Second)
	assert.Error(t, err)
}

func TestParseClockDriftParams_ZeroHoldTime(t *testing.T) {
	p, err := ParseClockDriftParams(map[string]string{"drift": "100s"}, 0)
	require.NoError(t, err)
	assert.Equal(t, 100*time.Second, p.Drift)
}

func TestParseRouteBurstParams_Defaults(t *testing.T) {
	p := ParseRouteBurstParams(nil)
	assert.Equal(t, DefaultRouteBurstCount, p.Count)
	assert.Equal(t, DefaultRouteBurstFamily, p.Family)
}

func TestParseRouteBurstParams_Custom(t *testing.T) {
	p := ParseRouteBurstParams(map[string]string{"count": "1000", "family": "ipv6/unicast"})
	assert.Equal(t, 1000, p.Count)
	assert.Equal(t, "ipv6/unicast", p.Family)
}

func TestParseRouteBurstParams_CountCapped(t *testing.T) {
	p := ParseRouteBurstParams(map[string]string{"count": "99999"})
	assert.Equal(t, MaxRouteBurstCount, p.Count)
}

func TestParseRouteBurstParams_InvalidCount(t *testing.T) {
	p := ParseRouteBurstParams(map[string]string{"count": "abc"})
	assert.Equal(t, DefaultRouteBurstCount, p.Count)
}

func TestParseWithdrawalBurstParams_Defaults(t *testing.T) {
	p := ParseWithdrawalBurstParams(nil)
	assert.Equal(t, DefaultWithdrawalBurstCount, p.Count)
}

func TestParseWithdrawalBurstParams_Capped(t *testing.T) {
	p := ParseWithdrawalBurstParams(map[string]string{"count": "50000"})
	assert.Equal(t, MaxWithdrawalBurstCount, p.Count)
}

func TestParseRouteFlapParams_Defaults(t *testing.T) {
	p := ParseRouteFlapParams(nil)
	assert.Equal(t, DefaultRouteFlapCount, p.Count)
	assert.Equal(t, DefaultRouteFlapCycles, p.Cycles)
	assert.Equal(t, DefaultRouteFlapInterval, p.Interval)
}

func TestParseRouteFlapParams_Custom(t *testing.T) {
	p := ParseRouteFlapParams(map[string]string{
		"count":    "100",
		"cycles":   "5",
		"interval": "200ms",
	})
	assert.Equal(t, 100, p.Count)
	assert.Equal(t, 5, p.Cycles)
	assert.Equal(t, 200*time.Millisecond, p.Interval)
}

func TestParseRouteFlapParams_CyclesCapped(t *testing.T) {
	p := ParseRouteFlapParams(map[string]string{"cycles": "100"})
	assert.Equal(t, MaxRouteFlapCycles, p.Cycles)
}

func TestParseRouteFlapParams_CountCapped(t *testing.T) {
	p := ParseRouteFlapParams(map[string]string{"count": "5000"})
	assert.Equal(t, MaxRouteFlapCount, p.Count)
}

func TestParseSlowPeerParams_Defaults(t *testing.T) {
	p := ParseSlowPeerParams(nil)
	assert.Equal(t, DefaultSlowPeerDelay, p.Delay)
	assert.Equal(t, DefaultSlowPeerDuration, p.Duration)
}

func TestParseSlowPeerParams_DelayClamped(t *testing.T) {
	p := ParseSlowPeerParams(map[string]string{"delay": "10ms"})
	assert.Equal(t, MinSlowPeerDelay, p.Delay)

	p = ParseSlowPeerParams(map[string]string{"delay": "60s"})
	assert.Equal(t, MaxSlowPeerDelay, p.Delay)
}

func TestParseZeroWindowParams_Defaults(t *testing.T) {
	p := ParseZeroWindowParams(nil)
	assert.Equal(t, DefaultZeroWindowDuration, p.Duration)
}

func TestParseZeroWindowParams_Capped(t *testing.T) {
	p := ParseZeroWindowParams(map[string]string{"duration": "300s"})
	assert.Equal(t, MaxZeroWindowDuration, p.Duration)
}

func TestIsV2Action(t *testing.T) {
	v2 := []ActionType{
		ActionClockDrift, ActionRouteBurst, ActionWithdrawalBurst,
		ActionRouteFlap, ActionSlowPeer, ActionZeroWindow,
	}
	for _, at := range v2 {
		assert.True(t, IsV2Action(at), "expected %s to be v2", at)
	}

	v1 := []ActionType{
		ActionTCPDisconnect, ActionNotificationCease, ActionHoldTimerExpiry,
		ActionDisconnectDuringBurst, ActionReconnectStorm, ActionConnectionCollision,
		ActionMalformedUpdate, ActionConfigReload, ActionSlowRead,
	}
	for _, at := range v1 {
		assert.False(t, IsV2Action(at), "expected %s to be v1", at)
	}
}
