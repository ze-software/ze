package web

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/chaos/peer"
)

// TestRouteMatrixRecordAndGet verifies basic record/retrieve for route matrix.
//
// VALIDATES: RecordSent + RecordReceived creates cells, Get retrieves them.
// PREVENTS: Missing route flow data in heatmap.
func TestRouteMatrixRecordAndGet(t *testing.T) {
	t.Parallel()

	m := NewRouteMatrix()
	p1 := netip.MustParsePrefix("10.0.0.0/24")
	p2 := netip.MustParsePrefix("10.0.1.0/24")

	now := time.Now()
	m.RecordSent(0, p1, now) // peer 0 announces 10.0.0.0/24
	m.RecordSent(0, p2, now) // peer 0 announces 10.0.1.0/24

	// peer 1 receives both prefixes from peer 0
	recvTime := now.Add(5 * time.Millisecond)
	if lat := m.RecordReceived(1, p1, recvTime); lat == 0 {
		t.Error("RecordReceived should return non-zero latency for known prefix")
	}
	if lat := m.RecordReceived(1, p2, recvTime); lat == 0 {
		t.Error("RecordReceived should return non-zero latency for known prefix")
	}

	if got := m.Get(0, 1); got != 2 {
		t.Errorf("Get(0,1) = %d, want 2", got)
	}
	if got := m.Get(1, 0); got != 0 {
		t.Errorf("Get(1,0) = %d, want 0 (no reverse flow)", got)
	}
}

// TestRouteMatrixUnknownPrefix verifies RecordReceived returns false for unknown prefix.
//
// VALIDATES: Unknown prefixes don't create matrix cells.
// PREVENTS: Ghost entries from prefixes never announced.
func TestRouteMatrixUnknownPrefix(t *testing.T) {
	t.Parallel()

	m := NewRouteMatrix()
	p := netip.MustParsePrefix("10.0.0.0/24")

	if lat := m.RecordReceived(1, p, time.Now()); lat != 0 {
		t.Error("RecordReceived should return zero for unknown prefix")
	}
	if m.Len() != 0 {
		t.Errorf("Len() = %d, want 0", m.Len())
	}
}

// TestRouteMatrixCreditFallback verifies that receives without a direct
// prefix match fall back to credit-based matching using family send counts.
//
// VALIDATES: When a prefix isn't in routeOrigins, RecordReceived uses the
// credit mechanism to attribute the receive to the best sender.
// PREVENTS: Empty matrix cells when prefix tracking has been evicted or
// when receives arrive before sends.
func TestRouteMatrixCreditFallback(t *testing.T) {
	t.Parallel()

	m := NewRouteMatrix()
	now := time.Now()

	// p0 sends 3 IPv4 prefixes — tracked in both routeOrigins and familySent.
	for i := range 3 {
		p := netip.MustParsePrefix("10.0." + itoa(i) + ".0/24")
		m.RecordSent(0, p, now)
	}

	// p1 receives a prefix that IS in routeOrigins → direct match.
	lat := m.RecordReceived(1, netip.MustParsePrefix("10.0.0.0/24"), now.Add(5*time.Millisecond))
	if lat == 0 {
		t.Error("direct match should return non-zero latency")
	}
	if got := m.Get(0, 1); got != 1 {
		t.Errorf("Get(0,1) after direct = %d, want 1", got)
	}

	// p2 receives a prefix NOT in routeOrigins → credit fallback.
	unknown := netip.MustParsePrefix("10.99.0.0/24")
	lat = m.RecordReceived(2, unknown, now.Add(10*time.Millisecond))
	// Credit match returns 0 latency (no timing info).
	if lat != 0 {
		t.Errorf("credit match should return 0 latency, got %v", lat)
	}
	if got := m.Get(0, 2); got != 1 {
		t.Errorf("Get(0,2) after credit = %d, want 1", got)
	}
}

// TestRouteMatrixDirectMatchConsumesCredit verifies that prefix-matched
// receives consume credits so credit fallback doesn't double-count them
// after prefix eviction.
//
// VALIDATES: Direct prefix matches decrement the credit budget. After
// eviction forces credit fallback, remaining budget is correct.
// PREVENTS: Over-counting when prefix maps are evicted mid-session and
// credit fallback re-counts routes already matched by prefix.
func TestRouteMatrixDirectMatchConsumesCredit(t *testing.T) {
	t.Parallel()

	m := NewRouteMatrix()
	now := time.Now()

	// p0 sends 3 prefixes — budget: familySent["ipv4/unicast"][0] = 3.
	p1 := netip.MustParsePrefix("10.0.0.0/24")
	p2 := netip.MustParsePrefix("10.0.1.0/24")
	p3 := netip.MustParsePrefix("10.0.2.0/24")
	m.RecordSent(0, p1, now)
	m.RecordSent(0, p2, now)
	m.RecordSent(0, p3, now)

	// p1 receives 2 via direct prefix match — should consume 2 credits.
	m.RecordReceived(1, p1, now.Add(time.Millisecond))
	m.RecordReceived(1, p2, now.Add(time.Millisecond))

	if got := m.Get(0, 1); got != 2 {
		t.Fatalf("Get(0,1) after direct = %d, want 2", got)
	}

	// Now receive an unknown prefix — credit fallback should only have
	// 1 remaining credit (3 sent - 2 consumed by direct matches).
	unknown1 := netip.MustParsePrefix("10.99.0.0/24")
	m.RecordReceived(1, unknown1, now.Add(2*time.Millisecond))
	if got := m.Get(0, 1); got != 3 {
		t.Errorf("Get(0,1) after 1 credit = %d, want 3", got)
	}

	// Budget exhausted: another unknown should NOT credit.
	unknown2 := netip.MustParsePrefix("10.99.1.0/24")
	m.RecordReceived(1, unknown2, now.Add(3*time.Millisecond))
	if got := m.Get(0, 1); got != 3 {
		t.Errorf("Get(0,1) after exhaustion = %d, want 3 (no more credits)", got)
	}

	// Verify stats.
	stats := m.Stats()
	if stats.DirectMatch != 2 {
		t.Errorf("DirectMatch = %d, want 2", stats.DirectMatch)
	}
	if stats.CreditMatch != 1 {
		t.Errorf("CreditMatch = %d, want 1", stats.CreditMatch)
	}
	if stats.Unmatched != 1 {
		t.Errorf("Unmatched = %d, want 1", stats.Unmatched)
	}
}

// TestRouteMatrixTopNPeers verifies top-N sorting by activity.
//
// VALIDATES: TopNPeers returns peers sorted by total route involvement.
// PREVENTS: Heatmap showing inactive peers instead of most active ones.
func TestRouteMatrixTopNPeers(t *testing.T) {
	t.Parallel()

	m := NewRouteMatrix()
	// peer 0 sends 10 prefixes, peer 1 sends 2, peer 2 sends 5
	now := time.Now()
	for i := range 10 {
		p := netip.MustParsePrefix("10.0." + itoa(i) + ".0/24")
		m.RecordSent(0, p, now)
		m.RecordReceived(3, p, now) // peer 3 receives all from peer 0
	}
	for i := range 2 {
		p := netip.MustParsePrefix("10.1." + itoa(i) + ".0/24")
		m.RecordSent(1, p, now)
		m.RecordReceived(3, p, now)
	}
	for i := range 5 {
		p := netip.MustParsePrefix("10.2." + itoa(i) + ".0/24")
		m.RecordSent(2, p, now)
		m.RecordReceived(3, p, now)
	}

	top := m.TopNPeers(3)
	if len(top) != 3 {
		t.Fatalf("TopNPeers(3) len = %d, want 3", len(top))
	}
	// Top 3 by activity are peers 3, 0, 2 — sorted by index: 0, 2, 3.
	if top[0] != 0 {
		t.Errorf("top[0] = %d, want 0 (sorted by index)", top[0])
	}
	if top[1] != 2 {
		t.Errorf("top[1] = %d, want 2 (sorted by index)", top[1])
	}
	if top[2] != 3 {
		t.Errorf("top[2] = %d, want 3 (sorted by index)", top[2])
	}
}

// TestRouteMatrixMaxCell verifies the max cell value for color scaling.
//
// VALIDATES: MaxCell returns the highest single-cell route count.
// PREVENTS: Heatmap colors using wrong normalization factor.
func TestRouteMatrixMaxCell(t *testing.T) {
	t.Parallel()

	m := NewRouteMatrix()
	p1 := netip.MustParsePrefix("10.0.0.0/24")
	p2 := netip.MustParsePrefix("10.0.1.0/24")

	now := time.Now()
	m.RecordSent(0, p1, now)
	m.RecordSent(0, p2, now)
	m.RecordReceived(1, p1, now)
	m.RecordReceived(1, p2, now)
	m.RecordReceived(1, p1, now) // duplicate receive

	if got := m.MaxCell(); got != 3 {
		t.Errorf("MaxCell() = %d, want 3", got)
	}
}

// TestHandleVizRouteMatrix verifies the route matrix endpoint renders the heatmap.
//
// VALIDATES: Route matrix endpoint returns HTML with heatmap grid.
// PREVENTS: Empty heatmap when route flow data exists.
func TestHandleVizRouteMatrix(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	// Simulate route flow: peer 0 sends, peer 1 receives.
	now := time.Now()
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	d.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: prefix})
	d.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Time: now, Prefix: prefix})

	req := httptest.NewRequest(http.MethodGet, "/viz/route-matrix", http.NoBody)
	w := httptest.NewRecorder()

	d.handleVizRouteMatrix(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Route Flow Matrix") {
		t.Error("response missing heading")
	}
	if !strings.Contains(body, "heatmap-grid") {
		t.Error("response missing heatmap grid")
	}
	if !strings.Contains(body, "heatmap-cell") {
		t.Error("response missing heatmap cells")
	}
}

// TestHandleVizRouteMatrixEmpty verifies empty state message.
//
// VALIDATES: Route matrix shows "no data" message when empty.
// PREVENTS: Broken/empty grid when no route flow has occurred.
func TestHandleVizRouteMatrixEmpty(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	req := httptest.NewRequest(http.MethodGet, "/viz/route-matrix", http.NoBody)
	w := httptest.NewRecorder()

	d.handleVizRouteMatrix(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "No route flow data yet") {
		t.Error("empty matrix should show no-data message")
	}
}

// TestHandleVizRouteMatrixTopParam verifies the top query parameter.
//
// VALIDATES: top=10 limits visible peers in the heatmap.
// PREVENTS: Ignoring the top parameter and showing all peers.
func TestHandleVizRouteMatrixTopParam(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(30)
	defer d.broker.Close()

	// Create routes from 20 different source peers.
	now := time.Now()
	for i := range 20 {
		p := netip.MustParsePrefix("10.0." + itoa(i) + ".0/24")
		d.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: i, Time: now, Prefix: p})
		d.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 20, Time: now, Prefix: p})
	}

	req := httptest.NewRequest(http.MethodGet, "/viz/route-matrix?top=10", http.NoBody)
	w := httptest.NewRecorder()

	d.handleVizRouteMatrix(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Peers") {
		t.Error("response missing peers stat")
	}
	// With top=10, should show selected="10" in dropdown.
	if !strings.Contains(body, `value="10" selected`) {
		t.Error("top=10 should be selected in dropdown")
	}
}

// TestHandleVizRouteMatrixLatencyMode verifies the latency toggle renders avg latency.
//
// VALIDATES: mode=latency switches cells from counts to avg latency display.
// PREVENTS: Latency toggle having no effect on rendered cells.
func TestHandleVizRouteMatrixLatencyMode(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	// Create route flow with timing gap for measurable latency.
	t0 := time.Now()
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	d.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: t0, Prefix: prefix})
	d.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Time: t0.Add(50 * time.Millisecond), Prefix: prefix})

	req := httptest.NewRequest(http.MethodGet, "/viz/route-matrix?mode=latency", http.NoBody)
	w := httptest.NewRecorder()

	d.handleVizRouteMatrix(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `value="latency" selected`) {
		t.Error("latency mode should be selected in dropdown")
	}
	if !strings.Contains(body, "50ms") {
		t.Error("latency cell should show ~50ms value")
	}
	if !strings.Contains(body, "Max Avg Latency") {
		t.Error("stats footer should show max avg latency")
	}
}

// TestHandleVizRouteMatrixFamilyFilter verifies family filtering.
//
// VALIDATES: family=ipv4/unicast filters matrix to only IPv4 routes.
// PREVENTS: Family filter being ignored and showing all routes.
func TestHandleVizRouteMatrixFamilyFilter(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	now := time.Now()
	// IPv4 route: peer 0 → peer 1.
	p4 := netip.MustParsePrefix("10.0.0.0/24")
	d.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: p4})
	d.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Time: now, Prefix: p4})

	// IPv6 route: peer 2 → peer 3.
	p6 := netip.MustParsePrefix("2001:db8::/32")
	d.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 2, Time: now, Prefix: p6})
	d.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 3, Time: now, Prefix: p6})

	// Filter to ipv4/unicast only.
	req := httptest.NewRequest(http.MethodGet, "/viz/route-matrix?family=ipv4/unicast", http.NoBody)
	w := httptest.NewRecorder()
	d.handleVizRouteMatrix(w, req)

	body := w.Body.String()
	// Should show the family dropdown with ipv4/unicast selected.
	if !strings.Contains(body, `value="ipv4/unicast" selected`) {
		t.Error("ipv4/unicast should be selected in family dropdown")
	}
	// Cell p0→p1 should have count 1 for IPv4.
	if !strings.Contains(body, "p0→p1: 1 route") {
		t.Error("IPv4 route should show count 1")
	}
}

// TestHandleVizRouteMatrixCustomPeers verifies custom peer selection.
//
// VALIDATES: peers=0,1 query parameter limits heatmap to those peers.
// PREVENTS: Custom peer selection being ignored.
func TestHandleVizRouteMatrixCustomPeers(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(10)
	defer d.broker.Close()

	now := time.Now()
	// Create routes involving peers 0, 1, 5, 7.
	for _, src := range []int{0, 1, 5, 7} {
		p := netip.MustParsePrefix("10.0." + itoa(src) + ".0/24")
		d.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: src, Time: now, Prefix: p})
		d.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 9, Time: now, Prefix: p})
	}

	// Request only peers 0 and 1.
	req := httptest.NewRequest(http.MethodGet, "/viz/route-matrix?peers=0,1", http.NoBody)
	w := httptest.NewRecorder()
	d.handleVizRouteMatrix(w, req)

	body := w.Body.String()
	// Should contain p0 and p1 headers.
	if !strings.Contains(body, ">p0<") {
		t.Error("custom peers should include p0")
	}
	if !strings.Contains(body, ">p1<") {
		t.Error("custom peers should include p1")
	}
	// Should NOT contain p5 or p7 headers.
	if strings.Contains(body, ">p5<") {
		t.Error("custom peers should NOT include p5")
	}
	if strings.Contains(body, ">p7<") {
		t.Error("custom peers should NOT include p7")
	}
}

// TestRouteMatrixFamilyTracking verifies per-family route count tracking.
//
// VALIDATES: GetByFamily returns correct counts per family.
// PREVENTS: Family filter showing wrong counts.
func TestRouteMatrixFamilyTracking(t *testing.T) {
	t.Parallel()

	m := NewRouteMatrix()
	now := time.Now()

	// IPv4 route.
	p4 := netip.MustParsePrefix("10.0.0.0/24")
	m.RecordSent(0, p4, now)
	m.RecordReceived(1, p4, now)

	// IPv6 route.
	p6 := netip.MustParsePrefix("2001:db8::/32")
	m.RecordSent(0, p6, now)
	m.RecordReceived(1, p6, now)

	// Total should be 2.
	if got := m.Get(0, 1); got != 2 {
		t.Errorf("Get(0,1) = %d, want 2", got)
	}

	// IPv4 only should be 1.
	if got := m.GetByFamily(0, 1, "ipv4/unicast"); got != 1 {
		t.Errorf("GetByFamily(0,1,ipv4) = %d, want 1", got)
	}

	// IPv6 only should be 1.
	if got := m.GetByFamily(0, 1, "ipv6/unicast"); got != 1 {
		t.Errorf("GetByFamily(0,1,ipv6) = %d, want 1", got)
	}

	// Empty family = total.
	if got := m.GetByFamily(0, 1, ""); got != 2 {
		t.Errorf("GetByFamily(0,1,'') = %d, want 2", got)
	}

	// Families should include both.
	fams := m.Families()
	if len(fams) != 2 {
		t.Fatalf("Families() len = %d, want 2", len(fams))
	}
}

// TestConvergenceHistogramBuckets verifies latency values are bucketed correctly.
//
// VALIDATES: Each of the 9 bucket ranges captures the correct latencies.
// PREVENTS: Off-by-one bucket boundaries causing wrong distribution.
func TestConvergenceHistogramBuckets(t *testing.T) {
	t.Parallel()

	h := NewConvergenceHistogram()

	// Record one value per bucket.
	latencies := []time.Duration{
		1 * time.Millisecond,    // 0-5ms
		7 * time.Millisecond,    // 5-10ms
		15 * time.Millisecond,   // 10-25ms
		30 * time.Millisecond,   // 25-50ms
		75 * time.Millisecond,   // 50-100ms
		150 * time.Millisecond,  // 100-250ms
		300 * time.Millisecond,  // 250-500ms
		750 * time.Millisecond,  // 500ms-1s
		1500 * time.Millisecond, // 1-2s
		3 * time.Second,         // 2-5s
		7 * time.Second,         // 5-10s
		15 * time.Second,        // 10-30s
		45 * time.Second,        // >30s
	}

	for _, lat := range latencies {
		h.Record(lat)
	}

	if h.Total != convergenceBucketCount {
		t.Fatalf("total = %d, want %d", h.Total, convergenceBucketCount)
	}

	for i, b := range h.Buckets {
		if b.Count != 1 {
			t.Errorf("bucket %d (%s) count = %d, want 1", i, b.Label, b.Count)
		}
	}
}

// TestConvergenceHistogramBoundary verifies boundary values go to the right bucket.
//
// VALIDATES: Exact boundary values (e.g., 5ms) go to the higher bucket.
// PREVENTS: Values at bucket boundaries counted in wrong bucket.
func TestConvergenceHistogramBoundary(t *testing.T) {
	t.Parallel()

	h := NewConvergenceHistogram()

	// 5ms exactly should go to "5-10ms" bucket (index 1), not "0-5ms" (index 0).
	h.Record(5 * time.Millisecond)
	if h.Buckets[0].Count != 0 {
		t.Errorf("bucket 0 (%s) count = %d, want 0", h.Buckets[0].Label, h.Buckets[0].Count)
	}
	if h.Buckets[1].Count != 1 {
		t.Errorf("bucket 1 (%s) count = %d, want 1", h.Buckets[1].Label, h.Buckets[1].Count)
	}

	// 1s exactly should go to "1-2s" bucket (index 8).
	h.Record(time.Second)
	if h.Buckets[8].Count != 1 {
		t.Errorf("bucket 8 (%s) count = %d, want 1", h.Buckets[8].Label, h.Buckets[8].Count)
	}

	// 45s should go to ">30s" bucket (last bucket).
	h.Record(45 * time.Second)
	last := convergenceBucketCount - 1
	if h.Buckets[last].Count != 1 {
		t.Errorf("bucket %d (%s) count = %d, want 1", last, h.Buckets[last].Label, h.Buckets[last].Count)
	}
}

// TestConvergenceHistogramStats verifies running stats (min, avg, max).
//
// VALIDATES: Min, max, avg, and slow count computed correctly.
// PREVENTS: Stats reflecting wrong values after multiple records.
func TestConvergenceHistogramStats(t *testing.T) {
	t.Parallel()

	h := NewConvergenceHistogram()
	h.Record(10 * time.Millisecond)
	h.Record(20 * time.Millisecond)
	h.Record(30 * time.Millisecond)

	if h.Min != 10*time.Millisecond {
		t.Errorf("min = %v, want 10ms", h.Min)
	}
	if h.Max != 30*time.Millisecond {
		t.Errorf("max = %v, want 30ms", h.Max)
	}
	if h.Avg() != 20*time.Millisecond {
		t.Errorf("avg = %v, want 20ms", h.Avg())
	}
	if h.SlowCount != 0 {
		t.Errorf("slow count = %d, want 0", h.SlowCount)
	}

	h.Record(2 * time.Second)
	if h.SlowCount != 1 {
		t.Errorf("slow count after 2s = %d, want 1", h.SlowCount)
	}
}

// TestConvergenceHistogramMaxCount verifies MaxCount for bar scaling.
//
// VALIDATES: MaxCount returns the highest bucket count.
// PREVENTS: Bar chart scaling using wrong denominator.
func TestConvergenceHistogramMaxCount(t *testing.T) {
	t.Parallel()

	h := NewConvergenceHistogram()
	// Put 5 values in the 0-5ms bucket.
	for range 5 {
		h.Record(1 * time.Millisecond)
	}
	// Put 3 values in the 50-100ms bucket.
	for range 3 {
		h.Record(75 * time.Millisecond)
	}

	if h.MaxCount() != 5 {
		t.Errorf("MaxCount() = %d, want 5", h.MaxCount())
	}
}

// TestHandleVizEvents verifies the event stream visualization endpoint.
//
// VALIDATES: Event stream returns HTML with event rows.
// PREVENTS: Empty event stream when events exist.
func TestHandleVizEvents(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	// Add some events.
	now := time.Now()
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	d.ProcessEvent(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 1, Time: now, ChaosAction: "disconnect"})

	req := httptest.NewRequest(http.MethodGet, "/viz/events", http.NoBody)
	w := httptest.NewRecorder()

	d.handleVizEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Event Stream") {
		t.Error("response missing Event Stream heading")
	}
	if !strings.Contains(body, "established") {
		t.Error("response missing established event")
	}
	if !strings.Contains(body, "chaos") {
		t.Error("response missing chaos event")
	}
}

// TestHandleVizEventsPeerFilter verifies peer filtering in event stream.
//
// VALIDATES: peer=1 query parameter shows only peer 1 events.
// PREVENTS: Filter not applied, showing all events.
func TestHandleVizEventsPeerFilter(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	now := time.Now()
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	d.ProcessEvent(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 1, Time: now, ChaosAction: "disconnect"})

	req := httptest.NewRequest(http.MethodGet, "/viz/events?peer=1", http.NoBody)
	w := httptest.NewRecorder()

	d.handleVizEvents(w, req)

	body := w.Body.String()
	// Should have peer 1 events.
	if !strings.Contains(body, "p1") {
		t.Error("filtered response should contain p1")
	}
	// Should NOT have peer 0's established event row (p0 in event-type span).
	// Count occurrences of "p0" in event-type spans.
	if strings.Contains(body, ">p0<") {
		t.Error("filtered response should NOT contain p0 event rows")
	}
}

// TestHandleVizEventsTypeFilter verifies type filtering in event stream.
//
// VALIDATES: type=chaos query parameter shows only chaos events.
// PREVENTS: Type filter matching wrong event types.
func TestHandleVizEventsTypeFilter(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	now := time.Now()
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	d.ProcessEvent(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 1, Time: now, ChaosAction: "disconnect"})

	req := httptest.NewRequest(http.MethodGet, "/viz/events?type=chaos", http.NoBody)
	w := httptest.NewRecorder()

	d.handleVizEvents(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "event-type-chaos") {
		t.Error("filtered response should contain chaos event rows")
	}
	// Event rows with "event-type-established" class should not appear when filtering for chaos.
	if strings.Contains(body, "event-type-established") {
		t.Error("filtered response should not have established event rows")
	}
}

// TestHandleVizConvergence verifies the convergence histogram endpoint.
//
// VALIDATES: Convergence endpoint returns histogram HTML with bar chart.
// PREVENTS: Empty histogram or wrong bucket labels.
func TestHandleVizConvergence(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	// Record some convergence data.
	d.state.mu.Lock()
	d.state.Convergence.Record(5 * time.Millisecond)
	d.state.Convergence.Record(50 * time.Millisecond)
	d.state.Convergence.Record(500 * time.Millisecond)
	d.state.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/viz/convergence", http.NoBody)
	w := httptest.NewRecorder()

	d.handleVizConvergence(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Convergence Histogram") {
		t.Error("response missing heading")
	}
	if !strings.Contains(body, "histogram-bar") {
		t.Error("response missing histogram bars")
	}
	if !strings.Contains(body, "Total") {
		t.Error("response missing stats")
	}
}

// TestHandleVizPeerTimeline verifies the peer timeline endpoint.
//
// VALIDATES: Timeline returns rows with status segments for peers with transitions.
// PREVENTS: Empty timeline when peers have state changes.
func TestHandleVizPeerTimeline(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	now := time.Now()
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	d.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 0, Time: now.Add(time.Second)})
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 2, Time: now})

	req := httptest.NewRequest(http.MethodGet, "/viz/peer-timeline", http.NoBody)
	w := httptest.NewRecorder()

	d.handleVizPeerTimeline(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Peer State Timeline") {
		t.Error("response missing heading")
	}
	if !strings.Contains(body, "timeline-row") {
		t.Error("response missing timeline rows")
	}
	if !strings.Contains(body, "timeline-segment") {
		t.Error("response missing timeline segments")
	}
}

// TestHandleVizPeerTimelinePagination verifies pagination works.
//
// VALIDATES: page=2 shows different peers than page=1.
// PREVENTS: All peers shown on every page regardless of parameter.
func TestHandleVizPeerTimelinePagination(t *testing.T) {
	t.Parallel()

	// Create enough peers for 2 pages (>30).
	d := newTestDashboard(40)
	defer d.broker.Close()

	now := time.Now()
	// Give all peers a transition so they show up.
	for i := range 40 {
		d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: i, Time: now})
	}

	// Page 1 should have p0.
	req := httptest.NewRequest(http.MethodGet, "/viz/peer-timeline?page=1", http.NoBody)
	w := httptest.NewRecorder()
	d.handleVizPeerTimeline(w, req)
	page1 := w.Body.String()

	// Page 2 should have p30+ but not p0.
	req = httptest.NewRequest(http.MethodGet, "/viz/peer-timeline?page=2", http.NoBody)
	w = httptest.NewRecorder()
	d.handleVizPeerTimeline(w, req)
	page2 := w.Body.String()

	if !strings.Contains(page1, ">p0<") {
		t.Error("page 1 should contain p0")
	}
	if strings.Contains(page2, ">p0<") {
		t.Error("page 2 should NOT contain p0")
	}
	if !strings.Contains(page2, "Page 2/2") {
		t.Error("page 2 should show page indicator")
	}
}

// TestHandleVizChaosTimeline verifies the chaos timeline endpoint.
//
// VALIDATES: Chaos timeline shows markers positioned by time.
// PREVENTS: Empty timeline when chaos events have occurred.
func TestHandleVizChaosTimeline(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	now := time.Now()
	d.ProcessEvent(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 0, Time: now, ChaosAction: "tcp-disconnect"})
	d.ProcessEvent(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 1, Time: now.Add(time.Second), ChaosAction: "hold-timer-expiry"})

	req := httptest.NewRequest(http.MethodGet, "/viz/chaos-timeline", http.NoBody)
	w := httptest.NewRecorder()

	d.handleVizChaosTimeline(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Chaos Timeline") {
		t.Error("response missing heading")
	}
	if !strings.Contains(body, "chaos-marker") {
		t.Error("response missing chaos markers")
	}
	if !strings.Contains(body, "tcp-disconnect") {
		t.Error("response missing tcp-disconnect action in legend or tooltip")
	}
	if !strings.Contains(body, "hold-timer-expiry") {
		t.Error("response missing hold-timer-expiry action")
	}
}

// TestChaosTimelineColorsMatchActionTypes verifies each real chaos action gets
// its designated color instead of falling through to default gray.
//
// VALIDATES: Timeline markers use action-specific colors from the color map.
// PREVENTS: All markers rendering as gray due to mismatched action names.
func TestChaosTimelineColorsMatchActionTypes(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	// Every real chaos.ActionType.String() value and its expected color.
	actions := map[string]string{
		"tcp-disconnect":          "#f85149",
		"notification-cease":      "#ffa657",
		"hold-timer-expiry":       "#d29922",
		"disconnect-during-burst": "#ff7b72",
		"reconnect-storm":         "#db6d28",
		"connection-collision":    "#d2a8ff",
		"malformed-update":        "#bc8cff",
		"config-reload":           "#79c0ff",
	}

	now := time.Now()
	i := 0
	for action := range actions {
		d.ProcessEvent(peer.Event{
			Type:        peer.EventChaosExecuted,
			PeerIndex:   i % 5,
			Time:        now.Add(time.Duration(i) * time.Second),
			ChaosAction: action,
		})
		i++
	}

	req := httptest.NewRequest(http.MethodGet, "/viz/chaos-timeline", http.NoBody)
	w := httptest.NewRecorder()

	d.handleVizChaosTimeline(w, req)

	body := w.Body.String()

	for action, color := range actions {
		// Check marker has the correct inline background color.
		marker := "background:" + color
		if !strings.Contains(body, marker) {
			t.Errorf("action %q: expected marker with %s, not found in output", action, marker)
		}
		// Check legend swatch has the correct color.
		swatch := `style="background:` + color + `"></span>` + action
		if !strings.Contains(body, swatch) {
			t.Errorf("action %q: expected legend swatch with color %s", action, color)
		}
	}
}

// TestSortIntSlice verifies the simple integer sort.
//
// VALIDATES: sortIntSlice produces ascending order.
// PREVENTS: Wrong sort order in peer timeline indices.
func TestSortIntSlice(t *testing.T) {
	t.Parallel()

	s := []int{5, 2, 8, 1, 9, 3}
	sortIntSlice(s)

	want := []int{1, 2, 3, 5, 8, 9}
	for i, v := range s {
		if v != want[i] {
			t.Errorf("index %d: got %d, want %d", i, v, want[i])
		}
	}
}

// TestPctOfDuration verifies percentage calculation.
//
// VALIDATES: Duration percentage calculated correctly, clamped to 0-100.
// PREVENTS: Negative or >100 percentages causing CSS layout issues.
func TestPctOfDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		d     time.Duration
		total time.Duration
		want  int
	}{
		{"zero", 0, time.Second, 0},
		{"half", 500 * time.Millisecond, time.Second, 50},
		{"full", time.Second, time.Second, 100},
		{"over", 2 * time.Second, time.Second, 100},
		{"zero_total", time.Second, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := pctOfDuration(tt.d, tt.total)
			if got != tt.want {
				t.Errorf("pctOfDuration(%v, %v) = %d, want %d", tt.d, tt.total, got, tt.want)
			}
		})
	}
}

// TestRouteMatrixCellPlacement verifies that route counts are stored in the
// correct [source, dest] cell in a multi-peer RR topology.
//
// VALIDATES: In a 4-peer scenario where p0 sends to p1/p2/p3 and p3 sends
// to p0/p1/p2, each cell [source, dest] contains the right count.
// PREVENTS: Swapped source/dest indices placing values in wrong cells.
func TestRouteMatrixCellPlacement(t *testing.T) {
	t.Parallel()

	m := NewRouteMatrix()
	now := time.Now()

	// p0 sends 3 prefixes, reflected (received) by p1, p2, p3.
	prefixes0 := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/24"),
		netip.MustParsePrefix("10.0.1.0/24"),
		netip.MustParsePrefix("10.0.2.0/24"),
	}
	for _, p := range prefixes0 {
		m.RecordSent(0, p, now)
		m.RecordReceived(1, p, now)
		m.RecordReceived(2, p, now)
		m.RecordReceived(3, p, now)
	}

	// p3 sends 2 prefixes, reflected (received) by p0, p1, p2.
	prefixes3 := []netip.Prefix{
		netip.MustParsePrefix("10.3.0.0/24"),
		netip.MustParsePrefix("10.3.1.0/24"),
	}
	for _, p := range prefixes3 {
		m.RecordSent(3, p, now)
		m.RecordReceived(0, p, now)
		m.RecordReceived(1, p, now)
		m.RecordReceived(2, p, now)
	}

	// Verify exact cell values. Row = source, Col = dest.
	tests := []struct {
		src, dst, want int
	}{
		{0, 1, 3}, {0, 2, 3}, {0, 3, 3}, // p0 sent 3 to each
		{3, 0, 2}, {3, 1, 2}, {3, 2, 2}, // p3 sent 2 to each
		{1, 0, 0}, {1, 2, 0}, {1, 3, 0}, // p1 sent nothing
		{2, 0, 0}, {2, 1, 0}, {2, 3, 0}, // p2 sent nothing
		{0, 0, 0}, {1, 1, 0}, {2, 2, 0}, {3, 3, 0}, // diagonal = 0
	}
	for _, tt := range tests {
		if got := m.Get(tt.src, tt.dst); got != tt.want {
			t.Errorf("Get(%d,%d) = %d, want %d", tt.src, tt.dst, got, tt.want)
		}
	}
}

// TestRouteMatrixNonUnicastCellPlacement verifies that non-unicast credit
// matching places route counts in the correct [source, dest] cells.
//
// VALIDATES: RecordNonUnicastSent + RecordNonUnicastReceived correctly
// credits the sender→receiver pair and tracks per-family counts.
// PREVENTS: Non-unicast routes counted in wrong matrix cell or missing
// from family-filtered view.
func TestRouteMatrixNonUnicastCellPlacement(t *testing.T) {
	t.Parallel()

	m := NewRouteMatrix()

	// Only ONE sender to eliminate Go map iteration non-determinism.
	// p0 sends 3 VPN routes.
	for range 3 {
		m.RecordNonUnicastSent(0, "ipv4/vpn")
	}

	// p1 receives 2 VPN routes → should credit p0→p1.
	for range 2 {
		m.RecordNonUnicastReceived(1, "ipv4/vpn")
	}

	// p2 receives 1 VPN route → should credit p0→p2.
	m.RecordNonUnicastReceived(2, "ipv4/vpn")

	if got := m.Get(0, 1); got != 2 {
		t.Errorf("Get(0,1) = %d, want 2 (p0→p1)", got)
	}
	if got := m.Get(0, 2); got != 1 {
		t.Errorf("Get(0,2) = %d, want 1 (p0→p2)", got)
	}

	// Self-credit prevention: p0 receiving should not credit p0→p0.
	m.RecordNonUnicastReceived(0, "ipv4/vpn")
	if got := m.Get(0, 0); got != 0 {
		t.Errorf("Get(0,0) = %d, want 0 (self-credit prevented)", got)
	}

	// Credits are per (sender, receiver) pair: p0 sent 3, so each receiver
	// can independently receive up to 3 from p0 (RR reflects to each peer).
	// p3 receives should credit p0→p3 (p0 still has capacity for p3).
	m.RecordNonUnicastReceived(3, "ipv4/vpn")
	if got := m.Get(0, 3); got != 1 {
		t.Errorf("Get(0,3) = %d, want 1 (p0→p3 credited)", got)
	}

	// Exhaust p0→p3: send 2 more receives for p3.
	m.RecordNonUnicastReceived(3, "ipv4/vpn")
	m.RecordNonUnicastReceived(3, "ipv4/vpn")
	if got := m.Get(0, 3); got != 3 {
		t.Errorf("Get(0,3) = %d, want 3 (all p0 credits used for p3)", got)
	}

	// Now p0→p3 is exhausted (3/3). One more receive should not credit.
	m.RecordNonUnicastReceived(3, "ipv4/vpn")
	if got := m.Get(0, 3); got != 3 {
		t.Errorf("Get(0,3) = %d, want 3 (no more credits from p0)", got)
	}

	// Family tracking: non-unicast routes should appear in GetByFamily.
	if got := m.GetByFamily(0, 1, "ipv4/vpn"); got != 2 {
		t.Errorf("GetByFamily(0,1,ipv4/vpn) = %d, want 2", got)
	}
	if got := m.GetByFamily(0, 2, "ipv4/vpn"); got != 1 {
		t.Errorf("GetByFamily(0,2,ipv4/vpn) = %d, want 1", got)
	}

	// Families list should include ipv4/vpn.
	fams := m.Families()
	found := false
	for _, f := range fams {
		if f == "ipv4/vpn" {
			found = true
		}
	}
	if !found {
		t.Error("Families() should include ipv4/vpn")
	}
}

// TestRouteMatrixRenderedCellTitles verifies that the rendered HTML heatmap
// places cell title attributes matching source→dest with correct counts.
//
// VALIDATES: Rendered cell title "pX→pY: N routes" matches actual cell values
// for asymmetric flows where p0→p1 ≠ p1→p0.
// PREVENTS: Row/column swap in grid rendering causing misplaced values.
func TestRouteMatrixRenderedCellTitles(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	now := time.Now()

	// p0 sends 2 prefixes, reflected to p1 (asymmetric: p0→p1 = 2).
	pa := netip.MustParsePrefix("10.0.0.0/24")
	pb := netip.MustParsePrefix("10.0.1.0/24")
	d.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: pa, Family: "ipv4/unicast"})
	d.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: pb, Family: "ipv4/unicast"})
	d.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Time: now, Prefix: pa, Family: "ipv4/unicast"})
	d.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Time: now, Prefix: pb, Family: "ipv4/unicast"})

	// p1 sends 1 prefix, reflected to p0 (asymmetric: p1→p0 = 1).
	pc := netip.MustParsePrefix("10.1.0.0/24")
	d.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 1, Time: now, Prefix: pc, Family: "ipv4/unicast"})
	d.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 0, Time: now, Prefix: pc, Family: "ipv4/unicast"})

	req := httptest.NewRequest(http.MethodGet, "/viz/route-matrix", http.NoBody)
	w := httptest.NewRecorder()
	d.handleVizRouteMatrix(w, req)

	body := w.Body.String()

	// p0→p1: 2 routes (row p0, column p1). Asymmetric — NOT swapped.
	if !strings.Contains(body, `p0→p1: 2 routes`) {
		t.Error("cell p0→p1 should show 2 routes")
	}

	// p1→p0: 1 routes (row p1, column p0). Opposite direction.
	if !strings.Contains(body, `p1→p0: 1 routes`) {
		t.Error("cell p1→p0 should show 1 routes")
	}

	// Diagonal cells should show 0.
	if !strings.Contains(body, `p0→p0: 0 routes`) {
		t.Error("diagonal cell p0→p0 should show 0 routes")
	}
	if !strings.Contains(body, `p1→p1: 0 routes`) {
		t.Error("diagonal cell p1→p1 should show 0 routes")
	}
}

// TestRouteMatrixCreditCellPlacement verifies that credit-based matching
// places route counts in the correct [source, dest] cell.
//
// VALIDATES: When credit matching attributes a receive to the best sender,
// the cell is stored as [sender, receiver] not [receiver, sender].
// PREVENTS: Swapped source/dest indices in credit-matched cells.
func TestRouteMatrixCreditCellPlacement(t *testing.T) {
	t.Parallel()

	m := NewRouteMatrix()
	now := time.Now()

	// p0 sends 3 prefixes (tracked in routeOrigins + familySent).
	for i := range 3 {
		p := netip.MustParsePrefix("10.0." + itoa(i) + ".0/24")
		m.RecordSent(0, p, now)
	}

	// p1 and p2 receive unknown prefixes → credit fallback to p0.
	m.RecordReceived(1, netip.MustParsePrefix("10.99.0.0/24"), now)
	m.RecordReceived(2, netip.MustParsePrefix("10.99.1.0/24"), now)

	// Cells should be p0→p1 and p0→p2 (source=p0, not dest=p0).
	if got := m.Get(0, 1); got != 1 {
		t.Errorf("Get(0,1) = %d, want 1 (p0→p1)", got)
	}
	if got := m.Get(0, 2); got != 1 {
		t.Errorf("Get(0,2) = %d, want 1 (p0→p2)", got)
	}
	// Reverse should be empty.
	if got := m.Get(1, 0); got != 0 {
		t.Errorf("Get(1,0) = %d, want 0 (no reverse flow)", got)
	}
	if got := m.Get(2, 0); got != 0 {
		t.Errorf("Get(2,0) = %d, want 0 (no reverse flow)", got)
	}
}

// TestChaosHistoryRecording verifies chaos events are recorded in history.
//
// VALIDATES: ProcessEvent records chaos events in ChaosHistory.
// PREVENTS: Chaos timeline showing no data despite chaos events occurring.
func TestChaosHistoryRecording(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	now := time.Now()
	d.ProcessEvent(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 2, Time: now, ChaosAction: "disconnect"})
	d.ProcessEvent(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 3, Time: now.Add(time.Second), ChaosAction: "withdraw"})

	d.state.RLock()
	defer d.state.RUnlock()

	if len(d.state.ChaosHistory) != 2 {
		t.Fatalf("chaos history len = %d, want 2", len(d.state.ChaosHistory))
	}
	if d.state.ChaosHistory[0].Action != "disconnect" {
		t.Errorf("entry 0 action = %q, want disconnect", d.state.ChaosHistory[0].Action)
	}
	if d.state.ChaosHistory[1].PeerIndex != 3 {
		t.Errorf("entry 1 peer = %d, want 3", d.state.ChaosHistory[1].PeerIndex)
	}
}

// TestPeerTransitionRecording verifies status changes are recorded for timeline.
//
// VALIDATES: ProcessEvent records peer state transitions.
// PREVENTS: Peer timeline showing no segments despite status changes.
func TestPeerTransitionRecording(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	now := time.Now()
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	d.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 0, Time: now.Add(time.Second)})
	d.ProcessEvent(peer.Event{Type: peer.EventReconnecting, PeerIndex: 0, Time: now.Add(2 * time.Second)})

	d.state.RLock()
	defer d.state.RUnlock()

	transitions := d.state.PeerTransitions[0]
	if len(transitions) != 3 {
		t.Fatalf("transitions len = %d, want 3", len(transitions))
	}
	if transitions[0].Status != PeerSyncing {
		t.Errorf("transition 0 = %v, want PeerSyncing", transitions[0].Status)
	}
	if transitions[1].Status != PeerDown {
		t.Errorf("transition 1 = %v, want PeerDown", transitions[1].Status)
	}
	if transitions[2].Status != PeerReconnecting {
		t.Errorf("transition 2 = %v, want PeerReconnecting", transitions[2].Status)
	}
}

// TestStatusColor verifies all peer statuses have distinct timeline colors.
//
// VALIDATES: AC-7 — PeerSyncing maps to cyan (#58a6ff), all statuses have colors.
// PREVENTS: Missing color for syncing peers in timeline visualization.
func TestStatusColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status PeerStatus
		want   string
	}{
		{PeerUp, "#3fb950"},
		{PeerSyncing, "#58a6ff"},
		{PeerDown, "#f85149"},
		{PeerReconnecting, "#d29922"},
		{PeerIdle, "#6e7681"},
	}

	for _, tt := range tests {
		if got := statusColor(tt.status); got != tt.want {
			t.Errorf("statusColor(%v) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

// TestChaosHistoryCap verifies ChaosHistory is capped to prevent unbounded growth.
//
// VALIDATES: ChaosHistory never exceeds maxChaosHistory entries.
// PREVENTS: Memory exhaustion during long-running chaos tests.
func TestChaosHistoryCap(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	now := time.Now()
	for i := range maxChaosHistory + 100 {
		d.ProcessEvent(peer.Event{
			Type:        peer.EventChaosExecuted,
			PeerIndex:   i % 5,
			Time:        now.Add(time.Duration(i) * time.Millisecond),
			ChaosAction: "disconnect",
		})
	}

	d.state.RLock()
	n := len(d.state.ChaosHistory)
	d.state.RUnlock()

	if n > maxChaosHistory {
		t.Fatalf("ChaosHistory len = %d, want <= %d", n, maxChaosHistory)
	}
	if n == 0 {
		t.Fatal("ChaosHistory is empty after many events")
	}
}

// TestPeerTransitionsCap verifies per-peer transitions are capped.
//
// VALIDATES: PeerTransitions per peer never exceeds maxPeerTransitions.
// PREVENTS: Memory exhaustion from rapidly flapping peers.
func TestPeerTransitionsCap(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	now := time.Now()
	// Alternate between up and down to create transitions.
	for i := range maxPeerTransitions + 100 {
		evType := peer.EventEstablished
		if i%2 == 1 {
			evType = peer.EventDisconnected
		}
		d.ProcessEvent(peer.Event{
			Type:      evType,
			PeerIndex: 0,
			Time:      now.Add(time.Duration(i) * time.Millisecond),
		})
	}

	d.state.RLock()
	n := len(d.state.PeerTransitions[0])
	d.state.RUnlock()

	if n > maxPeerTransitions {
		t.Fatalf("PeerTransitions[0] len = %d, want <= %d", n, maxPeerTransitions)
	}
	if n == 0 {
		t.Fatal("PeerTransitions[0] is empty after many transitions")
	}
}
