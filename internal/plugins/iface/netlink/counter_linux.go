// Design: docs/features/interfaces.md -- Raw interface counter continuity
// Related: show_linux.go -- Raw snapshot metadata and counter conversion

//go:build linux

package ifacenetlink

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// Counter dumps are bounded in time and memory even when links churn faster
// than the reader. Exceeding a bound discards the sample, never its events.
const (
	counterDumpTimeout = 5 * time.Second
	counterLinksMax    = 1 << 20
)

// Tokens are process-wide so replacing a backend cannot reuse an old token.
var counterGeneration atomic.Uint64

// counterSource orders link notifications and counter reads on one socket.
// A separate monitor socket cannot establish which incarnation a dump describes.
// Safe for concurrent use through snapshot and close. The backend MUST call
// close before it is discarded; close permanently prevents new socket opens.
type counterSource struct {
	mu     sync.Mutex
	socket *nl.NetlinkSocket
	states map[int]counterState
	closed bool
}

type counterState struct {
	generation uint64
	stats      netlink.LinkStatistics
	hasStats   bool
}

type counterLink struct {
	link       netlink.Link
	generation uint64
}

type counterDump struct {
	sequence    uint32
	portID      uint32
	links       []counterLink
	interrupted bool
	done        bool
	single      bool
	failure     error
}

func (s *counterSource) snapshot(name string) ([]counterLink, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, os.ErrClosed
	}
	if s.socket == nil {
		if err := s.open(); err != nil {
			return nil, err
		}
	}
	for attempt := 0; attempt <= linkListRetries; attempt++ {
		dump, err := s.dump(name)
		if err == nil {
			if dump.failure != nil {
				return nil, dump.failure
			}
			return dump.links, nil
		}
		if errors.Is(err, netlink.ErrDumpInterrupted) {
			continue
		}
		// ENOBUFS can hide a complete delete/recreate. A new dump cannot
		// recover that history, so no previous generation remains valid.
		s.invalidate()
		return nil, fmt.Errorf("counter snapshot: %w", err)
	}
	return nil, fmt.Errorf("counter snapshot after %d dumps: %w", linkListRetries+1, netlink.ErrDumpInterrupted)
}

func (s *counterSource) open() error {
	// Subscribe after bind: nl.Subscribe with a group also sets the Send
	// destination group. Requests here MUST be unicast to the kernel.
	socket, err := nl.Subscribe(unix.NETLINK_ROUTE)
	if err != nil {
		return fmt.Errorf("counter socket: %w", err)
	}
	if err := unix.SetsockoptInt(socket.GetFd(), unix.SOL_NETLINK, unix.NETLINK_ADD_MEMBERSHIP, unix.RTNLGRP_LINK); err != nil {
		socket.Close()
		return fmt.Errorf("counter link subscription: %w", err)
	}
	if err := socket.SetReceiveBufferSize(monitorReceiveBufferBytes, false); err != nil {
		socket.Close()
		return fmt.Errorf("counter receive buffer: %w", err)
	}
	timeout := unix.NsecToTimeval(counterDumpTimeout.Nanoseconds())
	if err := socket.SetSendTimeout(&timeout); err != nil {
		socket.Close()
		return fmt.Errorf("counter send timeout: %w", err)
	}
	s.socket = socket
	return nil
}

func (s *counterSource) dump(name string) (counterDump, error) {
	flags := unix.NLM_F_DUMP
	if name != "" {
		flags = unix.NLM_F_ACK
	}
	request := nl.NewNetlinkRequest(unix.RTM_GETLINK, flags)
	request.AddData(nl.NewIfInfomsg(unix.AF_UNSPEC))
	request.AddData(nl.NewRtAttr(unix.IFLA_EXT_MASK, nl.Uint32Attr(nl.RTEXT_FILTER_VF)))
	if name != "" {
		request.AddData(nl.NewRtAttr(unix.IFLA_IFNAME, nl.ZeroTerminated(name)))
	}
	if err := s.socket.Send(request); err != nil {
		return counterDump{}, err
	}
	portID, err := s.socket.GetPid()
	if err != nil {
		return counterDump{}, err
	}
	dump := counterDump{sequence: request.Seq, portID: portID, single: name != ""}
	deadline := time.Now().Add(counterDumpTimeout)
	// NLMSG_DONE, the single-link ACK, or the absolute deadline ends the
	// receive loop. Multicast traffic cannot extend that deadline.
	for !dump.done {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return counterDump{}, unix.ETIMEDOUT
		}
		timeout := unix.NsecToTimeval(remaining.Nanoseconds())
		if err := s.socket.SetReceiveTimeout(&timeout); err != nil {
			return counterDump{}, err
		}
		messages, from, err := s.socket.Receive()
		if err != nil {
			return counterDump{}, err
		}
		if from.Pid != nl.PidKernel {
			return counterDump{}, fmt.Errorf("counter message sender %d is not the kernel", from.Pid)
		}
		for _, message := range messages {
			if err := s.consume(&dump, message); err != nil {
				if errors.Is(err, netlink.ErrDumpInterrupted) {
					continue
				}
				return counterDump{}, err
			}
		}
	}
	if dump.interrupted {
		return counterDump{}, netlink.ErrDumpInterrupted
	}
	return dump, nil
}

// consume is called with mu held, in socket order. Only dump replies become
// samples; notifications update continuity before later replies are stamped.
func (s *counterSource) consume(dump *counterDump, message syscall.NetlinkMessage) error {
	reply := message.Header.Seq == dump.sequence && message.Header.Pid == dump.portID
	if reply {
		if message.Header.Flags&unix.NLM_F_DUMP_INTR != 0 {
			dump.interrupted = true
		}
	}
	switch message.Header.Type {
	case unix.NLMSG_NOOP:
		return nil
	case unix.NLMSG_OVERRUN:
		return unix.ENOBUFS
	case unix.NLMSG_DONE:
		if !reply {
			return errors.New("counter dump completion has an unexpected sequence")
		}
		dump.done = true
		if len(message.Data) > 0 {
			if len(message.Data) < 4 {
				return errors.New("short counter dump completion")
			}
		}
		if len(message.Data) >= 4 {
			if code := int32(nl.NativeEndian().Uint32(message.Data)); code != 0 {
				dump.failure = syscall.Errno(-code)
			}
		}
		if dump.interrupted {
			return netlink.ErrDumpInterrupted
		}
		return nil
	case unix.NLMSG_ERROR:
		if !reply {
			return errors.New("counter error has an unexpected request identity")
		}
		if len(message.Data) < 4 {
			return errors.New("short counter netlink error")
		}
		if code := int32(nl.NativeEndian().Uint32(message.Data)); code != 0 {
			dump.failure = syscall.Errno(-code)
			dump.done = true
		}
		if dump.single {
			dump.done = true
		}
		return nil
	case unix.RTM_NEWLINK, unix.RTM_DELLINK:
		if len(message.Data) < unix.SizeofIfInfomsg {
			return errors.New("short counter link message")
		}
		// AF_BRIDGE DELLINK removes bridge membership, not the netdevice.
		if message.Data[0] != unix.AF_UNSPEC {
			return nil
		}
	default:
		return nil
	}
	if message.Header.Type == unix.RTM_DELLINK {
		delete(s.states, int(nl.DeserializeIfInfomsg(message.Data).Index))
		return nil
	}
	header := unix.NlMsghdr(message.Header)
	link, err := netlink.LinkDeserialize(&header, message.Data)
	if err != nil {
		return err
	}
	index := link.Attrs().Index
	if s.states == nil {
		s.states = make(map[int]counterState)
	}
	state, exists := s.states[index]
	if !exists {
		if len(s.states) >= counterLinksMax {
			return errors.New("counter interface limit exceeded")
		}
		state.generation = counterGeneration.Add(1)
	}
	if stats := link.Attrs().Statistics; stats != nil {
		if state.hasStats {
			if counterStatsDecreased(stats, &state.stats) {
				state.generation = counterGeneration.Add(1)
			}
		}
		state.stats = *stats
		state.hasStats = true
	}
	s.states[index] = state
	if reply {
		if len(dump.links) >= counterLinksMax {
			return errors.New("counter dump interface limit exceeded")
		}
		dump.links = append(dump.links, counterLink{link: link, generation: state.generation})
	}
	return nil
}

// Compare the raw widths, not sFlow's low 32-bit packet/error counters. A
// 32-bit wire wrap is continuous. A hidden reset that has already overtaken
// the previous raw value cannot be inferred from rtnl_link_stats64.
func counterStatsDecreased(current, previous *netlink.LinkStatistics) bool {
	return current.RxBytes < previous.RxBytes ||
		current.RxPackets < previous.RxPackets ||
		current.RxErrors < previous.RxErrors ||
		current.RxDropped < previous.RxDropped ||
		current.Multicast < previous.Multicast ||
		current.TxBytes < previous.TxBytes ||
		current.TxPackets < previous.TxPackets ||
		current.TxErrors < previous.TxErrors ||
		current.TxDropped < previous.TxDropped
}

// invalidate requires mu and drops continuity when the notification history
// cannot be recovered. The next successful dump assigns fresh tokens.
func (s *counterSource) invalidate() {
	if s.socket != nil {
		s.socket.Close()
		s.socket = nil
	}
	s.states = nil
}

// close MUST be called before the backend is discarded. It waits for a bounded
// in-flight dump and releases its socket and all per-interface history.
func (s *counterSource) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.invalidate()
}
