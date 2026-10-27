// Design: docs/architecture/isis/isis-3-l2-transport.md -- panic containment in the receive tasks
// Related: backend_linux.go -- linuxCircuit.readLoop, the socket reader task
// Related: transport.go -- Transport.rxLoop and receiveEnded, the per-circuit fan-in task

package transport

import (
	"sync"

	"github.com/ze-software/ze/internal/core/taskguard"
)

// circuitRestart is what brings a circuit back after a contained panic in its
// receive path, in the words the guard's log line gives the operator.
const circuitRestart = "the circuit leaves IS-IS, its adjacencies go Down, and the transport reopens it at the next interface rescan or link-up event"

// receivePanicTasks is the IS-IS transport's panic boundary: one
// taskguard.Guard and the two tasks of a circuit's receive path. A contained
// panic is logged, counted in ze_task_panics_total{component="isis",task} and
// crash-recorded by the guard.
type receivePanicTasks struct {
	guard *taskguard.Guard
	// read is the backend's socket reader of one circuit
	// (linuxCircuit.readLoop): returning closes the circuit's receive channel,
	// which rxLoop turns into a link-down (receiveEnded).
	read taskguard.Task
	// fanIn is Transport.rxLoop of one circuit, which counts each frame and
	// hands it to the engine: its panic takes the circuit down (receiveEnded).
	fanIn taskguard.Task
}

// receivePanics answers the transport's panic boundary. It is built on first
// use, when the first circuit opens, so the guard logs through the logger the
// plugin installed with SetLogger at startup.
var receivePanics = sync.OnceValue(newReceivePanicTasks)

func newReceivePanicTasks() *receivePanicTasks {
	guard := taskguard.New("isis", logger())
	return &receivePanicTasks{
		guard: guard,
		read:  guard.Task("interface-read", circuitRestart),
		fanIn: guard.Task("interface-fan-in", circuitRestart),
	}
}

// receiveEnded takes c's circuit down when its receive path ended while the
// circuit was still open: the socket reader returned (a contained panic, or a
// socket error) or rxLoop itself panicked. Without it the circuit would stay
// open with nobody reading its socket, and its adjacencies would expire one by
// one with no cause in the log. A closed c.stop is HandleLinkDown's own, normal
// end, and leaves nothing to do. The circuit's existing link-down path signals
// the engine, which tears its adjacencies down, and the periodic rescan
// reopens it.
func (t *Transport) receiveEnded(c *circuit) {
	select {
	case <-c.stop:
		return
	default:
	}
	logger().Warn("isis/transport: the circuit stopped receiving, so it leaves IS-IS until the next rescan reopens it; a contained panic is logged by the guard",
		"interface", c.name, "rescan-interval", rescanInterval)
	if err := t.linkDown(c.name, c); err != nil {
		logger().Warn("isis/transport: close circuit whose receive path ended", "interface", c.name, "err", err)
	}
}

// describe names the circuit whose receive path a contained panic ended.
func (c *circuit) describe() taskguard.Context {
	return taskguard.Context{Unit: "interface " + c.name}
}
