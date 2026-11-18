// Design: docs/research/l2tpv2-ze-integration.md -- production original cleanup
// Related: kernel_ppp_owners_linux.go -- original owner dispatch and traversal

//go:build integration && linux

package l2tp

import (
	"errors"
	"os"
	"testing"

	"github.com/vishvananda/netlink"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestKernelOriginalCleanupClosesGenuineOwners drives the actual producer
// with DevPPPSetup's sealed owners. The transport callback checks both actual
// original files were closed first; their real original interface disappears.
// MUTATION: omit a typed owner or dispatch transport first in the production
// cleanup producer. Original-owner observations here MUST fail.
func TestKernelOriginalCleanupClosesGenuineOwners(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend := kernelCounterNativeBackend(t)
	creator, err := pppkernel.NewUnitCreator(backend)
	if err != nil {
		t.Fatal(err)
	}
	transport, release := kernelCounterTransportWithRelease(t)
	channel, unit, err := pppkernel.DevPPPSetup(creator, transport, "zc-close")
	if channel != nil {
		t.Cleanup(func() {
			if err := channel.Close(); err != nil {
				t.Error(err)
			}
		})
	}
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
	config, err := unit.SessionConfig(channel)
	if err != nil {
		t.Fatal(err)
	}
	identity := unit.Identity()
	if err := unit.WithDescriptor(func(fd int) error {
		if fd != config.UnitFD {
			t.Errorf("SessionConfig lost original unit descriptor: %d != %d", fd, config.UnitFD)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := channel.WithDescriptor(func(fd int) error {
		if fd != config.ChanFD {
			t.Errorf("SessionConfig lost original channel descriptor: %d != %d", fd, config.ChanFD)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if identity.NativeIndex == 0 {
		t.Fatal("genuine original unit has no native identity")
	}
	if _, err := unit.Counters(); err != nil {
		t.Fatalf("genuine original was not live before producer cleanup: %v", err)
	}
	transportCloses := 0
	owners := &kernelPPPOwners{unit: unit, channel: channel}
	failures := closeOriginalKernelPPP(owners, transport, func(original *os.File) error {
		transportCloses++
		if original != transport {
			t.Errorf("producer changed original transport: %p", original)
		}
		if err := unit.WithDescriptor(func(int) error { return nil }); !errors.Is(err, os.ErrClosed) {
			t.Errorf("unit remains open when transport closes: %v", err)
		}
		if err := channel.WithDescriptor(func(int) error { return nil }); !errors.Is(err, os.ErrClosed) {
			t.Errorf("channel remains open when transport closes: %v", err)
		}
		return release()
	})
	for _, failure := range failures {
		if failure != nil {
			t.Fatal(failure)
		}
	}
	if transportCloses != 1 {
		t.Fatalf("original transport closes=%d", transportCloses)
	}
	_, linkErr := netlink.LinkByIndex(int(identity.NativeIndex))
	var missing netlink.LinkNotFoundError
	if !errors.As(linkErr, &missing) {
		t.Fatalf("production cleanup did not confirm original PPP unit absence: %v", linkErr)
	}
	if _, err := unit.Counters(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed original unit remained measurable: %v", err)
	}
	// Re-observing typed Close is safe; the consumed transport label is absent.
	// A raw native close retry would turn these cached outcomes into EBADF.
	again := closeOriginalKernelPPP(owners, nil, func(*os.File) error {
		t.Error("producer retried a consumed transport")
		return nil
	})
	for _, failure := range again {
		if failure != nil {
			t.Fatalf("original typed close was not cached: %v", failure)
		}
	}
}
