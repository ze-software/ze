// Design: docs/guide/monitoring.md -- the pppoe rows of the contained-panic table
// RFC: rfc/short/rfc2516.md -- the PADT a contained admission panic sends
// Related: subsystem.go -- discoveryReader, which dispatches each frame through dispatchDiscovery
// Related: server.go -- handlePADR, which records the session it is admitting
// Related: drain.go -- callPPPoEAuthHandler and callPPPoEPoolHandler, the handler tasks

package pppoe

import (
	"log/slog"
	"net"
	"strconv"
	"sync"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// pppoePanicTasks is the PPPoE subsystem's panic boundary: one taskguard.Guard
// and each task it guards, declared once. A contained panic is logged, counted
// in ze_task_panics_total{component="pppoe",task} and crash-recorded by the
// guard.
type pppoePanicTasks struct {
	guard *taskguard.Guard
	// discovery is one discovery frame, parsed and handled on the discovery
	// reader goroutine: its panic ends the session that frame was admitting
	// (endAdmissionAfterPanic), and the reader goes on to the next frame.
	discovery taskguard.Task
	// auth is one call of the subscriber auth handler: its panic refuses that
	// session's authentication.
	auth taskguard.Task
	// pool is one call of the subscriber pool handler: its panic refuses that
	// session's address request.
	pool taskguard.Task
}

// newPPPoEPanicTasks declares the PPPoE tasks on a guard that logs to log.
func newPPPoEPanicTasks(log *slog.Logger) *pppoePanicTasks {
	guard := taskguard.New("pppoe", log)
	return &pppoePanicTasks{
		guard:     guard,
		discovery: guard.Task("discovery", "the session this frame was admitting ends; the host starts discovery again, and other hosts and sessions are unaffected"),
		auth:      guard.Task("auth-handler", "this session's authentication is refused; the subscriber's PPP retries it, and other sessions are unaffected"),
		pool:      guard.Task("pool-handler", "this session's address request is refused; the subscriber's PPP retries it, and other sessions are unaffected"),
	}
}

// pppoePanics answers the PPPoE panic boundary. It is built on first use, so
// the guard's logger is resolved after the log levels are configured. A test
// MAY replace it to read the guard's log line, and MUST restore it.
var pppoePanics = sync.OnceValue(func() *pppoePanicTasks {
	return newPPPoEPanicTasks(slogutil.Logger(SubsystemName))
})

// admission is the session a PADR in dispatch is admitting. handlePADR writes
// it once the session is in the table, records the PPP channel and unit
// descriptors while it still owns them, and marks padsSent as it sends the
// PADS; only the discovery reader goroutine touches it.
type admission struct {
	sid      uint16
	padsSent bool
	// pppFDsOpen says chanFD and unitFD are open and still handlePADR's: set
	// once the PPP channel is set up, cleared as the session is handed to the
	// PPP driver, which owns them from then on.
	pppFDsOpen bool
	chanFD     int
	unitFD     int
}

// dispatchDiscovery parses one discovery frame and hands it to its interface's
// server. It is the discovery task's panic boundary: a panic in the parse or
// the handler is contained here, so the reader keeps serving every other host.
// frame MUST NOT be retained past this call.
func (s *Subsystem) dispatchDiscovery(srv *InterfaceServer, frame []byte) {
	srv.admitting = admission{}
	var code byte
	var source net.HardwareAddr
	describe := func() taskguard.Context {
		unit := "discovery frame on " + srv.ifName
		if source != nil {
			unit += " from " + source.String()
		}
		ctx := taskguard.Context{Unit: unit}
		if code != 0 {
			ctx.MessageType = "discovery code " + strconv.Itoa(int(code))
		}
		return ctx
	}
	defer pppoePanics().discovery.Contain(describe, func(*taskguard.Panic) { srv.endAdmissionAfterPanic() })

	pkt, parseErr := ParseDiscovery(frame)
	if parseErr != nil {
		s.logger.Debug("pppoe: parse error", "interface", srv.ifName, "error", parseErr)
		return
	}
	code = pkt.Code
	source = net.HardwareAddr(pkt.SrcMAC[:])
	srv.HandleDiscovery(&pkt)
}

// describeHandlerCall names the unit a contained handler panic ended: the
// session the auth or pool request belongs to. The PPP driver carries the
// access interface's ifindex in the request's TunnelID.
func describeHandlerCall(ifindex, sessionID uint16) func() taskguard.Context {
	return func() taskguard.Context {
		return taskguard.Context{Unit: "ifindex " + strconv.Itoa(int(ifindex)) + ", session " + strconv.Itoa(int(sessionID))}
	}
}

// endAdmissionAfterPanic ends the session a contained discovery panic was
// admitting. The panicking frame may have left it half set up, with a socket
// bound or a PPP session queued, so it cannot continue.
//
// Before the PADS the host holds no session: Ze removes the entry and frees its
// SESSION_ID, and the host's own discovery timeout starts it again. After the
// PADS the host believes the session exists, so Ze tells it the session ended,
// since PPP on a session Ze never finished setting up cannot carry an LCP
// Terminate.
//
// RFC 2516 Section 5.5: "This packet may be sent anytime after a session is
// established to indicate that a PPPoE session has been terminated." and "A PPP
// peer SHOULD use the PPP protocol itself to bring down a PPPoE session, but the
// PADT MAY be used when PPP can not be used."
//
// A panic on a frame that admitted nothing (a PADI, a PADT, a parse) has no
// session to end: that frame is dropped, as a malformed one is.
func (s *InterfaceServer) endAdmissionAfterPanic() {
	admitted := s.admitting
	if admitted.sid == 0 {
		return
	}
	if admitted.pppFDsOpen {
		closePPPoxFD(admitted.chanFD)
		closePPPoxFD(admitted.unitFD)
	}
	if !admitted.padsSent {
		closePPPoxFD(s.sessions.detachTransport(admitted.sid))
		closePPPoxFD(s.sessions.Remove(admitted.sid))
		return
	}
	if s.pppDriver != nil {
		// The PPP session is absent when the panic came before it was queued:
		// ErrSessionNotFound then says there is nothing to stop.
		_ = s.pppDriver.StopSession(uint16(s.ifIndex), admitted.sid) //nolint:errcheck // see above
	}
	// RFC 2516 Section 5.5
	s.handleSessionDown(admitted.sid)
}
