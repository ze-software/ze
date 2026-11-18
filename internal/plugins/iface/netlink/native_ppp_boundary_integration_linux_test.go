// Design: docs/features/interfaces.md -- original PPP authority at actual send
// Related: native_ppp_integration_linux_test.go -- real original-file fixtures

//go:build integration && linux

package ifacenetlink

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestNativePPPProviderReplacementAtActualSend retires the real original
// after the provider's positive probe and replaces its native index with another
// genuine PPP unit immediately before Sendto. The survivor's creator can write;
// only the original-file kernel guard can keep the stale writer from doing so.
func TestNativePPPProviderReplacementAtActualSend(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	fixture := nativePPPProviderFixtureNew(t)
	backend := &netlinkBackend{}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Errorf("close replacement backend: %v", err)
		}
	})
	bound, err := backend.BindNativePPPUnit(fixture, fixture.link.Attrs().Index, fixture.unit)
	if err != nil {
		t.Fatal(err)
	}
	writer := bound.(*nativePPPWriter)
	t.Cleanup(func() {
		if err := writer.Close(); err != nil {
			t.Errorf("close replacement writer: %v", err)
		}
	})
	var survivor netlink.Link
	writer.beforeSend = func() {
		if !fixture.inBorrow.Load() {
			t.Fatal("actual send boundary has no original file borrow")
		}
		if err := netlink.LinkDel(fixture.link); err != nil {
			t.Fatal(err)
		}
		replacement := nativePPPProviderReplacementNew(t, fixture.link.Attrs().Index)
		survivor = replacement.link
		useful, err := backend.BindNativePPPUnit(replacement, survivor.Attrs().Index, replacement.unit)
		if err != nil {
			t.Fatalf("bind actual replacement creator after index reuse: %v", err)
		}
		t.Cleanup(func() {
			if err := useful.Close(); err != nil {
				t.Errorf("close actual replacement writer: %v", err)
			}
		})
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := useful.SetMTU(ctx, 1703); err != nil {
			t.Fatalf("replacement creator cannot perform a useful guarded write: %v", err)
		}
		nativePPPProviderRequireLink(t, survivor.Attrs().Index, 1703, "", false)
	}
	writer.beforeRead = func() {
		if fixture.inBorrow.Load() {
			t.Fatal("ACK wait retained original unit descriptor borrow")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = writer.SetMTU(ctx, 1433)
	if !errors.Is(err, unix.ESTALE) {
		t.Fatalf("actual stale original write lost native ESTALE: %v", err)
	}
	var rejection interface{ NativePPPWriteRejected() bool }
	if !errors.As(err, &rejection) {
		t.Fatalf("full native NACK lacks a definitive original-request receipt: %v", err)
	}
	if errors.Is(err, iface.ErrNativePPPPending) {
		t.Fatal("complete matched native rejection remained uncertain")
	}
	nativePPPProviderRequireLink(t, survivor.Attrs().Index, 1703, "", false)
}

// nativePPPProviderReplacementNew creates through a fresh unbound original file
// so PPP's native validate_owner callback is present at the reused index. A
// Dummy would stop earlier with EOPNOTSUPP and never exercise stale PPP custody.
func nativePPPProviderReplacementNew(t *testing.T, requestedIndex int) *nativePPPProviderFixture {
	t.Helper()
	file, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open fresh replacement PPP creator: %v", err)
	}
	replacement := &nativePPPProviderFixture{file: file, unit: -1}
	// Closing this exact creator releases its unit even if creation's ACK or
	// the following readback fails; no guessed unit number authorizes cleanup.
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("close actual replacement PPP creator: %v", err)
		}
	})
	if err := replacement.WithDescriptor(func(fd int) error {
		request := nl.NewNetlinkRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
		message := nl.NewIfInfomsg(unix.AF_UNSPEC)
		message.Index = int32(requestedIndex)
		request.AddData(message)
		request.AddData(nl.NewRtAttr(unix.IFLA_IFNAME, nl.ZeroTerminated("zpppsurvivor")))
		info := nl.NewRtAttr(unix.IFLA_LINKINFO, nil)
		info.AddRtAttr(nl.IFLA_INFO_KIND, nl.NonZeroTerminated("ppp"))
		data := info.AddRtAttr(nl.IFLA_INFO_DATA, nil)
		data.AddRtAttr(unix.IFLA_PPP_DEV_FD, nl.Uint32Attr(uint32(fd)))
		request.AddData(info)
		if _, err := request.Execute(unix.NETLINK_ROUTE, 0); err != nil {
			return err
		}
		number, err := unix.IoctlGetInt(fd, 0x80047456) // Actual PPPIOCGUNIT after native creation.
		if err != nil {
			return err
		}
		if number < 0 {
			return errors.New("native replacement creator returned a negative unit number")
		}
		replacement.unit = int32(number)
		return nil
	}); err != nil {
		t.Fatalf("create genuine PPP replacement at original index: %v", err)
	}
	link, err := netlink.LinkByName("zpppsurvivor")
	if err != nil {
		t.Fatal(err)
	}
	if link.Type() != "ppp" {
		t.Fatalf("native replacement type=%s, want ppp", link.Type())
	}
	if link.Attrs().Index != requestedIndex {
		t.Fatal("native replacement did not reuse captured index")
	}
	replacement.link = link
	return replacement
}
