// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- original client pair.
// Related: dialer.go -- worker lifetime; receive_linux.go -- one bounded attempt.

package pppoeclient

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/ze-software/ze/internal/component/iface"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

// clientProtocol is a PRIVATE logical protocol port. Only the production
// preparation below supplies a concrete *kernel.ClientSession. Pure fixtures
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
// MUST close it after all workers and the original unit have stopped.
// Zero/nil values are private and are never returned by prepareClientDial.
type clientDialPreparation struct {
	setup func(*os.File, string) (clientProtocol, error)
	close func() error
}

func prepareClientDial() (prepared *clientDialPreparation, err error) {
	if err := pppkernel.CheckOriginalPPPSupport(); err != nil {
		return nil, err
	}
	backend, err := iface.OpenBackend("netlink")
	if err != nil {
		return nil, err
	}
	closeBackend := sync.OnceValue(func() error { return closeClientOwner(backend.Close) })
	transferred := false
	defer func() {
		if failure := recover(); failure != nil {
			err = errors.Join(err, clientPanicFailure(failure))
		}
		if !transferred {
			err = errors.Join(err, closeBackend())
		}
	}()
	creator, err := pppkernel.NewUnitCreator(backend)
	if err != nil {
		return nil, err
	}
	prepared = &clientDialPreparation{
		setup: func(transport *os.File, name string) (clientProtocol, error) {
			return createClientPair(creator, transport, name)
		},
		close: closeBackend,
	}
	transferred = true
	return prepared, nil
}

// createClientPair retains every partial original on setup/config refusal.
// NewClientSession alone consumes BOTH originals on every outcome. No raw
// descriptor wrapper, numeric unit lookup or early server Connect is allowed.
// The original-file PPPoX channel creator is a required lower-layer contract:
// the old numeric PPPIOCGCHAN/PPPIOCATTCHAN implementation is NOT authority.
func createClientPair(creator *pppkernel.UnitCreator, transport *os.File, name string) (client clientProtocol, err error) {
	channel, unit, err := pppkernel.DevPPPSetup(creator, transport, name)
	defer func() {
		if failure := recover(); failure != nil {
			err = errors.Join(err, clientPanicFailure(failure))
		}
		if unit != nil {
			err = errors.Join(err, closeClientOwner(unit.Close))
		}
		if channel != nil {
			err = errors.Join(err, closeClientOwner(channel.Close))
		}
	}()
	if err != nil {
		return nil, err
	}
	config, err := unit.SessionConfig(channel)
	if err != nil {
		return nil, err
	}
	// Relinquish ownership BEFORE the consuming call, including its panic lane.
	channel, unit = nil, nil
	return pppkernel.NewClientSession(config)
}

// releaseOwners MUST run after channel stop and all phase/discovery/frame-reader
// joins. Preserve both close outcomes; a panicking unit close cannot strand the
// independent backend. Close returns this retained outcome on subsequent calls.
func (s *sessionLink) releaseOwners(closeUnit, closeBackend func() error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeErr = errors.Join(s.closeErr, closeClientOwner(closeUnit), closeClientOwner(closeBackend))
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
