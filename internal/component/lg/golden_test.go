// Related: render.go -- the helpers every component below calls
// Related: view.go -- the structs the fixture data builds
// Related: handler_ui.go -- the handlers that supply these data shapes

package lg

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/test/golden"
)

// lgGolden is the golden harness over the lg templ tree. The spec below, the
// fixture data and the Render call stay in this package. The walk over the
// sources, the coverage check, the fixture path rule and the byte comparison
// are shared with internal/component/web through internal/test/golden.
//
// The walk reads the package directory rather than an embedded FS: templ
// compiles each .templ into Go, so nothing is embedded any more. Ext keeps the
// walk to the sources and off the Go files beside them.
//
// Recapture a deliberate markup change with `go test ./internal/component/lg -run TestLGGoldenOutput -update-golden`.
var lgGolden = golden.Set{
	FS:      os.DirFS("."),
	Dir:     ".",
	Ext:     ".templ",
	Spec:    lgGoldenSpec,
	SpecVar: "lgGoldenSpec",
}

// lgGoldenSpec maps each templ source to the components it declares and the
// data each one renders with. TestLGGoldenOutput compares this map against the
// directory and fails when the two disagree. A new .templ file, or a new templ
// component inside an existing one, fails until it is captured here.
//
// Fixture names are the html/template names these components replaced. A
// component is a Go function and carries a Go name, and renaming 29 fixtures
// would hide the byte delta of the port inside a rename.
var lgGoldenSpec = golden.Spec{
	// The empty-state drawing, ported in phase 5 of
	// spec-web-templ-migration. The two graph builders beside it stay in
	// Go: markup_check_test.go carries the reason.
	"graph_empty.templ": {{
		Name:    "graphEmpty",
		Fixture: "graph_empty",
		Variants: []golden.Variant{
			{Name: "no-routes", Data: graphEmpty("No routes found")},
			{Name: "too-many", Data: graphEmpty("Too many ASes (240) for graph")},
		},
	}},
	"error.templ": {{
		Name:    "errorBanner",
		Fixture: "error_banner",
		Variants: []golden.Variant{
			{Name: "set", Data: errorBanner("engine unreachable")},
			{Name: "clear", Data: errorBanner("")},
		},
	}},
	"help.templ": {{
		Name:     "helpPage",
		Fixture:  "help",
		Variants: []golden.Variant{{Data: helpPage()}},
	}},
	"layout.templ": {{
		Name:    "pageLayout",
		Fixture: "layout",
		Variants: []golden.Variant{
			{Name: "peers", Data: lgLayoutFixture("Peers", "peers", pgPeersPage)},
			{Name: "search", Data: lgLayoutFixture("Route Search", "search", pgSearchPage)},
		},
	}},
	"peers.templ": {
		{Name: "peersPage", Fixture: "peers", Variants: lgPeersVariants(peersPage)},
		{Name: "peersContent", Fixture: "peers_content", Variants: lgPeersVariants(peersContent)},
		{Name: "peersTableBody", Fixture: "peers_table_body", Variants: []golden.Variant{
			{Name: "full", Data: peersTableBody(lgPeerRows())},
			{Name: "empty", Data: peersTableBody(nil)},
		}},
		{Name: "peersStreamError", Fixture: "peers_stream_error", Variants: []golden.Variant{
			{Data: peersStreamError("BGP engine unavailable")},
		}},
		{Name: "bmpPeersContent", Fixture: "bmp_peers_content", Variants: []golden.Variant{
			{Data: bmpPeersContent(lgBMPPeerRows())},
		}},
		{Name: "peerDetailArea", Fixture: "peer_detail_area", Variants: []golden.Variant{
			{Data: peerDetailArea()},
		}},
	},
	"peer_routes.templ": {
		{Name: "peerRoutesPage", Fixture: "peer_routes", Variants: lgPeerRoutesVariants(peerRoutesPage)},
		{Name: "peerRoutesContent", Fixture: "peer_routes_content", Variants: lgPeerRoutesVariants(peerRoutesContent)},
	},
	"route_detail.templ": {{
		Name:    "routeDetail",
		Fixture: "route_detail",
		Variants: []golden.Variant{
			{Name: "found", Data: routeDetail(routeDetailView{
				Prefix: "203.0.113.0/24",
				Route:  lgRouteRowPtr(0),
			})},
			{Name: "missing", Data: routeDetail(routeDetailView{
				Prefix: "203.0.113.0/24",
			})},
		},
	}},
	"route_table.templ": {{
		Name:     "routeResults",
		Fixture:  "route_results",
		Variants: lgRouteResultsVariants(),
	}},
	"search.templ": {
		{Name: "searchPage", Fixture: "search", Variants: lgSearchVariants(searchPage)},
		{Name: "searchForm", Fixture: "search_form", Variants: lgSearchVariants(searchForm)},
	},
}

// lgLayoutFixture reproduces what renderPage composes: one page's chrome
// around already-rendered content.
//
// page is what the head loads its assets from, so the two variants capture two
// different heads: the peers page opens an SSE stream and the search page does
// not. The content stands in for the body, which is why the peers capture
// carries the extension with no attribute using it.
func lgLayoutFixture(title, tab string, page pageID) templ.Component {
	return pageLayout(layoutView{Title: title, ActiveTab: tab, Page: page}, lgRawComponent("<p>page content</p>"))
}

// lgRawComponent writes fixed markup, standing in for the page component the
// layout wraps.
func lgRawComponent(markup string) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, markup)

		return err
	})
}

// lgPeerRows reproduces the rows extractPeers builds. The two differ in
// RemoteASName and State so both branches of each conditional render.
func lgPeerRows() []peerRow {
	return []peerRow{
		{
			Address: "192.0.2.1", RemoteAS: "64500", RemoteASName: "Example Transit",
			State: "established", Uptime: "3h12m4s",
			RoutesReceived: "1200", RoutesAccepted: "1180", RoutesSent: "42",
			UpdatesReceived: "9000", UpdatesSent: "12",
			Description: "transit",
		},
		{
			Address: "2001:db8::1", RemoteAS: "64501", RemoteASName: "",
			State: "idle", Uptime: "0s",
			RoutesReceived: "0", RoutesAccepted: "0", RoutesSent: "0",
			UpdatesReceived: "0", UpdatesSent: "0",
			Description: "",
		},
	}
}

// lgBMPPeerRows reproduces the rows extractBMPPeers builds. The two differ in
// State, PeerASName and IPv6 so both branches of each render.
func lgBMPPeerRows() []bmpPeerRow {
	return []bmpPeerRow{
		{
			Router: "198.51.100.7", PeerAS: "64502", PeerASName: "Example Two",
			BGPID: "198.51.100.1", State: "up", IPv6: false,
		},
		{
			Router: "198.51.100.8", PeerAS: "64503", PeerASName: "",
			BGPID: "198.51.100.2", State: "down", IPv6: true,
		},
	}
}

// lgRouteRows reproduces the rows routeRows builds from the decoded RIB. The
// first is the best path and the second is not, so both branches of Best
// render.
func lgRouteRows() []routeRow {
	return []routeRow{
		{
			Prefix: "203.0.113.0/24", NextHop: "192.0.2.1",
			ASPath:              []string{"64500", "64510"},
			Origin:              "igp",
			LocalPreference:     "100",
			MED:                 "0",
			PeerAddress:         "192.0.2.1",
			Best:                true,
			Communities:         []string{"64500:100", "64500:200"},
			LargeCommunities:    []string{"64500:1:2"},
			ExtendedCommunities: []string{"rt:64500:7"},
		},
		{
			Prefix: "198.51.100.0/24", NextHop: "2001:db8::1",
			ASPath:          []string{"64501"},
			Origin:          "incomplete",
			LocalPreference: "90",
			MED:             "5",
			PeerAddress:     "2001:db8::1",
			Best:            false,
		},
	}
}

// lgRouteRowPtr addresses one fixture route, for the view models that hold a
// pointer.
func lgRouteRowPtr(i int) *routeRow {
	rows := lgRouteRows()

	return &rows[i]
}

func lgPeersVariants(render func(peersView) templ.Component) []golden.Variant {
	return []golden.Variant{
		{Name: "full", Data: render(peersView{
			layoutView: layoutView{Title: "Peers", ActiveTab: "peers"},
			Peers:      lgPeerRows(),
			BMPPeers:   lgBMPPeerRows(),
		})},
		{Name: "empty", Data: render(peersView{
			layoutView: layoutView{Title: "Peers", ActiveTab: "peers"},
			Error:      "engine unreachable",
		})},
	}
}

func lgPeerRoutesVariants(render func(peerRoutesView) templ.Component) []golden.Variant {
	base := layoutView{Title: "Routes from 192.0.2.1", ActiveTab: "peers"}

	return []golden.Variant{
		{Name: "routes", Data: render(peerRoutesView{
			layoutView: base,
			Address:    "192.0.2.1",
			Peer: &peerInfoRow{
				State: "established", RemoteAS: "64500",
				RemoteASName: "Example Transit", Description: "transit",
			},
			Routes: lgRouteRows(),
		})},
		{Name: "summary", Data: render(peerRoutesView{
			layoutView: base,
			Address:    "192.0.2.1",
			Peer: &peerInfoRow{
				State: "idle", RemoteAS: "64501",
			},
			Histogram: []histogramRow{
				{Family: "ipv4 unicast", Length: "24", Count: "1200"},
				{Family: "ipv6 unicast", Length: "48", Count: "30"},
			},
		})},
		{Name: "empty", Data: render(peerRoutesView{
			layoutView: base,
			Address:    "192.0.2.1",
		})},
	}
}

func lgRouteResultsVariants() []golden.Variant {
	return []golden.Variant{
		{Name: "routes", Data: routeResults(searchView{
			layoutView: searchLayout,
			Prefix:     "203.0.113.0/24", ASPath: "64500 64510",
			Community: "64500:100", Family: "ipv4/unicast",
			Routes: lgRouteRows(), Count: 2,
		})},
		{Name: "no-prefix", Data: routeResults(searchView{
			layoutView: searchLayout,
			ASPath:     "64500",
			Routes:     lgRouteRows(), Count: 2,
		})},
		{Name: "empty", Data: routeResults(searchView{layoutView: searchLayout})},
		{Name: "error", Data: routeResults(searchView{
			layoutView: searchLayout,
			Prefix:     "bad",
			Error:      "invalid prefix",
		})},
	}
}

func lgSearchVariants(render func(searchView) templ.Component) []golden.Variant {
	return []golden.Variant{
		{Name: "blank", Data: render(searchView{layoutView: searchLayout})},
		{Name: "filled", Data: render(searchView{
			layoutView: searchLayout,
			Prefix:     "203.0.113.0/24", ASPath: "64500 64510",
			Community: "64500:100", Family: "ipv6/unicast",
			Routes: lgRouteRows(), Count: 2,
		})},
	}
}

// lgGoldenRoot is where the captured template fixtures live.
func lgGoldenRoot() string { return filepath.Join("testdata", "golden") }

// lgRenderUnit renders one spec variant.
func lgRenderUnit(t *testing.T, file, name string, data any) []byte {
	t.Helper()

	component, ok := data.(templ.Component)
	if !ok {
		t.Fatalf("variant of %q in %s carries %T, not a templ.Component", name, file, data)
	}

	var buf bytes.Buffer
	if err := component.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render %q from %s: %v", name, file, err)
	}

	return buf.Bytes()
}

// TestLGGoldenOutput captures the rendered bytes of every lg component and
// compares them against the committed fixtures.
//
// VALIDATES: the lg component set renders byte for byte what it rendered when
// the fixtures were captured.
// PREVENTS: a rendering change that keeps every substring assertion green and
// still moves the bytes an operator receives.
func TestLGGoldenOutput(t *testing.T) {
	files := lgGolden.Files(t)
	lgGolden.AssertCoversFS(t, files)

	root := lgGoldenRoot()
	if !golden.Updating() {
		if _, statErr := os.Stat(root); statErr != nil {
			t.Fatalf("fixture directory %s is missing; capture it with -update-golden: %v", root, statErr)
		}
	}

	written := make([]string, 0, len(files))

	for _, file := range files {
		for _, unit := range lgGolden.Spec[file] {
			content := false

			for _, variant := range unit.Variants {
				name := unit.FixtureName(variant)
				written = append(written, lgGolden.FixturePath(root, file, name))

				t.Run(name, func(t *testing.T) {
					got := lgRenderUnit(t, file, unit.Name, variant.Data)

					if strings.TrimSpace(string(got)) != "" {
						content = true
					}

					golden.Compare(t, lgGolden.FixturePath(root, file, name), got)
				})
			}

			if !content && !golden.Updating() {
				t.Errorf("component %q from %s rendered only whitespace in every variant; its fixture data does not reach the markup",
					unit.Name, file)
			}
		}
	}

	// A component deleted from the tree takes its spec entry with it. Its
	// fixture stays on disk, where the next reader counts bytes nobody
	// compares.
	golden.AssertCoversDir(t, root, "lgGoldenSpec", written)
}
