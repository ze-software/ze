// Design: docs/architecture/plugin/rib-storage-design.md -- received source leases.
package rib

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"sync"
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
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type sourceLeaseCalls struct {
	mu     sync.Mutex
	values []rpc.SourceLeaseInput
}

func (c *sourceLeaseCalls) count(action rpc.SourceLeaseAction, receipt uint64) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, input := range c.values {
		if input.Action != action {
			continue
		}
		if input.SourceReceipt == receipt {
			n++
		}
	}
	return n
}

// The native callback and framed Mux server run the real SDK SourceLease path.
// The callback refuses any operation performed while peerMu is held.
func sourceLeaseTestRIB(t *testing.T, native bool) (*RIBManager, *sourceLeaseCalls) {
	t.Helper()
	clientEnd, engineEnd := net.Pipe()
	connection := clientEnd
	calls := &sourceLeaseCalls{}
	var r *RIBManager
	handle := func(input rpc.SourceLeaseInput) (*rpc.SourceLeaseOutput, error) {
		if err := input.Validate(time.Now()); err != nil {
			return nil, err
		}
		if !r.peerMu.TryLock() {
			return nil, errors.New("source lease RPC held peerMu")
		}
		r.peerMu.Unlock()
		calls.mu.Lock()
		calls.values = append(calls.values, input)
		calls.mu.Unlock()
		return &rpc.SourceLeaseOutput{Action: input.Action, SourceReceipt: input.SourceReceipt, PeerOwner: input.PeerOwner}, nil
	}
	var engine *rpc.MuxConn
	serverDone := make(chan struct{})
	if native {
		bridge := rpc.NewDirectBridge()
		bridge.SetDispatchNative(func(_ context.Context, method string, raw any) (any, bool, error) {
			if method != rpc.MethodSourceLease {
				return nil, false, errors.New("unexpected engine operation")
			}
			input, ok := raw.(rpc.SourceLeaseInput)
			if !ok {
				return nil, true, errors.New("source lease native input has wrong type")
			}
			output, err := handle(input)
			return output, true, err
		})
		bridge.SetReady()
		connection = rpc.NewBridgedConn(clientEnd, bridge)
		close(serverDone)
	} else {
		engine = rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
		go func() {
			defer close(serverDone)
			for request := range engine.Requests() {
				var input rpc.SourceLeaseInput
				err := json.Unmarshal(request.Params, &input)
				if request.Method != rpc.MethodSourceLease {
					err = errors.New("unexpected engine operation")
				}
				var output *rpc.SourceLeaseOutput
				if err == nil {
					output, err = handle(input)
				}
				if err != nil {
					if engine.SendError(context.Background(), request.ID, err.Error()) != nil {
						return
					}
					continue
				}
				if engine.SendResult(context.Background(), request.ID, output) != nil {
					return
				}
			}
		}()
	}
	p := sdk.NewWithConn("bgp-rib", connection)
	r = newRIBManager(p)
	r.sourceLeases = newRIBSourceLeases(r)
	electionFixtureLocRIB(t, r, netip.MustParseAddr("10.0.0.1"))
	t.Cleanup(func() {
		for _, routes := range r.bgpPeers {
			routes.Release()
		}
		require.NoError(t, r.sourceLeases.settle(context.Background()))
		require.NoError(t, p.Close())
		if engine != nil {
			require.NoError(t, engine.Close())
		} else {
			require.NoError(t, engineEnd.Close())
		}
		<-serverDone
	})
	return r, calls
}

func sourceLeaseReceive(t *testing.T, r *RIBManager, peer netip.Addr, peerOwner, receipt uint64, jsonRail, labeled bool) (family.Family, []byte) {
	t.Helper()
	attrs := unicastAttrs([4]byte{10, 0, 0, 1}, 30, 100)
	body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
	body = append(body, 8, 10)
	fam := family.IPv4Unicast
	if labeled {
		fam = labeledFamily
		body = labeledUpdateBody([4]byte{10, 0, 0, 1}, 30, labeledNLRI(0, false, netip.MustParsePrefix("10.0.0.0/8"), []uint32{100}))
	}
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)
	wu := wireu.NewWireUpdate(body, ctxID)
	wu.SetSourceOwner(peerOwner)
	wu.SetSourceReceipt(receipt)
	aw, err := wu.Attrs()
	require.NoError(t, err)
	msg := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, MessageID: 9, SourcePolicyEpoch: 5, SourceReceipt: receipt, SourceOwner: peerOwner, RawBytes: body, WireUpdate: wu, AttrsWire: aw}
	if jsonRail {
		info := pluginapi.PeerInfo{PeerOwner: peerOwner, Address: peer, AddressStr: peer.String(), PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000}
		line := format.AppendMessage(nil, &info, msg, bgptypes.ContentConfig{Encoding: pluginapi.EncodingJSON, Format: pluginapi.FormatFull})
		event, err := parseEvent(line)
		require.NoError(t, err)
		r.dispatch(event)
	} else {
		r.handleReceivedStructured(&rpc.StructuredEvent{EventType: rpc.EventKindUpdate, PeerOwner: peerOwner, PeerAddress: peer.String(), PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000, SourceReceipt: receipt, RawMessage: &msg})
	}
	return fam, []byte{8, 10}
}

func TestRIBSourceLeaseCandidateLifetime(t *testing.T) {
	for _, native := range []bool{false, true} {
		for _, jsonRail := range []bool{false, true} {
			for _, labeled := range []bool{false, true} {
				t.Run(map[bool]string{false: "wire-", true: "native-"}[native]+testReceiptRailName(jsonRail, labeled), func(t *testing.T) {
					r, calls := sourceLeaseTestRIB(t, native)
					peer := netip.MustParseAddr("192.0.2.101")
					alternate := netip.MustParseAddr("192.0.2.102")
					r.peerUp[peer] = true
					fam, raw := sourceLeaseReceive(t, r, peer, 701, 31, jsonRail, labeled)
					sourceLeaseReceive(t, r, alternate, 702, 41, jsonRail, labeled)
					require.Equal(t, 1, calls.count(rpc.SourceLeaseAcquire, 31))
					require.Equal(t, 1, calls.count(rpc.SourceLeaseAcquire, 41), "an alternate also owns a lease")
					pinned, found := r.bgpPeers[peer].LookupRetained(fam, raw)
					require.True(t, found)
					sourceLeaseReceive(t, r, peer, 701, 32, jsonRail, labeled)
					require.NoError(t, r.sourceLeases.settle(t.Context()))
					require.Zero(t, calls.count(rpc.SourceLeaseRelease, 31), "an actual owned clone keeps the old source")
					pinned.Release()
					require.NoError(t, r.sourceLeases.settle(t.Context()))
					require.Equal(t, 1, calls.count(rpc.SourceLeaseRelease, 31))
					_, err := r.retainRoutes(peer.String(), nil, false)
					require.NoError(t, err)
					r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: peer.String(), State: rpc.SessionStateDown, Reason: rpc.ReasonTCPFailure})
					require.NotNil(t, r.bgpPeers[peer])
					require.Positive(t, calls.count(rpc.SourceLeaseRetain, 32))
					require.Zero(t, calls.count(rpc.SourceLeaseRelease, 32))
					if jsonRail {
						r.handleState(&Event{Peer: mustMarshal(t, map[string]any{"remote": map[string]any{"address": peer.String()}}), State: "down", Reason: rpc.ReasonPeerRemoved})
					} else {
						r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: peer.String(), State: rpc.SessionStateDown, Reason: rpc.ReasonPeerRemoved})
					}
					require.Nil(t, r.bgpPeers[peer], "permanent removal purges an already-DOWN retained source")
					require.Equal(t, 1, calls.count(rpc.SourceLeaseRelease, 32))
					require.NotNil(t, r.bgpPeers[alternate])
					require.Zero(t, calls.count(rpc.SourceLeaseRelease, 41))
					r.sourceLeases.mu.Lock()
					require.Len(t, r.sourceLeases.leases, 1, "only the remaining current source survives; no receipt history")
					r.sourceLeases.mu.Unlock()
				})
			}
		}
	}
}

func TestRIBSourceLeaseSourcePolicyAbort(t *testing.T) {
	r, calls := sourceLeaseTestRIB(t, true)
	peer := netip.MustParseAddr("192.0.2.103")
	r.peerUp[peer] = true
	sourceLeaseReceive(t, r, peer, 1, 51, false, false)
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "lease-abort", peer, 5, 0))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "lease-abort", peer, 5, 6))
	require.Zero(t, calls.count(rpc.SourceLeaseRelease, 51), "the denied in-place candidate is still needed for abort")
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "lease-abort", peer, 5, 7))
	require.Zero(t, calls.count(rpc.SourceLeaseRelease, 51))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "lease-finalize", peer, 7, 0))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "lease-finalize", peer, 7, 8))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyFinalize, "lease-finalize", peer, 7, 0))
	require.Equal(t, 1, calls.count(rpc.SourceLeaseRelease, 51))
}
