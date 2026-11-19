// Design: docs/research/vpp-deployment-reference.md -- startup configuration transactions
// Related: startup_lifecycle.go -- retained original manager cleanup before replacement

package vpp

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"testing/synctest"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// Apply must retain the original binder after its real restoration boundary
// refuses, and retry that receipt before any fresh manager can be admitted.
func TestVPPStartupCleanupRetainsOriginalBinderBeforeReplacement(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newDPDKCleanupFixture(t)
		fixture.bind(t)
		want := errors.New("original VFIO restoration refused")
		fixture.unbindFailure = want
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var starts int
		var allowReplacement bool
		var original *VPPManager
		runtime := newVPPLifecycle(ctx, func(ctx context.Context, settings *VPPSettings, _ bool, ready chan<- error) error {
			starts++
			manager := newCleanupOutcomeManager(t)
			if starts == 1 {
				original = manager
				manager.dpdk = fixture.binder
			} else if !allowReplacement {
				return errors.New("fresh manager entered before original cleanup completed")
			}
			return manager.runWithOps(ctx, managerRunOps{
				bindAll: manager.dpdk.BindAll,
				runOnce: func(ctx context.Context, _ string) error {
					ready <- nil
					<-ctx.Done()
					return ctx.Err()
				},
				unbindAll: func() error {
					if manager == original {
						return fixture.binder.unbindAllWithOps(fixture.ops)
					}
					return manager.dpdk.UnbindAll()
				},
			})
		})
		tx := newVPPConfigTx(runtime)
		if err := tx.Configure(startupLifecycleSections("first")); err != nil {
			t.Fatal(err)
		}
		if err := runtime.Start(); err != nil {
			t.Fatal(err)
		}
		if err := tx.Verify(startupLifecycleSections("second")); err != nil {
			t.Fatal(err)
		}
		failure := tx.Apply()
		if !errors.Is(failure, want) {
			t.Fatalf("apply lost original cleanup failure: %v", failure)
		}
		if starts != 1 {
			t.Fatalf("pending original cleanup admitted %d managers, want only the original", starts)
		}
		if !errors.Is(failure, sdk.ErrRunningConfigLost) {
			t.Fatalf("failed original cleanup did not retain the broken transaction: %v", failure)
		}
		if original == nil || original.dpdk != fixture.binder {
			t.Fatal("cleanup no longer belongs to the original manager and binder")
		}
		if runtime.active == nil {
			t.Fatal("failed original cleanup lost its lifecycle owner")
		}
		if runtime.active.cleanup == nil || runtime.active.cleanup.receipt == nil || runtime.active.cleanup.receipt.manager != original {
			t.Fatal("terminal cleanup receipt lost the exact original manager")
		}
		if err := runtime.Stop(); !errors.Is(err, want) {
			t.Fatalf("same-owner cleanup retry lost its unresolved cause: %v", err)
		}
		if starts != 1 || fixture.unbindCalls != 2 {
			t.Fatalf("retry started another manager or skipped exact cleanup: managers=%d cleanup=%d", starts, fixture.unbindCalls)
		}
		fixture.unbindFailure = nil
		if err := runtime.Stop(); err != nil {
			t.Fatalf("confirmed original cleanup retained only historical failure: %v", err)
		}
		fixture.restored(t)
		if runtime.active != nil {
			t.Fatal("confirmed cleanup retained the terminal manager")
		}
		allowReplacement = true
		if err := tx.Rollback(); err != nil {
			t.Fatalf("original configuration could not restart after exact cleanup: %v", err)
		}
		if starts != 2 {
			t.Fatalf("repaired rollback starts=%d, want one original and one restored manager", starts)
		}
		if err := runtime.Stop(); err != nil {
			t.Fatal(err)
		}
	})
}

// A returned ordinary history error must still permit restoration after the
// same original binder has actually completed its native restoration.
func TestVPPStartupCleanupHistoryDoesNotBlockConfirmedRestoration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newDPDKCleanupFixture(t)
		fixture.bind(t)
		want := errors.New("reaped original process exited unsuccessfully")
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var starts int
		runtime := newVPPLifecycle(ctx, func(ctx context.Context, settings *VPPSettings, _ bool, ready chan<- error) error {
			starts++
			current := starts
			manager := newCleanupOutcomeManager(t)
			if current == 1 {
				manager.dpdk = fixture.binder
			}
			return manager.runWithOps(ctx, managerRunOps{
				bindAll: manager.dpdk.BindAll,
				runOnce: func(ctx context.Context, _ string) error {
					ready <- nil
					<-ctx.Done()
					if current == 1 {
						return want
					}
					return ctx.Err()
				},
				unbindAll: func() error {
					if current == 1 {
						return fixture.binder.unbindAllWithOps(fixture.ops)
					}
					return manager.dpdk.UnbindAll()
				},
			})
		})
		tx := newVPPConfigTx(runtime)
		if err := tx.Configure(startupLifecycleSections("first")); err != nil {
			t.Fatal(err)
		}
		if err := runtime.Start(); err != nil {
			t.Fatal(err)
		}
		if err := tx.Verify(startupLifecycleSections("second")); err != nil {
			t.Fatal(err)
		}
		if err := tx.Apply(); !errors.Is(err, want) || errors.Is(err, sdk.ErrRunningConfigLost) {
			t.Fatalf("confirmed cleanup did not restore the original configuration: %v", err)
		}
		if starts != 2 {
			t.Fatalf("ordinary history restoration starts=%d, want two", starts)
		}
		fixture.restored(t)
		if err := runtime.Stop(); err != nil {
			t.Fatal(err)
		}
	})
}

// A failed later bind reaches BindAll's actual rollback with an original
// receipt still owned. Initial startup must retain it before any child starts.
func TestVPPStartupCleanupRetainsPartialInitialBind(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cpuRoot := t.TempDir()
		writeFixture(t, filepath.Join(cpuRoot, "online"), "0-3\n")
		writeFixture(t, filepath.Join(cpuRoot, "isolated"), "\n")
		setCPURoot(t, cpuRoot)
		fixture := newDPDKCleanupFixture(t)
		bindFailure := errors.New("second configured PCI driver read refused")
		restoreFailure := errors.New("initial rollback VFIO detach refused")
		fixture.unbindFailure = restoreFailure
		originalDriver := fixture.ops.currentDriver
		fixture.ops.currentDriver = func(pci string) (string, error) {
			if pci == dpdkCleanupPeerPCI {
				return "", bindFailure
			}
			return originalDriver(pci)
		}
		first := newCleanupOutcomeManager(t)
		first.settings.External = false
		first.settings.DPDK.Interfaces = []DPDKInterface{
			{PCIAddress: dpdkCleanupPCI, Name: "dpdk0"}, {PCIAddress: dpdkCleanupPeerPCI, Name: "dpdk1"},
		}
		confDir := t.TempDir()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var starts, childStarts int
		runtime := newVPPLifecycle(ctx, func(ctx context.Context, settings *VPPSettings, _ bool, ready chan<- error) error {
			starts++
			manager := NewVPPManager(settings, confDir, "/unused/partial-bind-vpp")
			if starts == 1 {
				manager.dpdk = fixture.binder
			}
			return manager.runWithOps(ctx, managerRunOps{
				bindAll: func(interfaces []DPDKInterface) error {
					return manager.dpdk.bindAllWithOps(interfaces, fixture.ops)
				},
				runOnce: func(ctx context.Context, _ string) error {
					childStarts++
					ready <- nil
					<-ctx.Done()
					return ctx.Err()
				},
				unbindAll: func() error { return manager.dpdk.unbindAllWithOps(fixture.ops) },
			})
		})
		if err := runtime.Install(first.settings); err != nil {
			t.Fatal(err)
		}
		failure := runtime.Start()
		if !errors.Is(failure, bindFailure) || !errors.Is(failure, restoreFailure) {
			t.Fatalf("initial partial bind lost either native cause: %v", failure)
		}
		if starts != 1 || childStarts != 0 || fixture.unbindCalls != 1 {
			t.Fatalf("initial failed bind escaped its original boundary: managers=%d children=%d cleanup=%d", starts, childStarts, fixture.unbindCalls)
		}
		if runtime.active == nil || runtime.active.cleanup == nil || runtime.active.cleanup.receipt == nil {
			t.Fatal("partial initial bind lost its terminal cleanup receipt")
		}
		if runtime.active.cleanup.receipt.manager.dpdk != fixture.binder {
			t.Fatal("partial bind handed cleanup to another manager or binder")
		}
		original, present := fixture.binder.savedDrivers[dpdkCleanupPCI]
		if !present || original.driver != "ixgbe" || original.override != dpdkCleanupOverride {
			t.Fatal("partial startup lost the original native restoration state")
		}
		second := *first.settings
		second.DPDK.Interfaces = []DPDKInterface{{PCIAddress: dpdkCleanupPCI, Name: "dpdk0"}}
		if err := runtime.Install(&second); !errors.Is(err, restoreFailure) || !errors.Is(err, sdk.ErrRunningConfigLost) {
			t.Fatalf("pending initial cleanup admitted another configuration: %v", err)
		}
		if starts != 1 || childStarts != 0 || fixture.unbindCalls != 2 {
			t.Fatalf("initial cleanup retry escaped original custody: managers=%d children=%d cleanup=%d", starts, childStarts, fixture.unbindCalls)
		}
		fixture.unbindFailure = nil
		if err := runtime.Install(&second); err != nil {
			t.Fatalf("confirmed original cleanup refused productive replacement: %v", err)
		}
		if starts != 2 || childStarts != 1 {
			t.Fatalf("replacement after initial cleanup managers=%d children=%d", starts, childStarts)
		}
		if err := runtime.Stop(); err != nil {
			t.Fatal(err)
		}
		fixture.restored(t)
	})
}

// Confirmed NIC restoration cannot clear an independent unresolved child
// cleanup result, and no fresh manager may replace that retained original.
func TestVPPStartupCleanupKeepsChildFailureAfterDriverRestoration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newDPDKCleanupFixture(t)
		fixture.bind(t)
		want := errors.New("original child descriptor cleanup unresolved")
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var starts int
		runtime := newVPPLifecycle(ctx, func(ctx context.Context, _ *VPPSettings, _ bool, ready chan<- error) error {
			starts++
			if starts != 1 {
				return errors.New("fresh manager entered after independent child cleanup failure")
			}
			manager := newCleanupOutcomeManager(t)
			manager.dpdk = fixture.binder
			return manager.runWithOps(ctx, managerRunOps{
				bindAll: manager.dpdk.BindAll,
				runOnce: func(ctx context.Context, _ string) error {
					ready <- nil
					<-ctx.Done()
					return nativeCleanupError{cause: want}
				},
				unbindAll: func() error { return fixture.binder.unbindAllWithOps(fixture.ops) },
			})
		})
		tx := newVPPConfigTx(runtime)
		if err := tx.Configure(startupLifecycleSections("first")); err != nil {
			t.Fatal(err)
		}
		if err := runtime.Start(); err != nil {
			t.Fatal(err)
		}
		if err := tx.Verify(startupLifecycleSections("second")); err != nil {
			t.Fatal(err)
		}
		if err := tx.Apply(); !errors.Is(err, want) || !errors.Is(err, sdk.ErrRunningConfigLost) {
			t.Fatalf("original child cleanup did not keep replacement blocked: %v", err)
		}
		fixture.restored(t)
		if err := runtime.Stop(); !errors.Is(err, want) {
			t.Fatalf("successful driver restoration erased independent child cleanup: %v", err)
		}
		if starts != 1 || runtime.active == nil {
			t.Fatal("independent child cleanup lost its retained original lifecycle")
		}
	})
}
