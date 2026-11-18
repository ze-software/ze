//go:build integration && linux

// Design: docs/research/l2tpv2-ze-integration.md -- actual PPP creation receipts
// These roots exercise a real /dev/ppp unit and the actual registered backend.
// No PPP negotiation, attached channel, forwarding or release-seal claim.

package kernel_test

import (
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
	"github.com/ze-software/ze/internal/test/userns"
)

func TestCreatedUnitNamedRenameKeepsOriginal(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _, original := createdUnitNative(t, "zc-native")
	createdUnitZero(t, unit)
	if err := netlink.LinkSetName(original, "zc-renamed"); err != nil {
		t.Fatal(err)
	}
	createdUnitTraffic(t, "zc-native", 0)
	createdUnitZero(t, unit)
}

func TestCreatedUnitUnnamedUsesActualEcho(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _, original := createdUnitNative(t, "")
	if unit.Identity().InterfaceName != fmt.Sprintf("ppp%d", unit.UnitNumber()) {
		t.Fatalf("unnamed PPP echo %+v does not match native unit %d", unit.Identity(), unit.UnitNumber())
	}
	if original.Type() != "ppp" {
		t.Fatalf("actual created link has type %q", original.Type())
	}
	createdUnitZero(t, unit)
}

func TestCreatedUnitForeignCallerKeepsOriginal(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _, original := createdUnitNative(t, "zc-native")
	createdUnitForeignNamespace(t, func() {
		createdUnitTraffic(t, "zc-native", original.Attrs().Index)
		createdUnitZero(t, unit)
	})
	createdUnitZero(t, unit)
}

func TestCreatedUnitDeleteReuseRefusesReplacement(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _, original := createdUnitNative(t, "zc-native")
	createdUnitZero(t, unit)
	if err := netlink.LinkDel(original); err != nil {
		t.Fatal(err)
	}
	if err := unit.WithDescriptor(func(fd int) error {
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		if _, err := unix.Poll(fds, 3000); err != nil {
			return err
		}
		if fds[0].Revents&unix.POLLHUP == 0 {
			return fmt.Errorf("deleted original file events %#x lack POLLHUP", fds[0].Revents)
		}
		// The still-readable unit number is a label, not proof of liveness.
		number, err := unix.IoctlGetInt(fd, 0x80047456)
		if err == nil && number != unit.UnitNumber() {
			return fmt.Errorf("dead original unit number changed to %d", number)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	createdUnitTraffic(t, "zc-native", original.Attrs().Index)
	if got, err := unit.Counters(); err == nil {
		t.Fatalf("dead original read replacement counters %+v", got)
	}
}

func TestCreatedUnitMoveReuseRefusesReplacement(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	creator, target := createdUnitMoveNamespaces(t)
	unit, _, original := createdUnitNative(t, "zc-native")
	createdUnitZero(t, unit)
	targetLinks, err := netlink.NewHandleAt(target, unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	defer targetLinks.Close()
	if err := netlink.LinkSetNsFd(original, int(target)); err != nil {
		t.Fatalf("move actual PPP unit: %v", err)
	}
	moved, err := targetLinks.LinkByName("zc-native")
	if err != nil || moved == nil || moved.Type() != "ppp" {
		t.Fatalf("target did not receive original PPP unit: link=%v error=%v", moved, err)
	}
	creatorLinks, err := netlink.NewHandleAt(creator, unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	defer creatorLinks.Close()
	if _, err := creatorLinks.LinkByName("zc-native"); err == nil {
		t.Fatal("original PPP unit is still visible in creator namespace")
	} else {
		var absent netlink.LinkNotFoundError
		if !errors.As(err, &absent) {
			t.Fatalf("creator absence was not confirmed: %v", err)
		}
	}
	createdUnitTraffic(t, "zc-native", original.Attrs().Index)
	if got, err := unit.Counters(); err == nil {
		t.Fatalf("moved original read creator replacement counters %+v", got)
	}
}

func TestCreatedUnitBackendClosureRefusesRead(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, backend, _ := createdUnitNative(t, "zc-native")
	createdUnitZero(t, unit)
	if err := backend.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := unit.Counters(); err == nil {
		t.Fatalf("closed creator socket still returned %+v", got)
	}
}

// A real create followed by a lost acknowledgment still owns the actual unit
// file. A partial echo is retained for diagnosis; it cannot authorize reads.
func TestCreatedUnitPartialEchoKeepsOriginalCustody(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	createdUnitFailedReply(t, false, false)
}

// Without an echo, the owner still retains the original /dev/ppp file. Its
// native unit number can identify the test's positive control, never a reader.
func TestCreatedUnitLostEchoKeepsOriginalCustody(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	createdUnitFailedReply(t, true, false)
}

func TestCreatedUnitPanicAfterCreationKeepsOriginalCustody(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	createdUnitFailedReply(t, true, true)
}

type createdUnitLostReply struct {
	iface.Backend
	loseEcho   bool
	panicAfter bool
	cause      error
}

func (backend *createdUnitLostReply) CreateLinkCounterReader(request *nl.NetlinkRequest) (netlink.Link, func() (*netlink.LinkStatistics, error), bool, error) {
	source, ok := backend.Backend.(interface {
		CreateLinkCounterReader(*nl.NetlinkRequest) (netlink.Link, func() (*netlink.LinkStatistics, error), bool, error)
	})
	if !ok {
		return nil, nil, false, errors.New("actual backend lacks creation-bound readers")
	}
	link, read, attempted, err := source.CreateLinkCounterReader(request)
	if err != nil {
		return link, read, attempted, err
	}
	if backend.panicAfter {
		panic(backend.cause)
	}
	if backend.loseEcho {
		return nil, nil, attempted, backend.cause
	}
	return link, nil, attempted, backend.cause
}

func createdUnitFailedReply(t *testing.T, loseEcho, panicAfter bool) {
	t.Helper()
	backend, err := iface.OpenBackend("netlink")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	lost := errors.New("controlled loss after actual original unit creation")
	creator, err := pppkernel.NewUnitCreator(&createdUnitLostReply{Backend: backend, loseEcho: loseEcho, panicAfter: panicAfter, cause: lost})
	if err != nil {
		t.Fatal(err)
	}
	unit, err := creator.Create("")
	if unit != nil {
		t.Cleanup(func() {
			if err := unit.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if !errors.Is(err, lost) || unit == nil || !unit.Attempted() {
		t.Fatalf("lost reply discarded original attempt: unit=%v error=%v", unit, err)
	}
	var number int
	if err := unit.WithDescriptor(func(fd int) error {
		var err error
		number, err = unix.IoctlGetInt(fd, 0x80047456)
		return err
	}); err != nil {
		t.Fatalf("uncertain original unit file was not retained: %v", err)
	}
	original, err := netlink.LinkByName(fmt.Sprintf("ppp%d", number))
	if err != nil || original == nil || original.Type() != "ppp" {
		t.Fatalf("controlled lost reply did not leave the real original alive: %v, %v", original, err)
	}
	identity := unit.Identity()
	if loseEcho {
		if identity != (ppp.SessionIdentity{}) {
			t.Fatalf("lost echo invented an identity %+v", identity)
		}
	} else if identity.NativeIndex != uint32(original.Attrs().Index) || identity.InterfaceName != original.Attrs().Name {
		t.Fatalf("partial echo was discarded or replaced: %+v versus %v", identity, original)
	}
	if got, err := unit.Counters(); err == nil {
		t.Fatalf("unconfirmed original creation returned counters %+v", got)
	}
	if err := unit.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := netlink.LinkByIndex(original.Attrs().Index); err == nil {
		t.Fatal("closing uncertain original file did not release its actual unit")
	} else {
		var absent netlink.LinkNotFoundError
		if !errors.As(err, &absent) {
			t.Fatalf("original absence was not confirmed after file close: %v", err)
		}
	}
}

func createdUnitNative(t *testing.T, name string) (*pppkernel.CreatedUnit, iface.Backend, netlink.Link) {
	t.Helper()
	backend, err := iface.OpenBackend("netlink")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	creator, err := pppkernel.NewUnitCreator(backend)
	if err != nil {
		t.Fatal(err)
	}
	unit, err := creator.Create(name)
	if unit != nil {
		t.Cleanup(func() {
			if err := unit.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	if unit == nil || !unit.Attempted() {
		t.Fatal("successful native creation has no original attempted receipt")
	}
	identity := unit.Identity()
	if identity.NativeIndex == 0 || identity.InterfaceName == "" || unit.UnitNumber() < 0 {
		t.Fatalf("creation did not retain actual identity and unit number: %+v unit=%d", identity, unit.UnitNumber())
	}
	if name != "" && identity.InterfaceName != name {
		t.Fatalf("native created name %q differs from requested %q", identity.InterfaceName, name)
	}
	original, err := netlink.LinkByIndex(int(identity.NativeIndex))
	if err != nil {
		t.Fatal(err)
	}
	if original.Type() != "ppp" || original.Attrs().Name != identity.InterfaceName {
		t.Fatalf("actual native PPP differs from creation echo: %v %+v", original, identity)
	}
	if err := unit.WithDescriptor(func(fd int) error {
		number, err := unix.IoctlGetInt(fd, 0x80047456)
		if err != nil {
			return err
		}
		if number != unit.UnitNumber() {
			return fmt.Errorf("actual original unit %d differs from receipt %d", number, unit.UnitNumber())
		}
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		if _, err := unix.Poll(fds, 0); err != nil {
			return err
		}
		if fds[0].Revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			return fmt.Errorf("live original file poll events %#x", fds[0].Revents)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return unit, backend, original
}

func createdUnitZero(t *testing.T, unit *pppkernel.CreatedUnit) {
	t.Helper()
	got, err := unit.Counters()
	if err != nil {
		t.Fatalf("read live original PPP counters: %v", err)
	}
	if got.RxBytes != 0 || got.TxBytes != 0 || got.RxPackets != 0 || got.TxPackets != 0 {
		t.Fatalf("idle original traffic counters = %+v", got)
	}
	for name, reading := range map[string]ppp.CounterReading{
		"rx errors": got.RxErrors, "tx errors": got.TxErrors,
		"rx drops": got.RxDropped, "tx drops": got.TxDropped,
	} {
		if value, err := reading.Value(); err != nil || value != 0 {
			t.Fatalf("idle original %s = %d, %v", name, value, err)
		}
	}
}

func createdUnitTraffic(t *testing.T, name string, index int) {
	t.Helper()
	cleanup, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	var owned netlink.Link
	t.Cleanup(func() {
		defer cleanup.Close()
		if owned != nil {
			if err := cleanup.LinkDel(owned); err != nil {
				t.Error(err)
			}
		}
	})
	dummy := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: name, Index: index}}
	if err := netlink.LinkAdd(dummy); err != nil {
		t.Fatal(err)
	}
	owned = dummy
	actual, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatal(err)
	}
	owned = actual
	if index > 0 && actual.Attrs().Index != index {
		t.Fatalf("replacement index = %d, want original %d", actual.Attrs().Index, index)
	}
	if err := netlink.LinkSetUp(actual); err != nil {
		t.Fatal(err)
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
	}()
	var frame [60]byte
	frame[0], frame[6], frame[12], frame[13] = 2, 2, 0x88, 0xb5
	if err := unix.Sendto(fd, frame[:], 0, &unix.SockaddrLinklayer{Ifindex: actual.Attrs().Index, Protocol: 0xb588, Halen: 6}); err != nil {
		t.Fatal(err)
	}
	observed, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatal(err)
	}
	stats := observed.Attrs().Statistics
	if stats == nil || stats.TxBytes == 0 || stats.TxPackets == 0 {
		t.Fatalf("replacement positive control has no real traffic: %+v", stats)
	}
}

func createdUnitForeignNamespace(t *testing.T, work func()) {
	t.Helper()
	runtime.LockOSThread()
	original, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	defer func() {
		if err := original.Close(); err != nil {
			t.Error(err)
		}
	}()
	other, err := netns.New()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	defer func() {
		if err := other.Close(); err != nil {
			t.Error(err)
		}
	}()
	defer func() {
		if err := netns.Set(original); err != nil {
			t.Fatalf("restore original namespace: %v", err)
		}
		runtime.UnlockOSThread()
	}()
	work()
}

func createdUnitMoveNamespaces(t *testing.T) (netns.NsHandle, netns.NsHandle) {
	t.Helper()
	var creator, target netns.NsHandle
	createdUnitForeignNamespace(t, func() {
		var err error
		target, err = netns.Get()
		if err != nil {
			t.Fatal(err)
		}
	})
	var err error
	creator, err = netns.Get()
	if err != nil {
		if closeErr := target.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := target.Close(); err != nil {
			t.Error(err)
		}
		if err := creator.Close(); err != nil {
			t.Error(err)
		}
	})
	return creator, target
}
