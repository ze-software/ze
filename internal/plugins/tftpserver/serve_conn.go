// Design: docs/architecture/provisioning/tftp-server.md -- serving on a socket the caller bound
// Related: handler.go -- serve, the request loop this starts

package tftpserver

import (
	"log/slog"
	"net"
)

// ServeConn serves the read requests that reach conn from the files under
// rootDir, at most transfersMax at a time, until stop is called. It is the
// server without the plugin around it: no config, no interface binding. The
// functional fixture that drives `ze init --from tftp://` against Ze's own
// server uses it.
//
// The caller owns conn and MUST call stop, which ends every transfer and
// closes conn, before it returns. stop returns once the request loop ended.
func ServeConn(conn *net.UDPConn, rootDir string, transfersMax int, log *slog.Logger) (stop func()) {
	generation := newTransferGeneration()
	sem := make(chan struct{}, transfersMax)
	done := make(chan struct{})
	go serveUntilClosed(generation, conn, rootDir, sem, log, done)
	return func() {
		generation.end(log)
		closeLogged(conn, log, "served socket")
		<-done
	}
}

// serveUntilClosed runs serve and closes done when conn is closed.
func serveUntilClosed(generation *transferGeneration, conn *net.UDPConn, rootDir string, sem chan struct{}, log *slog.Logger, done chan<- struct{}) {
	defer close(done)
	serve(generation, conn, rootDir, sem, log)
}
