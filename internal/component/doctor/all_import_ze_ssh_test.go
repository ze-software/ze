// Design: docs/architecture/doctor-and-health-checks.md -- link ssh doctor registrations in doctor tests.

//go:build ze_ssh

package doctor_test

import (
	// Mirror cmd/ze/hub/service_ssh.go so ssh's doctor check and code metadata
	// register in the doctor test binary just as they do in ze.
	_ "github.com/ze-software/ze/internal/component/ssh"
)
