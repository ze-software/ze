// Design: docs/architecture/l2tp/bng-5-pppoe.md -- native factory descriptor lifecycle
// Related: factory.go -- ordered PPPoX/channel/unit construction

//go:build integration && linux

package kernel

import (
	"errors"
	"os"
	"strconv"
	"testing"

	"github.com/vishvananda/netlink"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"golang.org/x/sys/unix"
)

// TestNativeFactoryOwnsRealPPPoXAndPPP exercises real Linux PPPoX connection,
// shared /dev/ppp construction, interface identity and independent owner closure
// on a virtual Ethernet device. It requires no subscriber or physical hardware.
func TestNativeFactoryOwnsRealPPPoXAndPPP(t *testing.T) {
	if _, err := os.Stat("/dev/ppp"); err != nil {
		if errors.Is(err, os.ErrNotExist) { t.Skip("kernel PPP prerequisite /dev/ppp is absent") }
		t.Fatal(err)
	}
	name := "zepppoe" + strconv.Itoa(os.Getpid())
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: name}}
	if err := netlink.LinkAdd(link); err != nil {
		if errors.Is(err, unix.EPERM) { t.Skip("CAP_NET_ADMIN is required for the virtual access device") }
		t.Fatal(err)
	}
	t.Cleanup(func() { if err := netlink.LinkDel(link); err != nil { t.Error(err) } })
	if err := netlink.LinkSetUp(link); err != nil { t.Fatal(err) }
	request := factoryRequest()
	request.Device = name
	f := newFactoryFixture()
	ops := factoryOps{
		backend: f.ops.backend, openTransport: openTransport,
		setup: pppkernel.DevPPPSetup, newSession: pppkernel.NewSession,
	}
	opened, err := openWithOps(t.Context(), request, ops)
	if err != nil {
		if errors.Is(err, unix.EPROTONOSUPPORT) { t.Skip("kernel PPPoE socket support is absent") }
		if errors.Is(err, unix.EAFNOSUPPORT) { t.Skip("kernel PPPoX socket support is absent") }
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := errors.Join(opened.Dataplane().Close(), opened.CloseTransport()); err != nil { t.Error(err) }
	})
	identity := opened.Dataplane().Identity()
	device, err := netlink.LinkByName(identity.InterfaceName)
	if err != nil { t.Fatal(err) }
	if uint32(device.Attrs().Index) != identity.NativeIndex { t.Fatalf("native identity mismatch: %+v", identity) }
	fd, err := opened.RelayDescriptor()
	if err != nil { t.Fatal(err) }
	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
	if err != nil { t.Fatal(err) }
	if flags&unix.FD_CLOEXEC == 0 { t.Fatal("PPPoX descriptor is inherited across exec") }
	if err := opened.Dataplane().Connect(); err != nil { t.Fatal(err) }
	if err := opened.Dataplane().SetReceiveMRU(1492); err != nil { t.Fatal(err) }
	if err := opened.Dataplane().Close(); err != nil { t.Fatal(err) }
	if _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); err != nil { t.Fatalf("PPP close released transport: %v", err) }
	if err := opened.CloseTransport(); err != nil { t.Fatal(err) }
	if _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); !errors.Is(err, unix.EBADF) { t.Fatalf("transport still open: %v", err) }
	if _, err := opened.RelayDescriptor(); err == nil { t.Fatal("closed transport still lends a descriptor") }
}
