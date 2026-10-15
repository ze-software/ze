package systemd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestUnitFileContent(t *testing.T) {
	// VALIDATES: AC-8 generated unit file contains correct ExecStart and systemd targets.
	// PREVENTS: shipping an unusable service unit with the wrong daemon command or boot target.
	unit := buildUnitFile(unitSpec{BinaryPath: "/usr/local/bin/ze", ConfigDir: "/etc/ze"})

	assertContains(t, unit, "[Unit]")
	assertContains(t, unit, "Description=Ze Network OS")
	assertContains(t, unit, "After=network-online.target")
	assertContains(t, unit, "Wants=network-online.target")
	assertContains(t, unit, `ExecStart=/bin/sh -c 'trap "" HUP; exec /usr/local/bin/ze start'`)
	assertContains(t, unit, "WorkingDirectory=/etc/ze")
	assertContains(t, unit, "Environment=ZE_CONFIG_DIR=/etc/ze")
	assertContains(t, unit, "WantedBy=multi-user.target")
}

func TestUnitFileCustomConfig(t *testing.T) {
	// VALIDATES: AC-6 --config override appears in WorkingDirectory and ZE_CONFIG_DIR.
	// PREVENTS: silently ignoring the operator-selected config directory.
	unit := buildUnitFile(unitSpec{BinaryPath: "/opt/ze/bin/ze", ConfigDir: "/custom/path"})

	assertContains(t, unit, "WorkingDirectory=/custom/path")
	assertContains(t, unit, "Environment=ZE_CONFIG_DIR=/custom/path")
}

func TestUnitFileCapabilities(t *testing.T) {
	// VALIDATES: AC-10 generated unit file runs as ze with required network capabilities.
	// PREVENTS: accidentally running the service as root or without BGP/network privileges.
	unit := buildUnitFile(unitSpec{BinaryPath: "/usr/bin/ze", ConfigDir: "/etc/ze"})

	assertContains(t, unit, "User=ze")
	assertContains(t, unit, "Group=ze")
	assertContains(t, unit, "AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE")
	assertContains(t, unit, "CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE")
}

func TestUnitFileHardening(t *testing.T) {
	// VALIDATES: AC-10/AC-13 generated unit file includes hardening and runtime directory setup.
	// PREVENTS: regressions that remove the service sandbox or /run/ze ownership setup.
	unit := buildUnitFile(unitSpec{BinaryPath: "/usr/bin/ze", ConfigDir: "/etc/ze"})

	assertContains(t, unit, "NoNewPrivileges=true")
	assertContains(t, unit, "ProtectSystem=true")
	assertContains(t, unit, "ProtectHome=true")
	assertContains(t, unit, "RuntimeDirectory=ze")
	assertContains(t, unit, "Restart=on-failure")
	assertContains(t, unit, "RestartSec=5")
}

// TestUnitFileMemoryLock verifies the unit lifts RLIMIT_MEMLOCK.
//
// VALIDATES: the generated unit lets the memlock plugin lock the whole ze
// executable, which the systemd default of 8 MiB is far too small for.
// PREVENTS: a daemon whose pages the kernel can evict under memory pressure,
// reported only as a doctor warning nobody reads.
func TestUnitFileMemoryLock(t *testing.T) {
	unit := buildUnitFile(unitSpec{BinaryPath: "/usr/bin/ze", ConfigDir: "/etc/ze"})

	assertContains(t, unit, "LimitMEMLOCK=infinity")
}

func TestUnitFileRuntimeDir(t *testing.T) {
	// VALIDATES: AC-13 generated unit file puts sockets and runtime files under /run/ze.
	// PREVENTS: daemon sockets falling back to /tmp/ze.socket under the ze user.
	unit := buildUnitFile(unitSpec{BinaryPath: "/usr/bin/ze", ConfigDir: "/etc/ze"})

	assertContains(t, unit, "Environment=XDG_RUNTIME_DIR=/run/ze")
	assertContains(t, unit, "RuntimeDirectory=ze")
}

func TestUnitFileNoPIDFile(t *testing.T) {
	// VALIDATES: AC-14 generated unit file uses Type=simple and has no PIDFile directive.
	// PREVENTS: adding a PIDFile line that systemd ignores for Type=simple.
	unit := buildUnitFile(unitSpec{BinaryPath: "/usr/bin/ze", ConfigDir: "/etc/ze"})

	assertContains(t, unit, "Type=simple")
	if strings.Contains(unit, "PIDFile=") {
		t.Fatalf("unit file must not contain PIDFile directive:\n%s", unit)
	}
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("expected %q in:\n%s", want, got)
	}
}

// TestUnitFileStartsZeWithSIGHUPIgnored runs the generated ExecStart command
// line, with a probe standing in for ze, and sends the probe SIGHUP.
//
// VALIDATES: the process ExecStart execs inherits SIGHUP as ignored, which Go
// keeps until signal.Notify registers it.
// PREVENTS: a `systemctl reload` during Go package initialization killing ze.
// DISCRIMINATES: drop the `trap "" HUP` from buildUnitFile and the probe dies
// on its own SIGHUP before it prints.
func TestUnitFileStartsZeWithSIGHUPIgnored(t *testing.T) {
	probe := filepath.Join(t.TempDir(), "ze")
	if err := os.WriteFile(probe, []byte("#!/bin/sh\nkill -HUP $$\necho alive \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	unit := buildUnitFile(unitSpec{BinaryPath: probe, ConfigDir: "/etc/ze"})
	var execStart string
	for line := range strings.SplitSeq(unit, "\n") {
		if value, ok := strings.CutPrefix(line, "ExecStart="); ok {
			execStart = value
		}
	}
	if execStart == "" {
		t.Fatalf("unit has no ExecStart:\n%s", unit)
	}
	// The line uses only the quoting systemd and sh share, so sh reads it as
	// systemd does.
	out, err := exec.CommandContext(t.Context(), "/bin/sh", "-c", execStart).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "alive start" {
		t.Fatalf("ExecStart %q: err=%v output=%q, want the probe to survive SIGHUP", execStart, err, out)
	}
}

// TestExecStartExecutableReadsTheInstalledWrapper feeds the doctor's unit
// reader the ExecStart line buildUnitFile writes.
//
// VALIDATES: the doctor judges the ze binary the shell execs.
// PREVENTS: the doctor checking /bin/sh and passing a unit whose ze is gone.
func TestExecStartExecutableReadsTheInstalledWrapper(t *testing.T) {
	unit := parseServiceUnit([]byte(buildUnitFile(unitSpec{BinaryPath: "/opt/ze/bin/ze", ConfigDir: "/etc/ze"})))
	if unit.execStart != "/opt/ze/bin/ze" {
		t.Fatalf("execStart = %q, want /opt/ze/bin/ze", unit.execStart)
	}
	if got := execStartExecutable("/usr/bin/ze start"); got != "/usr/bin/ze" {
		t.Fatalf("plain ExecStart = %q, want /usr/bin/ze", got)
	}
}

// TestBuildUnitFileStopTimeoutDerivedFromCeiling reads the stop settings back
// out of the generated unit's [Service] section and compares them with the
// drain constants, not with a copied number.
//
// VALIDATES: spec-daemon-drain-before-stop AC-15: TimeoutStopSec is the drain
// ceiling plus the teardown margin, and KillMode is mixed, so systemd signals
// ze alone at stop and kills nothing while ze drains and tears down.
// PREVENTS: a unit whose stop timeout drifts from the ceiling the daemon
// enforces, and a control-group kill that SIGTERMs every external plugin at the
// start of the drain.
func TestBuildUnitFileStopTimeoutDerivedFromCeiling(t *testing.T) {
	unit := buildUnitFile(unitSpec{BinaryPath: "/usr/bin/ze", ConfigDir: "/etc/ze"})

	settings := map[string]string{}
	inService := false
	for line := range strings.SplitSeq(unit, "\n") {
		if strings.HasPrefix(line, "[") {
			inService = line == "[Service]"
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if inService && ok {
			settings[key] = value
		}
	}

	want := strconv.Itoa(rpc.DrainCeilingSeconds + rpc.DrainTeardownMarginSeconds)
	if got := settings["TimeoutStopSec"]; got != want {
		t.Errorf("[Service] TimeoutStopSec = %q, want %q (drain ceiling %d s + teardown margin %d s)",
			got, want, rpc.DrainCeilingSeconds, rpc.DrainTeardownMarginSeconds)
	}
	if got := settings["KillMode"]; got != "mixed" {
		t.Errorf("[Service] KillMode = %q, want mixed", got)
	}
}

// TestUnitFileReloadRunsOnlyTheShell reads the ExecReload line buildUnitFile
// writes, checks it names no program beyond the shell ExecStart already runs,
// then runs it with MAINPID set to a stand-in process.
//
// VALIDATES: `systemctl reload ze` sends SIGHUP to the main PID through the
// shell's builtin kill, so the unit runs on a host without procps.
// PREVENTS: ExecReload=/bin/kill, which systemd 257 refuses on a Debian slim
// host ("Command /bin/kill is not executable") so reload never runs.
// DISCRIMINATES: put `/bin/kill -HUP $MAINPID` back and the executable check
// fails; drop the `-HUP` and the stand-in dies of SIGTERM instead.
func TestUnitFileReloadRunsOnlyTheShell(t *testing.T) {
	unit := buildUnitFile(unitSpec{BinaryPath: "/usr/bin/ze", ConfigDir: "/etc/ze"})
	if got := parseServiceUnit([]byte(unit)).execReload; len(got) != 1 || got[0] != execStartShell {
		t.Fatalf("ExecReload executables = %q, want only %s, the shell ExecStart runs:\n%s", got, execStartShell, unit)
	}

	var execReload string
	for line := range strings.SplitSeq(unit, "\n") {
		if value, ok := strings.CutPrefix(line, "ExecReload="); ok {
			execReload = value
		}
	}
	standIn := exec.CommandContext(t.Context(), "/bin/sleep", "30")
	if err := standIn.Start(); err != nil {
		t.Fatal(err)
	}
	// systemd passes MAINPID in the environment of every ExecReload command,
	// and the line uses only the quoting systemd and sh share.
	reload := exec.CommandContext(t.Context(), "/bin/sh", "-c", execReload)
	reload.Env = append(os.Environ(), "MAINPID="+strconv.Itoa(standIn.Process.Pid))
	if out, err := reload.CombinedOutput(); err != nil {
		t.Fatalf("ExecReload %q: err=%v output=%q", execReload, err, out)
	}
	err := standIn.Wait()
	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		t.Fatalf("stand-in ended with %v, want death by SIGHUP", err)
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGHUP {
		t.Fatalf("stand-in ended with %v, want death by SIGHUP", err)
	}
}
