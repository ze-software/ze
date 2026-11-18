// Design: docs/features/interfaces.md -- Creation-bound raw counter readers
// Related: counter_linux.go -- Ordered notification history and indexed queries
// Related: counter_create_request_linux.go -- Immutable exclusive request envelope
// Upstream: github.com/vishvananda/netlink v1.3.1, NetlinkSocket Send and GetPid:
// https://github.com/vishvananda/netlink/blob/v1.3.1/nl/nl_linux.go
// Upstream: github.com/vishvananda/netns v0.0.5, current-thread namespace Get:
// https://github.com/vishvananda/netns/blob/v0.0.5/netns_linux.go
// Upstream: Linux v7.3, socket namespace SIOCGSKNS:
// https://github.com/torvalds/linux/blob/v7.3/net/core/sock.c

//go:build linux

package ifacenetlink

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// CreateLinkCounterReader creates exclusively on the backend's ordered socket.
// Safe for concurrent use. The caller MUST keep input immutable until return
// and MUST retain the backend until its last reader call; its owner MUST then
// call Close. Readers borrow that lifetime and own no socket or Close operation.
// An attempted request can have created a link even when no echo was received.
// A partial echo is returned with a permanently refused reader on later error;
// the caller MUST preserve these facts when deciding native cleanup custody.
func (b *netlinkBackend) CreateLinkCounterReader(input *nl.NetlinkRequest) (netlink.Link, func() (*netlink.LinkStatistics, error), bool, error) {
	request, err := newCounterCreateRequest(input)
	if err != nil {
		return nil, nil, false, err
	}
	// The namespace proof and Send must describe the same calling thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	s := &b.counters
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, nil, false, os.ErrClosed
	}
	if s.socket == nil {
		if err := s.open(); err != nil {
			return nil, nil, false, err
		}
	}
	if err := s.checkCreateNamespace(); err != nil {
		return nil, nil, false, err
	}
	portID, err := s.socket.GetPid()
	if err != nil {
		return nil, nil, false, err
	}
	dump := counterDump{sequence: request.Seq, portID: portID, single: true}
	// From this point even an uncertain Send belongs to the caller's attempt.
	if err := s.socket.Send(request); err != nil {
		s.invalidate()
		return s.finishCreate(&dump, err)
	}
	if err := s.receiveCreate(&dump); err != nil {
		s.invalidate()
		return s.finishCreate(&dump, err)
	}
	return s.finishCreate(&dump, nil)
}

// checkCreateNamespace requires mu and a pinned OS thread. Both temporary FDs
// MUST be closed before return, so cleanup errors are known before any Send.
func (s *counterSource) checkCreateNamespace() (err error) {
	socketNamespace, err := unix.IoctlRetInt(s.socket.GetFd(), unix.SIOCGSKNS)
	if err != nil {
		return fmt.Errorf("counter socket namespace: %w", err)
	}
	defer func() {
		if closeErr := unix.Close(socketNamespace); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close counter socket namespace: %w", closeErr))
		}
	}()
	callerNamespace, err := netns.Get()
	if err != nil {
		return fmt.Errorf("counter caller namespace: %w", err)
	}
	defer func() {
		if closeErr := unix.Close(int(callerNamespace)); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close counter caller namespace: %w", closeErr))
		}
	}()
	var socketIdentity, callerIdentity unix.Stat_t
	if err := unix.Fstat(socketNamespace, &socketIdentity); err != nil {
		return fmt.Errorf("counter socket namespace identity: %w", err)
	}
	if err := unix.Fstat(int(callerNamespace), &callerIdentity); err != nil {
		return fmt.Errorf("counter caller namespace identity: %w", err)
	}
	if socketIdentity.Dev != callerIdentity.Dev {
		return errors.New("counter creation caller is outside the socket namespace")
	}
	if socketIdentity.Ino != callerIdentity.Ino {
		return errors.New("counter creation caller is outside the socket namespace")
	}
	return nil
}

// finishCreate requires mu and MUST only follow an attempted Send. It retains
// the first actual echo rather than looking up a possibly replaced state.
func (s *counterSource) finishCreate(dump *counterDump, cause error) (netlink.Link, func() (*netlink.LinkStatistics, error), bool, error) {
	if dump.failure != nil {
		if !errors.Is(cause, dump.failure) {
			cause = errors.Join(cause, dump.failure)
		}
	}
	if len(dump.links) == 0 {
		if cause == nil {
			cause = errors.New("counter creation has no confirmed link echo")
		}
		return nil, nil, true, cause
	}
	original := dump.links[0]
	index := original.link.Attrs().Index
	generation := original.generation
	if cause == nil {
		if !dump.createComplete() {
			cause = errors.New("counter creation acknowledgment is incomplete")
		}
	}
	if cause == nil {
		if index <= 0 {
			cause = errors.New("counter creation echo has no native index")
		}
	}
	if cause == nil {
		if original.link.Attrs().Name == "" {
			cause = errors.New("counter creation echo has no native name")
		}
	}
	if cause == nil {
		if s.states[index].generation != generation {
			cause = errors.New("counter creation original generation has ended")
		}
	}
	if cause != nil {
		return original.link, func() (*netlink.LinkStatistics, error) { return nil, cause }, true, cause
	}
	return original.link, func() (*netlink.LinkStatistics, error) {
		return s.readBoundCounters(index, generation)
	}, true, nil
}

// readBoundCounters borrows the backend lifetime. It MUST NOT reopen a lost
// socket: neither a new namespace nor fresh history can heal an old receipt.
func (s *counterSource) readBoundCounters(index int, generation uint64) (*netlink.LinkStatistics, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, os.ErrClosed
	}
	if s.states[index].generation != generation {
		return nil, errors.New("counter reader original generation has ended")
	}
	if s.socket == nil {
		return nil, errors.New("counter reader notification history is unavailable")
	}
	dump, err := s.dumpLink("", index)
	if err != nil {
		s.invalidate()
		return nil, fmt.Errorf("bound counter snapshot: %w", err)
	}
	if errors.Is(dump.failure, unix.ENODEV) {
		delete(s.states, index)
	}
	return s.boundCounters(&dump, index, generation)
}

// boundCounters requires mu and selects only a current reply of the original
// generation. Missing measurements never borrow the cached previous sample.
func (s *counterSource) boundCounters(dump *counterDump, index int, generation uint64) (*netlink.LinkStatistics, error) {
	if s.closed {
		return nil, os.ErrClosed
	}
	if s.states[index].generation != generation {
		return nil, errors.New("counter reader original generation has ended")
	}
	if dump.failure != nil {
		return nil, dump.failure
	}
	for _, current := range dump.links {
		if current.link.Attrs().Index != index {
			continue
		}
		if current.generation != generation {
			return nil, errors.New("counter reply is outside the original generation")
		}
		stats := current.link.Attrs().Statistics
		if stats == nil {
			return nil, errors.New("counter reply has no current statistics")
		}
		measurements := *stats
		return &measurements, nil
	}
	return nil, errors.New("counter reply does not contain the original interface")
}
