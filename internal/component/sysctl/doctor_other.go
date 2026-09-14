// Design: docs/features/ai-first.md -- the writability probe the readiness check runs
// Overview: doctor.go -- checkSysctlProcfs, the one caller

//go:build !linux

package sysctl

// procSysWritable is the probe checkSysctlProcfs runs. Off Linux there is no
// /proc/sys to write, and the doctor runner's stub for this platform reported
// nothing, so the probe answers nil and the check stays silent. It is a
// variable for the same reason as its Linux twin: a test stands in a refusal.
var procSysWritable = func(string) error { return nil }
