package fakefilter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// conformanceRoutes are the routes the harness judges: two prefixes of one
// family, each a filter-update the forked transport would carry.
func conformanceRoutes() []sdk.ConformanceRoute {
	return []sdk.ConformanceRoute{
		{Input: rpc.FilterUpdateInput{Filter: FilterName, Direction: "import", Peer: "127.0.0.1", PeerAS: 65001,
			Fields: json.RawMessage(`{"prefix":{"ipv4/unicast":["10.0.0.0/24"]}}`)}},
		{Input: rpc.FilterUpdateInput{Filter: FilterName, Direction: "import", Peer: "127.0.0.2", PeerAS: 65002,
			Fields: json.RawMessage(`{"prefix":{"ipv4/unicast":["10.1.0.0/24"]}}`)}},
	}
}

// TestFakeFilterConformance proves AC-35's fast-path half: the evaluator the
// plugin hands sdk.OnFilter, written against the SDK alone, passes the
// fast-path conformance harness (deadline, repeat, borrowed view, transport
// equality, concurrency). Method: the harness itself, under -race, with the
// stop trigger off, as a plugin runs when no test asks it to stop, and one of
// the two routes rejected, so both verdicts cross both transports.
func TestFakeFilterConformance(t *testing.T) {
	judge := newJudge(trigger{rejectPrefix: "10.1.0.0/24"}, func() {}, func(judgement) {})
	if !sdk.CheckFilterConformance(t, Name, judge.evaluate, conformanceRoutes()) {
		t.Fatal("the fakefilter evaluator breaks the fast-path contract")
	}
}

// TestFakeFilterAcceptsAndNamesThePrefix proves the verdict and the record a
// .ci reads: a route without the reject prefix is accepted, and the judged
// prefix is reported with its peer and the transport that carried it (an
// encoded route: the batch).
func TestFakeFilterAcceptsAndNamesThePrefix(t *testing.T) {
	var judged []judgement
	judge := newJudge(trigger{rejectPrefix: "10.1.0.0/24"}, func() {}, func(j judgement) { judged = append(judged, j) })
	routes := conformanceRoutes()
	route := rpc.InputFilterRoute(&routes[0].Input)
	out, err := judge.evaluate(&route)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if out == nil || out.Action != rpc.FilterAccept {
		t.Fatalf("verdict = %+v, want accept", out)
	}
	want := judgement{peer: "127.0.0.1", prefix: "10.0.0.0/24", transport: transportBatch, verdict: "accept"}
	if len(judged) != 1 || judged[0] != want {
		t.Fatalf("judged = %+v, want [%+v]", judged, want)
	}
}

// slotView is an engine view as the slot hands one: the declared fields, and
// a non-nil engine value.
type slotView struct{ fields []byte }

func (v slotView) Fields() ([]byte, error) { return v.fields, nil }
func (slotView) Raw() []byte               { return nil }
func (slotView) Engine() any               { return struct{}{} }

// TestFakeFilterRejectsTheNamedPrefix proves the reject verdict a .ci reads
// on the wire, on both transports: the route carrying the reject prefix is
// rejected and reported so, through the slot (a route with the engine's view)
// and through the batch (an encoded route).
func TestFakeFilterRejectsTheNamedPrefix(t *testing.T) {
	var judged []judgement
	judge := newJudge(trigger{rejectPrefix: "10.1.0.0/24"}, func() {}, func(j judgement) { judged = append(judged, j) })
	routes := conformanceRoutes()
	batch := rpc.InputFilterRoute(&routes[1].Input)
	slot := rpc.ViewFilterRoute(&rpc.FilterUpdateInput{Filter: FilterName, Direction: "import", Peer: "127.0.0.2", PeerAS: 65002},
		slotView{fields: routes[1].Input.Fields})
	for _, route := range []*rpc.FilterRoute{&batch, &slot} {
		out, err := judge.evaluate(route)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if out == nil || out.Action != rpc.FilterReject {
			t.Fatalf("verdict = %+v, want reject", out)
		}
	}
	want := []judgement{
		{peer: "127.0.0.2", prefix: "10.1.0.0/24", transport: transportBatch, verdict: "reject"},
		{peer: "127.0.0.2", prefix: "10.1.0.0/24", transport: transportSlot, verdict: "reject"},
	}
	if !slices.Equal(judged, want) {
		t.Fatalf("judged = %+v, want %+v", judged, want)
	}
}

// TestFakeFilterRefusesUnreadableFields proves the evaluator does not answer a
// verdict for a route whose fields it cannot read: the member's on-error
// decides that route, not an accept invented by the fixture.
func TestFakeFilterRefusesUnreadableFields(t *testing.T) {
	judge := newJudge(trigger{}, func() {}, func(judgement) {})
	input := rpc.FilterUpdateInput{Filter: FilterName, Direction: "import", Fields: json.RawMessage(`{"prefix":`)}
	route := rpc.InputFilterRoute(&input)
	if out, err := judge.evaluate(&route); err == nil {
		t.Fatalf("evaluate answered %+v for unreadable fields, want an error", out)
	}
}

// TestFakeFilterStopTriggerFiresOnce proves the AC-33 stop trigger: with
// stop-after N, the Nth judged route is still answered (by the live plugin),
// the stop runs exactly once on it, and never before it.
func TestFakeFilterStopTriggerFiresOnce(t *testing.T) {
	var stops atomic.Int32
	judge := newJudge(trigger{stopAfter: 2}, func() { stops.Add(1) }, func(judgement) {})
	routes := conformanceRoutes()
	for i := range 4 {
		route := rpc.InputFilterRoute(&routes[i%2].Input)
		out, err := judge.evaluate(&route)
		if err != nil || out == nil || out.Action != rpc.FilterAccept {
			t.Fatalf("route %d: verdict %+v, error %v, want accept", i, out, err)
		}
		want := int32(0)
		if i >= 1 {
			want = 1
		}
		if got := stops.Load(); got != want {
			t.Fatalf("after route %d: stop ran %d times, want %d", i, got, want)
		}
	}
}

// TestFakeFilterDeclaration proves the declaration the daemon receives at
// Stage 1 names the filter a chain references (fakefilter:judge), reads the
// prefix, and fails closed: the on-error AC-33 asserts.
func TestFakeFilterDeclaration(t *testing.T) {
	decls := filterDecls()
	if len(decls) != 1 {
		t.Fatalf("declared %d filters, want 1", len(decls))
	}
	decl := decls[0]
	if decl.Name != FilterName || decl.Direction != sdk.FilterImport || decl.OnError != sdk.OnErrorReject {
		t.Fatalf("declaration = %+v, want %s import on-error reject", decl, FilterName)
	}
	if len(decl.Reads) != 1 || decl.Reads[0] != fieldPrefix {
		t.Fatalf("reads = %q, want [%q]", decl.Reads, fieldPrefix)
	}
}

// TestFakeFilterRespawnedGenerationNeverStops proves the AC-33 respawn trigger:
// a run that finds the stopped file is generation 2, which drops the stop
// trigger (so ze's restart does not loop) and reports its generation; a run
// that does not is generation 1 and keeps it. Method: startGeneration before
// and after the file exists, then one judged route of each.
func TestFakeFilterRespawnedGenerationNeverStops(t *testing.T) {
	stopped := filepath.Join(t.TempDir(), "fakefilter.stopped")
	asked := trigger{stopAfter: 1, stoppedFile: stopped, restart: true}

	first := startGeneration(asked)
	if first.generation != 1 || first.stopAfter != 1 {
		t.Fatalf("before the stop: generation %d stop-after %d, want 1 and 1", first.generation, first.stopAfter)
	}
	if err := os.WriteFile(stopped, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	second := startGeneration(asked)
	if second.generation != 2 || second.stopAfter != 0 {
		t.Fatalf("after the stop: generation %d stop-after %d, want 2 and 0", second.generation, second.stopAfter)
	}

	var stops atomic.Int32
	var judged []judgement
	j := newJudge(second, func() { stops.Add(1) }, func(r judgement) { judged = append(judged, r) })
	route := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: FilterName, Direction: "import", Peer: "127.0.0.2",
		Fields: json.RawMessage(`{"prefix":{"ipv4/unicast":["10.1.0.0/24"]}}`)})
	if _, err := j.evaluate(&route); err != nil {
		t.Fatal(err)
	}
	if stops.Load() != 0 {
		t.Fatalf("generation 2 stopped %d times, want never", stops.Load())
	}
	if len(judged) != 1 || judged[0].generation != 2 {
		t.Fatalf("judged = %+v, want one route reported by generation 2", judged)
	}
}

// TestFakeFilterDeclaresRestartWhenAsked proves the Stage 1 declaration
// carries the restart failure policy only when the test asks for it.
func TestFakeFilterDeclaresRestartWhenAsked(t *testing.T) {
	if got := registration(trigger{}).FailurePolicy; got != rpc.FailureUnspecified {
		t.Fatalf("unasked failure policy = %v, want unspecified", got)
	}
	if got := registration(trigger{restart: true}).FailurePolicy; got != sdk.FailureRestart {
		t.Fatalf("asked failure policy = %v, want restart", got)
	}
}
