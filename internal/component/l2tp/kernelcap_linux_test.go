//go:build linux

package l2tp

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
)

// TestL2TPKernelCapabilitiesAreEnrolled proves L2TP declares both capabilities
// with their build symbols, each refusing a start, and that the package init()
// enrolled them. Method: read the declaration and the live enrolment.
func TestL2TPKernelCapabilitiesAreEnrolled(t *testing.T) {
	want := map[string]string{"l2tp": "CONFIG_L2TP", "l2tp-ppp": "CONFIG_PPPOL2TP"}
	for _, capability := range l2tpKernelCapabilities() {
		kernel, ok := want[capability.Subsystem]
		if !ok {
			t.Errorf("unexpected capability %s", capability.Subsystem)
			continue
		}
		if capability.Kernel != kernel {
			t.Errorf("%s enrolls %s, want %s", capability.Subsystem, capability.Kernel, kernel)
		}
		if capability.Degrades != "" {
			t.Errorf("%s must refuse a start: L2TP carries nothing without it", capability.Subsystem)
		}
	}
	enrolled := kernelcap.Enrolled()
	for subsystem := range want {
		if !slices.Contains(enrolled, subsystem) {
			t.Errorf("%s is not enrolled by the package init()", subsystem)
		}
	}
}

// TestL2TPInUse checks the predicate against the readings ParseParameters
// makes. Method: one tree per shape.
func TestL2TPInUse(t *testing.T) {
	if L2TPInUse(nil) {
		t.Fatal("a nil tree uses nothing")
	}
	empty := config.NewTree()
	if L2TPInUse(empty) {
		t.Fatal("a config with no l2tp block does not run L2TP")
	}
	present := config.NewTree()
	present.GetOrCreateContainer("l2tp")
	if !L2TPInUse(present) {
		t.Fatal("an l2tp block with no enabled leaf runs L2TP")
	}
	disabled := config.NewTree()
	disabled.GetOrCreateContainer("l2tp").Set("enabled", "false")
	if L2TPInUse(disabled) {
		t.Fatal("enabled false does not run L2TP")
	}
	enabled := config.NewTree()
	enabled.GetOrCreateContainer("l2tp").Set("enabled", "true")
	if !L2TPInUse(enabled) {
		t.Fatal("enabled true runs L2TP")
	}
}
