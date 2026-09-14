package doctor_test

import (
	"fmt"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

func TestZZDumpProbe(t *testing.T) {
	for _, phase := range []diagnostic.DoctorCheckPhase{
		diagnostic.DoctorPhasePreConfig,
		diagnostic.DoctorPhaseMissingConfig,
		diagnostic.DoctorPhasePostConfig,
	} {
		for _, c := range diagnostic.DoctorChecksForPhase(phase) {
			fmt.Printf("PROBE %s %d %s %s\n", phase, c.Order, c.Name, c.Component)
		}
	}
}
