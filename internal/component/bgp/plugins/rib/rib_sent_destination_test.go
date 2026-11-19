// Design: docs/architecture/plugin/rib-storage-design.md -- exact destination retirement.
// Related: rib_sent_destination.go -- terminal history and undo-pin ownership.
package rib

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The actual state and SENT intake retain two distinct configured owners at
// one address. Physical DOWN keeps replay useful; permanent removal must
// release the last real pool reference and leave no per-owner history across
// repeated recreation. The live same-address sibling remains replayable.
func TestRIBPermanentDestinationRemovalReleasesSentHistory(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(testReceiptRailName(jsonRail, false), func(t *testing.T) {
			r := newTestRIBManager(t)
			destinationRetirementCleanup(t, r)
			sibling := sourceIdentityPeer{owner: 9001, name: "surviving-destination", session: 19001}
			sibling.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
			survivor := destinationRetirementSent(t, r, jsonRail, sibling, 29001, nil)
			require.True(t, survivor.LocalOrigin)
			require.Zero(t, survivor.SourceOwner)
			for cycle := range uint64(32) {
				current := sourceIdentityPeer{owner: 10001 + cycle, name: "recreated-destination", session: 20001 + cycle*2}
				current.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
				entry := destinationRetirementSent(t, r, jsonRail, current, 30001+cycle, nil)
				require.Len(t, r.ribOut, 2, "only the current destination and its live sibling own SENT history")
				require.True(t, entry.LocalOrigin)
				require.Zero(t, entry.SourceOwner)
				current.state(t, r, jsonRail, rpc.SessionStateDown, rpc.ReasonTCPFailure)
				_, err := pool.RibOut.Get(entry.AttrHandle)
				require.NoError(t, err, "physical DOWN retains reconnect bytes")
				r.peerMu.RLock()
				replay := r.collectGroupedRibOutRoutes(current.owner)
				r.peerMu.RUnlock()
				require.Len(t, replay, 1, "retention must remain useful to the actual replay collector")
				current.session++
				current.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
				current.state(t, r, jsonRail, rpc.SessionStateDown, rpc.ReasonPeerRemoved)
				assert.NotContains(t, r.ribOut, current.owner, "the permanently removed owner cannot retain history")
				_, err = pool.RibOut.Get(entry.AttrHandle)
				assert.Error(t, err, "the actual removed destination's last pool reference must be released")
				require.Len(t, r.ribOut, 1, "recreation must not accumulate old configured owners")
				require.Len(t, r.peerMeta, 1)
				require.Len(t, r.peerUp, 1)
				require.Len(t, r.peerNames, 1)
				_, err = pool.RibOut.Get(survivor.AttrHandle)
				require.NoError(t, err, "same-address removal must not release the sibling's live bytes")
				r.peerMu.RLock()
				useful := r.collectGroupedRibOutRoutes(sibling.owner)
				r.peerMu.RUnlock()
				require.Len(t, useful, 1)
				require.True(t, r.peerUp[sibling.owner])
			}
		})
	}
}

// A real source lease and policy apply move both destination copies into undo
// pins. Permanently removing one destination releases its pin immediately.
// Abort must restore the healthy same-address destination usefully under the
// fresh epoch, never send to an invalid address or recreate the retired owner.
func TestRIBPermanentDestinationRemovalRetiresSourcePolicyPins(t *testing.T) {
	for _, native := range []bool{false, true} {
		for _, jsonRail := range []bool{false, true} {
			transport := "framed/"
			if native {
				transport = "native/"
			}
			t.Run(transport+testReceiptRailName(jsonRail, false), func(t *testing.T) {
				r, calls := sourceLeaseTestRIB(t, native)
				destinationRetirementCleanup(t, r)
				source, _ := sourceIdentityPeers()
				source.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
				source.receive(t, r, jsonRail, []byte{24, 10, 77, 1}, 100)
				require.Equal(t, 1, calls.count(rpc.SourceLeaseAcquire, source.receipt))
				removed := sourceIdentityPeer{owner: 9002, name: "removed-destination", session: 19002}
				healthy := sourceIdentityPeer{owner: 9003, name: "healthy-destination", session: 19003}
				removed.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
				healthy.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
				retiredCopy := destinationRetirementSent(t, r, jsonRail, removed, 29002, &source)
				healthyCopy := destinationRetirementSent(t, r, jsonRail, healthy, 29003, &source)
				var commands []rpc.UpdateRouteInput
				r.updateRefuseHook = func(input rpc.UpdateRouteInput) error {
					commands = append(commands, input)
					address, err := netip.ParseAddr(input.PeerSelector)
					if err != nil {
						return err
					}
					if address != netip.MustParseAddr(sourceIdentityAddress) {
						return errors.New("destination restoration selected another address")
					}
					return nil
				}
				r.updateHook = func(string, map[string]any) {}
				input := rpc.SourcePolicyInput{TransactionID: "permanent-destination-removal",
					Source: rpc.SourcePolicyIdentity{Peer: sourceIdentityAddress, PeerOwner: source.owner,
						SessionGeneration: source.generation, PolicyEpoch: 5, ReceiveFrontier: source.message, TemplateGeneration: 1},
					DeadlineMS: time.Now().Add(time.Minute).UnixMilli()}
				input.Operation = rpc.SourcePolicyPrepare
				require.NoError(t, input.Validate(time.Now()))
				require.NoError(t, r.handleSourcePolicy(t.Context(), input))
				input.Operation, input.NextPolicyEpoch = rpc.SourcePolicyApply, 6
				require.NoError(t, input.Validate(time.Now()))
				require.NoError(t, r.handleSourcePolicy(t.Context(), input))
				txn := r.sourcePolicy[source.owner]
				require.NotNil(t, txn)
				require.Len(t, txn.sent, 2, "both withdrawn copies must actually be pinned before removal")
				require.Len(t, commands, 2, "apply must usefully withdraw both advertised copies")
				_, err := pool.RibOut.Get(retiredCopy.AttrHandle)
				require.NoError(t, err, "the removed copy starts with a real live undo reference")
				removed.state(t, r, jsonRail, rpc.SessionStateDown, rpc.ReasonPeerRemoved)
				assert.Len(t, txn.sent, 1, "only the surviving destination remains undoable")
				_, err = pool.RibOut.Get(retiredCopy.AttrHandle)
				assert.Error(t, err, "permanent removal releases the undo pin immediately, before abort")
				_, err = pool.RibOut.Get(healthyCopy.AttrHandle)
				require.NoError(t, err)
				commands = nil
				input.Operation, input.NextPolicyEpoch = rpc.SourcePolicyAbort, 7
				require.NoError(t, input.Validate(time.Now()))
				require.NoError(t, r.handleSourcePolicy(t.Context(), input), "a removed destination must not manufacture a restore refusal")
				require.Len(t, commands, 1, "the healthy destination still receives its useful restoration")
				require.Equal(t, sourceIdentityAddress, commands[0].PeerSelector)
				require.Equal(t, strconv.FormatUint(healthy.owner, 10), commands[0].Meta["destination-owner"])
				require.Equal(t, strconv.FormatUint(healthyCopy.MsgID, 10), commands[0].Meta[bgptypes.SentOwnerMessageMeta])
				require.Equal(t, uint64(7), commands[0].SourcePolicyEpoch)
				require.Contains(t, commands[0].Command, "nlri ipv4/unicast add 180a4d01")
				require.NotContains(t, r.ribOut, removed.owner)
				require.NotContains(t, r.peerMeta, removed.owner)
				require.Len(t, r.ribOut, 1)
				restored := r.ribOut[healthy.owner][family.IPv4Unicast][ribOutKey{Prefix: netip.MustParsePrefix("10.77.1.0/24")}]
				require.Equal(t, healthyCopy.AttrHandle, restored.AttrHandle)
				require.Equal(t, uint64(7), restored.SourcePolicyEpoch)
				require.Equal(t, source.owner, restored.SourceOwner)
				require.Empty(t, txn.sent)
				require.Zero(t, calls.count(rpc.SourceLeaseRelease, source.receipt), "destination retirement must not revoke the useful received source")
			})
		}
	}
}

// destinationRetirementSent drives the actual raw-JSON or structured SENT
// consumer. Its distinct large-community value keeps pool-lifetime checks
// independent of unrelated tests' attribute blobs. The returned entry borrows
// inventory ownership; only the production retirement or test cleanup releases.
func destinationRetirementSent(t *testing.T, r *RIBManager, jsonRail bool, destination sourceIdentityPeer, message uint64, source *sourceIdentityPeer) ribOutEntry {
	t.Helper()
	var marker [12]byte
	binary.BigEndian.PutUint32(marker[0:4], 4200000123)
	binary.BigEndian.PutUint64(marker[4:12], message)
	attrs := appendAttr(unicastAttrs([4]byte{10, 0, 0, 1}, 30, 100), byte(attribute.AttrLargeCommunity), 0xc0, marker[:])
	raw := []byte{24, 10, 77, 1}
	var sourceOwner, sourceMessage uint64
	sourceAddress := ""
	if source != nil {
		sourceOwner, sourceMessage, sourceAddress = source.owner, source.message, sourceIdentityAddress
	}
	if jsonRail {
		event := &Event{PeerOwner: destination.owner,
			Message:       &MessageInfo{Type: rpc.EventKindUpdate, ID: message},
			Peer:          mustMarshal(t, PeerInfoJSON{Remote: PeerRemoteInfo{Address: sourceIdentityAddress, AS: 65001}}),
			RawAttributes: hex.EncodeToString(attrs), RawNLRI: map[family.Family]string{family.IPv4Unicast: hex.EncodeToString(raw)},
			FamilyOps:         map[family.Family][]FamilyOperation{family.IPv4Unicast: {{Action: routeaction.Add, NextHop: "10.0.0.1"}}},
			SourcePolicyEpoch: 5, RouteMeta: map[string]any{"source-peer": sourceAddress,
				"source-message-id": float64(sourceMessage), bgptypes.SourceOwnerMeta: strconv.FormatUint(sourceOwner, 10),
				bgptypes.SourceLocalMeta: source == nil}}
		r.handleSent(event)
	} else {
		encoding, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
		require.NoError(t, err)
		body := append([]byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}, attrs...)
		body = append(body, raw...)
		update := wireu.NewWireUpdate(body, encoding)
		attributes, err := update.Attrs()
		require.NoError(t, err)
		r.handleSentStructured(&rpc.StructuredEvent{PeerOwner: destination.owner, PeerAddress: sourceIdentityAddress,
			MessageID: message, SourcePeerStr: sourceAddress,
			RawMessage: &bgptypes.RawMessage{Type: msgtype.TypeUPDATE, MessageID: message, WireUpdate: update, AttrsWire: attributes,
				SourceMessageID: sourceMessage, SourcePolicyEpoch: 5, SourceOwner: sourceOwner, SourceLocal: source == nil}})
	}
	r.peerMu.RLock()
	entry, found := r.ribOut[destination.owner][family.IPv4Unicast][ribOutKey{Prefix: netip.MustParsePrefix("10.77.1.0/24")}]
	r.peerMu.RUnlock()
	require.True(t, found, "the real SENT consumer must first own a useful route")
	require.True(t, entry.AttrHandle.IsValid())
	require.Equal(t, message, entry.MsgID)
	return entry
}

// destinationRetirementCleanup releases only the references these tests still
// own, including on the intended baseline failure. Removed entries are absent
// from these maps; inspecting their dead handle before cleanup is the oracle.
func destinationRetirementCleanup(t *testing.T, r *RIBManager) {
	t.Helper()
	t.Cleanup(func() {
		r.peerMu.Lock()
		defer r.peerMu.Unlock()
		for _, families := range r.ribOut {
			for _, routes := range families {
				for _, entry := range routes {
					entry.release()
				}
			}
		}
		clear(r.ribOut)
		for _, transaction := range r.sourcePolicy {
			transaction.releaseSent()
		}
		clear(r.sourcePolicy)
	})
}
