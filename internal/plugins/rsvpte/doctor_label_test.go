package rsvpte

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/server"
	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/replay"
)

// refusingReplayWriter accepts every batch addressed to it, except a replay
// batch while refusal is set, which it acknowledges with refusal.
type refusingReplayWriter struct {
	held    uint64
	refusal error
}

func (w *refusingReplayWriter) handle(batch *mplsfibevents.EntryBatch) {
	if !batch.Addresses(w.held) {
		return
	}
	if replay.IsReplay(batch.ReplayID) {
		batch.Acknowledge(w.refusal)
		return
	}
	batch.Acknowledge(nil)
}

// TestRSVPTELabelDoctorReportsUnreplayedEntries proves the
// rsvp-te-label-entries check reports every installed entry a writer's replay
// did not restore, names the entry and the writer's cause under
// doctor-fib-mpls-no-writer with a fix, and clears once a later replay
// succeeds.
//
// VALIDATES: AC-22, AC-25 -- an LSP RSVP-TE holds up with no forwarding under
// it after a VPP reconnect reaches `show doctor` with its cause.
// METHOD: a writer on the production bus accepts a swap and a push, refuses
// the replay batch it asked for, then accepts the next replay.
func TestRSVPTELabelDoctorReportsUnreplayedEntries(t *testing.T) {
	bus, err := server.NewServer(&server.ServerConfig{}, nil)
	require.NoError(t, err)
	writer := &refusingReplayWriter{held: mplsfibevents.NewReplayToken(), refusal: errors.New("vpp: mpls table 0 missing")}
	t.Cleanup(mplsfibevents.EntryChange.Subscribe(bus, writer.handle))
	fib := &busFIB{bus: bus}

	nh := netip.MustParseAddr("10.0.0.3")
	require.NoError(t, fib.programSwap(1001, 18001, nh, 0))
	require.NoError(t, fib.programPush(netip.MustParsePrefix("10.0.0.9/32"), []uint32{18000}, nh, 0, 0))
	require.Empty(t, fib.labelDoctorFindings(), "entries a writer accepted are not findings")

	require.Error(t, fib.replay(&replay.Request{ReplayID: writer.held}))
	findings := fib.labelDoctorFindings()
	require.Len(t, findings, 2)
	for i := range findings {
		assert.Equal(t, mplsfibevents.CodeDoctorNoWriter, findings[i].Code)
		assert.Equal(t, "error", findings[i].Severity)
		assert.Contains(t, findings[i].Message, "table 0 missing", "the writer's cause is named")
		assert.NotEmpty(t, findings[i].Fix)
	}
	assert.Contains(t, findings[0].Message, "swap of in-label 1001", "swaps sort before pushes")
	assert.Contains(t, findings[1].Message, "push for FEC 10.0.0.9/32")

	writer.refusal = nil
	require.NoError(t, fib.replay(&replay.Request{ReplayID: writer.held}))
	assert.Empty(t, fib.labelDoctorFindings(), "a successful replay clears every finding")
}
