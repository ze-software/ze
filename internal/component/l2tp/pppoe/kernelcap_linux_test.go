//go:build linux

package pppoe

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
)

// TestPPPoEKernelCapabilityIsEnrolled proves PPPoE declares CONFIG_PPPOE as a
// capability that refuses a start, and that the package init() enrolled it.
// Method: read the declaration and the live enrolment.
func TestPPPoEKernelCapabilityIsEnrolled(t *testing.T) {
	capability := pppoeKernelCapability()
	if capability.Kernel != "CONFIG_PPPOE" {
		t.Fatalf("pppoe enrolls %s, want CONFIG_PPPOE", capability.Kernel)
	}
	if capability.Degrades != "" {
		t.Fatal("pppoe must refuse a start: no session carries traffic without the socket")
	}
	if !slices.Contains(kernelcap.Enrolled(), "pppoe") {
		t.Fatal("pppoe is not enrolled by the package init()")
	}
}

// TestPPPoEInUse checks the predicate against ExtractParameters' reading, where
// only `enabled true` runs the access concentrator. Method: one tree per shape.
func TestPPPoEInUse(t *testing.T) {
	if PPPoEInUse(nil) {
		t.Fatal("a nil tree uses nothing")
	}
	if PPPoEInUse(config.NewTree()) {
		t.Fatal("a config with no pppoe block does not run PPPoE")
	}
	bare := config.NewTree()
	bare.GetOrCreateContainer("pppoe")
	if PPPoEInUse(bare) {
		t.Fatal("a pppoe block without enabled true runs nothing (ExtractParameters starts disabled)")
	}
	enabled := config.NewTree()
	enabled.GetOrCreateContainer("pppoe").Set("enabled", "true")
	if !PPPoEInUse(enabled) {
		t.Fatal("enabled true runs PPPoE")
	}
}
