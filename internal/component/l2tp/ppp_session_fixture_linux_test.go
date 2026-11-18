// Design: docs/research/l2tpv2-ze-integration.md -- protocol fixtures with explicit stream ownership

package l2tp

import (
	"bytes"
	"errors"
	"io"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

// protocolSessionFixture owns only the streams used to reach authentication or
// observe dispatch. The nil embedded handle panics if a fixture unexpectedly
// reaches network configuration, exactly as the former nil interface backend.
type protocolSessionFixture struct {
	ppp.SessionHandle
	config    pppkernel.SessionConfig
	identity  ppp.SessionIdentity
	channel   io.ReadWriteCloser
	unit      io.ReadWriteCloser
	channelFD int
	closeOnce sync.Once
	closeErr  error
}

// ownedProtocolSessionFixture consumes real socket/file descriptors. Shutdown
// wakes a blocked socket reader before Close joins its descriptor lease; a
// /dev/null fixture has no blocked reader and needs only Close.
func ownedProtocolSessionFixture(cfg pppkernel.SessionConfig, identity ppp.SessionIdentity) (ppp.FrameChannel, ppp.SessionHandle, error) {
	owner := &protocolSessionFixture{
		config:    cfg,
		identity:  identity,
		channel:   pppkernel.NewFDFile(cfg.ChanFD, "fixture.ppp.channel"),
		unit:      pppkernel.NewFDFile(cfg.UnitFD, "fixture.ppp.unit"),
		channelFD: cfg.ChanFD,
	}
	return owner, owner, nil
}

// dispatchProtocolSessionFixture captures the actual factory input. Its fd
// numbers are recorder tokens, so this factory MUST NOT pass them to the OS.
func dispatchProtocolSessionFixture(cfg pppkernel.SessionConfig, identity ppp.SessionIdentity) (ppp.FrameChannel, ppp.SessionHandle, error) {
	owner := &protocolSessionFixture{
		config:    cfg,
		identity:  identity,
		channel:   &protocolDispatchStream{},
		channelFD: -1,
	}
	return owner, owner, nil
}

// protocolFixtureIdentity records the test's explicit synthetic origin. Neither
// field is inferred from a unit number or a native creation observation.
func protocolFixtureIdentity(name string, index uint32) ppp.SessionIdentity {
	return ppp.SessionIdentity{InterfaceName: name, NativeIndex: index}
}

type protocolDispatchStream struct{ bytes.Buffer }

func (*protocolDispatchStream) Close() error { return nil }

func (s *protocolSessionFixture) Control() io.ReadWriter        { return s.channel }
func (s *protocolSessionFixture) Opened() io.Reader             { return s.unit }
func (s *protocolSessionFixture) Identity() ppp.SessionIdentity { return s.identity }

func (s *protocolSessionFixture) InterruptRead() error {
	if s.channelFD < 0 {
		return errors.New("dispatch-only fixture has no interruptible socket")
	}
	return unix.Shutdown(s.channelFD, unix.SHUT_RD)
}

func (s *protocolSessionFixture) Close() error {
	s.closeOnce.Do(func() {
		if s.channelFD >= 0 {
			err := unix.Shutdown(s.channelFD, unix.SHUT_RDWR)
			// The ownership integration fixture uses /dev/null. It cannot
			// block in a read and has no socket shutdown operation.
			if errors.Is(err, unix.ENOTSOCK) {
				err = nil
			}
			s.closeErr = err
		}
		s.closeErr = errors.Join(s.closeErr, s.channel.Close())
		if s.unit != nil {
			s.closeErr = errors.Join(s.closeErr, s.unit.Close())
		}
	})
	return s.closeErr
}
