//go:build linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- sole receive ownership.
// Related: receive.go -- pull worker; kernel/client_linux.go -- original files.

package pppoeclient

import (
	"errors"
	"syscall"
)

// readPacket is called only by the sole frameReader. Each pair.ReadPacket is
// ONE channel-first, at-most100ms attempt. Retry belongs here, outside link.mu:
// Close, Connect and mode changes MUST get the lock between attempts. The pair
// owns original-file leases and admits unit input only after Connect AND MRU.
// This loop lasts until a packet, terminal error or channel stop.
func (s *sessionLink) readPacket(buf []byte) (int, error) {
	for {
		n, err := s.readAttempt(buf)
		if errors.Is(err, syscall.EAGAIN) {
			continue
		}
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		return n, err
	}
}
