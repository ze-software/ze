// Design: docs/architecture/plugin/rib-storage-design.md -- Route-server current retained membership.
// Related: server_withdrawal.go -- processForward installs current membership after live forwarding.
package rs

import (
	"encoding/hex"
	"fmt"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: both worker inputs install the UPDATE's revision and NO_LLGR on
// new and replaced rows (both false->true and true->false), keep sibling rows
// untouched on announcements and withdrawals, and maintain per-source counts.
// PREVENTS: a later UPDATE from claiming an unstamped older route because its
// message ID is zero, leaving stale NO_LLGR set on replacement, or conflating
// metadata across routes of one source.
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
			send := func(msgID uint64, prefixOctet byte, noLLGR, withdraw bool) {
				item := workItem{msgID: msgID, sourcePeer: source.address.String(), sourceOwner: source.owner,
					sourceReceipt: receipt, sourcePolicyEpoch: epoch}
				if native {
					var body []byte
					if withdraw {
						body = []byte{0, 4, 24, 203, 0, prefixOctet, 0, 0}
					} else {
						attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 1}
						if noLLGR {
							attrs = append(attrs, 0xc0, 8, 4, 0xff, 0xff, 0, 7)
						}
						body = append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
						body = append(body, 24, 203, 0, prefixOctet)
					}
					wire := wireu.NewWireUpdate(body, ctxID)
					attrsWire, err := wire.Attrs()
					require.NoError(t, err)
					item.msg = &bgptypes.RawMessage{WireUpdate: wire, AttrsWire: attrsWire, MessageID: msgID,
						SourceOwner: source.owner, SourceConnectionID: source.connectionID, SourceReceipt: receipt, SourcePolicyEpoch: epoch}
				} else {
					if withdraw {
						item.textPayload = fmt.Sprintf("peer 192.0.2.1 remote as 65001 received update %d nlri ipv4/unicast del prefix 203.0.%d.0/24", msgID, prefixOctet)
					} else {
						community := ""
						if noLLGR {
							community = "community no-llgr "
						}
						item.textPayload = fmt.Sprintf("peer 192.0.2.1 remote as 65001 received update %d origin igp next-hop 192.0.2.1 %snlri ipv4/unicast add prefix 203.0.%d.0/24", msgID, community, prefixOctet)
					}
				}
				rs.processForward(source, item)
			}
			send(0, 113, true, false)
			send(301, 114, false, false)
			send(302, 115, true, false)
			send(303, 114, true, false)  // false -> true
			send(304, 115, false, false) // true -> false
			send(305, 113, false, true)  // withdraw leaves siblings untouched

			rows := rs.withdrawals[source.address.String()]
			require.Len(t, rows, 2)
			require.Equal(t, 2, rs.addressTracked[source.address.String()])
			for _, want := range []struct {
				prefix string
				msgID  uint64
				noLLGR bool
			}{
				{"203.0.114.0/24", 303, true},
				{"203.0.115.0/24", 304, false},
			} {
				key := withdrawalKey{owner: source.owner, fam: family.IPv4Unicast, prefix: netip.MustParsePrefix(want.prefix)}
				require.Equal(t, withdrawalEntry{owner: source.owner, connectionID: source.connectionID, sourceReceipt: receipt,
					sourcePolicyEpoch: epoch, msgID: want.msgID, noLLGR: want.noLLGR}, rows[key], want.prefix)
			}
		})
	}
}

// VALIDATES: text inventory updates apply RFC 4271 Section 4.3 withdraw-before-announce
// ordering regardless of token order, and textNoLLGR recognizes all NO_LLGR spellings
// without matching non-community attribute values.
// PREVENTS: a text UPDATE carrying add before del for the same prefix from erasing
// the surviving announcement, or numeric NO_LLGR spellings from being missed.
func TestProcessForwardTextWithdrawBeforeAnnounceAndNoLLGR(t *testing.T) {
	rs := newTestRouteServer(t)
	source := rsFixtureKey("192.0.2.2", 12, 102)
	const receipt, epoch = 202, 8
	rs.peers[source] = &PeerState{Up: true, RSClient: true}

	item := workItem{
		msgID:             401,
		sourcePeer:        source.address.String(),
		sourceOwner:       source.owner,
		sourceReceipt:     receipt,
		sourcePolicyEpoch: epoch,
		textPayload:       "peer 192.0.2.2 remote as 65002 received update 401 origin igp next-hop 192.0.2.2 community 0xffff0007 nlri ipv4/unicast add prefix 198.51.100.0/24 nlri ipv4/unicast del prefix 198.51.100.0/24",
	}
	rs.processForward(source, item)

	key := withdrawalKey{owner: source.owner, fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("198.51.100.0/24")}
	entry, present := rs.withdrawals[source.address.String()][key]
	require.True(t, present, "RFC 4271 Section 4.3 requires add to win over del in the same UPDATE")
	require.Equal(t, uint64(401), entry.msgID)
	require.True(t, entry.noLLGR)

	require.True(t, textNoLLGR("peer 192.0.2.2 remote as 65002 received update 402 origin igp community [65535:1,4294901767] nlri ipv4/unicast add prefix 198.51.100.0/24"))
	require.False(t, textNoLLGR("peer 192.0.2.2 remote as 65002 received update 403 origin igp large-community 65535:7:1 nlri ipv4/unicast add prefix 198.51.100.0/24"))
}

// VALIDATES: extractWireNLRIRecords populates msgID and noLLGR directly from RawMessage
// alongside the rest of source provenance, including ADD-PATH rows.
// PREVENTS: callers of extractWireNLRIRecords + applyNLRIRecords from silently
// installing zero msgID or false noLLGR.
func TestExtractWireNLRIRecordsPopulatesFullProvenance(t *testing.T) {
	rs := newTestRouteServer(t)
	source := rsFixtureKey("192.0.2.3", 13, 103)
	enc := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
	ctxID, err := bgpctx.Registry.Register(enc)
	require.NoError(t, err)

	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 3, 0xc0, 8, 4, 0xff, 0xff, 0, 7}
	body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
	body = append(body, 0, 0, 0, 9, 24, 203, 0, 120)
	wire := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wire.Attrs()
	require.NoError(t, err)

	msg := &bgptypes.RawMessage{
		WireUpdate:         wire,
		AttrsWire:          attrsWire,
		MessageID:          505,
		SourceOwner:        source.owner,
		SourceConnectionID: source.connectionID,
		SourceReceipt:      203,
		SourcePolicyEpoch:  9,
	}
	records, err := extractWireNLRIRecords(msg)
	require.NoError(t, err)
	defer returnNLRIRecords(records)

	rs.withdrawalMu.Lock()
	rs.applyNLRIRecords(source.address.String(), records)
	rs.withdrawalMu.Unlock()

	key := withdrawalKey{owner: source.owner, fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("203.0.120.0/24"), pathID: 9, addPath: true}
	entry, present := rs.withdrawals[source.address.String()][key]
	require.True(t, present)
	require.Equal(t, uint64(505), entry.msgID)
	require.True(t, entry.noLLGR)
	require.Equal(t, uint64(203), entry.sourceReceipt)
	require.Equal(t, uint64(9), entry.sourcePolicyEpoch)
}

// VALIDATES: currentStoredRouteLocked resolves PrefixOnly and SourceWire routes
// via O(1) direct map probes across plain unicast, ADD-PATH unicast, and adopted
// entries, and hasWithdrawalForSourceLocked tracks per-source counts accurately
// when sibling owners share an address bucket.
// PREVENTS: quadratic withdrawal-map scans on peer-up replay or worker drain.
func TestCurrentStoredRouteAndSourceCountDirectLookup(t *testing.T) {
	rs := newTestRouteServer(t)
	source := rsFixtureKey("192.0.2.4", 14, 104)
	sibling := rsFixtureKey("192.0.2.4", 15, 105)
	rs.peers[source] = &PeerState{Up: true, RSClient: true, sourceLease: true, sourceReceipt: 301, adoptionOpen: true}
	rs.peers[sibling] = &PeerState{Up: true, RSClient: true, sourceLease: true, sourceReceipt: 302}

	rs.withdrawalMu.Lock()
	defer rs.withdrawalMu.Unlock()

	// Seed 500 sibling routes in the same address bucket.
	for i := range 500 {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i >> 8), byte(i), 0}), 24)
		key := withdrawalKey{owner: sibling.owner, fam: family.IPv4Unicast, prefix: prefix}
		rs.setWithdrawalLocked(sibling.address.String(), key, withdrawalEntry{
			owner: sibling.owner, connectionID: sibling.connectionID, sourceReceipt: 302, sourcePolicyEpoch: 1, msgID: uint64(i + 1),
		})
	}
	require.False(t, rs.hasWithdrawalForSourceLocked(source))
	require.True(t, rs.hasWithdrawalForSourceLocked(sibling))

	// Plain unicast route for source.
	plainPrefix := netip.MustParsePrefix("203.0.130.0/24")
	plainKey := withdrawalKey{owner: source.owner, fam: family.IPv4Unicast, prefix: plainPrefix}
	rs.setWithdrawalLocked(source.address.String(), plainKey, withdrawalEntry{
		owner: source.owner, connectionID: source.connectionID, sourceReceipt: 301, sourcePolicyEpoch: 5, msgID: 601,
	})

	// Unicast ADD-PATH route (native.addPath=true, entry.addPath=false, as recordKey/recordEntry produce).
	addPathPrefix := netip.MustParsePrefix("203.0.131.0/24")
	addPathKey := withdrawalKey{owner: source.owner, fam: family.IPv4Unicast, prefix: addPathPrefix, pathID: 42, addPath: true}
	rs.setWithdrawalLocked(source.address.String(), addPathKey, withdrawalEntry{
		owner: source.owner, connectionID: source.connectionID, sourceReceipt: 301, sourcePolicyEpoch: 5, msgID: 602,
	})

	// Adopted PrefixOnly route (native.addPath=false, entry.addPath=true, as adoptStoredRouteLocked produces).
	adoptedRoute := rpc.StoredRoute{
		SourcePeer:         source.address.String(),
		SourceOwner:        source.owner,
		SourceConnectionID: source.connectionID,
		SourceReceipt:      301,
		SourcePolicyEpoch:  5,
		MsgID:              603,
		Family:             family.IPv4Unicast.String(),
		PathID:             7,
		NLRIHex:            hex.EncodeToString([]byte{24, 203, 0, 132}),
		NLRIFraming:        rpc.NLRIFramingPrefixOnly,
	}
	require.True(t, rs.adoptStoredRouteLocked(source, adoptedRoute))

	require.True(t, rs.hasWithdrawalForSourceLocked(source))
	require.Equal(t, 503, rs.addressTracked[source.address.String()])

	plainStored := rpc.StoredRoute{
		SourcePeer: source.address.String(), SourceOwner: source.owner, SourceConnectionID: source.connectionID,
		SourceReceipt: 301, SourcePolicyEpoch: 5, MsgID: 601,
		Family: family.IPv4Unicast.String(), NLRIHex: hex.EncodeToString([]byte{24, 203, 0, 130}),
		NLRIFraming: rpc.NLRIFramingSourceWire,
	}
	require.True(t, rs.currentStoredRouteLocked(plainStored))

	plainWrongMsg := plainStored
	plainWrongMsg.MsgID = 999
	require.False(t, rs.currentStoredRouteLocked(plainWrongMsg))

	addPathStored := rpc.StoredRoute{
		SourcePeer: source.address.String(), SourceOwner: source.owner, SourceConnectionID: source.connectionID,
		SourceReceipt: 301, SourcePolicyEpoch: 5, MsgID: 602,
		Family: family.IPv4Unicast.String(), PathID: 42, NLRIHex: hex.EncodeToString([]byte{24, 203, 0, 131}),
		NLRIFraming: rpc.NLRIFramingPrefixOnly,
	}
	require.True(t, rs.currentStoredRouteLocked(addPathStored))

	// SourceWire with 4-byte path ID matching the adopted entry (entry.addPath=true).
	adoptedSourceWire := adoptedRoute
	adoptedSourceWire.NLRIFraming = rpc.NLRIFramingSourceWire
	adoptedSourceWire.NLRIHex = hex.EncodeToString([]byte{0, 0, 0, 7, 24, 203, 0, 132})
	require.True(t, rs.currentStoredRouteLocked(adoptedRoute))
	require.True(t, rs.currentStoredRouteLocked(adoptedSourceWire))

	// Removing source's 3 entries clears source's count while sibling's 500 remain tracked.
	rs.deleteWithdrawalLocked(source.address.String(), plainKey)
	rs.deleteWithdrawalLocked(source.address.String(), addPathKey)
	adoptedKey, err := storedKey(adoptedRoute, false)
	require.NoError(t, err)
	rs.deleteWithdrawalLocked(source.address.String(), adoptedKey)
	require.False(t, rs.hasWithdrawalForSourceLocked(source))
	require.True(t, rs.hasWithdrawalForSourceLocked(sibling))
	require.Equal(t, 500, rs.addressTracked[source.address.String()])
}
