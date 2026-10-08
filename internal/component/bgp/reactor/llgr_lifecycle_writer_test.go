// Design: docs/architecture/plugin/rib-storage-design.md -- received lifecycle and sent projection.
// Related: peer_ready_producer_test.go -- running RIB SDK and real command dispatcher.
package reactor

import (
	"encoding/json"
	"maps"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestLLGREORWriterKeepsRefreshedRoute separates received refresh, destination
// write, and asynchronous sent projection. The EOR command is the exact command
// dispatched by GR.handleEOREvent; the running RIB must withdraw only the stale
// path, never the refreshed one. Another source's newer writer receipt remains
// a separate control: an obsolete projection must not authorize its removal.
//
// Production onMessageBatchReceived enqueues independent subscribers without
// joining their handlers. Therefore the RIB can apply a fresh received UPDATE
// before RS has submitted its forward. No socket timing or sleeps choose these
// schedules; every sent event below is a snapshot of the actual final callback.
func TestLLGREORWriterKeepsRefreshedRoute(t *testing.T) {
	for _, schedule := range []string{"before-forward", "before-projection", "after-projection", "other-source", "shared-generation"} {
		t.Run(schedule, func(t *testing.T) {
			const owner = "bgp-rib"
			f := ownershipRailNew(t, nil)
			shared := schedule == "shared-generation"
			beforeForward := schedule == "before-forward" || shared
			if shared {
				ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
				ctxID, err := bgpctx.Registry.Register(ctx)
				require.NoError(t, err)
				f.ctxID = ctxID
				f.destination.sendCtx.Store(ctx)
				f.destination.sendCtxID = ctxID
				f.destination.session.setSendCtxID(ctxID)
				f.destination.refreshForwardFacts()
			}
			f.destination.settings.ProcessBindings = []ProcessBinding{sendUpdateOnly(owner)}
			api, proc := readyProducerServer(t, f.r, owner)
			f.destination.resetAPISync([]string{owner})
			raiseTestReplayFence(f.destination, owner)
			token := f.destination.initialReplayToken()
			readyProducerState(t, proc, f.destination, rpc.SessionStateUp, token)
			readyProducerAwait(t, api, owner, token, false)

			// The callback returns immediately, like asynchronous sent delivery.
			// Snapshot owns every borrowed byte before the writer reuses its body.
			type sentResult struct {
				message bgptypes.RawMessage
				err     error
			}
			sent := make(chan sentResult, 16)
			f.r.messageReceiver = &testMessageReceiver{onSent: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
				msg.WireUpdate = msg.WireUpdate.Snapshot()
				msg.RawBytes = msg.WireUpdate.Payload()
				msg.Meta = maps.Clone(msg.Meta)
				msg.SentPathSources = slices.Clone(msg.SentPathSources)
				attrs, err := msg.WireUpdate.Attrs()
				msg.AttrsWire = attrs
				sent <- sentResult{message: msg, err: err}
			}}
			f.destination.session.SetMessageCallback(f.r.notifyMessageReceiver)
			deliverSent := func(msg bgptypes.RawMessage) {
				require.NoError(t, proc.Bridge().DeliverStructured([]any{&rpc.StructuredEvent{
					PeerAddress: f.destination.Settings().Address.String(), EventType: rpc.EventKindUpdate,
					Direction: rpc.DirectionSent, MessageID: msg.MessageID, RawMessage: &msg,
					SourcePeerStr: msg.SourcePeerStr, SourceID: msg.SourceID, Meta: msg.Meta,
				}}))
			}
			nextSent := func() bgptypes.RawMessage {
				select {
				case result := <-sent:
					require.NoError(t, result.err)
					return result.message
				case <-time.After(5 * time.Second):
					t.Fatal("actual final writer did not publish the expected sent event")
					return bgptypes.RawMessage{}
				}
			}
			drainSent := func() {
				forwardSocketBarrier(t, f.r)
				f.flush(t)
				for {
					select {
					case result := <-sent:
						require.NoError(t, result.err)
						deliverSent(result.message)
					default:
						return
					}
				}
			}
			receive := func(source *Peer, raw []byte, fingerprint byte) *ReceivedUpdate {
				// RFC 4271 Section 4.3: each input carries native NLRIs and MED.
				update := f.publish(t, source, f.ctxID, ownershipRailBody(family.IPv4Unicast, false, raw, fingerprint))
				attrs, err := update.WireUpdate.Attrs()
				require.NoError(t, err)
				require.NoError(t, proc.Bridge().DeliverStructured([]any{&rpc.StructuredEvent{
					PeerAddress: source.Settings().Address.String(), EventType: rpc.EventKindUpdate,
					Direction: rpc.DirectionReceived, MessageID: update.WireUpdate.MessageID(),
					RawMessage: &bgptypes.RawMessage{WireUpdate: update.WireUpdate, AttrsWire: attrs, MessageID: update.WireUpdate.MessageID()},
				}}))
				return update
			}
			forward := func(update *ReceivedUpdate) bgptypes.RawMessage {
				require.NoError(t, api.ForwardUpdatesDirect([]uint64{update.WireUpdate.MessageID()},
					[]netip.AddrPort{f.destination.Settings().PeerKey()}, owner, plugin.OperatorSender()))
				forwardSocketBarrier(t, f.r)
				return nextSent()
			}

			freshRaw := []byte{24, 10, 0, 0}
			staleRaw := []byte{24, 10, 0, 2}
			var oldFresh, oldStale bgptypes.RawMessage
			if shared {
				// RFC 7911 Section 3: both received path IDs belong to ONE
				// UPDATE generation; refreshing zero must not preserve stale 17.
				freshRaw = pathsLimitNLRI(0, "10.0.0.0/24")
				staleRaw = pathsLimitNLRI(17, "10.0.0.0/24")
				oldFresh = forward(receive(f.source, append(slices.Clone(freshRaw), staleRaw...), 1))
				oldStale = oldFresh
				deliverSent(oldFresh)
			} else {
				oldFresh = forward(receive(f.source, freshRaw, 1))
				deliverSent(oldFresh)
				oldStale = forward(receive(f.source, staleRaw, 2))
				deliverSent(oldStale)
			}
			initial := ownershipRailRead(t, f.conn.written(), f.destination.sendCtx.Load())
			require.Len(t, initial, 2)
			freshPath, stalePath := initial[0].path, initial[1].path
			require.Equal(t, oldFresh.MessageID, llgrWriterReceipt(t, f, freshRaw, freshPath, f.source))
			require.Equal(t, oldStale.MessageID, llgrWriterReceipt(t, f, staleRaw, stalePath, f.source))

			source := f.source.Settings().Address.String()
			llgrWriterCommand(t, proc, "request bgp rib mark-stale", source, "0", "2", "ipv4/unicast")
			llgrWriterCommand(t, proc, "request bgp rib attach-community", source, "ipv4/unicast", "ffff0006")
			llgrWriterCommand(t, proc, "clear bgp rib out", f.destination.Settings().Address.String(), "ipv4/unicast")
			drainSent()
			require.Len(t, ownershipRailRead(t, f.conn.written(), f.destination.sendCtx.Load()), 4)
			// LLGR replay must preserve, not manufacture, logical writer receipts.
			require.Equal(t, oldFresh.MessageID, llgrWriterReceipt(t, f, freshRaw, freshPath, f.source))
			require.Equal(t, oldStale.MessageID, llgrWriterReceipt(t, f, staleRaw, stalePath, f.source))
			baseline := len(f.conn.written())

			// A retained source reconnects without replacing its Peer incarnation.
			f.source.forwardGeneration.Add(1)
			fresh := receive(f.source, freshRaw, 3)
			var pending []bgptypes.RawMessage
			var freshSent bgptypes.RawMessage
			if !beforeForward {
				freshSent = forward(fresh)
				if schedule == "before-projection" {
					pending = append(pending, freshSent)
				} else {
					deliverSent(freshSent)
				}
			}
			var replacement bgptypes.RawMessage
			if schedule == "other-source" {
				replacement = forward(receive(f.other, staleRaw, 4))
				pending = append(pending, replacement)
			}
			before := llgrWriterReceipt(t, f, freshRaw, freshPath, f.source)
			t.Logf("fresh received=%d; old sent projection=%d; actual fresh writer=%d; stale sent=%d",
				fresh.WireUpdate.MessageID(), oldFresh.MessageID, before, oldStale.MessageID)

			// This is GR.handleEOREvent's real registered command, not a mocked
			// lifecycle callback. Fresh received state is already applied, while
			// forward/projection progress is selected independently above.
			data := llgrWriterCommand(t, proc, "request bgp rib purge-stale", source, "ipv4/unicast")
			var result struct {
				Purged int `json:"purged"`
			}
			require.NoError(t, json.Unmarshal(data, &result))
			require.Equal(t, 1, result.Purged)
			drainSent()
			rows := llgrWriterRoutes(t, proc, "received", source)
			require.Len(t, rows, 1)
			expectedPrefix := "10.0.0.0/24"
			if shared {
				expectedPrefix += " [pathID=0]"
			}
			require.Equal(t, expectedPrefix, rows[0].Prefix)
			require.Zero(t, rows[0].StaleLevel)

			// Release delayed work only AFTER the EOR action has reached its
			// actual writer. A later reannouncement cannot excuse a transient
			// forbidden withdrawal of the refreshed prefix.
			if beforeForward {
				freshSent = forward(fresh)
				deliverSent(freshSent)
			}
			for _, msg := range pending {
				deliverSent(msg)
			}
			drainSent()
			wire := ownershipRailRead(t, f.conn.written()[baseline:], f.destination.sendCtx.Load())
			freshAnnounces, freshWithdraws, staleWithdraws := 0, 0, 0
			for _, event := range wire {
				if event.key == initial[0].key && event.path == freshPath {
					if event.withdraw {
						freshWithdraws++
					} else {
						require.Equal(t, byte(3), event.origin)
						freshAnnounces++
					}
				}
				if event.key == initial[1].key && event.path == stalePath && event.withdraw {
					staleWithdraws++
				}
			}
			require.Equal(t, 1, freshAnnounces, "actual refreshed announcement must reach the destination")
			require.Zero(t, freshWithdraws, "EOR must never withdraw a refreshed received path, including before its forward is submitted")
			require.Equal(t, freshSent.MessageID, llgrWriterReceipt(t, f, freshRaw, freshPath, f.source))
			if schedule == "other-source" {
				require.Zero(t, staleWithdraws, "old source cleanup must not remove the newer other-source owner")
				require.Equal(t, replacement.MessageID, llgrWriterReceipt(t, f, staleRaw, stalePath, f.other))
			} else {
				require.Equal(t, 1, staleWithdraws, "the unrefreshed owner must actually be withdrawn, not just deleted from RIB")
				require.Zero(t, llgrWriterReceipt(t, f, staleRaw, stalePath, nil))
			}
			sentRows := llgrWriterRoutes(t, proc, "sent", f.destination.Settings().Address.String())
			wantSent := 1
			if schedule == "other-source" {
				wantSent = 2
			}
			require.Len(t, sentRows, wantSent, "delayed feedback must not resurrect the purged sent path")
		})
	}
}

func llgrWriterCommand(t *testing.T, proc *process.Process, command string, args ...string) json.RawMessage {
	t.Helper()
	out, err := proc.Bridge().DispatchCommandArgs(t.Context(), command, args, "*")
	require.NoError(t, err)
	require.NotNil(t, out)
	defer out.TransportComplete()
	require.Equal(t, rpc.StatusDone, out.Status, out.Error)
	require.Empty(t, out.Error)
	return out.Data
}

type llgrWriterRoute struct {
	Prefix     string `json:"prefix"`
	StaleLevel uint8  `json:"stale-level"`
}

func llgrWriterRoutes(t *testing.T, proc *process.Process, direction, peer string) []llgrWriterRoute {
	t.Helper()
	data := llgrWriterCommand(t, proc, "show bgp rib", direction, "peer", peer)
	var result struct {
		Routes []llgrWriterRoute `json:"routes"`
	}
	require.NoError(t, json.Unmarshal(data, &result))
	return result.Routes
}

// llgrWriterReceipt reads the actual Session-bound writer table, never the RIB
// projection. A nil source asks for absence, not for an unrestricted owner.
func llgrWriterReceipt(t *testing.T, f *ownershipRailFixture, raw []byte, sentPath uint32, source *Peer) uint64 {
	t.Helper()
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	var path fwdPathKey
	addPath := f.destination.sendCtx.Load().AddPath(family.IPv4Unicast)
	if addPath {
		raw = raw[4:]
	}
	require.NoError(t, fwdPathKeyFor(&path, family.IPv4Unicast, sentPath, raw, false, scratch[:]))
	f.destination.adjOut.mu.Lock()
	defer f.destination.adjOut.mu.Unlock()
	entry := f.destination.adjOut.routes[adjOutKey{path: path, addPath: addPath}]
	if source == nil {
		require.Nil(t, entry)
		return 0
	}
	require.NotNil(t, entry)
	require.Same(t, source, entry.owner.source)
	require.NotZero(t, entry.revision)
	return entry.revision
}
