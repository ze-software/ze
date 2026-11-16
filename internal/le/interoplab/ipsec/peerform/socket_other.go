//go:build !linux

// Design: docs/architecture/testing/interop.md -- helpers run inside the Linux peer.
// Related: socket_linux.go -- Linux implementation; action_test.go -- portable fake.
package main

import "errors"

func openPeer(command) (peerIO, error) {
	return nil, errors.New("the IPsec peer form helper must run inside the Linux strongSwan container")
}
