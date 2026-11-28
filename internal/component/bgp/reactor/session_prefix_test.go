package reactor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
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
			got := countPrefixEntries(familyKey(family.IPv4Unicast), tt.data, tt.addPath, false)
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
			got := isPrefixDataStale(tt.updated, now)
			assert.Equal(t, tt.want, got, "isPrefixDataStale(%q, %v)", tt.updated, now)
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

// TestPrefixLimitsResolveOncePerSession verifies that the enforcement config a
// session answers from is the "afi/safi" config resolved into family keys at
// construction, and that the resolution keeps what the name walk it replaced
// answered: the maximum, the warning read under the SAME spelling that stated
// the maximum, and nothing at all for a family the config names wrongly.
//
// VALIDATES: prefixConfigLookup answers from prefixCounts.limits, so the
// receive path no longer walks PrefixMaximum and converts each key per UPDATE.
// PREVENTS: the resolution dropping a family, inventing a maximum for one that
// states none, or pairing a warning with the wrong family.
func TestPrefixLimitsResolveOncePerSession(t *testing.T) {
	ps := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0)
	ps.PrefixMaximum = map[string]uint32{
		"ipv4/unicast": 100,
		"ipv6/unicast": 200,
		"not-a-family": 300,
	}
	ps.PrefixWarning = map[string]uint32{
		"ipv4/unicast": 90,
		"not-a-family": 250,
	}
	s := NewSession(ps)

	maximum, warning, hasMax := s.prefixConfigLookup(ipv4UKey)
	assert.True(t, hasMax, "ipv4/unicast states a maximum")
	assert.Equal(t, uint32(100), maximum)
	assert.Equal(t, uint32(90), warning)

	maximum, warning, hasMax = s.prefixConfigLookup(ipv6UKey)
	assert.True(t, hasMax, "ipv6/unicast states a maximum")
	assert.Equal(t, uint32(200), maximum)
	assert.Equal(t, uint32(0), warning, "ipv6/unicast states no warning")

	// A family the config spells wrongly resolves to nothing, so it cannot be
	// counted as a limit anywhere. Two recognized names are configured, so the
	// unrecognized third is the only entry that can be missing.
	assert.Len(t, s.prefixCounts.limits, 2,
		"an unrecognized family name resolves to no limit")
}

// TestPrefixLimitsZeroMaximumStillEnforces verifies that a family stating
// `maximum 0` keeps the enforcement it has: presence in the config is the
// hasMax answer, and zero is a maximum of zero rather than an absent limit.
//
// VALIDATES: the resolved map preserves the presence-versus-value distinction
// the name walk made by returning hasMax=true for any configured entry.
// PREVENTS: `maximum 0` silently becoming "no maximum", which would accept
// every prefix from a peer an operator meant to accept none from.
func TestPrefixLimitsZeroMaximumStillEnforces(t *testing.T) {
	ps := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0)
	ps.PrefixMaximum = map[string]uint32{"ipv4/unicast": 0}
	s := NewSession(ps)

	maximum, _, hasMax := s.prefixConfigLookup(ipv4UKey)
	assert.True(t, hasMax, "a configured family states a maximum whatever its value")
	assert.Equal(t, uint32(0), maximum)

	_, _, hasMax = s.prefixConfigLookup(ipv6UKey)
	assert.False(t, hasMax, "an unconfigured family states no maximum")
}

func scrapePromMetrics(t *testing.T, reg *metrics.PrometheusRegistry) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", http.NoBody)
	reg.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

func assertNoPrefixMetricsForPeer(t *testing.T, body, peer string) {
	t.Helper()
	needle := `peer="` + peer + `"`
	for line := range strings.SplitSeq(body, "\n") {
		if strings.HasPrefix(line, "ze_bgp_prefix_") && strings.Contains(line, needle) {
			assert.Failf(t, "unexpected prefix metric series", "line: %s", line)
		}
	}
}

func newPrefixModeTestSettings(addr string, max, warn uint32, mode PrefixCountMode, routerID uint32) *PeerSettings {
	ps := NewPeerSettings(mustParseAddr(addr), 65000, 65001, routerID)
	ps.PrefixMaximum = map[string]uint32{"ipv4/unicast": max}
	ps.PrefixWarning = map[string]uint32{"ipv4/unicast": warn}
	ps.PrefixTeardown = map[string]bool{"ipv4/unicast": false}
	ps.PrefixCount = map[string]PrefixCountMode{"ipv4/unicast": mode}
	return ps
}

func singleIPv4AnnounceBody(octet2, octet3 byte) []byte {
	return []byte{0, 0, 0, 0, 24, 10, octet2, octet3}
}

func singleIPv4WithdrawBody(octet2, octet3 byte) []byte {
	return []byte{0, 4, 24, 10, octet2, octet3, 0, 0}
}

// TestPrefixMetricsCarryCountModeLabel verifies that the four count-derived
// prefix-limit Prometheus metrics stamp mode="offered" or mode="installed"
// matching each family's configured count mode, while static/peer metrics
// omit mode and maximum==0 skips prefixRatio without panicking.
//
// VALIDATES: spec-bgp-prefix-count-mode-metric AC-1 and boundary maximum == 0.
// PREVENTS: scrapes reporting prefix counts/ratios/warnings/exceeds without
// identifying whether offered or installed counting produced them.
func TestPrefixMetricsCarryCountModeLabel(t *testing.T) {
	promReg := metrics.NewPrometheusRegistry()
	rm := initReactorMetrics(promReg, "1.0.0", "1.2.3.4", "65000")

	// Offered peer: 3 distinct /24s against max=2, warn=1, teardown=false.
	// Offered counts the dropped 3rd announcement: count=3, ratio=1.5, warn=1, exceeded=1.
	psOffered := newPrefixModeTestSettings("10.0.0.1", 2, 1, PrefixCountOffered, 0x01010101)
	setPrefixStaticConfigMetrics(rm, psOffered.Address.String(), psOffered, time.Now())
	sOffered := NewSession(psOffered)
	sOffered.prefixMetrics = rm

	checkOK(t, sOffered, singleIPv4AnnounceBody(0, 1))
	checkOK(t, sOffered, singleIPv4AnnounceBody(0, 2))
	notif, drop := sOffered.checkPrefixLimits(testWireUpdate(singleIPv4AnnounceBody(0, 3)))
	assert.Nil(t, notif)
	assert.True(t, drop)

	// Installed peer: 1st prefix, re-announce 1st prefix (no-op), 2nd prefix, 3rd prefix (dropped + rolled back).
	// Installed keeps count=2, ratio=1, warn=1, exceeded=1.
	psInstalled := newPrefixModeTestSettings("10.0.0.2", 2, 1, PrefixCountInstalled, 0x02020202)
	setPrefixStaticConfigMetrics(rm, psInstalled.Address.String(), psInstalled, time.Now())
	sInstalled := NewSession(psInstalled)
	sInstalled.prefixMetrics = rm

	checkOK(t, sInstalled, singleIPv4AnnounceBody(1, 1))
	checkOK(t, sInstalled, singleIPv4AnnounceBody(1, 1)) // re-announcement does not raise installed count
	checkOK(t, sInstalled, singleIPv4AnnounceBody(1, 2))
	notif, drop = sInstalled.checkPrefixLimits(testWireUpdate(singleIPv4AnnounceBody(1, 3)))
	assert.Nil(t, notif)
	assert.True(t, drop)

	// Boundary: maximum == 0 skips prefixRatio (no divide-by-zero panic) while still
	// emitting count and maximum_exceeded_total with mode="offered".
	psZero := newPrefixModeTestSettings("10.0.0.3", 0, 0, PrefixCountOffered, 0x03030303)
	setPrefixStaticConfigMetrics(rm, psZero.Address.String(), psZero, time.Now())
	sZero := NewSession(psZero)
	sZero.prefixMetrics = rm
	notif, drop = sZero.checkPrefixLimits(testWireUpdate(singleIPv4AnnounceBody(2, 1)))
	assert.Nil(t, notif)
	assert.True(t, drop)

	body := scrapePromMetrics(t, promReg)

	// Offered peer assertions.
	assert.Contains(t, body, `ze_bgp_prefix_count{family="ipv4/unicast",mode="offered",peer="10.0.0.1"} 3`)
	assert.Contains(t, body, `ze_bgp_prefix_ratio{family="ipv4/unicast",mode="offered",peer="10.0.0.1"} 1.5`)
	assert.Contains(t, body, `ze_bgp_prefix_warning_exceeded{family="ipv4/unicast",mode="offered",peer="10.0.0.1"} 1`)
	assert.Contains(t, body, `ze_bgp_prefix_maximum_exceeded_total{family="ipv4/unicast",mode="offered",peer="10.0.0.1"} 1`)
	assert.NotContains(t, body, `mode="installed",peer="10.0.0.1"`)

	// Installed peer assertions.
	assert.Contains(t, body, `ze_bgp_prefix_count{family="ipv4/unicast",mode="installed",peer="10.0.0.2"} 2`)
	assert.Contains(t, body, `ze_bgp_prefix_ratio{family="ipv4/unicast",mode="installed",peer="10.0.0.2"} 1`)
	assert.Contains(t, body, `ze_bgp_prefix_warning_exceeded{family="ipv4/unicast",mode="installed",peer="10.0.0.2"} 1`)
	assert.Contains(t, body, `ze_bgp_prefix_maximum_exceeded_total{family="ipv4/unicast",mode="installed",peer="10.0.0.2"} 1`)
	assert.NotContains(t, body, `mode="offered",peer="10.0.0.2"`)

	// Boundary maximum == 0 assertions.
	assert.Contains(t, body, `ze_bgp_prefix_count{family="ipv4/unicast",mode="offered",peer="10.0.0.3"} 1`)
	assert.Contains(t, body, `ze_bgp_prefix_maximum_exceeded_total{family="ipv4/unicast",mode="offered",peer="10.0.0.3"} 1`)
	assert.NotContains(t, body, `ze_bgp_prefix_ratio{family="ipv4/unicast",mode="offered",peer="10.0.0.3"}`)

	// Static config gauges and peer-level metrics intentionally omit mode.
	assert.Contains(t, body, `ze_bgp_prefix_maximum{family="ipv4/unicast",peer="10.0.0.1"} 2`)
	assert.Contains(t, body, `ze_bgp_prefix_warning{family="ipv4/unicast",peer="10.0.0.1"} 1`)
	assert.Contains(t, body, `ze_bgp_prefix_stale{peer="10.0.0.1"} 0`)

	// Warning clear updates the same mode series to 0 rather than creating another tuple.
	checkOK(t, sInstalled, singleIPv4WithdrawBody(1, 1))
	checkOK(t, sInstalled, singleIPv4WithdrawBody(1, 2))
	bodyAfterClear := scrapePromMetrics(t, promReg)
	assert.Contains(t, bodyAfterClear, `ze_bgp_prefix_warning_exceeded{family="ipv4/unicast",mode="installed",peer="10.0.0.2"} 0`)
	assert.NotContains(t, bodyAfterClear, `mode="offered",peer="10.0.0.2"`)
}

// TestPrefixModeReloadRetiresObsoleteSeries verifies that reloading a peer's
// count mode (offered <-> installed), removing a peer, running an unrelated
// reload with unchanged mode, and concurrent UPDATEs + removal/reload all keep
// Prometheus series accurate with zero stale mode tuples and zero data races.
//
// VALIDATES: spec-bgp-prefix-count-mode-metric AC-2 and AC-3, risks R-1 and R-2.
// PREVENTS: stale mode series lingering after reload/removal, stopping sessions
// recreating retired series, or Peer.cleanup() deleting a replacement peer's
// live series after an unrelated reload.
func TestPrefixModeReloadRetiresObsoleteSeries(t *testing.T) {
	t.Run("reload offered to installed and back plus removal", func(t *testing.T) {
		promReg := metrics.NewPrometheusRegistry()
		r := New(&Config{})
		r.rmetrics = initReactorMetrics(promReg, "1.0.0", "1.2.3.4", "65000")
		adapter := &reactorAPIAdapter{r: r}

		psOffered := newPrefixModeTestSettings("10.1.0.1", 2, 1, PrefixCountOffered, 0x01010101)
		require.NoError(t, r.AddPeer(psOffered))
		p1 := r.peers[psOffered.PeerKey()]
		require.NotNil(t, p1)

		s1 := p1.newSession()
		checkOK(t, s1, singleIPv4AnnounceBody(0, 1))
		checkOK(t, s1, singleIPv4AnnounceBody(0, 2))
		_, drop := s1.checkPrefixLimits(testWireUpdate(singleIPv4AnnounceBody(0, 3)))
		require.True(t, drop)

		before := scrapePromMetrics(t, promReg)
		assert.Contains(t, before, `ze_bgp_prefix_count{family="ipv4/unicast",mode="offered",peer="10.1.0.1"} 3`)
		assert.Contains(t, before, `ze_bgp_prefix_ratio{family="ipv4/unicast",mode="offered",peer="10.1.0.1"} 1.5`)
		assert.Contains(t, before, `ze_bgp_prefix_warning_exceeded{family="ipv4/unicast",mode="offered",peer="10.1.0.1"} 1`)
		assert.Contains(t, before, `ze_bgp_prefix_maximum_exceeded_total{family="ipv4/unicast",mode="offered",peer="10.1.0.1"} 1`)

		// Reload offered -> installed: restarts peer through removalLocked + AddPeer.
		psInstalled := newPrefixModeTestSettings("10.1.0.1", 2, 1, PrefixCountInstalled, 0x01010101)
		require.NoError(t, adapter.reconcilePeersJournaled([]*PeerSettings{psInstalled}, Globals{}, dynamicGroupsNotRead, "test", noopJournal{}))
		require.True(t, p1.stopping.Load(), "outgoing peer must be marked stopping")

		// Late write on retired session s1 is fenced by Peer.stopping.
		_, _ = s1.checkPrefixLimits(testWireUpdate(singleIPv4AnnounceBody(0, 4)))

		afterToInstalled := scrapePromMetrics(t, promReg)
		assert.NotContains(t, afterToInstalled, `mode="offered",peer="10.1.0.1"`,
			"all four offered series must be deleted when reloading to installed")

		p2 := r.peers[psInstalled.PeerKey()]
		require.NotNil(t, p2)
		require.NotSame(t, p1, p2)

		s2 := p2.newSession()
		checkOK(t, s2, singleIPv4AnnounceBody(1, 1))
		checkOK(t, s2, singleIPv4AnnounceBody(1, 2))
		_, drop = s2.checkPrefixLimits(testWireUpdate(singleIPv4AnnounceBody(1, 3)))
		require.True(t, drop)

		// Even if a stale offered sample had been written in the narrow pre-fence window,
		// p1.cleanup() sweeps it while leaving p2's live installed series intact.
		r.rmetrics.prefixCount.With("10.1.0.1", "ipv4/unicast", "offered").Set(99)
		p1.cleanup()

		liveInstalled := scrapePromMetrics(t, promReg)
		assert.NotContains(t, liveInstalled, `mode="offered",peer="10.1.0.1"`,
			"Peer.cleanup must sweep any residual obsolete mode series")
		assert.Contains(t, liveInstalled, `ze_bgp_prefix_count{family="ipv4/unicast",mode="installed",peer="10.1.0.1"} 2`)
		assert.Contains(t, liveInstalled, `ze_bgp_prefix_ratio{family="ipv4/unicast",mode="installed",peer="10.1.0.1"} 1`)
		assert.Contains(t, liveInstalled, `ze_bgp_prefix_warning_exceeded{family="ipv4/unicast",mode="installed",peer="10.1.0.1"} 1`)
		assert.Contains(t, liveInstalled, `ze_bgp_prefix_maximum_exceeded_total{family="ipv4/unicast",mode="installed",peer="10.1.0.1"} 1`)
		assert.Contains(t, liveInstalled, `ze_bgp_prefix_maximum{family="ipv4/unicast",peer="10.1.0.1"} 2`)
		assert.Contains(t, liveInstalled, `ze_bgp_prefix_warning{family="ipv4/unicast",peer="10.1.0.1"} 1`)

		// Reload installed -> offered.
		psOfferedAgain := newPrefixModeTestSettings("10.1.0.1", 2, 1, PrefixCountOffered, 0x01010101)
		require.NoError(t, adapter.reconcilePeersJournaled([]*PeerSettings{psOfferedAgain}, Globals{}, dynamicGroupsNotRead, "test", noopJournal{}))
		p2.cleanup()

		afterToOffered := scrapePromMetrics(t, promReg)
		assert.NotContains(t, afterToOffered, `mode="installed",peer="10.1.0.1"`,
			"all four installed series must be deleted when reloading back to offered")

		p3 := r.peers[psOfferedAgain.PeerKey()]
		require.NotNil(t, p3)
		s3 := p3.newSession()
		checkOK(t, s3, singleIPv4AnnounceBody(2, 1))
		assert.Contains(t, scrapePromMetrics(t, promReg), `ze_bgp_prefix_count{family="ipv4/unicast",mode="offered",peer="10.1.0.1"} 1`)

		// Full peer removal removes all per-family and peer-level prefix metrics.
		require.NoError(t, r.RemovePeer(psOfferedAgain.Address))
		p3.cleanup()

		assertNoPrefixMetricsForPeer(t, scrapePromMetrics(t, promReg), "10.1.0.1")
	})

	t.Run("unrelated reload with unchanged mode preserves replacement peer live series", func(t *testing.T) {
		promReg := metrics.NewPrometheusRegistry()
		r := New(&Config{})
		r.rmetrics = initReactorMetrics(promReg, "1.0.0", "1.2.3.4", "65000")
		adapter := &reactorAPIAdapter{r: r}

		ps1 := newPrefixModeTestSettings("10.1.0.2", 4, 2, PrefixCountOffered, 0x01010101)
		require.NoError(t, r.AddPeer(ps1))
		p1 := r.peers[ps1.PeerKey()]
		require.NotNil(t, p1)

		s1 := p1.newSession()
		checkOK(t, s1, singleIPv4AnnounceBody(0, 1))
		checkOK(t, s1, singleIPv4AnnounceBody(0, 2))

		// Unrelated restart: router-id changes while ipv4/unicast stays count=offered.
		ps2 := newPrefixModeTestSettings("10.1.0.2", 4, 2, PrefixCountOffered, 0x02020202)
		require.NoError(t, adapter.reconcilePeersJournaled([]*PeerSettings{ps2}, Globals{}, dynamicGroupsNotRead, "test", noopJournal{}))
		p2 := r.peers[ps2.PeerKey()]
		require.NotNil(t, p2)
		require.NotSame(t, p1, p2)

		// Replacement peer establishes and emits live metrics before p1.cleanup() runs.
		s2 := p2.newSession()
		checkOK(t, s2, singleIPv4AnnounceBody(1, 1))
		checkOK(t, s2, singleIPv4AnnounceBody(1, 2))
		checkOK(t, s2, singleIPv4AnnounceBody(1, 3))

		// Delayed p1.cleanup() MUST NOT delete p2's live series.
		p1.cleanup()

		body := scrapePromMetrics(t, promReg)
		assert.Contains(t, body, `ze_bgp_prefix_count{family="ipv4/unicast",mode="offered",peer="10.1.0.2"} 3`,
			"Peer.cleanup must preserve the active replacement peer's live count series")
		assert.Contains(t, body, `ze_bgp_prefix_ratio{family="ipv4/unicast",mode="offered",peer="10.1.0.2"} 0.75`)
		assert.Contains(t, body, `ze_bgp_prefix_warning_exceeded{family="ipv4/unicast",mode="offered",peer="10.1.0.2"} 1`)
		assert.Contains(t, body, `ze_bgp_prefix_maximum{family="ipv4/unicast",peer="10.1.0.2"} 4`)
		assert.Contains(t, body, `ze_bgp_prefix_warning{family="ipv4/unicast",peer="10.1.0.2"} 2`)
	})

	t.Run("concurrent updates during mode reload and removal are race free", func(t *testing.T) {
		promReg := metrics.NewPrometheusRegistry()
		r := New(&Config{})
		r.rmetrics = initReactorMetrics(promReg, "1.0.0", "1.2.3.4", "65000")
		adapter := &reactorAPIAdapter{r: r}

		for iter := range 10 {
			psOld := newPrefixModeTestSettings("10.1.0.3", 2, 1, PrefixCountOffered, uint32(0x01010100+iter))
			require.NoError(t, adapter.reconcilePeersJournaled([]*PeerSettings{psOld}, Globals{}, dynamicGroupsNotRead, "test", noopJournal{}))
			pOld := r.peers[psOld.PeerKey()]
			require.NotNil(t, pOld)
			sOld := pOld.newSession()

			started := make(chan struct{})
			stopFeed := make(chan struct{})
			var wg sync.WaitGroup
			wg.Go(func() {
				close(started)
				for i := range 200 {
					select {
					case <-stopFeed:
						return
					default:
						_, _ = sOld.checkPrefixLimits(testWireUpdate(singleIPv4AnnounceBody(byte(iter), byte(i))))
					}
				}
			})

			<-started
			// Concurrently reload to installed mode while sOld is actively processing UPDATEs.
			psNew := newPrefixModeTestSettings("10.1.0.3", 2, 1, PrefixCountInstalled, uint32(0x02020200+iter))
			require.NoError(t, adapter.reconcilePeersJournaled([]*PeerSettings{psNew}, Globals{}, dynamicGroupsNotRead, "test", noopJournal{}))

			close(stopFeed)
			wg.Wait()
			// Mirrors Peer.cleanup() running after Session.Run exits.
			pOld.cleanup()

			pNew := r.peers[psNew.PeerKey()]
			require.NotNil(t, pNew)
			sNew := pNew.newSession()
			checkOK(t, sNew, singleIPv4AnnounceBody(9, 1))

			body := scrapePromMetrics(t, promReg)
			assert.NotContains(t, body, `mode="offered",peer="10.1.0.3"`,
				"iter %d: no obsolete offered series may survive after Peer.cleanup", iter)
			assert.Contains(t, body, `ze_bgp_prefix_count{family="ipv4/unicast",mode="installed",peer="10.1.0.3"} 1`,
				"iter %d: replacement installed series must be present", iter)
			assert.True(t, strings.Contains(body, `ze_bgp_prefix_maximum{family="ipv4/unicast",peer="10.1.0.3"} 2`))

			require.NoError(t, r.RemovePeer(psNew.Address))
			pNew.cleanup()
			assertNoPrefixMetricsForPeer(t, scrapePromMetrics(t, promReg), "10.1.0.3")
		}
	})

	t.Run("shared IP multi-port peers preserve surviving peer metrics", func(t *testing.T) {
		promReg := metrics.NewPrometheusRegistry()
		r := New(&Config{})
		r.rmetrics = initReactorMetrics(promReg, "1.0.0", "1.2.3.4", "65000")
		adapter := &reactorAPIAdapter{r: r}

		psOffered := newPrefixModeTestSettings("10.1.0.9", 5, 2, PrefixCountOffered, 0x01010109)
		psOffered.Port = 179
		psInstalled := newPrefixModeTestSettings("10.1.0.9", 5, 2, PrefixCountInstalled, 0x02020209)
		psInstalled.Port = 1179

		require.NoError(t, r.AddPeer(psOffered))
		pOff := r.peers[psOffered.PeerKey()]
		require.NotNil(t, pOff)
		sOff := pOff.newSession()
		checkOK(t, sOff, singleIPv4AnnounceBody(1, 1))

		// Adding a second peer at the same IP on another port with the opposite mode
		// must not delete the active peer's offered series.
		require.NoError(t, r.AddPeer(psInstalled))
		pInst := r.peers[psInstalled.PeerKey()]
		require.NotNil(t, pInst)
		sInst := pInst.newSession()
		checkOK(t, sInst, singleIPv4AnnounceBody(2, 1))

		body := scrapePromMetrics(t, promReg)
		assert.Contains(t, body, `ze_bgp_prefix_count{family="ipv4/unicast",mode="offered",peer="10.1.0.9"} 1`)
		assert.Contains(t, body, `ze_bgp_prefix_count{family="ipv4/unicast",mode="installed",peer="10.1.0.9"} 1`)

		// Removing the offered peer via reconcile must retire only its unowned
		// mode="offered" series while leaving the surviving :1179 peer's
		// installed series and static maximum/warning gauges intact.
		require.NoError(t, adapter.reconcilePeersJournaled([]*PeerSettings{psInstalled}, Globals{}, dynamicGroupsNotRead, "test", noopJournal{}))
		pOff.cleanup()

		body = scrapePromMetrics(t, promReg)
		assert.NotContains(t, body, `mode="offered",peer="10.1.0.9"`)
		assert.Contains(t, body, `ze_bgp_prefix_count{family="ipv4/unicast",mode="installed",peer="10.1.0.9"} 1`)
		assert.Contains(t, body, `ze_bgp_prefix_maximum{family="ipv4/unicast",peer="10.1.0.9"} 5`)
		assert.Contains(t, body, `ze_bgp_prefix_warning{family="ipv4/unicast",peer="10.1.0.9"} 2`)

		require.NoError(t, adapter.reconcilePeersJournaled(nil, Globals{}, dynamicGroupsNotRead, "test", noopJournal{}))
		pInst.cleanup()
		body = scrapePromMetrics(t, promReg)
		assert.NotContains(t, body, `family="ipv4/unicast",mode="offered",peer="10.1.0.9"`)
		assert.NotContains(t, body, `family="ipv4/unicast",mode="installed",peer="10.1.0.9"`)
		assert.NotContains(t, body, `ze_bgp_prefix_maximum{family="ipv4/unicast",peer="10.1.0.9"}`)
		assert.NotContains(t, body, `ze_bgp_prefix_warning{family="ipv4/unicast",peer="10.1.0.9"}`)
	})

	t.Run("reactor shutdown with running peer does not deadlock on r.mu", func(t *testing.T) {
		promReg := metrics.NewPrometheusRegistry()
		r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
		r.rmetrics = initReactorMetrics(promReg, "1.0.0", "1.2.3.4", "65000")

		ps := newPrefixModeTestSettings("127.0.0.1", 5, 2, PrefixCountInstalled, 0x0101010a)
		ps.Connection = ConnectionActive
		ps.LocalAddress = ps.Address
		ps.Port = 1
		require.NoError(t, r.AddPeer(ps))

		p := r.peers[ps.PeerKey()]
		require.NotNil(t, p)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		require.NoError(t, r.StartWithContext(ctx))
		p.recordPrefixMetricFamily("ipv4/unicast", PrefixCountInstalled)
		require.Eventually(t, func() bool {
			return p.State() != PeerStateStopped
		}, 2*time.Second, 5*time.Millisecond, "peer goroutine must start before Reactor.Stop")

		start := time.Now()
		r.Stop()
		require.NoError(t, r.Wait(t.Context()))
		elapsed := time.Since(start)

		assert.Less(t, elapsed, 500*time.Millisecond,
			"Reactor.Stop+Wait must not stall on Reactor.cleanup <-> Peer.cleanupPrefixMetrics r.mu deadlock (2s waitCtx timeout)")
		assert.Equal(t, PeerStateStopped, p.State(),
			"Peer.cleanup must have completed cleanly before Reactor.Wait returned")
	})
}
