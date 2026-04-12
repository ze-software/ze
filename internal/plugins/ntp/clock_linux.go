// Design: docs/features/interfaces.md -- NTP system clock operations

//go:build linux

package ntp

import (
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

// setClock sets the system clock via Settimeofday.
// Requires CAP_SYS_TIME (gokrazy grants this to all processes).
func setClock(t time.Time) error {
	tv := syscall.NsecToTimeval(t.UnixNano())
	if err := syscall.Settimeofday(&tv); err != nil {
		return fmt.Errorf("settimeofday: %w", err)
	}
	return nil
}

// rtcTime matches the kernel's struct rtc_time (linux/rtc.h).
type rtcTime struct {
	sec   int32
	min   int32
	hour  int32
	mday  int32
	mon   int32
	year  int32
	wday  int32
	yday  int32
	isdst int32
}

// rtcSetTimeIOCTL is the RTC_SET_TIME ioctl number (from linux/rtc.h).
const rtcSetTimeIOCTL = 0x4024700a

// setRTC writes the given time to /dev/rtc0. Returns nil if the RTC
// device does not exist (common on VMs). Other errors are returned.
func setRTC(t time.Time) error {
	f, err := os.OpenFile("/dev/rtc0", os.O_WRONLY, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No RTC device, not an error.
		}
		return fmt.Errorf("open /dev/rtc0: %w", err)
	}
	defer func() { _ = f.Close() }()

	utc := t.UTC()
	rt := rtcTime{
		sec:  int32(utc.Second()),
		min:  int32(utc.Minute()),
		hour: int32(utc.Hour()),
		mday: int32(utc.Day()),
		mon:  int32(utc.Month() - 1), // 0-based
		year: int32(utc.Year() - 1900),
	}

	if _, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		rtcSetTimeIOCTL,
		uintptr(unsafe.Pointer(&rt)), //nolint:gosec // required for ioctl RTC_SET_TIME
	); errno != 0 {
		return fmt.Errorf("ioctl RTC_SET_TIME: %w", errno)
	}
	return nil
}
