//go:build !linux

// Design: ai/rules/repo-maintenance.md -- macvlan capability probe (non-Linux stub)
// Overview: ifacenetlink.go -- package hub
//
// The macvlan owned-device mechanism only touches the kernel on Linux, so off
// Linux there is no CONFIG_MACVLAN dependency to warn about; the probe reports
// the capability available so the doctor check stays quiet.

package ifacenetlink

// probeMacvlanCapability is a stub on non-Linux: there is no macvlan dependency.
func probeMacvlanCapability() macvlanProbeResult { return macvlanProbeOK }
