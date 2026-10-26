// Design: docs/architecture/plugin/rib-storage-design.md -- a failed receive publication ends its source session
// Related: peer_session_end.go -- failReceivePublication
// Related: peer_run_test.go -- downOrderObserver
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// publicationPanicReceiver panics on every batch, as a defective receiver
// would. The delivery worker recovers it.
type publicationPanicReceiver struct{ MessageReceiver }

func (publicationPanicReceiver) OnMessageBatchReceived(*plugin.PeerInfo, []bgptypes.RawMessage) []int {
	panic("BUG: test receiver panics on delivery")
}

// TestOwedKeyPublicationFailureTearsDownSource is the Phase 2b teardown half:
// a receive publication that fails, through a panicking receiver or a nil
// one, ends the source's session with Cease / Out of Resources, and the
// session's down still arrives. The delivery worker calls the teardown
// itself, so a teardown that waited on the worker would hang until the
// deadline (A-11). The GR-capable half lands with D1's reason producer.
//
// VALIDATES: AC-17 teardown, A-11 (no wait on deliveryDone).
// PREVENTS: a source left Established while its UPDATEs never reach the plugins.
func TestOwedKeyPublicationFailureTearsDownSource(t *testing.T) {
	// The worker's seam hands it the failing receiver; the read goroutine
	// still reads the real one, so the UPDATE reaches the worker.
	cases := []struct {
		name     string
		receiver func(MessageReceiver) MessageReceiver
	}{
		{"panicking receiver", func(prior MessageReceiver) MessageReceiver {
			return publicationPanicReceiver{MessageReceiver: prior}
		}},
		{"nil receiver", func(MessageReceiver) MessageReceiver { return nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peers := lowLiveRouter(t, lowLiveSettings("10.0.0.1", 65000, 65002))
			v := peers[0]
			p := v.peer
			r := p.reactor
			observer := &downOrderObserver{downAdded: make(chan struct{}, 1)}
			r.addPeerObserver(observer)
			publication := p.receivePublication.Load()
			p.sessionEndHooks.Store(&sessionEndHooks{deliveryReceiver: tc.receiver})

			attrs := firstASAttrs(4, 65002)                                            // RFC 6793 Section 3.
			ownershipLiveSend(t, v, nil, attrs, netip.MustParsePrefix("10.20.0.0/24")) // RFC 4271 Section 4.3.

			// RFC 4486 Section 4: Cease (6) with subcode Out of Resources (8).
			lowEventually(t, func() bool { return publicationCeaseSent(v) }, "Cease / Out of Resources on the wire")
			lowEventually(t, func() bool { return p.State() != PeerStateEstablished }, "source left Established")
			lowEventually(t, func() bool { return len(observer.records()) == 1 }, "the source's down")
			if !publication.failed.Load() {
				t.Fatal("the publication that tore the session down is not marked failed")
			}
		})
	}
}

// publicationCeaseSent reports whether the remote received a NOTIFICATION
// Cease / Out of Resources.
//
// RFC 4271 Section 4.5: the NOTIFICATION body is Error code (1 octet), Error
// subcode (1 octet), Data (variable).
//
//	Byte offset: | 0..15 Marker | 16..17 Length | 18 Type | 19 Error code | 20 Error subcode |
func publicationCeaseSent(v *lowLivePeer) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, frame := range v.frames {
		if len(frame) < message.HeaderLen+2 {
			continue
		}
		if frame[18] != byte(msgtype.TypeNOTIFICATION) {
			continue
		}
		if frame[19] == byte(message.NotifyCease) && frame[20] == message.NotifyCeaseOutOfResources {
			return true
		}
	}
	return false
}

// owedSurvivorMessage is owedSurvivor with the survivor's received message ID
// chosen by the caller.
func owedSurvivorMessage(source netip.Addr, p []byte, message uint64) owedAnswer {
	attributes := []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00, 0x40, 0x03, 0x04, 192, 0, 2, 1}
	return func(request ribevents.RecoveryRequest) []ribevents.RecoveryOutcome {
		return recoverySurvivorAnswer(request, p, ribevents.RecoveryRoute{Source: source,
			NLRI: p, Attributes: attributes, NextHop: []byte{192, 0, 2, 1}, MessageID: message})
	}
}

// TestRecoveryAnswerMatchingEntryWritesNothing proves the "already holds it"
// test of the admission (Data Flow 3a and 8). Method: the source's route for
// P is installed at the destination, then the source's own withdrawal owes P
// and the RIB's answer elects a survivor of the same source. A survivor with
// the entry's received message ID is DECIDED WITH NOTHING TO WRITE; one with
// another message ID from the same identity is written. An entry installed by
// an initial-replay write compares with the received message ID its batch
// carried (initial-source-message), so electing that revision writes nothing.
//
// VALIDATES: Data Flow 3a last bullet; round 11, I-2.
// PREVENTS: an UPDATE re-sending the route the client already holds, and an
// initial-replay entry (message 0) that no election can match.
func TestRecoveryAnswerMatchingEntryWritesNothing(t *testing.T) {
	p := syncOrderPrefixWire // syncOrderPrefix, which the initial replay installs.
	cases := []struct {
		name      string
		initial   bool
		installed uint64
		elected   uint64
		writes    bool
	}{
		{name: "same-revision-writes-nothing", installed: 7, elected: 7},
		{name: "other-revision-is-written", installed: 7, elected: 8, writes: true},
		{name: "initial-replay-revision-writes-nothing", initial: true, installed: 81, elected: 81},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rail := newOwedRail(t)
			if tc.initial {
				owedRailInitialReplay(t, rail, tc.installed)
			} else {
				adjOutTestForward(t, rail.destination, rail.source, p, tc.installed)
			}
			entry, held := adjOutTestEntry(rail.destination)
			require.True(t, held)
			require.Equal(t, tc.installed, entry.owner.message)
			before := len(recoveryWrittenUpdates(t, rail.conn.written()))

			rail.forward(t, true, p) // The owner withdraws P: owed.
			request := rail.request(t)
			require.Equal(t, p, request.Keys[0].NLRI)
			rail.release <- owedSurvivorMessage(rail.source.Settings().Address, p, tc.elected)
			lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the round decided P")
			written := recoveryWrittenUpdates(t, rail.conn.written())[before:]
			if !tc.writes {
				require.Empty(t, written, "the elected route is the one the destination holds")
				return
			}
			require.Len(t, written, 1, "another revision of the same source is written")
			require.Equal(t, p, written[0].NLRI)
		})
	}
}

// owedRailInitialReplay installs syncOrderPrefix at the rail's destination
// through a forwarded initial replay of the rail's source, as bgp-rib's
// replayRoutesWithCursor sends one, naming received message message.
func owedRailInitialReplay(t *testing.T, rail *owedRail, message uint64) {
	t.Helper()
	rail.r.config = &Config{LocalAS: 65000}
	session := rail.destination.currentSession()
	session.writeMu.Lock()
	session.initialReplay = 5
	session.writeMu.Unlock()
	rail.destination.initialSyncEOROwed.Store(true)
	batch := adjOutBatch(syncOrderPrefix, "10.0.0.1")
	batch.Replay = true
	batch.SentOwnerMessage = 91
	batch.InitialReplay = 5
	batch.InitialSourcePeer = rail.source.Settings().Address.String()
	batch.InitialSourceID = uint32(rail.source.SourceID())
	batch.InitialSourceOwner = rail.source.sourceOwner
	batch.InitialSourceMessage = message
	api := &reactorAPIAdapter{r: rail.r}
	sel := selector.ParseDefault(rail.destination.Settings().Address.String())
	require.NoError(t, api.AnnounceNLRIBatch(t.Context(), sel, batch, plugin.OperatorSender()))
}

// owedRailEpoch reads the epoch of the rail's only owed record.
func owedRailEpoch(t *testing.T, rail *owedRail) uint64 {
	t.Helper()
	table := &rail.destination.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	require.Len(t, table.owed, 1)
	for _, record := range table.owed {
		return record.epoch
	}
	return 0
}

// TestOwedKeyCurrentAnnouncerNotExcluded proves AC-18 under Q-3. Method: the
// source A's route for P is installed at C, then A's session moves on and
// A's departure and A's re-announcement on its current session reach C's
// writer, in both orders, while a round is held. The epoch moves when the
// excluded A enters the current-announcer set, the held answer is refused,
// and the next round does not exclude A. After the announcing session in
// turn departs, the round excludes A again.
//
// VALIDATES: adjOutOwed.electableExcluded, noteAnnouncer's epoch move.
// PREVENTS: a departure of A's old session hiding A's live route from the
// election, so C is given another source's worse route or a withdrawal.
func TestOwedKeyCurrentAnnouncerNotExcluded(t *testing.T) {
	p := []byte{24, 192, 0, 2}
	for _, departureFirst := range []bool{true, false} {
		name := "reannouncement-then-departure"
		if departureFirst {
			name = "departure-then-reannouncement"
		}
		t.Run(name, func(t *testing.T) {
			rail := newOwedRail(t)
			other := adjOutTestOtherSource(rail.r, rail.source)
			attachRecoveryOwner(t, rail.r, rail.source, other)
			address := rail.source.Settings().Address
			adjOutTestForward(t, rail.destination, rail.source, p, 11)
			rail.source.advanceForwardGeneration() // A's first session ended; A is up again.
			var departures []chan error
			if departureFirst {
				departures = append(departures, rail.departure(t, address, "192.0.2.0/24"))
				require.Equal(t, []netip.Addr{address}, rail.request(t).Keys[0].Excluded,
					"the departed session's source is excluded")
				epoch := owedRailEpoch(t, rail)
				adjOutTestForward(t, rail.destination, rail.source, p, 12)
				require.NotEqual(t, epoch, owedRailEpoch(t, rail),
					"an excluded source entering the current-announcer set moves the epoch")
			} else {
				adjOutTestForward(t, rail.destination, other, p, 21)
				rail.request(t)
				adjOutTestForward(t, rail.destination, rail.source, p, 12)
				epoch := owedRailEpoch(t, rail)
				departures = append(departures, rail.departure(t, address, "192.0.2.0/24"))
				lowEventually(t, func() bool { return owedRailEpoch(t, rail) != epoch }, "the departure merges")
			}
			written := len(recoveryWrittenUpdates(t, rail.conn.written()))
			rail.release <- nil // The held round answered a stale question.
			require.Empty(t, rail.request(t).Keys[0].Excluded,
				"A's announcing session is current, so A is electable")
			require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), written,
				"an admission for a stale epoch writes nothing")

			rail.source.advanceForwardGeneration() // The announcing session departs.
			departures = append(departures, rail.departure(t, address, "192.0.2.0/24"))
			epoch := owedRailEpoch(t, rail)
			lowEventually(t, func() bool { return owedRailEpoch(t, rail) != epoch }, "the second departure merges")
			rail.release <- nil
			require.Equal(t, []netip.Addr{address}, rail.request(t).Keys[0].Excluded,
				"once its announcing session departs, A is excluded again")
			rail.release <- nil
			for _, done := range departures {
				require.NoError(t, departureResult(t, done))
			}
			require.Empty(t, rail.owed())
		})
	}
}

// TestOwedKeyAnnounceMoverWritesAndReowes proves AC-26 and the R-9
// announcement half. Method: A's route for P is installed at C, B's
// announcement owes P and the round is held, then an ANNOUNCE mover reaches
// C's writer before the RIB answers. Whatever the answer (the mover's own
// source elected, another source elected, the route C holds elected, or
// WITHDRAW), the admission's write happens or is decided, the record stays
// with its epoch moved, and the next round starts with no further UPDATE.
//
// VALIDATES: recoveryAdmission.settle's RE-OWED BY A MOVER branch.
// PREVENTS: a newer, better route lost because the round that raced it
// removed the record (round 11, B-2).
func TestOwedKeyAnnounceMoverWritesAndReowes(t *testing.T) {
	p := []byte{24, 192, 0, 2}
	cases := []struct {
		name   string
		mover  bool // true: the mover is A, false: B.
		answer func(a, b netip.Addr) owedAnswer
		writes int
	}{
		{name: "non-chosen-mover-written-reowed", mover: true,
			answer: func(_, b netip.Addr) owedAnswer { return owedSurvivorMessage(b, p, 21) }, writes: 1},
		{name: "chosen-mover-written-reowed", mover: true,
			answer: func(a, _ netip.Addr) owedAnswer { return owedSurvivorMessage(a, p, 13) }, writes: 1},
		{name: "held-choice-nothing-written-reowed", mover: false,
			answer: func(a, _ netip.Addr) owedAnswer { return owedSurvivorMessage(a, p, 11) }, writes: 0},
		{name: "withdraw-answer-written-reowed", mover: false,
			answer: func(_, _ netip.Addr) owedAnswer { return nil }, writes: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rail := newOwedRail(t)
			other := adjOutTestOtherSource(rail.r, rail.source)
			attachRecoveryOwner(t, rail.r, rail.source, other)
			adjOutTestForward(t, rail.destination, rail.source, p, 11)
			adjOutTestForward(t, rail.destination, other, p, 21)
			rail.request(t) // The round is held.
			if tc.mover {
				adjOutTestForward(t, rail.destination, rail.source, p, 13)
			} else {
				adjOutTestForward(t, rail.destination, other, p, 22)
			}
			epoch := owedRailEpoch(t, rail)
			before := len(recoveryWrittenUpdates(t, rail.conn.written()))
			rail.release <- tc.answer(rail.source.Settings().Address, other.Settings().Address)
			// No UPDATE reaches the writer from here: the next round is the
			// mover's own re-owe.
			request := rail.request(t)
			require.Equal(t, p, request.Keys[0].NLRI)
			require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), before+tc.writes)
			require.NotEqual(t, epoch, owedRailEpoch(t, rail), "RE-OWED moves the epoch and keeps the record")
			rail.release <- nil
			lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the next round decided P")
		})
	}
	t.Run("churn-stream", func(t *testing.T) {
		// AC-22 (e), round 11, N-4: A's withdrawal is owed, and B
		// re-announces P with no pause, one mover reaching C's writer inside
		// every held round. Each admission writes the revision the RIB
		// elected and is RE-OWED by the mover, so no admission ends without
		// a re-owe and none is LEFT OWED without a write; once B stops, C
		// holds B's last revision.
		const churn = 1000
		rail := newOwedRail(t)
		other := adjOutTestOtherSource(rail.r, rail.source)
		attachRecoveryOwner(t, rail.r, rail.source, other)
		adjOutTestForward(t, rail.destination, rail.source, p, 11)
		adjOutTestForward(t, rail.destination, other, p, 21)
		rail.forward(t, true, p) // A's withdrawal merges into the owed record.
		rail.request(t)
		message := uint64(21)
		for i := range churn {
			answered := message
			message = 100 + uint64(i)
			adjOutTestForward(t, rail.destination, other, p, message) // B's mover.
			rail.release <- owedSurvivorMessage(other.Settings().Address, p, answered)
			// RE-OWED: the next round starts with no further UPDATE.
			require.Equal(t, p, rail.request(t).Keys[0].NLRI, "admission %d was re-owed", i)
		}
		rail.release <- owedSurvivorMessage(other.Settings().Address, p, message)
		lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the round after the churn decided P")
		require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), churn+2,
			"A's route, then one write per admission: no admission retried without a decision")
		entry, held := adjOutTestEntry(rail.destination)
		require.True(t, held)
		require.Equal(t, other, entry.owner.source, "C holds B's route")
		require.Equal(t, message, entry.owner.message, "C holds B's last revision")
	})
}

// TestOwnerWithdrawalOwedWhileCandidateInFlight proves round 12, I-1 at C's
// writer. Method: C holds A's uncontested route for P with a recovery owner
// registered. A second candidate is in the RIB but its forward has not
// reached C: held back (in flight) or never coming (dropped on a generation
// move while GR keeps the path). A's owner withdrawal, or a lifecycle
// removal, is owed; the round elects the candidate and C's wire carries it
// and no withdrawal of P. An in-flight forward released later meets the
// elected entry, whose bit is set, is owed, and the next round writes
// nothing.
//
// VALIDATES: owesWithdrawal owes every source-owned removal, contested or
// not, while a recovery owner is registered.
// PREVENTS: a bare withdrawal first, losing the route C needs (red against
// round 11's bit-clear owner withdrawal written bare).
func TestOwnerWithdrawalOwedWhileCandidateInFlight(t *testing.T) {
	p := []byte{24, 192, 0, 2}
	for _, inFlight := range []bool{true, false} {
		for _, lifecycle := range []bool{false, true} {
			name := "dropped-on-generation-move"
			if inFlight {
				name = "in-flight"
			}
			if lifecycle {
				name += "-lifecycle-removal"
			}
			t.Run(name, func(t *testing.T) {
				rail := newOwedRail(t)
				other := adjOutTestOtherSource(rail.r, rail.source)
				attachRecoveryOwner(t, rail.r, rail.source, other)
				adjOutTestForward(t, rail.destination, rail.source, p, 11)
				if !inFlight {
					other.advanceForwardGeneration() // X restarted: its queued forward was dropped.
				}
				if lifecycle {
					owedRailExpected(t, rail.destination, ownershipRailBody(family.IPv4Unicast, true, p, 0), rail.revision(t))
				} else {
					rail.forward(t, true, p)
				}
				require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1, "the removal is owed, never bare")
				rail.request(t)
				rail.release <- owedSurvivorMessage(other.Settings().Address, p, 21)
				lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the round elected the candidate")
				if inFlight {
					adjOutTestForward(t, rail.destination, other, p, 21) // The held forward arrives.
					rail.request(t)
					rail.release <- owedSurvivorMessage(other.Settings().Address, p, 21)
					lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the late forward's round decided P")
				}
				written := recoveryWrittenUpdates(t, rail.conn.written())
				require.Len(t, written, 2, "C's wire: A's route, then the candidate's, once")
				require.Equal(t, p, written[1].NLRI)
				for _, update := range written {
					require.Empty(t, update.WithdrawnRoutes, "C's wire carries no withdrawal of P")
				}
				entry, _ := adjOutTestEntry(rail.destination)
				require.Same(t, other, entry.owner.source)
			})
		}
	}
}

// owedFramingThreeLabels is ownershipRailNative's announcement of fam with a
// three-label stack, the bottom label's S bit set.
func owedFramingThreeLabels(fam family.Family) []byte {
	one := ownershipRailNative(fam, 0, 1, false, 0)[4:]
	out := []byte{one[0] + 48, 0, 0x10, 0, 0, 0x20, 0}
	return append(out, one[1:]...) // The original label, S set, then RD and prefix.
}

// TestOwedKeyAnnouncementWithdrawFraming proves round 11, I-6 and AC-22 (g).
// Method: at C, a labeled route of the source with a THREE-label stack is
// installed, then another source's announcement of the same route owes the
// key. The record holds the withdrawal framing: the request names the
// announced route, and the RIB's WITHDRAW answer reaches C's wire as an
// MP_UNREACH_NLRI whose NLRI is exactly length, Compatibility 0x800000, RD
// (SAFI 128) and prefix, with none of the three labels. For IPv4 unicast
// the record's copy equals the announced NLRI.
//
// VALIDATES: stageAnnouncementOwe's conversion through
// nlrisplit.WithdrawalNLRIFromKey, for SAFI 4 and SAFI 128.
// PREVENTS: a WITHDRAW answer copying an announcement's label stack into
// MP_UNREACH_NLRI, which FRR rejects with a session reset.
func TestOwedKeyAnnouncementWithdrawFraming(t *testing.T) {
	for _, fam := range []family.Family{
		{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel},
		{AFI: family.AFIIPv4, SAFI: family.SAFIVPN},
		family.IPv4Unicast,
	} {
		t.Run(fam.String(), func(t *testing.T) {
			rail := newOwedRail(t)
			other := adjOutTestOtherSource(rail.r, rail.source)
			attachRecoveryOwner(t, rail.r, rail.source, other)
			rail.destination.negotiated.Load().families[fam] = true
			announce := ownershipRailNative(fam, 0, 1, false, 0)[4:]
			withdraw := announce
			if fam.SAFI != family.SAFIUnicast {
				announce = owedFramingThreeLabels(fam)
				withdraw = ownershipRailNative(fam, 0, 1, true, 0x80)[4:]
			}
			require.NoError(t, ownershipWriterForward(t, rail.destination, rail.source, ownershipRailBody(fam, false, announce, 0), false))
			require.NoError(t, ownershipWriterForward(t, rail.destination, other, ownershipRailBody(fam, false, announce, 0), false))
			require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1, "the second source's route is owed")

			request := rail.request(t)
			require.Len(t, request.Keys, 1)
			var askedScratch, announcedScratch [nlrisplit.PrefixKeyScratchSize]byte
			asked, err := nlrisplit.GetPrefixKey(fam)(request.Keys[0].NLRI, askedScratch[:], false)
			require.NoError(t, err, "the RIB reads the request's NLRI as an announcement")
			announced, err := nlrisplit.GetPrefixKey(fam)(announce, announcedScratch[:], false)
			require.NoError(t, err)
			require.Equal(t, announced, asked, "the request names the announced route")
			rail.release <- nil

			lowEventually(t, func() bool {
				return len(recoveryWrittenUpdates(t, rail.conn.written())) == 2
			}, "the RIB's WITHDRAW decision is written")
			update := recoveryWrittenUpdates(t, rail.conn.written())[1]
			if fam == family.IPv4Unicast {
				require.Equal(t, announce, update.WithdrawnRoutes, "a unicast record's copy is the announced NLRI")
				return
			}
			require.True(t, bytes.Contains(update.PathAttributes, withdraw),
				"MP_UNREACH_NLRI carries length, 0x800000, RD and prefix")
			require.False(t, bytes.Contains(update.PathAttributes, announce[1:10]), "no label of the stack is withdrawn")
		})
	}
}
