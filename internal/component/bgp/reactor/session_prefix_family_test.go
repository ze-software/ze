// Design: docs/architecture/core-design.md — prefix limit enforcement (RFC 4486)
// Overview: session_prefix.go — applyPrefixCheck, the enforcement producer
// Related: peer_run.go — prefixReconnectDecision, the reconnect producer
// Related: config_prefix_test.go — the parse half of the same behavior

package reactor

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// twoFamilyPrefixSettings builds PeerSettings with a maximum on both
// ipv4/unicast and ipv6/unicast, and the given teardown choice for each.
func twoFamilyPrefixSettings(v4Teardown, v6Teardown bool) *PeerSettings {
	ps := newTestPeerSettingsWithPrefix(3, 2)
	ps.PrefixMaximum["ipv6/unicast"] = 3
	ps.PrefixWarning["ipv6/unicast"] = 2
	ps.PrefixTeardown = map[string]bool{
		"ipv4/unicast": v4Teardown,
		"ipv6/unicast": v6Teardown,
	}
	return ps
}

// ipv4OverflowBody announces four IPv4 /24 prefixes, one over a maximum of 3.
func ipv4OverflowBody() []byte {
	return []byte{0, 0, 0, 0, 24, 10, 0, 0, 24, 10, 0, 1, 24, 10, 0, 2, 24, 10, 0, 3}
}

// ipv6OverflowBody announces four IPv6 /32 prefixes in an MP_REACH_NLRI
// attribute, one over a maximum of 3. RFC 4760 Section 3.
func ipv6OverflowBody() []byte {
	mpReach := []byte{
		0x00, 0x02, // AFI = 2 (IPv6)
		0x01,                                                       // SAFI = 1 (unicast)
		0x10,                                                       // next-hop length = 16
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, // 2001:db8::1
		0x00,                         // reserved
		0x20, 0x20, 0x01, 0x0d, 0xb8, // 2001:db8::/32
		0x20, 0x20, 0x01, 0x0d, 0xb9, // 2001:db9::/32
		0x20, 0x20, 0x01, 0x0d, 0xba, // 2001:dba::/32
		0x20, 0x20, 0x01, 0x0d, 0xbb, // 2001:dbb::/32
	}
	attr := append([]byte{
		0x90, // optional, transitive, extended length
		0x0e, // MP_REACH_NLRI
		0x00, byte(len(mpReach)),
	}, mpReach...)

	body := []byte{0, 0} // no withdrawn routes
	body = append(body, byte(len(attr)>>8), byte(len(attr)))
	return append(body, attr...)
}

// TestPrefixTeardownPerFamilyEnforcement verifies each family's own teardown
// choice decides what happens when that family overflows.
//
// VALIDATES: AC-1 and AC-2. With ipv4/unicast set to teardown and
// ipv6/unicast set to warn-only, an IPv4 overflow sends a Cease NOTIFICATION
// naming IPv4, and an IPv6 overflow drops the excess and keeps the session.
// PREVENTS: The defect this spec fixes. One shared scalar meant the warn-only
// family could stop the session, and the teardown family could fail to.
func TestPrefixTeardownPerFamilyEnforcement(t *testing.T) {
	t.Run("teardown family stops the session", func(t *testing.T) {
		s := NewSession(twoFamilyPrefixSettings(true, false))

		notif, drop := s.checkPrefixLimits(testWireUpdate(ipv4OverflowBody()))

		require.NotNil(t, notif, "ipv4/unicast asked for teardown")
		assert.False(t, drop)
		assert.Equal(t, message.NotifyCease, notif.ErrorCode)
		assert.Equal(t, message.NotifyCeaseMaxPrefixes, notif.ErrorSubcode)
		require.Len(t, notif.Data, 7)
		assert.Equal(t, []byte{0, 1, 1}, notif.Data[:3],
			"RFC 4486 Section 4 data names the offending family: AFI 1, SAFI 1")
		assert.Equal(t, "ipv4/unicast", familyString(s.prefixExceededFamily))
	})

	t.Run("warn-only family keeps the session", func(t *testing.T) {
		s := NewSession(twoFamilyPrefixSettings(true, false))

		notif, drop := s.checkPrefixLimits(testWireUpdate(ipv6OverflowBody()))

		assert.Nil(t, notif, "ipv6/unicast asked for warn-only, so no NOTIFICATION")
		assert.True(t, drop, "the excess NLRI is dropped, not installed")
		assert.Equal(t, int64(4), s.prefixCounts.counts[ipv6UKey])
		assert.Equal(t, int64(0), s.prefixCounts.counts[ipv4UKey],
			"the other family is untouched")
	})

	t.Run("the choice follows the family, not the key order", func(t *testing.T) {
		// The reverse assignment. "ipv4/unicast" sorts first, so a fix that
		// only moved which family wins would pass one of these two subtests
		// and fail the other.
		s := NewSession(twoFamilyPrefixSettings(false, true))

		notif, drop := s.checkPrefixLimits(testWireUpdate(ipv4OverflowBody()))
		assert.Nil(t, notif, "ipv4/unicast now asks for warn-only")
		assert.True(t, drop)

		s6 := NewSession(twoFamilyPrefixSettings(false, true))
		notif6, drop6 := s6.checkPrefixLimits(testWireUpdate(ipv6OverflowBody()))
		require.NotNil(t, notif6, "ipv6/unicast now asks for teardown")
		assert.False(t, drop6)
		assert.Equal(t, []byte{0, 2, 1}, notif6.Data[:3], "AFI 2, SAFI 1")
	})
}

// TestPrefixTeardownAbsentFamilyFailsClosed verifies a family with no teardown
// entry is enforced rather than treated as warn-only.
//
// VALIDATES: AC-6 and R-1. The map value type is bool, whose zero value is
// false, and false means warn-only. A bare map read on the enforcement path
// would therefore disable prefix protection for any family the operator did not
// name (ai/rules/fail-closed-guards.md).
// PREVENTS: A peer flooding the RIB through a family whose key is absent.
// Driven from checkPrefixLimits, the session entry point, not from the
// accessor alone.
func TestPrefixTeardownAbsentFamilyFailsClosed(t *testing.T) {
	ps := twoFamilyPrefixSettings(true, false)
	// Only ipv6/unicast is named. ipv4/unicast has a maximum and no teardown.
	ps.PrefixTeardown = map[string]bool{"ipv6/unicast": false}

	s := NewSession(ps)
	notif, drop := s.checkPrefixLimits(testWireUpdate(ipv4OverflowBody()))

	require.NotNil(t, notif,
		"an unconfigured family must enforce, never read as warn-only")
	assert.False(t, drop)
	assert.Equal(t, message.NotifyCeaseMaxPrefixes, notif.ErrorSubcode)
}

// TestPrefixTeardownNilMapFailsClosed verifies a peer that configures no
// teardown at all still enforces every family.
//
// VALIDATES: AC-6. NewPeerSettings leaves PrefixTeardown nil, and the YANG
// default is `teardown true`, so nil must read as enabled.
// PREVENTS: Direct callers (tests, API) silently losing prefix protection when
// the field stopped being a bool that defaulted to true.
func TestPrefixTeardownNilMapFailsClosed(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(3, 2)
	require.Nil(t, ps.PrefixTeardown, "NewPeerSettings seeds no entry")

	s := NewSession(ps)
	notif, _ := s.checkPrefixLimits(testWireUpdate(ipv4OverflowBody()))

	require.NotNil(t, notif, "a nil map must read as enforcement enabled")
}

// TestPrefixTeardownCauseNamesFamily verifies the teardown error carries the
// offending family while staying matchable by the sentinel.
//
// VALIDATES: The session-to-peer boundary. peer_run.go reads the family to pick
// that family's idle-timeout, and every existing errors.Is on the reconnect
// path must keep matching (R-3).
// PREVENTS: Replacing the sentinel with a new type, which would silently stop
// every existing matcher.
func TestPrefixTeardownCauseNamesFamily(t *testing.T) {
	s := NewSession(twoFamilyPrefixSettings(true, true))
	notif, _ := s.checkPrefixLimits(testWireUpdate(ipv6OverflowBody()))
	require.NotNil(t, notif)

	err := s.prefixTeardownCause()

	assert.ErrorIs(t, err, ErrPrefixLimitExceeded,
		"errors.Is on the sentinel must still match")
	var prefixErr *prefixLimitError
	require.True(t, errors.As(err, &prefixErr), "errors.As must recover the family")
	assert.Equal(t, "ipv6/unicast", prefixErr.Family)
	assert.Contains(t, err.Error(), "ipv6/unicast",
		"the operator-visible message names the family that stopped the session")
}

// TestPrefixIdleTimeoutPerFamily verifies the reconnect delay comes from the
// family that overflowed.
//
// VALIDATES: AC-4. The `idle-timeout` leaf sits in the per-family prefix
// container, so a teardown caused by ipv6/unicast waits its 7 seconds, not the
// 3600 seconds configured on ipv4/unicast.
// PREVENTS: A reconnect delay sized by a family that did not overflow.
func TestPrefixIdleTimeoutPerFamily(t *testing.T) {
	ps := twoFamilyPrefixSettings(true, true)
	ps.PrefixIdleTimeout = map[string]uint16{
		"ipv4/unicast": 3600,
		"ipv6/unicast": 7,
	}

	tests := []struct {
		family string
		want   time.Duration
	}{
		{"ipv6/unicast", 7 * time.Second},
		{"ipv4/unicast", 3600 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.family, func(t *testing.T) {
			err := errors.Join(ErrConnectionClosed, &prefixLimitError{Family: tt.family})

			plan, ok := prefixReconnectDecision(ps, err, 1)

			require.True(t, ok)
			assert.Equal(t, PrefixReconnectTimer, plan.Mode)
			assert.Equal(t, tt.want, plan.Delay)
			assert.Equal(t, tt.family, plan.Family)
		})
	}
}

// TestPrefixIdleTimeoutZeroSemantics states what `idle-timeout 0` does.
//
// VALIDATES: zero means the peer STAYS DOWN. The YANG description always said
// "0 = no reconnect", and the YANG default is 0, so this governs every peer that
// never sets the leaf.
// PREVENTS: the flap loop. A peer stopped for flooding the RIB that comes back
// on the usual backoff re-floods it and cycles.
//
// This test previously asserted the opposite. It was written to PIN the code's
// behavior while the disagreement between the YANG text and the code was open
// (A-7 of spec-fixit-bgp-per-family-prefix-enforcement), and it read: "zero
// declines the prefix backoff", after which Peer.run fell through to its normal
// backoff and reconnected. Thomas settled A-7 on 2026-08-03: 0 stays down, and
// `reconnect backoff` is the explicit way to ask for the old behavior. The old
// assertion encoded the defect, so it is corrected here rather than deleted.
func TestPrefixIdleTimeoutZeroSemantics(t *testing.T) {
	ps := twoFamilyPrefixSettings(true, true)
	ps.PrefixIdleTimeout = map[string]uint16{"ipv4/unicast": 0}
	err := errors.Join(ErrConnectionClosed, &prefixLimitError{Family: "ipv4/unicast"})

	plan, ok := prefixReconnectDecision(ps, err, 1)

	require.True(t, ok, "a prefix teardown is always the prefix path, whatever the mode")
	assert.Equal(t, PrefixReconnectNever, plan.Mode, "zero keeps the peer down")
	assert.Equal(t, time.Duration(0), plan.Delay, "never waits for nothing")
	assert.Equal(t, "ipv4/unicast", plan.Family, "the family is still reported for the log")
}

// TestPrefixReconnectExplicitModes verifies the `reconnect` leaf overrides the
// derived default in both directions.
//
// VALIDATES: `reconnect backoff` restores the pre-2026-08-03 behavior for an
// operator who wants it, and `reconnect never` is expressible beside a timer
// configured on another family.
// PREVENTS: one leaf answering two questions. `idle-timeout` sizes a wait;
// `reconnect` says whether there is one.
func TestPrefixReconnectExplicitModes(t *testing.T) {
	tests := []struct {
		name      string
		reconnect map[string]PrefixReconnectMode
		idle      map[string]uint16
		want      PrefixReconnectMode
		wantDelay time.Duration
	}{
		{"unset with no timer holds down", nil, nil, PrefixReconnectNever, 0},
		{"unset with a timer keeps the timer", nil, map[string]uint16{"ipv4/unicast": 30}, PrefixReconnectTimer, 30 * time.Second},
		{
			"explicit backoff",
			map[string]PrefixReconnectMode{"ipv4/unicast": PrefixReconnectBackoff},
			nil, PrefixReconnectBackoff, 0,
		},
		{
			"explicit never",
			map[string]PrefixReconnectMode{"ipv4/unicast": PrefixReconnectNever},
			nil, PrefixReconnectNever, 0,
		},
		{
			"another family's mode does not reach this one",
			map[string]PrefixReconnectMode{"ipv6/unicast": PrefixReconnectBackoff},
			nil, PrefixReconnectNever, 0,
		},
		{
			// A timer of zero seconds is not a timer. Peer.run would get a
			// delay of 0, the doubling never leaves zero, and the peer would
			// reconnect at once and re-exceed its maximum: a flap loop with no
			// backoff at all. parsePrefixReconnect refuses the pair in a config,
			// so only a PeerSettings built in Go reaches it, and the accessor
			// is where the refusal cannot be bypassed.
			"explicit timer with no idle-timeout holds down",
			map[string]PrefixReconnectMode{"ipv4/unicast": PrefixReconnectTimer},
			nil, PrefixReconnectNever, 0,
		},
		{
			"explicit timer with another family's idle-timeout still holds down",
			map[string]PrefixReconnectMode{"ipv4/unicast": PrefixReconnectTimer},
			map[string]uint16{"ipv6/unicast": 30}, PrefixReconnectNever, 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := twoFamilyPrefixSettings(true, true)
			ps.PrefixReconnect = tt.reconnect
			ps.PrefixIdleTimeout = tt.idle
			err := errors.Join(ErrConnectionClosed, &prefixLimitError{Family: "ipv4/unicast"})

			plan, ok := prefixReconnectDecision(ps, err, 1)

			require.True(t, ok)
			assert.Equal(t, tt.want, plan.Mode)
			assert.Equal(t, tt.wantDelay, plan.Delay)
		})
	}
}

// TestPrefixReconnectDelayNonPrefixError verifies an unrelated error never
// takes the prefix path.
//
// VALIDATES: the reconnect path only changes shape for a prefix teardown, and a
// prefix teardown that names no family fails closed to never.
// PREVENTS: A hold-timer expiry or TCP loss waiting a prefix idle-timeout, and
// an unnamed family reading as "reconnect".
func TestPrefixReconnectDelayNonPrefixError(t *testing.T) {
	ps := twoFamilyPrefixSettings(true, true)
	ps.PrefixIdleTimeout = map[string]uint16{"ipv4/unicast": 30}

	_, ok := prefixReconnectDecision(ps, ErrHoldTimerExpired, 1)
	assert.False(t, ok, "an unrelated error keeps the normal backoff")

	// The bare sentinel names no family. It IS a prefix teardown, so it takes
	// the prefix path, and the unconfigured family key "" resolves to never.
	//
	// The assertions here used to be the opposite pair (ok false, mode Unset),
	// which is the fall-through to the NORMAL connect backoff: a session stopped
	// for flooding the RIB coming straight back. The doc comment above this test
	// already claimed the fail-closed behavior, so the assertions were pinning a
	// fail-open path the code never intended (ai/rules/testing.md: the test is
	// what is wrong when it disagrees with the stated contract). The code now
	// decides, and this proves which way.
	plan, okBare := prefixReconnectDecision(ps, ErrPrefixLimitExceeded, 1)
	assert.True(t, okBare, "a prefix teardown is decided here even when it names no family")
	assert.Equal(t, PrefixReconnectNever, plan.Mode,
		"an unnamed family must never read as reconnect")
	assert.Empty(t, plan.Family)
	assert.Zero(t, plan.Delay)
}

// TestPrefixReconnectDelayBackoff verifies the doubling and the one-hour cap.
//
// VALIDATES: The backoff shape this spec preserves: idle-timeout x 2^(count-1),
// capped at one hour, with the count itself capped so the arithmetic cannot
// overflow int64 nanoseconds.
// PREVENTS: An overflow turning a long wait into an immediate reconnect.
func TestPrefixReconnectDelayBackoff(t *testing.T) {
	ps := twoFamilyPrefixSettings(true, true)
	ps.PrefixIdleTimeout = map[string]uint16{"ipv4/unicast": 30}
	err := errors.Join(ErrConnectionClosed, &prefixLimitError{Family: "ipv4/unicast"})

	tests := []struct {
		count uint32
		want  time.Duration
	}{
		{1, 30 * time.Second},
		{2, 60 * time.Second},
		{3, 120 * time.Second},
		{7, 1920 * time.Second}, // the last value under the cap
		{8, time.Hour},          // 3840s exceeds the cap
		{60, time.Hour},         // the counter ceiling
		{500, time.Hour},        // above the ceiling, still no overflow
	}
	for _, tt := range tests {
		plan, ok := prefixReconnectDecision(ps, err, tt.count)
		require.True(t, ok, "count=%d", tt.count)
		assert.Equal(t, tt.want, plan.Delay, "count=%d", tt.count)
	}
}

// TestPrefixIdleTimeoutMaximumPerFamily verifies the uint16 ceiling survives
// the delay computation.
//
// VALIDATES: idle-timeout 65535 is clamped by the one-hour cap, not by an
// overflow.
// PREVENTS: An off-by-one at the type boundary.
func TestPrefixIdleTimeoutMaximumPerFamily(t *testing.T) {
	ps := twoFamilyPrefixSettings(true, true)
	ps.PrefixIdleTimeout = map[string]uint16{"ipv4/unicast": 65535}
	err := errors.Join(ErrConnectionClosed, &prefixLimitError{Family: "ipv4/unicast"})

	plan, ok := prefixReconnectDecision(ps, err, 1)
	require.True(t, ok)
	assert.Equal(t, 65535*time.Second, plan.Delay, "first teardown uses the raw value")

	capped, ok := prefixReconnectDecision(ps, err, 2)
	require.True(t, ok)
	assert.Equal(t, time.Hour, capped.Delay, "the doubling meets the one-hour cap")
}

// TestPrefixUpdatedAggregatesOldest verifies the peer-level date is the oldest
// of the per-family dates.
//
// VALIDATES: AC-7. The `prefix-updated` JSON key, the prefix-stale report bus
// warning and the ze_bgp_prefix_stale gauge each keep one date per peer, and
// the oldest keeps the staleness alarm firing while any family is stale.
// PREVENTS: A fresh family hiding a stale one.
func TestPrefixUpdatedAggregatesOldest(t *testing.T) {
	tests := []struct {
		name    string
		updated map[string]string
		want    string
	}{
		{"nil map", nil, ""},
		{"one family", map[string]string{"ipv4/unicast": "2026-01-01"}, "2026-01-01"},
		{
			"oldest wins regardless of key order",
			map[string]string{"ipv4/unicast": "2026-07-30", "ipv6/unicast": "2020-01-01"},
			"2020-01-01",
		},
		{
			"oldest wins when it sorts first",
			map[string]string{"ipv4/unicast": "2020-01-01", "ipv6/unicast": "2026-07-30"},
			"2020-01-01",
		},
		{
			"an empty value is not a date",
			map[string]string{"ipv4/unicast": "", "ipv6/unicast": "2026-07-30"},
			"2026-07-30",
		},
		{
			"a parseable date beats an unparseable one",
			map[string]string{"ipv4/unicast": "not-a-date", "ipv6/unicast": "2026-07-30"},
			"2026-07-30",
		},
		{
			"an unparseable value is still reported when nothing parses",
			map[string]string{"ipv4/unicast": "not-a-date"},
			"not-a-date",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &PeerSettings{PrefixUpdated: tt.updated}
			assert.Equal(t, tt.want, ps.OldestPrefixUpdated())
		})
	}
}

// TestPrefixStaleUsesOldestFamily verifies the staleness verdict follows the
// oldest family.
//
// VALIDATES: AC-7. A peer with one stale family and one fresh family is stale.
// PREVENTS: A refreshed second family silently clearing an operator alarm.
func TestPrefixStaleUsesOldestFamily(t *testing.T) {
	now := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	ps := &PeerSettings{PrefixUpdated: map[string]string{
		"ipv4/unicast": "2026-08-01", // fresh
		"ipv6/unicast": "2020-01-01", // older than the 180 day threshold
	}}

	assert.True(t, isPrefixDataStale(ps.OldestPrefixUpdated(), now),
		"one stale family makes the peer stale")
}
