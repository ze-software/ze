//go:build linux

// Design: docs/research/vpp-deployment-reference.md -- Linux pidfs process identity
// Related: process_peer.go -- descriptor leases and value-only exit proofs
package vpp

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"golang.org/x/sys/unix"
)

type linuxProcessDescriptor struct {
	fd       int
	identity processIdentity
}

func (d *linuxProcessDescriptor) Identity() processIdentity { return d.identity }
func (d *linuxProcessDescriptor) Close() error              { return unix.Close(d.fd) }
func (d *linuxProcessDescriptor) Exited() (bool, error) {
	descriptors := [1]unix.PollFd{{Fd: int32(d.fd), Events: unix.POLLIN}}
	_, err := unix.Poll(descriptors[:], 0)
	if err != nil {
		return false, fmt.Errorf("vpp: observe Unix peer exit: %w", err)
	}
	events := descriptors[0].Revents
	if events&unix.POLLNVAL != 0 {
		return false, errors.New("vpp: Unix peer process descriptor is invalid")
	}
	if events&unix.POLLERR != 0 {
		return false, errors.New("vpp: Unix peer exit observation failed")
	}
	return events&unix.POLLIN != 0, nil
}

// captureProcessWitness borrows the connected Unix socket only during Control;
// SO_PEERPIDFD creates a separate owned descriptor for its retained peer pid.
// Linux7.3 net/core/sock.c::sk_getsockopt gets sk_peer_pid under its peer lock,
// so there is no credential-PID-to-pidfd_open reuse race, even across namespaces.
// fs/pidfs.c gives each process a stable 64-bit inode and pidfd_poll waits for
// the whole thread group. A 32-bit inode needs an additional generation; Ze's
// supported 64-bit targets compare the complete inode and refuse other ABIs.
func captureProcessWitness(connection *net.UnixConn) (*processWitness, error) {
	if strconv.IntSize != 64 {
		return nil, errors.New("vpp: physical peer identity requires a 64-bit Linux pidfs ABI; use Ze's runtime kernel on a supported architecture")
	}
	raw, err := connection.SyscallConn()
	if err != nil {
		return nil, err
	}
	fd := -1
	var captureErr error
	if err := raw.Control(func(socket uintptr) {
		fd, captureErr = unix.GetsockoptInt(int(socket), unix.SOL_SOCKET, unix.SO_PEERPIDFD)
	}); err != nil {
		if fd >= 0 {
			return nil, errors.Join(err, unix.Close(fd))
		}
		return nil, err
	}
	if captureErr != nil {
		return nil, fmt.Errorf("vpp: cannot retain the Unix peer process; use Ze's runtime kernel with SO_PEERPIDFD and pidfs: %w", captureErr)
	}
	return processWitnessForPIDFD(fd)
}

// The same native descriptor validation serves connection capture and doctor's
// owned socketpair capability probe; neither infers support from a version.
func processWitnessForPIDFD(fd int) (*processWitness, error) {
	var stat unix.Stat_t
	var filesystem unix.Statfs_t
	failure := unix.Fstat(fd, &stat)
	if failure == nil {
		failure = unix.Fstatfs(fd, &filesystem)
	}
	if failure == nil {
		if filesystem.Type != unix.PID_FS_MAGIC {
			failure = errors.New("vpp: Unix peer descriptor has no unique pidfs identity; use Ze's runtime kernel")
		}
	}
	if failure != nil {
		return nil, errors.Join(failure, unix.Close(fd))
	}
	descriptor := &linuxProcessDescriptor{fd: fd, identity: processIdentity{device: stat.Dev, inode: stat.Ino}}
	witness, err := newProcessWitness(descriptor)
	if err != nil {
		return nil, errors.Join(err, descriptor.Close())
	}
	if err := witness.alive(); err != nil {
		return nil, errors.Join(err, witness.release())
	}
	return witness, nil
}

func probePhysicalProcessCapability() error {
	if strconv.IntSize != 64 {
		return errors.New("VPP process ownership requires a supported 64-bit Ze runtime kernel")
	}
	sockets, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return err
	}
	fd, captureErr := unix.GetsockoptInt(sockets[0], unix.SOL_SOCKET, unix.SO_PEERPIDFD)
	cleanup := errors.Join(unix.Close(sockets[0]), unix.Close(sockets[1]))
	if captureErr != nil {
		return errors.Join(captureErr, cleanup)
	}
	witness, err := processWitnessForPIDFD(fd)
	if err != nil {
		return errors.Join(err, cleanup)
	}
	return errors.Join(cleanup, witness.release())
}
