// Design: docs/guide/rpki.md -- RTR Poll Timing, the read task's panic boundary
// Related: rtr_session.go -- RTRSession.readLoop defers the read task
// Related: rpki.go -- SetMetricsRegistry publishes the guard's counter

package rpki

import (
	"context"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"

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

// rpkiPanics answers the RPKI panic boundary, built on first use.
var rpkiPanics = sync.OnceValue(newRPKIPanicTasks)

// newRPKIPanicTasks builds the panic boundary over the plugin's own logger.
//
// The guard logs through pluginLogHandler and never through a second
// slogutil.Logger("bgp.rpki"): slogutil.Logger registers a fresh level for the
// subsystem each time it is called, so a second call left the plugin's logger on
// a level nothing could reach any more. `request log level bgp.rpki info` then
// changed only the guard's level, and "rtr: sync complete" was never logged.
// The handler reads the configured logger at each record, so a guard built
// before the engine configures that logger still reports a contained panic,
// and is never left on the discard logger the package starts with.
func newRPKIPanicTasks() *rpkiPanicTasks {
	guard := taskguard.New("rpki", slog.New(pluginLogHandler{}))
	return &rpkiPanicTasks{
		guard: guard,
		read:  guard.Task("rtr-read", rtrReadRestart),
	}
}

// pluginLogHandler forwards each record to the handler of the logger that
// logger() answers when the record is written, with the attributes and groups
// added through WithAttrs and WithGroup applied in order. Safe for concurrent
// use: it holds no mutable state.
type pluginLogHandler struct {
	derive func(slog.Handler) slog.Handler
}

// target is the handler a record goes to now.
func (h pluginLogHandler) target() slog.Handler {
	base := logger().Handler()
	if h.derive == nil {
		return base
	}
	return h.derive(base)
}

// Enabled answers with the configured logger's level, so a runtime level change reaches it.
func (h pluginLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.target().Enabled(ctx, level)
}

// Handle writes the record through the configured logger.
func (h pluginLogHandler) Handle(ctx context.Context, record slog.Record) error { //nolint:gocritic // slog.Handler requires the Record by value
	return h.target().Handle(ctx, record)
}

// WithAttrs adds attrs to every record, after this handler's own derivation.
func (h pluginLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h.then(func(base slog.Handler) slog.Handler { return base.WithAttrs(attrs) })
}

// WithGroup opens group name for every record, after this handler's own derivation.
func (h pluginLogHandler) WithGroup(name string) slog.Handler {
	return h.then(func(base slog.Handler) slog.Handler { return base.WithGroup(name) })
}

// then returns a handler that applies this handler's derivation, then next.
func (h pluginLogHandler) then(next func(slog.Handler) slog.Handler) slog.Handler {
	previous := h.derive
	return pluginLogHandler{derive: func(base slog.Handler) slog.Handler {
		if previous != nil {
			base = previous(base)
		}
		return next(base)
	}}
}

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
