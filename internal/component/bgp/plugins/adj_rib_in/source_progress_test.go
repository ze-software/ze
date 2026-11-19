// Design: docs/architecture/plugin/rib-storage-design.md -- original source processing frontier.
package adj_rib_in

import (
	"encoding/binary"
	"encoding/json"
	"github.com/stretchr/testify/require"
	bgp "github.com/ze-software/ze/internal/component/bgp"
	bgpformat "github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"net/netip"
	"testing"
)

// Actual typed and full JSON receive producers process removed B and useful C,
// then the existing eligibility command distinguishes lag from actual absence.
// A larger unrelated source ID, duplicate UP, empty UPDATE and new physical
// connection cannot invent original-source catch-up.
func TestEligibilityProgressFollowsWholeOriginalUpdateOnBothRails(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(map[bool]string{false: "typed", true: "json"}[jsonRail], func(t *testing.T) {
			r := newTestManager(t)
			address := netip.MustParseAddr("192.0.2.188")
			require.True(t, r.recordPeerState(address, true, 7, 9))
			contextID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
			require.NoError(t, err)
			deliver := func(peer netip.Addr, owner, connection, id uint64, withdraw, announce []byte) {
				attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 188}
				body := make([]byte, 4+len(withdraw)+len(attrs)+len(announce))
				binary.BigEndian.PutUint16(body[:2], uint16(len(withdraw)))
				copy(body[2:], withdraw)
				at := 2 + len(withdraw)
				binary.BigEndian.PutUint16(body[at:at+2], uint16(len(attrs)))
				copy(body[at+2:], attrs)
				copy(body[at+2+len(attrs):], announce)
				wire := wireu.NewWireUpdate(body, contextID)
				attributes, err := wire.Attrs()
				require.NoError(t, err)
				message := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, RawBytes: body, WireUpdate: wire, AttrsWire: attributes, MessageID: id, SourcePolicyEpoch: 1, SourceOwner: owner, Direction: rpc.DirectionReceived}
				if jsonRail {
					bytes := bgpformat.AppendMessage(nil, &plugin.PeerInfo{Address: peer, PeerAS: rpc.RemoteASOf(65001), PeerOwner: owner, ConnectionID: connection}, message, bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatFull})
					event, err := bgp.ParseEvent(bytes)
					require.NoError(t, err)
					r.handleReceived(event)
				} else {
					r.handleReceivedStructured(&rpc.StructuredEvent{EventType: rpc.EventKindUpdate, PeerAddress: peer.String(), PeerOwner: owner, ConnectionID: connection, RawMessage: &message})
				}
			}
			query := func(prefix string, msgID, min uint64) (ribevents.EligibilityAnswer, error) {
				request := ribevents.EligibilityRequest{Queries: []ribevents.EligibilityQuery{{Route: ribevents.ValidationRoute{Peer: address, PeerOwner: 7, ConnectionID: 9, Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix(prefix)}, MsgID: msgID, AfterMsgID: min}}}
				raw, err := json.Marshal(request)
				require.NoError(t, err)
				status, data, err := r.eligibilityAnswerCommand([]string{string(raw)})
				if err != nil {
					return ribevents.EligibilityAnswer{}, err
				}
				require.Equal(t, statusDone, status)
				answer, ok := data.(ribevents.EligibilityAnswer)
				require.True(t, ok)
				return answer, nil
			}
			b := []byte{24, 10, 77, 0}
			c := []byte{24, 10, 78, 0}
			deliver(address, 7, 9, 10, nil, b)
			answer, err := query("10.77.0.0/24", 10, 11)
			require.NoError(t, err)
			require.True(t, answer.Pending)
			require.Nil(t, answer.Answers)
			other := netip.MustParseAddr("192.0.2.189")
			require.True(t, r.recordPeerState(other, true, 8, 10))
			deliver(other, 8, 10, 1000, nil, c)
			require.True(t, r.recordPeerState(address, true, 7, 9))
			answer, err = query("10.77.0.0/24", 10, 11)
			require.NoError(t, err)
			require.True(t, answer.Pending, "unrelated source and duplicate UP do not prove catch-up")
			deliver(address, 7, 9, 11, b, nil)
			answer, err = query("10.77.0.0/24", 10, 11)
			require.NoError(t, err)
			require.False(t, answer.Pending)
			require.Equal(t, []bool{false}, answer.Answers, "removed B cannot be cold-adopted")
			deliver(address, 7, 9, 12, nil, c)
			answer, err = query("10.78.0.0/24", 12, 12)
			require.NoError(t, err)
			require.Equal(t, []bool{true}, answer.Answers, "independent useful C is eligible after its actual ingest")
			deliver(address, 7, 9, 13, nil, nil)
			answer, err = query("10.78.0.0/24", 12, 13)
			require.NoError(t, err)
			require.False(t, answer.Pending)
			require.Equal(t, []bool{true}, answer.Answers, "empty UPDATE commits progress")
			require.True(t, r.recordPeerState(address, true, 7, 19))
			_, err = query("10.78.0.0/24", 12, 13)
			require.Error(t, err, "successor cannot attest old physical progress")
			current, err := r.eligibilityProgress(ribevents.ValidationRoute{Peer: address, PeerOwner: 7, ConnectionID: 19}, 1)
			require.NoError(t, err)
			require.False(t, current, "new physical connection begins at zero")
		})
	}
}
