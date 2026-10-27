// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- panic containment at the signaling and receive tasks
// Related: engine.go -- engine.run defers the signaling task around each serve pass
// Related: transport_linux.go -- rawTransport.readLoop defers the receive task around each receive pass
// Related: fsm.go -- LSP.locked, which keeps an LSP lock free across a contained panic

package rsvpte

import (
	"net/netip"
	"strconv"
	"sync"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// The task names of the RSVP-TE panic guard, the task label of
// ze_task_panics_total{component="rsvp-te"}.
const (
	taskSignaling = "signaling"
	taskReceive   = "receive"
)

// signalingRestart and receiveRestart tell the operator what brings the unit
// back after a contained panic. The guard declares one restart text per task,
// and one message can name LSPs of different roles, so the signaling text names
// what restarts each role: the upstream's PATH refresh re-creates transit and
// egress state, while nothing refreshes a head-end's own state from outside:
// reconcileTunnels alone signals a configured tunnel or bypass, and it runs at
// startup and on each configuration commit.
const (
	signalingRestart = "each LSP the message named loses its state as if it had timed out: " +
		"a transit or egress LSP is set up again by the upstream's next PATH refresh, " +
		"a tunnel or bypass this router heads is signaled again only when the configuration is next committed; " +
		"the engine handles the next message at once"
	receiveRestart = "the datagram that panicked is dropped and the receiver reads the next one on the same socket at once"
)

// rsvpPanicTasks is the panic boundary of the RSVP-TE tasks that read network
// input.
type rsvpPanicTasks struct {
	guard     *taskguard.Guard
	signaling taskguard.Task
	receive   taskguard.Task
}

// rsvpPanics builds the guard once, on the first task that defers it. Its line
// goes to the rsvp-te subsystem logger rather than the engine logger, which may
// be the discard logger: a contained Ze defect is never silent.
var rsvpPanics = sync.OnceValue(func() *rsvpPanicTasks {
	guard := taskguard.New("rsvp-te", slogutil.Logger("rsvp-te"))
	return &rsvpPanicTasks{
		guard:     guard,
		signaling: guard.Task(taskSignaling, signalingRestart),
		receive:   guard.Task(taskReceive, receiveRestart),
	}
})

// describeHandling names the message a contained signaling panic was handling
// and the neighbor it came from. It runs on the signaling goroutine after the
// panic and reads only what handlePacket recorded, never the packet again: a
// decoder panic would panic a second time.
func (e *engine) describeHandling() taskguard.Context {
	described := taskguard.Context{Unit: "RSVP message from " + e.handlingFrom.String()}
	if e.handling != nil {
		described.MessageType = "type " + strconv.Itoa(int(e.handling.Header.MsgType))
	}
	return described
}

// endHandling is the signaling task's end. The LSPs the message named lose
// their state through tearLSPLocal, the path soft-state expiry takes
// (cleanupTick): the forwarding entry and label the LSP recorded are
// released, every admission charge its key holds is released, and lsp-down
// is emitted. Only the forwarding entry the LSP's recorded role installed is
// withdrawn: an LSP the handler created but gave no role yet installed none.
// No teardown message is sent, as on expiry; the upstream's next refresh
// re-creates transit and egress state, and the next configuration commit
// re-signals a head-end (signalingRestart). A label the handler allocated but had not yet
// recorded on the LSP is not returned to the free list: it is never reused,
// so it cannot collide, and one is lost per contained panic.
//
// RFC 2205 Section 2.4: "A teardown request may be initiated either by an
// application in an end system (sender or receiver), or by a router as the
// result of state timeout or service preemption."
//
// RFC 2205 Section 2.3: "Periodic transmission of refresh messages by hosts
// and routers is expected to handle the occasional loss of an RSVP message."
//
// handlePacket's deferred unlock released engine.control as the panic
// unwound, and LSP.locked released every LSP lock, so both are free here.
func (e *engine) endHandling(*taskguard.Panic) {
	msg := e.handling
	e.handling, e.handlingFrom = nil, netip.Addr{}
	if msg == nil {
		return
	}
	e.control.Lock()
	defer e.control.Unlock()

	for _, key := range handlingKeys(msg) {
		e.tearLSPLocal(key)
		// A handler that panicked after reserve but before recording the
		// LSP's AdmissionIface left a charge tearLSPLocal cannot name.
		e.admission.releaseEverywhere(key)
	}
}

// handlingKeys names the LSPs a message reaches: its sender for a PATH-type
// message, and each FILTER_SPEC's sender for a RESV-type message.
func handlingKeys(msg *ParsedMessage) []lspKey {
	var keys []lspKey
	if msg.HasSession && msg.HasSenderTemplate {
		keys = append(keys, keyFromMessage(msg))
	}
	for i := range msg.FlowDescriptors {
		for _, filter := range msg.FlowDescriptors[i].Filters {
			keys = append(keys, keyFromFilter(msg.Session, filter.Filter))
		}
	}
	return keys
}
