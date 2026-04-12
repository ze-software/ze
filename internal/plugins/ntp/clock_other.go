// Design: docs/features/interfaces.md -- NTP clock stub for non-Linux

//go:build !linux

package ntp

import (
	"fmt"
	"runtime"
	"time"
)

func setClock(_ time.Time) error {
	return fmt.Errorf("ntp: set clock not supported on %s", runtime.GOOS)
}

func setRTC(_ time.Time) error {
	return fmt.Errorf("ntp: rtc not supported on %s", runtime.GOOS)
}
