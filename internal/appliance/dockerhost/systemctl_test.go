package dockerhost

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSystemctlShimLimaAndColimaVerbs drives the shim through the
// supervisor's control socket with the argv Lima's `lima-guestagent
// install-systemd` runs, then the verbs Colima and the spec's AC-11 use.
//
// VALIDATES: spec AC-11 exit codes 0, 3 and 5; %t and %N expanded in the
// ExecStart Lima wrote; R-1 (the agent is started through the shim).
// Method: a real supervisor over a temp root, a real process as the unit.
// PREVENTS: Lima's guest agent never starting, so the instance is Degraded.
func TestSystemctlShimLimaAndColimaVerbs(t *testing.T) {
	root := t.TempDir()
	unitDir := filepath.Join(root, "etc", "systemd", "system")
	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "agent-runtime-dir")
	unit := "[Unit]\nDescription=lima-guestagent\n\n[Service]\n" +
		"ExecStart=/bin/sh -c 'echo \"$0\" > " + marker + "; exec sleep 60' --runtime-dir=\"%t/%N\"\n" +
		"Type=simple\nRestart=on-failure\n\n[Install]\nWantedBy=multi-user.target\n"
	if err := os.WriteFile(filepath.Join(unitDir, "lima-guestagent.service"), []byte(unit), 0o644); err != nil {
		t.Fatal(err)
	}

	sup, err := NewSupervisor(root, uint32(os.Getuid()), nil)
	if err != nil {
		t.Fatalf("NewSupervisor: %v", err)
	}
	socket := filepath.Join(root, "run", "ze-docker-host", "systemctl.sock")
	stop, err := sup.Serve(socket)
	if err != nil {
		t.Fatalf("Serve: %v", err)
	}
	defer stop()

	run := func(args ...string) (int, string, string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := RunShim(socket, args, &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	expect := func(want int, args ...string) string {
		t.Helper()
		code, out, errOut := run(args...)
		if code != want {
			t.Fatalf("systemctl %s = %d, want %d (stdout %q, stderr %q)", strings.Join(args, " "), code, want, out, errOut)
		}
		return out + errOut
	}

	// Lima's install-systemd sequence, verbatim.
	expect(0, "--system", "daemon-reload")
	expect(0, "--system", "enable", "lima-guestagent.service")
	expect(0, "--system", "try-restart", "lima-guestagent.service")
	expect(0, "--system", "start", "lima-guestagent.service")
	if out := expect(0, "is-active", "lima-guestagent.service"); out != "active\n" {
		t.Errorf("is-active printed %q", out)
	}
	want := "--runtime-dir=" + filepath.Join(root, "run") + "/lima-guestagent\n"
	deadline := time.Now().Add(5 * time.Second)
	for {
		got, err := os.ReadFile(marker)
		if err == nil && string(got) == want {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("unit argv not expanded: got %q (%v), want %q", got, err, want)
		}
		time.Sleep(20 * time.Millisecond)
	}

	expect(0, "restart", "lima-guestagent.service")
	expect(0, "stop", "lima-guestagent.service")
	if out := expect(3, "is-active", "lima-guestagent.service"); out != "inactive\n" {
		t.Errorf("is-active after stop printed %q", out)
	}
	out := expect(5, "start", "nosuch.service")
	if !strings.Contains(out, "nosuch.service") || !strings.Contains(out, "lima-guestagent.service") {
		t.Errorf("unknown unit message does not name the unit and the known units: %q", out)
	}
	expect(1, "frobnicate", "lima-guestagent.service")
}

// TestSystemctlRefusesAChangeFromANonAdmin verifies only the admin uid may
// start or stop a unit, while any user may ask whether one is active.
//
// VALIDATES: spec Security Review, Privilege. Method: a supervisor whose
// admin uid is not the test's.
// PREVENTS: the socket, which every user can reach, becoming a way to run
// a unit's command as root.
func TestSystemctlRefusesAChangeFromANonAdmin(t *testing.T) {
	root := t.TempDir()
	sup, err := NewSupervisor(root, uint32(os.Getuid())+1, nil)
	if err != nil {
		t.Fatalf("NewSupervisor: %v", err)
	}
	socket := filepath.Join(root, "s.sock")
	stop, err := sup.Serve(socket)
	if err != nil {
		t.Fatalf("Serve: %v", err)
	}
	defer stop()
	var stdout, stderr bytes.Buffer
	if code := RunShim(socket, []string{"daemon-reload"}, &stdout, &stderr); code != 4 {
		t.Errorf("daemon-reload from a non-admin = %d, want 4 (%q)", code, stderr.String())
	}
	stdout.Reset()
	if code := RunShim(socket, []string{"is-active", "sshd.service"}, &stdout, &stderr); code != 3 {
		t.Errorf("is-active from a non-admin = %d, want 3", code)
	}
}
