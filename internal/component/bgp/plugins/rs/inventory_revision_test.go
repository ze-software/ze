// Design: docs/architecture/plugin/rib-storage-design.md -- Route-server current retained membership.
// Related: server_withdrawal.go -- processForward installs current membership after live forwarding.
package rs

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// VALIDATES: both worker inputs install the UPDATE's revision and NO_LLGR on
// new and replaced rows, and preserve an unrelated row's original metadata.
// PREVENTS: a later UPDATE from claiming an unstamped older route because its
// message ID is zero, and conflating NO_LLGR across routes of one source.
func TestProcessForwardStampsOnlyAnnouncedInventoryRows(t *testing.T) {
	for _, native := range []bool{true, false} {
		name := "text"
		if native {
			name = "native"
		}
		t.Run(name, func(t *testing.T) {
			rs := newTestRouteServer(t)
			source := rsFixtureKey("192.0.2.1", 11, 101)
			const receipt, epoch = 201, 7
			rs.peers[source] = &PeerState{Up: true, RSClient: true}
			ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
			require.NoError(t, err)
			send := func(msgID uint64, prefixOctet byte, noLLGR bool) {
				item := workItem{msgID: msgID, sourcePeer: source.address.String(), sourceOwner: source.owner,
					sourceReceipt: receipt, sourcePolicyEpoch: epoch}
				if native {
					attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 1}
					if noLLGR {
						attrs = append(attrs, 0xc0, 8, 4, 0xff, 0xff, 0, 7)
					}
					body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
					body = append(body, 24, 203, 0, prefixOctet)
					wire := wireu.NewWireUpdate(body, ctxID)
					attrsWire, err := wire.Attrs()
					require.NoError(t, err)
					item.msg = &bgptypes.RawMessage{WireUpdate: wire, AttrsWire: attrsWire, MessageID: msgID,
						SourceOwner: source.owner, SourceConnectionID: source.connectionID, SourceReceipt: receipt, SourcePolicyEpoch: epoch}
				} else {
					community := ""
					if noLLGR {
						community = "community no-llgr "
					}
					item.textPayload = fmt.Sprintf("peer 192.0.2.1 remote as 65001 received update %d origin igp next-hop 192.0.2.1 %snlri ipv4/unicast add prefix 203.0.%d.0/24", msgID, community, prefixOctet)
				}
				rs.processForward(source, item)
			}
			send(0, 113, true)
			send(301, 114, false)
			send(302, 115, true)
			send(303, 114, true)

			rows := rs.withdrawals[source.address.String()]
			require.Len(t, rows, 3)
			for _, want := range []struct {
				prefix string
				msgID  uint64
				noLLGR bool
			}{
				{"203.0.113.0/24", 0, true},
				{"203.0.114.0/24", 303, true},
				{"203.0.115.0/24", 302, true},
			} {
				key := withdrawalKey{owner: source.owner, fam: family.IPv4Unicast, prefix: netip.MustParsePrefix(want.prefix)}
				require.Equal(t, withdrawalEntry{owner: source.owner, connectionID: source.connectionID, sourceReceipt: receipt,
					sourcePolicyEpoch: epoch, msgID: want.msgID, noLLGR: want.noLLGR}, rows[key], want.prefix)
			}
		})
	}
}
