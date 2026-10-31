//go:build !linux && !darwin

// Design: docs/architecture/testing/qemu-integration.md -- a VM never outlives its le

package testqemu

import "errors"

// errProcessStartTimeUnsupported is the refusal on a host where le has no
// reader for a process start time. Without one, the VM record cannot tell a
// live le from a reused pid, so a run stops the VM it just started and fails
// rather than leave one nothing would stop.
var errProcessStartTimeUnsupported = errors.New(
	"this host has no process start-time reader (le reads it on Linux and macOS only), " +
		"so a VM le starts here could outlive it: run the QEMU harness on Linux or macOS")

// processStartTime refuses on every host but Linux and macOS. It never answers
// a zero start time, which a reap would compare as if it were real.
func processStartTime(int) (int64, error) { return 0, errProcessStartTimeUnsupported }
