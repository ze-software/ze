// Design: docs/architecture/cli/plugin-modes.md — ze systemd unit file generation

package systemd

import "github.com/ze-software/ze/internal/core/textbuf"

const (
	serviceName     = "ze.service"
	serviceUser     = "ze"
	serviceGroup    = "ze"
	defaultUnitPath = "/etc/systemd/system/ze.service"
	capabilities    = "CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE"
)

type unitSpec struct {
	BinaryPath string
	ConfigDir  string
}

func buildUnitFile(spec unitSpec) string {
	var b textbuf.Buffer
	b.Grow(945 + len(spec.BinaryPath) + len(spec.ConfigDir)*2)
	b.Str("[Unit]\n")
	b.Str("Description=Ze Network OS\n")
	b.Str("After=network-online.target\n")
	b.Str("Wants=network-online.target\n")
	b.Byte('\n')
	b.Str("[Service]\n")
	b.Str("Type=simple\n")
	b.Str("User=ze\n")
	b.Str("Group=ze\n")
	b.Str("ExecStart=")
	b.Str(spec.BinaryPath)
	b.Str(" start\n")
	b.Str("ExecReload=/bin/kill -HUP $MAINPID\n")
	b.Str("Restart=on-failure\n")
	b.Str("RestartSec=5\n")
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
