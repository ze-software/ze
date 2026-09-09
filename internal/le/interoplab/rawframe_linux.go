//go:build linux

// Design: docs/architecture/testing/interop.md -- namespace-safe AF_PACKET injection.
// Related: bgp/isis_inject.go -- the IS-IS purge injector this generalizes.
// Related: pppoe/check_padr_replay.go -- replaying a captured PADR frame.
package interoplab

import (
	"errors"
	"fmt"
	"net"
	"runtime"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// SendFrameInNamespace enters the network namespace of the process named by
// pid, resolves interfaceName inside it, calls build with the resolved link to
// get the frame to send, and transmits that frame on an AF_PACKET socket bound
// to the same interface.
//
// A Docker container owns its own network namespace, so this is how a checker
// running on the host sends a frame as if the peer container itself had sent
// it: no socket outside that namespace, bound to that interface, can do it.
// build runs after the namespace switch because a caller that needs the
// peer's own hardware address (an injected frame carrying it as source, for
// example) can only read it from inside the peer's namespace.
//
// The calling goroutine's OS thread is locked for the whole namespace switch.
// A thread that fails to restore its original namespace exits with the
// goroutine rather than returning to the scheduler, because a thread stuck in
// another process's namespace must never run unrelated work.
func SendFrameInNamespace(
	pid int,
	interfaceName string,
	build func(link *net.Interface) ([]byte, error),
) (resultErr error) {
	if pid <= 0 {
		return fmt.Errorf("peer PID must be positive, got %d", pid)
	}
	if build == nil {
		return errors.New("frame builder is nil")
	}

	runtime.LockOSThread()
	peerNamespaceEntered := false
	namespaceRestored := false
	defer func() {
		// A thread that could not restore its original namespace MUST NOT return to
		// the Go scheduler. Exiting this locked goroutine makes the runtime retire it.
		if !peerNamespaceEntered || namespaceRestored {
			runtime.UnlockOSThread()
		}
	}()

	originalFD, err := unix.Open("/proc/self/ns/net", unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open original network namespace: %w", err)
	}
	defer func() {
		resultErr = joinRawFrameError(resultErr, "close original network namespace", unix.Close(originalFD))
	}()

	peerPath := textbuf.StrIntStr("/proc/", int64(pid), "/ns/net")
	peerFD, err := unix.Open(peerPath, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open peer network namespace %s: %w", peerPath, err)
	}
	defer func() {
		resultErr = joinRawFrameError(resultErr, "close peer network namespace", unix.Close(peerFD))
	}()

	if err := unix.Setns(peerFD, unix.CLONE_NEWNET); err != nil {
		return fmt.Errorf("enter peer network namespace: %w", err)
	}
	peerNamespaceEntered = true
	defer func() {
		restoreErr := unix.Setns(originalFD, unix.CLONE_NEWNET)
		namespaceRestored = restoreErr == nil
		resultErr = joinRawFrameError(resultErr, "restore original network namespace", restoreErr)
	}()

	link, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return fmt.Errorf("resolve %s in peer network namespace: %w", interfaceName, err)
	}

	frame, err := build(link)
	if err != nil {
		return err
	}
	if len(frame) == 0 {
		return errors.New("frame to send is empty")
	}

	protocol := int(hostToNetwork16(unix.ETH_P_ALL))
	socketFD, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, protocol)
	if err != nil {
		return fmt.Errorf("open AF_PACKET socket: %w", err)
	}
	defer func() {
		resultErr = joinRawFrameError(resultErr, "close AF_PACKET socket", unix.Close(socketFD))
	}()

	address := &unix.SockaddrLinklayer{
		Protocol: hostToNetwork16(unix.ETH_P_ALL),
		Ifindex:  link.Index,
	}
	if err := unix.Bind(socketFD, address); err != nil {
		return fmt.Errorf("bind AF_PACKET socket to %s: %w", interfaceName, err)
	}
	if err := unix.Sendto(socketFD, frame, 0, address); err != nil {
		return fmt.Errorf("send frame on %s: %w", interfaceName, err)
	}
	return nil
}

// joinRawFrameError adds one cleanup failure to the error already being
// returned, so a namespace or descriptor that would not close is reported
// beside the failure that led to it rather than replacing it.
func joinRawFrameError(resultErr error, operation string, err error) error {
	if err == nil {
		return resultErr
	}
	return errors.Join(resultErr, fmt.Errorf("%s: %w", operation, err))
}

func hostToNetwork16(value uint16) uint16 {
	return value<<8 | value>>8
}
