// Design: docs/architecture/l2tp/bng-5-pppoe.md -- kernel logical access binding

package kernel

import (
	"context"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
)

// TestPrepareKeepsResolvedLinuxDevice observes the shared resolver's input and
// the actual OS identity exported to discovery and session construction.
func TestPrepareKeepsResolvedLinuxDevice(t *testing.T) {
	calls := 0
	access, err := prepareWithResolver(context.Background(), "logical", func() error { return nil },
		func(name string) (iface.Binding, error) {
			calls++
			if name != "logical" {
				t.Errorf("resolver input=%q", name)
			}
			return iface.Binding{OsName: "actual0.10", Ifindex: 42,
				OperMAC: "02:03:04:05:06:07", MTU: 1500}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("resolver calls=%d", calls)
	}
	if access.Device() != "actual0.10" {
		t.Fatalf("session device=%q", access.Device())
	}
	if access.ControlDevice() != "actual0.10" {
		t.Fatalf("control device=%q", access.ControlDevice())
	}
	if access.DiscoveryIfIndex() != 42 {
		t.Fatalf("discovery index=%d", access.DiscoveryIfIndex())
	}
	if access.Namespace() != "" {
		t.Fatalf("namespace=%q", access.Namespace())
	}
	if access.HWAddr() != [6]byte{2, 3, 4, 5, 6, 7} {
		t.Fatalf("MAC=%v", access.HWAddr())
	}
	if access.MTU() != 1500 {
		t.Fatalf("MTU=%d", access.MTU())
	}
	if err := access.Close(); err != nil {
		t.Fatal(err)
	}
	if err := access.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestPrepareRefusesUnreadableBinding checks backend/read errors preserve
// their identity and absent or malformed link facts never become an access.
func TestPrepareRefusesUnreadableBinding(t *testing.T) {
	want := errors.New("lookup refused")
	for _, stage := range []string{"backend", "resolve", "device", "index", "MAC", "EUI64", "MTU", "canceled"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			requireBackend := func() error {
				if stage == "backend" {
					return want
				}
				return nil
			}
			access, err := prepareWithResolver(ctx, "logical", requireBackend, func(string) (iface.Binding, error) {
				binding := iface.Binding{OsName: "eth0", Ifindex: 42, OperMAC: "02:03:04:05:06:07", MTU: 1500}
				switch stage {
				case "resolve":
					return iface.Binding{}, want
				case "device":
					binding.OsName = ""
				case "index":
					binding.Ifindex = 0
				case "MAC":
					binding.OperMAC = "malformed"
				case "EUI64":
					binding.OperMAC = "02:03:04:05:06:07:08:09"
				case "MTU":
					binding.MTU = 0
				case "canceled":
					cancel()
				case "backend":
					t.Fatal("backend refusal reached resolver")
				}
				return binding, nil
			})
			if access != nil {
				t.Fatal("failed binding published")
			}
			if err == nil {
				t.Fatal("invalid binding accepted")
			}
			if stage == "backend" {
				if !errors.Is(err, want) {
					t.Fatalf("backend error lost: %v", err)
				}
			}
			if stage == "resolve" {
				if !errors.Is(err, want) {
					t.Fatalf("lookup error lost: %v", err)
				}
			}
			if stage == "canceled" {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation lost: %v", err)
				}
			}
		})
	}
}
