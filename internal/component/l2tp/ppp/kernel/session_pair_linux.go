//go:build linux

// Design: docs/research/l2tpv2-ze-integration.md -- original-file PPP attachment.
// Related: client_linux.go -- bounded read attempt inside private file leases.
// Linux API: https://github.com/torvalds/linux/blob/v7.3/drivers/net/ppp/ppp_generic.c
// Native ABI: Root0010, documented in appliance/kernel-profiles.md.

package kernel

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// withOriginalFiles MUST run with pair.mu held. This private helper retains
// owner mutexes and both real RawConn leases. Only the bounded client read may
// poll here; no public WithDescriptor callback waits for network input.
func (pair *claimedSessionPair) withOriginalFiles(operation func(*os.File, int, int) error) error {
	return pair.withRetainedOriginalFiles(func(file *os.File, channelFD, unitFD int) error {
		if err := originalPairLive(channelFD, unitFD); err != nil {
			return err
		}
		return operation(file, channelFD, unitFD)
	})
}

// Same custody checks and bounded leases, without live-only polling. Native
// exact-pair disconnect may settle a retired original unit/channel, and must
// not be blocked by a fresh liveness or creator query after retirement.
func (pair *claimedSessionPair) withRetainedOriginalFiles(operation func(*os.File, int, int) error) error {
	unitLease, ok := pair.unit.(*claimedCreatedUnit)
	if !ok {
		return errors.New("ppp: native attachment requires the original unit claim")
	}
	channelLease, ok := pair.channel.(*claimedCreatedChannel)
	if !ok {
		return errors.New("ppp: native attachment requires the original channel claim")
	}
	unit := unitLease.unit
	channel := channelLease.channel
	unit.mu.Lock()
	defer unit.mu.Unlock()
	channel.mu.Lock()
	defer channel.mu.Unlock()
	if unit.closed {
		return os.ErrClosed
	}
	if channel.closed {
		return os.ErrClosed
	}
	if unit.self != unit {
		return errors.New("ppp: original unit owner was copied")
	}
	if channel.self != channel {
		return errors.New("ppp: original channel owner was copied")
	}
	if unit.sessionClaim != unitLease {
		return errors.New("ppp: original unit claim changed")
	}
	if channel.sessionClaim != channelLease {
		return errors.New("ppp: original channel claim changed")
	}
	if !unit.ready {
		return errors.New("ppp: original unit is not confirmed")
	}
	if !channel.ready {
		return errors.New("ppp: original channel is not confirmed")
	}
	if unitLease.file == nil {
		return os.ErrClosed
	}
	if channelLease.file == nil {
		return os.ErrClosed
	}
	if unit.file != unitLease.file {
		return errors.New("ppp: original unit file changed")
	}
	if channel.file != channelLease.file {
		return errors.New("ppp: original channel file changed")
	}
	if unit.identity != pair.info.identity {
		return errors.New("ppp: original unit receipt changed")
	}
	if unit.number != pair.info.number {
		return errors.New("ppp: original unit number changed")
	}
	if err := validateSessionConfig(pair.cfg, sessionUnitDescription{
		identity: unit.identity, fd: unitLease.fd, number: unit.number,
		backend: unit.backend, namespace: unit.namespace,
	}); err != nil {
		return err
	}
	if err := validateSessionNamespace(channel.namespace, unit.namespace); err != nil {
		return err
	}
	unitConnection, err := unitLease.file.SyscallConn()
	if err != nil {
		return err
	}
	channelConnection, err := channelLease.file.SyscallConn()
	if err != nil {
		return err
	}
	var operationErr error
	err = unitConnection.Control(func(unitFD uintptr) {
		if int(unitFD) != unitLease.fd {
			operationErr = errors.New("ppp: original unit descriptor changed")
			return
		}
		channelErr := channelConnection.Control(func(channelFD uintptr) {
			if int(channelFD) != channelLease.fd {
				operationErr = errors.New("ppp: original channel descriptor changed")
				return
			}
			operationErr = operation(channelLease.file, int(channelFD), int(unitFD))
		})
		operationErr = errors.Join(operationErr, channelErr)
	})
	return errors.Join(err, operationErr)
}

// Polling is nonblocking and EINTR returns to the caller, rather than keeping
// an externally held lifecycle lock through an unbounded signal retry loop.
func originalPairLive(channelFD, unitFD int) error {
	fds := [2]unix.PollFd{{Fd: int32(channelFD), Events: unix.POLLIN}, {Fd: int32(unitFD), Events: unix.POLLIN}}
	if _, err := unix.Poll(fds[:], 0); err != nil {
		return err
	}
	for _, fd := range fds {
		if fd.Revents&unix.POLLNVAL != 0 {
			return unix.EBADF
		}
		if fd.Revents&unix.POLLHUP != 0 {
			return unix.ENXIO
		}
		if fd.Revents&unix.POLLERR != 0 {
			return unix.EIO
		}
	}
	return nil
}

// Called by typed server Connect; the existing server mutex precedes pair.mu.
// Client Connect already holds pair.mu and calls connectOriginalPairLocked.
func connectOriginalSessionPair(pair *claimedSessionPair) error {
	pair.mu.Lock()
	defer pair.mu.Unlock()
	if pair.channelClosed {
		return os.ErrClosed
	}
	if pair.unitClosed {
		return os.ErrClosed
	}
	return connectOriginalPairLocked(pair)
}

func connectOriginalPairLocked(pair *claimedSessionPair) error {
	return connectOriginalPairAfterLive(pair, nil)
}

// The server retains this SAME pair across disconnect/reconnect. Its state
// machine continues to own timing, family gates and the connected flag.
func disconnectOriginalSessionPair(pair *claimedSessionPair) error {
	pair.mu.Lock()
	defer pair.mu.Unlock()
	if pair.channelClosed {
		return os.ErrClosed
	}
	if pair.unitClosed {
		return os.ErrClosed
	}
	if pair.original == nil {
		return errors.New("ppp: original pair has no native capability receipt")
	}
	return pair.withRetainedOriginalFiles(func(_ *os.File, channelFD, unitFD int) error {
		if err := pppPairPeerOperation(channelFD, pppPairDisconnect, unitFD); err != nil {
			return originalAttachmentRefusal(err)
		}
		return nil
	})
}

// prepareOriginalSessionPair refuses missing/unknown primitive support before
// admitting a typed server or client lifetime, with complete pair rollback.
func prepareOriginalSessionPair(pair *claimedSessionPair) error {
	unitLease, ok := pair.unit.(*claimedCreatedUnit)
	if !ok {
		return errors.New("ppp: native preparation requires the original unit claim")
	}
	// Root query51 explicitly checks current->nsproxy->net_ns. Enter through
	// the SAME joined restore-safe primitive C1 owns, before pair/owner locks
	// and any descriptor lease. The namespace pointer is immutable after birth.
	result := runOriginalPPPNamespace(unitLease.unit.namespace, func() error {
		if err := probeOriginalPairAttachment(); err != nil {
			return err
		}
		pair.mu.Lock()
		defer pair.mu.Unlock()
		return pair.withOriginalFiles(func(_ *os.File, channelFD, unitFD int) error {
			if err := inspectOriginalPairFiles(pair, channelFD, unitFD); err != nil {
				return originalAttachmentRefusal(err)
			}
			return nil
		})
	})
	return result.err()
}

// Admission and the dependency-owned doctor share one nonmutating probe.
func probeOriginalPairAttachment() error { return checkOriginalAttachment() }
