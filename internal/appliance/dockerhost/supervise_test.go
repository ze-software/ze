package dockerhost

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestSupervisorDockerdArgv resolves docker.service through the supervisor
// with no Colima drop-in, with Colima's drop-in, and with a drop-in whose
// gateway is not an address, and checks the argv each time.
//
// VALIDATES: spec D-7 and R-4: dockerd runs a fixed argv (the unix socket,
// group docker, containerd's socket) and takes only --host-gateway-ip from
// Colima's docker.conf; Colima's `-H fd://` is never obeyed. containerd.service
// runs containerd on the seeded config, and both restart whenever they exit.
// PREVENTS: a dockerd left without a socket because socket activation was
// assumed, or a drop-in value reaching dockerd's argv unchecked.
func TestSupervisorDockerdArgv(t *testing.T) {
	root := t.TempDir()
	sup, err := NewSupervisor(root, uint32(os.Getuid()), DockerUnits())
	if err != nil {
		t.Fatalf("NewSupervisor: %v", err)
	}
	fixed := []string{"/usr/bin/dockerd", "--host=unix:///var/run/docker.sock", "--group=docker",
		"--containerd=/run/containerd/containerd.sock", "--data-root=/perm/var/lib/docker", "--exec-root=/run/docker"}

	spec, err := sup.lookup("docker.service")
	if err != nil {
		t.Fatalf("lookup docker.service: %v", err)
	}
	if !slices.Equal(spec.Argv, fixed) {
		t.Errorf("dockerd argv without a drop-in = %q, want %q", spec.Argv, fixed)
	}
	if spec.Restart != RestartAlways {
		t.Errorf("docker.service restart = %v, want RestartAlways", spec.Restart)
	}

	dropIn := guest(root, dockerDropInDir)
	if err := os.MkdirAll(dropIn, 0o755); err != nil {
		t.Fatal(err)
	}
	colima := "[Service]\nExecStart=\nExecStart=/usr/bin/dockerd -H fd:// --containerd=/run/containerd/containerd.sock --host-gateway-ip=192.168.5.2\n"
	if err := os.WriteFile(filepath.Join(dropIn, "docker.conf"), []byte(colima), 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err = sup.lookup("docker.service")
	if err != nil {
		t.Fatalf("lookup docker.service with Colima's drop-in: %v", err)
	}
	if want := append(slices.Clone(fixed), "--host-gateway-ip=192.168.5.2"); !slices.Equal(spec.Argv, want) {
		t.Errorf("dockerd argv with Colima's drop-in = %q, want %q", spec.Argv, want)
	}
	if slices.Contains(spec.Argv, "fd://") || slices.Contains(spec.Argv, "-H") {
		t.Errorf("dockerd argv %q obeys the drop-in's socket activation", spec.Argv)
	}

	bad := "[Service]\nExecStart=/usr/bin/dockerd --host-gateway-ip=192.168.5.2;reboot\n"
	if err := os.WriteFile(filepath.Join(dropIn, "docker.conf"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := sup.lookup("docker.service"); err == nil || !strings.Contains(err.Error(), "host-gateway-ip") {
		t.Errorf("a drop-in gateway that is not an address gave %v, want a refusal naming --host-gateway-ip", err)
	}

	spec, err = sup.lookup("containerd.service")
	if err != nil {
		t.Fatalf("lookup containerd.service: %v", err)
	}
	if want := []string{"/usr/bin/containerd", "--config", "/etc/containerd/config.toml",
		"--root", "/perm/var/lib/containerd", "--state", "/run/containerd"}; !slices.Equal(spec.Argv, want) {
		t.Errorf("containerd argv = %q, want %q", spec.Argv, want)
	}
	if spec.Restart != RestartAlways {
		t.Errorf("containerd.service restart = %v, want RestartAlways", spec.Restart)
	}
}

// TestContainerdConfigSeeded runs seedContainerdConfig twice over a temp root
// and asserts the first run writes /etc/containerd/config.toml and the second
// leaves an existing file alone.
//
// VALIDATES: spec AC-12: the config exists before Colima provisions, so
// Colima's `sudo cp config.toml config.colima.bak.toml` succeeds.
// PREVENTS: a boot overwriting the config Colima wrote.
func TestContainerdConfigSeeded(t *testing.T) {
	root := t.TempDir()
	if err := seedContainerdConfig(root); err != nil {
		t.Fatalf("seedContainerdConfig: %v", err)
	}
	path := guest(root, containerdConfig)
	seeded, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no seeded config: %v", err)
	}
	if !strings.Contains(string(seeded), "version") {
		t.Errorf("seeded config %q names no config version", seeded)
	}
	if err := os.WriteFile(path, []byte("colima's config\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := seedContainerdConfig(root); err != nil {
		t.Fatalf("seedContainerdConfig over an existing file: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "colima's config\n" { //nolint:errcheck // compared below
		t.Errorf("second seed replaced the existing config with %q", got)
	}
}
