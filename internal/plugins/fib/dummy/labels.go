// Design: docs/architecture/mpls/mpls-kernel.md -- MPLS label refusal by fib-dummy
// Related: fibdummy.go -- dummyFIB state and labelEntriesRefused counter
// Related: register.go -- in-daemon OnStarted subscription and cleanup
package fibdummy

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/ze"
)

// labelRefusalText is the exact refusal returned to any label producer whose
// batch addresses fib-dummy: what is wrong, why it matters, and the fix.
const labelRefusalText = "fib dummy programs no label, so this entry is not installed; delete fib dummy, or remove the label protocol"

var errFIBDummyRefusesLabels = errors.New(labelRefusalText)

// handleMPLSEntry refuses every (mpls-fib, entry) batch addressed to this
// fib-dummy instance so label producers never advertise labels nothing
// forwards. Batches tagged with another writer's replay token are left alone,
// uncounted and unacknowledged. Safe for concurrent use.
func (f *dummyFIB) handleMPLSEntry(batch *mplsfib.EntryBatch) {
	if batch == nil {
		return
	}
	if !batch.Addresses(f.labelReplayToken) {
		return
	}
	if n := len(batch.Entries); n > 0 {
		f.labelEntriesRefused.Add(uint64(n))
		if m := metricsPtr.Load(); m != nil {
			m.labelEntriesRefused.Add(float64(n))
		}
	}
	logger().Warn("fib-dummy: "+labelRefusalText,
		"entries", len(batch.Entries),
		"code", mplsfib.CodeDoctorNoWriter)
	if batch.Acknowledge != nil {
		batch.Acknowledge(errFIBDummyRefusesLabels)
	}
}

// subscribeAndRequestLabels subscribes to (mpls-fib, entry) and then emits one
// targeted (mpls-fib, replay-request) carrying this instance's token so entries
// a label source applied before fib-dummy opened reach it and are refused,
// counted, and reported by the source's check. fib-dummy does NOT subscribe to
// mplsfib.WriterStopped because every live apply in a reload window already
// receives its joined refusal and a second replay would double-count
// label-entries-refused.
func (f *dummyFIB) subscribeAndRequestLabels(eb ze.EventBus) (func(), error) {
	unsub := mplsfib.EntryChange.Subscribe(eb, f.handleMPLSEntry)
	if _, err := mplsfib.ReplayRequest.Emit(eb, &replay.Request{ReplayID: f.labelReplayToken}); err != nil {
		unsub()
		return nil, fmt.Errorf("fib-dummy: emit mpls-fib replay request: %w", err)
	}
	return unsub, nil
}
