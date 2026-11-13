// Design: docs/functional-tests.md -- Linux capability admission before a privileged child starts.

package fixture

import (
	"errors"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/runner"
)

func TestSysribExecutableKillCapabilityIsTheKernelBit(t *testing.T) {
	capability := runner.FixtureProcessControlCapability()
	if capability.Name != "CAP_KILL" {
		t.Fatalf("fixture process control capability=%q", capability.Name)
	}
	if capability.Bit != unix.CAP_KILL {
		t.Fatalf("fixture bit=%d, kernel CAP_KILL=%d", capability.Bit, unix.CAP_KILL)
	}
}

func TestSysribExecutableKillProbeReadsEffectiveCapability(t *testing.T) {
	for _, present := range []bool{false, true} {
		got, err := sysribExecutableKillCapability(func(header *unix.CapUserHeader, first *unix.CapUserData) error {
			if header.Version != unix.LINUX_CAPABILITY_VERSION_3 {
				t.Fatalf("capability ABI=%x", header.Version)
			}
			if header.Pid != 0 {
				t.Fatalf("capability query inspected another process: %d", header.Pid)
			}
			// Permitted alone is no authority to stop a privileged child.
			first.Permitted = 1 << unix.CAP_KILL
			if present {
				first.Effective = 1 << unix.CAP_KILL
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if got != present {
			t.Fatalf("effective CAP_KILL=%v, want %v", got, present)
		}
	}
}

func TestSysribExecutableKillProbePreservesUnknown(t *testing.T) {
	cause := errors.New("capget refused")
	got, err := sysribExecutableKillCapability(func(*unix.CapUserHeader, *unix.CapUserData) error {
		return cause
	})
	if !errors.Is(err, cause) {
		t.Fatalf("capget failure lost its cause: %v", err)
	}
	if got {
		t.Fatal("a failed capget query granted child control")
	}
}
