// Design: docs/architecture/doctor-and-health-checks.md -- link bgp/config doctor registrations in doctor tests.

//go:build ze_bgp

package doctor_test

import (
	// Mirror cmd/ze/infra_bgp.go so bgp/config's doctor checks and code metadata
	// register in the doctor test binary just as they do in ze.
	_ "github.com/ze-software/ze/internal/component/bgp/config"
)
