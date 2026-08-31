package systemd

import (
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
	assertContains(t, unit, "ExecStart=/usr/local/bin/ze start")
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
