// Design: docs/architecture/provisioning/tftp-server.md -- panic containment in the request and transfer tasks
// Related: handler.go -- serveRequests and runTransfer, the two boundaries

package tftpserver

import (
	"net"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/internal/core/tftp"
)

// taskRequest is the task label of a listener's request task in
// ze_task_panics_total{component="tftp"}.
const taskRequest = "request"

// taskTransfer is the task label of one file transfer in
// ze_task_panics_total{component="tftp"}.
const taskTransfer = "transfer"

// requestRestart and transferRestart are what follows a contained panic, in
// the words the guard's log line gives the operator.
const (
	requestRestart  = "the client is sent an ERROR (code 0) and the listener serves the next request"
	transferRestart = "the client is sent an ERROR (code 0) on the transfer's socket and the transfer " +
		"ends; every other transfer runs on"
)

// panicMessage is the text of the ERROR packet a contained panic sends. It
// names no internals: the cause is in the crash record.
const panicMessage = "internal error"

// tftpGuard is the panic boundary of the TFTP server. A contained panic is
// logged, counted in ze_task_panics_total{component="tftp",task=...} and
// crash-recorded.
var tftpGuard = taskguard.New("tftp", slogutil.Logger("tftpserver"))

// requestTask and transferTask are declared once so their counts exist before
// the first panic.
var (
	requestTask  = tftpGuard.Task(taskRequest, requestRestart)
	transferTask = tftpGuard.Task(taskTransfer, transferRestart)
)

// panicError is the ERROR packet a contained panic sends.
//
// RFC 1350 Section 2: "Most errors cause termination of the connection.  An
// error is signalled by sending an error packet."
//
//nolint:misspell // `signalled` is RFC 1350's spelling, and an RFC quote stays byte-exact
var panicError = tftp.BuildError(tftp.CodeNotDefined, panicMessage)

// signalPanic sends the panic's ERROR packet on conn, to target when conn is
// unconnected. The packet is not retransmitted, as RFC 1350 Section 2 allows,
// so a lost one costs the client its timeout. A send error is logged to the
// package logger: the log argument of the task that panicked may be the
// defect.
func signalPanic(conn *net.UDPConn, target *net.UDPAddr) {
	var err error
	if target == nil {
		_, err = conn.Write(panicError)
	} else {
		_, err = conn.WriteToUDP(panicError, target)
	}
	if err != nil {
		loggerPtr.Load().Debug("tftpserver: panic error response failed", "error", err)
	}
}
