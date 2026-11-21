//go:build integration && linux

// Design: docs/research/l2tpv2-ze-integration.md -- actual original PPP file guard
// Related: created_unit_integration_linux_test.go -- existing native receipt roots
// UAPI: Linux v7.3 plus frozen original-file patches 0007, 0008 and 0009.
// This is a kernel ABI test. It does not prove production SessionHandle wiring.

package kernel_test

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"net"
	"reflect"
	"syscall"
	"testing"
	"unsafe"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

var nativeOwnerRequired = flag.Bool("ppp-owner-guard-required", false,
	"fail when the dedicated Ze-kernel proof lacks its PPP device prerequisite")

// These are test-local definitions of the exact frozen 0007 UAPI, not a new
// production API. linux/if_link.h ends at TAILROOM=69; if_addr.h at MC_USERS=12.
const (
	nativeOwnerABI      = 1
	nativeOwnerSetlink  = 1 << 0
	nativeOwnerAddr4    = 1 << 1
	nativeOwnerAddr6    = 1 << 2
	nativeOwnerLinkAttr = 70
	nativeOwnerAddrAttr = 13
	nativeOwnerIoctl    = 0xc0147433 // _IOWR('t', 51, struct ppp_owner), 20 bytes.
)

// Native layout: 0 abi:u32, 4 features:u32, 8 ifindex:s32, 12 unit:s32,
// 16 reserved:u32. All fields have four-byte alignment, including compat.
type nativeOwnerReply struct {
	abi      uint32
	features uint32
	index    int32
	unit     int32
	reserved uint32
}

// TestNativeOwnerGuardProbe checks the exact native layout and requires the
// negotiated ABI before any test can send a guarded write on this kernel.
func TestNativeOwnerGuardProbe(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	if unsafe.Sizeof(nativeOwnerReply{}) != 20 {
		t.Fatal("test ppp_owner layout does not match the frozen kernel UAPI")
	}
	if unsafe.Alignof(nativeOwnerReply{}) != 4 {
		t.Fatal("test ppp_owner alignment does not match the frozen kernel UAPI")
	}
	for _, change := range []struct {
		name  string
		apply func(*nativeOwnerReply)
	}{
		{"version", func(reply *nativeOwnerReply) { reply.abi++ }},
		{"input features", func(reply *nativeOwnerReply) { reply.features = 1 }},
		{"reserved", func(reply *nativeOwnerReply) { reply.reserved = 1 }},
		{"missing index", func(reply *nativeOwnerReply) { reply.index = 0 }},
		{"invalid unit", func(reply *nativeOwnerReply) { reply.unit = -1 }},
	} {
		t.Run(change.name, func(t *testing.T) {
			reply := fixture.expected()
			change.apply(&reply)
			nativeOwnerErrno(t, nativeOwnerProbe(fixture.fd, &reply), unix.EINVAL)
		})
	}
	for _, mismatch := range []string{"index", "unit"} {
		reply := fixture.expected()
		if mismatch == "index" {
			reply.index++
		} else {
			reply.unit++
		}
		nativeOwnerErrno(t, nativeOwnerProbe(fixture.fd, &reply), unix.ESTALE)
	}
}

// TestNativeOwnerGuardCreatorDuplicate proves dup retains the original struct
// file. Both descriptors are test-owned raw opens, outside WithDescriptor's API.
func TestNativeOwnerGuardCreatorDuplicate(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	duplicate, err := unix.FcntlInt(uintptr(fixture.fd), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { nativeOwnerClose(t, duplicate) })
	fixture.probe(t, duplicate)
	fixture.mtu(t, duplicate, fixture.link.Attrs().Index, 1401)
	nativeOwnerRequireMTU(t, fixture.link, 1401)
}

// TestNativeOwnerGuardAttachedFileRefused proves equal PPPIOCGUNIT labels do
// not grant the authority held by the actual creator's open-file description.
func TestNativeOwnerGuardAttachedFileRefused(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	attached, err := unix.Open("/dev/ppp", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { nativeOwnerClose(t, attached) })
	if err := unix.IoctlSetPointerInt(attached, 0x4004743d, fixture.number); err != nil {
		t.Fatalf("PPPIOCATTACH actual original unit: %v", err)
	}
	number, err := unix.IoctlGetInt(attached, 0x80047456)
	if err != nil {
		t.Fatal(err)
	}
	if number != fixture.number {
		t.Fatalf("attached control labels unit %d, want %d", number, fixture.number)
	}
	reply := fixture.expected()
	nativeOwnerErrno(t, nativeOwnerProbe(attached, &reply), unix.ESTALE)
	fixture.mtu(t, fixture.fd, fixture.link.Attrs().Index, 1402)
	nativeOwnerErrno(t, fixture.send(nativeOwnerMTU(attached, fixture.link.Attrs().Index, 1302)), unix.ESTALE)
	nativeOwnerRequireMTU(t, fixture.link, 1402)
}

// TestNativeOwnerGuardWrongLiveUnitRefused keeps both real PPP units alive
// while directing the second creator's file at the first unit's native index.
func TestNativeOwnerGuardWrongLiveUnitRefused(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	first := nativeOwnerFixtureNew(t)
	second := nativeOwnerFixtureNew(t)
	reply := first.expected()
	nativeOwnerErrno(t, nativeOwnerProbe(second.fd, &reply), unix.ESTALE)
	first.mtu(t, first.fd, first.link.Attrs().Index, 1403)
	nativeOwnerErrno(t, first.send(nativeOwnerMTU(second.fd, first.link.Attrs().Index, 1303)), unix.ESTALE)
	nativeOwnerRequireMTU(t, first.link, 1403)
	second.probe(t, second.fd)
}

// TestNativeOwnerGuardWrongNamespaceRefused proves the ioctl rejects a caller
// outside the creator namespace and a same-index foreign device survives writes.
func TestNativeOwnerGuardWrongNamespaceRefused(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	createdUnitForeignNamespace(t, func() {
		reply := fixture.expected()
		nativeOwnerErrno(t, nativeOwnerProbe(fixture.fd, &reply), unix.EXDEV)
		foreign := nativeOwnerSocket(t)
		victim := nativeOwnerDummy(t, fixture.link.Attrs().Name, fixture.link.Attrs().Index)
		if err := netlink.LinkSetMTU(victim, 1704); err != nil {
			t.Fatal(err)
		}
		nativeOwnerRequireMTU(t, victim, 1704)
		nativeOwnerErrno(t, nativeOwnerSend(foreign, nativeOwnerMTU(fixture.fd, victim.Attrs().Index, 1304)), unix.EOPNOTSUPP)
		nativeOwnerRequireMTU(t, victim, 1704)
	})
	fixture.probe(t, fixture.fd)
}

// TestNativeOwnerGuardClosedFileRefused reserves its netlink socket before
// closing a duplicate, so opening the ACK channel cannot recycle the tested fd.
func TestNativeOwnerGuardClosedFileRefused(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	duplicate, err := unix.FcntlInt(uintptr(fixture.fd), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	nativeOwnerClose(t, duplicate)
	reply := fixture.expected()
	nativeOwnerErrno(t, nativeOwnerProbe(duplicate, &reply), unix.EBADF)
	nativeOwnerErrno(t, fixture.send(nativeOwnerMTU(duplicate, fixture.link.Attrs().Index, 1305)), unix.EBADF)
	fixture.probe(t, fixture.fd)
}

// TestNativeOwnerGuardLinkSetters proves useful guarded MTU and admin changes,
// then proves an unsupported guarded rename refuses without changing the name.
func TestNativeOwnerGuardLinkSetters(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	fixture.mtu(t, fixture.fd, fixture.link.Attrs().Index, 1406)
	nativeOwnerRequireMTU(t, fixture.link, 1406)
	for _, up := range []bool{true, false} {
		request := nativeOwnerLinkRequest(unix.RTM_SETLINK, unix.AF_UNSPEC, fixture.fd, fixture.link.Attrs().Index)
		message, ok := request.Data[0].(*nl.IfInfomsg)
		if !ok {
			t.Fatalf("link request header = %T, want *nl.IfInfomsg", request.Data[0])
		}
		message.Change = unix.IFF_UP
		if up {
			message.Flags = unix.IFF_UP
		}
		if err := fixture.send(request); err != nil {
			t.Fatalf("guarded admin setter up=%v: %v", up, err)
		}
		actual := nativeOwnerLink(t, fixture.link)
		if (actual.Attrs().Flags&net.FlagUp != 0) != up {
			t.Fatalf("guarded admin setter up=%v did not reach native state", up)
		}
	}
	request := nativeOwnerLinkRequest(unix.RTM_SETLINK, unix.AF_UNSPEC, fixture.fd, fixture.link.Attrs().Index)
	request.AddData(nl.NewRtAttr(unix.IFLA_IFNAME, nl.ZeroTerminated("zg-forbidden")))
	nativeOwnerErrno(t, fixture.send(request), unix.EOPNOTSUPP)
	if nativeOwnerLink(t, fixture.link).Attrs().Name != fixture.link.Attrs().Name {
		t.Fatal("unsupported guarded rename changed the original device")
	}
}

// TestNativeOwnerGuardRenameKeepsVictim proves the original file still owns
// its renamed unit, while neither an equal old name nor another file grants it.
func TestNativeOwnerGuardRenameKeepsVictim(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	oldName := fixture.link.Attrs().Name
	if err := netlink.LinkSetName(fixture.link, "zg-renamed"); err != nil {
		t.Fatal(err)
	}
	victim := nativeOwnerDummy(t, oldName, 0)
	if err := netlink.LinkSetMTU(victim, 1607); err != nil {
		t.Fatal(err)
	}
	fixture.probe(t, fixture.fd)
	fixture.mtu(t, fixture.fd, fixture.link.Attrs().Index, 1407)
	nativeOwnerRequireMTU(t, fixture.link, 1407)
	nativeOwnerErrno(t, fixture.send(nativeOwnerMTU(fixture.fd, victim.Attrs().Index, 1307)), unix.EOPNOTSUPP)
	nativeOwnerRequireMTU(t, victim, 1607)
}

// TestNativeOwnerGuardUnregisterReuseRefused retains the dead original file,
// then proves a native device reusing both old name and index is not its owner.
func TestNativeOwnerGuardUnregisterReuseRefused(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	if err := netlink.LinkDel(fixture.link); err != nil {
		t.Fatal(err)
	}
	reply := fixture.expected()
	nativeOwnerErrno(t, nativeOwnerProbe(fixture.fd, &reply), unix.ESTALE)
	victim := nativeOwnerDummy(t, fixture.link.Attrs().Name, fixture.link.Attrs().Index)
	if err := netlink.LinkSetMTU(victim, 1608); err != nil {
		t.Fatal(err)
	}
	nativeOwnerErrno(t, fixture.send(nativeOwnerMTU(fixture.fd, victim.Attrs().Index, 1308)), unix.EOPNOTSUPP)
	nativeOwnerRequireMTU(t, victim, 1608)
}

// TestNativeOwnerGuardAddress4 exercises both guarded native writes and an
// other live creator refusal against the same already-installed point-to-point pair.
func TestNativeOwnerGuardAddress4(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativeOwnerAddressCase(t, unix.AF_INET, "192.0.2.90/32", "192.0.2.91/32")
}

// TestNativeOwnerGuardAddress6 applies the same original-file contract to
// IPv6, with NODAD preventing unrelated address readiness from deciding the test.
func TestNativeOwnerGuardAddress6(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativeOwnerAddressCase(t, unix.AF_INET6, "2001:db8:90::1/128", "2001:db8:90::2/128")
}

// TestNativeOwnerGuardBridgeSetlinkRefused drives the AF_BRIDGE dispatch path,
// proving a guard cannot be ignored while an unguarded control changes learning.
func TestNativeOwnerGuardBridgeSetlinkRefused(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativeOwnerBridgeCase(t, unix.RTM_SETLINK)
}

// TestNativeOwnerGuardBridgeDellinkRefused requires the same refusal before
// AF_BRIDGE DELLINK dispatch; a SETLINK learning control proves the port is useful.
func TestNativeOwnerGuardBridgeDellinkRefused(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	nativeOwnerBridgeCase(t, unix.RTM_DELLINK)
}

// TestNativeOwnerGuardAlias proves the separately advertised birth-alias setter
// works for its creator and rejects another live creator before changing state.
func TestNativeOwnerGuardAlias(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	first := nativeOwnerFixtureNew(t)
	second := nativeOwnerFixtureNew(t)
	nativeOwnerAliasReady(t, first)
	if err := first.send(nativeOwnerAlias(first.fd, first.link.Attrs().Index, "ze-original-birth")); err != nil {
		t.Fatal(err)
	}
	nativeOwnerRequireAlias(t, first.link, "ze-original-birth")
	nativeOwnerErrno(t, first.send(nativeOwnerAlias(second.fd, first.link.Attrs().Index, "ze-foreign-birth")), unix.ESTALE)
	nativeOwnerRequireAlias(t, first.link, "ze-original-birth")
}

// TestNativeOwnerGuardAliasRenameKeepsVictim proves the birth stamp reaches the
// renamed original while its old-name replacement retains its own actual alias.
func TestNativeOwnerGuardAliasRenameKeepsVictim(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	nativeOwnerAliasReady(t, fixture)
	oldName := fixture.link.Attrs().Name
	if err := netlink.LinkSetName(fixture.link, "zg-alias-orig"); err != nil {
		t.Fatal(err)
	}
	victim := nativeOwnerDummy(t, oldName, 0)
	if err := netlink.LinkSetAlias(victim, "victim-control"); err != nil {
		t.Fatal(err)
	}
	nativeOwnerRequireAlias(t, victim, "victim-control")
	if err := fixture.send(nativeOwnerAlias(fixture.fd, fixture.link.Attrs().Index, "ze-renamed-birth")); err != nil {
		t.Fatal(err)
	}
	nativeOwnerRequireAlias(t, fixture.link, "ze-renamed-birth")
	nativeOwnerErrno(t, fixture.send(nativeOwnerAlias(fixture.fd, victim.Attrs().Index, "forbidden-birth")), unix.EOPNOTSUPP)
	nativeOwnerRequireAlias(t, victim, "victim-control")
}

// TestNativeOwnerGuardAliasReuseKeepsVictim refuses a dead original file after
// another device reuses both native index and name, retaining the victim alias.
func TestNativeOwnerGuardAliasReuseKeepsVictim(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	nativeOwnerAliasReady(t, fixture)
	if err := netlink.LinkDel(fixture.link); err != nil {
		t.Fatal(err)
	}
	victim := nativeOwnerDummy(t, fixture.link.Attrs().Name, fixture.link.Attrs().Index)
	if err := netlink.LinkSetAlias(victim, "reused-control"); err != nil {
		t.Fatal(err)
	}
	nativeOwnerRequireAlias(t, victim, "reused-control")
	nativeOwnerErrno(t, fixture.send(nativeOwnerAlias(fixture.fd, victim.Attrs().Index, "forbidden-birth")), unix.EOPNOTSUPP)
	nativeOwnerRequireAlias(t, victim, "reused-control")
}

func nativeOwnerAliasReady(t *testing.T, fixture *nativeOwnerFixture) {
	t.Helper()
	reply := fixture.expected()
	if err := nativeOwnerProbe(fixture.fd, &reply); err != nil {
		t.Fatal(err)
	}
	if reply.features&(1<<5) == 0 {
		t.Fatal("PPP original-file guard ABI lacks distinct ALIAS bit before guarded birth stamping")
	}
}

func nativeOwnerAlias(fd, index int, alias string) *nl.NetlinkRequest {
	request := nativeOwnerLinkRequest(unix.RTM_SETLINK, unix.AF_UNSPEC, fd, index)
	request.AddData(nl.NewRtAttr(unix.IFLA_IFALIAS, nl.ZeroTerminated(alias)))
	return request
}

func nativeOwnerRequireAlias(t *testing.T, link netlink.Link, alias string) {
	t.Helper()
	if got := nativeOwnerLink(t, link).Attrs().Alias; got != alias {
		t.Fatalf("native index %d alias=%q, want %q", link.Attrs().Index, got, alias)
	}
}

// TestNativeOwnerGuardCloseRace joins a test-owned duplicate's closer before
// every assertion. Either close wins before fget (EBADF, no write), or the held
// original file completes the write; no descriptor can be recycled in between.
func TestNativeOwnerGuardCloseRace(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	for iteration := range 32 {
		prior := nativeOwnerLink(t, fixture.link).Attrs().MTU
		duplicate, err := unix.FcntlInt(uintptr(fixture.fd), unix.F_DUPFD_CLOEXEC, 0)
		if err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		joined := make(chan error, 1)
		go func() {
			<-start
			joined <- unix.Close(duplicate)
		}()
		close(start)
		mtu := 1350 + iteration
		writeErr := fixture.send(nativeOwnerMTU(duplicate, fixture.link.Attrs().Index, mtu))
		closeErr := <-joined
		if closeErr != nil {
			t.Fatalf("join original-file duplicate closer: %v", closeErr)
		}
		if writeErr == nil {
			nativeOwnerRequireMTU(t, fixture.link, mtu)
		} else {
			nativeOwnerErrno(t, writeErr, unix.EBADF)
			nativeOwnerRequireMTU(t, fixture.link, prior)
		}
		fixture.probe(t, fixture.fd)
	}
}

// TestNativeOwnerGuardLinkCreationDeletionRefused checks the generic link paths
// cannot reinterpret a guarded request as an unguarded modification or deletion.
func TestNativeOwnerGuardLinkCreationDeletionRefused(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativeOwnerFixtureNew(t)
	fixture.mtu(t, fixture.fd, fixture.link.Attrs().Index, 1410)
	for _, command := range []int{unix.RTM_NEWLINK, unix.RTM_DELLINK} {
		request := nativeOwnerLinkRequest(command, unix.AF_UNSPEC, fixture.fd, fixture.link.Attrs().Index)
		request.AddData(nl.NewRtAttr(unix.IFLA_MTU, nl.Uint32Attr(1310)))
		nativeOwnerErrno(t, fixture.send(request), unix.EOPNOTSUPP)
		fixture.probe(t, fixture.fd)
		nativeOwnerRequireMTU(t, fixture.link, 1410)
	}
}

// nativeOwnerFixture owns both raw descriptors until cleanup. No concurrent
// operation, worker or lease exists; every native ACK is consumed before return.
type nativeOwnerFixture struct {
	fd     int
	socket int
	number int
	link   netlink.Link
}

func nativeOwnerFixtureNew(t *testing.T) *nativeOwnerFixture {
	t.Helper()
	fd, err := unix.Open("/dev/ppp", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		if !*nativeOwnerRequired {
			if errors.Is(err, unix.ENOENT) {
				t.Skipf("PPP device prerequisite is absent: %v", err)
			}
			if errors.Is(err, unix.ENODEV) {
				t.Skipf("PPP driver prerequisite is absent: %v", err)
			}
		}
		t.Fatalf("dedicated PPP guard fixture cannot open /dev/ppp: %v", err)
	}
	t.Cleanup(func() { nativeOwnerClose(t, fd) })
	number := int32(-1)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), 0xc004743e,
		uintptr(unsafe.Pointer(&number))) //nolint:gosec // actual PPPIOCNEWUNIT UAPI pointer.
	if errno != 0 {
		t.Fatalf("create task-owned real PPP unit: %v", errno)
	}
	link, err := netlink.LinkByName(fmt.Sprintf("ppp%d", number))
	if err != nil {
		t.Fatal(err)
	}
	if link.Type() != "ppp" {
		t.Fatalf("created unit type=%q, want ppp", link.Type())
	}
	if link.Attrs().Index <= 0 {
		t.Fatal("created PPP unit has no positive native index")
	}
	fixture := &nativeOwnerFixture{fd: fd, socket: nativeOwnerSocket(t), number: int(number), link: link}
	// ABI absence is a semantic failure even outside the dedicated carrier.
	// Old kernels may ignore an unknown netlink attr, so probing is mandatory.
	fixture.probe(t, fd)
	return fixture
}

func (fixture *nativeOwnerFixture) expected() nativeOwnerReply {
	return nativeOwnerReply{abi: nativeOwnerABI, index: int32(fixture.link.Attrs().Index), unit: int32(fixture.number)}
}

func nativeOwnerProbe(fd int, reply *nativeOwnerReply) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), nativeOwnerIoctl,
		uintptr(unsafe.Pointer(reply))) //nolint:gosec // exact struct ppp_owner UAPI pointer.
	if errno != 0 {
		return errno
	}
	return nil
}

func (fixture *nativeOwnerFixture) probe(t *testing.T, fd int) {
	t.Helper()
	reply := fixture.expected()
	if err := nativeOwnerProbe(fd, &reply); err != nil {
		t.Fatalf("PPP original-file guard ABI probe failed before guarded writes: %v", err)
	}
	if reply.abi != nativeOwnerABI {
		t.Fatalf("PPP guard ABI=%d, want %d", reply.abi, nativeOwnerABI)
	}
	if reply.index != int32(fixture.link.Attrs().Index) {
		t.Fatalf("PPP guard reply changed expected native index: %+v", reply)
	}
	if reply.unit != int32(fixture.number) {
		t.Fatalf("PPP guard reply changed expected unit: %+v", reply)
	}
	if reply.reserved != 0 {
		t.Fatalf("PPP guard reply has nonzero reserved bits: %+v", reply)
	}
	want := uint32(nativeOwnerSetlink | nativeOwnerAddr4 | nativeOwnerAddr6)
	if reply.features&want != want {
		t.Fatalf("PPP original-file guard lacks required link/address features: got %#x need %#x", reply.features, want)
	}
}

func nativeOwnerClose(t *testing.T, fd int) {
	t.Helper()
	if err := unix.Close(fd); err != nil {
		t.Errorf("close task-owned native descriptor %d: %v", fd, err)
	}
}

func nativeOwnerSocket(t *testing.T) int {
	t.Helper()
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { nativeOwnerClose(t, fd) })
	if err := unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		t.Fatal(err)
	}
	limit := unix.Timeval{Sec: 3}
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &limit); err != nil {
		t.Fatal(err)
	}
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_SNDTIMEO, &limit); err != nil {
		t.Fatal(err)
	}
	return fd
}

func (fixture *nativeOwnerFixture) send(request *nl.NetlinkRequest) error {
	return nativeOwnerSend(fixture.socket, request)
}

// nativeOwnerSend uses a preopened socket, never opens another fd, and accepts
// only a matching kernel ACK. Eight receives and per-call timeouts bound it.
// This helper uses only test-owned raw fds; it is not a production lease model.
func nativeOwnerSend(socket int, request *nl.NetlinkRequest) error {
	if err := unix.Sendto(socket, request.Serialize(), 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return err
	}
	var buffer [8192]byte
	for range 8 {
		count, from, err := unix.Recvfrom(socket, buffer[:], 0)
		if err != nil {
			return err
		}
		address, ok := from.(*unix.SockaddrNetlink)
		if !ok {
			return errors.New("native guard ACK sender is not netlink")
		}
		if address.Pid != 0 {
			return fmt.Errorf("native guard ACK sender pid=%d, want kernel", address.Pid)
		}
		messages, err := syscall.ParseNetlinkMessage(buffer[:count])
		if err != nil {
			return err
		}
		for _, message := range messages {
			if message.Header.Seq != request.Seq {
				continue
			}
			if message.Header.Type != unix.NLMSG_ERROR {
				return fmt.Errorf("native guarded write reply type=%d, want ACK", message.Header.Type)
			}
			if len(message.Data) < 4 {
				return errors.New("native guarded write ACK has no errno")
			}
			code := int32(binary.NativeEndian.Uint32(message.Data[:4]))
			if code == 0 {
				return nil
			}
			if code > 0 {
				return fmt.Errorf("native guarded write ACK has invalid positive errno %d", code)
			}
			return unix.Errno(-code)
		}
	}
	return errors.New("native guarded write received no matching ACK within eight messages")
}

func nativeOwnerLinkRequest(command, family, fd, index int) *nl.NetlinkRequest {
	request := nl.NewNetlinkRequest(command, unix.NLM_F_ACK)
	message := nl.NewIfInfomsg(family)
	message.Index = int32(index)
	request.AddData(message)
	request.AddData(nl.NewRtAttr(nativeOwnerLinkAttr, nl.Uint32Attr(uint32(int32(fd)))))
	return request
}

func nativeOwnerMTU(fd, index, mtu int) *nl.NetlinkRequest {
	request := nativeOwnerLinkRequest(unix.RTM_SETLINK, unix.AF_UNSPEC, fd, index)
	request.AddData(nl.NewRtAttr(unix.IFLA_MTU, nl.Uint32Attr(uint32(mtu))))
	return request
}

func (fixture *nativeOwnerFixture) mtu(t *testing.T, fd, index, mtu int) {
	t.Helper()
	if err := fixture.send(nativeOwnerMTU(fd, index, mtu)); err != nil {
		t.Fatalf("guarded native MTU write: %v", err)
	}
}

func nativeOwnerErrno(t *testing.T, err error, want unix.Errno) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("native original-file guard returned %v, want %v", err, want)
	}
}

func nativeOwnerLink(t *testing.T, link netlink.Link) netlink.Link {
	t.Helper()
	actual, err := netlink.LinkByIndex(link.Attrs().Index)
	if err != nil {
		t.Fatal(err)
	}
	return actual
}

func nativeOwnerRequireMTU(t *testing.T, link netlink.Link, mtu int) {
	t.Helper()
	if got := nativeOwnerLink(t, link).Attrs().MTU; got != mtu {
		t.Fatalf("native device index=%d MTU=%d, want %d", link.Attrs().Index, got, mtu)
	}
}

func nativeOwnerDummy(t *testing.T, name string, index int) netlink.Link {
	t.Helper()
	link := &netlink.Dummy{Name: name, Index: index}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatalf("create owned native victim: %v", err)
	}
	// Capture this namespace's cleanup socket before returning to another one.
	cleanup, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer cleanup.Close()
		if err := cleanup.LinkDel(link); err != nil {
			t.Errorf("delete owned native victim: %v", err)
		}
	})
	actual, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatal(err)
	}
	if index > 0 {
		if actual.Attrs().Index != index {
			t.Fatalf("owned victim index=%d, want reused index %d", actual.Attrs().Index, index)
		}
	}
	return actual
}

func nativeOwnerAddressCase(t *testing.T, family int, local, peer string) {
	t.Helper()
	fixture := nativeOwnerFixtureNew(t)
	other := nativeOwnerFixtureNew(t)
	address, err := netlink.ParseAddr(local)
	if err != nil {
		t.Fatal(err)
	}
	remote, err := netlink.ParseAddr(peer)
	if err != nil {
		t.Fatal(err)
	}
	address.Peer = remote.IPNet
	add := nativeOwnerAddressRequest(unix.RTM_NEWADDR, family, fixture.fd, fixture.link.Attrs().Index, address)
	if err := fixture.send(add); err != nil {
		t.Fatalf("guarded native address add: %v", err)
	}
	before, err := netlink.AddrList(fixture.link, family)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, actual := range before {
		if actual.Equal(*address) {
			if actual.Peer == nil {
				t.Fatal("native original address has no point-to-point peer")
			}
			if actual.PeerEqual(*address) {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("guarded native address add did not install exact pair: %+v", before)
	}
	for _, command := range []int{unix.RTM_NEWADDR, unix.RTM_DELADDR} {
		request := nativeOwnerAddressRequest(command, family, other.fd, fixture.link.Attrs().Index, address)
		nativeOwnerErrno(t, fixture.send(request), unix.ESTALE)
		after, err := netlink.AddrList(fixture.link, family)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(after, before) {
			t.Fatalf("nonowner address command %d changed native state: before=%+v after=%+v", command, before, after)
		}
	}
	remove := nativeOwnerAddressRequest(unix.RTM_DELADDR, family, fixture.fd, fixture.link.Attrs().Index, address)
	if err := fixture.send(remove); err != nil {
		t.Fatalf("guarded native address withdrawal: %v", err)
	}
	after, err := netlink.AddrList(fixture.link, family)
	if err != nil {
		t.Fatal(err)
	}
	for _, actual := range after {
		if actual.Equal(*address) {
			if actual.Peer == nil {
				t.Fatal("native original address has no point-to-point peer")
			}
			if actual.PeerEqual(*address) {
				t.Fatalf("guarded native withdrawal retained original pair: %+v", after)
			}
		}
	}
}

func nativeOwnerAddressRequest(command, family, fd, index int, address *netlink.Addr) *nl.NetlinkRequest {
	flags := unix.NLM_F_ACK
	if command == unix.RTM_NEWADDR {
		flags |= unix.NLM_F_CREATE | unix.NLM_F_EXCL
	}
	request := nl.NewNetlinkRequest(command, flags)
	prefix, _ := address.Mask.Size()
	message := &nl.IfAddrmsg{Family: uint8(family), Prefixlen: uint8(prefix), Index: uint32(index)}
	local := address.IP.To4()
	peer := address.Peer.IP.To4()
	if family == unix.AF_INET6 {
		message.Flags = unix.IFA_F_NODAD
		local = address.IP.To16()
		peer = address.Peer.IP.To16()
	}
	request.AddData(message)
	request.AddData(nl.NewRtAttr(unix.IFA_LOCAL, local))
	request.AddData(nl.NewRtAttr(unix.IFA_ADDRESS, peer))
	request.AddData(nl.NewRtAttr(nativeOwnerAddrAttr, nl.Uint32Attr(uint32(int32(fd)))))
	return request
}

func nativeOwnerBridgeCase(t *testing.T, command int) {
	t.Helper()
	fixture := nativeOwnerFixtureNew(t)
	bridge := &netlink.Bridge{Name: "zg-bridge"}
	if err := netlink.LinkAdd(bridge); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := netlink.LinkDel(bridge); err != nil {
			t.Error(err)
		}
	})
	port := nativeOwnerDummy(t, "zg-port", 0)
	if err := netlink.LinkSetMaster(port, bridge); err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{false, true} {
		if err := netlink.LinkSetLearning(port, enabled); err != nil {
			t.Fatalf("unguarded native bridge control: %v", err)
		}
		nativeOwnerLearning(t, port, enabled)
	}
	if err := netlink.BridgeVlanAdd(port, 123, false, false, false, false); err != nil {
		t.Fatal(err)
	}
	if err := netlink.BridgeVlanDel(port, 123, false, false, false, false); err != nil {
		t.Fatalf("unguarded bridge DELLINK positive control: %v", err)
	}
	if nativeOwnerVLAN(t, port, 123) {
		t.Fatal("unguarded bridge DELLINK left its VLAN installed")
	}
	if err := netlink.BridgeVlanAdd(port, 123, false, false, false, false); err != nil {
		t.Fatal(err)
	}
	if !nativeOwnerVLAN(t, port, 123) {
		t.Fatal("native bridge victim VLAN is not installed")
	}
	request := nativeOwnerLinkRequest(command, unix.AF_BRIDGE, fixture.fd, port.Attrs().Index)
	if command == unix.RTM_SETLINK {
		info := nl.NewRtAttr(unix.IFLA_PROTINFO|unix.NLA_F_NESTED, nil)
		info.AddRtAttr(nl.IFLA_BRPORT_LEARNING, []byte{0})
		request.AddData(info)
	} else {
		info := nl.NewRtAttr(unix.IFLA_AF_SPEC, nil)
		vlan := &nl.BridgeVlanInfo{Vid: 123}
		info.AddRtAttr(nl.IFLA_BRIDGE_VLAN_INFO, vlan.Serialize())
		request.AddData(info)
	}
	nativeOwnerErrno(t, fixture.send(request), unix.EOPNOTSUPP)
	nativeOwnerLearning(t, port, true)
	if !nativeOwnerVLAN(t, port, 123) {
		t.Fatal("guarded bridge request changed victim VLAN membership")
	}
	if got := nativeOwnerLink(t, port).Attrs().MasterIndex; got != bridge.Attrs().Index {
		t.Fatalf("guarded bridge request changed port master to %d", got)
	}
}

func nativeOwnerLearning(t *testing.T, port netlink.Link, want bool) {
	t.Helper()
	info, err := netlink.LinkGetProtinfo(port)
	if err != nil {
		t.Fatal(err)
	}
	if info.Learning != want {
		t.Fatalf("native bridge learning=%v, want %v", info.Learning, want)
	}
}

func nativeOwnerVLAN(t *testing.T, port netlink.Link, vid uint16) bool {
	t.Helper()
	all, err := netlink.BridgeVlanList()
	if err != nil {
		t.Fatal(err)
	}
	for _, info := range all[int32(port.Attrs().Index)] {
		if info.Vid == vid {
			return true
		}
	}
	return false
}
