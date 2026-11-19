// Related: handler_golden_test.go -- the capture of every route, on one fixed engine and one single-node graph per mode
// Related: graph_trace_mode_test.go -- traceModeEngine and traceModeRequest, the engine each capture here asks
// Related: layout.go, layout_nexthop.go, layout_trace.go -- the three drawings captured

package lg

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/golden"
)

// lgGraphFixtures is where the captured drawings live.
const lgGraphFixtures = "graph"

// lgGraphCase is one drawing: a network, the graph mode asked of it, and the
// SVG bytes the answer must keep producing.
type lgGraphCase struct {
	// Name is the fixture stem.
	Name string
	// Query is appended to the request: the mode, or nothing for the default.
	Query string
	// Engine answers the one RIB query and the peer summary.
	Engine traceModeEngine
}

// lgGraphTwoExits is a prefix two external neighbours announce: .1 and .2 are
// the exits, each with its own path to AS64500, .3 reflects .1's route and .4
// is its client. Both neighbours hold a session of their own with Ze and
// announce their own address, as a collector's peers do.
func lgGraphTwoExits() traceModeEngine {
	return traceModeEngine{
		summary: traceTestSummaryText(
			traceTestRouter("198.51.100.1"), traceTestRouter("198.51.100.2"),
			traceTestRouter("198.51.100.3"), traceTestRouter("198.51.100.4"),
			traceTestPeer{address: "203.0.113.1", remoteAS: 65101},
			traceTestPeer{address: "203.0.113.2", remoteAS: 65102},
		),
		rows: []string{
			traceTestRow("198.51.100.1", "198.51.100.1", 65101, 64500),
			traceTestRow("198.51.100.2", "198.51.100.2", 65102, 64500),
			traceTestRow("198.51.100.3", "198.51.100.1", 65101, 64500),
			traceTestRow("198.51.100.4", "198.51.100.3", 65101, 64500),
			traceTestRow("203.0.113.1", "203.0.113.1", 65101, 64500),
			traceTestRow("203.0.113.2", "203.0.113.2", 65102, 64500),
		},
	}
}

// lgGraphOneExit is a prefix one external neighbour announces: .1 is the only
// exit, .2 and .3 point at it, and .4 is the client of .3.
func lgGraphOneExit() traceModeEngine {
	return traceModeEngine{
		summary: traceTestSummaryText(
			traceTestRouter("198.51.100.1"), traceTestRouter("198.51.100.2"),
			traceTestRouter("198.51.100.3"), traceTestRouter("198.51.100.4"),
			traceTestPeer{address: "203.0.113.1", remoteAS: 65101},
		),
		rows: []string{
			traceTestRow("198.51.100.1", "198.51.100.1", 65101, 64500),
			traceTestRow("198.51.100.2", "198.51.100.1", 65101, 64500),
			traceTestRow("198.51.100.3", "198.51.100.1", 65101, 64500),
			traceTestRow("198.51.100.4", "198.51.100.3", 65101, 64500),
			traceTestRow("203.0.113.1", "203.0.113.1", 65101, 64500),
		},
	}
}

// lgGraphGoldenCases draws the two networks in each of the three modes.
var lgGraphGoldenCases = []lgGraphCase{
	{Name: "aspath-two-exits", Query: "", Engine: lgGraphTwoExits()},
	{Name: "aspath-one-exit", Query: "", Engine: lgGraphOneExit()},
	{Name: "nexthop-two-exits", Query: "&mode=nexthop", Engine: lgGraphTwoExits()},
	{Name: "nexthop-one-exit", Query: "&mode=nexthop", Engine: lgGraphOneExit()},
	{Name: "trace-two-exits", Query: "&mode=trace", Engine: lgGraphTwoExits()},
	{Name: "trace-one-exit", Query: "&mode=trace", Engine: lgGraphOneExit()},
}

// TestLGGraphGoldenOutput captures the whole SVG of a graph with two exits and
// of a graph with one, in each of the three modes, as a real request to
// GET /lg/graph answers it.
//
// The route capture (TestLGHandlerGoldenOutput) holds one graph per older
// mode, and that graph has one node: no edge, no second layer, no AS chain.
// These six are drawings with several layers, so a change to a coordinate, an
// arrow or a label of any of the three layouts shows as a changed byte.
//
// VALIDATES: the exact bytes of the AS path, next-hop and trace drawings of
// a two-exit and a one-exit network.
// PREVENTS: a layout change that moves or drops an element and leaves every
// substring check green.
func TestLGGraphGoldenOutput(t *testing.T) {
	names := make([]string, 0, len(lgGraphGoldenCases))
	for _, c := range lgGraphGoldenCases {
		names = append(names, c.Name)
	}
	golden.AssertUniqueNames(t, "fixture", "lgGraphGoldenCases", names)

	root := filepath.Join("testdata", lgGraphFixtures)
	written := make([]string, 0, len(lgGraphGoldenCases))
	for _, c := range lgGraphGoldenCases {
		fixture := filepath.Join(root, c.Name+".svg")
		written = append(written, fixture)

		t.Run(c.Name, func(t *testing.T) {
			rec, _ := traceModeRequest(t, c.Engine, c.Query)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
			}
			if contentType := rec.Header().Get("Content-Type"); contentType != "image/svg+xml" {
				t.Fatalf("Content-Type = %q, want image/svg+xml", contentType)
			}
			got := rec.Body.Bytes()
			// An empty-state drawing is a valid SVG too: a capture of it would
			// pin a message, not a graph.
			if !strings.Contains(string(got), `class="edge"`) {
				t.Fatalf("the drawing holds no edge, so it is not the graph of this network:\n%s", got)
			}

			again, _ := traceModeRequest(t, c.Engine, c.Query)
			if again.Body.String() != string(got) {
				t.Fatalf("%s answers two different drawings on two identical requests; the capture cannot pin it", c.Name)
			}

			golden.Compare(t, fixture, got)
		})
	}

	// A capture run writes every fixture, so the check that judges the
	// directory against the list does not apply to it.
	if golden.Updating() {
		return
	}
	golden.AssertCoversDir(t, root, "lgGraphGoldenCases", written)
}
