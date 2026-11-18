// Design: docs/features/interfaces.md -- Ordered exclusive creation replies
// Related: counter_linux.go -- Shared link continuity decoder
// Upstream: github.com/vishvananda/netlink v1.3.1, NetlinkSocket Receive:
// https://github.com/vishvananda/netlink/blob/v1.3.1/nl/nl_linux.go
// Upstream: Linux v7.3, netlink ACK/ECHO and rtnetlink link replies:
// https://github.com/torvalds/linux/blob/v7.3/net/netlink/af_netlink.c
// https://github.com/torvalds/linux/blob/v7.3/net/core/rtnetlink.c

//go:build linux

package ifacenetlink

import (
	"errors"
	"fmt"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

func (s *counterSource) receiveCreate(dump *counterDump) error {
	deadline := time.Now().Add(counterDumpTimeout)
	// Both matched ECHO and ACK, a negative ACK, or the absolute deadline end
	// this loop; multicast traffic cannot renew the five-second budget.
	for !dump.createComplete() {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return unix.ETIMEDOUT
		}
		timeout := unix.NsecToTimeval(remaining.Nanoseconds())
		if err := s.socket.SetReceiveTimeout(&timeout); err != nil {
			return err
		}
		messages, from, err := s.socket.Receive()
		if err != nil {
			return err
		}
		if err := s.consumeCreate(dump, messages, from.Pid); err != nil {
			return err
		}
	}
	return nil
}

func (dump *counterDump) createComplete() bool {
	if dump.failure != nil {
		return true
	}
	if !dump.done {
		return false
	}
	return len(dump.links) > 0
}

// consumeCreate requires mu and consumes the complete batch in socket order,
// including notifications after a reply. Only the matched echo supplies custody.
func (s *counterSource) consumeCreate(dump *counterDump, messages []syscall.NetlinkMessage, sender uint32) error {
	if sender != nl.PidKernel {
		return fmt.Errorf("counter creation sender %d is not the kernel", sender)
	}
	acknowledgedBeforeBatch := dump.done
	// A successful ACK without an echo keeps receiveCreate waiting. A later
	// contradictory negative ACK cannot erase that earlier completion history.
	if prior, confirmed := dump.failure.(counterCreateNegativeACK); confirmed { //nolint:errorlint // Only the exact private batch receipt grants authority; wrapped causes remain uncertain.
		dump.failure = syscall.Errno(prior)
	}
	var rejection counterCreateNegativeACK
	acknowledgments := 0
	for _, message := range messages {
		if message.Header.Type == unix.NLMSG_DONE {
			return errors.New("counter creation received unexpected dump completion")
		}
		if err := s.consume(dump, message); err != nil {
			return err
		}
		if dump.interrupted {
			return netlink.ErrDumpInterrupted
		}
		if message.Header.Type == unix.NLMSG_ERROR {
			acknowledgments++
			rejection = counterCreateNegativeReply(dump, message)
		}
	}
	// Only one validated original negative ACK in a fully consumed batch can
	// carry this private evidence. It is not yet a public safe-retry receipt.
	if acknowledgedBeforeBatch {
		return nil
	}
	if acknowledgments != 1 {
		return nil
	}
	if rejection == 0 {
		return nil
	}
	if len(dump.links) != 0 {
		return nil
	}
	dump.failure = rejection
	return nil
}
