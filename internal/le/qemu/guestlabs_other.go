//go:build !linux

// Design: docs/architecture/testing/qemu-integration.md -- guest-side evidence ports

package qemu

import (
	"context"
	"errors"
)

var errGuestLabsLinux = errors.New("qemu guest evidence requires Linux")

func runVRRPGuest(context.Context, string, []string) (guestLabReport, error) {
	return guestLabReport{}, errGuestLabsLinux
}

func runPPPoEAccelGuest(context.Context, string) (guestLabReport, error) {
	return guestLabReport{}, errGuestLabsLinux
}

func runNetnsGuest(context.Context, []string) (guestLabReport, error) {
	return guestLabReport{}, errGuestLabsLinux
}
