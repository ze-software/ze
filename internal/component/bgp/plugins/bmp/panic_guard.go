// Design: docs/guide/bmp.md -- the receiver session's panic boundary
// Related: bmp.go -- BMPPlugin.handleSession defers the receive task
// Related: register.go -- ConfigureMetrics publishes the guard's counter

package bmp

import (
	"strconv"
	"sync"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// receiveRestart is what brings a BMP receiver session back after a contained
// panic, in the words the guard's log line gives the operator. RFC 7854 Section
// 3.2 puts every option on the monitored router, so the router's own
// reconnect brings the session back; ze keeps listening.
const receiveRestart = "this router's BMP session closes and its routes are withdrawn; the router reconnects under its own configuration, and the listener keeps accepting"

// bmpPanicTasks is the BMP plugin's panic boundary: one taskguard.Guard and each
// task it guards, declared once. A contained panic is logged, counted in
// ze_task_panics_total{component="bmp",task} and crash-recorded by the guard.
type bmpPanicTasks struct {
	guard *taskguard.Guard
	// receive is BMPPlugin.handleSession, which decodes what one monitored router
	// sends: its panic ends that router's session through the session's own
	// teardown (withdraw-router, state removal, close).
	receive taskguard.Task
}

// bmpPanics answers the BMP panic boundary. It is built on first use, so the
// guard's logger is resolved after the log levels are configured. The logger is
// the plugin's subsystem name, the one the engine hands ConfigureEngineLogger,
// and never the discard logger the package starts with: a contained panic is
// always reported.
var bmpPanics = sync.OnceValue(func() *bmpPanicTasks {
	guard := taskguard.New("bmp", slogutil.Logger("bgp.bmp"))
	return &bmpPanicTasks{
		guard:   guard,
		receive: guard.Task("receive", receiveRestart),
	}
})

// msgInFlight is the BMP message handleSession is handling, as far as its common
// header was decoded. decoded is false until the first header decodes, because
// type 0 is Route Monitoring and a zero header would name it.
type msgInFlight struct {
	msgType uint8
	decoded bool
}

// describeReceive names the unit a contained receive panic ended: the monitored
// router, and the message type in flight when there was one.
func describeReceive(remote string, inFlight msgInFlight) taskguard.Context {
	unit := "bmp router " + remote
	if !inFlight.decoded {
		return taskguard.Context{Unit: unit}
	}
	return taskguard.Context{Unit: unit, MessageType: msgTypeName(inFlight.msgType)}
}

// msgTypeName names a BMP message type the way RFC 7854 Section 4 titles it, for
// the operator's log line. The set is open: a router may send any octet, so an
// unassigned type is named by its number.
func msgTypeName(msgType uint8) string {
	switch msgType {
	case MsgRouteMonitoring:
		return "Route Monitoring"
	case MsgStatisticsReport:
		return "Statistics Report"
	case MsgPeerDownNotify:
		return "Peer Down Notification"
	case MsgPeerUpNotify:
		return "Peer Up Notification"
	case MsgInitiation:
		return "Initiation"
	case MsgTermination:
		return "Termination"
	case MsgRouteMirroring:
		return "Route Mirroring"
	}
	return "unassigned type " + strconv.Itoa(int(msgType))
}
