// Design: docs/architecture/appliance/kernel-profiles.md -- original-unit control socket boundaries
// Related: created_unit_control_socket_fixture_integration_linux_test.go -- real creator and file probes

//go:build integration && linux

package kernel_test

import (
	"errors"
	"os"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestCreatedUnitControlSocketNativeOptions proves that actual original-bound
// files refuse option escapes without changing their state. Same-kind ordinary
// files retain their native setters, while both owned roles keep useful
// SO_REUSEADDR and original-device binding operations.
func TestCreatedUnitControlSocketNativeOptions(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _ := controlSocketNativeUnit(t, "zcopt")
	other, _ := controlSocketNativeUnit(t, "zcoptother")
	originalIndex := int(unit.Identity().NativeIndex)
	otherIndex := int(other.Identity().NativeIndex)
	if otherIndex == originalIndex {
		t.Fatal("binding control did not create distinct actual PPP devices")
	}
	for _, entry := range []struct {
		name string
		kind pppkernel.IPv6ControlSocketKind
	}{
		{"icmp6", pppkernel.IPv6ControlSocketICMP6Raw},
		{"udp6", pppkernel.IPv6ControlSocketUDP6},
	} {
		t.Run(entry.name, func(t *testing.T) {
			owned := controlSocketNativeOpen(t, unit, entry.kind)
			plain := controlSocketOptionsPlain(t, entry.kind)
			controlSocketNativeRequireSocket(t, owned, entry.kind, originalIndex)
			controlSocketNativeRequireSocket(t, plain, entry.kind, 0)

			// Successful owned setters rule out an implementation that refuses
			// every option simply because the file carries PPP ownership.
			controlSocketOptionsAllow(t, owned, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
			controlSocketOptionsAllow(t, plain, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
			controlSocketOptionsAllow(t, owned, unix.SOL_SOCKET, unix.SO_BINDTOIFINDEX, originalIndex)

			t.Run("v6-only", func(t *testing.T) {
				controlSocketOptionsRequire(t, owned, unix.SOL_IPV6, unix.IPV6_V6ONLY, 1)
				if entry.kind == pppkernel.IPv6ControlSocketUDP6 {
					controlSocketOptionsAllow(t, owned, unix.SOL_IPV6, unix.IPV6_V6ONLY, 1)
					controlSocketOptionsAllow(t, plain, unix.SOL_IPV6, unix.IPV6_V6ONLY, 1)
					controlSocketOptionsAllow(t, plain, unix.SOL_IPV6, unix.IPV6_V6ONLY, 0)
				} else {
					// inet6_create stores the raw protocol in inet_num. The native
					// V6ONLY setter rejects any nonzero inet_num, including a
					// reassertion; an ordinary ICMP6 setter cannot be a success
					// control. Preserve that exact native EINVAL and state.
					before, err := controlSocketOptionsGet(plain, unix.SOL_IPV6, unix.IPV6_V6ONLY)
					if err != nil {
						t.Fatal(err)
					}
					controlSocketOptionsRefuse(t, plain, unix.SOL_IPV6, unix.IPV6_V6ONLY, 0, unix.EINVAL)
					controlSocketOptionsRequire(t, plain, unix.SOL_IPV6, unix.IPV6_V6ONLY, before)
				}
				controlSocketOptionsRefuse(t, owned, unix.SOL_IPV6, unix.IPV6_V6ONLY, 0, unix.EOPNOTSUPP)
				controlSocketOptionsRequire(t, owned, unix.SOL_IPV6, unix.IPV6_V6ONLY, 1)
				controlSocketNativeRequireSocket(t, owned, entry.kind, originalIndex)
			})

			t.Run("reuseport", func(t *testing.T) {
				controlSocketOptionsRequire(t, owned, unix.SOL_SOCKET, unix.SO_REUSEPORT, 0)
				controlSocketOptionsAllow(t, plain, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
				controlSocketOptionsRefuse(t, owned, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1, unix.EOPNOTSUPP)
				controlSocketOptionsRequire(t, owned, unix.SOL_SOCKET, unix.SO_REUSEPORT, 0)
				controlSocketNativeRequireSocket(t, owned, entry.kind, originalIndex)
			})

			t.Run("binding", func(t *testing.T) {
				controlSocketOptionsAllow(t, plain, unix.SOL_SOCKET, unix.SO_BINDTOIFINDEX, originalIndex)
				for _, target := range []int{0, otherIndex} {
					controlSocketOptionsAllow(t, plain, unix.SOL_SOCKET, unix.SO_BINDTOIFINDEX, target)
					controlSocketOptionsRefuse(t, owned, unix.SOL_SOCKET, unix.SO_BINDTOIFINDEX, target, unix.ESTALE)
					controlSocketOptionsRequire(t, owned, unix.SOL_SOCKET, unix.SO_BINDTOIFINDEX, originalIndex)
					controlSocketNativeRequireSocket(t, owned, entry.kind, originalIndex)
				}
				controlSocketOptionsAllow(t, owned, unix.SOL_SOCKET, unix.SO_BINDTOIFINDEX, originalIndex)
			})

			if entry.kind == pppkernel.IPv6ControlSocketICMP6Raw {
				t.Run("header-inclusion", func(t *testing.T) {
					controlSocketOptionsRequire(t, owned, unix.SOL_IPV6, unix.IPV6_HDRINCL, 0)
					controlSocketOptionsAllow(t, plain, unix.SOL_IPV6, unix.IPV6_HDRINCL, 1)
					controlSocketOptionsRefuse(t, owned, unix.SOL_IPV6, unix.IPV6_HDRINCL, 1, unix.EOPNOTSUPP)
					controlSocketOptionsRequire(t, owned, unix.SOL_IPV6, unix.IPV6_HDRINCL, 0)
					controlSocketNativeRequireSocket(t, owned, entry.kind, originalIndex)
				})
			}
			if entry.kind == pppkernel.IPv6ControlSocketUDP6 {
				t.Run("encapsulation", func(t *testing.T) {
					// The registered runtime profile builds both ESP families and
					// XFRM in. Its ordinary UDP6 file must accept this actual
					// callback-bearing mode; an unsupported profile must fail.
					controlSocketOptionsRequire(t, owned, unix.SOL_UDP, unix.UDP_ENCAP, 0)
					controlSocketOptionsAllow(t, plain, unix.SOL_UDP, unix.UDP_ENCAP, unix.UDP_ENCAP_ESPINUDP)
					controlSocketOptionsRefuse(t, owned, unix.SOL_UDP, unix.UDP_ENCAP, unix.UDP_ENCAP_ESPINUDP, unix.EOPNOTSUPP)
					controlSocketOptionsRequire(t, owned, unix.SOL_UDP, unix.UDP_ENCAP, 0)
					controlSocketNativeRequireSocket(t, owned, entry.kind, originalIndex)
				})
			}
			controlSocketOptionsRequire(t, owned, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
			controlSocketNativeRequireSocket(t, owned, entry.kind, originalIndex)
		})
	}
}

// TestCreatedUnitControlSocketNativeAddressForm proves that the owned UDP6
// file cannot convert to IPv4. A genuinely connected v4-mapped ordinary UDP6
// file successfully converts, so an invalid conversion setup cannot pass.
func TestCreatedUnitControlSocketNativeAddressForm(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _ := controlSocketNativeUnit(t, "zcform")
	owned := controlSocketNativeOpen(t, unit, pppkernel.IPv6ControlSocketUDP6)
	plain := controlSocketOptionsPlain(t, pppkernel.IPv6ControlSocketUDP6)
	loopback, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetUp(loopback); err != nil {
		t.Fatal(err)
	}
	controlSocketOptionsAllow(t, plain, unix.SOL_IPV6, unix.IPV6_V6ONLY, 0)
	raw, err := plain.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var connectErr error
	controlErr := raw.Control(func(fd uintptr) {
		connectErr = unix.Connect(int(fd), &unix.SockaddrInet6{
			Port: 9,
			Addr: [16]byte{10: 0xff, 11: 0xff, 12: 127, 15: 1},
		})
	})
	if err := errors.Join(connectErr, controlErr); err != nil {
		t.Fatalf("connect actual ordinary v4-mapped UDP6 socket: %v", err)
	}
	controlSocketOptionsRequire(t, plain, unix.SOL_SOCKET, unix.SO_DOMAIN, unix.AF_INET6)
	if err := controlSocketOptionsSet(plain, unix.SOL_IPV6, unix.IPV6_ADDRFORM, unix.AF_INET); err != nil {
		t.Fatalf("actual ordinary IPv4 conversion: %v", err)
	}
	controlSocketOptionsRequire(t, plain, unix.SOL_SOCKET, unix.SO_DOMAIN, unix.AF_INET)
	controlSocketOptionsAllow(t, owned, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
	controlSocketOptionsRefuse(t, owned, unix.SOL_IPV6, unix.IPV6_ADDRFORM, unix.AF_INET, unix.EOPNOTSUPP)
	controlSocketOptionsRequire(t, owned, unix.SOL_IPV6, unix.IPV6_V6ONLY, 1)
	controlSocketNativeRequireSocket(t, owned, pppkernel.IPv6ControlSocketUDP6, int(unit.Identity().NativeIndex))
}

// controlSocketOptionsPlain mints only an ordinary userspace socket. It owns
// the new file's Close through cleanup; callers MUST not close it separately.
func controlSocketOptionsPlain(t *testing.T, kind pppkernel.IPv6ControlSocketKind) *os.File {
	t.Helper()
	var socketType, protocol int
	// The public numeric kind can be converted from an unknown value. Refuse
	// that value before minting an ordinary file, just like the owned API.
	switch kind {
	case pppkernel.IPv6ControlSocketUnspecified:
		t.Fatal("ordinary socket control has no kind")
		return nil
	case pppkernel.IPv6ControlSocketICMP6Raw:
		socketType = unix.SOCK_RAW
		protocol = unix.IPPROTO_ICMPV6
	case pppkernel.IPv6ControlSocketUDP6:
		socketType = unix.SOCK_DGRAM
		protocol = unix.IPPROTO_UDP
	default:
		t.Fatalf("ordinary socket control has unknown numeric kind %d", kind)
		return nil
	}
	fd, err := unix.Socket(unix.AF_INET6, socketType|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, protocol)
	if err != nil {
		t.Fatalf("create actual ordinary control socket: %v", err)
	}
	file := os.NewFile(uintptr(fd), "test-ordinary-ipv6-option-control")
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("close actual ordinary control socket: %v", err)
		}
	})
	return file
}

func controlSocketOptionsSet(file *os.File, level, option, value int) error {
	raw, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var optionErr error
	controlErr := raw.Control(func(fd uintptr) {
		optionErr = unix.SetsockoptInt(int(fd), level, option, value)
	})
	return errors.Join(optionErr, controlErr)
}

func controlSocketOptionsGet(file *os.File, level, option int) (int, error) {
	raw, err := file.SyscallConn()
	if err != nil {
		return 0, err
	}
	var value int
	var optionErr error
	controlErr := raw.Control(func(fd uintptr) {
		value, optionErr = unix.GetsockoptInt(int(fd), level, option)
	})
	return value, errors.Join(optionErr, controlErr)
}

func controlSocketOptionsAllow(t *testing.T, file *os.File, level, option, value int) {
	t.Helper()
	if err := controlSocketOptionsSet(file, level, option, value); err != nil {
		t.Fatalf("native permitted option level=%d option=%d value=%d: %v", level, option, value, err)
	}
	controlSocketOptionsRequire(t, file, level, option, value)
}

func controlSocketOptionsRefuse(t *testing.T, file *os.File, level, option, value int, wanted error) {
	t.Helper()
	err := controlSocketOptionsSet(file, level, option, value)
	if !errors.Is(err, wanted) {
		t.Fatalf("native refused option level=%d option=%d value=%d: got %v, want %v", level, option, value, err, wanted)
	}
}

func controlSocketOptionsRequire(t *testing.T, file *os.File, level, option, wanted int) {
	t.Helper()
	value, err := controlSocketOptionsGet(file, level, option)
	if err != nil {
		t.Fatalf("read native option level=%d option=%d: %v", level, option, err)
	}
	if value != wanted {
		t.Fatalf("native option level=%d option=%d: got %d, want %d", level, option, value, wanted)
	}
}
