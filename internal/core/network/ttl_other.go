//go:build !linux

// Design: docs/architecture/core-design.md, network abstraction layer
// Overview: network.go, RealDialer socket setup

package network

import "net"

func setIPTTL(_ int, _ net.IP, _ uint8) error {
	return errIPTTLSocketOptionsUnsupported
}

func setIPMinTTL(_ int, _ net.IP, _ uint8) error {
	return errIPTTLSocketOptionsUnsupported
}

func setListenIPTTL(_ int, _ uint8) error {
	return errIPTTLSocketOptionsUnsupported
}
