// Design: docs/architecture/plugin/rib-storage-design.md -- exact received-source ownership.
// Related: rib_source_lease_test.go -- real SDK lease calls on both transports.
// Related: source_policy.go -- source-scoped candidacy and lifecycle completion.
package rib

import (
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	pluginapi "github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRIBSourceIdentityKeepsSameAddressCandidates supplies two distinct
// Peer identities at one address. Both observed remote ports deliberately agree:
// socket metadata cannot substitute for the stable identity. The same prefix
// must remain two candidates, with independent names and original leases.
func TestRIBSourceIdentityKeepsSameAddressCandidates(t *testing.T) {
	for _, native := range []bool{false, true} {
		transport := "wire/"
		if native {
			transport = "native/"
		}
		for _, jsonRail := range []bool{false, true} {
			t.Run(transport+testReceiptRailName(jsonRail, false), func(t *testing.T) {
				r, calls := sourceLeaseTestRIB(t, native)
				first, second := sourceIdentityPeers()
				nlri := []byte{24, 10, 77, 1}
				first.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
				first.receive(t, r, jsonRail, nlri, 100)
				require.Equal(t, 1, sourceIdentityCount(t, r, first.name))
				second.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
				second.receive(t, r, jsonRail, nlri, 200)

				require.Equal(t, 1, calls.count(rpc.SourceLeaseAcquire, first.receipt))
				require.Equal(t, 1, calls.count(rpc.SourceLeaseAcquire, second.receipt))
				require.Equal(t, 2, sourceIdentityCount(t, r, ""), "one address can own two received candidates")
				require.Equal(t, 1, sourceIdentityCount(t, r, first.name))
				require.Equal(t, 1, sourceIdentityCount(t, r, second.name))
				require.Len(t, gatherCandidatesHeld(t, r, family.IPv4Unicast, nlri, false), 2)
				require.NoError(t, r.sourceLeases.settle(t.Context()))
				require.Zero(t, calls.count(rpc.SourceLeaseRelease, first.receipt), "the losing candidate still owns its original source")
				require.Zero(t, calls.count(rpc.SourceLeaseRelease, second.receipt))
			})
		}
	}
}

// TestRIBSourceIdentityDownPreservesSameAddressSibling removes one exact
// source after both have installed useful routes. Its sibling's route, name,
// election eligibility and source lease must survive on both event rails.
func TestRIBSourceIdentityDownPreservesSameAddressSibling(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(testReceiptRailName(jsonRail, false), func(t *testing.T) {
			r, calls := sourceLeaseTestRIB(t, true)
			first, second := sourceIdentityPeers()
			first.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
			first.receive(t, r, jsonRail, []byte{24, 10, 77, 1}, 100)
			second.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
			secondNLRI := []byte{24, 10, 77, 2}
			second.receive(t, r, jsonRail, secondNLRI, 100)
			require.Equal(t, 2, sourceIdentityCount(t, r, ""))

			first.state(t, r, jsonRail, rpc.SessionStateDown, rpc.ReasonPeerRemoved)

			require.Equal(t, 1, sourceIdentityCount(t, r, ""), "removing one Peer must not purge an address-wide store")
			require.Zero(t, sourceIdentityCount(t, r, first.name))
			require.Equal(t, 1, sourceIdentityCount(t, r, second.name))
			require.Len(t, gatherCandidatesHeld(t, r, family.IPv4Unicast, secondNLRI, false), 1)
			require.Equal(t, 1, calls.count(rpc.SourceLeaseRelease, first.receipt))
			require.Zero(t, calls.count(rpc.SourceLeaseRelease, second.receipt), "the remaining Peer retains useful original-source authority")
		})
	}
}

// TestRIBSourceIdentityPolicyPreservesSameAddressSibling applies a captured
// policy change to one owner at a shared address. The other owner's prefix
// remains an eligible candidate, rather than sharing its neighbor's denial.
func TestRIBSourceIdentityPolicyPreservesSameAddressSibling(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(testReceiptRailName(jsonRail, false), func(t *testing.T) {
			r, calls := sourceLeaseTestRIB(t, true)
			first, second := sourceIdentityPeers()
			firstNLRI, secondNLRI := []byte{24, 10, 77, 1}, []byte{24, 10, 77, 2}
			first.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
			first.receive(t, r, jsonRail, firstNLRI, 100)
			second.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
			second.receive(t, r, jsonRail, secondNLRI, 100)
			require.Equal(t, 2, sourceIdentityCount(t, r, ""))

			input := rpc.SourcePolicyInput{
				TransactionID: "same-address-policy", Operation: rpc.SourcePolicyPrepare,
				Source: rpc.SourcePolicyIdentity{Peer: sourceIdentityAddress, PeerOwner: first.owner,
					SessionGeneration: first.generation, PolicyEpoch: 5, ReceiveFrontier: first.message,
					TemplateGeneration: 1},
				DeadlineMS: time.Now().Add(time.Minute).UnixMilli(),
			}
			require.NoError(t, input.Validate(time.Now()))
			require.NoError(t, r.handleSourcePolicy(t.Context(), input))
			input.Operation = rpc.SourcePolicyApply
			input.NextPolicyEpoch = 6
			require.NoError(t, input.Validate(time.Now()))
			require.NoError(t, r.handleSourcePolicy(t.Context(), input))

			require.Empty(t, gatherCandidatesHeld(t, r, family.IPv4Unicast, firstNLRI, false))
			require.Len(t, gatherCandidatesHeld(t, r, family.IPv4Unicast, secondNLRI, false), 1,
				"a policy epoch belongs to its exact original Peer, not every Peer at the address")
			require.Equal(t, 1, sourceIdentityCount(t, r, second.name))
			require.Zero(t, calls.count(rpc.SourceLeaseRelease, second.receipt))
		})
	}
}

// TestRIBSourceIdentityFinalizePreservesSameAddressSibling finalizes one
// source's policy removal. Only that owner's denied paths and leases leave;
// the other source at the address keeps its stored and useful candidate.
func TestRIBSourceIdentityFinalizePreservesSameAddressSibling(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(testReceiptRailName(jsonRail, false), func(t *testing.T) {
			r, calls := sourceLeaseTestRIB(t, true)
			first, second := sourceIdentityPeers()
			firstNLRI := []byte{24, 10, 77, 1}
			first.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
			first.receive(t, r, jsonRail, firstNLRI, 100)
			second.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
			second.receive(t, r, jsonRail, []byte{24, 10, 77, 2}, 100)
			require.Equal(t, 2, sourceIdentityCount(t, r, ""))

			input := rpc.SourcePolicyInput{
				TransactionID: "same-address-finalize", Operation: rpc.SourcePolicyPrepare,
				Source: rpc.SourcePolicyIdentity{Peer: sourceIdentityAddress, PeerOwner: second.owner,
					SessionGeneration: second.generation, PolicyEpoch: 5, ReceiveFrontier: second.message,
					TemplateGeneration: 1},
				DeadlineMS: time.Now().Add(time.Minute).UnixMilli(),
			}
			require.NoError(t, input.Validate(time.Now()))
			require.NoError(t, r.handleSourcePolicy(t.Context(), input))
			input.Operation = rpc.SourcePolicyApply
			input.NextPolicyEpoch = 6
			require.NoError(t, input.Validate(time.Now()))
			require.NoError(t, r.handleSourcePolicy(t.Context(), input))
			input.Operation = rpc.SourcePolicyFinalize
			input.NextPolicyEpoch = 0
			require.NoError(t, input.Validate(time.Now()))
			require.NoError(t, r.handleSourcePolicy(t.Context(), input))

			require.Equal(t, 1, sourceIdentityCount(t, r, ""))
			require.Equal(t, 1, sourceIdentityCount(t, r, first.name))
			require.Zero(t, sourceIdentityCount(t, r, second.name))
			require.Len(t, gatherCandidatesHeld(t, r, family.IPv4Unicast, firstNLRI, false), 1)
			require.Zero(t, calls.count(rpc.SourceLeaseRelease, first.receipt))
			require.Equal(t, 1, calls.count(rpc.SourceLeaseRelease, second.receipt))
		})
	}
}

// TestRIBSourceIdentityReconnectRejectsOldDown changes the issuing receipt and
// observed TCP port while the stable Peer owner survives. An old connection's
// late DOWN cannot delete the replacement; the current connection's DOWN must.
func TestRIBSourceIdentityReconnectRejectsOldDown(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(testReceiptRailName(jsonRail, false), func(t *testing.T) {
			r, calls := sourceLeaseTestRIB(t, true)
			first, _ := sourceIdentityPeers()
			nlri := []byte{24, 10, 77, 1}
			first.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
			first.receive(t, r, jsonRail, nlri, 100)
			reconnected := sourceIdentityPeer{owner: first.owner, name: first.name,
				session: first.session + 100, receipt: first.receipt + 100,
				message: first.message + 100, generation: first.generation + 100,
				remotePort: first.remotePort + 1}
			reconnected.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
			reconnected.receive(t, r, jsonRail, nlri, 200)
			require.NoError(t, r.sourceLeases.settle(t.Context()))
			require.Equal(t, 1, sourceIdentityCount(t, r, first.name), "reconnect replaces one Peer store instead of creating a port-keyed history")
			require.Equal(t, 1, calls.count(rpc.SourceLeaseRelease, first.receipt))
			require.Zero(t, calls.count(rpc.SourceLeaseRelease, reconnected.receipt))

			first.state(t, r, jsonRail, rpc.SessionStateDown, rpc.ReasonTCPFailure)

			require.Equal(t, 1, sourceIdentityCount(t, r, first.name), "the old connection owns no removal of its successor")
			require.Len(t, gatherCandidatesHeld(t, r, family.IPv4Unicast, nlri, false), 1)
			require.Zero(t, calls.count(rpc.SourceLeaseRelease, reconnected.receipt))
			reconnected.state(t, r, jsonRail, rpc.SessionStateDown, rpc.ReasonPeerRemoved)
			require.Zero(t, sourceIdentityCount(t, r, ""), "the current connection still owns its actual removal")
		})
	}
}

const sourceIdentityAddress = "192.0.2.188"

type sourceIdentityPeer struct {
	owner      uint64
	name       string
	session    uint64
	receipt    uint64
	message    uint64
	generation uint64
	remotePort uint16
}

func sourceIdentityPeers() (sourceIdentityPeer, sourceIdentityPeer) {
	return sourceIdentityPeer{owner: 1701, name: "source-one", session: 2701,
			receipt: 3701, message: 4701, generation: 5701, remotePort: 49152},
		sourceIdentityPeer{owner: 1702, name: "source-two", session: 2702,
			receipt: 3702, message: 4702, generation: 5702, remotePort: 49152}
}

// state feeds the actual JSON or structured consumer. Attached UP requires no
// destination send grant or replay; these fixtures are source peers only.
func (peer sourceIdentityPeer) state(t *testing.T, r *RIBManager, jsonRail bool, state rpc.SessionState, reason string) {
	t.Helper()
	var generation uint64
	if state == rpc.SessionStateDown {
		generation = peer.generation
	}
	if jsonRail {
		spelling, err := state.MarshalText()
		require.NoError(t, err)
		encoded, err := json.Marshal(&Event{Type: "state", State: string(spelling), Reason: reason,
			PeerOwner: peer.owner, DestinationSession: peer.session, SessionGeneration: generation,
			Peer: mustMarshal(t, PeerInfoJSON{Name: peer.name, Remote: PeerRemoteInfo{Address: sourceIdentityAddress, AS: 65001}})})
		require.NoError(t, err)
		event, err := parseEvent(encoded)
		require.NoError(t, err)
		r.dispatch(event)
		return
	}
	r.dispatchStructured(&rpc.StructuredEvent{EventType: rpc.EventKindState,
		PeerOwner: peer.owner, PeerAddress: sourceIdentityAddress, PeerName: peer.name,
		PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000, RemotePort: peer.remotePort,
		DestinationSession: peer.session, SessionGeneration: generation, State: state, Reason: reason})
}

// receive keeps the original owner/receipt in both the raw message and the
// actual formatted JSON or structured event. The shared SDK fixture records
// acquisitions and releases, rather than granting source ownership by address.
func (peer sourceIdentityPeer) receive(t *testing.T, r *RIBManager, jsonRail bool, nlri []byte, preference uint32) {
	t.Helper()
	attrs := unicastAttrs([4]byte{10, 0, 0, 1}, 30, preference)
	body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
	body = append(body, nlri...)
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)
	wu := wireu.NewWireUpdate(body, ctxID)
	wu.SetSourceReceipt(peer.receipt)
	wu.SetSourceOwner(peer.owner)
	aw, err := wu.Attrs()
	require.NoError(t, err)
	message := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, MessageID: peer.message,
		SourcePolicyEpoch: 5, SourceReceipt: peer.receipt, SourceOwner: peer.owner,
		RawBytes: body, WireUpdate: wu, AttrsWire: aw}
	if jsonRail {
		address := netip.MustParseAddr(sourceIdentityAddress)
		info := pluginapi.PeerInfo{ConnectionID: 41, PeerOwner: peer.owner, Name: peer.name,
			Address: address, AddressStr: sourceIdentityAddress,
			PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000, RemotePort: peer.remotePort}
		encoded := format.AppendMessage(nil, &info, message,
			bgptypes.ContentConfig{Encoding: pluginapi.EncodingJSON, Format: pluginapi.FormatFull})
		event, err := parseEvent(encoded)
		require.NoError(t, err)
		r.dispatch(event)
		return
	}
	r.dispatchStructured(&rpc.StructuredEvent{ConnectionID: 41, EventType: rpc.EventKindUpdate,
		Direction: rpc.DirectionReceived, PeerOwner: peer.owner,
		PeerAddress: sourceIdentityAddress, PeerName: peer.name,
		PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000, RemotePort: peer.remotePort,
		SourceReceipt: peer.receipt, SourcePolicyEpoch: 5, RawMessage: &message})
}

func sourceIdentityCount(t *testing.T, r *RIBManager, name string) int {
	t.Helper()
	args := []string{"received", "count"}
	if name != "" {
		args = []string{"received", "peer", name, "count"}
	}
	answer, ok := r.showPipeline("*", args).(map[string]any)
	require.True(t, ok, "the received-route command must return a count")
	count, ok := answer[jsonKeyCount].(int)
	require.True(t, ok, "the command count must retain its declared integer shape")
	return count
}
