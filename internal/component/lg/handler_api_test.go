package lg

import (
	"testing"
)

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
