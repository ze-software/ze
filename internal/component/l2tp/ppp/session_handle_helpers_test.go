// Design: docs/architecture/l2tp/bng-5-pppoe.md -- fake session ownership for protocol tests.
package ppp

import (
	"context"
	"errors"
	"io"
	"net/netip"
	"strconv"
	"sync"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// testIfaceBackend is the existing recorder's string-boundary API, held only by
// the per-session test adapter. Production PPP never sees it.
type testIfaceBackend interface {
	SetMTU(string, int) error
	SetAdminUp(string) error
	AddAddressP2P(string, string, string) error
	RemoveAddress(string, string) error
	AddRoute(string, string, string, int, rtproto.Proto) error
	RemoveRoute(string, string, string, int, rtproto.Proto) error
}

type testSessionFrames struct {
	control io.ReadWriter
	opened  io.Reader
}

func (f *testSessionFrames) Control() io.ReadWriter { return f.control }
func (f *testSessionFrames) Opened() io.Reader      { return f.opened }
func (f *testSessionFrames) InterruptRead() error   { return interruptRead(f.control) }

// testSessionHandle keeps the original fake ioctl identifiers and recorder so
// the old assertions observe exactly the same operations, now per session.
// Close owns both borrowed streams and serializes repeated release.
type testSessionHandle struct {
	backend                 testIfaceBackend
	name                    string
	ops                     pppOps
	chanFD, unitFD, unitNum int
	owned                   []io.Closer
	once                    sync.Once
	connectionMu            sync.Mutex
	connected               bool
	closeErr                error
	openIPv6Carrier         func(context.Context) (IPv6Carrier, error)
	readCounters            func() (SessionCounters, error)
}

func newTestSessionHandle(backend testIfaceBackend, ifname string) *testSessionHandle {
	return &testSessionHandle{backend: backend, name: ifname, ops: pppOps{connect: func(int, int) error { return nil }}}
}

func (h *testSessionHandle) Identity() SessionIdentity {
	return SessionIdentity{InterfaceName: h.name, NativeIndex: uint32(h.unitNum)}
}
func (h *testSessionHandle) SetMTU(mtu int) error { return h.backend.SetMTU(h.name, mtu) }
func (h *testSessionHandle) SetAdminUp() error    { return h.backend.SetAdminUp(h.name) }

// Connect models the concrete handles: logical reconciliation is idempotent,
// while ops.connect observes physical attachment only. Reopen barrier tests
// intercept SessionHandle.Connect itself so this cache cannot hide early calls.
func (h *testSessionHandle) Connect() error {
	h.connectionMu.Lock()
	defer h.connectionMu.Unlock()
	if h.connected {
		return nil
	}
	if err := h.ops.connect(h.chanFD, h.unitNum); err != nil {
		return err
	}
	h.connected = true
	return nil
}
func (h *testSessionHandle) Disconnect() error {
	h.connectionMu.Lock()
	defer h.connectionMu.Unlock()
	if !h.connected {
		return nil
	}
	if err := h.ops.disconnect(h.chanFD); err != nil {
		return err
	}
	h.connected = false
	return nil
}
func (h *testSessionHandle) SetReceiveMRU(mru uint16) error { return h.ops.setMRU(h.unitFD, mru) }
func (h *testSessionHandle) AddAddressP2P(local, peer netip.Prefix) error {
	return h.backend.AddAddressP2P(h.name, local.String(), peer.String())
}
func (h *testSessionHandle) RemoveAddress(local netip.Prefix) error {
	return h.backend.RemoveAddress(h.name, local.String())
}
func (h *testSessionHandle) AddRoute(dest netip.Prefix, gateway netip.Addr, metric int, proto rtproto.Proto) error {
	return h.backend.AddRoute(h.name, dest.String(), gateway.String(), metric, proto)
}
func (h *testSessionHandle) RemoveRoute(dest netip.Prefix, gateway netip.Addr, metric int, proto rtproto.Proto) error {
	return h.backend.RemoveRoute(h.name, dest.String(), gateway.String(), metric, proto)
}
func (h *testSessionHandle) Counters() (SessionCounters, error) {
	if h.readCounters == nil {
		return SessionCounters{}, errors.New("test session counters not supplied")
	}
	return h.readCounters()
}
func (h *testSessionHandle) OpenIPv6Carrier(ctx context.Context) (IPv6Carrier, error) {
	if h.openIPv6Carrier == nil {
		return nil, errors.New("test IPv6 carrier not supplied")
	}
	return h.openIPv6Carrier(ctx)
}
func (h *testSessionHandle) Close() error {
	h.once.Do(func() {
		for _, stream := range h.owned {
			h.closeErr = errors.Join(h.closeErr, stream.Close())
		}
	})
	return h.closeErr
}

// testSessionStart substitutes the existing registered pipe wrappers for the
// kernel descriptors. The actual Driver still consumes StartSession directly.
func testSessionStart(input *StartSession, backend testIfaceBackend, ops pppOps, chanFD, unitFD, unitNum int) StartSession {
	// Preserve the old value parameter: bind a private payload copy.
	start := *input
	control := newChanFileFn(chanFD, "ppp.chan")
	opened := newUnitFileFn(unitFD)
	frames := &testSessionFrames{control: control, opened: opened}
	handle := newTestSessionHandle(backend, "ppp"+strconv.Itoa(unitNum))
	handle.ops, handle.chanFD, handle.unitFD, handle.unitNum = ops, chanFD, unitFD, unitNum
	handle.owned = append(handle.owned, control)
	if opened != nil {
		if unitFD != chanFD {
			handle.owned = append(handle.owned, opened)
		}
	}
	start.Frames, start.Dataplane = frames, handle
	if start.SubscriberID == "" {
		start.SubscriberID = "fixture:" + strconv.Itoa(int(start.TunnelID)) + ":" + strconv.Itoa(int(start.SessionID))
	}
	attempt, err := NewSessionAttempt(start.SubscriberID, handle)
	if err != nil {
		panic("BUG: invalid protocol fixture admission: " + err.Error())
	}
	start.Attempt = attempt
	return start
}

// newSessionTestDriver retains the existing auto-accepting fixture behavior;
// syscall and interface dependencies are supplied by each queued session.
func newSessionTestDriver() *Driver {
	driver := NewProductionDriver(discardLogger())
	go autoAcceptAuth(driver)
	return driver
}
