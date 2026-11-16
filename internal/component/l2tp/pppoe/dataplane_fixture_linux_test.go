// Design: docs/architecture/l2tp/bng-5-pppoe.md -- real descriptor fixture ownership
//go:build linux

package pppoe

import (
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

// No interface operation is expected before these fixtures stop at LCP/auth.
// Crossing that boundary still fails through the embedded nil dependency.
type fixtureKernelBackend struct{ pppkernel.InterfaceOperations }

func fixtureKernelSession(t *testing.T, channel, unit, unitNum int) (ppp.FrameChannel, ppp.SessionHandle) {
	t.Helper()
	frames, handle, err := pppkernel.NewSession(pppkernel.SessionConfig{
		ChanFD: channel, UnitFD: unit, UnitNum: unitNum, InterfaceName: "lo",
		Backend: fixtureKernelBackend{},
		ReadCounters: func(string) (ppp.SessionCounters, error) {
			return ppp.SessionCounters{}, errors.New("fixture does not supply raw interface counters")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	return frames, handle
}

type fixtureFDTransport struct {
	mu     sync.Mutex
	file   *os.File
	closed bool
	err    error
}

func (f *fixtureFDTransport) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.closed {
		f.closed = true
		f.err = f.file.Close()
	}
	return f.err
}
func (f *fixtureFDTransport) RelayDescriptor() (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return -1, errors.New("fixture transport closed")
	}
	return int(f.file.Fd()), nil
}

func fixtureFDOpened(t *testing.T, fd int, frames ppp.FrameChannel, handle ppp.SessionHandle) dataplane.OpenedSession {
	t.Helper()
	transport := &fixtureFDTransport{file: os.NewFile(uintptr(fd), "pppoe.fixture")}
	opened, err := dataplane.NewOpenedSession(frames, handle, transport)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = opened.Dataplane().Close(); _ = opened.CloseTransport() })
	return opened
}

//nolint:unparam // Keep the admitted native unit explicit at each session-open fixture call.
func fixtureKernelOpened(t *testing.T, transport, channel, unit, unitNum int) dataplane.OpenedSession {
	t.Helper()
	owner := &fixtureFDTransport{file: os.NewFile(uintptr(transport), "pppoe.fixture")}
	t.Cleanup(func() { _ = owner.Close() })
	frames, handle := fixtureKernelSession(t, channel, unit, unitNum)
	opened, err := dataplane.NewOpenedSession(frames, handle, owner)
	if err != nil {
		t.Fatal(err)
	}
	return opened
}
