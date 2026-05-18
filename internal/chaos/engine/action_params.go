// Design: docs/architecture/chaos-web-dashboard.md — parameterized chaos action types

package engine

import (
	"fmt"
	"strconv"
	"time"
)

// Parameter key constants for parameterized chaos actions.
const (
	ParamDrift    = "drift"
	ParamCount    = "count"
	ParamFamily   = "family"
	ParamCycles   = "cycles"
	ParamInterval = "interval"
	ParamDelay    = "delay"
	ParamDuration = "duration"
)

// Default values for parameterized action parameters.
const (
	DefaultRouteBurstCount      = 500
	DefaultRouteBurstFamily     = "ipv4/unicast"
	DefaultWithdrawalBurstCount = 100
	DefaultRouteFlapCount       = 50
	DefaultRouteFlapCycles      = 3
	DefaultRouteFlapInterval    = 100 * time.Millisecond
	DefaultSlowPeerDelay        = 2 * time.Second
	DefaultSlowPeerDuration     = 30 * time.Second
	DefaultZeroWindowDuration   = 15 * time.Second
	DefaultClockDrift           = 2 * time.Second

	MaxRouteBurstCount      = 10000
	MaxWithdrawalBurstCount = 10000
	MaxRouteFlapCycles      = 50
	MaxRouteFlapCount       = 1000
	MinSlowPeerDelay        = 100 * time.Millisecond
	MaxSlowPeerDelay        = 30 * time.Second
	MaxZeroWindowDuration   = 120 * time.Second
)

// ClockDriftParams holds validated parameters for ActionClockDrift.
type ClockDriftParams struct {
	Drift time.Duration
}

// ParseClockDriftParams extracts and validates ClockDrift parameters.
// holdTime is the peer's negotiated hold time, used for range validation.
func ParseClockDriftParams(params map[string]string, holdTime time.Duration) (ClockDriftParams, error) {
	drift := DefaultClockDrift
	if s, ok := params[ParamDrift]; ok {
		d, err := time.ParseDuration(s)
		if err != nil {
			return ClockDriftParams{}, fmt.Errorf("invalid drift %q: %w", s, err)
		}
		drift = d
	}
	abs := drift
	if abs < 0 {
		abs = -abs
	}
	if holdTime > 0 && abs >= holdTime {
		return ClockDriftParams{}, fmt.Errorf("drift %s must be less than hold time %s", drift, holdTime)
	}
	return ClockDriftParams{Drift: drift}, nil
}

// RouteBurstParams holds validated parameters for ActionRouteBurst.
type RouteBurstParams struct {
	Count  int
	Family string
}

// ParseRouteBurstParams extracts and validates RouteBurst parameters.
func ParseRouteBurstParams(params map[string]string) RouteBurstParams {
	count := DefaultRouteBurstCount
	if s, ok := params[ParamCount]; ok {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			count = v
		}
	}
	if count > MaxRouteBurstCount {
		count = MaxRouteBurstCount
	}
	family := DefaultRouteBurstFamily
	if s, ok := params[ParamFamily]; ok && s != "" {
		family = s
	}
	return RouteBurstParams{Count: count, Family: family}
}

// WithdrawalBurstParams holds validated parameters for ActionWithdrawalBurst.
type WithdrawalBurstParams struct {
	Count int
}

// ParseWithdrawalBurstParams extracts and validates WithdrawalBurst parameters.
func ParseWithdrawalBurstParams(params map[string]string) WithdrawalBurstParams {
	count := DefaultWithdrawalBurstCount
	if s, ok := params[ParamCount]; ok {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			count = v
		}
	}
	if count > MaxWithdrawalBurstCount {
		count = MaxWithdrawalBurstCount
	}
	return WithdrawalBurstParams{Count: count}
}

// RouteFlapParams holds validated parameters for ActionRouteFlap.
type RouteFlapParams struct {
	Count    int
	Cycles   int
	Interval time.Duration
}

// ParseRouteFlapParams extracts and validates RouteFlap parameters.
func ParseRouteFlapParams(params map[string]string) RouteFlapParams {
	count := DefaultRouteFlapCount
	if s, ok := params[ParamCount]; ok {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			count = v
		}
	}
	if count > MaxRouteFlapCount {
		count = MaxRouteFlapCount
	}
	cycles := DefaultRouteFlapCycles
	if s, ok := params[ParamCycles]; ok {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			cycles = v
		}
	}
	if cycles > MaxRouteFlapCycles {
		cycles = MaxRouteFlapCycles
	}
	interval := DefaultRouteFlapInterval
	if s, ok := params[ParamInterval]; ok {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			interval = d
		}
	}
	return RouteFlapParams{Count: count, Cycles: cycles, Interval: interval}
}

// SlowPeerParams holds validated parameters for ActionSlowPeer.
type SlowPeerParams struct {
	Delay    time.Duration
	Duration time.Duration
}

// ParseSlowPeerParams extracts and validates SlowPeer parameters.
func ParseSlowPeerParams(params map[string]string) SlowPeerParams {
	delay := DefaultSlowPeerDelay
	if s, ok := params[ParamDelay]; ok {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			delay = d
		}
	}
	if delay < MinSlowPeerDelay {
		delay = MinSlowPeerDelay
	}
	if delay > MaxSlowPeerDelay {
		delay = MaxSlowPeerDelay
	}
	dur := DefaultSlowPeerDuration
	if s, ok := params[ParamDuration]; ok {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			dur = d
		}
	}
	return SlowPeerParams{Delay: delay, Duration: dur}
}

// ZeroWindowParams holds validated parameters for ActionZeroWindow.
type ZeroWindowParams struct {
	Duration time.Duration
}

// ParseZeroWindowParams extracts and validates ZeroWindow parameters.
func ParseZeroWindowParams(params map[string]string) ZeroWindowParams {
	dur := DefaultZeroWindowDuration
	if s, ok := params[ParamDuration]; ok {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			dur = d
		}
	}
	if dur > MaxZeroWindowDuration {
		dur = MaxZeroWindowDuration
	}
	return ZeroWindowParams{Duration: dur}
}

// IsV2Action returns true if the action type is one of the 6 new parameterized actions.
func IsV2Action(t ActionType) bool {
	switch t {
	case ActionClockDrift, ActionRouteBurst, ActionWithdrawalBurst,
		ActionRouteFlap, ActionSlowPeer, ActionZeroWindow:
		return true
	case ActionTCPDisconnect, ActionNotificationCease, ActionHoldTimerExpiry,
		ActionDisconnectDuringBurst, ActionReconnectStorm, ActionConnectionCollision,
		ActionMalformedUpdate, ActionConfigReload, ActionSlowRead:
		return false
	}
	return false
}
