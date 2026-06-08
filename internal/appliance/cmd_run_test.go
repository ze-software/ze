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

func TestBuildQEMUCommandUsesConfiguredPorts(t *testing.T) {
	cfg := &ApplianceConfig{
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
