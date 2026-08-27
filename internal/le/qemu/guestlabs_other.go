//go:build !linux

// Design: plan/spec-le-is-a-ze-binary.md -- step 10 guest-side evidence ports

package qemu

import (
	"context"
	"errors"
)

var errGuestLabsLinux = errors.New("qemu guest evidence requires Linux")

func runVRRPGuest(context.Context, string, []string) (GuestLabReport, error) {
	return GuestLabReport{}, errGuestLabsLinux
}

func runPPPoEAccelGuest(context.Context, string) (GuestLabReport, error) {
	return GuestLabReport{}, errGuestLabsLinux
}

func runNetnsGuest(context.Context, []string) (GuestLabReport, error) {
	return GuestLabReport{}, errGuestLabsLinux
}
