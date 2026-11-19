// Design: docs/architecture/plugin/rib-storage-design.md -- original source ownership.
package adj_rib_in

import (
	"context"
	"github.com/stretchr/testify/require"
	bgp "github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	pluginapi "github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"net/netip"
	"testing"
	"time"
)

// adjOwnerProducer drives genuine structured and full-JSON event producers.
// Each fixture declares its original owner and physical connection explicitly;
// neither field is reconstructed from the shared address.
type adjOwnerProducer struct {
	t       *testing.T
	r       *AdjRIBInManager
	json    bool
	address netip.Addr
	context bgpctx.ContextID
	receipt uint64 // passive carrier fixture value, never acquired engine authority
}

func newAdjOwnerProducer(t *testing.T, jsonRail bool) *adjOwnerProducer {
	t.Helper()
	ctx, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)
	return &adjOwnerProducer{t: t, r: newTestManager(t), json: jsonRail, address: netip.MustParseAddr("192.0.2.188"), context: ctx}
}

func (p *adjOwnerProducer) state(owner, connection uint64, up bool) {
	p.t.Helper()
	state := rpc.SessionStateDown
	if up {
		state = rpc.SessionStateUp
	}
	if p.json {
		word := "down"
		if up {
			word = "up"
		}
		p.r.handleState(&bgp.Event{Type: "state", State: word, Reason: rpc.ReasonAttached, PeerOwner: owner, ConnectionID: connection,
			Peer: mustMarshal(p.t, bgp.PeerInfoJSON{Remote: bgp.PeerRemoteInfo{Address: p.address.String(), AS: 65000}})})
		return
	}
	p.r.handleStructuredState(&rpc.StructuredEvent{EventType: rpc.EventKindState, PeerAddress: p.address.String(),
		PeerOwner: owner, ConnectionID: connection, State: state, Reason: rpc.ReasonAttached})
}

func (p *adjOwnerProducer) receive(owner, connection, msgID uint64, octet byte) ribevents.ValidationRoute {
	p.t.Helper()
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 1}
	body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
	body = append(body, 24, 10, 77, octet)
	wire := wireu.NewWireUpdate(body, p.context)
	wire.SetSourceOwner(owner)
	packed, err := wire.Attrs()
	require.NoError(p.t, err)
	msg := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, MessageID: msgID, SourceOwner: owner, SourceReceipt: p.receipt, SourcePolicyEpoch: 5, RawBytes: body, WireUpdate: wire, AttrsWire: packed}
	if p.json {
		info := pluginapi.PeerInfo{Address: p.address, AddressStr: p.address.String(), PeerOwner: owner, ConnectionID: connection, PeerAS: rpc.RemoteASOf(65000), LocalAS: 65000}
		line := format.AppendMessage(nil, &info, msg, bgptypes.ContentConfig{Encoding: pluginapi.EncodingJSON, Format: pluginapi.FormatFull})
		event, err := bgp.ParseEvent(line)
		require.NoError(p.t, err)
		p.r.handleReceived(event)
	} else {
		p.r.handleReceivedStructured(&rpc.StructuredEvent{EventType: rpc.EventKindUpdate, PeerAddress: p.address.String(), PeerOwner: owner,
			ConnectionID: connection, PeerAS: rpc.RemoteASOf(65000), LocalAS: 65000, RawMessage: &msg})
	}
	return ribevents.ValidationRoute{Peer: p.address, PeerOwner: owner, ConnectionID: connection, Family: family.IPv4Unicast,
		Prefix: netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 77, octet, 0}), 24)}
}

func (p *adjOwnerProducer) policy(owner uint64, op rpc.SourcePolicyOperation, transaction string, next uint64) error {
	p.t.Helper()
	input := rpc.SourcePolicyInput{TransactionID: transaction, Operation: op,
		Source:          rpc.SourcePolicyIdentity{Peer: p.address.String(), PeerOwner: owner, SessionGeneration: 3, PolicyEpoch: 5, ReceiveFrontier: 99, TemplateGeneration: 1},
		NextPolicyEpoch: next, DeadlineMS: time.Now().Add(time.Minute).UnixMilli()}
	require.NoError(p.t, input.Validate(time.Now()))
	return p.r.handleSourcePolicy(context.Background(), input)
}

// Same-address owners can hold independent policy transactions. Applying and
// finalizing one removes only its bytes; the sibling remains useful throughout.
// MUTATION: key source-policy ledgers or denial bounds by address to redden this.
func TestAdjSourcePolicySeparatesSameAddressOwnersBothRails(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(map[bool]string{false: "typed", true: "full-json"}[jsonRail], func(t *testing.T) {
			p := newAdjOwnerProducer(t, jsonRail)
			p.state(1701, 41, true)
			p.state(1702, 42, true)
			first := p.receive(1701, 41, 71, 0)
			sibling := p.receive(1702, 42, 72, 1)
			require.True(t, p.r.routeEligible(first, 71))
			require.True(t, p.r.routeEligible(sibling, 72))
			require.NoError(t, p.policy(1701, rpc.SourcePolicyPrepare, "first", 0))
			require.NoError(t, p.policy(1702, rpc.SourcePolicyPrepare, "sibling", 0))
			require.NoError(t, p.policy(1701, rpc.SourcePolicyApply, "first", 6))
			require.False(t, p.r.routeEligible(first, 71))
			require.True(t, p.r.routeEligible(sibling, 72), "sibling retains useful output")
			require.NoError(t, p.policy(1701, rpc.SourcePolicyFinalize, "first", 0))
			require.False(t, p.r.routePresent(first))
			require.True(t, p.r.routeEligible(sibling, 72))
			require.NoError(t, p.policy(1702, rpc.SourcePolicyAbort, "sibling", 0))
		})
	}
}

// A late DOWN from a replaced connection cannot clear a successor's routes,
// its current policy transaction, or another owner's same-address routes.
// MUTATION: process captured DOWN through the address-only removal to redden this.
func TestAdjSourcePolicyLateDownPreservesSuccessorAndSiblingBothRails(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(map[bool]string{false: "typed", true: "full-json"}[jsonRail], func(t *testing.T) {
			p := newAdjOwnerProducer(t, jsonRail)
			p.state(1701, 41, true)
			p.state(1702, 42, true)
			sibling := p.receive(1702, 42, 71, 1)
			p.state(1701, 43, true)
			successor := p.receive(1701, 43, 72, 0)
			require.NoError(t, p.policy(1701, rpc.SourcePolicyPrepare, "successor", 0))
			p.state(1701, 41, false)
			require.True(t, p.r.routeEligible(successor, 72))
			require.True(t, p.r.routeEligible(sibling, 71))
			require.NoError(t, p.policy(1701, rpc.SourcePolicyAbort, "successor", 0))
			p.state(1701, 43, false)
			require.False(t, p.r.routePresent(successor))
			require.True(t, p.r.routeEligible(sibling, 71), "current owner DOWN preserves sibling")
			require.Error(t, p.policy(1701, rpc.SourcePolicyPrepare, "ended", 0))
		})
	}
}

// Missing original physical facts cannot borrow the sibling's established
// address. Once the actual owner establishes, unknown DOWN cannot retire it.
// MUTATION: accept connection zero as a live owner to redden this boundary.
func TestAdjSourcePolicyUnknownPhysicalStateHasNoAuthorityBothRails(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(map[bool]string{false: "typed", true: "full-json"}[jsonRail], func(t *testing.T) {
			p := newAdjOwnerProducer(t, jsonRail)
			p.state(1702, 42, true)
			sibling := p.receive(1702, 42, 71, 1)
			first := p.receive(1701, 41, 72, 0)
			p.state(1701, 0, true)
			require.Error(t, p.policy(1701, rpc.SourcePolicyPrepare, "unknown", 0))
			require.True(t, p.r.routeEligible(sibling, 71))
			p.state(1701, 41, true)
			require.NoError(t, p.policy(1701, rpc.SourcePolicyPrepare, "actual", 0))
			p.state(0, 0, false)
			p.state(1701, 0, false)
			require.True(t, p.r.routeEligible(first, 72))
			require.True(t, p.r.routeEligible(sibling, 71))
			require.NoError(t, p.policy(1701, rpc.SourcePolicyAbort, "actual", 0))
		})
	}
}

// Observational DOWN still reclaims unowned input while authenticated same-
// address routes remain present and useful. Both UPDATE paths preserve the
// absent original facts, rather than assigning the sibling's current identity.
func TestAdjObservedDownCleansOnlyUnownedInputBothRails(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(map[bool]string{false: "typed", true: "full-json"}[jsonRail], func(t *testing.T) {
			p := newAdjOwnerProducer(t, jsonRail)
			p.state(1701, 41, true)
			actual := p.receive(1701, 41, 71, 0)
			unowned := p.receive(0, 0, 72, 1)
			require.False(t, p.r.routeEligible(unowned, 72), "observations cannot lend source authority")
			require.Equal(t, 2, p.r.status().(map[string]any)["total-routes"], "unowned input remains visible")
			p.state(0, 0, false)
			require.Equal(t, 1, p.r.status().(map[string]any)["total-routes"], "observational DOWN reclaims its input")
			require.True(t, p.r.routeEligible(actual, 71), "observational cleanup cannot retire an authenticated source")
		})
	}
}
