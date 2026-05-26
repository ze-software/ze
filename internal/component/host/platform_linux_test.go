//go:build linux

package host

import (
	"testing"
)

func TestDetectPlatform_Gokrazy(t *testing.T) {
	d := &Detector{Root: "testdata/platform-gokrazy"}
	p, err := d.DetectPlatform()
	if err != nil {
		t.Fatalf("DetectPlatform: %v", err)
	}
	if p.Type != PlatformGokrazy {
		t.Errorf("Type = %v, want gokrazy", p.Type)
	}
	if !p.PermAvailable {
		t.Error("PermAvailable should be true")
	}
}

func TestDetectPlatform_Systemd(t *testing.T) {
	d := &Detector{Root: "testdata/platform-systemd"}
	p, err := d.DetectPlatform()
	if err != nil {
		t.Fatalf("DetectPlatform: %v", err)
	}
	if p.Type != PlatformSystemd {
		t.Errorf("Type = %v, want systemd", p.Type)
	}
	if !p.SystemdAvailable {
		t.Error("SystemdAvailable should be true")
	}
}

func TestDetectPlatform_Container(t *testing.T) {
	d := &Detector{Root: "testdata/platform-container"}
	p, err := d.DetectPlatform()
	if err != nil {
		t.Fatalf("DetectPlatform: %v", err)
	}
	if p.Type != PlatformContainer {
		t.Errorf("Type = %v, want container", p.Type)
	}
}

func TestDetectPlatform_ContainerCgroupsV2(t *testing.T) {
	d := &Detector{Root: "testdata/platform-container-cgv2"}
	p, err := d.DetectPlatform()
	if err != nil {
		t.Fatalf("DetectPlatform: %v", err)
	}
	if p.Type != PlatformContainer {
		t.Errorf("Type = %v, want container (cgroups v2 mountinfo detection)", p.Type)
	}
}

func TestDetectPlatform_PlainLinux(t *testing.T) {
	d := &Detector{Root: "testdata/platform-plain"}
	p, err := d.DetectPlatform()
	if err != nil {
		t.Fatalf("DetectPlatform: %v", err)
	}
	if p.Type != PlatformPlainLinux {
		t.Errorf("Type = %v, want plain-linux", p.Type)
	}
}

func TestClassifyPlatform_GokrazySocket(t *testing.T) {
	info := &PlatformInfo{GokrazyUpdateSocket: true}
	got := classifyPlatform(info, "/nonexistent")
	if got != PlatformGokrazy {
		t.Errorf("classifyPlatform = %v, want gokrazy", got)
	}
}

func TestClassifyPlatform_GokrazyPermReadOnly(t *testing.T) {
	info := &PlatformInfo{PermAvailable: true, ReadOnlyRoot: true}
	got := classifyPlatform(info, "/nonexistent")
	if got != PlatformGokrazy {
		t.Errorf("classifyPlatform = %v, want gokrazy", got)
	}
}

func TestClassifyPlatform_SystemdFallback(t *testing.T) {
	info := &PlatformInfo{SystemdAvailable: true}
	got := classifyPlatform(info, "/nonexistent")
	if got != PlatformSystemd {
		t.Errorf("classifyPlatform = %v, want systemd", got)
	}
}

func TestClassifyPlatform_PlainLinuxFallback(t *testing.T) {
	info := &PlatformInfo{}
	got := classifyPlatform(info, "/nonexistent")
	if got != PlatformPlainLinux {
		t.Errorf("classifyPlatform = %v, want plain-linux", got)
	}
}

func TestPlatformTypeString(t *testing.T) {
	cases := []struct {
		p    PlatformType
		want string
	}{
		{PlatformUnknown, "unknown"},
		{PlatformGokrazy, "gokrazy"},
		{PlatformSystemd, "systemd"},
		{PlatformContainer, "container"},
		{PlatformPlainLinux, "plain-linux"},
		{PlatformDarwin, "darwin"},
		{PlatformType(255), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.p.String(); got != tc.want {
			t.Errorf("PlatformType(%d).String() = %q, want %q", tc.p, got, tc.want)
		}
	}
}
