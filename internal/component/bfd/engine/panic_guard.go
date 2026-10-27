// Design: docs/architecture/bfd.md -- panic containment in the receive and express-loop tasks
// Related: loop.go -- Loop.serve, the express loop's boundary

package engine

import (
	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// taskExpress is the task label of the express loop in
// ze_task_panics_total{component="bfd"}.
const taskExpress = "express"

// expressRestart is what follows a contained express-loop panic, in the words
// the guard's log line gives the operator.
const expressRestart = "the express loop resumes; the session it was serving goes Down " +
	"and comes back through the BFD three-way handshake with its peer"

// expressGuard is the panic boundary of every BFD express loop. A contained
// panic is logged, counted in ze_task_panics_total{component="bfd",task="express"}
// and crash-recorded. A test may replace it before Start and restore it after
// Stop: Loop.serve reads it each time the task is entered.
var expressGuard = taskguard.New("bfd", slogutil.Logger("bfd.engine"))

// SetPanicMetrics publishes the express loop's panic counter on reg. The BFD
// component calls it from its metrics hook (bindMetricsRegistry).
func SetPanicMetrics(reg metrics.Registry) { expressGuard.SetMetricsRegistry(reg) }

// describeExpress names the unit a contained express-loop panic ended: the
// session the loop was serving, or the loop itself when the panic came before
// any session was selected (a packet that matched none, the echo timer pass).
// It runs on the express-loop goroutine, the only writer of l.serving.
func (l *Loop) describeExpress() taskguard.Context {
	entry := l.serving
	if entry == nil {
		return taskguard.Context{Unit: "express loop, no session selected", MessageType: l.servingWork}
	}
	return taskguard.Context{Unit: describeSessionKey(entry.machine.Key()), MessageType: l.servingWork}
}

// describeSessionKey writes a session key for the operator: the peer, then
// each field that sets the session apart.
func describeSessionKey(key api.Key) string {
	unit := "session to " + key.Peer.String() + " (" + key.Mode.String() + ", vrf " + key.VRF
	if key.Local.IsValid() {
		unit += ", local " + key.Local.String()
	}
	if key.Interface != "" {
		unit += ", interface " + key.Interface
	}
	return unit + ")"
}

// endServedSession takes the session the express loop was serving Down once the
// guard has contained a defect: Machine.DefectDown. Its state was being changed
// when the defect hit, so it can no longer be trusted, and Down is the state
// the BFD handshake rebuilds from. A session released or removed meanwhile is
// left alone. It runs on the express-loop goroutine after the failing frames
// unwound, so l.mu, which they hold only under defer, is free.
func (l *Loop) endServedSession() {
	entry := l.serving
	l.serving = nil
	if entry == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.sessions[entry.machine.Key()] != entry {
		return
	}
	entry.machine.DefectDown()
}
