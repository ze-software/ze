//go:build linux

// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- the DF send and the error queue on Linux
// Related: udp.go -- the transport, Send, SendDF and Run
// Related: udp_other.go -- the platform without IP_MTU_DISCOVER or IP_RECVERR
//
// The three syscalls the transport needs beyond the socket itself, each
// reached through internal/core/probe so the option table and the
// error-queue parser are declared once: IP_RECVERR at creation
// (probe.EnableErrorQueue), IP_MTU_DISCOVER toggled around one write
// (probe.WithDFMode), and the MSG_ERRQUEUE read (probe.DrainErrorQueue).

package transport

import (
	"fmt"
	"net"
	"syscall"

	"github.com/ze-software/ze/internal/core/probe"
)

// transportFamily is the address family of every socket newTransport opens:
// it listens on "udp4", so the socket options are the IPv4 ones.
const transportFamily = probe.FamilyIPv4

// rawConn is the socket's raw descriptor access, for the options set on it.
func (t *UDPTransport) rawConn() (syscall.RawConn, error) {
	return t.conn.SyscallConn()
}

// installErrorQueue sets IP_RECVERR on the fresh socket so a router's
// Fragmentation Needed for a DF datagram, and the kernel's own EMSGSIZE, are
// queued where drainErrorQueue reads them rather than dropped.
func (t *UDPTransport) installErrorQueue() error {
	raw, err := t.rawConn()
	if err != nil {
		return fmt.Errorf("transport: raw descriptor: %w", err)
	}
	return probe.EnableErrorQueue(raw, transportFamily)
}

// writeWithDF is the platform half of SendDF. The caller holds mu. The
// write inside the toggled option is the same write Send makes, so a
// refused DF send drains the queue as Send does: a write the kernel refused
// against its cached path MTU (the honor-cache mode) queues a LOCAL entry
// that sets no sk_err, and no read on the socket would ever wake the drain
// for it.
func (t *UDPTransport) writeWithDF(data, control []byte, remote *net.UDPAddr, df probe.DFMode) error {
	raw, err := t.rawConn()
	if err != nil {
		return fmt.Errorf("transport: raw descriptor: %w", err)
	}
	return probe.WithDFMode(raw, transportFamily, df, func() error {
		return t.write(data, control, remote)
	})
}

// drainErrorQueue reads every entry queued on the socket and delivers the
// size refusals, answering how many entries it read and whether one of
// them was this host's own refusal of a send. A drain that fails is logged
// and counts as nothing read, so Run reports the read error that woke it.
func (t *UDPTransport) drainErrorQueue() (entries int, local bool) {
	err := probe.DrainErrorQueue(t.conn, transportFamily, func(entry probe.QueuedError) {
		entries++
		if entry.Local {
			local = true
		}
		t.deliverQueuedError(entry)
	})
	if err != nil {
		t.logger.Warn("ike transport: error queue drain failed", "error", err)
	}
	return entries, local
}
