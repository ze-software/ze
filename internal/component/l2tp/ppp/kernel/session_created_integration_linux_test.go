//go:build integration && linux

// Design: docs/research/l2tpv2-ze-integration.md -- genuine original producer→session
// Real kernel PPPoE socket/channel and real creation-echo PPP unit. No wire
// negotiation, forwarding, remote policy or full transport-pipeline claim.

package kernel_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/test/userns"
)

func createdSessionChannel(t *testing.T, name string, sid uint16) *pppkernel.CreatedChannel {
	t.Helper()
	cleanup, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	link := &netlink.Dummy{Name: name}
	var owned bool
	t.Cleanup(func() {
		defer cleanup.Close()
		if owned {
			if err := cleanup.LinkDel(link); err != nil {
				t.Error(err)
			}
		}
	})
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatal(err)
	}
	owned = true
	actual, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatal(err)
	}
	link.Index = actual.Attrs().Index
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	fd, err := unix.Socket(unix.AF_PPPOX, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("real PPPoE transport socket: %v", err)
	}
	file := os.NewFile(uintptr(fd), "ppp.supplier.original.transport")
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := unix.Connect(fd, &unix.SockaddrPPPoE{SID: sid, Remote: []byte{2, 0, 0, 0, 0, 1}, Dev: name}); err != nil {
		t.Fatalf("connect real PPPoE transport: %v", err)
	}
	channel, err := pppkernel.CreatePPPChannel(file)
	if channel != nil {
		t.Cleanup(func() {
			if err := channel.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if err != nil {
		t.Fatalf("capture actual attached channel: %v", err)
	}
	if channel == nil {
		t.Fatal("actual channel producer returned no owner")
	}
	return channel
}

func createdSessionOpen(t *testing.T, cfg pppkernel.SessionConfig) ppp.SessionHandle {
	t.Helper()
	frames, handle, err := pppkernel.NewSession(cfg)
	if handle != nil {
		t.Cleanup(func() {
			if err := handle.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	if frames == nil || handle == nil {
		t.Fatal("actual constructor returned no session")
	}
	return handle
}

func createdSessionZero(t *testing.T, owner ppp.SessionHandle) {
	t.Helper()
	got, err := owner.Counters()
	if err != nil {
		t.Fatalf("read actual original counters: %v", err)
	}
	if got.RxBytes != 0 || got.TxBytes != 0 || got.RxPackets != 0 || got.TxPackets != 0 {
		t.Fatalf("original idle traffic %+v", got)
	}
	for _, reading := range []ppp.CounterReading{got.RxErrors, got.TxErrors, got.RxDropped, got.TxDropped} {
		if value, err := reading.Value(); err != nil || value != 0 {
			t.Fatalf("original diagnostic %d, %v", value, err)
		}
	}
}

func TestCreatedSessionOriginalCounterOwnerSurvivesRename(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _, link := createdUnitNative(t, "zc-native")
	channel := createdSessionChannel(t, "zc-access", 17)
	cfg, err := unit.SessionConfig(channel)
	if err != nil {
		t.Fatal(err)
	}
	owner := createdSessionOpen(t, cfg)
	if owner.Identity() != unit.Identity() {
		t.Fatalf("actual session identity %+v differs from echo %+v", owner.Identity(), unit.Identity())
	}
	createdSessionZero(t, owner)
	if err := netlink.LinkSetName(link, "zc-renamed"); err != nil {
		t.Fatal(err)
	}
	createdUnitTraffic(t, 0)
	createdSessionZero(t, owner)
	createdUnitForeignNamespace(t, func() {
		createdUnitTraffic(t, link.Attrs().Index)
		createdSessionZero(t, owner)
	})
}

func TestCreatedSessionForeignChannelCannotPairWithOriginalUnit(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _, _ := createdUnitNative(t, "zc-native")
	original := createdSessionChannel(t, "zc-origin", 17)
	cfg, err := unit.SessionConfig(original)
	if err != nil {
		t.Fatal(err)
	}
	var foreign *pppkernel.CreatedChannel
	var foreignConfig pppkernel.SessionConfig
	createdUnitForeignNamespace(t, func() {
		foreignUnit, _, _ := createdUnitNative(t, "zc-native")
		foreign = createdSessionChannel(t, "zc-foreign", 18)
		var err error
		foreignConfig, err = foreignUnit.SessionConfig(foreign)
		if err != nil {
			t.Fatal(err)
		}
		if foreignUnit.UnitNumber() != unit.UnitNumber() {
			t.Fatalf("foreign unit number %d differs from original %d", foreignUnit.UnitNumber(), unit.UnitNumber())
		}
	})
	if _, err := unit.SessionConfig(foreign); err == nil || !strings.Contains(err.Error(), "different namespaces") {
		t.Fatalf("cross-namespace snapshot: %v", err)
	}
	cfg.Channel, cfg.ChanFD = foreign, foreignConfig.ChanFD
	frames, handle, err := pppkernel.NewSession(cfg)
	if handle != nil {
		t.Cleanup(func() {
			if err := handle.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if err == nil || !strings.Contains(err.Error(), "different namespaces") || frames != nil || handle != nil {
		t.Fatalf("cross-namespace session admission: %v", err)
	}
	if err := foreign.WithDescriptor(func(int) error { return nil }); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("foreign channel rollback %v", err)
	}
	if _, err := unit.Counters(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("original unit rollback %v", err)
	}
	if err := original.WithDescriptor(func(int) error { return nil }); err != nil {
		t.Fatalf("unsubmitted original channel was closed: %v", err)
	}
}

func TestCreatedSessionReplayedUnitClosesFreshRealChannel(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _, _ := createdUnitNative(t, "zc-native")
	original := createdSessionChannel(t, "zc-first", 17)
	cfg, err := unit.SessionConfig(original)
	if err != nil {
		t.Fatal(err)
	}
	owner := createdSessionOpen(t, cfg)
	fresh := createdSessionChannel(t, "zc-second", 18)
	bad := cfg
	bad.Channel = fresh
	_, extra, replayErr := pppkernel.NewSession(bad)
	if extra != nil {
		t.Cleanup(func() {
			if err := extra.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if replayErr == nil || extra != nil {
		t.Fatalf("replayed unit admitted: %v", replayErr)
	}
	if err := fresh.WithDescriptor(func(int) error { return nil }); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("fresh real channel not closed: %v", err)
	}
	createdSessionZero(t, owner)
	if err := original.WithDescriptor(func(int) error { return nil }); err != nil {
		t.Fatalf("replay closed original channel: %v", err)
	}
}

func TestCreatedSessionDeadOriginalRefusesPublication(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, _, link := createdUnitNative(t, "zc-native")
	channel := createdSessionChannel(t, "zc-access", 17)
	cfg, err := unit.SessionConfig(channel)
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkDel(link); err != nil {
		t.Fatal(err)
	}
	if err := unit.WithDescriptor(func(fd int) error {
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		if _, err := unix.Poll(fds, 3000); err != nil {
			return err
		}
		if fds[0].Revents&unix.POLLHUP == 0 {
			return errors.New("deleted original lacks actual POLLHUP")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	frames, handle, err := pppkernel.NewSession(cfg)
	if handle != nil {
		t.Cleanup(func() {
			if err := handle.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if err == nil || frames != nil || handle != nil {
		t.Fatalf("dead actual original admitted: %v", err)
	}
	if err := channel.WithDescriptor(func(int) error { return nil }); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("fresh channel rollback %v", err)
	}
}
