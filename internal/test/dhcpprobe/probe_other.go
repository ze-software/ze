// Design: docs/architecture/provisioning/dhcp-server.md -- the DHCP server the probe exchanges with
// Related: probe_linux.go -- the probe this host cannot build

//go:build !linux

package dhcpprobe

import (
	"errors"
	"net/netip"
)

var errNeedsLinux = errors.New("dhcpprobe: the DHCP probe needs Linux (veth pairs and AF_PACKET); " +
	"run it on Linux or in Ze's QEMU guest with ./le test qemu")

// Probe is the Linux probe; on this host it cannot be built.
type Probe struct{}

// CreateLink refuses: this host has no veth.
func CreateLink(_ string, _ []netip.Prefix) error {
	return errNeedsLinux
}

// DeleteLink refuses: no link exists on this host.
func DeleteLink(_ string) error {
	return errNeedsLinux
}

// Open refuses: this host has no AF_PACKET.
func Open(_ string) (*Probe, error) {
	return nil, errNeedsLinux
}

// Close has nothing to release.
func (p *Probe) Close() error {
	return errNeedsLinux
}

// Run refuses: no probe exists on this host.
func (p *Probe) Run(_ []Step, _ func() error, _ func(string)) error {
	return errNeedsLinux
}
