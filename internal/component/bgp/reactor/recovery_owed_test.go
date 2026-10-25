// Design: docs/architecture/plugin/rib-storage-design.md -- a failed receive publication ends its source session
// Related: peer_session_end.go -- failReceivePublication
// Related: peer_run_test.go -- downOrderObserver
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
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
