package dockerhost

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
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

// TestServiceLaunchRefusedOnceStopping checks the restart path against a stop
// that overtakes it: launch on a service already stopping starts nothing,
// and a real service stopped inside its restart backoff ends at once.
//
// VALIDATES: review R1-1: stop sets stopping under svc.mu and launch checks
// it under the same lock, so a stop during the backoff cannot leave a new
// process running that nobody signals.
// PREVENTS: Stop signaling the dead process, then blocking forever on done
// while the restarted process runs.
func TestServiceLaunchRefusedOnceStopping(t *testing.T) {
	svc := &service{
		spec:     UnitSpec{Name: "sleeper.service", Argv: []string{"/bin/sleep", "60"}, Restart: RestartAlways, StopGrace: time.Second},
		log:      io.Discard,
		quit:     make(chan struct{}),
		done:     make(chan struct{}),
		stopping: true,
	}
	cmd, err := svc.launch()
	if cmd != nil {
		_ = cmd.Process.Kill() //nolint:errcheck // cleanup of the defect under test
		_ = cmd.Wait()         //nolint:errcheck // reaped
	}
	if !errors.Is(err, errServiceStopping) {
		t.Fatalf("launch on a stopping service = %v, want errServiceStopping", err)
	}
	if svc.process != nil {
		t.Fatalf("launch on a stopping service recorded process %d", svc.process.Pid)
	}

	// The second half proves the backoff wait itself watches quit: a stop
	// that lands inside the restart backoff ends the service at once rather
	// than after the backoff. It waits for the supervisor's "starting it
	// again" line, so the stop is known to land inside the backoff, then
	// bounds the stop well under restartBackoffMin. Without the quit case in
	// supervise's backoff select, the stop waits out the backoff and this
	// half fails.
	retrying := &announcement{marker: "starting it again", seen: make(chan struct{})}
	failing, err := startService(UnitSpec{Name: "fails.service", Argv: []string{"/bin/sh", "-c", "exit 1"}, Restart: RestartAlways, StopGrace: time.Second}, retrying)
	if err != nil {
		t.Fatalf("startService: %v", err)
	}
	select {
	case <-retrying.seen:
	case <-time.After(5 * time.Second):
		failing.stop()
		t.Fatal("the failing service never entered its restart backoff within 5s")
	}
	stopped := make(chan struct{})
	go stopAndClose(failing, stopped)
	bound := restartBackoffMin / 2
	select {
	case <-stopped:
	case <-time.After(bound):
		<-stopped
		t.Fatalf("stop during the restart backoff took longer than %s: the backoff wait does not watch quit", bound)
	}
}

// stopAndClose stops svc and closes stopped, so a test can bound the stop.
func stopAndClose(svc *service, stopped chan<- struct{}) {
	svc.stop()
	close(stopped)
}

// announcement is a service log that closes seen the first time a write
// carries marker. Safe for concurrent use: the service's process and its
// lifecycle goroutine both write to it.
type announcement struct {
	marker string
	seen   chan struct{}
	once   sync.Once
}

func (a *announcement) Write(p []byte) (int, error) {
	if strings.Contains(string(p), a.marker) {
		a.once.Do(func() { close(a.seen) })
	}
	return len(p), nil
}

// TestDockerUnitStopGrace checks each built-in unit's stop grace: dockerd
// gets stopGraceDocker, containerd stopGraceDefault, all within the bound
// the shim deadline is built on.
//
// VALIDATES: review R1-12: dockerd is not killed at 10 s while it stops its
// containers.
// PREVENTS: a grace above the shim deadline, which would time out systemctl
// stop while the unit is still stopping.
func TestDockerUnitStopGrace(t *testing.T) {
	sup, err := NewSupervisor(t.TempDir(), uint32(os.Getuid()), DockerUnits())
	if err != nil {
		t.Fatalf("NewSupervisor: %v", err)
	}
	want := map[string]time.Duration{"docker.service": stopGraceDocker, "containerd.service": stopGraceDefault}
	for name, grace := range want {
		spec, err := sup.lookup(name)
		if err != nil {
			t.Fatalf("lookup %s: %v", name, err)
		}
		if spec.StopGrace != grace {
			t.Errorf("%s stop grace = %s, want %s", name, spec.StopGrace, grace)
		}
	}
	if shimDeadline <= stopGraceMax {
		t.Errorf("shim deadline %s does not exceed the largest stop grace %s", shimDeadline, stopGraceMax)
	}
}
