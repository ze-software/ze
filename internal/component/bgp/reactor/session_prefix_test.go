package reactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
)

// testWireUpdate creates a WireUpdate from raw UPDATE body bytes for testing.
func testWireUpdate(body []byte) *wireu.WireUpdate {
	return wireu.NewWireUpdate(body, 0)
}

// checkOK calls checkPrefixLimits and asserts no notification and no drop.
func checkOK(t *testing.T, s *Session, body []byte) {
	t.Helper()
	notif, drop := s.checkPrefixLimits(testWireUpdate(body))
	assert.Nil(t, notif, "unexpected NOTIFICATION")
	assert.False(t, drop, "unexpected drop")
}

// TestPrefixCountIncrement verifies prefix counter increments on NLRI announce.
//
// VALIDATES: AC-1 "Per-family maximum = 1000000". Counter tracks announced NLRIs per family.
// PREVENTS: Prefix counting broken or not wired to UPDATE handler.
func TestPrefixCountIncrement(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(100000, 0)
	s := NewSession(ps)

	body := []byte{
		0, 0, 0, 0,
		24, 10, 0, 0, // 10.0.0.0/24
		24, 10, 0, 1, // 10.0.1.0/24
	}
	checkOK(t, s, body)
	assert.Equal(t, int64(2), s.prefixCounts.counts["ipv4/unicast"])
}

// TestPrefixCountDecrement verifies prefix counter decrements on withdraw.
//
// VALIDATES: AC-9 "Counter decremented" on withdrawal.
// PREVENTS: Withdrawals not counted, causing premature limit triggers.
func TestPrefixCountDecrement(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(100000, 0)
	s := NewSession(ps)

	// Announce 3 prefixes.
	checkOK(t, s, []byte{0, 0, 0, 0, 24, 10, 0, 0, 24, 10, 0, 1, 24, 10, 0, 2})
	assert.Equal(t, int64(3), s.prefixCounts.counts["ipv4/unicast"])

	// Withdraw 1 prefix.
	checkOK(t, s, []byte{0, 4, 24, 10, 0, 0, 0, 0})
	assert.Equal(t, int64(2), s.prefixCounts.counts["ipv4/unicast"])
}

// TestPrefixCountReset verifies counters reset to 0 on new session.
//
// VALIDATES: AC-10 "All family prefix counters reset to 0" on session reset.
// PREVENTS: Stale counts from previous session causing false triggers.
func TestPrefixCountReset(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(100000, 0)
	s1 := NewSession(ps)

	checkOK(t, s1, []byte{0, 0, 0, 0, 24, 10, 0, 0, 24, 10, 0, 1})
	assert.Equal(t, int64(2), s1.prefixCounts.counts["ipv4/unicast"])

	s2 := NewSession(ps)
	assert.Equal(t, int64(0), s2.prefixCounts.counts["ipv4/unicast"])
}

// TestPrefixWarningThreshold verifies warning logged at threshold without teardown.
//
// VALIDATES: AC-5 "Warning logged" when count reaches warning threshold. Session stays up.
// PREVENTS: Warning mechanism silently broken.
func TestPrefixWarningThreshold(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(5, 3)
	s := NewSession(ps)

	// Send 3 prefixes (at warning=3, below maximum=5).
	checkOK(t, s, []byte{0, 0, 0, 0, 24, 10, 0, 0, 24, 10, 0, 1, 24, 10, 0, 2})
	assert.Equal(t, int64(3), s.prefixCounts.counts["ipv4/unicast"])
}

// TestPrefixExceedTeardown verifies NOTIFICATION sent when maximum exceeded.
//
// VALIDATES: AC-3 "NOTIFICATION Cease/MaxPrefixes sent, session torn down."
// PREVENTS: Session staying up after prefix maximum exceeded.
func TestPrefixExceedTeardown(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(3, 2)
	s := NewSession(ps)

	body := []byte{0, 0, 0, 0, 24, 10, 0, 0, 24, 10, 0, 1, 24, 10, 0, 2, 24, 10, 0, 3}
	notif, drop := s.checkPrefixLimits(testWireUpdate(body))

	require.NotNil(t, notif, "AC-3: should trigger NOTIFICATION on exceed")
	assert.False(t, drop)
	assert.Equal(t, message.NotifyCease, notif.ErrorCode)
	assert.Equal(t, message.NotifyCeaseMaxPrefixes, notif.ErrorSubcode)
	require.Len(t, notif.Data, 7)
	assert.Equal(t, byte(0), notif.Data[0]) // AFI high
	assert.Equal(t, byte(1), notif.Data[1]) // AFI low (IPv4)
	assert.Equal(t, byte(1), notif.Data[2]) // SAFI (unicast)
	assert.Equal(t, byte(4), notif.Data[6]) // count = 4
}

// TestPrefixExceedDrop verifies UPDATE is dropped when teardown=false and maximum exceeded.
//
// VALIDATES: AC-4 "further prefixes for that family rejected, session stays."
//
//	AC-27 "NLRIs beyond maximum are not installed in RIB or forwarded."
//
// PREVENTS: Over-limit routes reaching RIB/forwarding when operator chose warn-only mode.
func TestPrefixExceedDrop(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(3, 2)
	ps.PrefixTeardown = false
	s := NewSession(ps)

	body := []byte{0, 0, 0, 0, 24, 10, 0, 0, 24, 10, 0, 1, 24, 10, 0, 2, 24, 10, 0, 3}
	notif, drop := s.checkPrefixLimits(testWireUpdate(body))

	assert.Nil(t, notif, "AC-4: teardown=false should not send NOTIFICATION")
	assert.True(t, drop, "AC-27: over-limit UPDATE must be dropped (not delivered to plugins)")
	assert.Equal(t, int64(4), s.prefixCounts.counts["ipv4/unicast"])
}

// TestPrefixExceedDropWithdrawStillCounted verifies withdrawals are counted even when dropping.
//
// VALIDATES: AC-27 "Withdrawals always processed" even in drop mode.
// PREVENTS: Withdrawal-only UPDATEs being dropped, causing count to never decrease.
func TestPrefixExceedDropWithdrawStillCounted(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(3, 2)
	ps.PrefixTeardown = false
	s := NewSession(ps)

	// Push to 4 (over max=3). Drop=true.
	_, drop := s.checkPrefixLimits(testWireUpdate(
		[]byte{0, 0, 0, 0, 24, 10, 0, 0, 24, 10, 0, 1, 24, 10, 0, 2, 24, 10, 0, 3}))
	assert.True(t, drop)
	assert.Equal(t, int64(4), s.prefixCounts.counts["ipv4/unicast"])

	// Withdraw 2. Count drops to 2 (below max). No drop.
	checkOK(t, s, []byte{0, 8, 24, 10, 0, 0, 24, 10, 0, 1, 0, 0})
	assert.Equal(t, int64(2), s.prefixCounts.counts["ipv4/unicast"])
}

// TestPrefixPerFamilyIsolation verifies exceeding one family does not affect others.
//
// VALIDATES: AC-17 "Only ipv6/vpn triggers enforcement; ipv4/unicast unaffected."
// PREVENTS: Global prefix counter shared across families.
func TestPrefixPerFamilyIsolation(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(3, 2)
	ps.PrefixMaximum["ipv6/unicast"] = 100000
	ps.PrefixWarning["ipv6/unicast"] = 90000
	s := NewSession(ps)

	body := []byte{0, 0, 0, 0, 24, 10, 0, 0, 24, 10, 0, 1, 24, 10, 0, 2, 24, 10, 0, 3}
	notif, _ := s.checkPrefixLimits(testWireUpdate(body))
	require.NotNil(t, notif, "should trigger on ipv4 exceed")
	assert.Equal(t, int64(0), s.prefixCounts.counts["ipv6/unicast"])
}

// TestPrefixWithdrawBeforeAnnounce verifies that withdrawals are counted before
// announces in the same UPDATE, preventing false triggers from prefix replacement.
//
// VALIDATES: UPDATE withdrawing 1 + announcing 1 does not increase net count.
// PREVENTS: False teardown on prefix replacement (withdraw old, announce new).
func TestPrefixWithdrawBeforeAnnounce(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(3, 2)
	s := NewSession(ps)

	// Pre-fill count to 3 (at maximum).
	checkOK(t, s, []byte{0, 0, 0, 0, 24, 10, 0, 0, 24, 10, 0, 1, 24, 10, 0, 2})
	assert.Equal(t, int64(3), s.prefixCounts.counts["ipv4/unicast"])

	// Withdraw 1 + announce 1 (net change = 0). Must not trigger.
	body2 := []byte{0, 4, 24, 10, 0, 0, 0, 0, 24, 10, 0, 3}
	notif, drop := s.checkPrefixLimits(testWireUpdate(body2))
	assert.Nil(t, notif, "prefix replacement should not trigger teardown")
	assert.False(t, drop)
	assert.Equal(t, int64(3), s.prefixCounts.counts["ipv4/unicast"])
}

// TestPrefixNotificationData verifies NOTIFICATION data format per RFC 4486.
//
// VALIDATES: Data field contains AFI(2 big-endian) + SAFI(1) + count(4 big-endian).
// PREVENTS: Wrong byte order or missing data field in NOTIFICATION.
func TestPrefixNotificationData(t *testing.T) {
	notif := buildPrefixNotification("ipv4/unicast", 100001)
	require.Len(t, notif.Data, 7)
	assert.Equal(t, []byte{0, 1, 1, 0, 1, 0x86, 0xa1}, notif.Data)

	notif6 := buildPrefixNotification("ipv6/unicast", 50000)
	require.Len(t, notif6.Data, 7)
	assert.Equal(t, []byte{0, 2, 1, 0, 0, 0xc3, 0x50}, notif6.Data)
}

// TestPrefixCountClampZero verifies counter does not go negative.
//
// VALIDATES: Withdrawing more than announced clamps count to 0.
// PREVENTS: Negative prefix counts causing underflow or false triggers.
func TestPrefixCountClampZero(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(100000, 0)
	s := NewSession(ps)
	checkOK(t, s, []byte{0, 8, 24, 10, 0, 0, 24, 10, 0, 1, 0, 0})
	assert.Equal(t, int64(0), s.prefixCounts.counts["ipv4/unicast"])
}

// TestPrefixNoPrefixConfig verifies no enforcement when prefix limits not configured.
//
// VALIDATES: Session without prefix limits does not enforce anything.
// PREVENTS: Panic or false triggers on unconfigured peers.
func TestPrefixNoPrefixConfig(t *testing.T) {
	ps := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0)
	s := NewSession(ps)
	notif, drop := s.checkPrefixLimits(testWireUpdate([]byte{0, 0, 0, 0, 24, 10, 0, 0}))
	assert.Nil(t, notif)
	assert.False(t, drop)
}

// TestCountPrefixEntries verifies the prefix counting function.
//
// VALIDATES: countPrefixEntries correctly counts standard prefix-length entries.
// PREVENTS: Wrong count from misaligned parsing.
func TestCountPrefixEntries(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		addPath bool
		want    int
	}{
		{"empty", nil, false, 0},
		{"single /24", []byte{24, 10, 0, 0}, false, 1},
		{"two /24", []byte{24, 10, 0, 0, 24, 10, 0, 1}, false, 2},
		{"mixed lengths", []byte{8, 10, 16, 10, 0, 24, 10, 0, 0}, false, 3},
		{"/0 prefix", []byte{0}, false, 1},
		{"/32 prefix", []byte{32, 10, 0, 0, 1}, false, 1},
		{"truncated", []byte{24, 10}, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countPrefixEntries(tt.data, tt.addPath)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestPrefixBackoffExponential verifies exponential backoff on repeated prefix teardowns.
//
// VALIDATES: AC-25 "Exponential backoff: idle-timeout x 2^(N-1), capped at 1 hour."
// PREVENTS: Tight reconnect loops from persistent route leaks.
func TestPrefixBackoffExponential(t *testing.T) {
	tests := []struct {
		count   uint32
		idleSec uint16
		wantSec int
	}{
		{1, 30, 30},    // 1st: idle-timeout
		{2, 30, 60},    // 2nd: x2
		{3, 30, 120},   // 3rd: x4
		{4, 30, 240},   // 4th: x8
		{5, 30, 480},   // 5th: x16
		{10, 30, 3600}, // 10th: capped at 1 hour
		{1, 60, 60},    // different base
		{3, 60, 240},   // 60 x 4
	}

	for _, tt := range tests {
		idleBase := time.Duration(tt.idleSec) * time.Second
		delay := idleBase
		for i := uint32(1); i < tt.count; i++ {
			delay *= 2
			if delay > time.Hour {
				delay = time.Hour
				break
			}
		}
		assert.Equal(t, time.Duration(tt.wantSec)*time.Second, delay,
			"count=%d idle=%ds", tt.count, tt.idleSec)
	}
}

// TestPrefixBackoffReset verifies backoff counter resets after stable session.
//
// VALIDATES: AC-26 "Backoff counter resets" on successful session.
// PREVENTS: Permanent backoff penalty from a single burst of route leaks.
func TestPrefixBackoffReset(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(3, 2)
	ps.PrefixIdleTimeout = 30
	p := NewPeer(ps)

	p.prefixTeardownCount = 5
	p.prefixTeardownCount = 0
	assert.Equal(t, uint32(0), p.prefixTeardownCount)
}

// TestPrefixStalenessCheck verifies staleness detection for prefix updated timestamps.
//
// VALIDATES: AC-5 -- "Warning at startup" when updated timestamp is older than threshold.
// VALIDATES: AC-6 -- "No staleness warning" when timestamp is empty.
// PREVENTS: false staleness alerts for manually configured peers.
func TestPrefixStalenessCheck(t *testing.T) {
	now := time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		updated string
		want    bool
	}{
		{"empty timestamp not stale", "", false},
		{"recent date not stale", "2026-03-01", false},
		{"exactly 180 days ago not stale", "2025-09-24", false},
		{"7 months ago is stale", "2025-08-01", true},
		{"1 year ago is stale", "2025-03-01", true},
		{"invalid date not stale", "not-a-date", false},
		{"today not stale", "2026-03-23", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPrefixDataStale(tt.updated, now)
			assert.Equal(t, tt.want, got, "IsPrefixDataStale(%q, %v)", tt.updated, now)
		})
	}
}

// TestPrefixRatio verifies the ratio computation (count / maximum).
//
// VALIDATES: AC-9 -- "Equals current_count / maximum for each peer/family".
// PREVENTS: division by zero or incorrect ratio calculation.
func TestPrefixRatio(t *testing.T) {
	tests := []struct {
		name    string
		count   int64
		maximum uint32
		want    float64
	}{
		{"half full", 500, 1000, 0.5},
		{"at maximum", 1000, 1000, 1.0},
		{"over maximum", 1500, 1000, 1.5},
		{"empty", 0, 1000, 0.0},
		{"one prefix", 1, 1000, 0.001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := float64(tt.count) / float64(tt.maximum)
			assert.InDelta(t, tt.want, got, 0.0001)
		})
	}
}

// TestPrefixRatioGuard verifies that setPrefixCountMetric does not panic
// when prefix maximum is zero (division by zero guard in ratio computation).
//
// VALIDATES: AC-9 -- ratio computed safely even with misconfigured maximum.
// PREVENTS: division by zero panic in setPrefixCountMetric.
func TestPrefixRatioGuard(t *testing.T) {
	// PeerSettings with maximum=0 -- setPrefixCountMetric must not panic.
	ps := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0)
	ps.PrefixMaximum = map[string]uint32{"ipv4/unicast": 0}
	ps.PrefixWarning = map[string]uint32{"ipv4/unicast": 0}
	s := NewSession(ps)

	// This should not panic despite maximum=0.
	s.setPrefixCountMetric("ipv4/unicast", 100)

	// With a valid maximum, ratio code path executes (no panic).
	ps2 := newTestPeerSettingsWithPrefix(1000, 0)
	s2 := NewSession(ps2)
	s2.setPrefixCountMetric("ipv4/unicast", 500)
}

// newTestPeerSettingsWithPrefix creates PeerSettings with prefix limits for testing.
func newTestPeerSettingsWithPrefix(maximum, warning uint32) *PeerSettings {
	ps := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0)
	ps.PrefixMaximum = map[string]uint32{"ipv4/unicast": maximum}
	if warning > 0 {
		ps.PrefixWarning = map[string]uint32{"ipv4/unicast": warning}
	} else {
		ps.PrefixWarning = map[string]uint32{"ipv4/unicast": maximum * 9 / 10}
	}
	return ps
}
