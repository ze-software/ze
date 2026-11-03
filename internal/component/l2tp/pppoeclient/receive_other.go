//go:build !linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- Linux descriptor boundary.
// Related: receive_linux.go -- Linux implementation.

package pppoeclient

import "errors"

func (s *sessionLink) readPacket([]byte) (int, error) {
	return 0, errors.New("pppoe-client: PPP descriptors require Linux")
}

func prepareClientDescriptors(int, int) error {
	return errors.New("pppoe-client: PPP descriptors require Linux")
}
