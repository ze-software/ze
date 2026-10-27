// Design: docs/guide/rpki.md -- RTR Poll Timing, the read task's panic boundary
// Related: rtr_session.go -- RTRSession.readLoop defers the read task
// Related: rpki.go -- SetMetricsRegistry publishes the guard's counter

package rpki

import (
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// rtrReadRestart is what brings an RTR session back after a contained panic, in
// the words the guard's log line gives the operator. The cache group owns the
// retry: a failed sync starts that cache's Retry Interval (cacheGroup.load).
const rtrReadRestart = "this cache's connection closes, the next cache in preference order takes the load, and this cache is asked again after its Retry Interval"

// rpkiPanicTasks is the RPKI plugin's panic boundary: one taskguard.Guard and
// each task it guards, declared once. A contained panic is logged, counted in
// ze_task_panics_total{component="rpki",task} and crash-recorded by the guard.
type rpkiPanicTasks struct {
	guard *taskguard.Guard
	// read is RTRSession.readLoop, which decodes the PDUs one cache sends: its
	// panic ends that sync with an Error Report Internal Error and fails the
	// attempt, so the cache group moves on.
	read taskguard.Task
}

// rpkiPanics answers the RPKI panic boundary. It is built on first use, so the
// guard's logger is resolved after the log levels are configured. The logger is
// the plugin's subsystem name, the one the engine hands ConfigureEngineLogger,
// and never the discard logger the package starts with: a contained panic is
// always reported.
var rpkiPanics = sync.OnceValue(func() *rpkiPanicTasks {
	guard := taskguard.New("rpki", slogutil.Logger("bgp.rpki"))
	return &rpkiPanicTasks{
		guard: guard,
		read:  guard.Task("rtr-read", rtrReadRestart),
	}
})

// pduInFlight is the PDU readLoop is handling, as far as its header was parsed.
// parsed is false until the first header of the sync parses, because type 0 is
// Serial Notify and a zero header would name it.
type pduInFlight struct {
	header rTRHeader
	parsed bool
}

// describeRead names the unit a contained read panic ended: this cache, and the
// PDU type in flight when there was one.
func (s *RTRSession) describeRead(inFlight pduInFlight) taskguard.Context {
	unit := "rtr cache " + net.JoinHostPort(s.address, strconv.Itoa(int(s.port)))
	if !inFlight.parsed {
		return taskguard.Context{Unit: unit}
	}
	return taskguard.Context{Unit: unit, MessageType: pduTypeName(inFlight.header.Type)}
}

// reportInternalError sends the Error Report a contained panic owes the cache
// before the session drops, on the connection readLoop was reading.
//
// RFC 8210 Section 12: "1: Internal Error (fatal):  The party reporting the
// error experienced some kind of internal error unrelated to protocol operation
// (ran out of memory, a coding assertion failed, et cetera)."
//
// The write is bounded like every other Error Report this session sends, so a
// cache that stopped reading cannot hold the cache group's goroutine.
func (s *RTRSession) reportInternalError(conn net.Conn, inFlight pduInFlight) {
	// RFC 8210 Section 5.11: "Error reports are only sent as responses to other
	// PDUs, not to report errors in Error Report PDUs."
	if !inFlight.parsed {
		logger().Warn("rtr: no Error Report sent after the contained panic: no PDU from the cache was being handled",
			"address", s.address)
		return
	}
	// RFC 8210 Section 5.11: "An Error Report PDU MUST NOT be sent for an Error
	// Report PDU."
	if inFlight.header.Type == pduErrorRpt {
		logger().Warn("rtr: no Error Report sent after the contained panic: the PDU being handled was itself an Error Report",
			"address", s.address)
		return
	}

	// RFC 8210 Section 5.11: "If the error is generic (e.g., "Internal Error")
	// and not associated with the PDU to which it is responding, the Erroneous
	// PDU field MUST be empty and the Length of Encapsulated PDU field MUST be
	// zero."
	var report [16]byte
	n := writeErrorReport(report[:], s.version, errInternalError, nil)
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		logger().Warn("rtr: no Error Report sent after the contained panic", "address", s.address, "error", err)
		return
	}
	if _, err := conn.Write(report[:n]); err != nil {
		logger().Warn("rtr: no Error Report sent after the contained panic", "address", s.address, "error", err)
	}
}
