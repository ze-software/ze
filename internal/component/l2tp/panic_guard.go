// Design: docs/guide/monitoring.md -- the l2tp rows of the contained-panic table
// RFC: rfc/short/rfc2661.md -- the StopCCN a contained control panic sends
// Related: reactor.go -- handle, the control task's boundary
// Related: drain.go -- callAuthHandler and callPoolHandler, the handler tasks
// Related: teardown.go -- teardownTunnel, the path a contained panic ends a tunnel by

package l2tp

import (
	"log/slog"
	"net/netip"
	"strconv"
	"sync"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// l2tpPanicTasks is the L2TP subsystem's panic boundary: one taskguard.Guard and
// each task it guards, declared once. A contained panic is logged, counted in
// ze_task_panics_total{component="l2tp",task} and crash-recorded by the guard.
//
// The listener's readLoop and tunnelReadLoop carry no boundary of their own:
// they copy a datagram into a pool slot and queue it, and decode nothing. What
// a peer sends is first parsed on the reactor goroutine, inside handle, which
// is where the control task's boundary sits.
type l2tpPanicTasks struct {
	guard *taskguard.Guard
	// control is l2tpReactor.handle, one received control datagram: its panic
	// clears the tunnel the datagram reached with a StopCCN
	// (clearTunnelAfterPanic), and the reactor keeps serving the others.
	control taskguard.Task
	// auth is one call of the subscriber auth handler: its panic refuses that
	// session's authentication.
	auth taskguard.Task
	// pool is one call of the subscriber pool handler: its panic refuses that
	// session's address request.
	pool taskguard.Task
}

// newL2TPPanicTasks declares the L2TP tasks on a guard that logs to log.
func newL2TPPanicTasks(log *slog.Logger) *l2tpPanicTasks {
	guard := taskguard.New("l2tp", log)
	return &l2tpPanicTasks{
		guard:   guard,
		control: guard.Task("control", "the tunnel is cleared with a StopCCN and its sessions end; the LAC re-establishes it, and the other tunnels keep running"),
		auth:    guard.Task("auth-handler", "this session's authentication is refused; the subscriber's PPP retries it, and other sessions are unaffected"),
		pool:    guard.Task("pool-handler", "this session's address request is refused; the subscriber's PPP retries it, and other sessions are unaffected"),
	}
}

// l2tpPanics answers the L2TP panic boundary. It is built on first use, so the
// guard's logger is resolved after the log levels are configured. A test MAY
// replace it to read the guard's log line, and MUST restore it.
var l2tpPanics = sync.OnceValue(func() *l2tpPanicTasks {
	return newL2TPPanicTasks(slogutil.Logger(SubsystemName))
})

// controlDispatch is what handle knows about the datagram it is dispatching,
// filled in as it learns it, so a contained panic names the unit it ended.
// Only the reactor goroutine touches it.
type controlDispatch struct {
	from        netip.AddrPort
	messageType MessageType
	// localTID is the tunnel the datagram reached, zero until
	// locateTunnelLocked found or created one.
	localTID uint16
}

// describe names the unit a contained control panic ended.
func (d *controlDispatch) describe() taskguard.Context {
	unit := "control datagram from " + d.from.String()
	if d.localTID != 0 {
		unit = "tunnel " + strconv.Itoa(int(d.localTID)) + ", " + unit
	}
	ctx := taskguard.Context{Unit: unit}
	if d.messageType != 0 {
		ctx.MessageType = d.messageType.String()
	}
	return ctx
}

// describeHandlerCall names the unit a contained handler panic ended: the
// session the auth or pool request belongs to.
func describeHandlerCall(tunnelID, sessionID uint16) func() taskguard.Context {
	return func() taskguard.Context {
		return taskguard.Context{Unit: "tunnel " + strconv.Itoa(int(tunnelID)) + ", session " + strconv.Itoa(int(sessionID))}
	}
}

// clearTunnelAfterPanic ends the tunnel a contained control panic reached. The
// panicking frame may have left the tunnel's FSM or reliable engine half
// updated, so the tunnel cannot continue: Ze closes the control connection
// rather than drop it in silence, so the LAC learns at once and does not keep
// sessions on a tunnel Ze no longer serves.
//
// RFC 2661 Section 6.4: "Stop-Control-Connection-Notification (StopCCN) is a
// control message sent by either the LAC or LNS to inform its peer that the
// tunnel is being shutdown and the control connection should be closed."
//
// The Result Code is the one Ze already sends when its own error ends a
// tunnel (tunnel_fsm.go, resultGeneralError), RFC 2661 Section 4.4.2:
// "1 - General request to clear control connection". Each session ends with
// RFC 2866 Section 5.10 value 9, "NAS detected some error on the NAS port".
//
// A panic before any tunnel was located (a header or SCCRQ parse) has no
// tunnel to clear: that datagram is dropped, as a malformed one is.
//
// It answers the guard's end function, which reads dispatch only after the
// panic, when handle has filled in what it learned.
func (r *l2tpReactor) clearTunnelAfterPanic(dispatch *controlDispatch) func(*taskguard.Panic) {
	return func(*taskguard.Panic) {
		if dispatch.localTID == 0 {
			return
		}
		// RFC 2661 Section 6.4
		err := r.teardownTunnel(dispatch.localTID, ResultCodeValue{Result: resultGeneralError}, l2tpevents.TerminateCauseNASError)
		if err != nil {
			r.logger.Warn("l2tp: the tunnel a contained panic reached could not be cleared",
				"tunnel-id", dispatch.localTID, "error", err.Error())
		}
	}
}
