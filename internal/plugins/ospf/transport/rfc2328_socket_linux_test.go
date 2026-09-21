//go:build linux

// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- the socket state Ze installs for
// the Linux IP stack: link-local TTL (RFC 2328 Appendix A.1) and OSPF group membership
// (RFC 2328 section 4.4). The options are installed on an unprivileged UDP socket here,
// because the raw protocol-89 socket needs CAP_NET_RAW; the IP-level options are the same.

package transport

import (
	"errors"
	"net/netip"
	"testing"

	"golang.org/x/sys/unix"
)

var loopback = [4]byte{127, 0, 0, 1}

func newIPv4Socket(t *testing.T) int {
	t.Helper()
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		t.Fatalf("socket: %v", err)
	}
	t.Cleanup(func() { closeFD(fd) })
	return fd
}

// RFC requirement: RFC2328-A.1-1 positive — the interface socket's unicast and multicast TTL
// are both 1 after Ze installs its options, where the kernel default was not 1
// (setMulticastOptions IP_TTL and IP_MULTICAST_TTL).
func TestOSPFSocketTTLIsOne(t *testing.T) {
	fd := newIPv4Socket(t)
	before, err := unix.GetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_TTL)
	if err != nil {
		t.Fatalf("getsockopt IP_TTL: %v", err)
	}
	if before == 1 {
		t.Fatalf("kernel default IP_TTL is already 1; the test cannot tell Ze's value from it")
	}
	if err := setMulticastOptions(fd, 1, loopback); err != nil {
		t.Fatalf("setMulticastOptions: %v", err)
	}
	ttl, err := unix.GetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_TTL)
	if err != nil {
		t.Fatalf("getsockopt IP_TTL: %v", err)
	}
	mttl, err := unix.GetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_MULTICAST_TTL)
	if err != nil {
		t.Fatalf("getsockopt IP_MULTICAST_TTL: %v", err)
	}
	if ttl != 1 || mttl != 1 {
		t.Fatalf("IP_TTL = %d, IP_MULTICAST_TTL = %d, want 1 and 1", ttl, mttl)
	}
}

// RFC requirement: RFC2328-A.1-1 negative — when the TTL cannot be installed the failure is
// returned, so no interface socket is put in service with the kernel's multi-hop default TTL
// (setMulticastOptions returns the setsockopt error on a closed descriptor).
func TestOSPFSocketTTLFailureIsAnError(t *testing.T) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		t.Fatalf("socket: %v", err)
	}
	closeFD(fd)
	err = setMulticastOptions(fd, 1, loopback)
	if err == nil {
		t.Fatal("setMulticastOptions on a closed descriptor returned nil, want the setsockopt error")
	}
	if !errors.Is(err, unix.EBADF) {
		t.Fatalf("error = %v, want EBADF", err)
	}
}

// RFC requirement: RFC2328-4.4-1 positive — Ze installs the receive side of OSPF multicast in the
// IP stack: joining AllSPFRouters on the interface's address adds the group membership and
// leaving it drops the membership again, both accepted by the kernel (joinGroup, leaveGroup).
func TestOSPFMulticastMembershipInstalled(t *testing.T) {
	fd := newIPv4Socket(t)
	if err := joinGroup(fd, 1, loopback, AllSPFRouters); err != nil {
		t.Fatalf("joinGroup AllSPFRouters: %v", err)
	}
	// A second join of the same group on the same socket is refused by the kernel, which is
	// the observable proof that the first membership was installed.
	if err := joinGroup(fd, 1, loopback, AllSPFRouters); !errors.Is(err, unix.EADDRINUSE) {
		t.Fatalf("second joinGroup = %v, want EADDRINUSE (membership installed)", err)
	}
	if err := leaveGroup(fd, 1, loopback, AllSPFRouters); err != nil {
		t.Fatalf("leaveGroup AllSPFRouters: %v", err)
	}
	if err := leaveGroup(fd, 1, loopback, AllSPFRouters); !errors.Is(err, unix.EADDRNOTAVAIL) {
		t.Fatalf("second leaveGroup = %v, want EADDRNOTAVAIL (membership dropped)", err)
	}
}

// RFC requirement: RFC2328-4.4-1 negative — a group that is not AllSPFRouters or AllDRouters is
// refused with ErrInvalidDestination before any membership is installed, so the OSPF socket
// never receives another protocol's multicast (joinGroup, leaveGroup).
func TestOSPFMulticastMembershipRefusesForeignGroup(t *testing.T) {
	fd := newIPv4Socket(t)
	rip := netip.MustParseAddr("224.0.0.9")
	if err := joinGroup(fd, 1, loopback, rip); !errors.Is(err, ErrInvalidDestination) {
		t.Fatalf("joinGroup %s = %v, want ErrInvalidDestination", rip, err)
	}
	if err := leaveGroup(fd, 1, loopback, rip); !errors.Is(err, ErrInvalidDestination) {
		t.Fatalf("leaveGroup %s = %v, want ErrInvalidDestination", rip, err)
	}
	// Nothing was installed: dropping the membership is refused by the kernel.
	mreq := unix.IPMreq{Multiaddr: rip.As4(), Interface: loopback}
	if err := unix.SetsockoptIPMreq(fd, unix.IPPROTO_IP, unix.IP_DROP_MEMBERSHIP, &mreq); !errors.Is(err, unix.EADDRNOTAVAIL) {
		t.Fatalf("IP_DROP_MEMBERSHIP %s = %v, want EADDRNOTAVAIL (never joined)", rip, err)
	}
}
