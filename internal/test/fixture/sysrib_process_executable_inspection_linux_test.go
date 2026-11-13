// Design: docs/functional-tests.md -- account-independent inspection uses the kernel capability.

package fixture

import (
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/runner"
)

func TestSysribExecutableInspectionCapabilityIsTheKernelBit(t *testing.T) {
	capability := runner.FixtureProcessInspectionCapability()
	if capability.Name != "CAP_SYS_PTRACE" {
		t.Fatalf("fixture inspection capability=%q", capability.Name)
	}
	if capability.Bit != unix.CAP_SYS_PTRACE {
		t.Fatalf("fixture bit=%d, kernel CAP_SYS_PTRACE=%d", capability.Bit, unix.CAP_SYS_PTRACE)
	}
}
