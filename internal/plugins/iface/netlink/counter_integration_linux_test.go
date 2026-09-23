//go:build integration && linux

package ifacenetlink

import (
	"errors"
	"os"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// TestCounterGenerationKernelIndexReuse exercises the real subscribed snapshot
// socket. A dummy is deleted and recreated at the same ifIndex between reads;
// its replacement sends more bytes, so a decrease-only detector cannot pass.
func TestCounterGenerationKernelIndexReuse(t *testing.T) {
	withRouteNetNS(t, func() {
		// Suppress background IPv6 solicitations in this private namespace,
		// so only the test's raw Ethernet frames advance the dummy counters.
		if err := os.WriteFile("/proc/sys/net/ipv6/conf/default/disable_ipv6", []byte("1"), 0o644); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
		}
		backend := &netlinkBackend{}
		t.Cleanup(func() {
			if err := backend.Close(); err != nil {
				t.Error(err)
			}
		})
		link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "counter0", Index: 77}}
		if err := netlink.LinkAdd(link); err != nil {
			t.Fatal(err)
		}
		if err := netlink.LinkSetUp(link); err != nil {
			t.Fatal(err)
		}
		counterSendFrames(t, 77, 1)
		before, err := backend.GetInterface("counter0")
		if err != nil {
			t.Fatal(err)
		}
		if before.Stats == nil || before.Stats.TxBytes == 0 || before.CounterGeneration == 0 {
			t.Fatalf("initial raw counter snapshot has no traffic or generation: %+v", before)
		}
		if _, err := backend.GetInterface("absent0"); err == nil {
			t.Fatal("missing interface lookup unexpectedly succeeded")
		}
		if err := netlink.LinkSetDown(link); err != nil {
			t.Fatal(err)
		}
		if err := netlink.LinkSetUp(link); err != nil {
			t.Fatal(err)
		}
		stable, err := backend.GetInterface("counter0")
		if err != nil {
			t.Fatal(err)
		}
		if stable.CounterGeneration != before.CounterGeneration {
			t.Fatal("ordinary down/up reset counter continuity")
		}
		if err := netlink.LinkDel(link); err != nil {
			t.Fatal(err)
		}
		if err := netlink.LinkAdd(link); err != nil {
			t.Fatal(err)
		}
		if err := netlink.LinkSetUp(link); err != nil {
			t.Fatal(err)
		}
		counterSendFrames(t, 77, 3)
		after, err := backend.ListInterfaces()
		if err != nil {
			t.Fatal(err)
		}
		for i := range after {
			if after[i].Index != before.Index {
				continue
			}
			if after[i].Stats == nil || after[i].Stats.TxBytes <= before.Stats.TxBytes {
				t.Fatalf("replacement did not exceed the old raw counters: %+v", after[i].Stats)
			}
			if after[i].CounterGeneration == 0 || after[i].CounterGeneration == before.CounterGeneration {
				t.Fatal("replacement with larger counters retained old generation")
			}
			return
		}
		t.Fatal("recreated interface is absent from the raw snapshot")
	})
}

func counterSendFrames(t *testing.T, index, count int) {
	t.Helper()
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			t.Skipf("requires CAP_NET_RAW: %v", err)
		}
		t.Fatal(err)
	}
	defer func() {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
	}()
	frame := [60]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 2, 0, 0, 0, 0, 1, 0x88, 0xb5}
	for range count {
		if err := unix.Sendto(fd, frame[:], 0, &unix.SockaddrLinklayer{Ifindex: index}); err != nil {
			t.Fatal(err)
		}
	}
}
