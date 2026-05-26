//go:build !linux

package host

import (
	"runtime"
	"testing"
)

func TestDetectPlatform_NonLinux(t *testing.T) {
	d := &Detector{}
	p, err := d.DetectPlatform()
	if err != nil {
		t.Fatalf("DetectPlatform: %v", err)
	}
	if runtime.GOOS == "darwin" && p.Type != PlatformDarwin {
		t.Errorf("Type = %v, want darwin", p.Type)
	}
	if p.RebootAllowed {
		t.Error("RebootAllowed should be false on non-Linux")
	}
}

func TestPlatformTypeString_NonLinux(t *testing.T) {
	cases := []struct {
		p    PlatformType
		want string
	}{
		{PlatformUnknown, "unknown"},
		{PlatformGokrazy, "gokrazy"},
		{PlatformDarwin, "darwin"},
	}
	for _, tc := range cases {
		if got := tc.p.String(); got != tc.want {
			t.Errorf("PlatformType(%d).String() = %q, want %q", tc.p, got, tc.want)
		}
	}
}
