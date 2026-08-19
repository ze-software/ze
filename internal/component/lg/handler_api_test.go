package lg

import (
	"encoding/json"
	"reflect"
	"testing"
)

// realSummaryJSON is the exact payload handleBgpSummary emits, as
// LGServer.query returns it: the aggregates and the peer rows as siblings at
// the top level, the peer IP under "address" (not "peer-address"), and uptime
// as a Go duration string (not a number).
//
// Producer: internal/component/bgp/plugins/cmd/peer/summary.go, handleBgpSummary.
// Keep this in sync with that handler -- it is the contract this transform
// consumes in production.
const realSummaryJSON = `{` +
	`"router-id":"10.0.0.1","local-as":65000,"uptime":"1h2m3s",` +
	`"peers-configured":1,"peers-established":1,"peers":[{` +
	`"address":"192.0.2.1","name":"peer1","description":"transit",` +
	`"remote-as":65001,"peer-type":"external","state":"established",` +
	`"state-changed":"2026-07-15T10:30:00Z","last-error":"Cease/Administrative Shutdown",` +
	`"uptime":"6m10s","updates-received":10,"updates-sent":5,` +
	`"keepalives-received":100,"keepalives-sent":50,` +
	`"routes-received":60,"routes-accepted":60,"routes-sent":50,` +
	`"eor-received":1,"eor-sent":1,"connections-dropped":0}]}`

// TestTransformProtocolsRealSummaryShape feeds transformProtocols the payload
// the engine actually produces, rather than a hand-built map.
//
// VALIDATES: peers are found at the top-level "peers" key; the peer is keyed
// and addressed from "address"; uptime is a number of seconds.
// PREVENTS: the /api/looking-glass/protocols/bgp endpoint returning an empty
// protocols map. The two sides once disagreed about where the rows live, so
// every peer was dropped and Alice-LG saw no sessions at all.
func TestTransformProtocolsRealSummaryShape(t *testing.T) {
	var ze map[string]any
	if err := json.Unmarshal([]byte(realSummaryJSON), &ze); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	bw := transformProtocols(ze)

	protocols, ok := bw["protocols"].(map[string]any)
	if !ok {
		t.Fatal("missing protocols map")
	}
	if len(protocols) != 1 {
		t.Fatalf("protocols has %d entries, want 1 (peers were dropped)", len(protocols))
	}

	peer, ok := protocols["peer1"].(map[string]any)
	if !ok {
		t.Fatal("missing peer1 in protocols")
	}

	if got, _ := peer["neighbor_address"].(string); got != "192.0.2.1" {
		t.Errorf("neighbor_address = %q, want %q", got, "192.0.2.1")
	}
	if got, _ := peer["neighbor_as"].(float64); got != 65001 {
		t.Errorf("neighbor_as = %v, want 65001", got)
	}
	// "6m10s" -> 370 seconds. Alice-LG expects a number, not the raw string.
	if got, _ := peer["uptime"].(float64); got != 370 {
		t.Errorf("uptime = %v, want 370", got)
	}
}

// TestTransformProtocolsStateChangedAndLastError verifies the two fields that
// handleBgpSummary now emits reach the birdwatcher output.
//
// VALIDATES: state_changed and last_error are populated from the real payload
// (AC-6).
// PREVENTS: regressing to the state where transformProtocols read `state-changed`
// and `last-error` that no producer emitted, so Alice-LG showed a blank "since"
// and never said why a peer went down.
func TestTransformProtocolsStateChangedAndLastError(t *testing.T) {
	var ze map[string]any
	if err := json.Unmarshal([]byte(realSummaryJSON), &ze); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	bw := transformProtocols(ze)

	protocols, ok := bw["protocols"].(map[string]any)
	if !ok {
		t.Fatal("missing protocols map")
	}
	peer, ok := protocols["peer1"].(map[string]any)
	if !ok {
		t.Fatal("missing peer1 in protocols")
	}

	if got, _ := peer["state_changed"].(string); got != "2026-07-15T10:30:00Z" {
		t.Errorf("state_changed = %q, want %q", got, "2026-07-15T10:30:00Z")
	}
	if got, _ := peer["last_error"].(string); got != "Cease/Administrative Shutdown" {
		t.Errorf("last_error = %q, want %q", got, "Cease/Administrative Shutdown")
	}
}

// TestTransformProtocolsRouteCountsFromRealSummary verifies the birdwatcher
// route-count fields are populated from the summary's per-peer keys.
//
// VALIDATES: AC-7 — routes_received/routes_imported come from routes-received/
// routes-accepted, routes_exported from routes-sent, and routes_filtered stays 0
// (Ze does not retain filtered routes).
// PREVENTS: the endpoint reporting all-zero route counts, which is what it did
// before summary.go emitted the keys (the consumer was always wired).
func TestTransformProtocolsRouteCountsFromRealSummary(t *testing.T) {
	var ze map[string]any
	if err := json.Unmarshal([]byte(realSummaryJSON), &ze); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	bw := transformProtocols(ze)
	protocols, ok := bw["protocols"].(map[string]any)
	if !ok {
		t.Fatal("missing protocols map")
	}
	peer, ok := protocols["peer1"].(map[string]any)
	if !ok {
		t.Fatal("missing peer1 in protocols")
	}

	if got, _ := peer["routes_received"].(float64); got != 60 {
		t.Errorf("routes_received = %v, want 60", got)
	}
	if got, _ := peer["routes_imported"].(float64); got != 60 {
		t.Errorf("routes_imported = %v, want 60", got)
	}
	if got, _ := peer["routes_exported"].(float64); got != 50 {
		t.Errorf("routes_exported = %v, want 50", got)
	}
	if got, _ := peer["routes_filtered"].(float64); got != 0 {
		t.Errorf("routes_filtered = %v, want 0 (Ze does not retain filtered routes)", got)
	}
}

// TestTransformProtocolsShortSinceFromRealSummary verifies the short format's
// `since` is populated from the same producer key.
//
// VALIDATES: AC-7.
// PREVENTS: /api/looking-glass/protocols returning peers with an empty `since`.
func TestTransformProtocolsShortSinceFromRealSummary(t *testing.T) {
	var ze map[string]any
	if err := json.Unmarshal([]byte(realSummaryJSON), &ze); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	bw := transformProtocolsShort(ze)

	protocols, ok := bw["protocols"].(map[string]any)
	if !ok {
		t.Fatal("missing protocols map")
	}
	peer, ok := protocols["peer1"].(map[string]any)
	if !ok {
		t.Fatal("missing peer1 in protocols")
	}
	if got, _ := peer["since"].(string); got != "2026-07-15T10:30:00Z" {
		t.Errorf("since = %q, want %q", got, "2026-07-15T10:30:00Z")
	}
}

// TestTransformProtocolsShortRealSummaryShape is the short-format counterpart.
//
// VALIDATES: the short protocols endpoint finds peers at the top-level "peers".
// PREVENTS: /api/looking-glass/protocols returning an empty map for the same
// misplaced-rows reason as the full transform.
func TestTransformProtocolsShortRealSummaryShape(t *testing.T) {
	var ze map[string]any
	if err := json.Unmarshal([]byte(realSummaryJSON), &ze); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	bw := transformProtocolsShort(ze)

	protocols, ok := bw["protocols"].(map[string]any)
	if !ok {
		t.Fatal("missing protocols map")
	}
	if len(protocols) != 1 {
		t.Fatalf("protocols has %d entries, want 1 (peers were dropped)", len(protocols))
	}

	peer, ok := protocols["peer1"].(map[string]any)
	if !ok {
		t.Fatal("missing peer1 in protocols")
	}
	if got, _ := peer["state"].(string); got != "established" {
		t.Errorf("state = %q, want %q", got, "established")
	}
}

// TestUptimeSecondsFormats verifies uptime coercion across both producer shapes.
//
// VALIDATES: Go duration strings convert to seconds; numeric input passes through.
// PREVENTS: regressing to getNum's behavior, which had no string case and so
// silently returned 0 for every real engine response.
func TestUptimeSecondsFormats(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want float64
	}{
		{"duration string", "6m10s", 370},
		{"hours", "1h2m3s", 3723},
		{"zero", "0s", 0},
		{"numeric passthrough", float64(3600), 3600},
		{"unparseable", "not-a-duration", 0},
		{"missing", nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peer := map[string]any{}
			if tc.in != nil {
				peer["uptime"] = tc.in
			}
			if got := uptimeSeconds(peer); got != tc.want {
				t.Errorf("uptimeSeconds(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestTransformStatusFields(t *testing.T) {
	// VALIDATES: birdwatcher status field mapping from ze JSON.
	// PREVENTS: wrong field names or values in API response.
	ze := map[string]any{
		"router-id":          "1.2.3.4",
		"version":            "26.03.30",
		"start-time":         "2026-01-01T00:00:00Z",
		"last-config-change": "2026-03-01T12:00:00Z",
	}

	bw := transformStatus(ze)

	status, ok := bw["status"].(map[string]any)
	if !ok {
		t.Fatal("missing status map")
	}

	checks := map[string]string{
		"router_id":     "1.2.3.4",
		"last_reboot":   "2026-01-01T00:00:00Z",
		"last_reconfig": "2026-03-01T12:00:00Z",
		"message":       "Ze BGP daemon",
		"version":       "26.03.30",
	}
	for key, want := range checks {
		got, _ := status[key].(string)
		if got != want {
			t.Errorf("status[%q] = %q, want %q", key, got, want)
		}
	}

	if _, ok := status["server_time"]; !ok {
		t.Error("missing server_time field")
	}
	if _, ok := status["current_server"]; !ok {
		t.Error("missing current_server field (required by Alice-LG)")
	}

	api, ok := bw["api"].(map[string]any)
	if !ok {
		t.Fatal("missing api map")
	}
	if api["Version"] != "Ze Looking Glass" {
		t.Errorf("api.Version = %v, want Ze Looking Glass", api["Version"])
	}
	if api["result_from_cache"] != false {
		t.Errorf("api.result_from_cache = %v, want false", api["result_from_cache"])
	}
}

func TestTransformProtocolsFields(t *testing.T) {
	// VALIDATES: birdwatcher protocol field mapping, including name fallback.
	// PREVENTS: missing or wrong field values in peer list.
	ze := map[string]any{
		"peers": []any{
			map[string]any{
				"name":            "peer1",
				"peer-address":    "10.0.0.1",
				"remote-as":       float64(65001),
				"state":           "established",
				"state-changed":   "2026-01-15T10:00:00Z",
				"description":     "test peer",
				"last-error":      "hold timer expired",
				"routes-received": float64(100),
				"routes-accepted": float64(95),
				"routes-sent":     float64(50),
				"routes-filtered": float64(5),
				"uptime":          float64(3600),
			},
		},
	}

	bw := transformProtocols(ze)

	protocols, ok := bw["protocols"].(map[string]any)
	if !ok {
		t.Fatal("missing protocols map")
	}

	peer, ok := protocols["peer1"].(map[string]any)
	if !ok {
		t.Fatal("missing peer1 in protocols")
	}

	strChecks := map[string]string{
		"bird_protocol":    "peer1",
		"state":            "established",
		"state_changed":    "2026-01-15T10:00:00Z",
		"neighbor_address": "10.0.0.1",
		"description":      "test peer",
		"last_error":       "hold timer expired",
		"table":            "master",
	}
	for key, want := range strChecks {
		got, _ := peer[key].(string)
		if got != want {
			t.Errorf("peer[%q] = %q, want %q", key, got, want)
		}
	}

	numChecks := map[string]float64{
		"neighbor_as":     65001,
		"routes_received": 100,
		"routes_imported": 95,
		"routes_exported": 50,
		"routes_filtered": 5,
		"uptime":          3600,
	}
	for key, want := range numChecks {
		got, _ := peer[key].(float64)
		if got != want {
			t.Errorf("peer[%q] = %v, want %v", key, got, want)
		}
	}

	// Nested routes object for Alice-LG.
	routes, ok := peer["routes"].(map[string]any)
	if !ok {
		t.Fatal("missing nested routes object")
	}
	if routes["imported"] != float64(95) {
		t.Errorf("routes.imported = %v, want 95", routes["imported"])
	}
	if routes["filtered"] != float64(5) {
		t.Errorf("routes.filtered = %v, want 5", routes["filtered"])
	}
	if routes["exported"] != float64(50) {
		t.Errorf("routes.exported = %v, want 50", routes["exported"])
	}
}

func TestTransformProtocolsNameFallback(t *testing.T) {
	// VALIDATES: peer without name uses peer-address as key.
	ze := map[string]any{
		"peers": []any{
			map[string]any{
				"peer-address": "10.0.0.1",
				"state":        "idle",
			},
		},
	}

	bw := transformProtocols(ze)
	protocols, _ := bw["protocols"].(map[string]any)

	if _, ok := protocols["10.0.0.1"]; !ok {
		t.Error("expected peer keyed by address when name is missing")
	}
}

func TestTransformProtocolsEmptyPeers(t *testing.T) {
	// VALIDATES: empty peer list produces empty protocols.
	ze := map[string]any{"peers": []any{}}
	bw := transformProtocols(ze)
	protocols, _ := bw["protocols"].(map[string]any)
	if len(protocols) != 0 {
		t.Errorf("expected 0 protocols, got %d", len(protocols))
	}
}

func TestTransformRoutesFields(t *testing.T) {
	// VALIDATES: birdwatcher route field mapping including nested bgp fields.
	// PREVENTS: wrong field names or values in route response.
	ze := map[string]any{
		"routes": []any{
			map[string]any{
				"prefix":           "10.0.0.0/24",
				"next-hop":         "10.0.0.1",
				"origin":           "igp",
				"as-path":          []any{float64(65001), float64(65002)},
				"local-preference": float64(100),
				"med":              float64(50),
				"community":        []any{"65000:100"},
				"large-community":  []any{"65000:0:100"},
				"peer-address":     "10.0.0.1",
			},
		},
	}

	bw := transformRoutes(ze, "fallback-peer")

	routes, ok := bw["routes"].([]any)
	if !ok || len(routes) != 1 {
		t.Fatalf("expected 1 route, got %v", bw["routes"])
	}

	route, _ := routes[0].(map[string]any)
	if route["network"] != "10.0.0.0/24" {
		t.Errorf("network = %v, want 10.0.0.0/24", route["network"])
	}
	if route["gateway"] != "10.0.0.1" {
		t.Errorf("gateway = %v, want 10.0.0.1", route["gateway"])
	}
	// from_protocol overridden by peer-address.
	if route["from_protocol"] != "10.0.0.1" {
		t.Errorf("from_protocol = %v, want 10.0.0.1 (override from peer-address)", route["from_protocol"])
	}
	// learnt_from from peer-address.
	if route["learnt_from"] != "10.0.0.1" {
		t.Errorf("learnt_from = %v, want 10.0.0.1", route["learnt_from"])
	}

	bgp, ok := route["bgp"].(map[string]any)
	if !ok {
		t.Fatal("missing bgp sub-map")
	}
	if bgp["origin"] != "igp" {
		t.Errorf("bgp.origin = %v, want igp", bgp["origin"])
	}
	if bgp["local_pref"] != float64(100) {
		t.Errorf("bgp.local_pref = %v, want 100", bgp["local_pref"])
	}
	if bgp["med"] != float64(50) {
		t.Errorf("bgp.med = %v, want 50", bgp["med"])
	}

	// Communities converted to integer-pair format.
	communities, ok := bgp["communities"].([]any)
	if !ok || len(communities) != 1 {
		t.Fatalf("expected 1 community, got %v", bgp["communities"])
	}
	comm, ok := communities[0].([]any)
	if !ok || len(comm) != 2 {
		t.Fatalf("community should be [int,int], got %v", communities[0])
	}
	if comm[0] != 65000 || comm[1] != 100 {
		t.Errorf("community = %v, want [65000, 100]", comm)
	}

	// Large communities converted to integer-triple format.
	largeCommunities, ok := bgp["large_communities"].([]any)
	if !ok || len(largeCommunities) != 1 {
		t.Fatalf("expected 1 large community, got %v", bgp["large_communities"])
	}
	lc, ok := largeCommunities[0].([]any)
	if !ok || len(lc) != 3 {
		t.Fatalf("large community should be [int,int,int], got %v", largeCommunities[0])
	}
	if lc[0] != 65000 || lc[1] != 0 || lc[2] != 100 {
		t.Errorf("large community = %v, want [65000, 0, 100]", lc)
	}

	count, _ := bw["routes_count"].(int)
	if count != 1 {
		t.Errorf("routes_count = %v, want 1", bw["routes_count"])
	}
}

func TestTransformRoutesPrefixesFallback(t *testing.T) {
	// VALIDATES: routes key fallback to prefixes.
	ze := map[string]any{
		"prefixes": []any{
			map[string]any{"prefix": "10.0.0.0/24"},
		},
	}

	bw := transformRoutes(ze, "")
	routes, _ := bw["routes"].([]any)
	if len(routes) != 1 {
		t.Errorf("expected 1 route via prefixes fallback, got %d", len(routes))
	}
}

func TestTransformRoutesEmptyNotNull(t *testing.T) {
	// VALIDATES: empty routes produces [] not null in JSON.
	// PREVENTS: Alice-LG breaking on null routes array.
	ze := map[string]any{}

	bw := transformRoutes(ze, "")
	routes, ok := bw["routes"].([]any)
	if !ok {
		t.Fatal("routes should be []any, not nil")
	}
	if routes == nil {
		t.Fatal("routes should be empty slice, not nil (json: [] not null)")
	}
	if len(routes) != 0 {
		t.Errorf("expected 0 routes, got %d", len(routes))
	}
}

func TestGetStr(t *testing.T) {
	// VALIDATES: string extraction from map with type fallback.
	// PREVENTS: panic on missing key or non-string value.
	m := map[string]any{
		"str":  "hello",
		"num":  float64(42),
		"nil":  nil,
		"bool": true,
	}

	tests := []struct {
		key  string
		want string
	}{
		{"str", "hello"},
		{"num", "42"},
		{"nil", ""},
		{"bool", "true"},
		{"missing", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := getStr(m, tt.key)
			if got != tt.want {
				t.Errorf("getStr(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestTransformBMPProtocolsFields(t *testing.T) {
	ze := map[string]any{
		"peers": []any{
			map[string]any{
				"router":      "10.0.0.1:12345",
				"peer-as":     float64(64501),
				"peer-bgp-id": "192.168.1.1",
				"up":          true,
			},
			map[string]any{
				"router":      "10.0.0.1:12345",
				"peer-as":     float64(64502),
				"peer-bgp-id": "192.168.2.1",
				"up":          false,
			},
		},
	}

	bw := transformBMPProtocols(ze)

	protocols, ok := bw["protocols"].(map[string]any)
	if !ok {
		t.Fatal("missing protocols map")
	}

	if len(protocols) != 2 {
		t.Fatalf("expected 2 protocols, got %d", len(protocols))
	}

	p1, ok := protocols["10.0.0.1:12345:192.168.1.1"].(map[string]any)
	if !ok {
		t.Fatal("missing protocol for first peer")
	}
	if p1["state"] != "up" {
		t.Errorf("state = %v, want up", p1["state"])
	}
	if p1["neighbor_as"] != float64(64501) {
		t.Errorf("neighbor_as = %v, want 64501", p1["neighbor_as"])
	}
	if p1["table"] != "bmp" {
		t.Errorf("table = %v, want bmp", p1["table"])
	}

	p2, ok := protocols["10.0.0.1:12345:192.168.2.1"].(map[string]any)
	if !ok {
		t.Fatal("missing protocol for second peer")
	}
	if p2["state"] != "down" {
		t.Errorf("state = %v, want down", p2["state"])
	}
}

func TestTransformBMPProtocolsEmpty(t *testing.T) {
	ze := map[string]any{}

	bw := transformBMPProtocols(ze)

	protocols, ok := bw["protocols"].(map[string]any)
	if !ok {
		t.Fatal("missing protocols map")
	}
	if len(protocols) != 0 {
		t.Errorf("expected 0 protocols, got %d", len(protocols))
	}
}

func TestLGBGPProtocolsExcludesBMP(t *testing.T) {
	ze := map[string]any{
		"peers": []any{
			map[string]any{
				"name":         "peer1",
				"peer-address": "10.0.0.1",
				"state":        "established",
				"remote-as":    float64(64501),
			},
		},
	}

	bw := transformProtocols(ze)

	protocols, ok := bw["protocols"].(map[string]any)
	if !ok {
		t.Fatal("missing protocols map")
	}
	for name := range protocols {
		if name == "bmp" {
			t.Error("BGP protocols should not include BMP entries")
		}
	}
}

func TestTransformCommunities(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   any
		want any
	}{
		{"numeric pair", []any{"65000:100"}, []any{[]any{65000, 100}}},
		{"well-known no-export", []any{"no-export"}, []any{[]any{65535, 65281}}},
		{"well-known no-advertise", []any{"no-advertise"}, []any{[]any{65535, 65282}}},
		{"well-known nopeer", []any{"nopeer"}, []any{[]any{65535, 65284}}},
		{"well-known graceful-shutdown", []any{"graceful-shutdown"}, []any{[]any{65535, 0}}},
		{"well-known accept-own", []any{"accept-own"}, []any{[]any{65535, 1}}},
		{"well-known route-filter-v4", []any{"route-filter-v4"}, []any{[]any{65535, 3}}},
		{"well-known route-filter-v6", []any{"route-filter-v6"}, []any{[]any{65535, 5}}},
		{"well-known llgr-stale", []any{"llgr-stale"}, []any{[]any{65535, 6}}},
		{"well-known no-llgr", []any{"no-llgr"}, []any{[]any{65535, 7}}},
		{"well-known accept-own-nexthop", []any{"accept-own-nexthop"}, []any{[]any{65535, 8}}},
		{"well-known standby-pe", []any{"standby-pe"}, []any{[]any{65535, 9}}},
		{"well-known blackhole", []any{"blackhole"}, []any{[]any{65535, 666}}},
		{"mixed", []any{"65000:100", "no-export"}, []any{[]any{65000, 100}, []any{65535, 65281}}},
		{"nil input", nil, nil},
		{"empty array", []any{}, []any(nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := transformCommunities(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("transformCommunities(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestGetNum(t *testing.T) {
	// VALIDATES: numeric extraction from map with type handling.
	// PREVENTS: panic or wrong value for different numeric types.
	m := map[string]any{
		"f64":   float64(3.14),
		"int":   int(42),
		"int64": int64(1000),
		"str":   "not a number",
		"nil":   nil,
	}

	tests := []struct {
		key  string
		want float64
	}{
		{"f64", 3.14},
		{"int", 42},
		{"int64", 1000},
		{"str", 0},
		{"nil", 0},
		{"missing", 0},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := getNum(m, tt.key)
			if got != tt.want {
				t.Errorf("getNum(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

// TestRouteCountsAvailableFlagsFabricatedZeros pins the field that tells a
// consumer whether the four route counts mean anything.
//
// VALIDATES: routes_counts_available is true only when the producer actually
// supplied counts, and the counts themselves keep their compatibility zeros.
// PREVENTS: the fabricated zero being indistinguishable from a real one.
// fetchRibRouteCounts OMITS the keys when bgp-rib is not loaded and says they
// are "never faked to 0"; getNum then returns 0 for the missing key and this
// transform published it, so an operator could not tell "no routes" from "Ze
// cannot tell you" (ai/rules/evidence.md).
//
// Upstream birdwatcher omits the key entirely in this case (bird/parser.go,
// setChangeCount, returns early on BIRD's "---"). Ze keeps the zero for
// Alice-LG compatibility by owner decision of 2026-08-05 and carries the truth
// beside it; docs/architecture/api/birdwatcher-compat.md records both.
func TestRouteCountsAvailableFlagsFabricatedZeros(t *testing.T) {
	withCounts := map[string]any{
		"peers": []any{map[string]any{
			"address": "192.0.2.1", "name": "peer1", "state": "established",
			"routes-received": float64(60), "routes-accepted": float64(60), "routes-sent": float64(50),
		}},
	}
	withoutCounts := map[string]any{
		"peers": []any{map[string]any{
			"address": "192.0.2.2", "name": "peer2", "state": "established",
		}},
	}

	got := func(ze map[string]any) map[string]any {
		bw := transformProtocols(ze)
		protocols, ok := bw["protocols"].(map[string]any)
		if !ok || len(protocols) != 1 {
			t.Fatalf("expected exactly one protocol, got %v", bw["protocols"])
		}
		for _, p := range protocols {
			m, ok := p.(map[string]any)
			if !ok {
				t.Fatalf("protocol is not an object: %T", p)
			}
			return m
		}
		return nil
	}

	present := got(withCounts)
	if present["routes_counts_available"] != true {
		t.Errorf("counts supplied: routes_counts_available = %v, want true", present["routes_counts_available"])
	}
	if present["routes_received"] != float64(60) {
		t.Errorf("counts supplied: routes_received = %v, want 60", present["routes_received"])
	}

	absent := got(withoutCounts)
	if absent["routes_counts_available"] != false {
		t.Errorf("counts absent: routes_counts_available = %v, want false", absent["routes_counts_available"])
	}
	if absent["routes_received"] != float64(0) {
		t.Errorf("counts absent: routes_received = %v, want the compatibility zero", absent["routes_received"])
	}
}

// TestBMPProtocolsDeclareCountsUnavailable covers G-3, the same defect without
// even the producer's honesty behind it.
//
// VALIDATES: transformBMPProtocols admits its four counts are placeholders.
// PREVENTS: a BMP-monitored peer reporting a confident zero for counts no
// source was ever consulted for.
func TestBMPProtocolsDeclareCountsUnavailable(t *testing.T) {
	ze := map[string]any{
		"peers": []any{map[string]any{
			"router": "r1", "peer-bgp-id": "10.0.0.1",
			"peer-as": float64(65001), "up": true,
		}},
	}

	bw := transformBMPProtocols(ze)
	protocols, ok := bw["protocols"].(map[string]any)
	if !ok || len(protocols) == 0 {
		t.Fatalf("expected at least one BMP protocol, got %v", bw["protocols"])
	}
	for name, p := range protocols {
		m, ok := p.(map[string]any)
		if !ok {
			t.Fatalf("protocol %s is not an object", name)
		}
		if m["routes_counts_available"] != false {
			t.Errorf("%s: routes_counts_available = %v, want false (no source is consulted)",
				name, m["routes_counts_available"])
		}
	}
}

// TestSummaryPeersReadsFlatPayload verifies the looking-glass API finds the
// peer rows in the payload handleBgpSummary now answers, where the aggregates
// and the rows are siblings.
//
// VALIDATES: AC-3 — summaryPeers returns the rows from the top-level "peers"
// key, for the real payload and for the array parseJSON promotes.
// PREVENTS: the public peer table rendering empty. A missing key unmarshals to
// a zero value, so a reader that looked in the wrong place would answer no
// peers rather than an error.
func TestSummaryPeersReadsFlatPayload(t *testing.T) {
	var ze map[string]any
	if err := json.Unmarshal([]byte(realSummaryJSON), &ze); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	peers := summaryPeers(ze)
	if len(peers) != 1 {
		t.Fatalf("summaryPeers returned %d rows, want 1", len(peers))
	}
	row, ok := peers[0].(map[string]any)
	if !ok {
		t.Fatalf("row is not an object: %T", peers[0])
	}
	if got, _ := row["address"].(string); got != "192.0.2.1" {
		t.Errorf("address = %q, want %q", got, "192.0.2.1")
	}

	// parseJSON promotes a bare array to the same key, so a producer that
	// answers only the rows reaches the same reader.
	promoted := map[string]any{"peers": []any{map[string]any{"address": "192.0.2.9"}}}
	if got := summaryPeers(promoted); len(got) != 1 {
		t.Fatalf("promoted array: %d rows, want 1", len(got))
	}

	// A payload with no rows answers none, rather than panicking.
	if got := summaryPeers(map[string]any{"router-id": "10.0.0.1"}); got != nil {
		t.Errorf("payload without peers returned %v, want nil", got)
	}
}
