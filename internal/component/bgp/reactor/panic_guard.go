// Design: docs/architecture/behavior/peer-lifecycle.md -- panic containment in the session and peer tasks
// Related: session.go -- Session.Run, the read task's boundary
// Related: peer_run.go -- safeRunOnce and the delivery worker
// Related: listener.go -- safeHandle, the accepted connection's handler

package reactor

import (
	"sync"

	"github.com/ze-software/ze/internal/core/taskguard"
)

// sessionRestart is what brings a BGP session back after a contained panic, in
// the words the guard's log line gives the operator.
const sessionRestart = "the session reconnects after the peer's connect-retry backoff"

// bgpPanicTasks is the BGP subsystem's panic boundary: one taskguard.Guard and
// each task it guards, declared once. A contained panic is logged, counted in
// ze_task_panics_total{component="bgp",task} and crash-recorded by the guard.
type bgpPanicTasks struct {
	guard *taskguard.Guard
	// read is Session.Run's read loop, which decodes what the peer sends: its
	// panic ends the session with a Cease / Unspecific (ceaseAfterPanic).
	read taskguard.Task
	// cancel is the goroutine Session.Run starts to close the connection when
	// the session ends: its panic closes the connection.
	cancel taskguard.Task
	// connect is Peer.safeRunOnce, one connection cycle: a panic outside Run's
	// own boundary (dial, OPEN setup) ends the cycle with an error.
	connect taskguard.Task
	// delivery is the peer's worker that hands received UPDATEs to the plugins:
	// its panic fails the receive publication, which ends the session.
	delivery taskguard.Task
	// accept is the listener's handler of one accepted connection: its panic
	// closes that connection and the listener keeps accepting.
	accept taskguard.Task
}

// bgpPanics answers the BGP panic boundary. It is built on first use, so the
// guard's logger is resolved after the log levels are configured.
var bgpPanics = sync.OnceValue(func() *bgpPanicTasks {
	guard := taskguard.New("bgp", sessionLogger())
	return &bgpPanicTasks{
		guard:    guard,
		read:     guard.Task("read", sessionRestart),
		cancel:   guard.Task("cancel", sessionRestart),
		connect:  guard.Task("connect", sessionRestart),
		delivery: guard.Task("delivery", sessionRestart),
		accept:   guard.Task("accept", "the peer's next connection is accepted as usual"),
	}
})
