// Design: docs/architecture/l2tp/bng-5-pppoe.md -- non-Linux stubs

//go:build !linux

package pppoe

import (
	"errors"
	"time"
)

var errSocketClosed = errors.New("pppoe: socket closed")
var errNotLinux = errors.New("pppoe: PPPoE requires Linux")

func openDiscoverySocket() (int, error) {
	return -1, errNotLinux
}

func closeDiscoverySocket(fd int) {}

func readDiscoveryFrame(fd int, buf []byte) (int, int, error) {
	return 0, 0, errNotLinux
}

func sendDiscoveryFrame(fd, ifindex int, frame []byte) error {
	return errNotLinux
}

func pppoeCreate(ifname string, sid uint16, remoteMAC [EthALen]byte) (int, error) {
	return -1, errNotLinux
}

func closePPPoxFD(fd int) {}

// Exported wrappers for cross-package use (PPPoE client in iface).

func OpenDiscoverySocket() (int, error)                       { return openDiscoverySocket() }
func CloseDiscoveryFD(fd int)                                 { closeDiscoverySocket(fd) }
func ReadDiscoveryFrame(fd int, buf []byte) (int, int, error) { return readDiscoveryFrame(fd, buf) }
func SendDiscoveryFrame(fd, ifindex int, frame []byte) error {
	return sendDiscoveryFrame(fd, ifindex, frame)
}
func PPPoECreate(ifname string, sid uint16, remoteMAC [EthALen]byte) (int, error) {
	return pppoeCreate(ifname, sid, remoteMAC)
}
func ClosePPPoxFD(fd int)                         { closePPPoxFD(fd) }
func SetRecvTimeout(_ int, _ time.Duration) error { return errNotLinux }
