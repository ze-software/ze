//go:build linux

// Design: docs/architecture/testing/interop.md -- namespace-safe AF_PACKET injection.
package bgp

import (
	"fmt"
	"net"
	"runtime"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func injectISISPurgeHost(pid int, interfaceName string, pdu []byte) (resultErr error) {
	if pid <= 0 {
		return fmt.Errorf("ze peer PID must be positive, got %d", pid)
	}
	if len(pdu) != isisPurgePDULength {
		return fmt.Errorf("IS-IS purge PDU is %d octets, expected %d", len(pdu), isisPurgePDULength)
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
		resultErr = joinISISInjectionError(resultErr, "close original network namespace", unix.Close(originalFD))
	}()

	peerPath := textbuf.StrIntStr("/proc/", int64(pid), "/ns/net")
	peerFD, err := unix.Open(peerPath, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open Ze network namespace %s: %w", peerPath, err)
	}
	defer func() {
		resultErr = joinISISInjectionError(resultErr, "close Ze network namespace", unix.Close(peerFD))
	}()

	if err := unix.Setns(peerFD, unix.CLONE_NEWNET); err != nil {
		return fmt.Errorf("enter Ze network namespace: %w", err)
	}
	peerNamespaceEntered = true
	defer func() {
		restoreErr := unix.Setns(originalFD, unix.CLONE_NEWNET)
		namespaceRestored = restoreErr == nil
		resultErr = joinISISInjectionError(resultErr, "restore original network namespace", restoreErr)
	}()

	link, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return fmt.Errorf("resolve %s in Ze network namespace: %w", interfaceName, err)
	}
	if len(link.HardwareAddr) != 6 {
		return fmt.Errorf("%s has %d-octet hardware address, expected 6", interfaceName, len(link.HardwareAddr))
	}

	protocol := int(hostToNetwork16(unix.ETH_P_ALL))
	socketFD, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, protocol)
	if err != nil {
		return fmt.Errorf("open AF_PACKET socket: %w", err)
	}
	defer func() {
		resultErr = joinISISInjectionError(resultErr, "close AF_PACKET socket", unix.Close(socketFD))
	}()

	address := &unix.SockaddrLinklayer{
		Protocol: hostToNetwork16(unix.ETH_P_ALL),
		Ifindex:  link.Index,
	}
	if err := unix.Bind(socketFD, address); err != nil {
		return fmt.Errorf("bind AF_PACKET socket to %s: %w", interfaceName, err)
	}
	frame, err := buildISISEthernetFrame(link.HardwareAddr, pdu)
	if err != nil {
		return err
	}
	if err := unix.Sendto(socketFD, frame, 0, address); err != nil {
		return fmt.Errorf("send IS-IS purge on %s: %w", interfaceName, err)
	}
	return nil
}

func hostToNetwork16(value uint16) uint16 {
	return value<<8 | value>>8
}
