//go:build !linux

// Design: docs/features/interfaces.md -- the CAP_SYS_TIME probe the readiness check runs
// Overview: doctor.go -- checkNTPClockPrivilege's registration and its sibling check
//
// Off Linux there is no capability mask to read, and the doctor runner's stub
// for this platform reported nothing, so the check stays silent.

package ntp

import "github.com/ze-software/ze/internal/core/diagnostic"

func checkNTPClockPrivilege(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return nil
}
