// Design: docs/features/interfaces.md -- original creator file guarded PPP provider
// Related: docs/architecture/appliance/kernel-profiles.md -- PPP original-resource ABI

//go:build integration && linux

package ifacenetlink

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/netip"
	"os"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/test/userns"
)

var nativePPPProviderRequired = flag.Bool("ppp-provider-required", false,
	"require the actual original PPP provider prerequisite in its dedicated Ze-kernel proof")

// TestNativePPPProviderUsefulRenamedOriginal proves the production optional
// provider changes its retained original device after rename while preserving
// a real unrelated replacement at the old name. Both unit authority and query51
// are native; this fixture supplies neither a feature receipt nor a fake fd.
func TestNativePPPProviderUsefulRenamedOriginal(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativePPPProviderFixtureNew(t)
	backend := &netlinkBackend{}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Errorf("close original provider backend: %v", err)
		}
	})
	provider, ok := any(backend).(iface.NativePPPBackend)
	if !ok {
		t.Fatal("netlink backend does not provide original PPP writers")
	}
	writer, err := provider.BindNativePPPUnit(fixture, fixture.link.Attrs().Index, fixture.unit)
	if err != nil {
		if writer != nil {
			if closeErr := writer.Close(); closeErr != nil {
				t.Errorf("retain failed writer cleanup: %v", closeErr)
			}
		}
		t.Fatalf("bind actual original PPP provider after positive native query51: %v", err)
	}
	if writer == nil {
		t.Fatal("successful native binding returned no writer")
	}
	t.Cleanup(func() {
		if err := writer.Close(); err != nil {
			t.Errorf("close actual original PPP writer: %v", err)
		}
	})
	oldName := fixture.link.Attrs().Name
	if err := netlink.LinkSetName(fixture.link, "zppporiginal"); err != nil {
		t.Fatal(err)
	}
	survivor := &netlink.Dummy{Name: oldName, MTU: 1701}
	if err := netlink.LinkAdd(survivor); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := netlink.LinkDel(survivor); err != nil {
			t.Errorf("delete task-owned survivor: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := writer.SetMTU(ctx, 1431); err != nil {
		t.Fatalf("guarded useful original MTU: %v", err)
	}
	nativePPPProviderRequireLink(t, fixture.link.Attrs().Index, 1431, "", false)
	if err := writer.SetAlias(ctx, "ze:test:original-ppp"); err != nil {
		t.Fatalf("guarded useful original alias: %v", err)
	}
	if err := writer.SetAdminUp(ctx); err != nil {
		t.Fatalf("guarded useful original admin up: %v", err)
	}
	nativePPPProviderRequireLink(t, fixture.link.Attrs().Index, 1431, "ze:test:original-ppp", true)
	if err := writer.SetAdminDown(ctx); err != nil {
		t.Fatalf("guarded useful original admin down: %v", err)
	}
	nativePPPProviderRequireLink(t, fixture.link.Attrs().Index, 1431, "ze:test:original-ppp", false)
	nativePPPProviderRequireLink(t, survivor.Attrs().Index, 1701, "", false)
	if fixture.inBorrow.Load() {
		t.Fatal("writer retained the original unit descriptor borrow after completion")
	}
}

// TestNativePPPProviderForeignCreatorRefused keeps both real creator files
// alive. An equal operation tuple never upgrades the foreign file into the
// actual original device's authority; the useful original control still works.
func TestNativePPPProviderForeignCreatorRefused(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	original := nativePPPProviderFixtureNew(t)
	foreign := nativePPPProviderFixtureNew(t)
	backend := &netlinkBackend{}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Errorf("close original provider backend: %v", err)
		}
	})
	writer, err := backend.BindNativePPPUnit(foreign, original.link.Attrs().Index, original.unit)
	if writer != nil {
		if closeErr := writer.Close(); closeErr != nil {
			t.Errorf("close refused foreign binding: %v", closeErr)
		}
	}
	if err == nil {
		t.Fatal("foreign actual original file acquired authority over the victim unit")
	}
	if !errors.Is(err, unix.ESTALE) {
		t.Fatalf("foreign tuple refusal lost actual native ESTALE: %v", err)
	}
	useful, err := backend.BindNativePPPUnit(original, original.link.Attrs().Index, original.unit)
	if err != nil {
		t.Fatalf("useful original binding after foreign refusal: %v", err)
	}
	t.Cleanup(func() {
		if err := useful.Close(); err != nil {
			t.Errorf("close useful original writer: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := useful.SetMTU(ctx, 1432); err != nil {
		t.Fatal(err)
	}
	nativePPPProviderRequireLink(t, original.link.Attrs().Index, 1432, "", false)
	nativePPPProviderRequireLink(t, foreign.link.Attrs().Index, 1500, "", false)
}

// TestNativePPPProviderAddressRouteFamilies proves both actual address families,
// all retained masks, exact peer refusal, IPv4 sibling preservation and native
// RouteReplace semantics through this production provider. The kernel must
// positively advertise EXACT_ADDR_PAIR before a delete can be attempted.
func TestNativePPPProviderAddressRouteFamilies(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	for _, entry := range []struct {
		name        string
		local       string
		peer        string
		otherLocal  string
		otherPeer   string
		wrongPeer   string
		destination string
	}{
		{"IPv4 non-host", "198.51.100.1/24", "198.51.100.2/24", "198.51.100.3/24", "198.51.100.4/24", "198.51.100.99/24", "192.0.2.0/24"},
		{"IPv6 link-local", "fe80::1/64", "fe80::2/64", "fe80::3/64", "fe80::4/64", "fe80::99/64", "2001:db8:1234::/48"},
	} {
		// One closure per family, so each family's context is canceled when
		// that family ends rather than when the whole test returns.
		func() {
			t.Logf("actual native family: %s", entry.name)
			fixture := nativePPPProviderFixtureNew(t)
			backend := &netlinkBackend{}
			t.Cleanup(func() {
				if err := backend.Close(); err != nil {
					t.Errorf("close family backend: %v", err)
				}
			})
			writer, err := backend.BindNativePPPUnit(fixture, fixture.link.Attrs().Index, fixture.unit)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := writer.Close(); err != nil {
					t.Errorf("close family writer: %v", err)
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if err := writer.SetAdminUp(ctx); err != nil {
				t.Fatal(err)
			}
			local := netip.MustParsePrefix(entry.local)
			peer := netip.MustParsePrefix(entry.peer)
			otherLocal := netip.MustParsePrefix(entry.otherLocal)
			otherPeer := netip.MustParsePrefix(entry.otherPeer)
			if err := writer.AddAddressP2P(ctx, local, peer); err != nil {
				t.Fatal(err)
			}
			if err := writer.AddAddressP2P(ctx, otherLocal, otherPeer); err != nil {
				t.Fatal(err)
			}
			nativePPPProviderRequirePair(t, fixture.link, local, peer, true)
			nativePPPProviderRequirePair(t, fixture.link, otherLocal, otherPeer, true)
			destination := netip.MustParsePrefix(entry.destination)
			width := 128
			if destination.Addr().Is4() {
				width = 32
			}
			oldRoute := &netlink.Route{LinkIndex: fixture.link.Attrs().Index,
				Dst: &net.IPNet{IP: net.IP(destination.Addr().AsSlice()), Mask: net.CIDRMask(destination.Bits(), width)},
				Gw:  net.IP(peer.Addr().AsSlice()), Priority: 19, Protocol: netlink.RouteProtocol(rtproto.Static), MTU: 1301}
			if err := netlink.RouteReplace(oldRoute); err != nil {
				t.Fatal(err)
			}
			if err := writer.AddRoute(ctx, destination, peer.Addr(), 19, rtproto.Iface); err != nil {
				t.Fatalf("actual guarded RouteReplace must replace an existing key: %v", err)
			}
			nativePPPProviderRequireRoute(t, fixture.link, destination, peer.Addr(), true)
			if err := writer.RemoveRoute(ctx, destination, peer.Addr(), 19, rtproto.Iface); err != nil {
				t.Fatal(err)
			}
			nativePPPProviderRequireRoute(t, fixture.link, destination, peer.Addr(), false)
			if err := writer.RemoveAddressP2P(ctx, local, netip.MustParsePrefix(entry.wrongPeer)); err == nil {
				t.Fatal("same-subnet foreign peer matched an exact original pair delete")
			}
			nativePPPProviderRequirePair(t, fixture.link, local, peer, true)
			nativePPPProviderRequirePair(t, fixture.link, otherLocal, otherPeer, true)
			if err := writer.RemoveAddressP2P(ctx, local, peer); err != nil {
				t.Fatal(err)
			}
			nativePPPProviderRequirePair(t, fixture.link, local, peer, false)
			nativePPPProviderRequirePair(t, fixture.link, otherLocal, otherPeer, true)
			if err := writer.RemoveAddressP2P(ctx, otherLocal, otherPeer); err != nil {
				t.Fatal(err)
			}
			nativePPPProviderRequirePair(t, fixture.link, otherLocal, otherPeer, false)
		}()
	}
}

func nativePPPProviderRequirePair(t *testing.T, link netlink.Link, local, peer netip.Prefix, wanted bool) {
	t.Helper()
	addresses, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		t.Fatal(err)
	}
	present := false
	for _, address := range addresses {
		actualLocal, ok := netip.AddrFromSlice(address.IP)
		if !ok {
			t.Fatal("native address list returned an invalid local address")
		}
		if actualLocal.Unmap() != local.Addr() {
			continue
		}
		if address.Peer == nil {
			continue
		}
		actualPeer, ok := netip.AddrFromSlice(address.Peer.IP)
		if !ok {
			t.Fatal("native address list returned an invalid peer address")
		}
		bits, width := address.Peer.Mask.Size()
		if width == 0 {
			t.Fatal("native address list returned an invalid peer mask")
		}
		if actualPeer.Unmap() != peer.Addr() {
			continue
		}
		if bits != peer.Bits() {
			continue
		}
		present = true
	}
	if present != wanted {
		t.Fatalf("native pair local=%s peer=%s present=%v, want %v", local, peer, present, wanted)
	}
}

func nativePPPProviderRequireRoute(t *testing.T, link netlink.Link, destination netip.Prefix, gateway netip.Addr, wanted bool) {
	t.Helper()
	family := netlink.FAMILY_V6
	if destination.Addr().Is4() {
		family = netlink.FAMILY_V4
	}
	routes, err := netlink.RouteList(link, family)
	if err != nil {
		t.Fatal(err)
	}
	present := false
	for i := range routes {
		route := &routes[i]
		if route.Dst == nil {
			continue
		}
		if route.Dst.String() != destination.Masked().String() {
			continue
		}
		if route.Priority != 19 {
			continue
		}
		if route.Protocol != netlink.RouteProtocol(rtproto.Iface) {
			continue
		}
		if !route.Gw.Equal(net.IP(gateway.AsSlice())) {
			continue
		}
		if route.MTU != 0 {
			t.Fatal("guarded RouteReplace retained the old route's MTU")
		}
		present = true
	}
	if present != wanted {
		t.Fatalf("native route %s via %s present=%v, want %v", destination, gateway, present, wanted)
	}
}

// This actual original file fixture is a lease adapter, not an ownership
// simulator. Closing it closes the precise creator file produced by NEWUNIT.
type nativePPPProviderFixture struct {
	file     *os.File
	unit     int32
	link     netlink.Link
	inBorrow atomic.Bool
}

func nativePPPProviderFixtureNew(t *testing.T) *nativePPPProviderFixture {
	t.Helper()
	file, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		if !*nativePPPProviderRequired {
			if errors.Is(err, unix.ENOENT) {
				t.Skipf("actual PPP device is absent: %v", err)
			}
			if errors.Is(err, unix.ENODEV) {
				t.Skipf("actual PPP driver is absent: %v", err)
			}
		}
		t.Fatalf("open actual original PPP creator: %v", err)
	}
	fixture := &nativePPPProviderFixture{file: file, unit: -1}
	t.Cleanup(func() {
		if err := fixture.file.Close(); err != nil {
			t.Errorf("close actual original PPP creator: %v", err)
		}
	})
	if err := fixture.WithDescriptor(func(fd int) error {
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), 0xc004743e,
			uintptr(unsafe.Pointer(&fixture.unit))) //nolint:gosec // actual PPPIOCNEWUNIT UAPI pointer.
		if errno != 0 {
			return errno
		}
		return nil
	}); err != nil {
		t.Fatalf("create actual original PPP unit: %v", err)
	}
	link, err := netlink.LinkByName(fmt.Sprintf("ppp%d", fixture.unit))
	if err != nil {
		t.Fatal(err)
	}
	if link.Type() != "ppp" {
		t.Fatalf("native created unit type=%s, want ppp", link.Type())
	}
	fixture.link = link
	// This independent actual syscall is a fixture prerequisite. It cannot
	// substitute for the production provider performing its own positive query.
	probe := struct {
		ABI      uint32
		Features uint32
		Index    int32
		Unit     int32
		Reserved uint32
	}{ABI: 1, Index: int32(link.Attrs().Index), Unit: fixture.unit}
	if err := fixture.WithDescriptor(func(fd int) error {
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), 0xc0147433,
			uintptr(unsafe.Pointer(&probe))) //nolint:gosec // actual creator-only PPPIOCGOWNER pointer.
		if errno != 0 {
			return errno
		}
		return nil
	}); err != nil {
		t.Fatalf("actual carried PPP query51 must succeed before provider assertions: %v", err)
	}
	if probe.ABI != 1 {
		t.Fatal("actual query51 changed the ABI")
	}
	if probe.Features&1 == 0 {
		t.Fatal("actual query51 lacks guarded SETLINK")
	}
	if probe.Index != int32(link.Attrs().Index) {
		t.Fatal("actual query51 changed the original index")
	}
	if probe.Unit != fixture.unit {
		t.Fatal("actual query51 changed the original unit")
	}
	if probe.Reserved != 0 {
		t.Fatal("actual query51 returned nonzero reserved data")
	}
	return fixture
}

func (fixture *nativePPPProviderFixture) WithDescriptor(operation func(int) error) error {
	connection, err := fixture.file.SyscallConn()
	if err != nil {
		return err
	}
	var operationErr error
	controlErr := connection.Control(func(fd uintptr) {
		fixture.inBorrow.Store(true)
		defer fixture.inBorrow.Store(false)
		operationErr = operation(int(fd))
	})
	return errors.Join(operationErr, controlErr)
}

func nativePPPProviderRequireLink(t *testing.T, index, mtu int, alias string, up bool) {
	t.Helper()
	link, err := netlink.LinkByIndex(index)
	if err != nil {
		t.Fatal(err)
	}
	if link.Attrs().MTU != mtu {
		t.Fatalf("native index %d MTU=%d, want %d", index, link.Attrs().MTU, mtu)
	}
	if link.Attrs().Alias != alias {
		t.Fatalf("native index %d alias=%q, want %q", index, link.Attrs().Alias, alias)
	}
	actualUp := link.Attrs().RawFlags&unix.IFF_UP != 0
	if actualUp != up {
		t.Fatalf("native index %d admin up=%v, want %v", index, actualUp, up)
	}
}
