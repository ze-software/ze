// Design: docs/architecture/doctor-and-health-checks.md -- link telemetry/exporter doctor registrations in doctor tests.

//go:build ze_telemetry

package doctor_test

import (
	// Mirror cmd/ze/hub/register_telemetry.go so telemetry/exporter's doctor
	// check and code metadata register in the doctor test binary just as they do
	// in ze.
	_ "github.com/ze-software/ze/internal/component/telemetry/exporter"
)
