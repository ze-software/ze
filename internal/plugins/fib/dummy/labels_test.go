// VALIDATES: fib-dummy MPLS label refusal, foreign replay isolation, targeted
// label replay request on open, and refusal text integration with
// mplsfib.Refusals (AC-9, A-12).
// PREVENTS: fib-dummy acknowledging label installs as successful, interfering
// with another writer's targeted replay, missing pre-existing label entries on
// open, or returning refusal text without what/why/fix.
package fibdummy

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/replay"
)

// labelTestBus delivers emits synchronously to in-process subscribers,
// matching the daemon EventBus dispatch semantics for mplsfib.
type labelTestBus struct {
	mu     sync.Mutex
	nextID int
	subs   map[string]map[int]func(any)
}

func newLabelTestBus() *labelTestBus {
	return &labelTestBus{subs: make(map[string]map[int]func(any))}
}

func (b *labelTestBus) Emit(namespace, eventType string, payload any) (int, error) {
	key := namespace + "/" + eventType
	b.mu.Lock()
	handlers := make([]func(any), 0, len(b.subs[key]))
	for _, h := range b.subs[key] {
		handlers = append(handlers, h)
	}
	b.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
	return len(handlers), nil
}

func (b *labelTestBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	key := namespace + "/" + eventType
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	if b.subs[key] == nil {
		b.subs[key] = make(map[int]func(any))
	}
	b.subs[key][id] = handler
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		delete(b.subs[key], id)
		b.mu.Unlock()
	}
}

func TestLabelBatchIsRefused(t *testing.T) {
	prev := metricsPtr.Load()
	t.Cleanup(func() { metricsPtr.Store(prev) })

	reg := newTrackingMetricsRegistry()
	SetMetricsRegistry(reg)

	bus := newLabelTestBus()
	f := newDummyFIB(time.Now().UTC())
	t.Cleanup(mplsfib.EntryChange.Subscribe(bus, f.handleMPLSEntry))

	entries := []mplsfib.Entry{
		{
			Action:  mplsfib.ActionAdd,
			Op:      mplsfib.OpPop,
			InLabel: 16001,
			FEC:     netip.MustParsePrefix("10.0.0.1/32"),
			Source:  2,
		},
		{
			Action:    mplsfib.ActionAdd,
			Op:        mplsfib.OpPush,
			FEC:       netip.MustParsePrefix("10.0.0.2/32"),
			OutLabels: []uint32{16002},
			NextHop:   netip.MustParseAddr("192.0.2.1"),
			Source:    2,
		},
	}

	err := mplsfib.Apply(bus, entries)
	require.Error(t, err)
	assert.ErrorIs(t, err, errFIBDummyRefusesLabels)
	assert.Equal(t, uint64(2), f.counters().LabelEntriesRefused)
	assert.Equal(t, float64(2), reg.counters["ze_fibdummy_label_entries_refused_total"].value())
}

func TestLabelReplayForAnotherWriterIsLeftAlone(t *testing.T) {
	bus := newLabelTestBus()
	f := newDummyFIB(time.Now().UTC())
	t.Cleanup(mplsfib.EntryChange.Subscribe(bus, f.handleMPLSEntry))

	otherToken := mplsfib.NewReplayToken()
	require.NotEqual(t, f.labelReplayToken, otherToken)

	acked := false
	batch := &mplsfib.EntryBatch{
		ReplayID: otherToken,
		Entries: []mplsfib.Entry{{
			Action:  mplsfib.ActionAdd,
			Op:      mplsfib.OpPop,
			InLabel: 16010,
			FEC:     netip.MustParsePrefix("10.0.0.10/32"),
			Source:  2,
		}},
		Acknowledge: func(_ error) {
			acked = true
		},
	}

	_, err := mplsfib.EntryChange.Emit(bus, batch)
	require.NoError(t, err)
	assert.False(t, acked, "fib-dummy must not acknowledge another writer's replay batch")
	assert.Equal(t, uint64(0), f.counters().LabelEntriesRefused, "fib-dummy must not count another writer's replay batch")
}

func TestLabelReplayRequestedOnOpen(t *testing.T) {
	bus := newLabelTestBus()
	f := newDummyFIB(time.Now().UTC())

	var seenTokens []uint64
	var replayErr error
	t.Cleanup(mplsfib.ReplayRequest.Subscribe(bus, func(req *replay.Request) {
		if req == nil {
			return
		}
		seenTokens = append(seenTokens, req.ReplayID)
		replayErr = mplsfib.ApplyReplay(bus, req.ReplayID, []mplsfib.Entry{
			{
				Action:  mplsfib.ActionAdd,
				Op:      mplsfib.OpPop,
				InLabel: 16020,
				FEC:     netip.MustParsePrefix("10.0.0.20/32"),
				Source:  2,
			},
		})
	}))

	unsub, err := f.subscribeAndRequestLabels(bus)
	require.NoError(t, err)
	t.Cleanup(unsub)

	require.Len(t, seenTokens, 1, "fib-dummy must emit one targeted label replay request on open")
	assert.True(t, replay.IsReplay(seenTokens[0]), "token must be non-zero")
	assert.NotEqual(t, replay.Broadcast, seenTokens[0], "token must not be replay.Broadcast")
	assert.Equal(t, f.labelReplayToken, seenTokens[0])
	require.Error(t, replayErr)
	assert.ErrorIs(t, replayErr, errFIBDummyRefusesLabels)
	assert.Equal(t, uint64(1), f.counters().LabelEntriesRefused)
}

func TestLabelRefusalTextCarriesWhatWhyFix(t *testing.T) {
	const want = "fib dummy programs no label, so this entry is not installed; delete fib dummy, or remove the label protocol"
	assert.Equal(t, want, errFIBDummyRefusesLabels.Error())

	var refusals mplsfib.Refusals
	entry := mplsfib.Entry{
		Action:  mplsfib.ActionAdd,
		Op:      mplsfib.OpPop,
		InLabel: 16030,
		FEC:     netip.MustParsePrefix("10.0.0.30/32"),
		Source:  2,
	}
	refusals.Record(&entry, errFIBDummyRefusesLabels)

	findings := refusals.Findings("ldp", "fix the writer's cause named in the message")
	require.Len(t, findings, 1)
	assert.Equal(t, mplsfib.CodeDoctorNoWriter, findings[0].Code)
	assert.Equal(t, string(diagnostic.SeverityError), findings[0].Severity)
	assert.Contains(t, findings[0].Message, want)

	// Verify fib-dummy's two diagnostic codes are registered in diagnostic metadata.
	for _, code := range []string{codeDoctorHoldsRoutes, codeDoctorInvalidChange} {
		assert.NotNil(t, diagnostic.Lookup(code), "diagnostic code %q must be registered", code)
	}
}
