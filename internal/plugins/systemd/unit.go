// Design: docs/architecture/cli/plugin-modes.md — ze systemd unit file generation

package systemd

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	serviceName     = "ze.service"
	serviceUser     = "ze"
	serviceGroup    = "ze"
	defaultUnitPath = "/etc/systemd/system/ze.service"
	capabilities    = "CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE"
)

// stopTimeoutSeconds is the TimeoutStopSec every ze unit needs: the drain
// ceiling plus the teardown margin the daemon is held to (pkg/plugin/rpc,
// drain.go), so systemd never kills ze while it drains or tears down. It is
// derived, never copied: buildUnitFile writes it and checkServiceStop
// (doctor.go) refuses an effective value below it.
const stopTimeoutSeconds = rpc.DrainCeilingSeconds + rpc.DrainTeardownMarginSeconds

// stopKillMode is the KillMode every ze unit needs. systemd's default,
// control-group, sends the stop SIGTERM to every process in the unit's cgroup,
// external plugins included, so each one would die at the start of the drain
// that should reach it. mixed sends SIGTERM to ze alone and keeps the SIGKILL
// at the timeout for the whole cgroup.
const stopKillMode = "mixed"

type unitSpec struct {
	BinaryPath string
	ConfigDir  string
}

// execStartShell is the shell that starts ze with SIGHUP ignored. ExecReload
// runs it too, so the unit needs no program beyond it and ze.
const execStartShell = "/bin/sh"

func buildUnitFile(spec unitSpec) string {
	var b textbuf.Buffer
	b.Grow(978 + len(spec.BinaryPath) + len(spec.ConfigDir)*2)
	b.Str("[Unit]\n")
	b.Str("Description=Ze Network OS\n")
	b.Str("After=network-online.target\n")
	b.Str("Wants=network-online.target\n")
	b.Byte('\n')
	b.Str("[Service]\n")
	b.Str("Type=simple\n")
	b.Str("User=ze\n")
	b.Str("Group=ze\n")
	// ze starts with SIGHUP ignored. Go keeps an inherited SIG_IGN for SIGHUP
	// until signal.Notify registers the signal, and ze registers it first thing
	// in main. Without this, a `systemctl reload` that lands during Go package
	// initialization, before any handler can exist, kills the daemon. systemd has
	// no directive that sets a signal to SIG_IGN (IgnoreSIGPIPE= covers SIGPIPE
	// only), so the shell sets it and execs ze, which keeps the main PID.
	// execStartExecutable (doctor.go) reads this shape back.
	b.Str("ExecStart=")
	b.Str(execStartShell)
	b.Str(" -c 'trap \"\" HUP; exec ")
	b.Str(spec.BinaryPath)
	b.Str(" start'\n")
	// The reload uses the shell's builtin kill. /bin/kill comes from procps,
	// which a minimal host (Debian trixie slim) does not install, and systemd
	// then refuses the unit's reload ("Command /bin/kill is not executable").
	// `ze signal reload` is no substitute: it reloads over SSH, so it needs the
	// SSH listener and login credentials, where SIGHUP needs neither. systemd sets
	// MAINPID in the reload's environment, and exec keeps ze as the main PID.
	b.Str("ExecReload=")
	b.Str(execStartShell)
	b.Str(" -c 'kill -HUP $MAINPID'\n")
	b.Str("Restart=on-failure\n")
	b.Str("RestartSec=5\n")
	// The stop drain runs between the SIGTERM and the teardown, so the unit
	// gives ze the whole ceiling plus the margin before the SIGKILL, and
	// signals ze alone so ze can drain its external plugins itself.
	b.Str("TimeoutStopSec=")
	b.Int(stopTimeoutSeconds)
	b.Byte('\n')
	b.Str("KillMode=")
	b.Str(stopKillMode)
	b.Byte('\n')
	b.Str("LimitNOFILE=65536\n")
	b.Str("LimitCORE=infinity\n")
	// The memlock plugin locks the whole ze executable in memory, and the entire
	// mapped size is charged against RLIMIT_MEMLOCK. The systemd default of 8 MiB
	// is smaller than the binary, so the lock would fail under it.
	b.Str("LimitMEMLOCK=infinity\n")
	b.Str("WorkingDirectory=")
	b.Str(spec.ConfigDir)
	b.Byte('\n')
	b.Str("Environment=ZE_CONFIG_DIR=")
	b.Str(spec.ConfigDir)
	b.Byte('\n')
	b.Str("Environment=XDG_RUNTIME_DIR=/run/ze\n")
	b.Str("AmbientCapabilities=")
	b.Str(capabilities)
	b.Byte('\n')
	b.Str("CapabilityBoundingSet=")
	b.Str(capabilities)
	b.Byte('\n')
	b.Str("# Host tuning may need CAP_SYS_NICE. Future VRF/netns support may need CAP_SYS_ADMIN.\n")
	b.Str("# Add those capabilities only when the configured feature requires them.\n")
	b.Str("NoNewPrivileges=true\n")
	b.Str("ProtectSystem=true\n")
	b.Str("ProtectHome=true\n")
	b.Str("RuntimeDirectory=ze\n")
	b.Byte('\n')
	b.Str("[Install]\n")
	b.Str("WantedBy=multi-user.target\n")
	return b.String()
}
