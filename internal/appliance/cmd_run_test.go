package appliance

import (
	"testing"
)

func findArg(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// TestRunQEMUMemoryBytes verifies the QEMU -m size derives from
// image.memory-bytes and keeps the 512 MiB default when unset (AC-13).
func TestRunQEMUMemoryBytes(t *testing.T) {
	tests := []struct {
		name   string
		memory string
		want   string
	}{
		{"unset defaults to 512", "", "512"},
		{"1gb", "1gb", "1024"},
		{"4gb", "4gb", "4096"},
		{"512mb", "512mb", "512"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig("test")
			cfg.Image.Memory = tt.memory
			_, args := buildQEMUCommand(&cfg, "/tmp/test.img")
			if got := findArg(args, "-m"); got != tt.want {
				t.Errorf("-m = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildQEMUCommandUsesConfiguredPorts(t *testing.T) {
	cfg := &applianceConfig{
		SSH:   SSHConfig{Host: "0.0.0.0", Port: "8822"},
		Web:   WebConfig{Enabled: true, Host: "0.0.0.0", Port: "9443"},
		Image: ImageConfig{Arch: archAMD64},
		QEMU: QEMUConfig{
			SSHPort:     2222,
			WebPort:     28080,
			GokrazyPort: 18080,
		},
	}

	_, args := buildQEMUCommand(cfg, "/tmp/test.img")

	nic := findArg(args, "-nic")
	want := "user,model=e1000,hostfwd=tcp::28080-:9443,hostfwd=tcp::2222-:8822,hostfwd=tcp::18080-:443"
	if nic != want {
		t.Errorf("-nic arg:\n  got:  %s\n  want: %s", nic, want)
	}
}

func TestBuildQEMUCommandDefaultPorts(t *testing.T) {
	cfg := DefaultConfig("test")
	cfg.QEMU = QEMUConfig{
		SSHPort:     2222,
		WebPort:     28080,
		GokrazyPort: 18080,
	}

	_, args := buildQEMUCommand(&cfg, "/tmp/test.img")

	nic := findArg(args, "-nic")
	want := "user,model=e1000,hostfwd=tcp::28080-:8080,hostfwd=tcp::2222-:22,hostfwd=tcp::18080-:443"
	if nic != want {
		t.Errorf("-nic arg:\n  got:  %s\n  want: %s", nic, want)
	}
}
