// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- original client pair.
// Related: dialer.go -- worker lifetime; receive_linux.go -- one bounded attempt.

package pppoeclient

import (
	"fmt"
	"io"
	"os"

	"github.com/ze-software/ze/internal/component/iface"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

// clientProtocol is a PRIVATE logical protocol port. Only the production
// preparation in client_native_owner.go supplies a concrete *kernel.ClientSession. Pure fixtures
// implement protocol behavior here and never manufacture public Created grants.
type clientProtocol interface {
	UnitNumber() int
	ReadPacket([]byte) (int, error)
	WritePacket([]byte) (int, error)
	Connect() error
	SetReceiveMRU(uint16) error
	SetIPv4Mode(pppkernel.NPMode) error
	SetIPv6Mode(pppkernel.NPMode) error
	CloseChannel() error
	CloseUnit() error
}

var _ clientProtocol = (*pppkernel.ClientSession)(nil)

// readAttempt MUST perform only one pair attempt. Retry belongs to readPacket,
// after this method returns and releases mu. The sole frameReader calls it;
// pair.ReadPacket owns the bounded original-file lease, never the caller lock.
func (s *sessionLink) readAttempt(buffer []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, io.ErrClosedPipe
	}
	return s.client.ReadPacket(buffer)
}

// clientDialPreparation owns one independent backend captured before any
// transport allocation. Its setup never reopens the active backend. The dialer
// MUST invoke close after protocol workers stop; that owner retires the exact
// original unit, settles its native writer, then releases its original backend.
// Zero/nil values are private and are never returned by prepareClientDial.
type clientDialPreparation struct {
	setup        func(*os.File, string) (clientProtocol, error)
	close        func() error
	nativeWriter func() iface.NativePPPWriter
}

func closeClientOwner(closeOwner func() error) (err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = clientPanicFailure(failure)
		}
	}()
	return closeOwner()
}

func clientPanicFailure(failure any) error {
	if cause, ok := failure.(error); ok {
		return cause
	}
	return fmt.Errorf("pppoe-client: original resource operation panicked: %v", failure)
}
