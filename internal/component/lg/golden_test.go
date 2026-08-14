// Related: render.go -- parseLGTemplates builds the template set captured here
// Related: handler_ui.go -- the handlers that supply the data shapes below

package lg

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/golden"
)

// lgGolden is the golden harness over the lg template tree. The spec below, the
// fixture data and the ExecuteTemplate call stay in this package. The walk over
// the FS, the coverage check, the fixture path rule and the byte comparison are
// shared with internal/component/web through internal/test/golden.
//
// Recapture a deliberate markup change with `make ze-web-golden-update`.
var lgGolden = golden.Set{
	FS:      templatesFS,
	Dir:     "templates",
	Spec:    lgGoldenSpec,
	SpecVar: "lgGoldenSpec",
}

// lgGoldenSpec maps each file in the embedded template FS to the templates it
// defines and the data each one renders with. TestLGGoldenOutput compares this
// map against the FS and fails when the two disagree. A new template file, or a
// new {{define}} inside an existing file, fails until it is captured here.
var lgGoldenSpec = golden.Spec{
	"templates/error.html": {{
		Name: "error_banner",
		Variants: []golden.Variant{
			{Name: "set", Data: map[string]any{"Error": "engine unreachable"}},
			{Name: "clear", Data: map[string]any{"Error": ""}},
		},
	}},
	"templates/help.html": {{
		Name:     "help",
		Variants: []golden.Variant{{Data: map[string]any{"Title": "Help", "ActiveTab": "help"}}},
	}},
	"templates/layout.html": {{
		Name: "layout",
		Variants: []golden.Variant{
			{Name: "peers", Data: lgLayoutData("Peers", "peers")},
			{Name: "search", Data: lgLayoutData("Route Search", "search")},
		},
	}},
	"templates/peers.html": {
		{Name: "peers", Variants: lgPeersVariants()},
		{Name: "peers_content", Variants: lgPeersVariants()},
		{Name: "peers_table_body", Variants: lgPeersVariants()},
		{Name: "bmp_peers_content", Variants: []golden.Variant{
			{Data: map[string]any{"BMPPeers": lgBMPPeerRows()}},
		}},
		{Name: "peer_detail_area", Variants: []golden.Variant{{Data: map[string]any{}}}},
	},
	"templates/peer_routes.html": {
		{Name: "peer_routes", Variants: lgPeerRoutesVariants()},
		{Name: "peer_routes_content", Variants: lgPeerRoutesVariants()},
	},
	"templates/route_detail.html": {{
		Name: "route_detail",
		Variants: []golden.Variant{
			{Name: "found", Data: map[string]any{
				"Route":  lgRouteRows()[0],
				"Prefix": "203.0.113.0/24",
			}},
			{Name: "missing", Data: map[string]any{
				"Route":  nil,
				"Prefix": "203.0.113.0/24",
			}},
		},
	}},
	"templates/route_table.html": {{
		Name:     "route_results",
		Variants: lgRouteResultsVariants(),
	}},
	"templates/search.html": {
		{Name: "search", Variants: lgSearchVariants()},
		{Name: "search_form", Variants: lgSearchVariants()},
	},
}

// lgLayoutData reproduces the map renderPage builds for the layout template.
func lgLayoutData(title, tab string) map[string]any {
	return map[string]any{
		"Title":     title,
		"ActiveTab": tab,
		"Content":   template.HTML("<p>page content</p>"), //nolint:gosec // fixed test fixture
	}
}

// lgPeerRows reproduces the entries extractPeers builds. The two rows differ in
// RemoteASName and State so both branches of each conditional render.
func lgPeerRows() []map[string]any {
	return []map[string]any{
		{
			"Address": "192.0.2.1", "RemoteAS": "64500", "RemoteASName": "Example Transit",
			"State": "established", "Uptime": "3h12m4s",
			"RoutesReceived": "1200", "RoutesAccepted": "1180", "RoutesSent": "42",
			"UpdatesReceived": "9000", "UpdatesSent": "12",
			"Description": "transit", "Name": "upstream",
		},
		{
			"Address": "2001:db8::1", "RemoteAS": "64501", "RemoteASName": "",
			"State": "idle", "Uptime": "0s",
			"RoutesReceived": "0", "RoutesAccepted": "0", "RoutesSent": "0",
			"UpdatesReceived": "0", "UpdatesSent": "0",
			"Description": "", "Name": "peer2",
		},
	}
}

// lgBMPPeerRows reproduces the entries extractBMPPeers builds. The two rows
// differ in State, PeerASName and IPv6 so both branches of each render.
func lgBMPPeerRows() []map[string]any {
	return []map[string]any{
		{
			"Router": "198.51.100.7", "PeerAS": "64502", "PeerASName": "Example Two",
			"BGPID": "198.51.100.1", "State": "up", "IPv6": false,
		},
		{
			"Router": "198.51.100.8", "PeerAS": "64503", "PeerASName": "",
			"BGPID": "198.51.100.2", "State": "down", "IPv6": true,
		},
	}
}

// lgRouteRows reproduces the maps extractRoutes yields. The first is the best
// path and the second is not, so both branches of isBest render.
func lgRouteRows() []any {
	return []any{
		map[string]any{
			"prefix": "203.0.113.0/24", "next-hop": "192.0.2.1",
			"as-path":            []any{float64(64500), float64(64510)},
			"origin":             "igp",
			"local-preference":   float64(100),
			"med":                float64(0),
			"peer-address":       "192.0.2.1",
			"best":               true,
			"community":          []any{"64500:100", "64500:200"},
			"large-community":    []any{"64500:1:2"},
			"extended-community": []any{"rt:64500:7"},
		},
		map[string]any{
			"prefix": "198.51.100.0/24", "next-hop": "2001:db8::1",
			"as-path":          []any{float64(64501)},
			"origin":           "incomplete",
			"local-preference": float64(90),
			"med":              float64(5),
			"peer-address":     "2001:db8::1",
			"best":             false,
		},
	}
}

func lgPeersVariants() []golden.Variant {
	return []golden.Variant{
		{Name: "full", Data: map[string]any{
			"Title":     "Peers",
			"ActiveTab": "peers",
			"Peers":     lgPeerRows(),
			"BGPPeers":  lgPeerRows(),
			"BMPPeers":  lgBMPPeerRows(),
			"Error":     "",
		}},
		{Name: "empty", Data: map[string]any{
			"Title":     "Peers",
			"ActiveTab": "peers",
			"Peers":     nil,
			"BGPPeers":  nil,
			"BMPPeers":  nil,
			"Error":     "engine unreachable",
		}},
	}
}

func lgPeerRoutesVariants() []golden.Variant {
	return []golden.Variant{
		{Name: "routes", Data: map[string]any{
			"Title": "Routes from 192.0.2.1", "ActiveTab": "peers",
			"Address": "192.0.2.1",
			"Peer": map[string]any{
				"state": "established", "remote-as": "64500",
				"remote-as-name": "Example Transit", "description": "transit",
			},
			"PrefixSummary": nil,
			"Count":         2,
			"Routes":        lgRouteRows(),
			"Error":         "",
		}},
		{Name: "summary", Data: map[string]any{
			"Title": "Routes from 192.0.2.1", "ActiveTab": "peers",
			"Address": "192.0.2.1",
			"Peer": map[string]any{
				"state": "idle", "remote-as": "64501",
				"remote-as-name": "", "description": "",
			},
			"PrefixSummary": []map[string]any{
				{"Family": "ipv4 unicast", "Length": 24, "Count": 1200},
				{"Family": "ipv6 unicast", "Length": 48, "Count": 30},
			},
			"Count":  1230,
			"Routes": nil,
			"Error":  "",
		}},
		{Name: "empty", Data: map[string]any{
			"Title": "Routes from 192.0.2.1", "ActiveTab": "peers",
			"Address":       "192.0.2.1",
			"Peer":          nil,
			"PrefixSummary": nil,
			"Count":         0,
			"Routes":        nil,
			"Error":         "",
		}},
	}
}

func lgRouteResultsVariants() []golden.Variant {
	return []golden.Variant{
		{Name: "routes", Data: map[string]any{
			"Title": "Route Search", "ActiveTab": "search",
			"Prefix": "203.0.113.0/24", "ASPath": "64500 64510",
			"Community": "64500:100", "Family": "ipv4/unicast",
			"Routes": lgRouteRows(), "Count": 2, "Error": "",
		}},
		{Name: "no-prefix", Data: map[string]any{
			"Title": "Route Search", "ActiveTab": "search",
			"Prefix": "", "ASPath": "64500", "Community": "", "Family": "",
			"Routes": lgRouteRows(), "Count": 2, "Error": "",
		}},
		{Name: "empty", Data: map[string]any{
			"Title": "Route Search", "ActiveTab": "search",
			"Prefix": "", "ASPath": "", "Community": "", "Family": "",
			"Routes": nil, "Count": 0, "Error": "",
		}},
		{Name: "error", Data: map[string]any{
			"Title": "Route Search", "ActiveTab": "search",
			"Prefix": "bad", "ASPath": "", "Community": "", "Family": "",
			"Routes": nil, "Count": 0, "Error": "invalid prefix",
		}},
	}
}

func lgSearchVariants() []golden.Variant {
	return []golden.Variant{
		{Name: "blank", Data: map[string]any{
			"Title": "Route Search", "ActiveTab": "search",
			"Prefix": "", "ASPath": "", "Community": "", "Family": "",
		}},
		{Name: "filled", Data: map[string]any{
			"Title": "Route Search", "ActiveTab": "search",
			"Prefix": "203.0.113.0/24", "ASPath": "64500 64510",
			"Community": "64500:100", "Family": "ipv6/unicast",
			"Routes": lgRouteRows(), "Count": 2, "Error": "",
		}},
	}
}

// TestLGGoldenOutput captures the rendered bytes of every lg template and
// compares them against the committed fixtures.
//
// VALIDATES: the lg template set renders byte for byte what it rendered when
// the fixtures were captured.
// PREVENTS: a rendering-engine change that keeps every substring assertion
// green and still moves the bytes an operator receives.
func TestLGGoldenOutput(t *testing.T) {
	tpl, err := parseLGTemplates()
	if err != nil {
		t.Fatalf("parse lg templates: %v", err)
	}

	files := lgGolden.Files(t)
	lgGolden.AssertCoversFS(t, files)

	root := filepath.Join("testdata", "golden")
	if !golden.Updating() {
		if _, statErr := os.Stat(root); statErr != nil {
			t.Fatalf("fixture directory %s is missing; capture it with -update-golden: %v", root, statErr)
		}
	}

	for _, file := range files {
		for _, unit := range lgGolden.Spec[file] {
			content := false

			for _, variant := range unit.Variants {
				name := unit.FixtureName(variant)

				t.Run(name, func(t *testing.T) {
					var buf bytes.Buffer
					// Execute the parsed template directly rather than through
					// renderPage or renderFragment. Those wrappers log the
					// error and answer HTTP 500, so a capture taken through
					// them would record an error page as if it were markup.
					if err := tpl.ExecuteTemplate(&buf, unit.Name, variant.Data); err != nil {
						t.Fatalf("render %q from %s: %v", unit.Name, file, err)
					}

					if strings.TrimSpace(buf.String()) != "" {
						content = true
					}

					golden.Compare(t, lgGolden.FixturePath(root, file, name), buf.Bytes())
				})
			}

			if !content && !golden.Updating() {
				t.Errorf("template %q from %s rendered only whitespace in every variant; its fixture data does not reach the markup",
					unit.Name, file)
			}
		}
	}
}
