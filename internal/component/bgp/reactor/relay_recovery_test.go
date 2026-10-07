// Design: docs/architecture/plugin/rib-storage-design.md -- causal recovery admission.
package reactor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/plugins/cmd/update"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
)

// recoveryAdvertiseFailed establishes sent history through the actual final
// writer before stopping the configured source. Bodies already have the
// destination's negotiated framing, including its assigned ADD-PATH IDs.
func recoveryAdvertiseFailed(t *testing.T, r *Reactor, destination *Peer, bodies ...[]byte) {
	t.Helper()
	failed, _ := newAnnouncePeer(t, "192.0.2.10")
	failed.recvCtxID = destination.sendContextID()
	r.peers[failed.Settings().PeerKey()] = failed
	for _, body := range bodies {
		require.NoError(t, ownershipWriterForward(t, destination, failed, body, false))
	}
	failed.setState(PeerStateStopped)
}

// RFC 4271 Section 6 advertises the new best route after the
// failed path is invalidated; RFC 4271 Section 3.1 makes a newer advertisement
// replace the preceding route rather than coexist with it on a non-ADD-PATH peer.
// PREVENTS: a destination-wide receipt fence dropping a required replacement on
// unrelated traffic, or an old DOWN overwriting a newer target advertisement.
func TestRecoveryFinalWorkerRequestsCausalReresolution(t *testing.T) {
	for _, newerTarget := range []bool{false, true} {
		name := "unrelated-prefix"
		if newerTarget {
			name = "newer-target-owner"
		}
		t.Run(name, func(t *testing.T) {
			var destination, source *Peer
			var once sync.Once
			var superseded atomic.Bool
			writeResult := make(chan error, 1)
			generationProof := make(chan bool, 2)
			r, src, dst, conn, ctxID := newSyncOrderRailWith(t, func(key fwdKey, items []fwdItem) {
				// The worker also receives data-free completion sentinels.
				// Observe recovery admissions, not those ordinary barriers.
				if len(items) == 0 || items[0].recovery == nil {
					fwdBatchHandler(key, items)
					return
				}
				generationProof <- len(items) == 1 && len(items[0].recovery.items) == 1 &&
					items[0].recovery.items[0].receivedPeer == source &&
					items[0].recovery.items[0].receivedGeneration == source.forwardGeneration.Load()
				once.Do(func() {
					body := bytes.Clone(syncOrderAnnounceBody)
					if !newerTarget {
						body[len(body)-1] = 1 // 192.0.1.0/24 is unrelated to the target.
					}
					session := destination.session
					session.writeMu.Lock()
					err := session.writeRawUpdateBody(body)
					if err == nil {
						err = session.bufWriter.Flush()
					}
					session.writeMu.Unlock()
					superseded.Store(newerTarget)
					writeResult <- err
				})
				fwdBatchHandler(key, items)
			})
			source, destination = src, dst
			r.clock = clock.RealClock{}
			source.recvCtxID = ctxID
			destination.sendingInitialRoutes.Store(0)
			destination.session.setSendCtxID(ctxID)
			destination.session.onMessageReceived = r.notifyMessageReceiver
			stored := storedIPv4Route(source.Settings().Address.String())
			attrs, err := hex.DecodeString(stored.AttrHex)
			require.NoError(t, err)
			calls := 0
			owner := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryRoute, error) {
				calls++
				require.Equal(t, [][]byte{syncOrderPrefixWire}, request.NLRIs)
				if superseded.Load() {
					return nil, nil // The causally newer sent owner is outside the DOWN cut.
				}
				return []ribevents.RecoveryRoute{{Source: source.Settings().Address, NLRI: syncOrderPrefixWire,
					Attributes: attrs, NextHop: []byte{1, 1, 1, 1}, MessageID: 81, SentNLRI: syncOrderPrefixWire}}, nil
			})
			t.Cleanup(owner.Close)
			api := &reactorAPIAdapter{r: r}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			request := ribevents.RecoveryRequest{Source: netip.MustParseAddr("192.0.2.10"),
				Destination: destination.Settings().Address, Family: family.IPv4Unicast, NLRIs: [][]byte{syncOrderPrefixWire}, Cut: 90}
			session, sequence := recoverySnapshot(destination)
			// RFC 4271 Section 6: replacement is admitted only at the final writer.
			retry, err := api.relayRecovery(ctx, request, destination, session, sequence, plugin.OperatorSender(), api.recoverySources())
			require.NoError(t, err)
			require.True(t, retry, "intervening output requires a fresh causal snapshot, not silent success")
			require.NoError(t, <-writeResult)
			require.True(t, <-generationProof, "reconstruction must retain its surviving source generation")
			require.Len(t, parseWireUpdates(t, conn.written()), 1, "stale queued recovery must not write")

			// The production caller drains the owning process's FIFO before this
			// second lookup. This deterministic provider has already processed the
			// injected send when the first item completion answers above.
			session, sequence = recoverySnapshot(destination)
			// RFC 4271 Section 6: unrelated output does not cancel the required repair.
			retry, err = api.relayRecovery(ctx, request, destination, session, sequence, plugin.OperatorSender(), api.recoverySources())
			require.NoError(t, err)
			require.False(t, retry)
			require.Equal(t, 2, calls)
			updates := parseWireUpdates(t, conn.written())
			if newerTarget {
				require.Len(t, updates, 1)
				require.True(t, updates[0].announces)
			} else {
				require.Len(t, updates, 2)
				require.False(t, updates[0].announces)
				require.True(t, updates[1].announces, "survivor must eventually replace the failed route")
			}
			for _, update := range updates {
				require.False(t, update.withdraws, "a valid replacement must not transiently disappear")
			}
		})
	}
}

// PREVENTS: an asynchronous stored-route replay passing final admission after
// its source session has been replaced, even when the source address is reused.
func TestRecoveryFinalWorkerRejectsChangedSourceGeneration(t *testing.T) {
	r, source, destination, conn, _ := newSyncOrderRail(t)
	destination.sendingInitialRoutes.Store(0)
	session, sequence := recoverySnapshot(destination)
	admission := &recoveryAdmission{peer: destination, session: session, sequence: sequence, done: make(chan bool, 1)}
	generation := source.forwardGeneration.Load()
	source.forwardGeneration.Add(1)
	admission.items = []fwdItem{{peer: destination, rawBodies: [][]byte{syncOrderAnnounceBody},
		receivedPeer: source, receivedGeneration: generation}}
	item := fwdItem{peer: destination, recovery: admission}
	// Exercise the real worker and its completion/resource owner synchronously.
	r.fwdPool.safeBatchHandle(fwdKey{peerAddr: destination.Settings().PeerKey()}, []fwdItem{item})
	require.True(t, <-admission.done, "a stale candidate requires causal re-election")
	require.Empty(t, conn.written())
}

// TestRecoveryLookupRejectsReconnectedSource reconnects the elected source
// inside the RIB lookup, after its old bytes were selected. Only the next lookup's
// new bytes may reach the real writer; the first receipt must request election.
func TestRecoveryLookupRejectsReconnectedSource(t *testing.T) {
	r, source, destination, conn, ctxID := newSyncOrderRail(t)
	r.clock = clock.RealClock{}
	source.recvCtxID = ctxID
	destination.sendingInitialRoutes.Store(0)
	destination.session.setSendCtxID(ctxID)
	destination.session.onMessageReceived = r.notifyMessageReceiver
	stored := storedIPv4Route(source.Settings().Address.String())
	attrs, err := hex.DecodeString(stored.AttrHex)
	require.NoError(t, err)
	calls := 0
	owner := ribevents.PublishRecovery(func(ribevents.RecoveryRequest) ([]ribevents.RecoveryRoute, error) {
		calls++
		raw := syncOrderPrefixWire
		if calls == 1 {
			source.setState(PeerStateStopped)
			source.setState(PeerStateEstablished)
		} else {
			raw = []byte{24, 192, 0, 3}
		}
		return []ribevents.RecoveryRoute{{Source: source.Settings().Address, NLRI: raw,
			Attributes: attrs, NextHop: []byte{1, 1, 1, 1}, MessageID: 81, SentNLRI: syncOrderPrefixWire}}, nil
	})
	t.Cleanup(owner.Close)
	api := &reactorAPIAdapter{r: r}
	request := ribevents.RecoveryRequest{Source: netip.MustParseAddr("192.0.2.10"),
		Destination: destination.Settings().Address, Family: family.IPv4Unicast, NLRIs: [][]byte{syncOrderPrefixWire}, Cut: 90}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	session, sequence := recoverySnapshot(destination)
	retry, err := api.relayRecovery(ctx, request, destination, session, sequence, plugin.OperatorSender(), api.recoverySources())
	require.NoError(t, err)
	require.True(t, retry)
	require.Empty(t, conn.written(), "old selected bytes must not acquire the reconnect generation")
	session, sequence = recoverySnapshot(destination)
	retry, err = api.relayRecovery(ctx, request, destination, session, sequence, plugin.OperatorSender(), api.recoverySources())
	require.NoError(t, err)
	require.False(t, retry)
	require.Equal(t, 2, calls)
	bodies := recoveryWrittenUpdates(t, conn.written())
	require.Len(t, bodies, 1)
	require.Equal(t, []byte{24, 192, 0, 3}, bodies[0].NLRI)
}

// TestRecoveryMixedPolicySectionsCompleteOnce drives the existing raw mixed
// policy shape through reconstruction and the actual destination writer. The
// three-section case adds an independent legacy withdrawal, exposing duplicate
// completions as well as a sibling's self-induced sequence conflict.
func TestRecoveryMixedPolicySectionsCompleteOnce(t *testing.T) {
	for _, three := range []bool{false, true} {
		t.Run(map[bool]string{false: "two", true: "three"}[three], func(t *testing.T) {
			r, source, destination, conn, ctxID := newSyncOrderRail(t)
			r.clock = clock.RealClock{}
			source.recvCtxID = ctxID
			destination.sendingInitialRoutes.Store(0)
			destination.session.setSendCtxID(ctxID)
			destination.session.onMessageReceived = r.notifyMessageReceiver
			destination.settings.NextHopMode = NextHopUnchanged
			destination.settings.ExportFilters = []filterapi.FilterRef{{Name: "policy:mixed-raw"}}
			destination.negotiated.Load().families[family.IPv6Unicast] = true
			destination.refreshForwardFacts()
			mpBody, wantMPWithdrawal := llnhLinkLocalOnlyPayload(t)
			mpUpdate, err := message.UnpackUpdate(mpBody)
			require.NoError(t, err)
			attrs := append(bytes.Clone(mpUpdate.PathAttributes), makeAttr(0x40, 3, []byte{192, 0, 2, 1})...)
			wantLegacy := []byte{24, 203, 0, 113}
			override := buildModTestPayload(attrs, wantLegacy)
			wantWithdraw := []byte{24, 198, 51, 100}
			if three {
				override = append([]byte{0, 4, 24, 198, 51, 100}, override[2:]...)
			}
			// Recovery removes actual failed-source output, including the
			// policy-generated MP withdrawal and independent legacy sibling.
			// RFC 4271 Section 4.3 and RFC 4760 Section 3.
			seed := [][]byte{ownershipRailBody(family.IPv6Unicast, false, wantMPWithdrawal[3:], 0)}
			if three {
				seed = append(seed, ownershipRailBody(family.IPv4Unicast, false, wantWithdraw, 0))
			}
			recoveryAdvertiseFailed(t, r, destination, seed...)
			setup := conn.written()
			require.Len(t, recoveryWrittenUpdates(t, setup), len(seed))
			r.api = &pluginserver.Server{}
			calls := 0
			r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
				calls++
				return PolicyResponse{Action: PolicyModify, Raw: override}
			}
			wantLocalPref := []byte{0, 0, 0, 177}
			r.orderedEgressSteps = orderedEgressStepsFromFuncs(func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
				mods.Op(5, filterapi.AttrModSet, wantLocalPref)
				return true
			})
			r.orderedEgressSteps = append(r.orderedEgressSteps, orderedEgressStep{name: policyChainStepName, policyChain: true})
			stored := storedIPv4Route(source.Settings().Address.String())
			original, err := hex.DecodeString(stored.AttrHex)
			require.NoError(t, err)
			owner := ribevents.PublishRecovery(func(ribevents.RecoveryRequest) ([]ribevents.RecoveryRoute, error) {
				return []ribevents.RecoveryRoute{{Source: source.Settings().Address, NLRI: syncOrderPrefixWire,
					Attributes: original, NextHop: []byte{1, 1, 1, 1}, MessageID: 81, SentNLRI: syncOrderPrefixWire}}, nil
			})
			t.Cleanup(owner.Close)
			api := &reactorAPIAdapter{r: r}
			request := ribevents.RecoveryRequest{Source: netip.MustParseAddr("192.0.2.10"),
				Destination: destination.Settings().Address, Family: family.IPv4Unicast, NLRIs: [][]byte{syncOrderPrefixWire}, Cut: 90}
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			session, sequence := recoverySnapshot(destination)
			retry, err := api.relayRecovery(ctx, request, destination, session, sequence, plugin.OperatorSender(), api.recoverySources())
			require.NoError(t, err)
			require.False(t, retry, "own sections must not invalidate their lookup receipt")
			require.Equal(t, 1, calls)
			var announced, withdrawn, mpWithdrawn int
			for _, written := range recoveryWrittenUpdates(t, conn.written()[len(setup):]) {
				if len(written.NLRI) != 0 {
					announced++
					require.Equal(t, wantLegacy, written.NLRI)
					_, _, value, found := attribute.AttrFind(written.PathAttributes, attribute.AttrLocalPref)
					require.True(t, found)
					require.Equal(t, wantLocalPref, value)
				}
				if len(written.WithdrawnRoutes) != 0 {
					withdrawn++
					require.True(t, three)
					require.Equal(t, wantWithdraw, written.WithdrawnRoutes)
				}
				if _, _, value, found := attribute.AttrFind(written.PathAttributes, attribute.AttrMPUnreachNLRI); found {
					mpWithdrawn++
					require.Equal(t, wantMPWithdrawal, value)
				}
				_, _, _, found := attribute.AttrFind(written.PathAttributes, attribute.AttrMPReachNLRI)
				require.False(t, found, "unusable link-local next hop must not escape")
			}
			require.Equal(t, 1, announced)
			require.Equal(t, 1, mpWithdrawn)
			require.Equal(t, map[bool]int{false: 0, true: 1}[three], withdrawn)
			// Joining exercises the old third-release deadlock rather than
			// abandoning a still-blocked destination worker after assertions.
			r.fwdPool.Stop()
		})
	}
}

// TestRecoveryCanceledAndStoppedOperation releases three child resource owners
// but publishes only one completion, including an operation never written.
func TestRecoveryCanceledAndStoppedOperation(t *testing.T) {
	for _, stopped := range []bool{false, true} {
		t.Run(map[bool]string{false: "canceled", true: "stopped"}[stopped], func(t *testing.T) {
			r, _, destination, conn, _ := newSyncOrderRail(t)
			destination.sendingInitialRoutes.Store(0)
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			session, sequence := recoverySnapshot(destination)
			admission := &recoveryAdmission{peer: destination, session: session,
				sequence: sequence, ctx: ctx, done: make(chan bool, 1)}
			released := 0
			buffers := newPeerPool(4096)
			for range 3 {
				raw, index := buffers.Get()
				require.NotNil(t, raw)
				copy(raw, syncOrderWithdrawBody)
				admission.items = append(admission.items, fwdItem{peer: destination,
					rawBodies: [][]byte{raw[:len(syncOrderWithdrawBody)]}, done: func() { released++ },
					peerBufIdx: index, peerPoolRef: buffers})
			}
			item := fwdItem{peer: destination, recovery: admission}
			key := fwdKey{peerAddr: destination.Settings().PeerKey()}
			if stopped {
				r.fwdPool.Stop()
				require.False(t, r.fwdPool.dispatchOverflow(key, item))
				require.Error(t, admission.err)
			} else {
				r.fwdPool.safeBatchHandle(key, []fwdItem{item})
				require.ErrorIs(t, admission.err, context.Canceled)
			}
			require.False(t, <-admission.done)
			require.Equal(t, 3, released)
			require.Equal(t, peerPoolSize, buffers.top, "every borrowed child buffer returns once")
			require.Empty(t, admission.done, "the operation completes once, not once per child")
			require.Empty(t, conn.written())
		})
	}
}

// recoveryWrittenUpdates decodes complete actual BGP frames, rejecting partial
// output instead of mistaking an unfinished flush for a missing route.
func recoveryWrittenUpdates(t *testing.T, raw []byte) []*message.Update {
	t.Helper()
	var updates []*message.Update
	for len(raw) != 0 {
		require.GreaterOrEqual(t, len(raw), message.HeaderLen)
		length := int(binary.BigEndian.Uint16(raw[16:18]))
		require.GreaterOrEqual(t, length, message.HeaderLen)
		require.LessOrEqual(t, length, len(raw))
		parsed, err := message.UnpackUpdate(raw[message.HeaderLen:length])
		require.NoError(t, err)
		updates = append(updates, parsed)
		raw = raw[length:]
	}
	return updates
}

// TestRecoveryParserRetainsNativeAddPathAndPrunesSent uses the real hex parser,
// registered RIB, strict delivery drain, and recipient TCP. Ingress IDs zero and
// seven identify different affected prefixes, not bytes that may be stripped
// from their prefix by recovery's native encoder.
func TestRecoveryParserRetainsNativeAddPathAndPrunesSent(t *testing.T) {
	peers := extendedRecoveryPeers(t, false, false)
	source, destination := peers[0], peers[1]
	source.send(t, message.PackTo(fatalLengthAnnouncement(), nil))
	lowEventually(t, func() bool {
		announced, _ := fatalLengthRecipientRoutes(t, destination)
		return announced == 3
	}, "source advertisements reach recipient before recovery")
	parsed, err := update.ParseUpdateWire(strings.Fields(
		"nlri ipv4/unicast addpath del 0000000018cb0072 del 0000000718cb0073"), plugin.WireEncodingHex)
	require.NoError(t, err)
	require.Len(t, parsed.Groups, 1)
	r := source.peer.reactor
	batch := bgptypes.NLRIBatch{Family: family.IPv4Unicast, NLRIs: parsed.Groups[0].Withdraw,
		RecoverySource: source.peer.Settings().Address, RecoveryCut: ^uint64(0)}
	api := &reactorAPIAdapter{r: r}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, api.recoverNLRIBatch(ctx, batch, []*Peer{destination.peer}, plugin.OperatorSender()))
	lowEventually(t, func() bool {
		_, withdrawn := fatalLengthRecipientRoutes(t, destination)
		return withdrawn == 3 // The helper returns a two-prefix bit mask, not a count.
	}, "both exact parsed identities are withdrawn on recipient TCP")
	command := r.api.Dispatcher().Registry().Lookup("request bgp rib recovery")
	require.NoError(t, command.Process.DrainEventsApplied(ctx))
	remaining, err := ribevents.RecoveryProvider().Lookup(ribevents.RecoveryRequest{
		Source: batch.RecoverySource, Destination: destination.peer.Settings().Address,
		Family: batch.Family, NLRIs: [][]byte{{24, 203, 0, 114}, {24, 203, 0, 115}}, Cut: batch.RecoveryCut})
	require.NoError(t, err)
	require.Empty(t, remaining, "successful actual withdrawals must prune their sent ownership")
}

// TestRecoveryHardFailureRetiresDestination makes the real lookup owner refuse
// election after routes were advertised. The affected established recipient must
// receive Cease and lose its session, not retain the failed source indefinitely.
func TestRecoveryHardFailureRetiresDestination(t *testing.T) {
	peers := extendedRecoveryPeers(t, false, false)
	source, destination := peers[0], peers[1]
	source.send(t, message.PackTo(fatalLengthAnnouncement(), nil))
	lowEventually(t, func() bool {
		announced, _ := fatalLengthRecipientRoutes(t, destination)
		return announced == 3
	}, "source advertisements precede the ownership failure")
	prior := ribevents.RecoveryProvider()
	failure := errors.New("RIB projection unavailable")
	owner := ribevents.PublishRecovery(func(ribevents.RecoveryRequest) ([]ribevents.RecoveryRoute, error) {
		return nil, failure
	})
	defer func() {
		owner.Close()
		restored := ribevents.PublishRecovery(prior.Lookup)
		t.Cleanup(restored.Close)
	}()
	parsed, err := update.ParseUpdateWire(strings.Fields("nlri ipv4/unicast del 18cb0072"), plugin.WireEncodingHex)
	require.NoError(t, err)
	session := destination.peer.currentSession()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err = (&reactorAPIAdapter{r: source.peer.reactor}).recoverNLRIBatch(ctx,
		bgptypes.NLRIBatch{Family: family.IPv4Unicast, NLRIs: parsed.Groups[0].Withdraw,
			RecoverySource: source.peer.Settings().Address, RecoveryCut: ^uint64(0)},
		[]*Peer{destination.peer}, plugin.OperatorSender())
	require.ErrorIs(t, err, failure)
	lowEventually(t, func() bool {
		destination.mu.Lock()
		defer destination.mu.Unlock()
		for _, frame := range destination.frames {
			if len(frame) >= message.HeaderLen+2 && frame[18] == 3 &&
				frame[19] == byte(message.NotifyCease) && frame[20] == message.NotifyCeaseOutOfResources {
				return true
			}
		}
		return false
	}, "recipient receives an explicit fail-closed NOTIFICATION")
	lowEventually(t, func() bool { return destination.peer.currentSession() != session },
		"failed recovery retires the affected destination session")
}

// recoveryClock changes source state exactly when reconstruction stamps its
// received time, after resolveRelaySource and before forwarding admission.
type recoveryClock struct {
	clock.Clock
	onNow func()
}

func (c recoveryClock) Now() time.Time {
	c.onNow()
	return c.Clock.Now()
}

// TestRecoverySourceLossBeforeEnqueueReresolves exercises errForwardNoSource
// from the actual admission gate, rather than only the final writer fence.
func TestRecoverySourceLossBeforeEnqueueReresolves(t *testing.T) {
	r, source, destination, conn, ctxID := newSyncOrderRail(t)
	source.recvCtxID = ctxID
	destination.sendingInitialRoutes.Store(0)
	destination.session.setSendCtxID(ctxID)
	destination.session.onMessageReceived = r.notifyMessageReceiver
	r.clock = clock.RealClock{}
	recoveryAdvertiseFailed(t, r, destination, syncOrderAnnounceBody)
	setup := conn.written()
	require.Len(t, recoveryWrittenUpdates(t, setup), 1)
	var once sync.Once
	r.clock = recoveryClock{Clock: clock.RealClock{}, onNow: func() {
		once.Do(func() { source.setState(PeerStateStopped) })
	}}
	stored := storedIPv4Route(source.Settings().Address.String())
	attrs, err := hex.DecodeString(stored.AttrHex)
	require.NoError(t, err)
	calls := 0
	failed := netip.MustParseAddr("192.0.2.10")
	owner := ribevents.PublishRecovery(func(ribevents.RecoveryRequest) ([]ribevents.RecoveryRoute, error) {
		calls++
		if calls == 1 {
			return []ribevents.RecoveryRoute{{Source: source.Settings().Address, NLRI: syncOrderPrefixWire,
				Attributes: attrs, NextHop: []byte{1, 1, 1, 1}, MessageID: 81, SentNLRI: syncOrderPrefixWire}}, nil
		}
		return []ribevents.RecoveryRoute{{Source: failed, NLRI: syncOrderPrefixWire,
			SentNLRI: syncOrderPrefixWire, Withdraw: true}}, nil
	})
	t.Cleanup(owner.Close)
	api := &reactorAPIAdapter{r: r}
	request := ribevents.RecoveryRequest{Source: failed, Destination: destination.Settings().Address,
		Family: family.IPv4Unicast, NLRIs: [][]byte{syncOrderPrefixWire}, Cut: 90}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	session, sequence := recoverySnapshot(destination)
	retry, err := api.relayRecovery(ctx, request, destination, session, sequence, plugin.OperatorSender(), api.recoverySources())
	require.NoError(t, err)
	require.True(t, retry, "pre-enqueue source loss must preserve the failed owner's obligation")
	require.Empty(t, conn.written()[len(setup):])
	session, sequence = recoverySnapshot(destination)
	retry, err = api.relayRecovery(ctx, request, destination, session, sequence, plugin.OperatorSender(), api.recoverySources())
	require.NoError(t, err)
	require.False(t, retry)
	written := recoveryWrittenUpdates(t, conn.written()[len(setup):])
	require.Len(t, written, 1)
	require.Equal(t, syncOrderPrefixWire, written[0].WithdrawnRoutes)
}

// TestRecoveryAddPathWritesEntireLookup admits several destination-framed
// identifiers, including zero, as one operation. An older implementation wrote
// just the first path and restarted a full sent-inventory scan for its siblings.
func TestRecoveryAddPathWritesEntireLookup(t *testing.T) {
	r, _, destination, conn, _ := newSyncOrderRail(t)
	r.clock = clock.RealClock{}
	destination.sendingInitialRoutes.Store(0)
	encoding := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
	ctxID, err := bgpctx.Registry.Register(encoding)
	require.NoError(t, err)
	destination.sendCtx.Store(encoding)
	destination.sendCtxID = ctxID
	destination.session.setSendCtxID(ctxID)
	destination.session.onMessageReceived = r.notifyMessageReceiver
	failed := netip.MustParseAddr("192.0.2.10")
	want := [][]byte{{0, 0, 0, 0, 24, 192, 0, 2}, {0, 0, 0, 7, 24, 192, 0, 2}, {0, 0, 0, 17, 24, 192, 0, 3}}
	var seed [][]byte
	for _, raw := range want {
		// RFC 4271 Section 4.3 and RFC 7911 Section 3.
		seed = append(seed, ownershipRailBody(family.IPv4Unicast, false, raw, 0))
	}
	recoveryAdvertiseFailed(t, r, destination, seed...)
	setup := conn.written()
	require.Len(t, recoveryWrittenUpdates(t, setup), len(want))
	calls := 0
	owner := ribevents.PublishRecovery(func(ribevents.RecoveryRequest) ([]ribevents.RecoveryRoute, error) {
		calls++
		var routes []ribevents.RecoveryRoute
		for _, raw := range want {
			routes = append(routes, ribevents.RecoveryRoute{Source: failed, NLRI: raw, SentNLRI: raw, Withdraw: true})
		}
		return routes, nil
	})
	t.Cleanup(owner.Close)
	api := &reactorAPIAdapter{r: r}
	request := ribevents.RecoveryRequest{Source: failed, Destination: destination.Settings().Address,
		Family: family.IPv4Unicast, NLRIs: [][]byte{syncOrderPrefixWire, {24, 192, 0, 3}}, SentAddPath: true, Cut: 90}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	session, sequence := recoverySnapshot(destination)
	retry, err := api.relayRecovery(ctx, request, destination, session, sequence, plugin.OperatorSender(), api.recoverySources())
	require.NoError(t, err)
	require.False(t, retry)
	require.Equal(t, 1, calls)
	var got [][]byte
	for _, written := range recoveryWrittenUpdates(t, conn.written()[len(setup):]) {
		require.Empty(t, written.NLRI)
		got = append(got, written.WithdrawnRoutes)
	}
	require.Equal(t, want, got, "every advertised identifier must survive destination framing")
}

// TestRecoveryDestinationReconnectInvalidatesLookup swaps the destination
// session during selection while its send counter stays unchanged. Only a fresh
// lookup may write to the new connection.
func TestRecoveryDestinationReconnectInvalidatesLookup(t *testing.T) {
	r, source, destination, oldConn, ctxID := newSyncOrderRail(t)
	r.clock = clock.RealClock{}
	source.recvCtxID = ctxID
	destination.sendingInitialRoutes.Store(0)
	destination.session.setSendCtxID(ctxID)
	next, newConn := newSyncOrderDest(t, bgpctx.Registry.Get(ctxID), ctxID)
	next.session.setSendCtxID(ctxID)
	next.session.onMessageReceived = r.notifyMessageReceiver
	stored := storedIPv4Route(source.Settings().Address.String())
	attrs, err := hex.DecodeString(stored.AttrHex)
	require.NoError(t, err)
	calls := 0
	owner := ribevents.PublishRecovery(func(ribevents.RecoveryRequest) ([]ribevents.RecoveryRoute, error) {
		calls++
		if calls == 1 {
			destination.mu.Lock()
			destination.session = next.session
			destination.mu.Unlock()
		}
		return []ribevents.RecoveryRoute{{Source: source.Settings().Address, NLRI: syncOrderPrefixWire,
			Attributes: attrs, NextHop: []byte{1, 1, 1, 1}, MessageID: 81, SentNLRI: syncOrderPrefixWire}}, nil
	})
	t.Cleanup(owner.Close)
	api := &reactorAPIAdapter{r: r}
	request := ribevents.RecoveryRequest{Source: netip.MustParseAddr("192.0.2.10"),
		Destination: destination.Settings().Address, Family: family.IPv4Unicast,
		NLRIs: [][]byte{syncOrderPrefixWire}, Cut: 90}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	session, sequence := recoverySnapshot(destination)
	retry, err := api.relayRecovery(ctx, request, destination, session, sequence, plugin.OperatorSender(), api.recoverySources())
	require.NoError(t, err)
	require.True(t, retry)
	require.Empty(t, oldConn.written())
	require.Empty(t, newConn.written())
	session, sequence = recoverySnapshot(destination)
	retry, err = api.relayRecovery(ctx, request, destination, session, sequence, plugin.OperatorSender(), api.recoverySources())
	require.NoError(t, err)
	require.False(t, retry)
	require.Empty(t, oldConn.written())
	require.Len(t, recoveryWrittenUpdates(t, newConn.written()), 1)
}

// recoveryFailureConn fails one real transport operation and observes closure.
// A partial flush preserves one complete UPDATE on the recipient side, rather
// than merely reporting an error before any output.
type recoveryFailureConn struct {
	recordingConn
	mode       string
	failure    error
	writes     int
	deadline   bool
	closed     bool
	firstFrame int
}

func (c *recoveryFailureConn) Write(raw []byte) (int, error) {
	c.writes++
	if c.mode == "write" && c.writes == 2 {
		return 0, c.failure
	}
	if c.mode == "flush" && c.writes == 1 {
		n, _ := c.recordingConn.Write(raw[:c.firstFrame])
		return n, c.failure
	}
	return c.recordingConn.Write(raw)
}

func (c *recoveryFailureConn) SetWriteDeadline(deadline time.Time) error {
	if c.mode == "deadline" && !deadline.IsZero() && !c.deadline {
		c.deadline = true
		return c.failure
	}
	return nil
}

func (c *recoveryFailureConn) Close() error {
	c.closed = true
	return nil
}

// TestRecoveryWriterFailureOverridesPartialRetry drives the production caller,
// reconstruction, destination worker and fail-close path. Source loss after the
// first section marks retry, but cannot erase a later body/flush failure.
func TestRecoveryWriterFailureOverridesPartialRetry(t *testing.T) {
	for _, mode := range []string{"deadline", "write", "flush"} {
		t.Run(mode, func(t *testing.T) {
			admitted := make(chan *recoveryAdmission, 1)
			var released atomic.Int32
			r, source, destination, seedConn, ctxID := newSyncOrderRailWith(t, func(key fwdKey, items []fwdItem) {
				for i := range items {
					if admission := items[i].recovery; admission != nil {
						admitted <- admission
						for j := range admission.items {
							done := admission.items[j].done
							admission.items[j].done = func() {
								if done != nil {
									done()
								}
								released.Add(1)
							}
						}
					}
				}
				fwdBatchHandler(key, items)
			})
			r.clock = clock.RealClock{}
			r.api = flowForwardPluginServer(t, New(&Config{ListenAddr: "127.0.0.1:0"}))
			lowEventually(t, func() bool {
				command := r.api.Dispatcher().Registry().Lookup("request bgp rib recovery")
				return command != nil && command.Process != nil
			}, "recovery command owner registered")
			source.recvCtxID = ctxID
			destination.sendingInitialRoutes.Store(0)
			session := destination.currentSession()
			session.setSendCtxID(ctxID)
			// Seed both failed-source slots before arming transport failure
			// and the onWrite source-loss hook.
			// RFC 4271 Section 4.3.
			recoveryAdvertiseFailed(t, r, destination, syncOrderAnnounceBody,
				ownershipRailBody(family.IPv4Unicast, false, []byte{32, 192, 0, 2, 9}, 0))
			require.Len(t, recoveryWrittenUpdates(t, seedConn.written()), 2)
			var once sync.Once
			session.onWrite = func() {
				once.Do(func() { source.setState(PeerStateStopped) })
			}
			failure := errors.New("injected recovery " + mode + " failure")
			conn := &recoveryFailureConn{mode: mode, failure: failure,
				firstFrame: message.HeaderLen + len(syncOrderWithdrawBody)}
			session.conn = conn
			size := 4096
			if mode == "write" {
				size = conn.firstFrame
			}
			session.bufWriter = bufio.NewWriterSize(conn, size)
			stored := storedIPv4Route(source.Settings().Address.String())
			attrs, err := hex.DecodeString(stored.AttrHex)
			require.NoError(t, err)
			failed := netip.MustParseAddr("192.0.2.10")
			calls := 0
			owner := ribevents.PublishRecovery(func(ribevents.RecoveryRequest) ([]ribevents.RecoveryRoute, error) {
				calls++
				if calls > 1 {
					return nil, errors.New("hard write failure incorrectly requested another election")
				}
				return []ribevents.RecoveryRoute{
					{Source: failed, NLRI: syncOrderPrefixWire, SentNLRI: syncOrderPrefixWire, Withdraw: true},
					{Source: source.Settings().Address, NLRI: syncOrderPrefixWire, SentNLRI: syncOrderPrefixWire,
						Attributes: attrs, NextHop: []byte{1, 1, 1, 1}, MessageID: 81},
					{Source: failed, NLRI: []byte{32, 192, 0, 2, 9}, SentNLRI: []byte{32, 192, 0, 2, 9}, Withdraw: true},
				}, nil
			})
			t.Cleanup(owner.Close)
			parsed, err := update.ParseUpdateWire(strings.Fields("nlri ipv4/unicast del 18c00002"), plugin.WireEncodingHex)
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			err = (&reactorAPIAdapter{r: r}).recoverNLRIBatch(ctx,
				bgptypes.NLRIBatch{Family: family.IPv4Unicast, NLRIs: parsed.Groups[0].Withdraw,
					RecoverySource: failed, RecoveryCut: 90}, []*Peer{destination}, plugin.OperatorSender())
			require.ErrorIs(t, err, failure, "the consumer must receive the actual transport failure")
			r.fwdPool.Stop()
			require.Equal(t, 1, calls, "hard failure wins over re-election")
			admission := <-admitted
			require.Equal(t, mode != "deadline", admission.retry)
			require.ErrorIs(t, admission.err, failure)
			require.False(t, admission.written)
			require.Empty(t, admission.done, "the consumer receives exactly one completion")
			require.Empty(t, admission.items, "all staged ownership is released")
			require.Equal(t, int32(3), released.Load())
			require.True(t, conn.closed, "the affected recipient transport must be retired")
			raw := conn.written()
			if mode != "deadline" {
				require.GreaterOrEqual(t, len(raw), conn.firstFrame)
				written := recoveryWrittenUpdates(t, raw[:conn.firstFrame])
				require.Len(t, written, 1)
				require.Equal(t, syncOrderPrefixWire, written[0].WithdrawnRoutes)
				raw = raw[conn.firstFrame:]
			}
			if mode == "deadline" {
				require.GreaterOrEqual(t, len(raw), message.HeaderLen+2)
				require.Equal(t, byte(3), raw[18])
				require.Equal(t, byte(message.NotifyCease), raw[19])
				require.Equal(t, message.NotifyCeaseOutOfResources, raw[20])
			} else {
				// bufio retains the write error, so even Cease cannot escape.
				// The consumer must see transport closure after partial output.
				require.Empty(t, raw, "no later section may escape the failed transport")
			}
		})
	}
}

// TestRecoveryRetryOwnsReplacementSession distinguishes the replacement selected
// by a fresh attempt from a later reconnect which that failed attempt never owned.
func TestRecoveryRetryOwnsReplacementSession(t *testing.T) {
	for _, reconnectAgain := range []bool{false, true} {
		t.Run(map[bool]string{false: "admitted-replacement", true: "unrelated-reconnect"}[reconnectAgain], func(t *testing.T) {
			r, _, destination, oldConn, ctxID := newSyncOrderRail(t)
			r.clock = clock.RealClock{}
			r.api = flowForwardPluginServer(t, New(&Config{ListenAddr: "127.0.0.1:0"}))
			lowEventually(t, func() bool {
				command := r.api.Dispatcher().Registry().Lookup("request bgp rib recovery")
				return command != nil && command.Process != nil
			}, "recovery command owner registered")
			destination.sendingInitialRoutes.Store(0)
			original := destination.currentSession()
			replacement, replacementConn := newSyncOrderDest(t, bgpctx.Registry.Get(ctxID), ctxID)
			unrelated, unrelatedConn := newSyncOrderDest(t, bgpctx.Registry.Get(ctxID), ctxID)
			failure := errors.New("replacement attempt lookup failed")
			calls := 0
			owner := ribevents.PublishRecovery(func(ribevents.RecoveryRequest) ([]ribevents.RecoveryRoute, error) {
				calls++
				destination.mu.Lock()
				defer destination.mu.Unlock()
				if calls == 1 {
					destination.session = replacement.session
					return nil, nil // The changed session forces the actual caller to retry.
				}
				if reconnectAgain {
					destination.session = unrelated.session
				}
				return nil, failure
			})
			t.Cleanup(owner.Close)
			parsed, err := update.ParseUpdateWire(strings.Fields("nlri ipv4/unicast del 18c00002"), plugin.WireEncodingHex)
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			err = (&reactorAPIAdapter{r: r}).recoverNLRIBatch(ctx,
				bgptypes.NLRIBatch{Family: family.IPv4Unicast, NLRIs: parsed.Groups[0].Withdraw,
					RecoverySource: netip.MustParseAddr("192.0.2.10"), RecoveryCut: 90},
				[]*Peer{destination}, plugin.OperatorSender())
			require.ErrorIs(t, err, failure)
			require.Equal(t, 2, calls)
			require.Empty(t, oldConn.written())
			require.False(t, original.tearingDown.Load())
			require.Empty(t, unrelatedConn.written())
			require.False(t, unrelated.session.tearingDown.Load(), "unadmitted reconnect must remain usable")
			if reconnectAgain {
				require.Empty(t, replacementConn.written())
				require.False(t, replacement.session.tearingDown.Load())
				unrelated.session.writeMu.Lock()
				err = unrelated.session.writeRawUpdateBody(syncOrderWithdrawBody)
				if err == nil {
					err = unrelated.session.flushWrites()
				}
				unrelated.session.writeMu.Unlock()
				require.NoError(t, err)
				require.Len(t, recoveryWrittenUpdates(t, unrelatedConn.written()), 1)
			} else {
				raw := replacementConn.written()
				require.GreaterOrEqual(t, len(raw), message.HeaderLen+2)
				require.Equal(t, byte(3), raw[18])
				require.Equal(t, byte(message.NotifyCease), raw[19])
				require.Equal(t, message.NotifyCeaseOutOfResources, raw[20])
				require.True(t, replacement.session.tearingDown.Load())
			}
		})
	}
}
