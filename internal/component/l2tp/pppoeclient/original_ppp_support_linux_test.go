//go:build linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- active client scope.

package pppoeclient

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// Only entries the iface reconciler will start require native support; the
// disable leaf is presence-based, matching the real config producer.
func TestClientOriginalPPPSupportRegisteredScope(t *testing.T) {
	var checks int
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if check.Name == "pppoe-client-original-ppp-support" {
			checks++
		}
	}
	if checks != 1 {
		t.Fatalf("client registrations %d", checks)
	}
	for _, active := range []bool{false, true} {
		tree := config.NewTree()
		entry := config.NewTree()
		if !active {
			entry.Set("disable", "")
		}
		tree.GetOrCreateContainer("interface").AddListEntry("pppoe-client", "wan", entry)
		got, err := clientOriginalPPPSupportScope(diagnostic.DoctorCheckContext{Tree: tree})
		if err != nil {
			t.Fatal(err)
		}
		var want uint32
		if active {
			want = pppkernel.OriginalPPPSupportRequirements(false)
		}
		if got != want {
			t.Fatalf("required %#x want %#x", got, want)
		}
	}
}
