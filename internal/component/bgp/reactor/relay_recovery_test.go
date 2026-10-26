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
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
)

// recoveryAdvertiseFailed establishes sent history through the actual final
// writer before stopping the configured source, and returns that source.
// Bodies already have the destination's negotiated framing, including its
// assigned ADD-PATH IDs. Stopping the source ends the session that owns the
// destination's entries, so a departure owes them.
func recoveryAdvertiseFailed(t *testing.T, r *Reactor, destination *Peer, bodies ...[]byte) *Peer {
	t.Helper()
	failed, _ := newAnnouncePeer(t, "192.0.2.10")
	failed.recvCtxID = destination.sendContextID()
	r.peers[failed.Settings().PeerKey()] = failed
	for _, body := range bodies {
		require.NoError(t, ownershipWriterForward(t, destination, failed, body, false))
	}
	failed.setState(PeerStateStopped)
	return failed
}

// recoveryOwnerStart registers the bgp-rib recovery command owner and starts
// the destination session's recovery worker, as runOnce does before
// Established. Without both, a departure retires the destination as
// ownerless. It MUST run before the test publishes its own lookup double,
// because the registered RIB publishes the real one when it starts.
func recoveryOwnerStart(t *testing.T, r *Reactor, destination *Peer) {
	t.Helper()
	if r.api == nil {
		r.api = flowForwardPluginServer(t, New(&Config{ListenAddr: "127.0.0.1:0"}))
	}
	lowEventually(t, func() bool {
		command := r.api.Dispatcher().Registry().Lookup(recoveryCommandName)
		return command != nil && command.Process != nil
	}, "recovery command owner registered")
	destination.reactor = r
	session := destination.currentSession()
	session.startRecovery(destination)
	t.Cleanup(session.joinRecovery)
}

// recoveryDepartureBatch is bgp-rs's departure command for source over the
// IPv4 prefixes, covering every session of the source so far: its callers
// test the decision, not which session a departure answers.
func recoveryDepartureBatch(source netip.Addr, prefixes ...string) bgptypes.NLRIBatch {
	batch := bgptypes.NLRIBatch{Family: family.IPv4Unicast, RecoverySource: source,
		RecoveryGeneration: adjOutEverySession}
	for _, prefix := range prefixes {
		batch.NLRIs = append(batch.NLRIs, nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix(prefix), 0, false))
	}
	return batch
}

// recoveryDepart runs the departure command and returns once every key it
// owed is decided.
func recoveryDepart(t *testing.T, r *Reactor, destination *Peer, source netip.Addr, prefixes ...string) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	return (&reactorAPIAdapter{r: r}).recoverNLRIBatch(ctx, recoveryDepartureBatch(source, prefixes...),
		[]*Peer{destination}, plugin.ProcessSender("bgp-rs"))
}

// recoveryOweForTest owes one IPv4 key at the destination session, as a
// writer or a departure does.
func recoveryOweForTest(t *testing.T, destination *Peer, session *Session, raw []byte) (adjOutKey, *adjOutOwed) {
	t.Helper()
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	var key adjOutKey
	require.NoError(t, fwdPathKeyFor(&key.path, family.IPv4Unicast, 0, raw, false, scratch[:]))
	table := &destination.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	return key, table.owe(key, session, family.IPv4Unicast, raw)
}

// recoveryOwedRecord reads the destination's record for key.
func recoveryOwedRecord(destination *Peer, key adjOutKey) *adjOutOwed {
	table := &destination.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	return table.owed[key]
}

// recoveryLookups records every lookup request a double receives.
type recoveryLookups struct {
	mu       sync.Mutex
	requests []ribevents.RecoveryRequest
}

func (l *recoveryLookups) add(request ribevents.RecoveryRequest) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.requests = append(l.requests, request)
	return len(l.requests)
}

func (l *recoveryLookups) all() []ribevents.RecoveryRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]ribevents.RecoveryRequest(nil), l.requests...)
}

// RFC 4271 Section 6 advertises the new best route after the
// failed path is invalidated; RFC 4271 Section 3.1 makes a newer advertisement
// replace the preceding route rather than coexist with it on a non-ADD-PATH peer.
// PREVENTS: unrelated traffic dropping a required replacement, or an old DOWN
// overwriting a newer target advertisement.
// Method: the departed source owns P at the destination through its ended
// session. "unrelated-prefix" writes 192.0.1.0/24 to the destination between
// the lookup and the admission; the per-key admission still writes the
// replacement once, with no second election. "newer-target-owner" lets the
// live source's newer advertisement own P before the DOWN, so the departure
// owes nothing and asks the RIB nothing.
func TestRecoveryFinalWorkerRequestsCausalReresolution(t *testing.T) {
	for _, newerTarget := range []bool{false, true} {
		name := "unrelated-prefix"
		if newerTarget {
			name = "newer-target-owner"
		}
		t.Run(name, func(t *testing.T) {
			var destination, source *Peer
			var once sync.Once
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
					body[len(body)-1] = 1 // 192.0.1.0/24 is unrelated to the target.
					session := destination.session
					session.writeMu.Lock()
					err := session.writeRawUpdateBody(body)
					if err == nil {
						err = session.bufWriter.Flush()
					}
					session.writeMu.Unlock()
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
			failed := recoveryAdvertiseFailed(t, r, destination, syncOrderAnnounceBody)
			if newerTarget {
				// RFC 4271 Section 3.1: the live source's newer advertisement
				// replaces the departed source's route at the destination.
				require.NoError(t, ownershipWriterForward(t, destination, source, syncOrderAnnounceBody, false))
			}
			setup := len(conn.written())
			stored := storedIPv4Route(source.Settings().Address.String())
			attrs, err := hex.DecodeString(stored.AttrHex)
			require.NoError(t, err)
			recoveryOwnerStart(t, r, destination)
			var calls atomic.Int32
			owner := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
				calls.Add(1)
				return recoverySurvivorAnswer(request, syncOrderPrefixWire, ribevents.RecoveryRoute{Source: source.Settings().Address,
					NLRI: syncOrderPrefixWire, Attributes: attrs, NextHop: []byte{1, 1, 1, 1}, MessageID: 81}), nil
			})
			t.Cleanup(owner.Close)
			// RFC 4271 Section 6: replacement is admitted only at the final writer.
			require.NoError(t, recoveryDepart(t, r, destination, failed.Settings().Address, "192.0.2.0/24"))
			updates := recoveryWrittenUpdates(t, conn.written()[setup:])
			if newerTarget {
				require.Zero(t, calls.Load(), "an old DOWN owes nothing where a newer advertisement owns the key")
				require.Empty(t, updates, "an old DOWN never overwrites a newer target advertisement")
				return
			}
			require.NoError(t, <-writeResult)
			require.True(t, <-generationProof, "reconstruction must retain its surviving source generation")
			// RFC 4271 Section 6: unrelated output does not cancel the required repair.
			require.Equal(t, int32(1), calls.Load(), "unrelated output never forces a second election")
			require.Len(t, updates, 2)
			require.Equal(t, []byte{24, 192, 0, 1}, updates[0].NLRI)
			require.Equal(t, syncOrderPrefixWire, updates[1].NLRI, "the replacement is written once, with no retry")
			for _, update := range updates {
				require.Empty(t, update.WithdrawnRoutes, "a valid replacement must not transiently disappear")
			}
		})
	}
}

// TestRecoveryLookupRejectsReconnectedSource reconnects the elected source
// inside the RIB lookup, after its old bytes were selected. The old bytes
// never reach the writer: the key is owed again with the reconnected source
// excluded, and the next election, with no candidate left, withdraws it. The
// reconnected session's new bytes reach the destination through its own
// forward, never through a re-election of the old ones.
func TestRecoveryLookupRejectsReconnectedSource(t *testing.T) {
	r, source, destination, conn, ctxID := newSyncOrderRail(t)
	r.clock = clock.RealClock{}
	source.recvCtxID = ctxID
	destination.sendingInitialRoutes.Store(0)
	destination.session.setSendCtxID(ctxID)
	destination.session.onMessageReceived = r.notifyMessageReceiver
	failed := recoveryAdvertiseFailed(t, r, destination, syncOrderAnnounceBody)
	setup := len(conn.written())
	stored := storedIPv4Route(source.Settings().Address.String())
	attrs, err := hex.DecodeString(stored.AttrHex)
	require.NoError(t, err)
	recoveryOwnerStart(t, r, destination)
	var lookups recoveryLookups
	owner := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
		if lookups.add(request) != 1 {
			return recoveryWithdrawAnswer(request), nil
		}
		source.setState(PeerStateStopped)
		source.setState(PeerStateEstablished)
		return recoverySurvivorAnswer(request, syncOrderPrefixWire, ribevents.RecoveryRoute{Source: source.Settings().Address,
			NLRI: syncOrderPrefixWire, Attributes: attrs, NextHop: []byte{1, 1, 1, 1}, MessageID: 81}), nil
	})
	t.Cleanup(owner.Close)
	require.NoError(t, recoveryDepart(t, r, destination, failed.Settings().Address, "192.0.2.0/24"))
	requests := lookups.all()
	require.Len(t, requests, 2)
	require.ElementsMatch(t, []netip.Addr{failed.Settings().Address, source.Settings().Address},
		requests[1].Keys[0].Excluded, "the reconnected source's old selection is excluded from the next election")
	written := recoveryWrittenUpdates(t, conn.written()[setup:])
	require.Len(t, written, 1)
	require.Empty(t, written[0].NLRI, "old selected bytes must not acquire the reconnect generation")
	require.Equal(t, syncOrderPrefixWire, written[0].WithdrawnRoutes)
}

// TestRecoveryMixedPolicySectionsCompleteOnce drives the existing raw mixed
// policy shape through reconstruction and the actual destination writer. The
// three-section case adds an independent legacy withdrawal, exposing duplicate
// completions. The departed source owns every key the policy output touches.
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
			destination.settings.ExportPolicy = mustPolicy(t, []filterapi.FilterRef{{Name: "policy:mixed-raw"}}, filterapi.FinalAccept)
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
			seed := [][]byte{syncOrderAnnounceBody, ownershipRailBody(family.IPv6Unicast, false, wantMPWithdrawal[3:], 0)}
			if three {
				seed = append(seed, ownershipRailBody(family.IPv4Unicast, false, wantWithdraw, 0))
			}
			failed := recoveryAdvertiseFailed(t, r, destination, seed...)
			setup := conn.written()
			require.Len(t, recoveryWrittenUpdates(t, setup), len(seed))
			recoveryOwnerStart(t, r, destination)
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
			owner := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
				return recoverySurvivorAnswer(request, syncOrderPrefixWire, ribevents.RecoveryRoute{Source: source.Settings().Address,
					NLRI: syncOrderPrefixWire, Attributes: original, NextHop: []byte{1, 1, 1, 1}, MessageID: 81}), nil
			})
			t.Cleanup(owner.Close)
			require.NoError(t, recoveryDepart(t, r, destination, failed.Settings().Address, "192.0.2.0/24"))
			require.Equal(t, 1, calls, "own sections must not invalidate their lookup receipt")
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
// Method: the admission's key is owed at the destination session with the
// round's epoch, so the canceled admission acts on a live record; both
// halves assert the record survives, left owed for the next round. A second
// completion would close the done channel twice and panic.
func TestRecoveryCanceledAndStoppedOperation(t *testing.T) {
	for _, stopped := range []bool{false, true} {
		t.Run(map[bool]string{false: "canceled", true: "stopped"}[stopped], func(t *testing.T) {
			r, _, destination, conn, _ := newSyncOrderRail(t)
			destination.sendingInitialRoutes.Store(0)
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			session := destination.currentSession()
			key, record := recoveryOweForTest(t, destination, session, syncOrderPrefixWire)
			admission := newRecoveryAdmission(ctx, destination, session, (&reactorAPIAdapter{r: r}).recoverySources(),
				[]recoveryAdmitKey{{key: key, record: record, epoch: record.epoch}})
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
			admission.mark(0)
			item := fwdItem{peer: destination, recovery: admission}
			fwd := fwdKey{peerAddr: destination.Settings().PeerKey()}
			if stopped {
				r.fwdPool.Stop()
				require.False(t, r.fwdPool.dispatchOverflow(fwd, &item))
				require.Error(t, admission.err)
			} else {
				r.fwdPool.safeBatchHandle(fwd, []fwdItem{item})
				require.ErrorIs(t, admission.err, context.Canceled)
			}
			select {
			case <-admission.done:
			default:
				t.Fatal("the operation never completed")
			}
			require.Equal(t, 3, released)
			require.Equal(t, peerPoolSize, buffers.top, "every borrowed child buffer returns once")
			require.Empty(t, conn.written())
			require.Same(t, record, recoveryOwedRecord(destination, key),
				"an admission that never ran leaves its key owed for the next round")
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

// lowLiveDepart ends a live source's session the way a real departure does:
// its transport closes, and the test waits until it has left Established, so
// the generation that owns its routes at every destination is no longer
// current. A running Session.Run keeps the FSM and the reactor in agreement.
func lowLiveDepart(t *testing.T, source *lowLivePeer) {
	t.Helper()
	require.NoError(t, source.remote.Close())
	lowEventually(t, func() bool { return source.peer.State() != PeerStateEstablished },
		"the departed source left Established")
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
	lowLiveDepart(t, source)
	parsed, err := update.ParseUpdateWire(strings.Fields(
		"nlri ipv4/unicast addpath del 0000000018cb0072 del 0000000718cb0073"), plugin.WireEncodingHex)
	require.NoError(t, err)
	require.Len(t, parsed.Groups, 1)
	r := source.peer.reactor
	batch := bgptypes.NLRIBatch{Family: family.IPv4Unicast, NLRIs: parsed.Groups[0].Withdraw,
		RecoverySource: source.peer.Settings().Address, RecoveryGeneration: source.peer.forwardGeneration.Load()}
	api := &reactorAPIAdapter{r: r}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, api.recoverNLRIBatch(ctx, batch, []*Peer{destination.peer}, plugin.OperatorSender()))
	lowEventually(t, func() bool {
		_, withdrawn := fatalLengthRecipientRoutes(t, destination)
		return withdrawn == 3 // The helper returns a two-prefix bit mask, not a count.
	}, "both exact parsed identities are withdrawn on recipient TCP")
	command := r.api.Dispatcher().Registry().Lookup(recoveryCommandName)
	require.NoError(t, command.Process.DrainEventsApplied(ctx))
	remaining, err := ribevents.RecoveryProvider().Lookup(recoveryDepartureRequest(batch.RecoverySource,
		destination.peer.Settings().Address, batch.Family, []byte{24, 203, 0, 114}, []byte{24, 203, 0, 115}))
	require.NoError(t, err)
	require.Len(t, remaining, 2)
	for _, outcome := range remaining {
		require.Empty(t, outcome.Sent, "successful actual withdrawals must prune their sent ownership")
	}
}

// TestRecoveryHardFailureRetiresDestination makes the real lookup owner refuse
// election after routes were advertised and their source departed. The
// affected established recipient must receive Cease and lose its session, not
// retain the failed source indefinitely. The departure's keys complete with
// the retired session.
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
	owner := ribevents.PublishRecovery(func(ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
		return nil, failure
	})
	defer func() {
		owner.Close()
		restored := ribevents.PublishRecovery(prior.Lookup)
		t.Cleanup(restored.Close)
	}()
	lowLiveDepart(t, source)
	parsed, err := update.ParseUpdateWire(strings.Fields("nlri ipv4/unicast del 18cb0072"), plugin.WireEncodingHex)
	require.NoError(t, err)
	session := destination.peer.currentSession()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err = (&reactorAPIAdapter{r: source.peer.reactor}).recoverNLRIBatch(ctx,
		bgptypes.NLRIBatch{Family: family.IPv4Unicast, NLRIs: parsed.Groups[0].Withdraw,
			RecoverySource: source.peer.Settings().Address, RecoveryGeneration: source.peer.forwardGeneration.Load()},
		[]*Peer{destination.peer}, plugin.OperatorSender())
	require.NoError(t, err, "the owed keys complete with the retired session")
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

// TestRecoverySourceLossBeforeEnqueueReowes exercises a survivor whose source
// session ends while its replacement is reconstructed, before the forwarding
// admission. The lost source's route is never written: the key is owed again
// with that source excluded, and the next election withdraws it.
func TestRecoverySourceLossBeforeEnqueueReowes(t *testing.T) {
	r, source, destination, conn, ctxID := newSyncOrderRail(t)
	source.recvCtxID = ctxID
	destination.sendingInitialRoutes.Store(0)
	destination.session.setSendCtxID(ctxID)
	destination.session.onMessageReceived = r.notifyMessageReceiver
	r.clock = clock.RealClock{}
	failed := recoveryAdvertiseFailed(t, r, destination, syncOrderAnnounceBody)
	setup := conn.written()
	require.Len(t, recoveryWrittenUpdates(t, setup), 1)
	recoveryOwnerStart(t, r, destination)
	stored := storedIPv4Route(source.Settings().Address.String())
	attrs, err := hex.DecodeString(stored.AttrHex)
	require.NoError(t, err)
	var lookups recoveryLookups
	owner := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
		if lookups.add(request) == 1 {
			return recoverySurvivorAnswer(request, syncOrderPrefixWire, ribevents.RecoveryRoute{Source: source.Settings().Address,
				NLRI: syncOrderPrefixWire, Attributes: attrs, NextHop: []byte{1, 1, 1, 1}, MessageID: 81}), nil
		}
		return recoveryWithdrawAnswer(request), nil
	})
	t.Cleanup(owner.Close)
	var once sync.Once
	r.clock = recoveryClock{Clock: clock.RealClock{}, onNow: func() {
		once.Do(func() { source.setState(PeerStateStopped) })
	}}
	require.NoError(t, recoveryDepart(t, r, destination, failed.Settings().Address, "192.0.2.0/24"))
	requests := lookups.all()
	require.Len(t, requests, 2)
	require.Contains(t, requests[1].Keys[0].Excluded, source.Settings().Address,
		"pre-enqueue source loss owes the key again without that source")
	written := recoveryWrittenUpdates(t, conn.written()[len(setup):])
	require.Len(t, written, 1)
	require.Empty(t, written[0].NLRI, "the lost source's route is never written")
	require.Equal(t, syncOrderPrefixWire, written[0].WithdrawnRoutes)
}

// recoveryAddPathRail is a sync-order rail whose destination negotiated
// ADD-PATH for IPv4 unicast.
func recoveryAddPathRail(t *testing.T) (*Reactor, *Peer, *recordingConn) {
	t.Helper()
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
	return r, destination, conn
}

// recoveryAddPathAnswer decides every key with WITHDRAW of the identifiers
// sent lists for its prefix, as the RIB's SentAddPath branch answers.
func recoveryAddPathAnswer(sent map[string][][]byte) func(ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
	return func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
		if !request.SentAddPath {
			return nil, errors.New("an ADD-PATH departure must ask for the sent identifiers")
		}
		outcomes := make([]ribevents.RecoveryOutcome, len(request.Keys))
		for i := range request.Keys {
			outcomes[i] = ribevents.RecoveryOutcome{NLRI: request.Keys[i].NLRI, Withdraw: true,
				Sent: sent[string(request.Keys[i].NLRI)]}
		}
		return outcomes, nil
	}
}

// TestRecoveryAddPathWritesEntireLookup admits several destination-framed
// identifiers, including zero, as one operation. An older implementation wrote
// just the first path and restarted a full sent-inventory scan for its siblings.
func TestRecoveryAddPathWritesEntireLookup(t *testing.T) {
	r, destination, conn := recoveryAddPathRail(t)
	want := [][]byte{{0, 0, 0, 0, 24, 192, 0, 2}, {0, 0, 0, 7, 24, 192, 0, 2}, {0, 0, 0, 17, 24, 192, 0, 3}}
	var seed [][]byte
	for _, raw := range want {
		// RFC 4271 Section 4.3 and RFC 7911 Section 3.
		seed = append(seed, ownershipRailBody(family.IPv4Unicast, false, raw, 0))
	}
	failed := recoveryAdvertiseFailed(t, r, destination, seed...)
	setup := conn.written()
	require.Len(t, recoveryWrittenUpdates(t, setup), len(want))
	recoveryOwnerStart(t, r, destination)
	var calls atomic.Int32
	answer := recoveryAddPathAnswer(map[string][][]byte{
		string(syncOrderPrefixWire): want[:2], string([]byte{24, 192, 0, 3}): want[2:]})
	owner := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
		calls.Add(1)
		return answer(request)
	})
	t.Cleanup(owner.Close)
	require.NoError(t, recoveryDepart(t, r, destination, failed.Settings().Address, "192.0.2.0/24", "192.0.3.0/24"))
	require.Equal(t, int32(1), calls.Load(), "one lookup answers every identifier")
	var got [][]byte
	for _, written := range recoveryWrittenUpdates(t, conn.written()[len(setup):]) {
		require.Empty(t, written.NLRI)
		got = append(got, written.WithdrawnRoutes)
	}
	require.ElementsMatch(t, want, got, "every advertised identifier must survive destination framing")
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
	closed     atomic.Bool // Written by the teardown, read by the test.
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
	c.closed.Store(true)
	return nil
}

// TestRecoveryWriterFailureOverridesPartialRetry drives the departure, the
// recovery worker, reconstruction, the destination worker and the fail-close
// path. Three owed keys are decided in one admission, in the request's order:
// a withdrawal, a survivor whose source is lost on the first write, and a
// withdrawal. The survivor's key is owed again, and that never masks the
// later body, deadline or flush failure: the destination ends with Cease or
// transport closure, and no later section escapes.
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
			source.recvCtxID = ctxID
			destination.sendingInitialRoutes.Store(0)
			session := destination.currentSession()
			session.setSendCtxID(ctxID)
			// Seed every failed-source key before arming transport failure
			// and the onWrite source-loss hook.
			// RFC 4271 Section 4.3.
			var seed [][]byte
			for _, raw := range [][]byte{{24, 192, 0, 2}, {24, 192, 0, 3}, {24, 192, 0, 4}} {
				seed = append(seed, ownershipRailBody(family.IPv4Unicast, false, raw, 0))
			}
			failed := recoveryAdvertiseFailed(t, r, destination, seed...)
			require.Len(t, recoveryWrittenUpdates(t, seedConn.written()), 3)
			recoveryOwnerStart(t, r, destination)
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
			var lookups recoveryLookups
			owner := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
				if lookups.add(request) > 1 {
					return nil, errors.New("hard write failure incorrectly requested another election")
				}
				if len(request.Keys) != 3 {
					return nil, errors.New("the three owed keys were not decided in one lookup")
				}
				outcomes := recoveryWithdrawAnswer(request)
				outcomes[1] = recoverySurvivorAnswer(request, nil, ribevents.RecoveryRoute{
					Source: source.Settings().Address, NLRI: request.Keys[1].NLRI,
					Attributes: attrs, NextHop: []byte{1, 1, 1, 1}, MessageID: 81})[1]
				return outcomes, nil
			})
			t.Cleanup(owner.Close)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			departed := make(chan error, 1)
			go func() {
				departed <- (&reactorAPIAdapter{r: r}).recoverNLRIBatch(ctx,
					recoveryDepartureBatch(failed.Settings().Address, "192.0.2.0/24", "192.0.3.0/24", "192.0.4.0/24"),
					[]*Peer{destination}, plugin.ProcessSender("bgp-rs"))
			}()
			lowEventually(t, session.tearingDown.Load, "the failed write retires the destination session")
			// A failed body or flush resets the table at once; a failed
			// deadline leaves that to runOnce, which this fake session lacks.
			cancel()
			if err := <-departed; err != nil {
				require.ErrorIs(t, err, context.Canceled)
			}
			r.fwdPool.Stop()
			requests := lookups.all()
			require.Len(t, requests, 1, "hard failure wins over re-election")
			admission := <-admitted
			require.Equal(t, mode != "deadline", admission.keys[1].outcome == recoveryKeyReowed,
				"the lost survivor's key is owed again once its source is gone")
			require.ErrorIs(t, admission.err, failure, "the actual transport failure is the admission's result")
			select {
			case <-admission.done:
			default:
				t.Fatal("the admission never completed")
			}
			require.Empty(t, admission.items, "all staged ownership is released")
			require.Equal(t, int32(3), released.Load())
			lowEventually(t, conn.closed.Load, "the affected recipient transport must be retired")
			raw := conn.written()
			if mode != "deadline" {
				require.GreaterOrEqual(t, len(raw), conn.firstFrame)
				written := recoveryWrittenUpdates(t, raw[:conn.firstFrame])
				require.Len(t, written, 1)
				require.Equal(t, requests[0].Keys[0].NLRI, written[0].WithdrawnRoutes)
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

// TestRecoveryRetryOwnsReplacementSession proves a failed recovery attempt
// retires only the destination session its owed record names. Method: the
// departed source owns P at the original session; the lookup swaps the
// destination to an unrelated reconnect and fails. The original session is
// retired with Cease; the reconnect it never owned receives nothing and stays
// usable. Owed records are per destination session and a table reset
// completes them, so no attempt ever starts on a replacement session.
func TestRecoveryRetryOwnsReplacementSession(t *testing.T) {
	r, _, destination, oldConn, ctxID := newSyncOrderRail(t)
	r.clock = clock.RealClock{}
	destination.sendingInitialRoutes.Store(0)
	failed := recoveryAdvertiseFailed(t, r, destination, syncOrderAnnounceBody)
	setup := len(oldConn.written())
	original := destination.currentSession()
	recoveryOwnerStart(t, r, destination)
	unrelated, unrelatedConn := newSyncOrderDest(t, bgpctx.Registry.Get(ctxID), ctxID)
	failure := errors.New("replacement attempt lookup failed")
	owner := ribevents.PublishRecovery(func(ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
		destination.mu.Lock()
		defer destination.mu.Unlock()
		destination.session = unrelated.session
		return nil, failure
	})
	t.Cleanup(owner.Close)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- (&reactorAPIAdapter{r: r}).recoverNLRIBatch(ctx,
			recoveryDepartureBatch(failed.Settings().Address, "192.0.2.0/24"),
			[]*Peer{destination}, plugin.ProcessSender("bgp-rs"))
	}()
	lowEventually(t, func() bool { return original.tearingDown.Load() },
		"the failed attempt retires the session its record names")
	cancel() // The fake session has no runOnce to reset its table.
	require.ErrorIs(t, <-done, context.Canceled, "canceling stops only the waiter")
	// The teardown seals the session on its way to the wire (Session.seal), so
	// the flag can be seen before the Cease reaches the transport.
	lowEventually(t, func() bool { return len(oldConn.written()) >= setup+message.HeaderLen+2 },
		"the Cease reaches the original session's transport")
	raw := oldConn.written()[setup:]
	require.GreaterOrEqual(t, len(raw), message.HeaderLen+2)
	require.Equal(t, byte(3), raw[18])
	require.Equal(t, byte(message.NotifyCease), raw[19])
	require.Equal(t, message.NotifyCeaseOutOfResources, raw[20])
	require.Empty(t, unrelatedConn.written())
	require.False(t, unrelated.session.tearingDown.Load(), "unadmitted reconnect must remain usable")
	unrelated.session.writeMu.Lock()
	err := unrelated.session.writeRawUpdateBody(syncOrderWithdrawBody)
	if err == nil {
		err = unrelated.session.flushWrites()
	}
	unrelated.session.writeMu.Unlock()
	require.NoError(t, err)
	require.Len(t, recoveryWrittenUpdates(t, unrelatedConn.written()), 1)
}

// recoverySurvivorAnswer decides every requested key with one survivor that
// replaces the destination's advertisement sent.
func recoverySurvivorAnswer(request ribevents.RecoveryRequest, sent []byte, survivor ribevents.RecoveryRoute) []ribevents.RecoveryOutcome {
	outcomes := make([]ribevents.RecoveryOutcome, len(request.Keys))
	for i := range request.Keys {
		outcomes[i] = ribevents.RecoveryOutcome{NLRI: request.Keys[i].NLRI,
			Survivors: []ribevents.RecoveryRoute{survivor}, Sent: [][]byte{sent}}
	}
	return outcomes
}

// recoveryWithdrawAnswer decides every requested key with WITHDRAW: no
// candidate survives. A non-ADD-PATH key withdraws its record's advertisement.
func recoveryWithdrawAnswer(request ribevents.RecoveryRequest) []ribevents.RecoveryOutcome {
	outcomes := make([]ribevents.RecoveryOutcome, len(request.Keys))
	for i := range request.Keys {
		outcomes[i] = ribevents.RecoveryOutcome{NLRI: request.Keys[i].NLRI, Withdraw: true}
	}
	return outcomes
}

// recoveryDepartureRequest names one departed source's keys at a destination.
func recoveryDepartureRequest(source, destination netip.Addr, fam family.Family, keys ...[]byte) ribevents.RecoveryRequest {
	request := ribevents.RecoveryRequest{Destination: destination, Family: fam}
	for _, raw := range keys {
		request.Keys = append(request.Keys, ribevents.RecoveryKey{NLRI: raw, Excluded: []netip.Addr{source}})
	}
	return request
}
