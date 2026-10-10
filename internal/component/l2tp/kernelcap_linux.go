// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: register_kernelcap_linux.go -- the init() that enrolls what this file builds
// Related: genl_linux.go -- the generic netlink family the control probe asks for
// Related: pppox_linux.go -- the PPPoL2TP socket the session probe opens
//
// L2TP needs two kernel pieces: the l2tp generic netlink family that creates
// tunnels and sessions (CONFIG_L2TP), and the PPPoL2TP socket a PPP session
// rides on (CONFIG_PPPOL2TP). Each is a capability, so ze doctor names the one
// that is missing and a Docker host that lacks one is refused before the L2TP
// labs run. Both replace the old /proc/modules lookup, which answered "not
// loaded" for a kernel that builds them in and for one that has not yet loaded
// a module it can load on demand.

//go:build linux

package l2tp

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

const kernelcapComponent = "l2tp"

// l2tpKernelCapabilities returns the two capabilities the L2TP subsystem enrolls.
func l2tpKernelCapabilities() []kernelcap.Capability {
	return []kernelcap.Capability{
		{
			Subsystem:   "l2tp",
			Component:   kernelcapComponent,
			Kernel:      "CONFIG_L2TP",
			ConfigLeaf:  "l2tp",
			CodeAbsent:  diagnostic.CodeDoctorL2TPUnavailable,
			CodeUnknown: diagnostic.CodeDoctorL2TPUnknown,
			Order:       747,
			InUse:       L2TPInUse,
			Probe:       l2tpControlProbe,
		},
		{
			Subsystem:   "l2tp-ppp",
			Component:   kernelcapComponent,
			Kernel:      "CONFIG_PPPOL2TP",
			ConfigLeaf:  "l2tp",
			CodeAbsent:  diagnostic.CodeDoctorL2TPPPPUnavailable,
			CodeUnknown: diagnostic.CodeDoctorL2TPPPPUnknown,
			Order:       748,
			InUse:       L2TPInUse,
			Probe:       l2tpSessionProbe,
		},
	}
}

// L2TPInUse reports whether the configuration runs the L2TP subsystem: an l2tp
// block whose enabled leaf is not false. It is the reading ParseParameters
// (config.go) makes, where the block's presence implies enabled.
func L2TPInUse(tree *config.Tree) bool {
	if tree == nil {
		return false
	}
	block := tree.GetContainer("l2tp")
	if block == nil {
		return false
	}
	enabled, set := block.Get("enabled")
	if !set {
		return true
	}
	return enabled == configTrue
}

func l2tpControlProbe() kernelcap.Result {
	return kernelcap.GenericNetlinkFamily(genlL2TPName)
}

func l2tpSessionProbe() kernelcap.Result {
	return kernelcap.PPPoXProtocol(pxProtoOL2TP)
}
