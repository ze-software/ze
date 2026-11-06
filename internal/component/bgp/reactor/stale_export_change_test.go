// Design: docs/architecture/core-design.md -- final-only export policy reconciliation.
package reactor

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// exportPolicySwap is the copier a reload's export policy change delivers.
func exportPolicySwap(dst, src *PeerSettings) { dst.ExportPolicy = src.ExportPolicy }

// staleChangeFixture is one established destination whose export chain holds the
// opted-in "stale" member, under a judge that accepts "stale" and "passes",
// answers what answers names, and rejects every other member, recording the
// order it was asked in.
type staleChangeFixture struct {
	a       *reactorAPIAdapter
	peer    *Peer
	conn    *recordingConn
	batch   bgptypes.NLRIBatch
	answers map[string]rpc.FilterUpdateOutput
	// respond, when set, answers a call it names, ahead of answers.
	respond func(rpc.FilterUpdateInput) (rpc.FilterUpdateOutput, bool)
	callMu  sync.Mutex
	calls   []string
}

func newStaleChangeFixture(t *testing.T, typed bool, fields ...string) *staleChangeFixture {
	t.Helper()
	f := &staleChangeFixture{}
	judge := func(input rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
		f.callMu.Lock()
		f.calls = append(f.calls, input.Filter)
		f.callMu.Unlock()
		if f.respond != nil {
			if answer, named := f.respond(input); named {
				return &answer, nil
			}
		}
		if answer, named := f.answers[input.Filter]; named {
			return &answer, nil
		}
		if input.Filter == "stale" || input.Filter == "passes" {
			return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, nil
		}
		return &rpc.FilterUpdateOutput{Action: rpc.FilterReject}, nil
	}
	reads, sets := []string{"prefix"}, []string{"as-path"}
	if len(fields) != 0 {
		reads, sets = append([]string{rpc.FilterFieldRaw}, fields...), fields
	}
	transport := &writerBoundaryTransport{answerTransport: &answerTransport{
		reads: reads, sets: sets}, conn: errorVerdictConnection(t, typed, judge)}
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
	f.peer, f.conn = writerBoundaryPeer(t, r)
	// peer_run.go sets this on every session a Peer runs.
	f.peer.session.aigpPeer = f.peer
	f.peer.settings.ExportPolicy = answerPolicyAt(t, "bgp export", routepolicy.FinalAccept, "stale")
	f.batch = staleReadvertiseBatch(t)
	f.a = &reactorAPIAdapter{r: r}
	return f
}

// seedOwner writes the route past the export gate, as the original send did, and
// answers the exact receipt the RIB would hold for it.
func (f *staleChangeFixture) seedOwner(t *testing.T) uint64 {
	t.Helper()
	facts := announceFactsFor(f.peer, f.batch.Family, netip.MustParseAddr("10.0.0.1"), false, f.peer.negotiated.Load())
	seed, err := f.a.buildBatchAnnounceUpdate(make([]byte, 4096), make([]byte, 4096), f.batch, facts)
	if err != nil {
		t.Fatal(err)
	}
	gate := f.peer.session.egressRouteFilter
	// The gate that passed the route read the chain current at that send.
	f.peer.session.egressRouteFilter = func([]byte) (bool, []byte, []byte) {
		f.peer.session.gateGeneration = f.peer.exportGeneration.Load()
		return false, nil, nil
	}
	if err := f.peer.session.SendUpdate(seed); err != nil {
		t.Fatal(err)
	}
	f.peer.session.egressRouteFilter = gate
	owner := f.peer.session.sentReceipt.MessageID()
	if owner == 0 {
		t.Fatal("seed write produced no ownership receipt")
	}
	return owner
}

// TestExportPolicyWatermarkBoundary pins the comparison: a zero watermark means
// no reactor site built this Peer and no policy changed, and everything strictly
// below a set one is older, the zero owner of unknown provenance included.
func TestExportPolicyWatermarkBoundary(t *testing.T) {
	cases := []struct {
		name    string
		from    uint64
		message uint64
		want    bool
	}{
		{name: "never-changed", from: 0, message: 5, want: false},
		{name: "never-changed-unknown-owner", from: 0, message: 0, want: false},
		{name: "below", from: 10, message: 9, want: true},
		{name: "at", from: 10, message: 10, want: false},
		{name: "above", from: 10, message: 11, want: false},
		{name: "unknown-owner", from: 10, message: 0, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peer := &Peer{}
			peer.raiseExportPolicyFrom(tc.from)
			if got := peer.exportPolicyNewerThan(tc.message); got != tc.want {
				t.Fatalf("exportPolicyNewerThan(%d) with watermark %d = %v, want %v", tc.message, tc.from, got, tc.want)
			}
		})
	}
}

// TestStaleWholeChainWithoutLibraryFailsClosed proves a batch that must be
// re-judged, toward a destination whose reload removed the chain after the rail
// was chosen, is withheld as no decision rather than kept unjudged.
func TestStaleWholeChainWithoutLibraryFailsClosed(t *testing.T) {
	a := &reactorAPIAdapter{r: &Reactor{attrModHandlers: attrModHandlersWithDefaults()}}
	chain := staleChain{policy: filterapi.Policy{}, asn4: true, judgeAll: true}
	outcome, modified := a.decideStaleReadvertise(filterapi.PeerFilterInfo{PeerAS: 65001}, chain, minimalAnnounceBody, 1)
	if outcome != staleFilterFailed || modified != nil {
		t.Fatalf("outcome=%d modified=%x, want staleFilterFailed and no body", outcome, modified)
	}
}
