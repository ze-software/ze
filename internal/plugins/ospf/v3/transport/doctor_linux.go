//go:build linux

// Design: docs/architecture/ospf/ospfv3-3-ipv6-transport.md -- Linux raw IPv6 socket probe
// RFC: rfc/short/rfc5340.md (§2.9 raw IPv6 proto 89)

package transport

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

// rawSocketOpen opens the socket the probe tries. It is a test seam: a test
// swaps it to drive each branch of checkRawSocket without the host's refusal.
var rawSocketOpen = openRawSocket

// openRawSocket opens a raw IPv6 proto-89 socket, the one the OSPFv3
// transport receives on.
func openRawSocket() (int, error) {
	return unix.Socket(unix.AF_INET6, unix.SOCK_RAW, Protocol)
}

// checkRawSocket opens and immediately closes the raw IPv6 socket the OSPFv3
// transport needs, before the daemon starts. It returns nil when the socket
// opens, and otherwise the reason the host refused it, wrapping the socket
// error.
func checkRawSocket() error {
	fd, err := rawSocketOpen()
	if err != nil {
		return rawSocketRefusal(err)
	}
	if cerr := unix.Close(fd); cerr != nil {
		logger().Warn("ospfv3/transport: close raw-socket probe", "err", cerr)
	}
	return nil
}

// rawSocketImpact is what a refused probe means for the operator.
const rawSocketImpact = "OSPFv3 cannot open its raw IPv6 socket for protocol 89, " +
	"so it forms no adjacencies on this host"

// rawSocketRefusal explains a refused probe socket. EPERM and EACCES mean the
// process lacks CAP_NET_RAW, which granting the capability fixes, so only they
// carry that hint. Any other error has a cause of its own, and the wrapped
// error names it: EAFNOSUPPORT, for one, means the kernel runs without IPv6.
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
