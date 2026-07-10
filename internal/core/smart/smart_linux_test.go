// VALIDATES: the SMART privilege-classification helpers — isPermissionError /
// isPermissionErrno recognize EPERM/EACCES (wrapped included) but not ENOENT,
// permissionDenied builds the Unavailable sentinel, and Detect short-circuits to
// nil in testdata mode (root != "") without opening any device.
// PREVENTS: a permission failure being misreported as a healthy/absent disk, or
// testdata mode accidentally issuing a real ioctl against /dev.

//go:build linux

package smart

import (
	"fmt"
	"io"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

func TestIsPermissionError(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"EPERM", unix.EPERM, true},
		{"EACCES", unix.EACCES, true},
		{"os.ErrPermission", os.ErrPermission, true},
		{"wrapped EACCES", fmt.Errorf("open: %w", unix.EACCES), true},
		{"ENOENT", unix.ENOENT, false},
		{"EOF", io.EOF, false},
		{"nil", nil, false},
	} {
		if got := isPermissionError(tc.err); got != tc.want {
			t.Errorf("%s: isPermissionError = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestIsPermissionErrno(t *testing.T) {
	for _, tc := range []struct {
		errno unix.Errno
		want  bool
	}{
		{unix.EPERM, true},
		{unix.EACCES, true},
		{unix.ENOENT, false},
		{0, false},
	} {
		if got := isPermissionErrno(tc.errno); got != tc.want {
			t.Errorf("isPermissionErrno(%v) = %v, want %v", tc.errno, got, tc.want)
		}
	}
}

func TestPermissionDenied(t *testing.T) {
	info := permissionDenied()
	if info == nil {
		t.Fatal("permissionDenied returned nil")
	}
	if !info.Unavailable {
		t.Error("Unavailable = false, want true")
	}
	if info.UnavailableNote != "insufficient privileges" {
		t.Errorf("UnavailableNote = %q, want %q", info.UnavailableNote, "insufficient privileges")
	}
	if info.Healthy {
		t.Error("Healthy = true, want false for a permission-denied result")
	}
}

func TestDetectTestdataModeReturnsNil(t *testing.T) {
	// A non-empty root selects testdata mode: Detect must return nil without
	// touching /dev, so the (nonexistent) device name is irrelevant.
	if got := Detect("nvme0n1", "/nonexistent/testdata/root"); got != nil {
		t.Errorf("Detect(root != \"\") = %+v, want nil", got)
	}
	if got := Detect("sda", "/some/root"); got != nil {
		t.Errorf("Detect(ATA, root != \"\") = %+v, want nil", got)
	}
}
