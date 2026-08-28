//go:build !linux

package qemu

import (
	"context"
	"errors"
	"testing"
)

func TestGuestLabsRefuseOutsideLinux(t *testing.T) {
	t.Parallel()
	for name, run := range map[string]func() error{
		"vrrp":        func() error { _, err := runVRRPGuest(context.Background(), "", vrrpScenarioNames); return err },
		"pppoe-accel": func() error { _, err := runPPPoEAccelGuest(context.Background(), ""); return err },
		"netns":       func() error { _, err := runNetnsGuest(context.Background(), defaultNetnsSuites); return err },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := run(); !errors.Is(err, errGuestLabsLinux) {
				t.Fatalf("refusal = %v", err)
			}
		})
	}
}
