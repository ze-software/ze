// Design: plan/spec-bng-5-pppoe.md -- non-Linux stubs

//go:build !linux

package pppoe

import "errors"

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

func resolveInterface(name string) (ifindex int, hwaddr [EthALen]byte, mtu int, err error) {
	return 0, hwaddr, 0, errNotLinux
}

func pppoeCreate(ifname string, sid uint16, remoteMAC [EthALen]byte) (int, error) {
	return -1, errNotLinux
}

func closePPPoxFD(fd int) {}
