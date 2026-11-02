package reactor

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// staleExportChain answers the stale readvertise chain of a destination whose
// export policy holds the one plugin member probe:f, under final reject.
func staleExportChain(t *testing.T) staleChain {
	t.Helper()
	reg, err := routepolicy.NewRegistry()
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	members := []policychain.Member{{Name: "f", Plugin: "probe"}}
	chain, err := policychain.Compile(libraryTestExport, reg, members, routepolicy.FinalReject, policychain.Bases{LocalPreference: 100})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	policy := mustPolicy(t, []routepolicy.Ref{{Name: "probe:f"}}, routepolicy.FinalReject).WithLibrary(chain)
	return staleChain{policy: policy, asn4: true}
}

// TestDecideStaleReadvertiseRunsOptedPluginFilter proves AC-13's rail half
// (spec-filter-wire-6): the stale readvertise decision runs the plugin filter
// of the destination's export chain that declares readvertise, maps its
// verdict onto the rail's outcome (a set becomes staleModify with the body the
// set builds, a withdraw staleWithdraw), and runs no filter that does not opt
// in (staleKeep, the route sent unchanged, the filter never called). The
// chain's final is reject, so a keep proves the rail ignores it. Method: the
// externalTransport stand-in answers probe:f; no registered readvertise
// egress filter exists, so only the chain decides.
func TestDecideStaleReadvertiseRunsOptedPluginFilter(t *testing.T) {
	dest := filterapi.PeerFilterInfo{Address: mustParseAddr("10.0.0.2"), PeerAS: 65000, LocalAS: 65000, IsIBGP: true}
	setLP0 := rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{
		{Field: "local-preference", Action: rpc.FilterSetActionSet, Value: json.RawMessage("0")}}}

	for _, tc := range []struct {
		name        string
		readvertise bool
		reply       rpc.FilterUpdateOutput
		want        staleOutcome
		wantCalls   int
	}{
		{"opted filter sets", true, setLP0, staleModify, 1},
		{"opted filter withdraws", true, rpc.FilterUpdateOutput{Action: rpc.FilterWithdraw}, staleWithdraw, 1},
		{"opted filter rejects", true, rpc.FilterUpdateOutput{Action: rpc.FilterReject}, staleSuppress, 1},
		{"opted filter accepts", true, rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, staleKeep, 1},
		{"filter not opted in", false, setLP0, staleKeep, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := &externalTransport{reads: []string{"local-preference"}, sets: []string{"local-preference"},
				reply: tc.reply, readvertise: tc.readvertise}
			r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
			a := &reactorAPIAdapter{r: r}
			received := libraryTestBody(100)

			outcome, modified := a.decideStaleReadvertise(dest, staleExportChain(t), received, 1)
			if outcome != tc.want {
				t.Fatalf("outcome = %v, want %v", outcome, tc.want)
			}
			if len(transport.sent) != tc.wantCalls {
				t.Fatalf("probe:f was called %d times, want %d", len(transport.sent), tc.wantCalls)
			}
			if tc.wantCalls > 0 && transport.sent[0].Direction != "export" {
				t.Fatalf("probe:f was sent direction %q, want export", transport.sent[0].Direction)
			}
			if tc.want != staleModify {
				if modified != nil {
					t.Fatalf("outcome %v carried a modified body %x", outcome, modified)
				}
				return
			}
			want := textBody(t, r, received, func(acc *filterapi.ModAccumulator) {
				acc.OpCopy(uint8(attribute.AttrLocalPref), filterapi.AttrModSet, []byte{0, 0, 0, 0})
			})
			if !bytes.Equal(modified, want) {
				t.Fatalf("modified body = %x, want %x", modified, want)
			}
		})
	}
}
