// Design: ai/rules/repo-maintenance.md -- self-contained doctor checks owned by
// the package that owns the runtime dependency.
// Related: doctor_linux.go -- reads net.ipv6.conf.<device>.forwarding
// Related: doctor_other.go -- the non-Linux stub, which knows no forwarding state
//
// A router that advertises itself tells hosts to send it their off-link
// traffic. With IPv6 forwarding off on that device the kernel drops that
// traffic, so every host on the link autoconfigures, installs a default route,
// and loses connectivity, with nothing in the daemon output saying why.
//
// Config verify cannot catch this: forwarding can arrive from a sysctl profile
// or from global config after verify runs, so rejecting the configuration would
// be wrong (design decision D-6). Ze reports the state instead of changing it,
// because a silent kernel change outside declared config breaks the sysctl
// model.

package ifacera

import (
	"sort"

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
		sort.Strings(names)
		for _, name := range names {
			if !advertises(entries[name]) {
				continue
			}
			// Forwarding is a property of the device, so several advertising
			// units on one interface share one warning.
			on, known := ipv6ForwardingReader(name)
			if !known || on {
				continue
			}
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-iface-ra-forwarding",
				Severity: diagnostic.SeverityWarning,
				Message: tb.Reset().
					Str("interface ").Str(name).
					Str(" sends Router Advertisements while net.ipv6.conf.").Str(name).
					Str(".forwarding is 0; hosts will install a default route through Ze and their off-link traffic will be dropped").
					String(),
			})
		}
	}
	return diags
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
