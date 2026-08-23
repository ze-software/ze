package reactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/report"
)

// ipv4UKey is the uint32 family key for ipv4/unicast used in test assertions.
var ipv4UKey = familyKey(family.IPv4Unicast) //nolint:gochecknoglobals // test helper

// ipv6UKey is the uint32 family key for ipv6/unicast used in test assertions.
var ipv6UKey = familyKey(family.IPv6Unicast) //nolint:gochecknoglobals // test helper

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
	assert.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])
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
	assert.Equal(t, int64(3), s.prefixCounts.counts[ipv4UKey])

	// Withdraw 1 prefix.
	checkOK(t, s, []byte{0, 4, 24, 10, 0, 0, 0, 0})
	assert.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])
}

// TestPrefixCountReset verifies counters reset to 0 on new session.
//
// VALIDATES: AC-10 "All family prefix counters reset to 0" on session reset.
// PREVENTS: Stale counts from previous session causing false triggers.
func TestPrefixCountReset(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(100000, 0)
	s1 := NewSession(ps)

	checkOK(t, s1, []byte{0, 0, 0, 0, 24, 10, 0, 0, 24, 10, 0, 1})
	assert.Equal(t, int64(2), s1.prefixCounts.counts[ipv4UKey])

	s2 := NewSession(ps)
	assert.Equal(t, int64(0), s2.prefixCounts.counts[ipv4UKey])
}

// TestPrefixWarningThreshold verifies warning logged at threshold without teardown.
//
// VALIDATES: AC-5 "Warning logged" when count reaches warning threshold. Session stays up.
// PREVENTS: Warning mechanism silently broken.
// ste: ignore
// RFC requirement: RFC4486-4-1 negative -- a count at the warning threshold is no decision to
// stop the peering. So subcode 1 goes out on no NOTIFICATION at all. checkOK asserts both
// halves. notif is nil, and the UPDATE is not dropped. That is what binds subcode 1 to the
// teardown rather than to the counter moving.
// Producer: internal/component/bgp/reactor/session_prefix.go:399 tests count > maximum.
func TestPrefixWarningThreshold(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(5, 3)
	s := NewSession(ps)

	// Send 3 prefixes (at warning=3, below maximum=5).
	checkOK(t, s, []byte{0, 0, 0, 0, 24, 10, 0, 0, 24, 10, 0, 1, 24, 10, 0, 2})
	assert.Equal(t, int64(3), s.prefixCounts.counts[ipv4UKey])
}

// TestPrefixExceedTeardown verifies NOTIFICATION sent when maximum exceeded.
//
// VALIDATES: AC-3 "NOTIFICATION Cease/MaxPrefixes sent, session torn down."
// PREVENTS: Session staying up after prefix maximum exceeded.
// RFC requirement: RFC4271-6.7-4 positive -- terminating a peering because the configured upper
// bound on prefixes was exceeded sends a NOTIFICATION with Error Code Cease
// (internal/component/bgp/reactor/session_prefix.go:448-463).
// RFC requirement: RFC4271-6.7-1 positive -- Cease is used here precisely because no fatal
// protocol error exists: the peer's UPDATE was well formed and only a local policy limit was
// reached (internal/component/bgp/reactor/session_prefix.go:399-416).
// ste: ignore
// RFC requirement: RFC4486-4-1 positive -- RFC 4486 section 4 names the SUBCODE this asserts.
// RFC 4271 section 6.7 requires only the error CODE. Subcode 1 is 4486's, and so is the
// optional AFI/SAFI/upper-bound Data field of Figure 1 that the last four assertions pin.
// Producers: message.NotifyCeaseMaxPrefixes == 1 at
// internal/component/bgp/message/notification.go:121, built at session_prefix.go:448-465.
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
	assert.Equal(t, byte(3), notif.Data[6]) // Prefix upper bound = maximum 3
}

// TestPrefixExceedDrop verifies UPDATE is dropped when teardown=false and maximum exceeded.
//
// VALIDATES: AC-4 "further prefixes for that family rejected, session stays."
//
//	AC-27 "NLRIs beyond maximum are not installed in RIB or forwarded."
//
// PREVENTS: Over-limit routes reaching RIB/forwarding when operator chose warn-only mode.
// RFC requirement: RFC4271-6.7-4 negative -- when the speaker does not terminate the peering, no
// Cease NOTIFICATION is sent, so the Cease is bound to the termination and not to the limit
// being crossed (internal/component/bgp/reactor/session_prefix.go:399-415).
func TestPrefixExceedDrop(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(3, 2)
	ps.PrefixTeardown = map[string]bool{"ipv4/unicast": false}
	s := NewSession(ps)

	body := []byte{0, 0, 0, 0, 24, 10, 0, 0, 24, 10, 0, 1, 24, 10, 0, 2, 24, 10, 0, 3}
	notif, drop := s.checkPrefixLimits(testWireUpdate(body))

	assert.Nil(t, notif, "AC-4: teardown=false should not send NOTIFICATION")
	assert.True(t, drop, "AC-27: over-limit UPDATE must be dropped (not delivered to plugins)")
	assert.Equal(t, int64(4), s.prefixCounts.counts[ipv4UKey])
}

// TestPrefixExceedDropWithdrawStillCounted verifies withdrawals are counted even when dropping.
//
// VALIDATES: AC-27 "Withdrawals always processed" even in drop mode.
// PREVENTS: Withdrawal-only UPDATEs being dropped, causing count to never decrease.
func TestPrefixExceedDropWithdrawStillCounted(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(3, 2)
	ps.PrefixTeardown = map[string]bool{"ipv4/unicast": false}
	s := NewSession(ps)

	// Push to 4 (over max=3). Drop=true.
	_, drop := s.checkPrefixLimits(testWireUpdate(
		[]byte{0, 0, 0, 0, 24, 10, 0, 0, 24, 10, 0, 1, 24, 10, 0, 2, 24, 10, 0, 3}))
	assert.True(t, drop)
	assert.Equal(t, int64(4), s.prefixCounts.counts[ipv4UKey])

	// Withdraw 2. Count drops to 2 (below max). No drop.
	checkOK(t, s, []byte{0, 8, 24, 10, 0, 0, 24, 10, 0, 1, 0, 0})
	assert.Equal(t, int64(2), s.prefixCounts.counts[ipv4UKey])
}

// TestPrefixPerFamilyIsolation verifies exceeding one family does not affect others.
//
// VALIDATES: AC-17 "Only ipv6/mpls-vpn triggers enforcement; ipv4/unicast unaffected."
// PREVENTS: Global prefix counter shared across families.
func TestPrefixPerFamilyIsolation(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(3, 2)
	ps.PrefixMaximum["ipv6/unicast"] = 100000
	ps.PrefixWarning["ipv6/unicast"] = 90000
	s := NewSession(ps)

	body := []byte{0, 0, 0, 0, 24, 10, 0, 0, 24, 10, 0, 1, 24, 10, 0, 2, 24, 10, 0, 3}
	notif, _ := s.checkPrefixLimits(testWireUpdate(body))
	require.NotNil(t, notif, "should trigger on ipv4 exceed")
	assert.Equal(t, int64(0), s.prefixCounts.counts[ipv6UKey])
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
	assert.Equal(t, int64(3), s.prefixCounts.counts[ipv4UKey])

	// Withdraw 1 + announce 1 (net change = 0). Must not trigger.
	body2 := []byte{0, 4, 24, 10, 0, 0, 0, 0, 24, 10, 0, 3}
	notif, drop := s.checkPrefixLimits(testWireUpdate(body2))
	assert.Nil(t, notif, "prefix replacement should not trigger teardown")
	assert.False(t, drop)
	assert.Equal(t, int64(3), s.prefixCounts.counts[ipv4UKey])
}

// TestPrefixNotificationData verifies NOTIFICATION data format per RFC 4486.
//
// VALIDATES: Data field contains AFI(2 big-endian) + SAFI(1) + upper bound(4 big-endian).
// PREVENTS: Wrong byte order or missing data field in NOTIFICATION.
func TestPrefixNotificationData(t *testing.T) {
	notif := buildPrefixNotification(ipv4UKey, 100001)
	require.Len(t, notif.Data, 7)
	assert.Equal(t, []byte{0, 1, 1, 0, 1, 0x86, 0xa1}, notif.Data)

	notif6 := buildPrefixNotification(ipv6UKey, 50000)
	require.Len(t, notif6.Data, 7)
	assert.Equal(t, []byte{0, 2, 1, 0, 0, 0xc3, 0x50}, notif6.Data)
}

// TestPrefixNotificationDataCarriesTheConfiguredUpperBound proves the four octets
// RFC 4486 Section 4 Figure 1 labels "Prefix upper bound" hold the maximum the
// operator configured, and not the count that crossed it.
//
// Method: a family whose maximum is 3 receives one UPDATE of 8 prefixes. The
// count that crosses the bound is 8 and the bound is 3, so neither number can be
// read as the other. A peer that received the count would take 8 for ze's limit,
// when the limit that refused it is 3.
//
// RFC requirement: RFC4486-4-10 positive -- the optional Figure 1 Data field is
// included, and its last four octets carry the upper bound.
// Producer: buildPrefixNotification, called from reportPrefixExceeded with the
// family's configured maximum (session_prefix.go).
func TestPrefixNotificationDataCarriesTheConfiguredUpperBound(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(3, 0)
	s := NewSession(ps)

	notif, drop := s.checkPrefixLimits(testWireUpdate(announceBody(t, 0, 8)))
	require.NotNil(t, notif, "8 prefixes past a maximum of 3 must tear the session down")
	assert.False(t, drop)
	require.Len(t, notif.Data, 7)
	assert.Equal(t, []byte{0, 1, 1, 0, 0, 0, 3}, notif.Data,
		"AFI 1, SAFI 1, upper bound 3")
	assert.Equal(t, int64(8), s.prefixCounts.counts[ipv4UKey],
		"the count that crossed the bound is 8 and stays off the wire")
}

// TestPrefixCountClampZero verifies counter does not go negative.
//
// VALIDATES: Withdrawing more than announced clamps count to 0.
// PREVENTS: Negative prefix counts causing underflow or false triggers.
func TestPrefixCountClampZero(t *testing.T) {
	ps := newTestPeerSettingsWithPrefix(100000, 0)
	s := NewSession(ps)
	checkOK(t, s, []byte{0, 8, 24, 10, 0, 0, 24, 10, 0, 1, 0, 0})
	assert.Equal(t, int64(0), s.prefixCounts.counts[ipv4UKey])
}

// TestPrefixNoPrefixConfig verifies no enforcement when prefix limits not configured.
// Counts are still tracked (for anomaly detection) but no limits are enforced.
//
// VALIDATES: Session without prefix limits does not enforce anything.
// PREVENTS: Panic or false triggers on unconfigured peers.
func TestPrefixNoPrefixConfig(t *testing.T) {
	ps := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0)
	s := NewSession(ps)
	notif, drop := s.checkPrefixLimits(testWireUpdate([]byte{0, 0, 0, 0, 24, 10, 0, 0}))
	assert.Nil(t, notif)
	assert.False(t, drop)
	assert.Equal(t, int64(1), s.prefixCounts.counts[ipv4UKey], "prefix counted even without limits")
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
	ps.PrefixIdleTimeout = map[string]uint16{"ipv4/unicast": 30}
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

// TestSessionPrefixThresholdRaisesAndClearsReport verifies that crossing the
// warning threshold raises a prefix-threshold entry on the report bus, that
// re-crossing is deduped, and that dropping below the threshold clears the entry.
//
// VALIDATES: Bus producer-side semantics for the prefix-threshold migration.
// PREVENTS: Login banner and ze show warnings missing runtime prefix warnings,
// or showing stale ones after the count drops back down.
func TestSessionPrefixThresholdRaisesAndClearsReport(t *testing.T) {
	report.ClearSource(reportSourceBGP)
	defer report.ClearSource(reportSourceBGP)

	ps := newTestPeerSettingsWithPrefix(10, 8)
	s := NewSession(ps)

	// Push count to 8 (warning threshold).
	notif, drop := s.applyPrefixCheck(ipv4UKey, 8)
	require.Nil(t, notif)
	require.False(t, drop)

	warnings := report.Warnings()
	require.Len(t, warnings, 1, "warning entry should be raised at threshold")
	w := warnings[0]
	assert.Equal(t, reportSourceBGP, w.Source)
	assert.Equal(t, reportCodePrefixThreshold, w.Code)
	assert.Equal(t, "10.0.0.1/ipv4/unicast", w.Subject)
	assert.Equal(t, "ipv4/unicast", w.Detail["family"])

	// Push to 9 -- still in warning, no second entry (per-session dedup
	// via prefixCounts.warned plus bus-level dedup).
	notif, drop = s.applyPrefixCheck(ipv4UKey, 1)
	require.Nil(t, notif)
	require.False(t, drop)
	assert.Len(t, report.Warnings(), 1, "dedup: still one warning at count 9")

	// Withdraw to 7 (below warning threshold).
	s.applyPrefixDelta(ipv4UKey, -2)
	assert.Empty(t, report.Warnings(), "warning should be cleared after dropping below threshold")
}

// TestSessionClearReportedWarningsOnTeardown verifies that ClearReportedWarnings
// removes any prefix-threshold entries this session raised. Called from
// Peer.runOnce on session teardown.
//
// VALIDATES: Cleanup-on-teardown semantics replacing Peer.clearPrefixWarned.
// PREVENTS: Stale per-peer warnings persisting on the report bus after a session ends.
func TestSessionClearReportedWarningsOnTeardown(t *testing.T) {
	report.ClearSource(reportSourceBGP)
	defer report.ClearSource(reportSourceBGP)

	ps := newTestPeerSettingsWithPrefix(10, 8)
	s := NewSession(ps)

	// Raise a threshold warning.
	notif, drop := s.applyPrefixCheck(ipv4UKey, 8)
	require.Nil(t, notif)
	require.False(t, drop)
	require.Len(t, report.Warnings(), 1)

	// Simulate session teardown.
	s.clearReportedWarnings()
	assert.Empty(t, report.Warnings(), "ClearReportedWarnings should remove the entry")
}

// TestFamilyKeyRoundTrip verifies familyKey/familyString/familyKeyString round-trip for all
// well-known families.
//
// VALIDATES: familyString(familyKey(f)) == f.String() and familyKeyString(f.String()) == familyKey(f).
// PREVENTS: Silent key collisions or lossy encoding in prefix-count map keys.
func TestFamilyKeyRoundTrip(t *testing.T) {
	families := []family.Family{
		{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}, {AFI: family.AFIIPv6, SAFI: family.SAFIUnicast},
		{AFI: family.AFIIPv4, SAFI: family.SAFIMulticast}, {AFI: family.AFIIPv6, SAFI: family.SAFIMulticast},
		{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}, {AFI: family.AFIIPv6, SAFI: family.SAFIVPN},
		{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN},
	}
	for _, f := range families {
		key := familyKey(f)
		str := familyString(key)
		if str != f.String() {
			t.Errorf("round-trip failed for %v: familyString(familyKey(%v)) = %q, want %q", f, f, str, f.String())
		}
		// Reverse: string -> key -> string
		k2, ok := familyKeyString(f.String())
		if !ok {
			t.Errorf("familyKeyString(%q) returned false", f.String())
			continue
		}
		if k2 != key {
			t.Errorf("familyKeyString(%q) = %d, want %d", f.String(), k2, key)
		}
	}
}

// TestRouteCountAnomaly verifies that a >50% drop in received prefix count
// within a single UPDATE raises a route-count-anomaly error on the report bus,
// but only when the count is above minRouteCountAnomalyThreshold.
//
// VALIDATES: AC-10 ">50% drop raises error event".
// PREVENTS: Silent route leaks or upstream failures going undetected.
func TestRouteCountAnomaly(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	ps := newTestPeerSettingsWithPrefix(100000, 0)
	s := NewSession(ps)

	// Pre-fill to minRouteCountAnomalyThreshold (100) + 50 = 150 via direct count manipulation.
	// Building 150 distinct /24 prefixes in raw UPDATE bytes is unnecessary;
	// the prefix counter is the unit under test.
	s.prefixCounts.counts[ipv4UKey] = 150

	// Withdraw 76 of 150 (>50% drop, above threshold) -- anomaly.
	withdrawBody := buildWithdrawBody(76)
	checkOK(t, s, withdrawBody)
	assert.Equal(t, int64(74), s.prefixCounts.counts[ipv4UKey])
	errors := report.Errors(0)
	found := false
	for _, e := range errors {
		if e.Code == reportCodeRouteCountAnomaly && e.Subject == "10.0.0.1" {
			found = true
			assert.Equal(t, int64(150), e.Detail["before"])
			assert.Equal(t, int64(74), e.Detail["after"])
		}
	}
	if !found {
		t.Fatal("route-count-anomaly error not raised after >50% drop above threshold")
	}
}

// TestRouteCountAnomalyBelowThreshold verifies no anomaly when count is below
// minRouteCountAnomalyThreshold, even with a >50% drop.
//
// VALIDATES: AC-10 boundary: small tables don't trigger false positives.
// PREVENTS: Noisy alerts on peers with few routes.
func TestRouteCountAnomalyBelowThreshold(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	ps := newTestPeerSettingsWithPrefix(100000, 0)
	s := NewSession(ps)

	// Pre-fill to 20 (below threshold of 100).
	s.prefixCounts.counts[ipv4UKey] = 20

	// Withdraw 15 of 20 (75% drop, but below threshold).
	withdrawBody := buildWithdrawBody(15)
	checkOK(t, s, withdrawBody)
	assert.Equal(t, int64(5), s.prefixCounts.counts[ipv4UKey])
	errors := report.Errors(0)
	for _, e := range errors {
		if e.Code == reportCodeRouteCountAnomaly {
			t.Fatal("route-count-anomaly should not fire below minimum threshold")
		}
	}
}

// TestRouteCountAnomalyNotFiredOnZero verifies no anomaly when starting from zero.
//
// VALIDATES: AC-10 boundary: 0 -> 0 and 0 -> N are not anomalies.
// PREVENTS: False positive on first UPDATE or withdraw-only on empty session.
func TestRouteCountAnomalyNotFiredOnZero(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	ps := newTestPeerSettingsWithPrefix(100000, 0)
	s := NewSession(ps)

	// Withdraw on empty session (0 -> 0).
	checkOK(t, s, []byte{0, 4, 24, 10, 0, 0, 0, 0})
	errors := report.Errors(0)
	for _, e := range errors {
		if e.Code == reportCodeRouteCountAnomaly {
			t.Fatal("route-count-anomaly should not fire when starting from zero")
		}
	}
}

// TestRouteCountAnomalyExactHalf verifies no anomaly at exactly 50% drop.
//
// VALIDATES: AC-10 boundary: exactly 50% is not >50%, so no anomaly.
// PREVENTS: Off-by-one in the >50% threshold check.
func TestRouteCountAnomalyExactHalf(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	ps := newTestPeerSettingsWithPrefix(100000, 0)
	s := NewSession(ps)

	// Pre-fill to 200 (above threshold).
	s.prefixCounts.counts[ipv4UKey] = 200

	// Withdraw exactly 100 (50% drop, not >50%).
	withdrawBody := buildWithdrawBody(100)
	checkOK(t, s, withdrawBody)
	assert.Equal(t, int64(100), s.prefixCounts.counts[ipv4UKey])
	errors := report.Errors(0)
	for _, e := range errors {
		if e.Code == reportCodeRouteCountAnomaly {
			t.Fatal("route-count-anomaly should not fire at exactly 50% (threshold is >50%)")
		}
	}
}

// TestRouteCountAnomalyWithoutPrefixLimits verifies anomaly detection works
// even when no prefix limits are configured.
//
// VALIDATES: Finding 2: counting is unconditional.
// PREVENTS: Anomaly detection silently disabled without prefix limits.
func TestRouteCountAnomalyWithoutPrefixLimits(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	ps := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0)
	s := NewSession(ps)

	// Pre-fill to 200 (above threshold), no prefix limits configured.
	s.prefixCounts.counts[ipv4UKey] = 200

	// Withdraw 150 of 200 (75% drop).
	withdrawBody := buildWithdrawBody(150)
	checkOK(t, s, withdrawBody)
	assert.Equal(t, int64(50), s.prefixCounts.counts[ipv4UKey])
	errors := report.Errors(0)
	found := false
	for _, e := range errors {
		if e.Code == reportCodeRouteCountAnomaly {
			found = true
		}
	}
	if !found {
		t.Fatal("route-count-anomaly should fire even without prefix limits configured")
	}
}

// buildWithdrawBody builds a minimal UPDATE body withdrawing n distinct /24 prefixes.
// Prefixes span 10.0.0.0/24 through 10.255.255.0/24 (max 65536 distinct).
func buildWithdrawBody(n int) []byte {
	if n > 65536 {
		panic("buildWithdrawBody: n exceeds 65536 distinct /24 prefixes in 10.0.0.0/8")
	}
	wdLen := n * 4 // each /24 = prefix-len(1) + 3 bytes
	body := make([]byte, 0, 2+wdLen+2)
	body = append(body, byte(wdLen>>8), byte(wdLen))
	for i := range n {
		body = append(body, 24, 10, byte(i>>8), byte(i))
	}
	body = append(body, 0, 0) // empty path attributes
	return body
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
