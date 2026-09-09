//go:build !linux

// Design: docs/architecture/testing/interop.md -- Linux-only AF_PACKET injection.
package interoplab

import (
	"errors"
	"net"
)

// SendFrameInNamespace has no non-Linux implementation: entering another
// process's network namespace and binding an AF_PACKET socket are both
// Linux-only facilities.
func SendFrameInNamespace(int, string, func(link *net.Interface) ([]byte, error)) error {
	return errors.New("sending a frame in another process's network namespace requires Linux")
}
