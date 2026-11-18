// Design: docs/architecture/l2tp/bng-5-pppoe.md -- actual typed channel and unit setup

//go:build integration && linux

package kernel_test

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
	"github.com/ze-software/ze/internal/test/userns"
)

// The unnamed path uses the actual echo and original file, not a guessed index.
func TestDevPPPSetupTypedNativeUnnamed(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	creator, transport := typedSetupNative(t, nil)
	channel, unit, err := pppkernel.DevPPPSetup(creator, transport, "")
	typedSetupCleanup(t, channel, unit)
	if err != nil {
		t.Fatal(err)
	}
	typedSetupOriginal(t, channel, unit)
	if want := fmt.Sprintf("ppp%d", unit.UnitNumber()); unit.Identity().InterfaceName != want {
		t.Fatalf("actual unnamed echo=%q, want native unit label %q", unit.Identity().InterfaceName, want)
	}
}

// An original channel whose actual file received descriptor zero remains owned.
// userns.Enter re-executes this one top-level test in an isolated process.
func TestDevPPPSetupTypedNativeDescriptorZero(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	creator, transport := typedSetupNative(t, nil)
	if err := unix.Close(0); err != nil {
		t.Fatalf("release this isolated child's standard input: %v", err)
	}
	channel, unit, err := pppkernel.DevPPPSetup(creator, transport, "zc-setup-zero")
	typedSetupCleanup(t, channel, unit)
	if err != nil {
		t.Fatal(err)
	}
	if err := channel.WithDescriptor(func(fd int) error {
		if fd != 0 {
			return fmt.Errorf("original attached channel descriptor=%d, want0", fd)
		}
		_, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	typedSetupOriginal(t, channel, unit)
	if err := channel.Close(); err != nil {
		t.Fatal(err)
	}
	if err := channel.WithDescriptor(func(int) error { return nil }); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed descriptor-zero owner still lent authority: %v", err)
	}
	if err := unit.WithDescriptor(func(fd int) error {
		_, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
		return err
	}); err != nil {
		t.Fatalf("closing channel zero released the independent original unit: %v", err)
	}
}

// Missing creator refuses before attachment and preserves the transport owner.
func TestDevPPPSetupTypedNativeRefusalPreservesTransport(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	_, transport := typedSetupNative(t, nil)
	channel, unit, err := pppkernel.DevPPPSetup(nil, transport, "zc-refused")
	if err == nil {
		t.Fatal("missing creator admitted a native setup")
	}
	if channel != nil {
		t.Fatal("refused creator allocated an original channel")
	}
	if unit != nil {
		t.Fatal("refused creator allocated an original unit")
	}
	if err := sourceLease(transport, func(fd int) error { _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); return err }); err != nil {
		t.Fatalf("refused setup closed its caller's original transport: %v", err)
	}
}

// A lost reply after actual creation returns both original receipts and closes
// only their files. The independently owned original transport remains live.
func TestDevPPPSetupTypedNativeLostReplyRetainsCleanup(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	typedSetupFailedCreation(t, false)
}

// A provider panic after actual native creation follows the same custody path.
func TestDevPPPSetupTypedNativeProviderPanicRetainsCleanup(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	typedSetupFailedCreation(t, true)
}

func typedSetupFailedCreation(t *testing.T, panicAfter bool) {
	t.Helper()
	failure := errors.New("native creation reply withheld after actual success")
	var wrapper *typedSetupLostReply
	creator, transport := typedSetupNative(t, func(backend iface.Backend) iface.Backend {
		wrapper = &typedSetupLostReply{Backend: backend, cause: failure, panicAfter: panicAfter}
		return wrapper
	})
	channel, unit, err := pppkernel.DevPPPSetup(creator, transport, "zc-setup-fail")
	typedSetupCleanup(t, channel, unit)
	if !errors.Is(err, failure) {
		t.Fatalf("setup lost original after-create cause: %v", err)
	}
	if channel == nil {
		t.Fatal("failed native creation lost original attached channel")
	}
	if unit == nil {
		t.Fatal("failed native creation lost original unit file")
	}
	if wrapper.created == nil {
		t.Fatal("fixture never obtained an actual native creation echo")
	}
	if wrapper.created.Attrs().Index <= 0 {
		t.Fatal("fixture never created an actual native index")
	}
	if unit.CreationRejected() {
		t.Fatal("lost reply or provider panic became a definitive rejection")
	}
	if err := channel.WithDescriptor(func(int) error { return nil }); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("failed setup did not close original channel: %v", err)
	}
	if err := unit.WithDescriptor(func(int) error { return nil }); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("failed setup did not close original unit: %v", err)
	}
	if err := channel.Close(); err != nil {
		t.Fatalf("repeated original channel close: %v", err)
	}
	if err := unit.Close(); err != nil {
		t.Fatalf("repeated original unit close: %v", err)
	}
	if _, err := netlink.LinkByName("zc-setup-fail"); err == nil {
		t.Fatal("original failed unit file still owns its native device")
	} else {
		if _, absent := errors.AsType[netlink.LinkNotFoundError](err); !absent {
			t.Fatalf("original failed device absence is unconfirmed: %v", err)
		}
	}
	if err := sourceLease(transport, func(fd int) error { _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); return err }); err != nil {
		t.Fatalf("failed setup closed independent original PPPoX transport: %v", err)
	}
}

type typedSetupLostReply struct {
	iface.Backend
	cause      error
	panicAfter bool
	created    netlink.Link
}

func (backend *typedSetupLostReply) CreateLinkCounterReader(request *nl.NetlinkRequest) (netlink.Link, func() (*netlink.LinkStatistics, error), bool, error) {
	source, present := backend.Backend.(interface {
		CreateLinkCounterReader(*nl.NetlinkRequest) (netlink.Link, func() (*netlink.LinkStatistics, error), bool, error)
	})
	if !present {
		return nil, nil, false, errors.New("native fixture backend has no creation-bound reader")
	}
	link, read, attempted, err := source.CreateLinkCounterReader(request)
	if err != nil {
		return link, read, attempted, err
	}
	backend.created = link
	if backend.panicAfter {
		panic(backend.cause)
	}
	return link, read, attempted, backend.cause
}

func typedSetupNative(t *testing.T, wrap func(iface.Backend) iface.Backend) (*pppkernel.UnitCreator, *os.File) {
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
	if wrap != nil {
		backend = wrap(backend)
	}
	creator, err := pppkernel.NewUnitCreator(backend)
	if err != nil {
		t.Fatal(err)
	}
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "zc-setup-access"}}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := netlink.LinkDel(link); err != nil {
			t.Error(err)
		}
	})
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	transport, err := pppoe.PPPoECreate(pppoe.InterfaceBinding{Device: "zc-setup-access"}, 73, [6]byte{2, 0, 0, 0, 0, 73})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := transport.Close(); err != nil {
			t.Error(err)
		}
	})
	return creator, transport
}

func typedSetupCleanup(t *testing.T, channel *pppkernel.CreatedChannel, unit *pppkernel.CreatedUnit) {
	t.Helper()
	t.Cleanup(func() {
		if err := errors.Join(unit.Close(), channel.Close()); err != nil {
			t.Error(err)
		}
	})
}

func typedSetupOriginal(t *testing.T, channel *pppkernel.CreatedChannel, unit *pppkernel.CreatedUnit) {
	t.Helper()
	if channel == nil {
		t.Fatal("successful setup has no original attached channel")
	}
	if unit == nil {
		t.Fatal("successful setup has no original unit")
	}
	if _, err := unit.SessionConfig(channel); err != nil {
		t.Fatalf("actual original channel/unit pair cannot be admitted: %v", err)
	}
	if err := unit.WithDescriptor(func(fd int) error {
		number, err := unix.IoctlGetInt(fd, 0x80047456)
		if err != nil {
			return err
		}
		if number != unit.UnitNumber() {
			return fmt.Errorf("native unit=%d, creation receipt=%d", number, unit.UnitNumber())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	actual, err := netlink.LinkByIndex(int(unit.Identity().NativeIndex))
	if err != nil {
		t.Fatal(err)
	}
	if actual.Type() != "ppp" {
		t.Fatalf("created native device has type %q", actual.Type())
	}
	if actual.Attrs().Name != unit.Identity().InterfaceName {
		t.Fatalf("actual native name=%q, creation echo=%q", actual.Attrs().Name, unit.Identity().InterfaceName)
	}
}
