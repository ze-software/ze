//go:build linux

// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- Linux raw-socket probe for the doctor check
//
// checkRawSocket opens and immediately closes an AF_INET/SOCK_RAW socket for
// protocol 112, the exact rx capability the VRRP transport needs. A refusal
// returns its reason, wrapping the socket error, which the doctor check
// surfaces as doctor-vrrp-raw-socket.

package transport

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

// rawSocketOpen opens the socket the probe tries. It is a test seam: a test
// swaps it to drive each branch of checkRawSocket without the host's refusal.
var rawSocketOpen = openRawSocket

// openRawSocket opens a raw IPv4 proto-112 socket, the one the VRRP transport
// receives on.
func openRawSocket() (int, error) {
	return unix.Socket(unix.AF_INET, unix.SOCK_RAW, int(packet.ProtoNumber))
}

// checkRawSocket returns nil when a raw proto-112 IP socket can be opened, and
// the reason it cannot otherwise.
func checkRawSocket() error {
	fd, err := rawSocketOpen()
	if err != nil {
		return rawSocketRefusal(err)
	}
	if cerr := unix.Close(fd); cerr != nil {
		logger().Warn("vrrp/transport: close raw-socket probe", "err", cerr)
	}
	return nil
}

// rawSocketImpact is what a refused probe means for the operator.
const rawSocketImpact = "VRRP cannot open its raw IPv4 socket for protocol 112, " +
	"so this router can neither send nor hear VRRP advertisements"

// rawSocketRefusal explains a refused probe socket. EPERM and EACCES mean the
// process lacks CAP_NET_RAW, which granting the capability fixes, so only they
// carry that hint. Any other error (EPROTONOSUPPORT, EMFILE) has a cause of
// its own, and the wrapped error names it.
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
