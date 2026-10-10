// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Overview: probe.go -- the shared /proc root and the test override
// Related: probe_linux.go -- the XFRM and MPLS probes beside these
//
// Two probes several owners share: a generic netlink family (L2TP control,
// WireGuard) and a PPPoX socket protocol (PPPoL2TP, PPPoE). Each asks the kernel
// the question the subsystem itself asks first, so a modular kernel loads the
// module on the way, exactly as it would for the subsystem, and nothing is
// created that outlives the call.

//go:build linux

package kernelcap

import (
	"errors"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// afPPPOX is the PPP over X socket family (include/linux/socket.h, AF_PPPOX).
const afPPPOX = 24

// genlFamilyLookup asks the generic netlink controller for a family. Tests
// replace it and MUST restore it.
var genlFamilyLookup = func(name string) error {
	_, err := netlink.GenlFamilyGet(name)
	return err
}

// pppoxSocketOpen opens and closes one PPPoX socket. Tests replace it and MUST
// restore it.
var pppoxSocketOpen = func(protocol int) error {
	fd, err := unix.Socket(afPPPOX, unix.SOCK_DGRAM, protocol)
	if err != nil {
		return err
	}
	return unix.Close(fd)
}

// GenericNetlinkFamily reports whether the kernel registers the generic netlink
// family name. CTRL_CMD_GETFAMILY needs no privilege, and on a modular kernel the
// controller requests the module that aliases the family before it answers.
func GenericNetlinkFamily(name string) Result {
	return classifyGenericNetlinkFamily(genlFamilyLookup(name))
}

// classifyGenericNetlinkFamily maps the controller's answer to a verdict. ENOENT
// is the controller's "no such family" after its module request, so it is the
// one absent answer; anything else says nothing about the family.
func classifyGenericNetlinkFamily(err error) Result {
	if err == nil {
		return Result{State: StatePresent}
	}
	if errors.Is(err, unix.ENOENT) {
		return Result{State: StateAbsent, Reason: err}
	}
	return Result{State: StateUnknown, Reason: err}
}

// PPPoXProtocol reports whether the kernel can open an AF_PPPOX socket of the
// given protocol. pppox_create (drivers/net/ppp/pppox.c) requests the
// protocol's module before it answers, and neither the PPPoE nor the PPPoL2TP
// create path checks a privilege, so the open is an unprivileged, complete
// answer. The socket is closed before it is bound, so nothing outlives the call.
func PPPoXProtocol(protocol int) Result {
	return classifyPPPoXProtocol(pppoxSocketOpen(protocol))
}

// classifyPPPoXProtocol maps the socket answer to a verdict. EAFNOSUPPORT is a
// kernel with no AF_PPPOX at all (CONFIG_PPPOX), EPROTONOSUPPORT one whose
// AF_PPPOX has no handler for the protocol; both are absent. Anything else, a
// seccomp EPERM for one, says nothing about the kernel.
func classifyPPPoXProtocol(err error) Result {
	if err == nil {
		return Result{State: StatePresent}
	}
	if errors.Is(err, unix.EAFNOSUPPORT) {
		return Result{State: StateAbsent, Reason: err}
	}
	if errors.Is(err, unix.EPROTONOSUPPORT) {
		return Result{State: StateAbsent, Reason: err}
	}
	return Result{State: StateUnknown, Reason: err}
}
