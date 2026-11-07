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

// errNoDeviceAddrs is the answer of a platform where ze reads no interface
// address. A request no relay forwarded then draws no reply and a warning.
var errNoDeviceAddrs = errors.New("reading an interface's addresses is implemented on Linux only")

// readDeviceAddrs cannot read an interface's addresses off Linux, so it
// refuses.
func readDeviceAddrs(string) ([]netip.Addr, error) {
	return nil, errNoDeviceAddrs
}

// readNeighbor cannot read a neighbor table off Linux, so it refuses and the
// caller broadcasts.
func readNeighbor(string, netip.Addr) (neighborEntry, error) {
	return neighborEntry{}, errNoNeighborInstall
}
