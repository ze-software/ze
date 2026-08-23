package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// seedShowRows fills one peer's adj-rib-in with the given number of routes.
func seedShowRows(t testing.TB, r *RIBManager, rows int) {
	t.Helper()
	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001)
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	for i := range rows {
		// A distinct /32 per row: 10.<a>.<b>.<c>/32.
		nlri := []byte{32, 10, byte(i >> 16), byte(i >> 8), byte(i)}
		peerRIB.Insert(fam, attrBytes, nlri, true)
	}
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB
}

// showRecords answers the walk `show bgp rib` produces.
func showRecords(t testing.TB, r *RIBManager) sdk.Records {
	t.Helper()
	status, payload, err := r.handleCommand("show bgp rib", "*", []string{"received"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	records, ok := payload.(sdk.Records)
	require.True(t, ok, "show bgp rib must answer with a walk, got %T", payload)
	return records
}

// TestShowPipelineStreams holds `show bgp rib` to writing one line for each row
// once the table passes the buffering window.
//
// VALIDATES: spec-record-answers-3-zero-alloc AC-4 -- rows stream and the
// daemon never holds the whole table as one document.
// PREVENTS: showPipeline collecting its rows and answering with one document
// again, which is the shape that cannot carry a full table at all: a document
// wider than one wire message is rejected as a fault (boundedRecord,
// pkg/plugin/rpc/answer_write.go).
func TestShowPipelineStreams(t *testing.T) {
	// One past the window is what makes the answer a stream. The window is the
	// encoder's, so the number is read from it rather than restated here.
	const rows = rpc.AnswerBufferThreshold + 1

	r := newTestRIBManager(t)
	seedShowRows(t, r, rows)

	w := &answerWriter{rib: r}
	require.NoError(t, showRecords(t, r).WriteAnswer(w, 7, ""))

	// A head, one line for each row, and a terminator. A walk collected into
	// one document writes THREE lines whatever its row count, so this count is
	// what says each row reached the wire on its own.
	assert.Equal(t, rows+2, w.lines)
}

// TestShowPipelineCollapsesInsideTheWindow is the other side of the pair: a
// short answer is still one document, so no consumer of a small table meets a
// new shape.
func TestShowPipelineCollapsesInsideTheWindow(t *testing.T) {
	r := newTestRIBManager(t)
	seedShowRows(t, r, 2)

	w := &answerWriter{rib: r}
	require.NoError(t, showRecords(t, r).WriteAnswer(w, 7, ""))

	assert.Equal(t, 3, w.lines, "a walk inside the window is a head, one document and a terminator")
}

// TestShowPipelineNoLockAcrossWrite proves no answer line is written while
// peerMu is held.
//
// VALIDATES: spec-record-answers-3-zero-alloc AC-5 for `show bgp rib`, which
// held peerMu.RLock across its whole drain until this conversion.
// PREVENTS: an operator's slow terminal holding the lock every received UPDATE
// needs. handleReceivedStructured takes peerMu.Lock for each UPDATE it
// processes, so a walk holding the read side across its writes stalls UPDATE
// processing for as long as the dump takes.
func TestShowPipelineNoLockAcrossWrite(t *testing.T) {
	// Past the window, so rows are written one at a time and the walk is
	// suspended in its yield for every one of those writes. Inside the window
	// nothing is written until the walk has ended, which would make this pass
	// for a reason that is not the one under test.
	const rows = rpc.AnswerBufferThreshold + 1

	r := newTestRIBManager(t)
	seedShowRows(t, r, rows)

	w := &answerWriter{rib: r}
	require.NoError(t, showRecords(t, r).WriteAnswer(w, 7, ""))

	assert.False(t, w.locked, "peerMu was held while an answer line was written")
	assert.Equal(t, rows+2, w.lines, "the walk must stream for this to have been tested")
}
