// Design: plan/learned/957-ospf-3-ip-transport.md -- OSPFv2 raw IPv4 transport orchestrator
// Related: backend_linux.go -- Linux AF_INET/SOCK_RAW backend

package transport

import (
	"errors"
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/plugins/ospf/wire"
	"github.com/ze-software/ze/pkg/ze"
)

const (
	rescanInterval      = 30 * time.Second
	socketsPerInterface = 2
	dropMalformedIPv4   = "malformed-ipv4"
)

var loggerPtr atomic.Pointer[slog.Logger]

func init() { loggerPtr.Store(slogutil.DiscardLogger()) }

func logger() *slog.Logger { return loggerPtr.Load() }

func SetLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// RawPacket is the AF-neutral received OSPF datagram, shared via the wire leaf so the
// OSPFv2 and OSPFv3 transports return the same type to the engine. The IPv4 transport
// leaves Dst/HopLimit zero (it strips the IP header); Payload is owned by the receiver
// (the Linux backend copies out of its shared receive buffer before queueing).
type RawPacket = wire.RawPacket

func packetTypeLabel(payload []byte) string {
	if len(payload) < 2 {
		return "short"
	}
	switch payload[1] {
	case 1:
		return "hello"
	case 2:
		return "dbdesc"
	case 3:
		return "lsreq"
	case 4:
		return "lsupdate"
	case 5:
		return "lsack"
	default:
		return "unknown"
	}
}

// InterfaceHandle is one open per-interface OSPF raw IPv4 socket.
type InterfaceHandle interface {
	IfIndex() int
	Send(dst netip.Addr, payload []byte) error
	Recv() <-chan RawPacket
	JoinAllSPFRouters() error
	JoinAllDRouters() error
	LeaveAllDRouters() error
	Close() error
}

type dropRecorder func(reason string)

// DropRecorder is passed to test backends so they can report receive drops
// through the transport-owned metrics. It aliases the internal callback type so
// production backends do not expose metrics internals.
type DropRecorder = dropRecorder

// Backend opens per-interface raw sockets. Tests substitute a fake backend.
type Backend interface {
	OpenInterface(name string, recordDrop dropRecorder) (InterfaceHandle, error)
}

type ifaceState struct {
	name   string
	handle InterfaceHandle
	stop   chan struct{}
}

// Transport is the OSPFv2 raw IPv4 transport orchestrator.
type Transport struct {
	backend Backend

	mu         sync.Mutex
	enabled    map[string]struct{}
	interfaces map[string]*ifaceState

	deliver chan RawPacket
	onDown  func(ifindex int, name string)
	onUp    func(ifindex int, name string)
	signer  func(name string, payload []byte) []byte

	subscribe   func(name string) (<-chan iface.LinkEvent, func())
	ifaceSubs   map[string]func()
	eventsWired bool
	teardown    []func()

	metrics *transportMetrics
	wg      sync.WaitGroup
}

// New constructs a Transport over backend. Production passes NewBackend(); tests
// pass a fake. Nil backend is allowed for tests that exercise pure helpers only.
func New(backend Backend) *Transport {
	return &Transport{
		backend:    backend,
		enabled:    make(map[string]struct{}),
		interfaces: make(map[string]*ifaceState),
		ifaceSubs:  make(map[string]func()),
		subscribe:  iface.Subscribe,
		deliver:    make(chan RawPacket, 256),
		metrics:    nopTransportMetrics(),
	}
}

func (t *Transport) SetMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.metrics = newTransportMetrics(reg)
}

// EnableInterface marks an interface as OSPF-enabled. It is opened only when a
// link-up event or rescan says the interface is up.
func (t *Transport) EnableInterface(name string) {
	t.mu.Lock()
	t.enabled[name] = struct{}{}
	wired := t.eventsWired
	t.mu.Unlock()
	if wired {
		t.subscribeIface(name)
	}
}

func (t *Transport) DisableInterface(name string) {
	t.mu.Lock()
	delete(t.enabled, name)
	cancel := t.ifaceSubs[name]
	delete(t.ifaceSubs, name)
	t.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if err := t.HandleLinkDown(name); err != nil {
		logger().Warn("ospf/transport: close on disable", "interface", name, "err", err)
	}
}

// SetSigner installs a hook that authenticates (signs) every outgoing OSPF packet
// just before it is sent. The hook returns the wire bytes to transmit (the original
// payload when no auth is configured for the interface). ospf-12 owns the signer.
func (t *Transport) SetSigner(fn func(name string, payload []byte) []byte) {
	t.mu.Lock()
	t.signer = fn
	t.mu.Unlock()
}

func (t *Transport) OnInterfaceDown(fn func(ifindex int, name string)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onDown = fn
}

func (t *Transport) OnInterfaceUp(fn func(ifindex int, name string)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onUp = fn
}

func (t *Transport) Receive() <-chan RawPacket { return t.deliver }

func (t *Transport) SubscribeIfaceEvents(eb ze.EventBus) func() {
	return t.subscribeIfaceEventsWithRescan(eb, rescanInterval)
}

func (t *Transport) subscribeIfaceEventsWithRescan(eb ze.EventBus, interval time.Duration) func() {
	if eb == nil {
		return func() {}
	}
	t.mu.Lock()
	t.eventsWired = true
	names := make([]string, 0, len(t.enabled))
	for n := range t.enabled {
		names = append(names, n)
	}
	t.mu.Unlock()
	for _, n := range names {
		t.subscribeIface(n)
	}

	stopRescan := make(chan struct{})
	if interval > 0 {
		t.wg.Go(func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-stopRescan:
					return
				case <-ticker.C:
					t.RescanInterfaces()
				}
			}
		})
	}

	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			t.mu.Lock()
			t.eventsWired = false
			cancels := make([]func(), 0, len(t.ifaceSubs))
			for n, c := range t.ifaceSubs {
				cancels = append(cancels, c)
				delete(t.ifaceSubs, n)
			}
			t.mu.Unlock()
			for _, c := range cancels {
				c()
			}
			close(stopRescan)
		})
	}
	t.mu.Lock()
	t.teardown = append(t.teardown, cleanup)
	t.mu.Unlock()
	return cleanup
}

func (t *Transport) subscribeIface(name string) {
	ch, cancel := t.subscribe(name)
	t.mu.Lock()
	if _, dup := t.ifaceSubs[name]; dup || !t.eventsWired {
		t.mu.Unlock()
		cancel()
		return
	}
	t.ifaceSubs[name] = cancel
	t.mu.Unlock()

	t.wg.Go(func() {
		for ev := range ch {
			var err error
			if ev.Kind == iface.LinkDown {
				err = t.HandleLinkDown(ev.Name)
			} else {
				err = t.HandleLinkUp(ev.Name)
			}
			if err != nil {
				logger().Warn("ospf/transport: iface event handling", "interface", ev.Name, "kind", string(ev.Kind), "err", err)
			}
		}
	})
}

func (t *Transport) RescanInterfaces() {
	t.mu.Lock()
	pending := make([]string, 0, len(t.enabled))
	for name := range t.enabled {
		if _, open := t.interfaces[name]; !open {
			pending = append(pending, name)
		}
	}
	t.mu.Unlock()
	for _, name := range pending {
		if err := t.HandleLinkUp(name); err != nil {
			logger().Warn("ospf/transport: rescan open interface", "interface", name, "err", err)
		}
	}
}

func (t *Transport) HandleLinkUp(name string) error {
	t.mu.Lock()
	_, enabled := t.enabled[name]
	_, open := t.interfaces[name]
	t.mu.Unlock()
	if !enabled || open {
		return nil
	}
	if t.backend == nil {
		return ErrNoBackend
	}
	handle, err := t.backend.OpenInterface(name, t.dropRecorder(name))
	if err != nil {
		return err
	}
	if err := handle.JoinAllSPFRouters(); err != nil {
		if cerr := handle.Close(); cerr != nil {
			logger().Warn("ospf/transport: close after join failure", "interface", name, "err", cerr)
		}
		return err
	}

	st := &ifaceState{name: name, handle: handle, stop: make(chan struct{})}
	t.mu.Lock()
	if _, stillEnabled := t.enabled[name]; !stillEnabled {
		t.mu.Unlock()
		if cerr := handle.Close(); cerr != nil {
			logger().Warn("ospf/transport: close disabled interface", "interface", name, "err", cerr)
		}
		return nil
	}
	if _, dup := t.interfaces[name]; dup {
		t.mu.Unlock()
		if cerr := handle.Close(); cerr != nil {
			logger().Warn("ospf/transport: close duplicate interface", "interface", name, "err", cerr)
		}
		return nil
	}
	t.interfaces[name] = st
	t.metrics.socketsOpen.Set(float64(len(t.interfaces) * socketsPerInterface))
	onUp := t.onUp
	t.mu.Unlock()

	t.wg.Go(func() { t.rxLoop(st) })
	if onUp != nil {
		onUp(handle.IfIndex(), name)
	}
	return nil
}

func (t *Transport) HandleLinkDown(name string) error {
	t.mu.Lock()
	st, open := t.interfaces[name]
	if !open {
		t.mu.Unlock()
		return nil
	}
	delete(t.interfaces, name)
	t.metrics.socketsOpen.Set(float64(len(t.interfaces) * socketsPerInterface))
	onDown := t.onDown
	t.mu.Unlock()

	close(st.stop)
	ifindex := st.handle.IfIndex()
	err := st.handle.Close()
	if onDown != nil {
		onDown(ifindex, name)
	}
	return err
}
func (t *Transport) dropRecorder(name string) dropRecorder {
	return func(reason string) {
		t.metrics.packetsDropped.With(name, reason).Inc()
	}
}

func (t *Transport) RecordDrop(name, reason string) {
	t.metrics.packetsDropped.With(name, reason).Inc()
}

func (t *Transport) rxLoop(st *ifaceState) {
	recv := st.handle.Recv()
	for {
		select {
		case <-st.stop:
			return
		case pkt, ok := <-recv:
			if !ok {
				return
			}
			t.metrics.packetsReceived.With(st.name, packetTypeLabel(pkt.Payload)).Inc()
			select {
			case t.deliver <- pkt:
			case <-st.stop:
				return
			}
		}
	}
}

var (
	ErrNoBackend          = errors.New("ospf/transport: no backend")
	ErrInterfaceNotOpen   = errors.New("ospf/transport: interface not open")
	ErrInvalidDestination = errors.New("ospf/transport: invalid destination")
)

// SendPacket sends final OSPF bytes to dst on name. The payload is sent
// byte-for-byte; packet construction and validation are not transport concerns.
func (t *Transport) SendPacket(name string, dst netip.Addr, payload []byte) error {
	if !dst.Is4() {
		return ErrInvalidDestination
	}
	t.mu.Lock()
	st, open := t.interfaces[name]
	signer := t.signer
	t.mu.Unlock()
	if !open {
		return ErrInterfaceNotOpen
	}
	if signer != nil {
		payload = signer(name, payload)
	}
	if err := st.handle.Send(dst, payload); err != nil {
		t.metrics.packetsDropped.With(name, "send-error").Inc()
		return err
	}
	t.metrics.packetsSent.With(name, packetTypeLabel(payload)).Inc()
	return nil
}

// routedSender is the optional capability of an InterfaceHandle to send a unicast packet
// ROUTED (TTL > 1) instead of on the TTL-1 link-local socket. The Linux backend implements
// it; a handle without it falls back to the ordinary Send.
type routedSender interface {
	SendRouted(dst netip.Addr, payload []byte) error
}

// SendPacketRouted sends OSPF bytes to a unicast dst on name with a routed TTL (> 1), for
// virtual links whose packets must traverse a transit area rather than a single link (RFC
// 2328 section 8.1 / section 15). src is unused for IPv4 (the kernel selects the source
// from the routed egress); it exists so the engine's address-family-neutral Transport
// interface can also carry the OSPFv3 global source.
func (t *Transport) SendPacketRouted(name string, dst, _ netip.Addr, payload []byte) error {
	if !dst.Is4() {
		return ErrInvalidDestination
	}
	t.mu.Lock()
	st, open := t.interfaces[name]
	signer := t.signer
	t.mu.Unlock()
	if !open {
		return ErrInterfaceNotOpen
	}
	if signer != nil {
		payload = signer(name, payload)
	}
	var err error
	if rs, ok := st.handle.(routedSender); ok {
		err = rs.SendRouted(dst, payload)
	} else {
		err = st.handle.Send(dst, payload)
	}
	if err != nil {
		t.metrics.packetsDropped.With(name, "send-error").Inc()
		return err
	}
	t.metrics.packetsSent.With(name, packetTypeLabel(payload)).Inc()
	return nil
}

func (t *Transport) JoinAllDRouters(name string) error {
	t.mu.Lock()
	st, open := t.interfaces[name]
	t.mu.Unlock()
	if !open {
		return ErrInterfaceNotOpen
	}
	return st.handle.JoinAllDRouters()
}

func (t *Transport) LeaveAllDRouters(name string) error {
	t.mu.Lock()
	st, open := t.interfaces[name]
	t.mu.Unlock()
	if !open {
		return ErrInterfaceNotOpen
	}
	return st.handle.LeaveAllDRouters()
}

func (t *Transport) InterfaceOpen(name string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.interfaces[name]
	return ok
}

func (t *Transport) OpenInterfaceCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.interfaces)
}

func (t *Transport) InterfaceNameByIfIndex(ifindex int) (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for name, st := range t.interfaces {
		if st.handle.IfIndex() == ifindex {
			return name, true
		}
	}
	return "", false
}

func (t *Transport) Close() {
	t.mu.Lock()
	teardown := t.teardown
	t.teardown = nil
	names := make([]string, 0, len(t.interfaces))
	for name := range t.interfaces {
		names = append(names, name)
	}
	t.mu.Unlock()
	for _, fn := range teardown {
		fn()
	}
	for _, name := range names {
		if err := t.HandleLinkDown(name); err != nil {
			logger().Warn("ospf/transport: close interface", "interface", name, "err", err)
		}
	}
	t.wg.Wait()
}

// StripIPv4Header removes the IPv4 header delivered by a raw socket and returns
// the OSPF payload and source address from the IP header. It rejects short
// datagrams, illegal IHL values, and IHL overruns before slicing.
func StripIPv4Header(data []byte) ([]byte, netip.Addr, bool) {
	if len(data) < 20 {
		return nil, netip.Addr{}, false
	}
	ihl := int(data[0]&0x0f) * 4
	if ihl < 20 || ihl > len(data) {
		return nil, netip.Addr{}, false
	}
	src := netip.AddrFrom4([4]byte{data[12], data[13], data[14], data[15]})
	return data[ihl:], src, true
}
