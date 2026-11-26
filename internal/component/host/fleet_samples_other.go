// Design: docs/architecture/fleet-config.md — non-Linux counterpart for host fleet metric sampling
// Related: fleet_samples_linux.go — Linux procfs and statfs implementation

//go:build !linux

package host

import "github.com/ze-software/ze/pkg/fleet"

// SampleFleetMetrics returns an empty host SourceSnapshot on non-Linux
// platforms where Linux procfs and statfs counters are unavailable.
func SampleFleetMetrics() fleet.SourceSnapshot {
	return fleet.SourceSnapshot{
		Owner:    fleet.SampleOwnerHost,
		Readings: []fleet.SourceReading{},
	}
}
