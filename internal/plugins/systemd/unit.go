// Design: docs/architecture/cli/plugin-modes.md — ze systemd unit file generation

package systemd

import "strings"

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
	var b strings.Builder
	b.Grow(945 + len(spec.BinaryPath) + len(spec.ConfigDir)*2)
	b.WriteString("[Unit]\n")
	b.WriteString("Description=Ze Network OS\n")
	b.WriteString("After=network-online.target\n")
	b.WriteString("Wants=network-online.target\n")
	b.WriteByte('\n')
	b.WriteString("[Service]\n")
	b.WriteString("Type=simple\n")
	b.WriteString("User=ze\n")
	b.WriteString("Group=ze\n")
	b.WriteString("ExecStart=")
	b.WriteString(spec.BinaryPath)
	b.WriteString(" start\n")
	b.WriteString("ExecReload=/bin/kill -HUP $MAINPID\n")
	b.WriteString("Restart=on-failure\n")
	b.WriteString("RestartSec=5\n")
	b.WriteString("LimitNOFILE=65536\n")
	b.WriteString("LimitCORE=infinity\n")
	// The memlock plugin locks the whole ze executable in memory, and the entire
	// mapped size is charged against RLIMIT_MEMLOCK. The systemd default of 8 MiB
	// is smaller than the binary, so the lock would fail under it.
	b.WriteString("LimitMEMLOCK=infinity\n")
	b.WriteString("WorkingDirectory=")
	b.WriteString(spec.ConfigDir)
	b.WriteByte('\n')
	b.WriteString("Environment=ZE_CONFIG_DIR=")
	b.WriteString(spec.ConfigDir)
	b.WriteByte('\n')
	b.WriteString("Environment=XDG_RUNTIME_DIR=/run/ze\n")
	b.WriteString("AmbientCapabilities=")
	b.WriteString(capabilities)
	b.WriteByte('\n')
	b.WriteString("CapabilityBoundingSet=")
	b.WriteString(capabilities)
	b.WriteByte('\n')
	b.WriteString("# Host tuning may need CAP_SYS_NICE. Future VRF/netns support may need CAP_SYS_ADMIN.\n")
	b.WriteString("# Add those capabilities only when the configured feature requires them.\n")
	b.WriteString("NoNewPrivileges=true\n")
	b.WriteString("ProtectSystem=true\n")
	b.WriteString("ProtectHome=true\n")
	b.WriteString("RuntimeDirectory=ze\n")
	b.WriteByte('\n')
	b.WriteString("[Install]\n")
	b.WriteString("WantedBy=multi-user.target\n")
	return b.String()
}
