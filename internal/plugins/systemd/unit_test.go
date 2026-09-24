package systemd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
