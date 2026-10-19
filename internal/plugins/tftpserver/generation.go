// Design: docs/architecture/provisioning/tftp-server.md -- transfers cut when a config change applies
// Related: handler.go -- handleRRQ joins every transfer it starts
// Related: config_tx.go -- install ends the replaced generation

package tftpserver

import (
	"log/slog"
	"net"
	"sync"
)

// transferGeneration is the set of transfers started under one running
// config. end closes every transfer socket before it returns, so once a
// reload has applied no block of a file from the replaced root directory is
// sent: a transfer's next read or write fails and the transfer ends.
//
// The close is synchronous on purpose. A context.AfterFunc close runs in a
// goroutine of its own, so the reload returned before the socket closed and
// a client ACK that raced it was answered from the replaced root.
//
// Safe for concurrent use.
type transferGeneration struct {
	mu sync.Mutex
	// ended is true once end ran. A transfer that joins after it is refused,
	// because its listener served a config that is no longer running.
	ended bool
	// transfers holds the socket of every transfer that joined and has not
	// left. Its size is bounded by the generation's max-transfers semaphore.
	transfers map[*net.UDPConn]struct{}
}

// newTransferGeneration answers an empty generation that accepts transfers.
func newTransferGeneration() *transferGeneration {
	return &transferGeneration{transfers: make(map[*net.UDPConn]struct{})}
}

// join adds the socket of a starting transfer. It answers false when the
// generation has ended, and the caller MUST then close conn and not serve.
// A caller that joined MUST call leave when the transfer ends.
func (g *transferGeneration) join(conn *net.UDPConn) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.ended {
		return false
	}
	g.transfers[conn] = struct{}{}
	return true
}

// leave removes the socket of a transfer that ended. It answers true when the
// caller still owns conn and MUST close it, and false when end already closed
// it. MUST be called once for each successful join.
func (g *transferGeneration) leave(conn *net.UDPConn) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, owned := g.transfers[conn]; !owned {
		return false
	}
	delete(g.transfers, conn)
	return true
}

// end closes the socket of every transfer that joined and refuses every later
// join. Calling it again does nothing. A close error is logged to log.
func (g *transferGeneration) end(log *slog.Logger) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.ended = true
	for conn := range g.transfers {
		delete(g.transfers, conn)
		closeLogged(conn, log, "transfer socket")
	}
}
