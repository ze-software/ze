//go:build !linux

// Design: docs/architecture/diagnostics/active-probes.md -- the error queue off Linux
// Related: errqueue_linux.go -- the Linux half with the same signatures
// Related: errqueue.go -- the outcome types and the named absences
//
// A platform without IP_RECVERR has no socket error queue to read and no
// IP_MTU to ask. The stubs keep the package compiling there and name the
// absence: a drain answers ErrErrQueueUnsupported and never an empty queue,
// and the estimate answers ErrPathMTUUnsupported and never a zero. A DF
// probe never opens off Linux in the first place (socket_other.go), so
// neither stub is reached by a prober; they exist so a caller that asks is
// told the capability is absent rather than handed a plausible nothing.

package probe

import (
	"context"
	"net"
	"net/netip"
)

// DrainErrorQueue reports the capability absent. The signature matches
// errqueue_linux.go so every caller is written once.
func DrainErrorQueue(_ net.PacketConn, _ Family, _ func(QueuedError)) error {
	return ErrErrQueueUnsupported
}

// KernelPathMTU reports the capability absent. The signature matches
// errqueue_linux.go so every caller is written once.
func KernelPathMTU(_ context.Context, _ netip.Addr) (uint32, error) {
	return 0, ErrPathMTUUnsupported
}
