// Design: docs/architecture/plugin/rib-storage-design.md -- whole original RIB UPDATE progress.
package rib

import (
	"encoding/binary"
	"encoding/json"
	"github.com/stretchr/testify/require"
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
)

// The actual native/full JSON receive callbacks commit FlowSpec removals and
// useful siblings before progress. SourceLease uses both real SDK transports;
// the existing instance command, rather than a global provider, answers it.
func TestRIBOriginalProgressFollowsWholeUpdateOnBothReceiveRails(t *testing.T) {
	for _, native := range []bool{false, true} {
		for _, jsonRail := range []bool{false, true} {
			t.Run(map[bool]string{false: "framed-lease", true: "native-lease"}[native]+map[bool]string{false: "/typed", true: "/full-json"}[jsonRail], func(t *testing.T) {
				r, _ := sourceLeaseTestRIB(t, native)
				previousBus := eventBusPtr.Load()
				bus := newTestEventBus()
				SetEventBus(bus)
				t.Cleanup(func() { eventBusPtr.Store(previousBus) })
				address := netip.MustParseAddr("192.0.2.188")
				other := netip.MustParseAddr("192.0.2.189")
				r.peerMu.Lock()
				r.recordPeerStateLocked(7, address, 17, 9, true)
				r.recordPeerStateLocked(8, other, 18, 10, true)
				r.peerMu.Unlock()
				fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
				contextID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
				require.NoError(t, err)
				var activeRoute ribevents.ValidationRoute
				var activeID uint64
				inFlight := 0
				checkInFlight := true
				unsubscribe := ribevents.ValidationChange.Subscribe(bus, func([]ribevents.ValidationRoute) {
					if !checkInFlight || activeID == 0 {
						return
					}
					caught, err := r.sourceProgress(activeRoute, activeID)
					require.NoError(t, err)
					require.False(t, caught, "local subscriber cannot observe a partially applied UPDATE as caught up")
					inFlight++
				})
				t.Cleanup(unsubscribe)
				deliver := func(peer netip.Addr, owner, connection, receipt, id uint64, rule []byte, withdraw bool) {
					activeRoute = ribevents.ValidationRoute{Peer: peer, PeerOwner: owner, ConnectionID: connection}
					activeID = id
					defer func() { activeID = 0 }()
					attrs := flowValidationAttrs(flowValidationPath(2, 65001), netip.Addr{}, 0, nil)
					if len(rule) > 0 {
						mp := []byte{0, 1, byte(fam.SAFI)}
						code := byte(15)
						if !withdraw {
							code = 14
							mp = append(mp, 0, 0)
						}
						mp = append(mp, rule...)
						attrs = append(attrs, 0x80, code, byte(len(mp)))
						attrs = append(attrs, mp...)
					}
					body := []byte{0, 0, 0, 0}
					binary.BigEndian.PutUint16(body[2:], uint16(len(attrs)))
					body = append(body, attrs...)
					wire := wireu.NewWireUpdate(body, contextID)
					wire.SetSourceOwner(owner)
					wire.SetSourceReceipt(receipt)
					attributes, err := wire.Attrs()
					require.NoError(t, err)
					message := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, Direction: rpc.DirectionReceived, RawBytes: body, WireUpdate: wire, AttrsWire: attributes, MessageID: id, SourcePolicyEpoch: 5, SourceReceipt: receipt, SourceOwner: owner}
					if jsonRail {
						info := pluginapi.PeerInfo{Address: peer, AddressStr: peer.String(), PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000, PeerOwner: owner, ConnectionID: connection}
						raw := format.AppendMessage(nil, &info, message, bgptypes.ContentConfig{Encoding: pluginapi.EncodingJSON, Format: pluginapi.FormatFull})
						event, err := parseEvent(raw)
						require.NoError(t, err)
						r.handleReceived(event)
					} else {
						r.handleReceivedStructured(&rpc.StructuredEvent{EventType: rpc.EventKindUpdate, PeerAddress: peer.String(), PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000, PeerOwner: owner, ConnectionID: connection, RawMessage: &message})
					}
				}
				b := []byte{5, 1, 24, 10, 77, 0}
				c := []byte{5, 1, 24, 10, 78, 0}
				query := func(rule []byte, id, min uint64) (ribevents.FlowSpecAnswer, error) {
					input := ribevents.FlowSpecRequest{Queries: []ribevents.FlowSpecQuery{{Operation: ribevents.FlowSpecPathOf, Route: ribevents.ValidationRoute{Peer: address, PeerOwner: 7, ConnectionID: 9, Family: fam, NLRI: ribevents.FlowSpecKey(rule)}, MsgID: id, AfterMsgID: min}}}
					raw, err := json.Marshal(input)
					require.NoError(t, err)
					status, data, err := r.flowSpecGateAnswerCommand("", []string{string(raw)})
					if err != nil {
						return ribevents.FlowSpecAnswer{}, err
					}
					require.Equal(t, statusDone, status)
					answer, ok := data.(ribevents.FlowSpecAnswer)
					require.True(t, ok)
					return answer, nil
				}
				deliver(address, 7, 9, 31, 10, b, false)
				answer, err := query(b, 10, 11)
				require.NoError(t, err)
				require.True(t, answer.Pending)
				require.Empty(t, answer.Results)
				deliver(other, 8, 10, 41, 1000, c, false)
				r.peerMu.Lock()
				r.recordPeerStateLocked(7, address, 17, 9, true)
				r.peerMu.Unlock()
				answer, err = query(b, 10, 11)
				require.NoError(t, err)
				require.True(t, answer.Pending, "foreign higher message and duplicate UP cannot satisfy original floor")
				deliver(address, 7, 9, 31, 11, b, true)
				answer, err = query(b, 10, 11)
				require.NoError(t, err)
				require.False(t, answer.Pending)
				require.Len(t, answer.Results, 1)
				require.False(t, answer.Results[0].Verdict, "withdrawn B is absent only after its original mutation")
				deliver(address, 7, 9, 31, 12, c, false)
				answer, err = query(c, 12, 12)
				require.NoError(t, err)
				require.Len(t, answer.Results, 1)
				require.True(t, answer.Results[0].Verdict)
				require.Equal(t, uint64(31), answer.Results[0].Path.SourceReceipt)
				deliver(address, 7, 9, 31, 13, nil, false)
				answer, err = query(c, 12, 13)
				require.NoError(t, err)
				require.False(t, answer.Pending)
				require.True(t, answer.Results[0].Verdict, "empty UPDATE commits progress without deleting useful C")
				require.Positive(t, inFlight, "actual local FlowSpec subscriber observed a committed-store selection phase before progress publication")
				r.peerMu.Lock()
				r.recordPeerStateLocked(7, address, 27, 19, true)
				r.peerMu.Unlock()
				_, err = query(c, 12, 13)
				require.Error(t, err, "successor cannot answer predecessor physical progress")
				caught, err := r.sourceProgress(ribevents.ValidationRoute{Peer: address, PeerOwner: 7, ConnectionID: 19}, 1)
				require.NoError(t, err)
				require.False(t, caught)
				checkInFlight = false
				deliver(address, 7, 19, 32, 20, []byte{5, 1, 24, 10}, false)
				current := ribevents.ValidationRoute{Peer: address, PeerOwner: 7, ConnectionID: 19}
				_, err = r.sourceProgress(current, 20)
				require.Error(t, err, "truncated original UPDATE cannot certify false current absence")
				deliver(address, 7, 19, 32, 21, c, false)
				_, err = r.sourceProgress(current, 20)
				require.Error(t, err, "a larger later valid UPDATE cannot conceal the unapplied gap")
				r.peerMu.Lock()
				r.recordPeerStateLocked(7, address, 27, 19, true)
				r.peerMu.Unlock()
				_, err = r.sourceProgress(current, 20)
				require.Error(t, err, "duplicate physical UP cannot clear original failed application")
				caught, err = r.sourceProgress(ribevents.ValidationRoute{Peer: other, PeerOwner: 8, ConnectionID: 10}, 1000)
				require.NoError(t, err)
				require.True(t, caught, "foreign healthy actor remains useful")
				r.peerMu.Lock()
				r.recordPeerStateLocked(7, address, 37, 29, true)
				r.peerMu.Unlock()
				caught, err = r.sourceProgress(ribevents.ValidationRoute{Peer: address, PeerOwner: 7, ConnectionID: 29}, 1)
				require.NoError(t, err)
				require.False(t, caught, "new physical actor resets only its own failed progress")
			})
		}
	}
}
