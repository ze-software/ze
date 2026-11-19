//go:build integration && linux

// Design: docs/architecture/l2tp/bng-5-pppoe.md -- prepared access backend ownership
// Related: prepare.go -- real parent acquisition and captured session construction

package kernel

import (
	"errors"
	"net"
	"os"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestPreparedKernelAccessSurvivesActiveBackendReplacement opens real children
// before and after replacing the active kind with a non-native trap. A global
// activeBackend lookup must refuse the second admission, while the original
// prepared backend's real counter socket remains useful. This proves parent
// lifetime and bound admission, not creation-bound child counter readers.
func TestPreparedKernelAccessSurvivesActiveBackendReplacement(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newPreparedNativeAccess(t)
	first := f.open(t, 71)
	f.useChild(t, first, 1400)
	active := iface.GetBackend()
	f.readAccess(t, active)

	const trapName = "prepared-owner-trap"
	trapOpens := 0
	if err := iface.RegisterBackend(trapName, func() (iface.Backend, error) {
		trapOpens++
		return &preparedNativeTrapBackend{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := iface.LoadBackend(trapName); err != nil {
		t.Fatal(err)
	}
	if trapOpens != 1 {
		t.Fatalf("trap acquisitions=%d, want 1", trapOpens)
	}
	if iface.ActiveBackendName() != trapName {
		t.Fatal("active backend kind was not replaced")
	}
	if _, err := iface.RequireActiveBackend(Name); err == nil {
		t.Fatal("replacement did not make a global netlink selection refuse")
	}
	if _, err := active.GetInterface(f.access.ControlDevice()); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("replaced active backend did not close its original socket: %v", err)
	}
	f.readAccess(t, f.backend)
	second := f.open(t, 72)
	f.distinctChildren(t, first, second)
	f.useChild(t, second, 1390)
	f.useChild(t, first, 1380)
	if f.acquisitions != 1 {
		t.Fatalf("prepared backend acquisitions=%d, want 1 for both children", f.acquisitions)
	}
}

// TestPreparedKernelAccessKeepsNativeSurvivorAfterChildClose ends both roles of
// one real PPPoX/PPP child and requires the other original unit and its parent
// socket to remain useful. No fake session or interface backend supplies them.
func TestPreparedKernelAccessKeepsNativeSurvivorAfterChildClose(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newPreparedNativeAccess(t)
	first, second := f.open(t, 73), f.open(t, 74)
	f.distinctChildren(t, first, second)
	f.useChild(t, first, 1400)
	f.useChild(t, second, 1390)
	firstIdentity := first.Dataplane().Identity()
	endPreparedNativeChild(t, first)
	preparedNativeUnitAbsent(t, firstIdentity)
	f.readAccess(t, f.backend)
	f.useChild(t, second, 1380)
	if f.acquisitions != 1 {
		t.Fatalf("prepared backend acquisitions=%d, want 1 for both children", f.acquisitions)
	}
}

// TestPreparedKernelAccessRetainsPendingNativeChild refuses the existing native
// release seal of a real Access.Open result. The original unit must retain its
// confirmed birth while its transport ends, its sibling and parent stay useful,
// and retrying that same owner after confirmation releases the original unit.
// This does not exercise the parent package's Subsystem.Stop/Reload decision.
func TestPreparedKernelAccessRetainsPendingNativeChild(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newPreparedNativeAccess(t)
	first, second := f.open(t, 75), f.open(t, 76)
	f.distinctChildren(t, first, second)
	f.useChild(t, first, 1400)
	f.useChild(t, second, 1390)
	identity := first.Dataplane().Identity()
	release, ok := first.Dataplane().(ppp.NativeReleaseHandle)
	if !ok {
		t.Fatal("real kernel child has no native release capability")
	}
	cause := errors.New("original native cleanup remains unconfirmed")
	seal := &preparedNativeReleaseSeal{tag: "ze-prepared-access-pending", failure: cause}
	// Unblock the original release before child/parent cleanup on every exit.
	t.Cleanup(func() { seal.failure = nil })
	if err := release.BindNativeReleaseSeal(seal); err != nil {
		t.Fatal(err)
	}
	if !release.NativeReleaseBound() {
		t.Fatal("binding did not retain the original native unit")
	}
	if err := first.Dataplane().Close(); !errors.Is(err, cause) {
		t.Fatalf("unconfirmed native Close lost its original refusal: %v", err)
	}
	if !release.NativeReleasePending() {
		t.Fatal("failed child departure lost its original native owner")
	}
	if release.NativeReleaseBirthTag() != seal.tag {
		t.Fatal("pending release changed its confirmed birth receipt")
	}
	original, err := netlink.LinkByIndex(int(identity.NativeIndex))
	if err != nil {
		t.Fatalf("pending release lost its real PPP unit: %v", err)
	}
	if original.Attrs().Name != identity.InterfaceName {
		t.Fatal("pending release no longer identifies the original unit")
	}
	if original.Attrs().Alias != seal.tag {
		t.Fatal("pending release no longer carries its confirmed native birth")
	}
	if err := first.CloseTransport(); err != nil {
		t.Fatal(err)
	}
	f.readAccess(t, f.backend)
	f.useChild(t, second, 1380)
	seal.failure = nil
	if err := first.Dataplane().Close(); err != nil {
		t.Fatalf("confirmed original cleanup did not release its native owner: %v", err)
	}
	if release.NativeReleasePending() {
		t.Fatal("confirmed cleanup retained a pending native owner")
	}
	preparedNativeUnitAbsent(t, identity)
}

// TestPreparedKernelAccessFinalCloseReleasesBackend ends both roles of both real
// children before closing their parent. Only the captured backend's socket must
// refuse afterward; the active backend and configured access device survive.
func TestPreparedKernelAccessFinalCloseReleasesBackend(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newPreparedNativeAccess(t)
	first, second := f.open(t, 77), f.open(t, 78)
	f.distinctChildren(t, first, second)
	f.useChild(t, first, 1400)
	f.useChild(t, second, 1390)
	endPreparedNativeChild(t, first)
	endPreparedNativeChild(t, second)
	preparedNativeUnitAbsent(t, first.Dataplane().Identity())
	preparedNativeUnitAbsent(t, second.Dataplane().Identity())
	f.readAccess(t, f.backend)
	if err := f.access.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := f.backend.GetInterface(f.access.ControlDevice()); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("final parent Close did not seal its native socket: %v", err)
	}
	f.readAccess(t, iface.GetBackend())
	opened, err := f.access.Open(t.Context(), f.request(79))
	if opened != nil {
		t.Cleanup(func() {
			if err := opened.Dataplane().Close(); err != nil {
				t.Error(err)
			}
			if err := opened.CloseTransport(); err != nil {
				t.Error(err)
			}
		})
		t.Fatalf("closed parent admitted another child: %v", opened)
	}
	if err == nil {
		t.Fatal("closed parent returned no admission refusal")
	}
}

// preparedNativeAccess owns an actual independent netlink backend and Access.
// Each child cleanup is registered after the parent, so both child roles finish
// before parent Close. Each root runs alone in userns.Enter's private process.
// Not safe for concurrent use; acquisitions records synchronous preparation.
type preparedNativeAccess struct {
	access       dataplane.Access
	backend      iface.Backend
	acquisitions int
}

func newPreparedNativeAccess(t *testing.T) *preparedNativeAccess {
	t.Helper()
	probe, err := unix.Open("/dev/ppp", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			t.Skipf("native PPP prerequisite /dev/ppp is absent: %v", err)
		}
		if errors.Is(err, unix.ENODEV) {
			t.Skipf("native PPP prerequisite /dev/ppp has no driver: %v", err)
		}
		if errors.Is(err, unix.EACCES) {
			t.Skipf("native PPP prerequisite /dev/ppp read-write is denied: %v", err)
		}
		if errors.Is(err, unix.EPERM) {
			t.Skipf("native PPP prerequisite /dev/ppp capability is unavailable: %v", err)
		}
		t.Fatal(err)
	}
	if err := unix.Close(probe); err != nil {
		t.Fatal(err)
	}
	device := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{
		Name: "ze-pa-access", HardwareAddr: net.HardwareAddr{2, 0, 0, 0, 0, 1}, MTU: 1500,
	}}
	if err := netlink.LinkAdd(device); err != nil {
		if errors.Is(err, unix.EPERM) {
			t.Skip("CAP_NET_ADMIN is required for the private native access device")
		}
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := netlink.LinkDel(device); err != nil {
			t.Error(err)
		}
	})
	if err := netlink.LinkSetUp(device); err != nil {
		t.Fatal(err)
	}
	if iface.GetBackend() != nil {
		t.Fatal("isolated native test unexpectedly inherited an active backend")
	}
	if err := iface.LoadBackend(Name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := iface.CloseBackend(); err != nil {
			t.Error(err)
		}
	})
	f := &preparedNativeAccess{}
	f.access, err = prepareWithResolver(t.Context(), "prepared-access", func() (iface.Backend, error) {
		f.acquisitions++
		f.backend, err = iface.OpenBackend(Name)
		return f.backend, err
	}, func(logical string) (iface.Binding, error) {
		if logical != "prepared-access" {
			t.Fatalf("resolver logical access=%q", logical)
		}
		actual, err := netlink.LinkByIndex(device.Attrs().Index)
		if err != nil {
			return iface.Binding{}, err
		}
		attrs := actual.Attrs()
		return iface.Binding{OsName: attrs.Name, Ifindex: attrs.Index,
			OperMAC: attrs.HardwareAddr.String(), MTU: attrs.MTU}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.access.Close(); err != nil {
			t.Error(err)
		}
	})
	if f.backend == iface.GetBackend() {
		t.Fatal("preparation borrowed the active backend instead of its independent owner")
	}
	f.readAccess(t, f.backend)
	return f
}

func (f *preparedNativeAccess) request(id uint16) dataplane.Request {
	return dataplane.Request{AccessName: "prepared-access", Device: f.access.Device(),
		ControlDevice: f.access.ControlDevice(), Namespace: f.access.Namespace(),
		SessionID: id, PeerMAC: [6]byte{2, 0, 0, 0, 0, 42}, CaptureProxyLCP: true}
}

func (f *preparedNativeAccess) open(t *testing.T, id uint16) dataplane.OpenedSession {
	t.Helper()
	opened, err := f.access.Open(t.Context(), f.request(id))
	if err != nil {
		if errors.Is(err, unix.EPROTONOSUPPORT) {
			t.Skipf("kernel PPPoE socket prerequisite is unavailable: %v", err)
		}
		if errors.Is(err, unix.EAFNOSUPPORT) {
			t.Skipf("kernel PPPoX socket prerequisite is unavailable: %v", err)
		}
		t.Fatalf("open real child %d through original prepared owner: %v", id, err)
	}
	t.Cleanup(func() {
		if err := opened.Dataplane().Close(); err != nil {
			t.Error(err)
		}
		if err := opened.CloseTransport(); err != nil {
			t.Error(err)
		}
	})
	return opened
}

func (f *preparedNativeAccess) readAccess(t *testing.T, backend iface.Backend) {
	t.Helper()
	info, err := backend.GetInterface(f.access.ControlDevice())
	if err != nil {
		t.Fatalf("read original access through its real backend socket: %v", err)
	}
	if info == nil {
		t.Fatal("native socket returned no access snapshot")
	}
	if info.Name != f.access.ControlDevice() {
		t.Fatalf("native access name=%q, want %q", info.Name, f.access.ControlDevice())
	}
	if info.Index != f.access.DiscoveryIfIndex() {
		t.Fatalf("native access index=%d, want %d", info.Index, f.access.DiscoveryIfIndex())
	}
}

func (*preparedNativeAccess) useChild(t *testing.T, opened dataplane.OpenedSession, mtu int) {
	t.Helper()
	if err := opened.WithDescriptor(func(fd int) error {
		domain, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_DOMAIN)
		if err != nil {
			return err
		}
		if domain != unix.AF_PPPOX {
			t.Fatalf("transport domain=%d, want actual AF_PPPOX", domain)
		}
		return nil
	}); err != nil {
		t.Fatalf("borrow original real PPPoX transport: %v", err)
	}
	handle := opened.Dataplane()
	if err := handle.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := handle.SetReceiveMRU(1492); err != nil {
		t.Fatal(err)
	}
	if err := handle.SetMTU(mtu); err != nil {
		t.Fatal(err)
	}
	identity := handle.Identity()
	if identity.NativeIndex == 0 {
		t.Fatal("real PPP child has no native index")
	}
	link, err := netlink.LinkByIndex(int(identity.NativeIndex))
	if err != nil {
		t.Fatalf("read original real PPP unit: %v", err)
	}
	if link.Type() != "ppp" {
		t.Fatalf("native child kind=%q, want ppp", link.Type())
	}
	if link.Attrs().Name != identity.InterfaceName {
		t.Fatal("native PPP unit differs from its original identity")
	}
	if link.Attrs().MTU != mtu {
		t.Fatalf("native PPP MTU=%d, want installed %d", link.Attrs().MTU, mtu)
	}
}

func (*preparedNativeAccess) distinctChildren(t *testing.T, first, second dataplane.OpenedSession) {
	t.Helper()
	if first.Dataplane().Identity().NativeIndex == second.Dataplane().Identity().NativeIndex {
		t.Fatal("two admitted native children share one PPP unit")
	}
}

func endPreparedNativeChild(t *testing.T, opened dataplane.OpenedSession) {
	t.Helper()
	if err := opened.Dataplane().Close(); err != nil {
		t.Fatal(err)
	}
	if err := opened.CloseTransport(); err != nil {
		t.Fatal(err)
	}
	called := false
	err := opened.WithDescriptor(func(int) error { called = true; return nil })
	if err == nil {
		t.Fatal("closed real transport still lends its descriptor")
	}
	if called {
		t.Fatal("closed real transport invoked a descriptor borrower")
	}
}

func preparedNativeUnitAbsent(t *testing.T, identity ppp.SessionIdentity) {
	t.Helper()
	link, err := netlink.LinkByIndex(int(identity.NativeIndex))
	if err == nil {
		t.Fatalf("released original PPP unit remains: %+v", link.Attrs())
	}
	var absent netlink.LinkNotFoundError
	if !errors.As(err, &absent) {
		t.Fatalf("native unit absence was not confirmed: %v", err)
	}
}

// The trap is only the replacement active kind, never a native parent or child.
// Its inherited methods cannot supply native operations; only Close is usable.
type preparedNativeTrapBackend struct{ iface.Backend }

func (*preparedNativeTrapBackend) Close() error { return nil }

// This refusal uses the existing real session's native-release boundary. It is
// not a fake session/backend. Not safe for concurrent use; this test is serial.
type preparedNativeReleaseSeal struct {
	tag     string
	failure error
}

func (s *preparedNativeReleaseSeal) NativeBirthTag() string   { return s.tag }
func (s *preparedNativeReleaseSeal) SealNativeRelease() error { return s.failure }
