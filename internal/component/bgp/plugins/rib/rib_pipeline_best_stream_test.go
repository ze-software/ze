// Design: docs/architecture/plugin/rib-storage-design.md -- the best-path walk
// answers with rows, so a large table streams and no socket write happens while
// peerMu is held.
//
// The goal of this file is the pair of properties the conversion has to keep:
// the payload an operator reads is the payload the same command produced before
// it (AC-8), and the walk holds no lock across the write that carries a row
// (AC-5). Both are driven through the command an operator runs rather than
// through the pipeline function, so a change that keeps the function and moves
// the command still meets them.

package rib

import (
	"encoding/json"
	"io"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// bestPathGoldenPayload is the answer `show bgp rib best` produced for
// seedBestPathFixture BEFORE the walk answered with rows, captured on
// 2026-08-22 from the implementation that built one document under peerMu.
//
// It is written out rather than derived, because a golden derived from the code
// under test moves with it. What would have to break for this to notice: a
// changed field name, a changed field ORDER, a dropped or gained attribute, a
// changed number spelling, a lost peer, a lost prefix, or the envelope key
// changing from best-path. What it cannot notice is a change that keeps every
// byte, which is what AC-8 asks for.
const bestPathGoldenPayload = `{"best-path":[{"family":"ipv4/unicast","prefix":"10.0.0.0/24","best-peer":"192.0.2.2","attributes":{"as-path":{"optional":false,"partial":false,"transitive":true,"value":[65001]},"local-preference":{"optional":false,"partial":false,"transitive":true,"value":200},"next-hop":"10.0.0.1","origin":{"optional":false,"partial":false,"transitive":true,"value":"igp"}}},{"family":"ipv4/unicast","prefix":"172.16.0.0/24","best-peer":"192.0.2.1","attributes":{"as-path":{"optional":false,"partial":false,"transitive":true,"value":[65001]},"community":{"optional":true,"partial":false,"transitive":true,"value":["65000:100"]},"local-preference":{"optional":false,"partial":false,"transitive":true,"value":100},"next-hop":"10.0.0.1","origin":{"optional":false,"partial":false,"transitive":true,"value":"igp"}}}]}`

// seedBestPathFixture fills r with two prefixes across two peers: one prefix
// both peers carry (so the decision process picks one) and one only the first
// peer carries. The winning entries differ in their attribute sets, so a row
// that lost an attribute, or gained one, is visible in the golden above.
func seedBestPathFixture(t *testing.T, r *RIBManager) {
	t.Helper()

	fam := family.IPv4Unicast

	// 10.0.0.0/24 on both peers; 192.0.2.2 wins on LOCAL_PREF.
	shared := []byte{24, 10, 0, 0}
	attrLow := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop, testWireLocalPref100)
	attrHigh := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop, testWireLocalPref200)

	peerA := storage.NewPeerRIB("192.0.2.1")
	peerA.Insert(fam, attrLow, shared, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerA

	peerB := storage.NewPeerRIB("192.0.2.2")
	peerB.Insert(fam, attrHigh, shared, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.2")] = peerB

	// 172.16.0.0/24 on peer A alone, carrying a community the other prefix
	// does not, so the two rows differ in their attribute key set.
	only := []byte{24, 172, 16, 0}
	attrCommunity := concatBytes(attrLow, testWireCommunity65000100)
	peerA.Insert(fam, attrCommunity, only, true)
}

// testWireCommunity65000100 carries COMMUNITIES = 65000:100.
var testWireCommunity65000100 = []byte{0xC0, 0x08, 0x04, 0xFD, 0xE8, 0x00, 0x64}

// TestBestPipelinePayloadUnchanged drives the command an operator runs and
// holds its answer to the bytes the same command produced before the walk
// answered with rows (AC-8).
//
// VALIDATES: AC-8 -- a converted command's payload is byte-identical to the
// payload it produced before this spec, for a walk inside the buffering window.
// PREVENTS: the conversion changing the document an operator, a plugin, or a
// buffered surface reads, which is the one thing this refactor must not do.
func TestBestPipelinePayloadUnchanged(t *testing.T) {
	r := newTestRIBManager(t)
	seedBestPathFixture(t, r)

	status, payload, err := r.handleCommand("show bgp rib best", "*", nil)
	require.NoError(t, err)
	require.Equal(t, statusDone, status)

	got, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.Equal(t, bestPathGoldenPayload, string(got))
}

// wireASPath builds an AS_PATH attribute carrying one four-byte ASN. Each route
// of a seeded table gets its own ASN, so the attribute pool interns one entry
// for each route and a refcount says something about ONE row.
func wireASPath(asn uint32) []byte {
	return []byte{
		0x40, 0x02, 0x06, 0x02, 0x01,
		byte(asn >> 24), byte(asn >> 16), byte(asn >> 8), byte(asn),
	}
}

// seededASNs hands each seeded table its own block of AS numbers.
//
// The attribute pools are process-wide and refcounted, so two tables that shared
// an AS path would share one pool entry, and a test that reads a reference count
// would be reading the other table's reference as well. Every table interning
// its own AS paths is what makes a count say something about ONE table.
var seededASNs atomic.Uint32

// seedBestPathRows fills r with rows routes on one peer, each with its own
// prefix and its own AS path.
func seedBestPathRows(t testing.TB, r *RIBManager, rows int) *storage.PeerRIB {
	t.Helper()
	require.Less(t, rows, 1<<24, "the seeded prefixes must fit 10.x.y.z/32")

	base := 65000 + seededASNs.Add(uint32(rows)) - uint32(rows)

	fam := family.IPv4Unicast
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	for i := range rows {
		nlri := []byte{32, 10, byte(i >> 16), byte(i >> 8), byte(i)}
		attrs := concatBytes(testWireOriginIGP, wireASPath(base+uint32(i)), testWireNextHop, testWireLocalPref100)
		peerRIB.Insert(fam, attrs, nlri, true)
	}
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB
	return peerRIB
}

// bestRecords runs the command an operator runs and reports the walk it
// answered with. A command that answered with anything else fails the test
// here, because every property below is a property of the WALK.
func bestRecords(t testing.TB, r *RIBManager) sdk.Records {
	t.Helper()
	status, payload, err := r.handleCommand("show bgp rib best", "*", nil)
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	records, ok := payload.(sdk.Records)
	require.True(t, ok, "show bgp rib best must answer with a walk, got %T", payload)
	return records
}

// answerWriter takes the answer's lines and answers two questions about each
// one: how many lines the walk wrote, and whether any reader held peerMu while
// the line was written.
type answerWriter struct {
	rib    *RIBManager
	lines  int
	locked bool
}

// Write records the line and tests the lock.
//
// TryLock takes the WRITE side, so it fails while any reader holds peerMu. The
// walk that produced this line is suspended in its yield on this goroutine, so a
// read lock it is holding is visible from here. That is the whole mechanism
// behind AC-5: the assertion is not that the code looks lock-free, it is that
// the lock can be taken at the moment the socket write happens.
func (w *answerWriter) Write(p []byte) (int, error) {
	if w.rib.peerMu.TryLock() {
		w.rib.peerMu.Unlock()
	} else {
		w.locked = true
	}
	w.lines++
	return len(p), nil
}

// writeBestAnswer writes the walk to w exactly as the SDK writes it to the
// engine's socket (Records.WriteAnswer, pkg/plugin/records.go).
func writeBestAnswer(t testing.TB, records sdk.Records, w io.Writer) {
	t.Helper()
	require.NoError(t, records.WriteAnswer(w, 7, ""))
}

// TestBestPipelineStreams holds the walk to writing one line for each row once
// the table passes the buffering window.
//
// VALIDATES: AC-4 -- rows stream and the daemon never holds the whole table as
// one document.
// PREVENTS: the walk collecting its rows and answering with one document again,
// which is the shape that cannot carry a full table at all: a document wider
// than one wire message is rejected as a fault (boundedRecord,
// pkg/plugin/rpc/answer_write.go).
func TestBestPipelineStreams(t *testing.T) {
	// One past the window is what makes the answer a stream. The window is the
	// encoder's, so the number is read from it rather than restated here.
	const rows = rpc.AnswerBufferThreshold + 1

	r := newTestRIBManager(t)
	seedBestPathRows(t, r, rows)

	w := &answerWriter{rib: r}
	writeBestAnswer(t, bestRecords(t, r), w)

	// A head, one line for each row, and a terminator. A walk collected into one
	// document writes THREE lines whatever its row count, so this count is what
	// says each row reached the wire on its own.
	assert.Equal(t, rows+2, w.lines)
}

// TestBestPipelineCollapsesInsideTheWindow is the other side of the pair: a
// short walk is still the one document the command has always answered with, so
// no consumer of a small table meets a new shape.
//
// VALIDATES: the type decision stays the encoder's, taken from the row count.
// PREVENTS: a converted command streaming an answer a consumer reads as one
// value.
func TestBestPipelineCollapsesInsideTheWindow(t *testing.T) {
	r := newTestRIBManager(t)
	seedBestPathRows(t, r, 2)

	w := &answerWriter{rib: r}
	writeBestAnswer(t, bestRecords(t, r), w)

	assert.Equal(t, 3, w.lines, "a walk inside the window is a head, one document and a terminator")
}

// TestBestPipelineNoLockAcrossWrite proves no answer line is written while
// peerMu is held.
//
// VALIDATES: AC-5 -- no socket write happens while peerMu is held.
// PREVENTS: an operator's slow terminal holding the lock every received UPDATE
// needs, which couples the BGP data plane to the reader of a show command.
// handleReceivedStructured takes peerMu.Lock for each UPDATE it processes
// (rib_structured.go), so a walk that held the read side across its writes would
// stall UPDATE processing for as long as the dump takes.
func TestBestPipelineNoLockAcrossWrite(t *testing.T) {
	// Past the window, so the rows are written one at a time and the walk is
	// suspended in its yield for every one of those writes. Inside the window
	// nothing is written until the walk has ended, which would make this pass
	// for a reason that is not the one under test.
	const rows = rpc.AnswerBufferThreshold + 1

	r := newTestRIBManager(t)
	seedBestPathRows(t, r, rows)

	w := &answerWriter{rib: r}
	writeBestAnswer(t, bestRecords(t, r), w)

	require.Equal(t, rows+2, w.lines, "the rows must reach the writer one at a time for this to test anything")
	assert.False(t, w.locked, "peerMu was held while an answer line was written")
}

// TestBestPipelineHandleSafeOutsideLock proves what makes the dereference safe
// once the lock is gone.
//
// VALIDATES: AC-12 and R-3 -- the pool-handle dereference is safe outside the
// lock, because the walk holds a REFERENCE to the entry rather than a lock over
// it.
// PREVENTS: the row reading attributes the RIB has already released. In a
// release build a released slot returns to its shard's free list and is
// re-interned with other bytes (attrpool, slotReuseEnabled), so the row would
// carry another route's attributes rather than an error.
func TestBestPipelineHandleSafeOutsideLock(t *testing.T) {
	r := newTestRIBManager(t)

	fam := family.IPv4Unicast
	nlri := []byte{24, 10, 0, 0}
	attrs := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop, testWireLocalPref100)
	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrs, nlri, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	r.peerMu.RLock()
	source := newBestSource(r, "*", nil)
	r.peerMu.RUnlock()
	defer source.release()

	item, ok := source.Next()
	require.True(t, ok)

	// The withdrawal an UPDATE performs, on the path that performs it.
	// PeerRIB.Remove releases the entry's pool handles under the storage lock
	// alone: handleReceivedStructured gives peerMu back before it removes
	// anything (rib_structured.go), so holding peerMu would not have stopped
	// this.
	require.True(t, peerRIB.Remove(fam, nlri))

	// The row is built after the route is gone and still carries the attributes
	// it was selected with, because Next took a reference to them inside the
	// storage lock.
	result := bestResultFor(item)
	require.NotEmpty(t, result.Attrs, "the row lost the attributes its reference was taken for")
	assert.Equal(t, "10.0.0.1", result.Attrs["next-hop"])
}

// TestBestPathRowsReleaseWhenTheWalkStops proves the reference is given back on
// the path that has no last row: a consumer that stops the walk.
//
// VALIDATES: R-1 -- every acquisition has a paired release on every path,
// including the one `| first 10` takes over a million-row table.
// PREVENTS: a pool slot held for the life of the process for every bounded read
// an operator makes, which is a leak that degrades over hours rather than
// failing a test.
func TestBestPathRowsReleaseWhenTheWalkStops(t *testing.T) {
	r := newTestRIBManager(t)
	peerRIB := seedBestPathRows(t, r, 3)

	fam := family.IPv4Unicast
	first := []byte{32, 10, 0, 0, 0}
	entry, ok := peerRIB.Lookup(fam, first)
	require.True(t, ok)
	handle := entry.ASPath
	require.True(t, handle.IsValid())

	rows := 0
	for range bestRecords(t, r).Rows {
		rows++
		break
	}
	require.Equal(t, 1, rows)

	// The RIB gives back its own reference. Nothing else may be holding one.
	require.True(t, peerRIB.Remove(fam, first))

	_, err := pool.ASPath.Get(handle)
	require.Error(t, err, "the walk kept a reference to a row it had already handed over")
}

// walkDeadline bounds the concurrent walk below. It is far above the
// milliseconds the walk takes and far below any suite timeout, so it separates a
// wedged walk from a slow host without either one waiting on the other.
const walkDeadline = 30 * time.Second

// TestBestPipelineWalkSurvivesConcurrentUpdates runs the walk beside the writer
// that removes and re-adds the routes it reads.
//
// VALIDATES: AC-5's second half -- the race detector is clean over a dump that
// runs while UPDATEs are being processed.
// PREVENTS: the lock change turning a safe read into a torn one. It is a race
// probe rather than an assertion about content: what a row carries for a route
// that is being withdrawn under it is a question the walk answers either way,
// and the answer is the prefix with no attributes.
func TestBestPipelineWalkSurvivesConcurrentUpdates(t *testing.T) {
	const rows = 64

	r := newTestRIBManager(t)
	peerRIB := seedBestPathRows(t, r, rows)

	fam := family.IPv4Unicast
	stop := make(chan struct{})
	var writer sync.WaitGroup
	writer.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			// The attributes the route comes back with do not matter here: what
			// this goroutine is for is the removal, which releases the pool
			// handles the walk is reading.
			attrs := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop, testWireLocalPref100)
			for i := range rows {
				nlri := []byte{32, 10, byte(i >> 16), byte(i >> 8), byte(i)}
				peerRIB.Remove(fam, nlri)
				peerRIB.Insert(fam, attrs, nlri, true)
			}
		}
	})

	w := &answerWriter{rib: r}
	records := bestRecords(t, r)

	// The walk runs under a deadline, because the failure this test exists to
	// catch is a WEDGE and not a wrong answer: a walk that takes the storage
	// lock twice stops for good behind the writer, and a test that waited for it
	// would report the package timeout minutes later instead of the walk.
	done := make(chan struct{})
	var writeErr error
	go func() {
		defer close(done)
		writeErr = records.WriteAnswer(w, 7, "")
	}()
	select {
	case <-done:
	case <-time.After(walkDeadline):
		close(stop)
		t.Fatal("the walk did not finish beside a writer removing the routes it reads")
	}

	close(stop)
	writer.Wait()

	require.NoError(t, writeErr)
	assert.False(t, w.locked, "peerMu was held while an answer line was written")
}

// BenchmarkBestPathRowsWalk measures what one route of a best-path dump costs
// when the answer is written.
//
// b.N is the ROW count: the table is seeded with b.N routes and the whole answer
// is written once, so allocs/op reads as allocations for each row. The number
// covers the whole command -- the item list the walk is built from, the per-row
// pool reference, the row's own encoding and the line the writer sends -- which
// is what an operator's dump actually pays.
//
// Run it past AnswerBufferThreshold rows or it measures the collapse path
// instead of the streamed one: `-benchtime=1000x`.
func BenchmarkBestPathRowsWalk(b *testing.B) {
	r := newTestRIBManager(b)
	seedBestPathRows(b, r, b.N)

	b.ReportAllocs()
	b.ResetTimer()
	writeBestAnswer(b, bestRecords(b, r), io.Discard)
}
