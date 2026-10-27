// Design: docs/architecture/isis/isis-4-component-config.md -- panic containment in the engine's receive worker
// Related: server.go -- startReceiveLoop and receiveWorker, the worker that runs receivePDUs
// Related: circuit/runtime.go -- Circuit.TeardownNeighbor, the per-SNPA adjacency purge

package isis

import (
	"net"
	"strconv"
	"sync"

	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/circuit"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
)

// receiveRestart is what follows a contained panic in the receive worker, in the
// words the guard's log line gives the operator.
const receiveRestart = "the PDU is dropped and the adjacencies of the neighbor that sent it go Down; its next Hello brings them back, and every other adjacency keeps its state"

// enginePanicTasks is the IS-IS engine's panic boundary: one taskguard.Guard and
// the receive worker's task. The transport's circuit tasks have their own guard
// (transport/panic_guard.go) with distinct task names under the same component.
type enginePanicTasks struct {
	guard *taskguard.Guard
	// receive is the engine's receive worker handling one PDU. Its panic purges
	// the sending neighbor's adjacencies on that circuit (endReceivePanic); the
	// worker resumes with the next PDU.
	receive taskguard.Task
}

// enginePanics answers the engine's panic boundary, built on first use so the
// guard logs through the logger the plugin installed with setLogger. It is one
// per plugin, so setMetricsRegistry hands it the registry.
var enginePanics = sync.OnceValue(newEnginePanicTasks)

func newEnginePanicTasks() *enginePanicTasks {
	guard := taskguard.New("isis", logger())
	return &enginePanicTasks{guard: guard, receive: guard.Task("receive", receiveRestart)}
}

// receivePDUs dispatches received frames until the engine is canceled or recv
// closes, and then answers false. A panic while one PDU is handled is contained
// by the receive task: the sending neighbor's adjacencies go Down
// (endReceivePanic) and the pass answers true, so the worker resumes with the
// next PDU. The guard is deferred once per pass, not once per PDU.
func (e *engine) receivePDUs(recv <-chan transport.RawFrame) (resume bool) {
	var current transport.RawFrame
	defer enginePanics().receive.Contain(
		func() taskguard.Context { return describeReceivePanic(current) },
		func(*taskguard.Panic) {
			e.endReceivePanic(current)
			resume = true
		})

	for {
		select {
		case <-e.ctx.Done():
			return false
		case rf, ok := <-recv:
			if !ok {
				return false
			}
			current = rf
			e.dispatch.dispatch(rf)
		}
	}
}

// describeReceivePanic names the PDU a contained receive panic ended by its
// arrival interface index and its sender's SNPA, from the frame header only, so
// describing never decodes the PDU again. It runs once per contained panic.
func describeReceivePanic(rf transport.RawFrame) taskguard.Context {
	return taskguard.Context{Unit: "PDU on ifindex " + strconv.Itoa(rf.IfIndex) + " from " + net.HardwareAddr(rf.SrcMAC[:]).String()}
}

// endReceivePanic ends the PDU a contained receive panic interrupted by purging
// the adjacencies of the neighbor that sent it, and only those: the panic may
// have left that neighbor's adjacency half-updated, while every other adjacency
// on the circuit, and on every other circuit, is untouched. The neighbor is its
// SNPA, the frame's source MAC, which the circuit keys its adjacencies' SNPA
// from. Nothing is sent to the neighbor first, as when a holding timer expires.
//
// ISO/IEC 10589 Section 10.6: "If the holding timer expires before another
// accepted IIH arrives, the IS MUST purge the adjacency and report
// adjacencyStateChange Down."
func (e *engine) endReceivePanic(rf transport.RawFrame) {
	c := e.receiveCircuit(rf.IfIndex)
	if c == nil {
		logger().Warn("isis: a contained receive panic left no adjacency to take down: the PDU's interface has no live circuit, so it is only dropped",
			"ifindex", rf.IfIndex)
		return
	}
	snpa := net.HardwareAddr(rf.SrcMAC[:]).String()
	// ISO/IEC 10589 Section 10.6: purge the adjacency, report Down.
	if c.TeardownNeighbor(adjacency.SNPA(rf.SrcMAC)) == 0 {
		logger().Warn("isis: a contained receive panic left no adjacency to take down: the circuit holds none from the PDU's sender, so the PDU is only dropped",
			"ifindex", rf.IfIndex, "snpa", snpa)
		return
	}
	logger().Warn("isis: the adjacencies of the neighbor whose PDU panicked the receive worker are Down; its next Hello brings them back",
		"ifindex", rf.IfIndex, "snpa", snpa)
}

// receiveCircuit answers the live circuit on ifindex, or nil. circuitsMu is
// released by defer.
func (e *engine) receiveCircuit(ifindex int) *circuit.Circuit {
	e.circuitsMu.RLock()
	defer e.circuitsMu.RUnlock()
	return e.circuits[ifindex]
}
