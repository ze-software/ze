// Design: docs/architecture/fleet-config.md — managed fleet health report producer registration
// Related: report.go — collectFleetHealth and per-session report worker

package managed

import "github.com/ze-software/ze/pkg/fleet"

func init() {
	fleet.MustRegisterReportProducer(fleet.ReportProducer{
		Name:     fleetHealthProducerName,
		Method:   fleet.VerbHealthReport,
		Interval: FleetHealthInterval,
		Collect:  collectFleetHealth,
	})
}
