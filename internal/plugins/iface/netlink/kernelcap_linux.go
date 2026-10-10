// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: register_kernelcap_linux.go -- the init() that enrolls what this file builds
// Related: wireguard_linux.go -- the WireGuard device the family serves
// Related: xfrm_linux.go -- the xfrm link the interface probe asks for
//
// Two interface kinds this backend creates need a kernel piece no other check
// asks about: WireGuard (CONFIG_WIREGUARD, configured through its generic
// netlink family) and the xfrm interface (CONFIG_XFRM_INTERFACE, an rtnetlink
// link kind). Each is a capability, so ze doctor names the missing one and a
// Docker host that lacks one is refused before a lab runs.
//
// The xfrm interface has no read-only question. rtnetlink answers ENODEV to a
// RTM_NEWLINK without NLM_F_CREATE before it looks at the kind
// (net/core/rtnetlink.c, __rtnl_newlink), and it exposes no list of kinds. With
// NLM_F_CREATE it resolves the kind, loading its module on demand, and answers
// EOPNOTSUPP "Unknown device type" when no kind matches. xfrmi_newlink
// (net/xfrm/xfrm_interface_core.c) then refuses interface id 0 with EINVAL
// "if_id must be non zero" before registering anything, so on a kernel that
// has the kind nothing is created. Kernels before that check would create the
// link, so the request runs inside a throwaway namespace
// (kernelcap.InThrowawayNetworkNamespace), and whatever the kernel does there
// disappears with it. Creating that namespace needs CAP_SYS_ADMIN; without it
// the probe answers unknown and says so.

//go:build linux

package ifacenetlink

import (
	"errors"
	"fmt"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

const (
	kernelcapComponent = "iface"
	wireguardFamily    = "wireguard"
	xfrmProbeLinkName  = "zeprobe0"
)

// xfrmInterfaceProbeAdd sends the RTM_NEWLINK from inside the throwaway
// namespace. Tests replace it and MUST restore it.
var xfrmInterfaceProbeAdd = func(handle *netlink.Handle) error {
	return handle.LinkAdd(&netlink.Xfrmi{LinkAttrs: netlink.LinkAttrs{Name: xfrmProbeLinkName}})
}

// ifaceKernelCapabilities returns the two capabilities this backend enrolls.
func ifaceKernelCapabilities() []kernelcap.Capability {
	return []kernelcap.Capability{
		{
			Subsystem:   "wireguard",
			Component:   kernelcapComponent,
			Kernel:      "CONFIG_WIREGUARD",
			ConfigLeaf:  "interface wireguard",
			CodeAbsent:  diagnostic.CodeDoctorWireGuardUnavailable,
			CodeUnknown: diagnostic.CodeDoctorWireGuardUnknown,
			Order:       751,
			InUse:       interfaceKindInUse("wireguard"),
			Probe:       wireguardProbe,
		},
		{
			Subsystem:   "xfrm-interface",
			Component:   kernelcapComponent,
			Kernel:      "CONFIG_XFRM_INTERFACE",
			ConfigLeaf:  "interface xfrm",
			CodeAbsent:  diagnostic.CodeDoctorXFRMInterfaceUnavailable,
			CodeUnknown: diagnostic.CodeDoctorXFRMInterfaceUnknown,
			Order:       752,
			InUse:       interfaceKindInUse("xfrm"),
			Probe:       xfrmInterfaceProbe,
		},
	}
}

// interfaceKindInUse answers whether the configuration asks THIS backend for at
// least one interface of the list kind. A vpp backend creates its interfaces in
// VPP, so the kernel is not asked; the backend leaf defaults to netlink
// (ze-iface-conf.yang).
func interfaceKindInUse(kind string) func(*config.Tree) bool {
	return func(tree *config.Tree) bool {
		if tree == nil {
			return false
		}
		block := tree.GetContainer("interface")
		if block == nil {
			return false
		}
		backend, set := block.Get("backend")
		if set && backend != backendName {
			return false
		}
		return len(block.GetList(kind)) > 0
	}
}

func wireguardProbe() kernelcap.Result {
	return kernelcap.GenericNetlinkFamily(wireguardFamily)
}

// xfrmInterfaceProbe sends the link request from a throwaway namespace, so a
// kernel that creates the link creates it there and nowhere else.
func xfrmInterfaceProbe() kernelcap.Result {
	return kernelcap.InThrowawayNetworkNamespace(askXFRMInterface)
}

// askXFRMInterface runs inside the throwaway namespace of xfrmInterfaceProbe.
func askXFRMInterface() kernelcap.Result {
	handle, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		return kernelcap.Result{State: kernelcap.StateUnknown, Reason: fmt.Errorf("open rtnetlink in the probe namespace: %w", err)}
	}
	defer handle.Close()

	return classifyXFRMInterfaceProbe(xfrmInterfaceProbeAdd(handle))
}

// classifyXFRMInterfaceProbe maps the RTM_NEWLINK answer to a verdict. EINVAL is
// xfrmi_newlink refusing interface id 0, which it reaches only after the kind
// resolved; success is an older kernel creating the link in the throwaway
// namespace. EOPNOTSUPP is rtnetlink's "Unknown device type". Anything else, an
// EPERM for one, says nothing about the kernel.
func classifyXFRMInterfaceProbe(err error) kernelcap.Result {
	if err == nil {
		return kernelcap.Result{State: kernelcap.StatePresent}
	}
	if errors.Is(err, unix.EINVAL) {
		return kernelcap.Result{State: kernelcap.StatePresent}
	}
	if errors.Is(err, unix.EOPNOTSUPP) {
		return kernelcap.Result{State: kernelcap.StateAbsent, Reason: err}
	}
	return kernelcap.Result{State: kernelcap.StateUnknown, Reason: err}
}
