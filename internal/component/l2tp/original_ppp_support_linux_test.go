//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- L2TP-only discovery.

package l2tp

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// This package does not import PPPoE or the CPE client. L2TP-only composition
// must still discover its own dependency through the existing registry.
func TestL2TPOriginalPPPSupportRegisteredScope(t *testing.T) {
	var checks int
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if check.Name == "l2tp-original-ppp-support" {
			checks++
		}
	}
	if checks != 1 {
		t.Fatalf("L2TP-only registrations %d", checks)
	}
	for _, test := range []struct {
		name   string
		source string
		want   uint32
	}{
		{"empty", "", 0}, {"disabled", "l2tp { enabled false }", 0},
		{"native", "l2tp {}", pppkernel.OriginalPPPSupportRequirements(false)},
		{"relay", "l2tp { relay access { remote upstream } }", pppkernel.OriginalPPPSupportRequirements(true)},
		{"realm", "l2tp { relay-realm example.net { remote upstream } }", pppkernel.OriginalPPPSupportRequirements(true)},
	} {
		t.Run(test.name, func(t *testing.T) {
			schema, err := config.YANGSchema()
			if err != nil {
				t.Fatal(err)
			}
			tree, err := config.NewParser(schema).Parse(test.source)
			if err != nil {
				t.Fatal(err)
			}
			got, err := l2tpOriginalPPPSupportScope(diagnostic.DoctorCheckContext{Tree: tree})
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("required %#x want %#x", got, test.want)
			}
		})
	}
}
