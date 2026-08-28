//go:build !linux

// Design: plan/spec-le-is-a-ze-binary.md -- step 10 guest-side evidence ports

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
