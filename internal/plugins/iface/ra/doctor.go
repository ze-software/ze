// Design: ai/rules/repo-maintenance.md -- self-contained doctor checks owned by
// the package that owns the runtime dependency.
// Related: doctor_linux.go -- reads net.ipv6.conf.<device>.forwarding, all.forwarding and force_forwarding
// Related: doctor_other.go -- the non-Linux stub, which knows no forwarding state
//
// A router that advertises itself tells hosts to send it their off-link
// traffic. With IPv6 forwarding off on that device, or with the kernel's
// global switch off, the kernel drops that traffic, so every host on the link autoconfigures, installs a default route,
// and loses connectivity, with nothing in the daemon output saying why.
//
// Config verify cannot catch this: forwarding can arrive from a sysctl profile
// or from global config after verify runs, so rejecting the configuration would
// be wrong (design decision D-6). Ze reports the state instead of changing it,
// because a silent kernel change outside declared config breaks the sysctl
// model.

package ifacera

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// interfaceKinds are the interface list names that carry units, and so can
// carry a router-advertisement container.
var interfaceKinds = []string{"ethernet", "veth", "bridge", "dummy", "tunnel", "wireguard", "xfrm"}

// ipv6ForwardingReader reports whether IPv6 forwarding is on for a device, and
// whether the state could be read at all. A test replaces it; the platform
// implementations are doctor_linux.go and doctor_other.go.
var ipv6ForwardingReader = readIPv6Forwarding

// ipv6KernelForwardingReader reports whether Linux forwards an IPv6 packet
// that arrives on a device, and whether that could be read. Per-device
// forwarding alone routes nothing: ip6_forward (net/ipv6/ip6_output.c) drops
// the packet unless net.ipv6.conf.all.forwarding is 1 or the ingress device's
// force_forwarding is 1. A test replaces it.
var ipv6KernelForwardingReader = readIPv6KernelForwarding

// raForwardingDoctorCheck is the registration, installed from register.go.
var raForwardingDoctorCheck = diagnostic.DoctorCheck{
	Name:         "iface-ra-forwarding",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        743,
	Component:    "iface",
	Dependencies: []string{"ipv6-forwarding"},
	// The check runs anywhere; on a platform with no forwarding sysctl the
	// reader says "unknown" and the check stays silent.
	Platforms: []string{diagnostic.DoctorPlatformAny},
	Codes:     []string{"doctor-iface-ra-forwarding"},
	Check:     checkRAForwarding,
}

// checkRAForwarding reports one warning per interface that advertises while
// IPv6 forwarding is off on its device. It says nothing when the state cannot
// be read, because an unreadable sysctl is not evidence of a problem.
func checkRAForwarding(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	ifaceBlock := tree.GetContainer("interface")
	if ifaceBlock == nil {
		return nil
	}
	// The router-advertisement container is netlink-only, so a vpp tree holds
	// no sender whose forwarding state would matter.
	if backend, _ := ifaceBlock.Get("backend"); backend == "vpp" {
		return nil
	}

	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer
	for _, kind := range interfaceKinds {
		entries := ifaceBlock.GetList(kind)
		names := ifaceBlock.ListKeys(kind)
		// A map gives no order, so the reported interfaces are sorted and two
		// runs of `ze doctor` read the same.
		slices.Sort(names)
		for _, name := range names {
			if !advertises(entries[name]) {
				continue
			}
			// The sysctl is keyed by the OS device, which is not the logical
			// name whenever os-name aliases it.
			device := osDeviceOf(name, entries[name])
			// Forwarding is a property of the device, so several advertising
			// units on one interface share one warning.
			on, known := ipv6ForwardingReader(device)
			if !known {
				continue
			}
			if !on {
				diags = append(diags, diagnostic.Diagnostic{
					Code:     "doctor-iface-ra-forwarding",
					Severity: diagnostic.SeverityWarning,
					Message: tb.Reset().
						Str("interface ").Str(name).
						Str(" sends Router Advertisements while net.ipv6.conf.").Str(device).
						Str(".forwarding is 0; hosts will install a default route through Ze and their off-link traffic will be dropped").
						String(),
				})
				continue
			}
			routes, routesKnown := ipv6KernelForwardingReader(device)
			if !routesKnown {
				continue
			}
			if routes {
				continue
			}
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-iface-ra-forwarding",
				Severity: diagnostic.SeverityWarning,
				Message: tb.Reset().
					Str("interface ").Str(name).
					Str(" sends Router Advertisements while net.ipv6.conf.all.forwarding is 0 and net.ipv6.conf.").Str(device).
					Str(".force_forwarding is not 1, so Linux forwards no IPv6 packet and hosts' off-link traffic is dropped; ").
					Str("set ipv6 forwarding true on the unit, and remove any sysctl setting that holds net.ipv6.conf.all.forwarding at 0 (ze show sysctl names it)").
					String(),
			})
		}
	}
	return diags
}

// osDeviceOf returns the kernel device a logical interface name binds to,
// reading the same os-name leaf the resolver reads (osDeviceFor,
// internal/component/iface/resolve.go): the leaf when it is set, the logical
// name otherwise.
//
// The resolver's other selector, mac/match, names no device in the
// configuration and cannot be answered without a running backend. An interface
// bound that way keeps its logical name here, readIPv6Forwarding finds no such
// path, and the check stays silent rather than naming a device it did not read.
func osDeviceOf(name string, entry *config.Tree) string {
	if entry == nil {
		return name
	}
	if osName, ok := entry.Get("os-name"); ok && osName != "" {
		return osName
	}
	return name
}

// advertises reports whether any unit of one interface enables Router
// Advertisements.
func advertises(entry *config.Tree) bool {
	if entry == nil {
		return false
	}
	for _, unit := range entry.GetList("unit") {
		if unit == nil {
			continue
		}
		ipv6 := unit.GetContainer("ipv6")
		if ipv6 == nil {
			continue
		}
		ra := ipv6.GetContainer("router-advertisement")
		if ra == nil {
			continue
		}
		if enabled, _ := ra.Get("enabled"); enabled == "true" {
			return true
		}
	}
	return false
}
