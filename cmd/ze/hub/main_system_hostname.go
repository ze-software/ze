// Design: docs/guide/appliance.md -- the kernel host name follows system host on the appliance
// Related: main_system.go -- the other system appliers, called beside this one
// Related: main_system_appliance.go, main_system_nonappliance.go -- kernelHostnameOwned per build

package hub

import (
	"log/slog"
	"os"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// kernelHostnameOps reads and sets the kernel host name. Tests stand in a
// kernel; the daemon uses realKernelHostnameOps.
type kernelHostnameOps struct {
	get func() (string, error)
	set func(name string) error
}

var realKernelHostnameOps = kernelHostnameOps{get: os.Hostname, set: setKernelHostname}

// applyKernelHostname sets the kernel host name from `system host` at start.
// Best-effort like the other system appliers: a failure is one warn line and
// a `ze doctor` finding, never a refused start.
func applyKernelHostname(tree *zeconfig.Tree) {
	name, configured := system.ConfiguredHostName(tree)
	reconcileKernelHostname(name, configured, realKernelHostnameOps, slogutil.Logger("system"))
}

// applyKernelHostnameFromMap is applyKernelHostname on the reload path, after
// the reload is accepted, so a failure never refuses the reload.
func applyKernelHostnameFromMap(tree map[string]any) {
	name, configured := system.ConfiguredHostNameFromMap(tree)
	reconcileKernelHostname(name, configured, realKernelHostnameOps, slogutil.Logger("system"))
}

// reconcileKernelHostname sets the kernel host name to name when this build
// owns it, the leaf is set, and the kernel carries another name. An unset leaf
// keeps the image's name. It returns nothing: the caller has already accepted
// the config, and a failure leaves the previous name with one warn line.
func reconcileKernelHostname(name string, configured bool, ops kernelHostnameOps, log *slog.Logger) {
	// On a distro the operating system owns the host name (systemd-hostnamed),
	// so only the appliance build sets it.
	if !kernelHostnameOwned {
		return
	}
	if !configured {
		return
	}
	// Validation refused an invalid value at commit; an unset $ENV_VAR that
	// left the literal reference reaches here and is not a host name.
	if err := system.ValidateHostName(name); err != nil {
		log.Warn("kernel host name not set: system host is not a valid host name",
			"host", name, "error", err, "fix", "set system host to letters, digits and hyphens, for example cpe-lab")
		return
	}
	current, err := ops.get()
	if err != nil {
		log.Warn("kernel host name not set: it cannot be read", "host", name, "error", err,
			"fix", "check that /proc/sys/kernel/hostname is readable, then reload the config")
		return
	}
	if current == name {
		return
	}
	if err := ops.set(name); err != nil {
		log.Warn("kernel host name not set: sethostname refused system host",
			"host", name, "kernel-hostname", current, "error", err,
			"fix", "ze needs CAP_SYS_ADMIN to set the host name: run ze as root on the appliance (leave ze.user unset), then reload")
		return
	}
	log.Info("kernel host name set from system host", "host", name, "previous", current)
}
