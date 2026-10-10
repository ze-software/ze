// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: apparmor.go -- the registry this profile joins
// Related: register_apparmor_netsysctl.go -- its registration
//
// Docker starts a container with /proc/sys read-only, and docker-default denies
// every write under it but kernel/. A sysctl a peer sets once, before its
// daemon starts, is a `--sysctl` argument the runtime writes into the
// container's own network namespace, and needs neither. A sysctl written while
// the peer runs (a knob on an interface the peer created, a value a checker cuts
// and restores) needs both lifted, so such a peer runs with systempaths
// unconfined and under this profile: docker-default with /proc/sys writes denied
// everywhere but net/, whose entries in a container's own network namespace
// change nothing outside it. One profile serves every lab (owner, 2026-10-10:
// "make it generic").

package interoplab

import (
	"context"
	"strings"

	"github.com/ze-software/ze/internal/component/kernelcap"
)

// NetSysctlAppArmorProfileName is the profile a peer that writes its network
// namespace's sysctls at run time runs under.
const NetSysctlAppArmorProfileName = "ze-lab-net-sysctl"

// netSysctlProcSysKeep is what the profile admits under /proc/sys.
const netSysctlProcSysKeep = "net/"

// NetSysctlWriteArguments answers the docker run arguments that make /proc/sys
// writable for a peer running under NetSysctlAppArmorProfileName. They MUST go
// together: without the profile the writes are confined by nothing on an
// AppArmor host but docker-default, which denies them.
func NetSysctlWriteArguments() []string {
	return []string{"--security-opt", "systempaths=unconfined"}
}

// NetSysctlSecurityArguments answers every docker run argument a container
// that writes its network namespace's sysctls at run time takes on this
// daemon: NetSysctlWriteArguments, then the NetSysctlAppArmorProfileName
// option on a daemon that applies AppArmor. It refuses, naming the load
// command, a host whose kernel lists its profiles without this one. On a
// daemon that applies no AppArmor nothing confines the writes to net/, so
// container root can write any host sysctl whose handler checks only
// ownership (R-3 of spec-lab-containers-least-privilege); --privileged
// lifts that and every other limit.
func (d *Docker) NetSysctlSecurityArguments(ctx context.Context) ([]string, error) {
	profile, err := d.appArmorSecurityOption(ctx, NetSysctlAppArmorProfileName)
	if err != nil {
		return nil, err
	}
	return append(NetSysctlWriteArguments(), profile...), nil
}

// NetSysctlGranted reports whether the profile admits a write of key, spelled
// as sysctl spells it (net.ipv4.ip_forward).
func NetSysctlGranted(key string) bool {
	return strings.HasPrefix(strings.ReplaceAll(key, ".", "/"), netSysctlProcSysKeep)
}

// netSysctlAppArmorProfile is docker-default with its /proc/sys write rules
// narrowed to netSysctlProcSysKeep, and docker-default's `deny mount,` kept.
func netSysctlAppArmorProfile() string {
	return kernelcap.DockerDefaultProfile(NetSysctlAppArmorProfileName,
		kernelcap.ProcSysWriteDenies(netSysctlProcSysKeep), "  deny mount,\n")
}
