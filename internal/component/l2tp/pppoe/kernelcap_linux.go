// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: register_kernelcap_linux.go -- the init() that enrolls what this file builds
// Related: kernel_linux.go -- the AF_PPPOX PX_PROTO_OE socket the probe opens
//
// The access concentrator hands every session to the kernel as an AF_PPPOX
// PX_PROTO_OE socket (CONFIG_PPPOE). That is a capability, so ze doctor names it
// when it is missing and a Docker host that lacks it is refused before the
// PPPoE labs run. It replaces the old /proc/modules lookup, which answered "not
// loaded" for a kernel that builds pppoe in.

//go:build linux

package pppoe

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// pppoeKernelCapability is the one capability the PPPoE subsystem enrolls.
func pppoeKernelCapability() kernelcap.Capability {
	return kernelcap.Capability{
		Subsystem:   "pppoe",
		Component:   "pppoe",
		Kernel:      "CONFIG_PPPOE",
		ConfigLeaf:  "pppoe",
		CodeAbsent:  diagnostic.CodeDoctorPPPoEUnavailable,
		CodeUnknown: diagnostic.CodeDoctorPPPoEUnknown,
		Order:       749,
		InUse:       PPPoEInUse,
		Probe:       pppoeSessionProbe,
	}
}

// PPPoEInUse reports whether the configuration runs the access concentrator.
// ExtractParameters (config.go) starts from Enabled false and sets it only from
// the enabled leaf, so a pppoe block without `enabled true` runs nothing.
func PPPoEInUse(tree *config.Tree) bool {
	if tree == nil {
		return false
	}
	block := tree.GetContainer("pppoe")
	if block == nil {
		return false
	}
	enabled, _ := block.Get("enabled")
	return enabled == "true"
}

func pppoeSessionProbe() kernelcap.Result {
	return kernelcap.PPPoXProtocol(pxProtoOE)
}
