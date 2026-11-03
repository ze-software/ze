package reactor

import (
	"encoding/json"
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/route"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"

	"github.com/stretchr/testify/assert"
)

// This file pins ONE property on the stale readvertise rail: a chain member
// that cannot answer is not a policy decision, and the rail's caller must be
// able to tell the two apart. Every test here is a PAIR -- a member that cannot
// answer beside one that rejects -- because a test that only drives the
// failure passes against code that treats both the same way.
//
// forwardUpdateCore (reactor_api_forward.go) reads the same distinction from
// the export chain step (egressStepResult.failed) and is covered by
// rfc7947_reactor_api_relay_test.go. The RS fast path runs no engine egress
// step: a plugin filter there is an export chain member, and a chain that
// cannot run hands the destination back.

// staleRailCase is one member answer on the stale readvertise rail: the
// opted member probe:f either cannot be asked (err), or answers reply; noHandlers
// leaves the reactor without attribute handlers, so a set it answers cannot be
// built.
type staleRailCase struct {
	name       string
	reply      rpc.FilterUpdateOutput
	err        error
	noHandlers bool
}

// staleRailFailureCases are the two failure modes of the stale readvertise rail
// beside its policy control: a member that cannot answer (its plugin's call
// failed, as when the process has gone), a member whose LOCAL_PREF=0 set cannot
// be built, and a member that rejects.
var staleRailFailureCases = []staleRailCase{
	{name: "filter that cannot answer", err: errors.New("plugin connection closed")},
	{name: "unbuildable modification", noHandlers: true, reply: rpc.FilterUpdateOutput{Action: rpc.FilterModify,
		Sets: []rpc.FilterSet{{Field: "local-preference", Action: rpc.FilterSetActionSet, Value: json.RawMessage("0")}}}},
	{name: "rejecting filter", reply: rpc.FilterUpdateOutput{Action: rpc.FilterReject}},
}

// transport answers the stand-in for probe:f's plugin.
func (tc staleRailCase) transport() *externalTransport {
	transport := staleRailTransport(tc.reply)
	transport.err = tc.err
	return transport
}

// handlers answers the reactor's attribute handlers: none for noHandlers.
func (tc staleRailCase) handlers() map[uint8]filterapi.AttrModHandler {
	if tc.noHandlers {
		return map[uint8]filterapi.AttrModHandler{}
	}
	return attrModHandlersWithDefaults()
}

// TestDecideStaleReadvertiseFailureIsNotPolicy pins the same discrimination on
// the RFC 9494 stale re-advertise rail's decision half, over BOTH of its failure
// modes and the policy control.
//
// VALIDATES: AC-7 -- a readvertise member that cannot answer yields
// staleFilterFailed, a member whose modifications cannot be built yields
// staleBuildFailed, and only a member that rejects yields staleSuppress. All
// three withhold the route: the destination's LLGR capability is what the
// readvertise decision turns on, so RFC 9494 Section 4.3 forbids sending the
// stale route on a guess. What differs is the cause the caller reports.
// PREVENTS: either failure branch returning staleSuppress, which reached the
// operator as a peer that declined the family.
// Method: the destination's export chain holds the readvertise-opted member
// probe:f, answered by the externalTransport stand-in (staleRailFailureCases).
func TestDecideStaleReadvertiseFailureIsNotPolicy(t *testing.T) {
	dest := filterapi.PeerFilterInfo{
		Address: mustParseAddr("10.0.0.2"),
		PeerAS:  65001,
		LocalAS: 65000,
	}
	want := map[string]staleOutcome{
		"filter that cannot answer": staleFilterFailed,
		"unbuildable modification":  staleBuildFailed,
		"rejecting filter":          staleSuppress,
	}

	for _, tc := range staleRailFailureCases {
		t.Run(tc.name, func(t *testing.T) {
			transport := tc.transport()
			r := &Reactor{
				attrModHandlers:     tc.handlers(),
				filterTransportSeam: transport,
			}
			a := &reactorAPIAdapter{r: r}

			body := append([]byte(nil), minimalAnnounceBody...)
			outcome, modified := a.decideStaleReadvertise(dest, staleChainAt(t, staleTestExport), body, 1)

			assert.Equal(t, want[tc.name], outcome)
			assert.Nil(t, modified, "neither outcome carries a body")
			assert.Len(t, transport.sent, 1, "the opted member must be asked once")
		})
	}
}

// TestAnnounceNLRIBatchStaleFailureIsNotFamilyMismatch drives the same three
// cases from the entry point an LLGR re-advertise actually enters, so the
// distinction is proven where an operator reads it rather than only in the
// helper (ai/rules/evidence.md, test corollary).
//
// VALIDATES: AC-7 -- each failure surfaces its OWN cause, and only a policy
// suppression keeps ErrNoPeersAcceptedFamily.
// PREVENTS: reporting a defect in Ze as "no peers have family negotiated". That
// cause is untrue -- the family IS negotiated here -- and it is the cause
// DispatchNLRIGroups downgrades to a warning, so both failures reached the
// operator as a routine skip.
// Method: the peer's export policy is the chain of the readvertise-opted member
// probe:f, so the stale batch takes the readvertise rail (staleRailToward).
func TestAnnounceNLRIBatchStaleFailureIsNotFamilyMismatch(t *testing.T) {
	dest := netip.MustParseAddr("10.0.0.2")
	want := map[string]error{
		"filter that cannot answer": errStaleReadvertiseFilterFailed,
		"unbuildable modification":  errStaleReadvertiseBuildFailed,
		"rejecting filter":          route.ErrNoPeersAcceptedFamily,
	}

	for _, tc := range staleRailFailureCases {
		t.Run(tc.name, func(t *testing.T) {
			wantErr := want[tc.name]
			settings := &PeerSettings{
				Connection:   ConnectionBoth,
				Address:      dest,
				LocalAS:      65000,
				PeerAS:       65001,
				RouterID:     0x01020301,
				ExportPolicy: staleChainAt(t, staleTestExport).policy,
			}
			peer := NewPeer(settings)
			peer.state.Store(int32(PeerStateEstablished))
			peer.negotiated.Store(&NegotiatedCapabilities{
				families: map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
			})

			r := &Reactor{
				attrModHandlers:     tc.handlers(),
				config:              &Config{LocalAS: 65000},
				peers:               map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
				filterTransportSeam: tc.transport(),
			}
			a := &reactorAPIAdapter{r: r}

			err := a.AnnounceNLRIBatch(t.Context(), selector.All(), staleReadvertiseBatch(t), plugin.OperatorSender())
			assert.ErrorIs(t, err, wantErr)

			// The two failures must not ALSO read as a family mismatch, and
			// each must carry the shared wrapper so one errors.Is at the
			// caller catches both. Asserting only the specific error would
			// pass against a wrapper that had swallowed the distinction.
			if errors.Is(wantErr, errStaleReadvertiseWithheld) {
				assert.NotErrorIs(t, err, route.ErrNoPeersAcceptedFamily,
					"a failure of this speaker must not be reported as a peer declining the family")
				assert.ErrorIs(t, err, errStaleReadvertiseWithheld)
			}
		})
	}
}
