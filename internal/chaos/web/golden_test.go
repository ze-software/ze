// Related: render.go -- writeLayout and the writers every case below reaches
// Related: handlers.go -- registerRoutes, the route list these cases mirror
// Related: internal/component/web/handler_golden_test.go -- the same capture for
// the operator UI, whose shape this one follows

package web

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/golden"
)

// chaosGoldenFixtures is where the captured responses live.
const chaosGoldenFixtures = "golden"

// chaosGoldenPeers is the peer count every case renders. It is fixed here so
// two runs render the same dashboard.
const chaosGoldenPeers = 6

// chaosGoldenRun is how long the captured run has been going when a case
// renders. Every seeded time is an offset from now minus this, so a fixture
// holds elapsed times rather than wall-clock ones.
//
// Three hours is chosen, not decorative. formatElapsed prints hours and minutes
// above an hour and minutes and SECONDS below one, so a shorter run would put a
// second-resolution age in the markup. Such a fixture goes red when the render
// stalls across a second boundary. At three hours the age only changes on the
// minute, which no unit test reaches.
const chaosGoldenRun = 3 * time.Hour

// chaosGoldenSyncDuration is the sync time the stats card shows. ProcessEvent
// computes SyncDuration from time.Since when the last peer sends its EOR, so
// the capture pins it afterwards.
const chaosGoldenSyncDuration = 4500 * time.Millisecond

// chaosUptimeRewrite drops the uptime writeLayout renders.
//
// The producer is writeLayout (render.go), which computes
// FormatDuration(time.Since(s.StartTime)) on every request. FormatDuration
// keeps milliseconds, so the value moves between two renders of the same state.
//
// The data-start attribute beside it is NOT dropped: it is a whole number of
// seconds, which the seeded StartTime pins. Only the text node is replaced, and
// the pattern requires the id and the attribute around it, so a layout that
// stops rendering the span leaves the raw value in the fixture instead of a
// normalized one.
var chaosUptimeRewrite = golden.Rewrite{
	Pattern:     regexp.MustCompile(`(<span id="uptime" data-start="\d+">)[^<]*(</span>)`),
	Replacement: "${1}<UPTIME>${2}",
}

// chaosRunDurationRewrite drops the run duration the chaos timeline shows.
//
// The producer is writeChaosTimeline (viz.go), which computes
// FormatDuration(now.Sub(s.StartTime)) for its Duration stat. Same reason as
// the uptime above: millisecond resolution against a clock that moves.
//
// The marker positions in the same panel are NOT dropped. They are percentages
// of a three-hour window, which the seeded times pin.
var chaosRunDurationRewrite = golden.Rewrite{
	Pattern:     regexp.MustCompile(`(<span class="stat-label">Duration </span><span class="stat-value">)[^<]*(</span>)`),
	Replacement: "${1}<DURATION>${2}",
}

// chaosGoldenCase is one HTTP request and the bytes it must keep producing.
type chaosGoldenCase struct {
	// Name is the fixture stem.
	Name string
	// Method and Target are the request line.
	Method string
	Target string
	// Form is the body of a POST, sent as a URL-encoded form.
	Form url.Values
	// Setup changes the seeded dashboard before the request is served.
	Setup func(*Dashboard)
	// HTMX sends the header htmx puts on every request it issues. Every button
	// on the dashboard is an hx-post, so it selects the answer a refused one
	// receives: a fragment for the target the request named, rather than the
	// bare status line http.Error writes.
	HTMX bool
	// Status is the status this case must answer with, and zero means 200. A
	// case that captures a refusal states the status it expects, so the check
	// below still pins one status per case rather than accepting any.
	Status int
	// Stream marks a response the handler writes until the client leaves. The
	// request is canceled before it is served, so the capture holds the head
	// alone.
	Stream bool
	// Rewrites normalize a volatile span. Each one names its producer.
	Rewrites []golden.Rewrite
}

// chaosGoldenNoControl removes the control channels, which is what a run with
// no --chaos-rate gives the dashboard.
func chaosGoldenNoControl(d *Dashboard) {
	d.control = nil
	d.routeControl = nil
	d.restartCh = nil
	d.state.Control = ControlState{}
	d.state.Properties = nil
}

// chaosGoldenCases are the chaos responses this capture pins.
//
// Chaos writes its HTML from Go string literals, so no template capture can
// reach it and no .templ file names what it renders. These bytes are the only
// record of what an operator watching a chaos run receives.
//
// Every route registerRoutes registers has a case: TestChaosGoldenOutput reads
// that list out of the source and fails on a route no case reaches.
var chaosGoldenCases = []chaosGoldenCase{
	{Name: "assets-ze-svg", Method: http.MethodGet, Target: "/assets/ze.svg"},
	{Name: "events-stream", Method: http.MethodGet, Target: "/events", Stream: true},

	{Name: "index", Method: http.MethodGet, Target: "/",
		Rewrites: []golden.Rewrite{chaosUptimeRewrite}},
	{Name: "index-no-control", Method: http.MethodGet, Target: "/",
		Setup:    chaosGoldenNoControl,
		Rewrites: []golden.Rewrite{chaosUptimeRewrite}},

	{Name: "peers", Method: http.MethodGet, Target: "/peers"},
	{Name: "peers-search", Method: http.MethodGet, Target: "/peers?search=1&status=up"},
	// The two sorted cases below are the regression test for a shuffling table.
	// Every column ties, and a tie used to keep the order a map range gave, so
	// each request answered a different row order. They pin one order per call
	// site of sortPeers: the peer table and the All Peers tab.
	{Name: "peers-sort-status", Method: http.MethodGet, Target: "/peers?sort=status&dir=asc"},
	{Name: "peers-grid", Method: http.MethodGet, Target: "/peers/grid"},
	{Name: "peers-grid-filtered", Method: http.MethodGet, Target: "/peers/grid?status=fault&search=-0"},
	{Name: "peers-table", Method: http.MethodGet, Target: "/peers/table"},
	{Name: "peer-detail", Method: http.MethodGet, Target: "/peer/2"},
	{Name: "peer-close", Method: http.MethodGet, Target: "/peer/close"},
	{Name: "peer-pin", Method: http.MethodPost, Target: "/peers/3/pin"},
	{Name: "peer-promote", Method: http.MethodPost, Target: "/peers/promote",
		Form: url.Values{"id": {"5"}}},

	{Name: "sidebar-stats", Method: http.MethodGet, Target: "/sidebar/stats"},
	{Name: "sidebar-events", Method: http.MethodGet, Target: "/sidebar/events"},
	{Name: "sidebar-active-set", Method: http.MethodGet, Target: "/sidebar/active-set"},
	{Name: "active-set-max-visible", Method: http.MethodPost, Target: "/active-set/max-visible",
		Form: url.Values{"n": {"4"}}},

	{Name: "viz-events", Method: http.MethodGet, Target: "/viz/events"},
	{Name: "viz-events-filtered", Method: http.MethodGet, Target: "/viz/events?peer=2&type=chaos"},
	{Name: "viz-convergence", Method: http.MethodGet, Target: "/viz/convergence"},
	{Name: "viz-convergence-trend", Method: http.MethodGet, Target: "/viz/convergence-trend"},
	{Name: "viz-peer-timeline", Method: http.MethodGet, Target: "/viz/peer-timeline"},
	{Name: "viz-chaos-events", Method: http.MethodGet, Target: "/viz/chaos-events"},
	{Name: "viz-chaos-timeline", Method: http.MethodGet, Target: "/viz/chaos-timeline",
		Rewrites: []golden.Rewrite{chaosRunDurationRewrite}},
	{Name: "viz-route-matrix", Method: http.MethodGet, Target: "/viz/route-matrix"},
	{Name: "viz-route-matrix-latency", Method: http.MethodGet,
		Target: "/viz/route-matrix?mode=latency&top=10&family=ipv4/unicast"},
	{Name: "viz-route-matrix-cell", Method: http.MethodGet, Target: "/viz/route-matrix/cell?src=0&dst=2"},
	{Name: "viz-families", Method: http.MethodGet, Target: "/viz/families"},
	{Name: "viz-all-peers", Method: http.MethodGet, Target: "/viz/all-peers"},
	{Name: "viz-all-peers-sort-chaos", Method: http.MethodGet, Target: "/viz/all-peers?sort=chaos&dir=desc"},
	{Name: "viz-panels", Method: http.MethodGet, Target: "/viz/panels",
		Rewrites: []golden.Rewrite{chaosRunDurationRewrite}},
	{Name: "viz-panel-content", Method: http.MethodGet, Target: "/viz/panel-content?panel=1&viz=route-matrix"},

	{Name: "control-pause", Method: http.MethodPost, Target: "/control/pause"},
	{Name: "control-resume", Method: http.MethodPost, Target: "/control/resume"},
	{Name: "control-rate", Method: http.MethodPost, Target: "/control/rate",
		Form: url.Values{"rate": {"0.25"}}},
	{Name: "control-trigger", Method: http.MethodPost, Target: "/control/trigger",
		Form: url.Values{"action": {"disconnect"}, "peers": {"1,4"}}},
	{Name: "control-stop", Method: http.MethodPost, Target: "/control/stop"},
	{Name: "control-speed", Method: http.MethodPost, Target: "/control/speed",
		Form: url.Values{"factor": {"100"}}},
	{Name: "control-restart", Method: http.MethodPost, Target: "/control/restart",
		Form: url.Values{"seed": {"77"}}},

	// Refused requests, captured in pairs. Every control on the dashboard is an
	// hx-post, so an operator meets these two answers whenever a run carries no
	// control channel or a form field is wrong. The bare status line is what
	// every other client still receives, and the pair is what the conversion
	// changed.
	{Name: "control-pause-no-control", Method: http.MethodPost, Target: "/control/pause",
		Setup: chaosGoldenNoControl, Status: http.StatusServiceUnavailable},
	{Name: "control-pause-no-control-htmx", Method: http.MethodPost, Target: "/control/pause",
		Setup: chaosGoldenNoControl, HTMX: true, Status: http.StatusServiceUnavailable},
	{Name: "active-set-max-visible-invalid-htmx", Method: http.MethodPost, Target: "/active-set/max-visible",
		Form: url.Values{"n": {"nine"}}, HTMX: true, Status: http.StatusBadRequest},

	{Name: "route-control-pause", Method: http.MethodPost, Target: "/control/route/pause"},
	{Name: "route-control-resume", Method: http.MethodPost, Target: "/control/route/resume"},
	{Name: "route-control-rate", Method: http.MethodPost, Target: "/control/route/rate",
		Form: url.Values{"rate": {"0.40"}}},
	{Name: "route-control-stop", Method: http.MethodPost, Target: "/control/route/stop"},
}

// chaosSSECase is one fragment the broadcast loop sends over the event stream.
//
// No route answers these, so the HTTP capture cannot reach them, and they carry
// the only hx-swap-oob attributes in chaos. htmx 4 reverses the order an
// out-of-band swap applies in, so these are the fragments a cutover is most
// likely to change and the ones nothing else would show.
type chaosSSECase struct {
	// Name is the fixture stem.
	Name string
	// Renderer is the function the case captures, spelled as the source
	// declares it. The coverage check reads that set out of the source.
	Renderer string
	// Render builds the fragment.
	Render func(*Dashboard) string
}

// chaosSSECases pins every payload the SSE broadcast sends.
var chaosSSECases = []chaosSSECase{
	{Name: "sse-stats", Renderer: "renderStats",
		Render: (*Dashboard).renderStats},
	{Name: "sse-convergence", Renderer: "renderConvergence",
		Render: (*Dashboard).renderConvergence},
	{Name: "sse-events", Renderer: "renderRecentEvents",
		Render: (*Dashboard).renderRecentEvents},
	{Name: "sse-peer-row", Renderer: "renderPeerRow",
		Render: func(d *Dashboard) string { return d.renderPeerRow(1) }},
	{Name: "sse-peer-row-insert", Renderer: "renderPeerRowInsert",
		Render: func(d *Dashboard) string { return d.renderPeerRowInsert(4) }},
	{Name: "sse-peer-cell", Renderer: "renderPeerCell",
		Render: func(d *Dashboard) string { return d.renderPeerCell(2) }},
	{Name: "sse-peer-removal", Renderer: "renderPeerRemoval",
		Render: func(_ *Dashboard) string { return renderPeerRemoval(3) }},
	{Name: "sse-toast", Renderer: "renderToast",
		Render: chaosGoldenToast},
}

// chaosGoldenToast renders the toast a disconnect queues, through the same
// producer the broadcast loop uses.
func chaosGoldenToast(d *Dashboard) string {
	toast, ok := toastForEvent(peer.Event{
		Type:      peer.EventDisconnected,
		PeerIndex: 1,
		Time:      d.state.StartTime,
	})
	if !ok {
		return ""
	}

	return renderToast(toast)
}

// chaosRendererDecl matches a payload builder's declaration.
var chaosRendererDecl = regexp.MustCompile(`(?m)^func (?:\(d \*Dashboard\) )?(render[A-Za-z]*)\(`)

// chaosSSEFiles are the files holding the fragments the broadcast loop sends.
// renderConvergenceTrend sits in viz_convergence_trend.go and reaches the
// operator through its own route, which viz-convergence-trend captures.
var chaosSSEFiles = []string{"dashboard.go", "render.go"}

// chaosSSERenderers reads the payload builders out of the source, so a renderer
// added later needs a fixture instead of passing unseen.
func chaosSSERenderers(t *testing.T) []string {
	t.Helper()

	var names []string

	for _, file := range chaosSSEFiles {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s for its SSE payload builders: %v", file, err)
		}

		for _, match := range chaosRendererDecl.FindAllStringSubmatch(string(src), -1) {
			names = append(names, match[1])
		}
	}

	if len(names) == 0 {
		t.Fatalf("%v declare no payload builder; the capture has lost its renderer list", chaosSSEFiles)
	}

	return names
}

// chaosSSERender builds one fragment from a freshly seeded dashboard.
func chaosSSERender(t *testing.T, c chaosSSECase) []byte {
	t.Helper()

	d := chaosGoldenDashboard()
	defer d.broker.Close()

	got := c.Render(d)
	if strings.TrimSpace(got) == "" {
		t.Fatalf("case %q renders nothing, so the capture would freeze a fragment nobody sees", c.Name)
	}

	return []byte(got)
}

// chaosGoldenDashboard builds the dashboard every case renders.
//
// The state is seeded through ProcessEvent, the same path a live run takes, so
// the counters, the active set, the route matrix, the convergence histogram and
// the timeline transitions hold what the production code puts there rather than
// what a test wrote into the fields.
//
// Every seeded time is StartTime plus a fixed offset, and StartTime is now
// minus chaosGoldenRun. Nothing here reads the wall clock directly, so two
// captures a week apart render the same bytes.
func chaosGoldenDashboard() *Dashboard {
	d := newTestDashboard(chaosGoldenPeers)
	s := d.state

	start := time.Now().Add(-chaosGoldenRun)
	s.StartTime = start
	s.Seed = 4242
	s.WarmupDuration = 5 * time.Minute
	s.ConvergenceDeadline = 250 * time.Millisecond

	at := start.Add

	d.control = make(chan ControlCommand, len(chaosGoldenCases))
	d.routeControl = make(chan ControlCommand, len(chaosGoldenCases))
	d.restartCh = make(chan uint64, 1)

	s.Control = ControlState{
		Rate:             0.35,
		Status:           statusRunning,
		RestartAvailable: true,
		RouteRate:        0.20,
		RouteStatus:      statusRunning,
		Seed:             4242,
		SpeedFactor:      10,
		SpeedAvailable:   true,
	}
	s.Properties = []PropertyBadge{
		{Name: "no-route-leak", Pass: true},
		{Name: "eor-after-establish", Pass: false, Violations: []string{"p1 sent EOR before establish"}},
	}

	chaosGoldenSession(d, at)
	chaosGoldenRoutes(d, at)
	chaosGoldenFaults(d, at)

	// EORCount reaching PeerCount makes ProcessEvent stamp SyncDuration from
	// time.Since. The capture pins it: a fixture must not hold the time the
	// test itself took to seed.
	s.SyncDuration = chaosGoldenSyncDuration

	return d
}

// chaosGoldenSession brings every peer up: established, then EOR.
func chaosGoldenSession(d *Dashboard, at func(time.Duration) time.Time) {
	families := []string{"ipv4/unicast", "ipv6/unicast"}
	vpn := []string{"ipv4/unicast", "ipv6/unicast", "ipv4/vpn"}

	for idx := range chaosGoldenPeers {
		fams := families
		if idx == 4 {
			fams = vpn
		}

		d.ProcessEvent(peer.Event{
			Type:      peer.EventEstablished,
			PeerIndex: idx,
			Time:      at(time.Duration(idx+1) * time.Second),
			Families:  fams,
		})

		d.state.Peers[idx].FamilySentTarget = map[string]int{"ipv4/unicast": 40, "ipv6/unicast": 20}

		d.ProcessEvent(peer.Event{
			Type:      peer.EventEORSent,
			PeerIndex: idx,
			Time:      at(time.Duration(10+idx) * time.Second),
			Count:     1,
			Families:  fams,
			BytesSent: int64(64 + idx),
		})
	}
}

// chaosGoldenRoutes sends one prefix per peer and has every other peer receive
// it, which is what fills the route matrix and the convergence histogram.
func chaosGoldenRoutes(d *Dashboard, at func(time.Duration) time.Time) {
	for src := range chaosGoldenPeers {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 0, byte(src), 0}), 24)
		sent := at(time.Duration(60+src*7) * time.Second)

		d.ProcessEvent(peer.Event{
			Type:      peer.EventRouteSent,
			PeerIndex: src,
			Time:      sent,
			Prefix:    prefix,
			Family:    "ipv4/unicast",
			BytesSent: int64(120 + src*8),
		})

		for dst := range chaosGoldenPeers {
			if dst == src {
				continue
			}

			d.ProcessEvent(peer.Event{
				Type:      peer.EventRouteReceived,
				PeerIndex: dst,
				Time:      sent.Add(time.Duration(15+dst*11) * time.Millisecond),
				Prefix:    prefix,
				Family:    "ipv4/unicast",
				BytesRecv: int64(96 + dst*4),
			})
		}
	}

	d.ProcessEvent(peer.Event{
		Type: peer.EventRouteWithdrawn, PeerIndex: 1, Time: at(500 * time.Second),
		Prefix: netip.MustParsePrefix("10.0.1.0/24"), Family: "ipv4/unicast",
	})
	d.ProcessEvent(peer.Event{
		Type: peer.EventWithdrawalSent, PeerIndex: 2, Time: at(510 * time.Second), Count: 4,
	})
	d.ProcessEvent(peer.Event{
		Type: peer.EventRouteAction, PeerIndex: 3, Time: at(520 * time.Second), RouteAction: "flap",
	})
	d.ProcessEvent(peer.Event{
		Type: peer.EventDroppedEvents, PeerIndex: 0, Time: at(530 * time.Second), Count: 2,
	})
}

// chaosGoldenFaults injects the chaos actions and the session losses the
// timeline, the chaos tabs and the fault filter render.
//
// Every offset is older than the rolling sixty-second window the chaos timeline
// draws beside the overall one. That window ends at now, so an entry inside it
// would sit at a position the clock moves. The overall track holds every marker
// and is a fixed fraction of chaosGoldenRun.
func chaosGoldenFaults(d *Dashboard, at func(time.Duration) time.Time) {
	actions := []struct {
		offset time.Duration
		idx    int
		action string
	}{
		{1000 * time.Second, 1, "disconnect"},
		{4000 * time.Second, 3, "delay"},
		{7000 * time.Second, 1, "corrupt"},
	}

	for _, a := range actions {
		d.ProcessEvent(peer.Event{
			Type:        peer.EventChaosExecuted,
			PeerIndex:   a.idx,
			Time:        at(a.offset),
			ChaosAction: a.action,
		})
	}

	d.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 1, Time: at(1001 * time.Second)})
	d.ProcessEvent(peer.Event{Type: peer.EventReconnecting, PeerIndex: 1, Time: at(1100 * time.Second)})
	d.ProcessEvent(peer.Event{
		Type: peer.EventEstablished, PeerIndex: 1, Time: at(1200 * time.Second),
		Families: []string{"ipv4/unicast", "ipv6/unicast"},
	})
	d.ProcessEvent(peer.Event{
		Type: peer.EventEORSent, PeerIndex: 1, Time: at(1201 * time.Second), Count: 1,
	})
	d.ProcessEvent(peer.Event{Type: peer.EventError, PeerIndex: 5, Time: at(4200 * time.Second)})
}

// chaosGoldenMux registers the real routes, so a case reaches its handler the
// way a browser does and the pattern it matched can be read back.
func chaosGoldenMux(t *testing.T, d *Dashboard) *http.ServeMux {
	t.Helper()

	mux := http.NewServeMux()
	if err := registerRoutes(mux, d); err != nil {
		t.Fatalf("register the dashboard routes: %v", err)
	}

	return mux
}

// chaosGoldenRequest builds one case's request.
func chaosGoldenRequest(t *testing.T, c chaosGoldenCase) *http.Request {
	t.Helper()

	req := httptest.NewRequest(c.Method, c.Target, http.NoBody)
	if len(c.Form) > 0 {
		req = httptest.NewRequest(c.Method, c.Target, strings.NewReader(c.Form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	if c.HTMX {
		req.Header.Set("HX-Request", "true")
	}

	return req
}

// chaosGoldenServe renders one case and returns the fixture bytes.
func chaosGoldenServe(t *testing.T, c chaosGoldenCase) []byte {
	t.Helper()

	d := chaosGoldenDashboard()
	defer d.broker.Close()

	if c.Setup != nil {
		c.Setup(d)
	}

	req := chaosGoldenRequest(t, c)

	if c.Stream {
		// The broker writes until the client leaves. A canceled request ends
		// that loop after the head, which is the part a fixture can hold.
		ctx, cancel := context.WithCancel(req.Context())
		cancel()

		req = req.WithContext(ctx)
	}

	rec := httptest.NewRecorder()
	chaosGoldenMux(t, d).ServeHTTP(rec, req)

	return golden.Response(rec, c.Rewrites)
}

// VALIDATES: every captured chaos response still renders the bytes its fixture
// holds, naming the page when one changes.
// PREVENTS: a markup change reaching the chaos dashboard unseen. Chaos holds
// more than half of ze's htmx attribute occurrences and had no fixture of any
// kind, so a rename across the UI would have shown nothing here.
func TestChaosGoldenOutput(t *testing.T) {
	names := make([]string, 0, len(chaosGoldenCases))
	for _, c := range chaosGoldenCases {
		names = append(names, c.Name)
	}

	golden.AssertUniqueNames(t, "fixture", "chaosGoldenCases", names)

	root := filepath.Join("testdata", chaosGoldenFixtures)

	if !golden.Updating() {
		if _, err := os.Stat(root); err != nil {
			t.Fatalf("fixture directory %s is missing; capture it with -update-golden: %v", root, err)
		}
	}

	mux := chaosGoldenMux(t, chaosGoldenDashboard())
	reached := make([]string, 0, len(chaosGoldenCases))
	written := make([]string, 0, len(chaosGoldenCases)+len(chaosSSECases))

	for _, c := range chaosGoldenCases {
		_, pattern := mux.Handler(chaosGoldenRequest(t, c))
		if pattern == "" {
			t.Errorf("case %q requests %s %s, which no route serves", c.Name, c.Method, c.Target)

			continue
		}

		reached = append(reached, pattern)

		fixture := filepath.Join(root, c.Name+".txt")
		written = append(written, fixture)

		t.Run(c.Name, func(t *testing.T) {
			got := chaosGoldenServe(t, c)

			// Every case here answers the status it declares, and a case that
			// declares none answers 200. A handler that starts answering an
			// error would otherwise have that error captured as the answer the
			// fixture expects.
			want := c.Status
			if want == 0 {
				want = http.StatusOK
			}

			if status := chaosGoldenStatus(t, c.Name, got); status != want {
				t.Fatalf("case %q answered %d, want %d:\n%s", c.Name, status, want, got)
			}

			golden.AssertResponseHasBody(t, c.Name, got, c.Stream)
			golden.Compare(t, fixture, got)
		})
	}

	covered := make([]string, 0, len(chaosSSECases))

	for _, c := range chaosSSECases {
		covered = append(covered, c.Renderer)

		fixture := filepath.Join(root, c.Name+".html")
		written = append(written, fixture)

		t.Run(c.Name, func(t *testing.T) {
			golden.Compare(t, fixture, chaosSSERender(t, c))
		})
	}

	golden.AssertCoversNames(t, "route", "chaosGoldenCases", chaosLiveRoutes(t), reached)
	golden.AssertCoversNames(t, "renderer", "chaosSSECases", chaosSSERenderers(t), covered)
	golden.AssertCoversDir(t, root, "chaosGoldenCases", written)
}

// chaosGoldenStatus reads the status line golden.Response wrote.
func chaosGoldenStatus(t *testing.T, name string, got []byte) int {
	t.Helper()

	line, _, _ := strings.Cut(string(got), "\n")

	status, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "status:")))
	if err != nil {
		t.Fatalf("case %q captures no status line this check can read: %v", name, err)
	}

	return status
}

// chaosLiveRoutes returns every route the dashboard serves, read out of the
// source rather than typed here. A route added to registerRoutes appears in
// this list with no edit, and the coverage check then names it.
func chaosLiveRoutes(t *testing.T) []string {
	t.Helper()

	live, dynamic := golden.RoutePatterns(t, "handlers.go")

	// registerRoutes registers every route from a literal. A computed pattern
	// is a route this capture cannot name, so it is a finding rather than a
	// gap the reader has to notice.
	for _, d := range dynamic {
		t.Errorf("%s registers a route whose pattern is %s; this capture reads handlers.go and cannot name it",
			d.Pos, d.Expr)
	}

	return live
}

// VALIDATES: two captures of one case from identical state hold the same bytes.
// PREVENTS: a fixture that flaps. The chaos dashboard renders elapsed times,
// timeline positions and peer sets out of maps, and any one of them would give
// a gate that fails on runs nobody changed anything in.
//
// It builds the state twice rather than serving one dashboard twice. Go
// randomizes map iteration per range, so a renderer that leaks the order of
// Peers, entries or cells differs between two serves. A second dashboard also
// catches a value seeded from the clock instead of from StartTime.
func TestChaosCaptureIsDeterministic(t *testing.T) {
	for _, c := range chaosGoldenCases {
		t.Run(c.Name, func(t *testing.T) {
			chaosAssertSameTwice(t, c.Name, chaosGoldenServe(t, c), chaosGoldenServe(t, c))
		})
	}

	for _, c := range chaosSSECases {
		t.Run(c.Name, func(t *testing.T) {
			chaosAssertSameTwice(t, c.Name, chaosSSERender(t, c), chaosSSERender(t, c))
		})
	}
}

// chaosAssertSameTwice refuses two renders of one case that hold different
// bytes.
func chaosAssertSameTwice(t *testing.T, name string, first, second []byte) {
	t.Helper()

	if bytes.Equal(first, second) {
		return
	}

	t.Errorf("case %q renders two different answers from identical state, so no fixture can hold it\n%s",
		name, chaosFirstDifference(first, second))
}

// chaosFirstDifference names the byte two renders part company at, with a
// window of each side. A failure then names the value that moved instead of
// printing two whole pages.
func chaosFirstDifference(first, second []byte) string {
	at := 0
	for at < len(first) && at < len(second) && first[at] == second[at] {
		at++
	}

	const window = 60

	from := max(at-window/2, 0)

	var tb textbuf.Buffer

	return tb.Str("first difference at byte ").Int(int64(at)).
		Str("\n  first:  ").Quoted(chaosWindow(first, from, at+window)).
		Str("\n  second: ").Quoted(chaosWindow(second, from, at+window)).String()
}

// chaosWindow cuts a window out of one render, with both ends inside it.
func chaosWindow(b []byte, from, to int) string {
	return string(b[min(from, len(b)):min(to, len(b))])
}
