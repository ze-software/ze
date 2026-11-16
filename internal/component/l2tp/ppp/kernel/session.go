// Design: docs/research/l2tpv2-ze-integration.md -- kernel PPP session ownership

package kernel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"sync"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/rtproto"
)

// NewSession consumes the supplied channel and unit descriptors on every
// outcome. The caller MUST NOT close them after calling this constructor.
// The returned streams are borrowed; the returned handle is their sole owner.
// The separate PPPoX transport remains with the transport subsystem, allowing a
// kernel relay to release its local PPP lease without ending the transport.
func NewSession(cfg SessionConfig) (ppp.FrameChannel, ppp.SessionHandle, error) {
	return newSession(cfg, productionSessionOps())
}

type session struct {
	mu           sync.Mutex
	closed       bool
	closeErr     error
	identity     ppp.SessionIdentity
	channel      io.ReadWriteCloser
	unit         io.ReadWriteCloser
	channelFD    int
	unitFD       int
	unitNum      int
	backend      InterfaceOperations
	readCounters func(string) (ppp.SessionCounters, error)
	ops          sessionOps
	connected    bool
	addresses    map[netip.Prefix]sessionAddress
	routes       map[sessionRoute]struct{}
}

const maxSessionAddresses = 1024

// Bound retained cleanup intents even when a backend's add result is unknown.
const maxSessionRoutes = 1024

// sessionRoute identifies a path on this owner's exclusively held PPP unit.
// Its zero value is invalid. Only newSessionRoute constructs native queries;
// AddRoute separately refuses the wildcard query protocol as a producer.
type sessionRoute struct {
	destination netip.Prefix
	gateway     netip.Addr
	metric      int
	protocol    rtproto.Proto
}

// Canonical request values and receipt identity must agree before any backend
// call; native serialization must not truncate an input into another path.
func newSessionRoute(destination netip.Prefix, gateway netip.Addr, metric int, protocol rtproto.Proto) (sessionRoute, error) {
	if !destination.IsValid() {
		return sessionRoute{}, errors.New("ppp: invalid session route")
	}
	if !gateway.IsValid() {
		return sessionRoute{}, errors.New("ppp: invalid session route peer")
	}
	peer := gateway.Unmap()
	if peer.IsUnspecified() {
		return sessionRoute{}, errors.New("ppp: session route peer is unspecified")
	}
	if metric < 0 {
		return sessionRoute{}, errors.New("ppp: session route metric is negative")
	}
	if uint64(metric) > uint64(^uint32(0)) {
		return sessionRoute{}, errors.New("ppp: session route metric exceeds native width")
	}
	if protocol < rtproto.Any {
		return sessionRoute{}, errors.New("ppp: session route protocol is negative")
	}
	if uint64(protocol) > uint64(^uint8(0)) {
		return sessionRoute{}, errors.New("ppp: session route protocol exceeds native width")
	}
	return sessionRoute{destination: destination.Masked(), gateway: peer, metric: metric, protocol: protocol}, nil
}

type sessionAddressState uint8

const (
	sessionAddressEligible sessionAddressState = iota + 1
	sessionAddressRetiring
)

type sessionAddress struct {
	peer  netip.Prefix
	state sessionAddressState
}

func newSession(cfg SessionConfig, ops sessionOps) (frames ppp.FrameChannel, handle ppp.SessionHandle, err error) {
	var channel, unit io.ReadWriteCloser
	transferred := false
	defer func() {
		if transferred {
			return
		}
		failure := recover()
		cleanupErr := closeSessionStream(channel, cfg.ChanFD, ops.closeFD)
		if cfg.UnitFD != cfg.ChanFD {
			cleanupErr = errors.Join(cleanupErr, closeSessionStream(unit, cfg.UnitFD, ops.closeFD))
		}
		// The enclosing transport guard owns panic containment. Construction
		// owns rollback, including a panic before any error result was assigned.
		if failure != nil {
			if cleanupErr != nil {
				panic(errors.Join(sessionPanicFailure(failure), cleanupErr))
			}
			panic(failure)
		}
		err = errors.Join(err, cleanupErr)
	}()
	if err := validateSessionConfig(cfg); err != nil {
		return nil, nil, err
	}
	if ops.setIPv4Mode == nil || ops.setIPv6Mode == nil {
		return nil, nil, errors.New("ppp: missing kernel family mode operations")
	}
	name := cfg.InterfaceName
	if name == "" {
		name = "ppp" + strconv.Itoa(cfg.UnitNum)
	}
	index, err := ops.interfaceIndex(name)
	if err != nil {
		return nil, nil, fmt.Errorf("ppp: cannot identify session interface %s: %w", name, err)
	}
	if index == 0 {
		return nil, nil, fmt.Errorf("ppp: session interface %s has no native index", name)
	}
	channel, err = ops.wrap(cfg.ChanFD, "ppp.chan")
	if err != nil {
		return nil, nil, err
	}
	if nilSessionDependency(channel) {
		channel = nil
		return nil, nil, errors.New("ppp: kernel channel wrapper returned no stream")
	}
	unit, err = ops.wrap(cfg.UnitFD, "ppp.unit")
	if err != nil {
		return nil, nil, err
	}
	if nilSessionDependency(unit) {
		unit = nil
		return nil, nil, errors.New("ppp: kernel unit wrapper returned no stream")
	}
	owner := &session{identity: ppp.SessionIdentity{InterfaceName: name, NativeIndex: index}, channel: channel, unit: unit, channelFD: cfg.ChanFD, unitFD: cfg.UnitFD, unitNum: cfg.UnitNum, backend: cfg.Backend, readCounters: cfg.ReadCounters, ops: ops}
	transferred = true
	return owner, owner, nil
}

// Each stream has its own panic boundary so a broken closer cannot prevent
// release of the other resource, during construction or final owner cleanup.
// Descriptor zero is owned; only a negative value denotes no descriptor.
func closeSessionStream(stream io.ReadWriteCloser, fd int, closeFD func(int) error) (err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = fmt.Errorf("ppp: closing descriptor %d panicked: %w", fd, sessionPanicFailure(failure))
		}
	}()
	if !nilSessionDependency(stream) {
		return stream.Close()
	}
	if fd < 0 {
		return nil
	}
	return closeFD(fd)
}

func sessionPanicFailure(failure any) error {
	if cause, ok := failure.(error); ok {
		return cause
	}
	return fmt.Errorf("ppp: session operation panicked: %v", failure)
}

// Control and Opened lend separate physical streams. Their wrappers deliberately
// expose neither Close nor a descriptor, so a borrower cannot release ownership.
func (s *session) Control() io.ReadWriter        { return sessionControl{owner: s} }
func (s *session) Opened() io.Reader             { return sessionOpened{owner: s} }
func (s *session) Identity() ppp.SessionIdentity { return s.identity }

type sessionControl struct{ owner *session }

func (b sessionControl) Read(buffer []byte) (int, error) { return b.owner.channel.Read(buffer) }
func (b sessionControl) Write(buffer []byte) (int, error) {
	s := b.owner
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	channel := s.channel
	s.mu.Unlock()
	// The immutable stream owns its descriptor lease. The session lock must
	// not prevent its Close operation while a write is in progress.
	return channel.Write(buffer)
}

type sessionOpened struct{ owner *session }

func (b sessionOpened) Read(buffer []byte) (int, error) { return b.owner.unit.Read(buffer) }

// InterruptRead is sticky for the control reader and leaves writes available
// until Close. The driver MUST join that reader before placing a relay bridge.
func (s *session) InterruptRead() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	interrupter, ok := s.channel.(readInterrupter)
	if !ok {
		return errors.New("ppp: session control reader cannot be interrupted for relay")
	}
	return interrupter.InterruptRead()
}

// Close wakes blocked readers before releasing their descriptors. The caller
// MUST join its readers after Close; joining first can wait forever for input.
// Safe for concurrent use with every operation and with repeated Close calls.
// Every resource is attempted once; failures and cleanup panics are returned
// together as an error, and every repeated Close returns that same result.
func (s *session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return s.closeErr
	}
	s.closed = true
	s.closeErr = errors.Join(
		closeSessionStream(s.channel, s.channelFD, s.ops.closeFD),
		closeSessionStream(s.unit, s.unitFD, s.ops.closeFD),
	)
	return s.closeErr
}

func (s *session) SetMTU(mtu int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	if mtu <= 0 {
		return errors.New("ppp: session MTU must be positive")
	}
	return s.backend.SetMTU(s.identity.InterfaceName, mtu)
}
func (s *session) Connect() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	if !s.connected {
		// Linux units default to passing network protocols. Both gates must be
		// closed before attachment can expose even an unconfigured family.
		if err := errors.Join(s.ops.setIPv4Mode(s.unitFD, NPModeDrop), s.ops.setIPv6Mode(s.unitFD, NPModeDrop)); err != nil {
			return err
		}
		if err := s.ops.connect(s.channelFD, s.unitNum); err != nil {
			return err
		}
		s.connected = true
	}
	// The common NCP driver calls Connect only after assignment acknowledgement.
	// An address alone is insufficient: a failed withdrawal remains cleanup-only.
	return errors.Join(s.ops.setIPv4Mode(s.unitFD, s.familyMode(true)), s.ops.setIPv6Mode(s.unitFD, s.familyMode(false)))
}
func (s *session) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	if !s.connected {
		return nil
	}
	if err := s.ops.disconnect(s.channelFD); err != nil {
		return err
	}
	s.connected = false
	return nil
}
func (s *session) SetReceiveMRU(mru uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	if mru == 0 {
		return errors.New("ppp: session receive MRU must be positive")
	}
	return s.ops.setMRU(s.unitFD, mru)
}
func (s *session) SetAdminUp() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	return s.backend.SetAdminUp(s.identity.InterfaceName)
}
func (s *session) AddAddressP2P(local, peer netip.Prefix) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	if !local.IsValid() {
		return errors.New("ppp: invalid local session address")
	}
	if !peer.IsValid() {
		return errors.New("ppp: invalid peer session address")
	}
	if local.Addr().Is4() != peer.Addr().Is4() {
		return errors.New("ppp: session address pair has different families")
	}
	if owned, ok := s.addresses[local]; ok {
		if owned.peer == peer && owned.state == sessionAddressEligible {
			return nil
		}
		return errors.New("ppp: session address is already owned; withdraw it before replacing")
	}
	if len(s.addresses) >= maxSessionAddresses {
		return errors.New("ppp: session address ownership limit reached")
	}
	if s.addresses == nil {
		s.addresses = make(map[netip.Prefix]sessionAddress)
	}
	// Retain cleanup ownership even when a backend reports a partial add. Only
	// a successful add may become eligible for the later NCP acknowledgement.
	s.addresses[local] = sessionAddress{peer: peer, state: sessionAddressRetiring}
	if err := s.backend.AddAddressP2P(s.identity.InterfaceName, local.String(), peer.String()); err != nil {
		return err
	}
	s.addresses[local] = sessionAddress{peer: peer, state: sessionAddressEligible}
	return nil
}

// RemoveAddress MUST retain an uncertain receipt until native withdrawal
// succeeds. The caller MUST retain its cleanup owner after an error and retry;
// absence of this owner's receipt is idempotent and never deletes native state.
func (s *session) RemoveAddress(local netip.Prefix) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	if !local.IsValid() {
		return errors.New("ppp: invalid local session address")
	}
	owned, ok := s.addresses[local]
	if !ok {
		// Cleanup may follow an add refused before an ownership receipt.
		// No native mutation is owed by this owner; never delete another
		// owner's address to make an absent receipt disappear.
		return nil
	}
	owned.state = sessionAddressRetiring
	s.addresses[local] = owned
	setMode := s.ops.setIPv6Mode
	if local.Addr().Is4() {
		setMode = s.ops.setIPv4Mode
	}
	if err := setMode(s.unitFD, NPModeDrop); err != nil {
		return err
	}
	if err := s.backend.RemoveAddress(s.identity.InterfaceName, local.String()); err != nil {
		return err
	}
	delete(s.addresses, local)
	return nil
}

// RFC 1661 section 3.6 requires discarding a protocol while its NCP is not
// Opened. A retiring pair vetoes the family, including after failed cleanup.
func (s *session) familyMode(ipv4 bool) NPMode {
	mode := NPModeDrop
	for local, owned := range s.addresses {
		if local.Addr().Is4() != ipv4 {
			continue
		}
		if owned.state == sessionAddressRetiring {
			return NPModeDrop
		}
		mode = NPModePass
	}
	return mode
}

// AddRoute MUST retain an exact cleanup receipt before invoking the backend,
// including when that call returns an uncertain result. Callers MUST withdraw
// an attempted route through RemoveRoute and retain this owner for retries.
// Safe for concurrent use; native mutation keeps the unit lease locked.
func (s *session) AddRoute(destination netip.Prefix, gateway netip.Addr, metric int, protocol rtproto.Proto) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	route, err := newSessionRoute(destination, gateway, metric, protocol)
	if err != nil {
		return err
	}
	if protocol == rtproto.Any {
		return errors.New("ppp: session route protocol names no producer")
	}
	if _, owned := s.routes[route]; !owned {
		// Linux treats deletion priority zero as a wildcard, including
		// explicit zero. Retained uncertain paths must stay unambiguous.
		for retained := range s.routes {
			if retained.destination != route.destination {
				continue
			}
			if retained.gateway != route.gateway {
				continue
			}
			if retained.protocol != route.protocol {
				continue
			}
			if retained.metric > 0 {
				if route.metric > 0 {
					continue
				}
			}
			return errors.New("ppp: session route priorities make zero-metric withdrawal ambiguous")
		}
		if len(s.routes) >= maxSessionRoutes {
			return errors.New("ppp: session route ownership limit reached")
		}
		if s.routes == nil {
			s.routes = make(map[sessionRoute]struct{})
		}
		// The backend exposes only an error, so an invoked add may have
		// installed its path before failing. Keep that exact cleanup intent.
		s.routes[route] = struct{}{}
	}
	return s.backend.AddRoute(s.identity.InterfaceName, route.destination.String(), route.gateway.String(), route.metric, route.protocol)
}

// RemoveRoute MUST withdraw only this owner's exact cleanup receipt. A missing
// receipt is already complete and performs no native operation. An error keeps
// the receipt; callers MUST retain this owner and retry that same withdrawal.
// Safe for concurrent use; Close cannot recycle the unit during native I/O.
func (s *session) RemoveRoute(destination netip.Prefix, gateway netip.Addr, metric int, protocol rtproto.Proto) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	route, err := newSessionRoute(destination, gateway, metric, protocol)
	if err != nil {
		return err
	}
	if _, owned := s.routes[route]; !owned {
		// No attempted mutation belongs to this owner. Never turn another
		// producer's matching route into this session's cleanup obligation.
		return nil
	}
	if err := s.backend.RemoveRoute(s.identity.InterfaceName, route.destination.String(), route.gateway.String(), route.metric, route.protocol); err != nil {
		return err
	}
	delete(s.routes, route)
	return nil
}
func (s *session) Counters() (ppp.SessionCounters, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ppp.SessionCounters{}, io.ErrClosedPipe
	}
	return s.readCounters(s.identity.InterfaceName)
}
func (s *session) OpenIPv6Carrier(ctx context.Context) (ppp.IPv6Carrier, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, io.ErrClosedPipe
	}
	return openIPv6Carrier(ctx, s.identity.InterfaceName)
}

var _ ppp.FrameChannel = (*session)(nil)
var _ ppp.SessionHandle = (*session)(nil)
