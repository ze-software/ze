// Design: docs/architecture/provisioning/dhcp-server.md -- Reply delivery to a client with no address
// Related: register.go -- replyDestination, the caller that falls back to broadcast

//go:build !linux

package dhcpserver

import (
	"errors"
	"net"
	"net/netip"
)

// errNoNeighborInstall is the answer of a platform with no neighbor table ze
// writes. replyDestination then broadcasts the reply.
var errNoNeighborInstall = errors.New("installing a neighbor entry is implemented on Linux only")

// installNeighbor cannot write a neighbor entry off Linux, so it refuses and
// the caller broadcasts.
func installNeighbor(string, netip.Addr, net.HardwareAddr) error {
	return errNoNeighborInstall
}
