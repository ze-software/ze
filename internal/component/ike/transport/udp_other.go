//go:build !linux

// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- the DF send and the error queue off Linux
// Related: udp_linux.go -- the platform that carries both
// Related: udp.go -- the transport, Send, SendDF and Run
//
// No platform Ze builds for other than Linux carries IP_MTU_DISCOVER or
// IP_RECVERR. The stubs keep the transport compiling there and name the
// absence: a DF send is refused with probe.ErrDFUnsupported and writes
// nothing, never sent with the bit silently clear. The socket opens without
// an error queue, so a read error there is never one the queue explains and
// the drain answers that nothing was read.

package transport

import (
	"net"

	"github.com/ze-software/ze/internal/core/probe"
)

// installErrorQueue installs nothing: there is no IP_RECVERR to set, and the
// socket opens exactly as it did before the option existed.
func (t *UDPTransport) installErrorQueue() error {
	return nil
}

// writeWithDF reports the capability absent and writes nothing.
func (t *UDPTransport) writeWithDF(_, _ []byte, _ *net.UDPAddr, _ probe.DFMode) error {
	return probe.ErrDFUnsupported
}

// drainErrorQueue answers that nothing was read: the socket has no error
// queue, so no read error and no write error is the queue's doing.
func (t *UDPTransport) drainErrorQueue() (entries int, local bool) {
	return 0, false
}
