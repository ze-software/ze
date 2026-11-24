// VALIDATES: spec-fib-dummy AC-22, AC-24, R-14, R-15 -- BGP EPE targeted label
// replay behavior both before and after acquiring its label source, reload-window
// isolation from a displaced refusing writer, recovery of refused labels without
// duplicate installs or forwarding flaps, and post-stop snapshot advertisement.
// PREVENTS: EPE missing cleanup on initial replay, sending live cleanup to a
// displaced fib-dummy, dropping refused labels on replay, re-applying installed
// labels on the next publish, or installing a label while an old label awaits
// removal.
package epe

import (
	"errors"
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/ze"
)

// recordingEPEWriter stands in for a label writer on the production bus,
// recording every batch addressed to its token and acknowledging with refusal.
type recordingEPEWriter struct {
	mu      sync.Mutex
	held    uint64
	refusal error
	batches []mplsfib.EntryBatch
}

func (w *recordingEPEWriter) handle(batch *mplsfib.EntryBatch) {
	if batch == nil {
		return
	}
	if !batch.Addresses(w.held) {
		return
	}
	w.mu.Lock()
	w.batches = append(w.batches, mplsfib.EntryBatch{
		Entries:  slices.Clone(batch.Entries),
		ReplayID: batch.ReplayID,
	})
	err := w.refusal
	w.mu.Unlock()
	if batch.Acknowledge != nil {
		batch.Acknowledge(err)
	}
}

func (w *recordingEPEWriter) snapshotBatches() []mplsfib.EntryBatch {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.batches)
}

func makeEPESession(peer netip.Addr) epeSession {
	return epeSession{
		local:        linkstateevents.NodeID{ASN: 65000, BGPRouterID: netip.MustParseAddr("192.0.2.254")},
		remote:       linkstateevents.NodeID{ASN: 65001, BGPRouterID: peer},
		localAddress: netip.MustParseAddr("192.0.2.254"),
	}
}

func subscribeSnapshots(t *testing.T, bus ze.EventBus) *[]linkstateevents.Snapshot {
	t.Helper()
	var mu sync.Mutex
	var snapshots []linkstateevents.Snapshot
	t.Cleanup(epeSnapshots.Subscribe(bus, func(snap *linkstateevents.Snapshot) {
		if snap == nil {
			return
		}
		mu.Lock()
		snapshots = append(snapshots, *snap)
		mu.Unlock()
	}))
	return &snapshots
}

func TestEPEReplayRunsCleanupBeforeFirstInstall(t *testing.T) {
	dummyErr := errors.New("fib dummy programs no label, so this entry is not installed; delete fib dummy, or remove the label protocol")
	dummy := &recordingEPEWriter{held: mplsfib.NewReplayToken(), refusal: dummyErr}
	bus := newEPELabelBus(t)
	unsubDummy := mplsfib.EntryChange.Subscribe(bus, dummy.handle)

	snapshots := subscribeSnapshots(t, bus)

	source := NewSource(bus)
	t.Cleanup(mplsfib.ReplayRequest.Subscribe(bus, source.ReplayLabels))

	peer1 := netip.MustParseAddr("192.0.2.1")
	peer2 := netip.MustParseAddr("192.0.2.2")
	cfg := Config{
		Base: 16000,
		Size: 100,
		Peers: map[netip.Addr]PeerConfig{
			peer1: {Index: 1},
			peer2: {Index: 2},
		},
	}
	source.sessions[peer1] = makeEPESession(peer1)
	source.sessions[peer2] = makeEPESession(peer2)

	// Boot under fib-dummy: cleanup is refused, s.acquired stays false, no add
	// is sent, and labelDoctorFindings holds every wanted label.
	require.Error(t, source.Configure(cfg))
	assert.False(t, source.acquired)
	assert.Empty(t, source.installed)
	dummyBatches := dummy.snapshotBatches()
	require.Len(t, dummyBatches, 1)
	require.Len(t, dummyBatches[0].Entries, 1)
	assert.Equal(t, mplsfib.ActionRemoveLabelSource, dummyBatches[0].Entries[0].Action)
	require.Len(t, source.labelDoctorFindings(), 2)

	// Remove dummy and test a writer that ALSO refuses cleanup: gets no add and
	// keeps both findings.
	unsubDummy()
	refusingNew := &recordingEPEWriter{held: mplsfib.NewReplayToken(), refusal: errors.New("new writer cleanup refused")}
	unsubRefusingNew := mplsfib.EntryChange.Subscribe(bus, refusingNew.handle)

	_, err := mplsfib.ReplayRequest.Emit(bus, &replay.Request{ReplayID: refusingNew.held})
	require.NoError(t, err)
	assert.False(t, source.acquired)
	refusingBatches := refusingNew.snapshotBatches()
	require.Len(t, refusingBatches, 1, "refused cleanup must block every add")
	assert.Equal(t, mplsfib.ActionRemoveLabelSource, refusingBatches[0].Entries[0].Action)
	assert.Equal(t, refusingNew.held, refusingBatches[0].ReplayID)
	require.Len(t, source.labelDoctorFindings(), 2)
	unsubRefusingNew()

	// Now subscribe an accepting writer and request a targeted replay.
	accepting := &recordingEPEWriter{held: mplsfib.NewReplayToken()}
	t.Cleanup(mplsfib.EntryChange.Subscribe(bus, accepting.handle))
	// Track any live (ReplayID == 0) batches during replay.
	liveDuringReplay := 0
	unsubLiveSpy := mplsfib.EntryChange.Subscribe(bus, func(batch *mplsfib.EntryBatch) {
		if batch == nil {
			return
		}
		if batch.ReplayID == 0 {
			liveDuringReplay++
		}
	})

	_, err = mplsfib.ReplayRequest.Emit(bus, &replay.Request{ReplayID: accepting.held})
	require.NoError(t, err)
	unsubLiveSpy()

	assert.Zero(t, liveDuringReplay, "no live (token 0) batch may be emitted while answering an unacquired replay")
	batches := accepting.snapshotBatches()
	require.Len(t, batches, 3, "cleanup batch first, then one add batch per wanted peer")
	for i, b := range batches {
		assert.Equal(t, accepting.held, b.ReplayID, "batch %d must carry asking writer's token", i)
		require.Len(t, b.Entries, 1)
	}
	assert.Equal(t, mplsfib.ActionRemoveLabelSource, batches[0].Entries[0].Action)
	assert.Equal(t, mplsfib.ActionAdd, batches[1].Entries[0].Action)
	assert.Equal(t, uint32(16001), batches[1].Entries[0].InLabel)
	assert.Equal(t, mplsfib.ActionAdd, batches[2].Entries[0].Action)
	assert.Equal(t, uint32(16002), batches[2].Entries[0].InLabel)

	assert.True(t, source.acquired)
	assert.Equal(t, uint32(16001), source.installed[peer1])
	assert.Equal(t, uint32(16002), source.installed[peer2])
	assert.Empty(t, source.labelDoctorFindings())
	require.NotEmpty(t, *snapshots)
	lastSnap := (*snapshots)[len(*snapshots)-1]
	assert.Len(t, lastSnap.Links, 2, "snapshot must advertise both peers once installed")

	// Next Configure with the same config emits neither cleanup nor add (no flap).
	before := len(accepting.snapshotBatches())
	require.NoError(t, source.Configure(cfg))
	assert.Len(t, accepting.snapshotBatches(), before, "same-config Configure after replay must not flap")
}

func TestEPEReplayCleanupNeverReachesTheReplacedWriter(t *testing.T) {
	dummyErr := errors.New("fib dummy programs no label, so this entry is not installed; delete fib dummy, or remove the label protocol")
	dummy := &recordingEPEWriter{held: mplsfib.NewReplayToken(), refusal: dummyErr}
	bus := newEPELabelBus(t)
	t.Cleanup(mplsfib.EntryChange.Subscribe(bus, dummy.handle))

	snapshots := subscribeSnapshots(t, bus)

	source := NewSource(bus)
	t.Cleanup(mplsfib.ReplayRequest.Subscribe(bus, source.ReplayLabels))

	peer := netip.MustParseAddr("192.0.2.1")
	cfg := Config{
		Base:  16000,
		Size:  100,
		Peers: map[netip.Addr]PeerConfig{peer: {Index: 1}},
	}
	source.sessions[peer] = makeEPESession(peer)

	// EPE runs unacquired under the refusing writer.
	require.Error(t, source.Configure(cfg))
	assert.False(t, source.acquired)
	dummyBefore := len(dummy.snapshotBatches())

	// Reload window: the new accepting writer subscribes and emits its replay
	// request while the refusing writer is STILL subscribed.
	newWriter := &recordingEPEWriter{held: mplsfib.NewReplayToken()}
	t.Cleanup(mplsfib.EntryChange.Subscribe(bus, newWriter.handle))

	_, err := mplsfib.ReplayRequest.Emit(bus, &replay.Request{ReplayID: newWriter.held})
	require.NoError(t, err)

	assert.Len(t, dummy.snapshotBatches(), dummyBefore,
		"replaced writer must consume and acknowledge zero batches during targeted replay")
	newBatches := newWriter.snapshotBatches()
	require.Len(t, newBatches, 2, "new writer sees tagged cleanup followed by tagged add")
	assert.Equal(t, newWriter.held, newBatches[0].ReplayID)
	assert.Equal(t, mplsfib.ActionRemoveLabelSource, newBatches[0].Entries[0].Action)
	assert.Equal(t, newWriter.held, newBatches[1].ReplayID)
	assert.Equal(t, mplsfib.ActionAdd, newBatches[1].Entries[0].Action)
	assert.Equal(t, uint32(16001), newBatches[1].Entries[0].InLabel)

	assert.True(t, source.acquired)
	assert.Equal(t, uint32(16001), source.installed[peer])
	assert.Empty(t, source.labelDoctorFindings())
	require.NotEmpty(t, *snapshots)
	assert.Len(t, (*snapshots)[len(*snapshots)-1].Links, 1)
}

func TestEPEReplayReappliesRefusedLabel(t *testing.T) {
	bus := newEPELabelBus(t)
	refusing := &recordingEPEWriter{held: mplsfib.NewReplayToken(), refusal: errors.New("refused add")}
	unsubRefusing := mplsfib.EntryChange.Subscribe(bus, refusing.handle)

	snapshots := subscribeSnapshots(t, bus)

	source := NewSource(bus)
	t.Cleanup(mplsfib.ReplayRequest.Subscribe(bus, source.ReplayLabels))
	peer := netip.MustParseAddr("192.0.2.1")
	source.config = Config{
		Base:  16000,
		Size:  100,
		Peers: map[netip.Addr]PeerConfig{peer: {Index: 1}},
	}
	source.sessions[peer] = makeEPESession(peer)
	source.acquired = true

	// Acquired EPE attempts live publish; writer refuses the PeerNode SID add.
	require.Error(t, publish(source))
	assert.Empty(t, source.installed)
	require.Len(t, source.labelDoctorFindings(), 1)
	unsubRefusing()

	// Second accepting writer opens and requests a targeted replay.
	accepting := &recordingEPEWriter{held: mplsfib.NewReplayToken()}
	t.Cleanup(mplsfib.EntryChange.Subscribe(bus, accepting.handle))

	_, err := mplsfib.ReplayRequest.Emit(bus, &replay.Request{ReplayID: accepting.held})
	require.NoError(t, err)

	batches := accepting.snapshotBatches()
	require.Len(t, batches, 1)
	assert.Equal(t, accepting.held, batches[0].ReplayID)
	require.Len(t, batches[0].Entries, 1)
	assert.Equal(t, uint32(16001), batches[0].Entries[0].InLabel)
	assert.Equal(t, peer, batches[0].Entries[0].NextHop)

	assert.Equal(t, uint32(16001), source.installed[peer])
	assert.Empty(t, source.labelDoctorFindings())
	require.NotEmpty(t, *snapshots)
	assert.Len(t, (*snapshots)[len(*snapshots)-1].Links, 1, "snapshot must advertise the newly installed SID")
}

func TestEPEReplayInstalledLabelIsNotAppliedAgain(t *testing.T) {
	bus := newEPELabelBus(t)
	refusing := &recordingEPEWriter{held: mplsfib.NewReplayToken(), refusal: errors.New("refused add")}
	unsubRefusing := mplsfib.EntryChange.Subscribe(bus, refusing.handle)

	source := NewSource(bus)
	t.Cleanup(mplsfib.ReplayRequest.Subscribe(bus, source.ReplayLabels))
	peer := netip.MustParseAddr("192.0.2.1")
	cfg := Config{
		Base:  16000,
		Size:  100,
		Peers: map[netip.Addr]PeerConfig{peer: {Index: 1}},
	}
	source.config = cfg
	source.sessions[peer] = makeEPESession(peer)
	source.acquired = true

	require.Error(t, publish(source))
	unsubRefusing()

	accepting := &recordingEPEWriter{held: mplsfib.NewReplayToken()}
	t.Cleanup(mplsfib.EntryChange.Subscribe(bus, accepting.handle))

	_, err := mplsfib.ReplayRequest.Emit(bus, &replay.Request{ReplayID: accepting.held})
	require.NoError(t, err)
	batches := accepting.snapshotBatches()
	require.Len(t, batches, 1)
	require.Len(t, batches[0].Entries, 1, "each wanted label appears once in the replay batch")

	// Next Configure with the same config must NOT re-apply the installed label.
	require.NoError(t, source.Configure(cfg))
	assert.Len(t, accepting.snapshotBatches(), 1, "label installed by replay must not be applied again")
}

func TestEPEReplaySkipsPeerAwaitingRemoval(t *testing.T) {
	bus := newEPELabelBus(t)
	writer := &recordingEPEWriter{held: mplsfib.NewReplayToken()}
	t.Cleanup(mplsfib.EntryChange.Subscribe(bus, writer.handle))

	source := NewSource(bus)
	t.Cleanup(mplsfib.ReplayRequest.Subscribe(bus, source.ReplayLabels))

	peerChanged := netip.MustParseAddr("192.0.2.1")
	peerRetiring := netip.MustParseAddr("192.0.2.2")
	peerColliding := netip.MustParseAddr("192.0.2.3")
	peerClean := netip.MustParseAddr("192.0.2.4")

	source.config = Config{
		Base: 16000,
		Size: 100,
		Peers: map[netip.Addr]PeerConfig{
			peerChanged:   {Index: 11}, // wants 16011, but old 16001 removal failed
			peerColliding: {Index: 2},  // wants 16002, occupied by peerRetiring's failed removal
			peerClean:     {Index: 4},  // wants 16004, clean
		},
	}
	source.sessions[peerChanged] = makeEPESession(peerChanged)
	source.sessions[peerColliding] = makeEPESession(peerColliding)
	source.sessions[peerClean] = makeEPESession(peerClean)
	source.acquired = true
	source.installed[peerChanged] = 16001
	source.installed[peerRetiring] = 16002

	_, err := mplsfib.ReplayRequest.Emit(bus, &replay.Request{ReplayID: writer.held})
	require.NoError(t, err)

	batches := writer.snapshotBatches()
	require.Len(t, batches, 1)
	require.Len(t, batches[0].Entries, 1, "only peerClean's label may be replayed")
	assert.Equal(t, uint32(16004), batches[0].Entries[0].InLabel)
	assert.Equal(t, peerClean, batches[0].Entries[0].NextHop)
}

// VALIDATES: spec-fib-dummy AC-24, R-15 -- a live EPE add landing during the
// reload window while both an accepting writer and a refusing fib-dummy are
// subscribed is withheld from the snapshot and recorded refused; once dummy
// unsubscribes and the remaining writer emits a targeted replay on
// WriterStopped, EPE installs/records the label, clears its finding, and emits
// the snapshot advertising the PeerNode SID.
func TestEPEReloadWindowLiveAddRecoveredAfterWriterStopped(t *testing.T) {
	bus := newEPELabelBus(t)
	kernel := &recordingEPEWriter{held: mplsfib.NewReplayToken()}
	dummyErr := errors.New("fib dummy programs no label, so this entry is not installed; delete fib dummy, or remove the label protocol")
	dummy := &recordingEPEWriter{held: mplsfib.NewReplayToken(), refusal: dummyErr}

	t.Cleanup(mplsfib.EntryChange.Subscribe(bus, kernel.handle))
	unsubDummy := mplsfib.EntryChange.Subscribe(bus, dummy.handle)
	t.Cleanup(mplsfib.WriterStopped.Subscribe(bus, func() {
		_, err := mplsfib.ReplayRequest.Emit(bus, &replay.Request{ReplayID: kernel.held})
		assert.NoError(t, err)
	}))

	snapshots := subscribeSnapshots(t, bus)

	source := NewSource(bus)
	t.Cleanup(mplsfib.ReplayRequest.Subscribe(bus, source.ReplayLabels))
	peer := netip.MustParseAddr("192.0.2.1")
	source.config = Config{
		Base:  16000,
		Size:  100,
		Peers: map[netip.Addr]PeerConfig{peer: {Index: 1}},
	}
	source.sessions[peer] = makeEPESession(peer)
	source.acquired = true

	// Live add during reload window hits both writers: kernel accepts, dummy
	// refuses, joined error keeps label out of installed and snapshot empty.
	require.ErrorIs(t, publish(source), dummyErr)
	assert.Empty(t, source.installed)
	require.Len(t, source.labelDoctorFindings(), 1)
	require.NotEmpty(t, *snapshots)
	assert.Empty(t, (*snapshots)[len(*snapshots)-1].Links, "refused SID must not be advertised")

	// Dummy unsubscribes and WriterStopped fires -> kernel issues targeted
	// replay -> EPE records label, clears finding, advertises SID.
	unsubDummy()
	_, err := mplsfib.WriterStopped.Emit(bus)
	require.NoError(t, err)

	assert.Equal(t, uint32(16001), source.installed[peer])
	assert.Empty(t, source.labelDoctorFindings())
	assert.Len(t, (*snapshots)[len(*snapshots)-1].Links, 1, "post-stop replay must advertise the PeerNode SID")
}
