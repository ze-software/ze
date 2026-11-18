// Design: docs/architecture/l2tp/bng-5-pppoe.md -- synthetic descriptor fixture ownership
//go:build linux

package pppoe

import (
	"context"
	"errors"
	"io"
	"net/netip"
	"os"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/core/rtproto"
)

// fixtureSyntheticSession owns socket-backed control and unit streams. It
// preserves their separate descriptor lifetimes without claiming native PPP
// creation. Callers supply identity explicitly. These protocol fixtures stop at
// LCP/auth; every native operation is refused. No public typed owner is minted.
func fixtureSyntheticSession(t *testing.T, channel, unit int, identity ppp.SessionIdentity) (ppp.FrameChannel, ppp.SessionHandle) {
	t.Helper()
	// Nonblocking sockets let os.File's runtime poller wake a borrowed read
	// when this synthetic owner closes or interrupts it. No PPP ioctl is used.
	if err := unix.SetNonblock(channel, true); err != nil {
		t.Fatal(errors.Join(err, unix.Close(channel), unix.Close(unit)))
	}
	if err := unix.SetNonblock(unit, true); err != nil {
		t.Fatal(errors.Join(err, unix.Close(channel), unix.Close(unit)))
	}
	owner := &fixtureSyntheticOwner{
		identity: identity,
		channel:  os.NewFile(uintptr(channel), "pppoe.synthetic.control"),
		unit:     os.NewFile(uintptr(unit), "pppoe.synthetic.unit"),
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	return owner, owner
}

// fixtureSyntheticOwner is a comparable test owner, never a native binding.
// Files support one reader and concurrent writers. Close is idempotent and
// wakes both borrowed streams; this fixture supports no native mutations.
type fixtureSyntheticOwner struct {
	identity  ppp.SessionIdentity
	channel   *os.File
	unit      *os.File
	closeOnce sync.Once
	closeErr  error
}

func (owner *fixtureSyntheticOwner) Control() io.ReadWriter { return owner.channel }
func (owner *fixtureSyntheticOwner) Opened() io.Reader      { return owner.unit }
func (owner *fixtureSyntheticOwner) InterruptRead() error {
	return owner.channel.SetReadDeadline(time.Now())
}
func (owner *fixtureSyntheticOwner) Identity() ppp.SessionIdentity { return owner.identity }
func (owner *fixtureSyntheticOwner) Close() error {
	owner.closeOnce.Do(func() { owner.closeErr = errors.Join(owner.channel.Close(), owner.unit.Close()) })
	return owner.closeErr
}

var errSyntheticNativeOperation = errors.New("synthetic PPP fixture supplies no native operation")

func (*fixtureSyntheticOwner) SetMTU(int) error           { return errSyntheticNativeOperation }
func (*fixtureSyntheticOwner) Connect() error             { return errSyntheticNativeOperation }
func (*fixtureSyntheticOwner) Disconnect() error          { return errSyntheticNativeOperation }
func (*fixtureSyntheticOwner) SetReceiveMRU(uint16) error { return errSyntheticNativeOperation }
func (*fixtureSyntheticOwner) SetAdminUp() error          { return errSyntheticNativeOperation }
func (*fixtureSyntheticOwner) AddAddressP2P(netip.Prefix, netip.Prefix) error {
	return errSyntheticNativeOperation
}
func (*fixtureSyntheticOwner) RemoveAddress(netip.Prefix) error { return errSyntheticNativeOperation }
func (*fixtureSyntheticOwner) AddRoute(netip.Prefix, netip.Addr, int, rtproto.Proto) error {
	return errSyntheticNativeOperation
}
func (*fixtureSyntheticOwner) RemoveRoute(netip.Prefix, netip.Addr, int, rtproto.Proto) error {
	return errSyntheticNativeOperation
}
func (*fixtureSyntheticOwner) Counters() (ppp.SessionCounters, error) {
	return ppp.SessionCounters{}, errSyntheticNativeOperation
}
func (*fixtureSyntheticOwner) OpenIPv6Carrier(context.Context) (ppp.IPv6Carrier, error) {
	return nil, errSyntheticNativeOperation
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
	t.Cleanup(func() {
		if err := errors.Join(opened.Dataplane().Close(), opened.CloseTransport()); err != nil {
			t.Error(err)
		}
	})
	return opened
}

func fixtureSyntheticOpened(t *testing.T, transport, channel, unit int, identity ppp.SessionIdentity) dataplane.OpenedSession {
	t.Helper()
	owner := &fixtureFDTransport{file: os.NewFile(uintptr(transport), "pppoe.fixture")}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	frames, handle := fixtureSyntheticSession(t, channel, unit, identity)
	opened, err := dataplane.NewOpenedSession(frames, handle, owner)
	if err != nil {
		t.Fatal(err)
	}
	return opened
}
