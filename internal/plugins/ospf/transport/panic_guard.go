// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- panic containment in the receive tasks
// Related: backend_linux.go -- linuxInterface.readLoop, the socket reader task
// Related: transport.go -- Transport.rxLoop and receiveEnded, the per-interface fan-in task

package transport

import (
	"sync"

	"github.com/ze-software/ze/internal/core/taskguard"
)

// interfaceRestart is what brings an interface back after a contained panic in
// its receive path, in the words the guard's log line gives the operator.
const interfaceRestart = "the interface leaves OSPF, its neighbors go Down, and the transport reopens it at the next interface rescan or link-up event"

// receivePanicTasks is the OSPFv2 transport's panic boundary: one
// taskguard.Guard and the two tasks of an interface's receive path. A contained
// panic is logged, counted in ze_task_panics_total{component="ospf",task} and
// crash-recorded by the guard.
type receivePanicTasks struct {
	guard *taskguard.Guard
	// read is the backend's socket reader of one interface
	// (linuxInterface.readLoop): returning closes the interface's receive
	// channel, which rxLoop turns into a link-down (receiveEnded).
	read taskguard.Task
	// fanIn is Transport.rxLoop of one interface, which counts each packet and
	// hands it to the engine: its panic takes the interface down (receiveEnded).
	fanIn taskguard.Task
}

// receivePanics answers the transport's panic boundary. It is built on first
// use, when the first interface opens, so the guard logs through the logger the
// plugin installed with SetLogger at startup.
var receivePanics = sync.OnceValue(newReceivePanicTasks)

func newReceivePanicTasks() *receivePanicTasks {
	guard := taskguard.New("ospf", logger())
	return &receivePanicTasks{
		guard: guard,
		read:  guard.Task("interface-read", interfaceRestart),
		fanIn: guard.Task("interface-fan-in", interfaceRestart),
	}
}

// receiveEnded takes st's interface down when its receive path ended while the
// interface was still open: the socket reader returned (a contained panic, or a
// socket error) or rxLoop itself panicked. Without it the interface would stay
// open with nobody reading its socket, and its neighbors would expire one by one
// with no cause in the log. A closed st.stop is HandleLinkDown's own, normal
// end, and leaves nothing to do. The interface's existing link-down path tears
// its neighbors down (RFC 2328 Section 9.3, InterfaceDown) and the periodic
// rescan reopens it (RescanInterfaces).
func (t *Transport) receiveEnded(st *ifaceState) {
	select {
	case <-st.stop:
		return
	default:
	}
	logger().Warn("ospf/transport: the interface stopped receiving, so it leaves OSPF until the next rescan reopens it; the cause is logged above",
		"interface", st.name, "rescan-interval", rescanInterval)
	if err := t.linkDown(st.name, st); err != nil {
		logger().Warn("ospf/transport: close interface whose receive path ended", "interface", st.name, "err", err)
	}
}

// describe names the interface whose receive path a contained panic ended.
func (st *ifaceState) describe() taskguard.Context {
	return taskguard.Context{Unit: "interface " + st.name}
}
