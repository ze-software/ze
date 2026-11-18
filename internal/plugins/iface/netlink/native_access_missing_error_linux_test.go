// Design: docs/features/interfaces.md -- original native VLAN custody
// Related: native_access_linux.go -- missing-link retirement classification
//go:build linux

package ifacenetlink

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"golang.org/x/sys/unix"
)

// Goal: only a genuine missing-link cause permits absence reconciliation.
// Method: distinguish direct, wrapped and joined ENODEV from unrelated errors.
func TestNativeAccessLinkAbsentRequiresMissingCause(t *testing.T) {
	cases := []struct {
		name   string
		cause  error
		absent bool
	}{
		{name: "no failure"},
		{name: "native missing", cause: unix.ENODEV, absent: true},
		{name: "wrapped missing", cause: fmt.Errorf("original lookup: %w", unix.ENODEV), absent: true},
		{name: "joined missing", cause: errors.Join(context.Canceled, unix.ENODEV), absent: true},
		{name: "permission refused", cause: unix.EPERM},
		{name: "busy device", cause: unix.EBUSY},
		{name: "other absence", cause: unix.ENOENT},
		{name: "caller expired", cause: context.DeadlineExceeded},
		{name: "joined unrelated", cause: errors.Join(context.Canceled, unix.EPERM)},
	}
	for _, entry := range cases {
		t.Run(entry.name, func(t *testing.T) {
			if actual := nativeVLANLinkAbsent(entry.cause); actual != entry.absent {
				t.Fatalf("missing-link classification = %v, want %v", actual, entry.absent)
			}
		})
	}
}
