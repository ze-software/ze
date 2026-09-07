package vpp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/cpulist"
	"github.com/ze-software/ze/internal/core/env"
)

// TestHostCPUInventoryIsolationUnknown is the guard test for the defect class
// this feature is most exposed to: an unreadable isolated file must not read as
// "the kernel isolated nothing".
//
// VALIDATES: ai/rules/principles.md -- code that cannot answer says so. An empty
// Isolated set with IsolationKnown false is "we could not tell", and with
// IsolationKnown true it is "no CPU is isolated".
// PREVENTS: Ze reporting a clean isolation state on a host it never managed to
// read, which would hide exactly the misconfiguration this feature exists to
// find.
func TestHostCPUInventoryIsolationUnknown(t *testing.T) {
	t.Run("isolated file absent leaves isolation unknown", func(t *testing.T) {
		root := t.TempDir()
		writeFixture(t, filepath.Join(root, "online"), "0-3\n")

		inv := inventoryFromRoot(t, root)
		if inv.IsolationKnown {
			t.Error("isolation reported as known with no isolated file to read")
		}
		if len(inv.Isolated) != 0 {
			t.Errorf("Isolated = %v, want empty", inv.Isolated)
		}
		if len(inv.Online) != 4 {
			t.Errorf("Online = %v, want 4 CPUs", inv.Online)
		}
	})

	t.Run("empty isolated file is a known empty set", func(t *testing.T) {
		root := t.TempDir()
		writeFixture(t, filepath.Join(root, "online"), "0-3\n")
		writeFixture(t, filepath.Join(root, "isolated"), "\n")

		inv := inventoryFromRoot(t, root)
		if !inv.IsolationKnown {
			t.Error("an empty isolated file is an answer, not a failure to read")
		}
		if len(inv.Isolated) != 0 {
			t.Errorf("Isolated = %v, want empty", inv.Isolated)
		}
	})

	t.Run("isolated cores are read", func(t *testing.T) {
		root := t.TempDir()
		writeFixture(t, filepath.Join(root, "online"), "0-7\n")
		writeFixture(t, filepath.Join(root, "isolated"), "2-4\n")

		inv := inventoryFromRoot(t, root)
		if !inv.IsolationKnown || cpulist.Format(inv.Isolated) != "2-4" {
			t.Errorf("Isolated = %v (known %v), want 2-4 known", inv.Isolated, inv.IsolationKnown)
		}
	})
}

// TestHostCPUInventoryUnreadableOnlineIsAnError proves the online file is the
// half Ze refuses to guess at: without it there is no answer to "does core N
// exist", and an empty set would answer "no" to every core.
func TestHostCPUInventoryUnreadableOnlineIsAnError(t *testing.T) {
	t.Run("absent online file", func(t *testing.T) {
		root := t.TempDir()
		if _, err := inventoryFromRootErr(t, root); err == nil {
			t.Fatal("a missing online file returned an inventory, want an error")
		}
	})

	t.Run("empty online file", func(t *testing.T) {
		root := t.TempDir()
		writeFixture(t, filepath.Join(root, "online"), "\n")
		_, err := inventoryFromRootErr(t, root)
		if err == nil {
			t.Fatal("an empty online file returned an inventory, want an error")
		}
		if !strings.Contains(err.Error(), "no online CPUs") {
			t.Errorf("error = %q, want it to name the empty inventory", err)
		}
	})
}

// TestCPUValidateCannotReadHost proves the entry point operators reach, config
// validation, refuses a CPU placement it could not check rather than accepting
// it in silence.
//
// VALIDATES: AC-4 at the guard level -- "the host could not be read" is not
// "every requested core exists".
// PREVENTS: a commit succeeding on a host where nothing verified the cores, so
// the operator believes a check ran.
func TestCPUValidateCannotReadHost(t *testing.T) {
	setCPURoot(t, filepath.Join(t.TempDir(), "absent"))

	cpu := CPUSettings{MainCore: new(uint8(0)), Workers: new(uint8(2))}
	err := cpu.validate()
	if err == nil {
		t.Fatal("validate accepted a CPU placement on a host it could not read")
	}
	if !strings.Contains(err.Error(), "vpp cpu") {
		t.Errorf("error = %q, want it to name the vpp cpu config", err)
	}
}

// TestCPUValidateNoPlacementReadsNoHost proves AC-6: with no cpu placement leaf
// set, validation neither reads the host nor refuses anything, so a config that
// validated before this feature still validates.
func TestCPUValidateNoPlacementReadsNoHost(t *testing.T) {
	setCPURoot(t, filepath.Join(t.TempDir(), "absent"))

	usec := uint32(10000)
	cpu := CPUSettings{PollSleepMicroseconds: &usec}
	if err := cpu.validate(); err != nil {
		t.Fatalf("validate refused a config that places no thread: %v", err)
	}
}

// TestCPUValidateWorkersAndCoresConflict proves the two ways to name the worker
// CPUs cannot be given at once. Ze does not pick between two answers to one
// question.
func TestCPUValidateWorkersAndCoresConflict(t *testing.T) {
	cpu := CPUSettings{Workers: new(uint8(2)), WorkerCores: []uint8{4, 5}}
	err := cpu.validate()
	if err == nil {
		t.Fatal("validate accepted both workers and worker-cores")
	}
	if !strings.Contains(err.Error(), "not both") {
		t.Errorf("error = %q, want it to name the conflict", err)
	}
}

// writeFixture writes one sysfs fixture file.
func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// setCPURoot points the CPU inventory reader at a fixture directory for the
// duration of one test.
func setCPURoot(t *testing.T, root string) {
	t.Helper()
	// Registered before t.Setenv so it runs AFTER the variable is restored:
	// cleanups run last-in-first-out, and a cache rebuilt from the fixture
	// value would outlive the test that set it.
	t.Cleanup(env.ResetCache)
	t.Setenv("ZE_TEST_VPP_CPU_ROOT", root)
	env.ResetCache()
}

// inventoryFromRoot reads an inventory from a fixture root and fails the test
// when the read errors.
func inventoryFromRoot(t *testing.T, root string) CPUInventory {
	t.Helper()
	inv, err := inventoryFromRootErr(t, root)
	if err != nil {
		t.Fatalf("hostCPUInventory: %v", err)
	}
	return inv
}

func inventoryFromRootErr(t *testing.T, root string) (CPUInventory, error) {
	t.Helper()
	setCPURoot(t, root)
	return hostCPUInventory()
}

// TestWorkerCoresSkipAnOfflineIsolatedCPU proves a CPU the kernel isolated but
// the host no longer runs never reaches corelist-workers.
//
// VALIDATES: AC-4 on the worker COUNT path -- the cores Ze derives itself are
// held to the same online inventory as the cores an operator names. The
// contiguous fallback already checked it; the isolated path did not.
// PREVENTS: /sys/devices/system/cpu/isolated naming a CPU that was taken
// offline after boot, which the kernel never revises, so a clean commit writes
// a core list VPP cannot start on -- the verify-passes-runtime-fails defect
// this spec exists to close.
func TestWorkerCoresSkipAnOfflineIsolatedCPU(t *testing.T) {
	// The kernel isolated 2-5 at boot; cores 4 and 5 were hotplugged out.
	inv := CPUInventory{
		Online:         []uint8{0, 1, 2, 3},
		Isolated:       []uint8{2, 3, 4, 5},
		IsolationKnown: true,
	}
	mainCore := uint8(0)

	t.Run("the offline cores are not placed", func(t *testing.T) {
		cpu := CPUSettings{MainCore: &mainCore, Workers: new(uint8(2))}
		cores, err := resolveWorkerCores(&cpu, inv)
		if err != nil {
			t.Fatalf("resolveWorkerCores: %v", err)
		}
		if got := cpulist.Format(cores); got != "2-3" {
			t.Errorf("corelist = %q, want %q", got, "2-3")
		}
	})

	t.Run("a count only the offline cores could satisfy is refused", func(t *testing.T) {
		cpu := CPUSettings{MainCore: &mainCore, Workers: new(uint8(3))}
		if _, err := resolveWorkerCores(&cpu, inv); err == nil {
			t.Fatal("resolveWorkerCores placed a worker on an offline CPU")
		}
		err := cpu.validateAgainst(inv)
		if err == nil {
			t.Fatal("validateAgainst accepted a placement the host cannot run")
		}
		if !strings.Contains(err.Error(), "available 2-3") {
			t.Errorf("error = %q, want it to name the cores actually available", err)
		}
	})
}
