package rib

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
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

// TestShowPipelineOrdersTheSameWithAndWithoutATerminal pins that one command
// has ONE row order.
//
// `show bgp rib` streams; `show bgp rib | json` builds a document through
// jsonTerminal. The terminal used to re-sort its rows by peer, direction,
// family and prefix, which gave the same command two orderings depending only
// on whether the operator typed `| json`. The streaming path cannot match a
// global sort, because sorting a stream means holding every row.
//
// VALIDATES: both paths yield the sources' own order, which is deterministic
// because each source sorts its peer list at construction.
// PREVENTS: a re-sort returning to either path and splitting the order again.
func TestShowPipelineOrdersTheSameWithAndWithoutATerminal(t *testing.T) {
	r := newTestRIBManager(t)
	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001)
	for _, peer := range []string{"198.51.100.7", "192.0.2.1", "203.0.113.9"} {
		peerRIB := storage.NewPeerRIB(peer)
		peerRIB.Insert(fam, attrBytes, []byte{24, 10, 0, 0}, true)
		peerRIB.Insert(fam, attrBytes, []byte{24, 10, 0, 1}, true)
		r.bgpPeers[netip.MustParseAddr(peer)] = peerRIB
	}

	streamed := showRowKeys(t, r)

	var document map[string]any
	require.NoError(t, json.Unmarshal(
		mustMarshal(t, r.showPipeline("*", []string{"received", "json"})), &document))
	rows, ok := document["routes"].([]any)
	require.True(t, ok, "the json terminal answers a routes list, got %v", document)

	fromTerminal := make([]string, 0, len(rows))
	for _, raw := range rows {
		row, isRow := raw.(map[string]any)
		require.True(t, isRow)
		peer, _ := row["peer"].(string)
		direction, _ := row["direction"].(string)
		famName, _ := row["family"].(string)
		prefix, _ := row["prefix"].(string)
		var key textbuf.Buffer
		fromTerminal = append(fromTerminal, key.Str(peer).Byte('|').Str(direction).
			Byte('|').Str(famName).Byte('|').Str(prefix).String())
	}

	assert.Equal(t, streamed, fromTerminal,
		"the streamed and buffered answers order the same rows differently")
}
