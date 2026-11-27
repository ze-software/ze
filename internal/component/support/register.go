// Design: docs/architecture/core-design.md -- support bundle module registrations
// Related: modules.go -- module registry helpers and selection filters
// Related: runtime_collectors.go -- core live runtime support bundle collectors

package support

import "github.com/ze-software/ze/internal/core/supportreg"

func init() {
	staticModules := []struct {
		name string
		fn   moduleCollector
	}{
		{"version", collectVersion},
		{"doctor", collectDoctor},
		{"host", collectHost},
		{"config", collectConfig},
		{"crashes", collectCrashes},
		{"disk", collectDisk},
		{"interfaces", collectInterfaces},
		{"routes", collectRoutes},
		{"neighbors", collectNeighbors},
		{"env", collectEnv},
		{"sysctl", collectSysctl},
		{"platform", collectPlatform},
		{"runtime", collectRuntime},
		{"dmesg", collectDmesg},
		{"sockets", collectSockets},
		{"modules", collectModules},
		{"conntrack", collectConntrack},
		{"fds", collectFDs},
		{"dns", collectDNS},
		{"firewall", collectFirewall},
	}
	for _, m := range staticModules {
		supportreg.Register(m.name, m.fn)
	}

	runtimeModules := []struct {
		name string
		fn   moduleCollector
	}{
		{"log-ring", collectLogRing},
		{"report", collectReport},
		{"health", collectHealth},
		{"watchdog", collectWatchdog},
		{"bad-packets", collectBadPackets},
		{"drift", collectDrift},
		{"goroutines", collectGoroutines},
	}
	for _, m := range runtimeModules {
		supportreg.Register(m.name, m.fn)
	}
}
