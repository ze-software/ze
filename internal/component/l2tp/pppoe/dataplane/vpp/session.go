// Design: docs/architecture/l2tp/bng-5-pppoe.md -- session-owned VPP forwarding
// Related: factory.go -- creation before PADS; session_routes.go -- owned paths
package vpp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"sync"
	"time"

	"go.fd.io/govpp/api"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// session owns both the VPP interface and its one polled control socket. PPP
// borrows it as a FrameChannel and owns it as SessionHandle; the AC uses the
// same idempotent Close for immediate PADT gating. Close wakes readers before
// removing routes and the interface, so no recycled descriptor is used. All
// control/data-plane operations are safe for concurrent use; mu serializes API
// state changes, while socket Close interrupts blocking I/O independently.
type session struct {
	mu       sync.Mutex
	identity ppp.SessionIdentity
	api      *binaryAPI
	socket   *controlSocket
	tables   [2]uint32
	source   uint8
	sid      uint16
	mac      [6]byte
	// readCounters reads the created index from the shared statistics
	// snapshot; created is when that index existed, so no snapshot taken
	// before it can answer for this session.
	readCounters counterReader
	created      time.Time
	routes       map[sessionRoute]struct{} // confirmed or uncertain native paths
	// stagedRoutes holds unpublished route intents. Failed adds or family
	// withdrawals can owe both native cleanup and later publication; a retained
	// withdrawal owner blocks publication until the old native paths are gone.
	stagedRoutes map[sessionRoute]struct{}
	addresses    map[netip.Prefix]sessionRoute
	// A withdrawing pair leaves addresses before its first native mutation.
	// It remains here until its exact peer and any closing-family paths are gone.
	withdrawals map[netip.Prefix]sessionRoute
	ready       bool // transport readiness; each negotiated family gates separately
	closed      bool
	closeErr    error
	release     func()
	retain      func(error)
}

func (s *session) Identity() ppp.SessionIdentity { return s.identity }
func (s *session) Control() io.ReadWriter        { return s.socket }

// A socket has one PPP reader throughout NCP; no post-connect unit exists.
func (s *session) Opened() io.Reader    { return nil }
func (s *session) InterruptRead() error { return s.socket.InterruptRead() }

// Close invalidates the control endpoint first, then removes every path this
// owner installed before deleting the session. Cleanup is bounded by one API
// deadline for the whole teardown, and every failure remains in the cached
// result; a failed delete is never reported as successful release.
func (s *session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return s.closeErr
	}
	s.closed = true
	clear(s.stagedRoutes)
	s.closeErr = s.socket.Close()
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	cleanupErr := s.closeNative(ctx)
	if cleanupErr != nil {
		s.retain(cleanupErr)
	}
	s.closeErr = errors.Join(s.closeErr, cleanupErr)
	s.release()
	return s.closeErr
}

// closeNative MUST run with mu held after the frame owner has closed. Retried
// cleanup uses this same sequence without reviving the closed session. Every
// native child owner, including policers, MUST finish before session deletion.
func (s *session) closeNative(ctx context.Context) error {
	if s.api.processExited() {
		clear(s.routes)
		clear(s.stagedRoutes)
		clear(s.addresses)
		clear(s.withdrawals)
		return nil
	}
	cleanupErr := s.api.setAdmin(ctx, s.identity.NativeIndex, false)
	// Inspect both arcs of both families even after a partial/unknown Connect.
	// An unset local flag cannot prove native forwarding is quarantined.
	for family := range 2 {
		cleanupErr = errors.Join(cleanupErr, s.api.setIP(ctx, s.identity.NativeIndex, family == 1, false))
	}
	for route := range s.routes {
		receipt, err := s.programRoute(ctx, false, route)
		cleanupErr = errors.Join(cleanupErr, err)
		if receipt == routeConfirmed {
			delete(s.routes, route)
		}
	}
	if cleanupErr == nil && len(s.routes) == 0 {
		cleanupErr = s.api.remove(ctx, s.sid, s.mac, s.tables[0])
	}
	return cleanupErr
}

func (s *session) SetMTU(mtu int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	if mtu < ppp.MinMRU {
		return fmt.Errorf("pppoe-vpp: MTU %d is below %d", mtu, ppp.MinMRU)
	}
	if mtu > ppp.MaxFrameLen {
		return fmt.Errorf("pppoe-vpp: MTU %d exceeds supported PPP payload %d", mtu, ppp.MaxFrameLen)
	}
	return s.api.setMTU(context.Background(), s.identity.NativeIndex, mtu)
}

// Connect MUST follow the common PPP owner's assignment acknowledgement.
// AddAddressP2P MUST precede that acknowledgement and this publication barrier.
// Each call publishes staged routes only for families with an address pair,
// then opens only fully confirmed families. IPv6 service startup calls it again
// after installing its late link-local pair. No NCP state is inferred here.
func (s *session) Connect() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	s.ready = true
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	var err error
	for family := range 2 {
		ipv6 := family == 1
		if s.hasWithdrawalFamily(ipv6) {
			err = errors.Join(err, s.api.setIP(ctx, s.identity.NativeIndex, ipv6, false))
			continue
		}
		if s.hasAddressFamily(ipv6) {
			if publishErr := s.publishRoutes(ctx, ipv6); publishErr != nil {
				err = errors.Join(err, publishErr, s.api.setIP(ctx, s.identity.NativeIndex, ipv6, false))
				continue
			}
		}
		err = errors.Join(err, s.api.setIP(ctx, s.identity.NativeIndex, ipv6, s.addressFamilyCount(ipv6) != 0))
	}
	return err
}

func (s *session) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	var err error
	for family := range 2 {
		err = errors.Join(err, s.api.setIP(context.Background(), s.identity.NativeIndex, family == 1, false))
	}
	if err == nil {
		s.ready = false
	}
	return err
}

// SetReceiveMRU selects the fixed 1500-byte control receive window required by
// RFC 1661 Section 6.1. AF_PPPOX is a datagram endpoint with no PPP unit ioctl;
// its bounded reader rejects oversized datagrams instead of truncating them.
func (s *session) SetReceiveMRU(mru uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	if mru != ppp.MaxFrameLen {
		return fmt.Errorf("pppoe-vpp: socket receive MRU must be %d, got %d", ppp.MaxFrameLen, mru)
	}
	s.socket.receiveMRU.Store(int32(mru))
	return nil
}

func (s *session) SetAdminUp() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	return s.api.setAdmin(context.Background(), s.identity.NativeIndex, true)
}

// counterReader answers one created interface's counters from a statistics
// dump that started no earlier than notBefore. A failed read, a missing row and
// a duplicate row are errors (vppcomp.InterfaceCountersForGeneration).
type counterReader func(index uint32, notBefore time.Time) (api.InterfaceCounters, error)

// Counters is a fresh read: the dump starts after this call. The accounting
// baseline and the terminal read after Disconnect use it, because an older
// dump would understate the baseline or drop the last traffic before Stop.
// Concurrent fresh reads of many sessions still share one dump.
func (s *session) Counters() (ppp.SessionCounters, error) {
	return s.countersSince(time.Now())
}

// CountersWithin serves an interval reader, which reads every session once
// per interval: all sessions read within maxAge share one VPP dump instead of
// taking one dump each. The answer was measured no earlier than maxAge before
// the call and never before the session's interface existed. maxAge <= 0 is
// a fresh read. Safe for concurrent use.
func (s *session) CountersWithin(maxAge time.Duration) (ppp.SessionCounters, error) {
	now := time.Now()
	if maxAge <= 0 {
		return s.countersSince(now)
	}
	return s.countersSince(now.Add(-maxAge))
}

func (s *session) countersSince(notBefore time.Time) (ppp.SessionCounters, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ppp.SessionCounters{}, io.ErrClosedPipe
	}
	if s.readCounters == nil {
		return ppp.SessionCounters{}, errors.New("pppoe-vpp: VPP stats connection is unavailable")
	}
	// A dump that started before the interface was created has no row for it.
	if notBefore.Before(s.created) {
		notBefore = s.created
	}
	counter, err := s.readCounters(s.identity.NativeIndex, notBefore)
	if err != nil {
		return ppp.SessionCounters{}, err
	}
	return ppp.SessionCounters{RxBytes: counter.Rx.Bytes, TxBytes: counter.Tx.Bytes, RxPackets: counter.Rx.Packets, TxPackets: counter.Tx.Packets}, nil
}

var _ ppp.SessionHandle = (*session)(nil)
var _ ppp.FrameChannel = (*session)(nil)
