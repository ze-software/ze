//go:build linux

// Design: docs/architecture/isis/isis-3-l2-transport.md -- raw-socket probe for the doctor check
//
// checkRawSocket opens and immediately closes an AF_PACKET/SOCK_RAW socket,
// the exact capability the IS-IS transport needs. A refusal returns its
// reason, wrapping the socket error, which the doctor check surfaces as
// doctor-isis-raw-socket.

package transport

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

// rawSocketOpen opens the socket the probe tries. It is a test seam: a test
// swaps it to drive each branch of checkRawSocket without the host's refusal.
var rawSocketOpen = openRawSocket

// openRawSocket opens a raw AF_PACKET socket for every ethertype, the one the
// IS-IS transport receives on.
func openRawSocket() (int, error) {
	return unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ALL)))
}

// checkRawSocket returns nil when a raw AF_PACKET socket can be opened, and
// the reason it cannot otherwise.
func checkRawSocket() error {
	fd, err := rawSocketOpen()
	if err != nil {
		return rawSocketRefusal(err)
	}
	if cerr := unix.Close(fd); cerr != nil {
		logger().Warn("isis/transport: close raw-socket probe", "err", cerr)
	}
	return nil
}

// rawSocketImpact is what a refused probe means for the operator.
const rawSocketImpact = "IS-IS cannot open its raw AF_PACKET socket, " +
	"so it forms no adjacencies on this host"

// rawSocketRefusal explains a refused probe socket. EPERM and EACCES mean the
// process lacks CAP_NET_RAW, which granting the capability fixes, so only they
// carry that hint. Any other error (EAFNOSUPPORT, EMFILE) has a cause of its
// own, and the wrapped error names it.
func rawSocketRefusal(err error) error {
	switch {
	case errors.Is(err, unix.EPERM), errors.Is(err, unix.EACCES):
		return fmt.Errorf(rawSocketImpact+". Ze lacks the CAP_NET_RAW capability: "+
			"run ze as root, or grant the ze binary that capability with "+
			"setcap cap_net_raw+ep, then run ze doctor again: %w", err)
	default:
		return fmt.Errorf(rawSocketImpact+". The system error names the cause: "+
			"fix it on the host, then run ze doctor again: %w", err)
	}
}
