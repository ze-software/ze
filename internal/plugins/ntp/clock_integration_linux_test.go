// VALIDATES: the privileged clock adjusters run against the real kernel —
// setRTC no-ops cleanly when /dev/rtc0 is absent and slewClock issues a
// zero-offset Adjtimex without error (or is skipped without CAP_SYS_TIME).
// Auto-enrolled in the QEMU integration run via the derived `integration &&
// linux` package list.
// PREVENTS: an Adjtimex/RTC-ioctl regression surfacing only when a live
// appliance without an RTC syncs its clock.

//go:build integration && linux

package ntp

import (
	"errors"
	"testing"

	"golang.org/x/sys/unix"
)

func TestSetRTCNoDevice(t *testing.T) {
	// setRTC must not fail when /dev/rtc0 is absent (the common QEMU case) so
	// the sync loop stays healthy on RTC-less hardware.
	if err := setRTC(currentTime()); err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			t.Skipf("setRTC needs privileges here: %v", err)
		}
		t.Errorf("setRTC returned an unexpected error: %v", err)
	}
}

func TestSlewClockZeroOffset(t *testing.T) {
	// A zero-offset slew is a harmless no-op adjustment; it still requires
	// CAP_SYS_TIME, so skip gracefully when unprivileged.
	if err := slewClock(0); err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			t.Skipf("slewClock needs CAP_SYS_TIME: %v", err)
		}
		t.Errorf("slewClock(0) returned an unexpected error: %v", err)
	}
}
