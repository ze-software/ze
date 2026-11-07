// Design: docs/guide/firewall.md -- tcp-mss-set, the clamp a forwarded SYN meets
// Related: msslab_linux.go -- the router this host cannot build

//go:build !linux

package msslab

import (
	"errors"
	"time"
)

var errNeedsLinux = errors.New("msslab: the MSS lab needs Linux (veth pairs, AF_PACKET and nftables); " +
	"run it on Linux or in Ze's QEMU guest with ./le test qemu")

// Lab is the Linux router; on this host it cannot be built.
type Lab struct{}

// Open refuses: this host has no veth or AF_PACKET.
func Open(_, _ string) (*Lab, error) {
	return nil, errNeedsLinux
}

// Close has nothing to release.
func (l *Lab) Close() error {
	return errNeedsLinux
}

// Forward refuses: no lab exists on this host.
func (l *Lab) Forward(_ Family, _ Option, _ time.Duration) (Seen, error) {
	return Seen{}, errNeedsLinux
}

// SetMTU refuses: no lab exists on this host.
func (l *Lab) SetMTU(_, _ int) error {
	return errNeedsLinux
}
