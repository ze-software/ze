// Design: docs/architecture/ldp/mpls-ldp.md -- panic containment at the session and discovery tasks
// Related: register.go -- runSession and readDiscoveryLoop defer these tasks
// Related: session.go -- Session.handlingType names the message a contained panic was handling
// Related: wire.go -- statusInternalError, the Notification a contained session panic sends

package ldp

import (
	"strconv"
	"sync"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// The task names of the LDP panic guard, the task label of
// ze_task_panics_total{component="ldp"}.
const (
	taskSession   = "session"
	taskDiscovery = "discovery"
)

// sessionRestart and discoveryRestart tell the operator what brings the unit
// back after a contained panic.
const (
	sessionRestart   = "the session closes after an Internal Error Notification; the neighbor's next Hello sets it up again, after the setup backoff when it never reached OPERATIONAL"
	discoveryRestart = "the datagram that panicked is dropped and discovery reads the next one on the same socket at once"
)

// ldpPanicTasks is the panic boundary of the LDP tasks that read network input.
type ldpPanicTasks struct {
	guard     *taskguard.Guard
	session   taskguard.Task
	discovery taskguard.Task
}

// ldpPanics builds the guard once, on the first task that defers it. Its line
// goes to the ldp subsystem logger rather than the engine logger, which may be
// the discard logger: a contained Ze defect is never silent.
var ldpPanics = sync.OnceValue(func() *ldpPanicTasks {
	guard := taskguard.New("ldp", slogutil.Logger("ldp"))
	return &ldpPanicTasks{
		guard:     guard,
		session:   guard.Task(taskSession, sessionRestart),
		discovery: guard.Task(taskDiscovery, discoveryRestart),
	}
})

// describeSession names the session a contained panic ended and the message
// it was applying. It runs on the ReadLoop goroutine, after the panic.
func (s *Session) describeSession() taskguard.Context {
	described := taskguard.Context{Unit: "session with " + s.PeerAddr().String()}
	if s.handlingType != 0 {
		described.MessageType = "0x" + string(strconv.AppendUint(nil, uint64(s.handlingType), 16))
	}
	return described
}

// notifyInternalError is the session task's end: it tells the peer the session
// is over before runSession's deferred Stop closes the transport and onDone
// runs the session-down teardown. sendNotification arms the write deadline
// before it takes writeMu, so a write another goroutine has blocked under
// writeMu fails within notificationWriteTimeout and the guard never waits
// longer. writeMu itself is released by defer on every path, so the frame that
// panicked cannot have left it held.
func (s *Session) notifyInternalError(*taskguard.Panic) {
	// RFC 5036 Section 3.5.1.2.7: "When such a condition prevents an
	// implementation from interacting correctly with a peer, the
	// implementation should, when capable of doing so, use the Internal
	// Error Status Code to signal the peer.  This is a fatal error."
	if err := s.sendNotification(statusInternalError, 0, 0); err != nil {
		s.log.Warn("ldp: no Internal Error Notification was sent to the peer; the session closes without one",
			"peer", s.PeerAddr(), "error", err)
	}
}

// describeDiscovery names the discovery unit a contained panic hit: the
// interface the Hello arrived on.
func describeDiscovery(ifName string) func() taskguard.Context {
	return func() taskguard.Context {
		if ifName == "" {
			return taskguard.Context{Unit: "discovery on the system multicast interface", MessageType: "Hello"}
		}
		return taskguard.Context{Unit: "discovery on interface " + ifName, MessageType: "Hello"}
	}
}
