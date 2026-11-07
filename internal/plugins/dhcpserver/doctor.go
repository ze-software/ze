// Design: docs/architecture/provisioning/dhcp-server.md -- the readiness check this plugin owns
// Overview: register.go -- the init() that installs the registration below
// Related: handler.go -- the listeners startListeners binds on the same interfaces
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. A check that asks whether each `listen-interface`
// exists belongs to the plugin that binds a socket on each one
// (ai/patterns/registration.md, "Doctor Check Registry"), so it is owned here
// now and dropping this plugin drops its check with it.

package dhcpserver

import (
	"net"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codeDHCPIface names a listen interface the server cannot bind: one whose
// name is not a device name, or one the host does not hold.
// internal/core/diagnostic/codes.go declares it, so `ze show diagnostic
// doctor-dhcp-iface` answers.
const codeDHCPIface = "doctor-dhcp-iface"

// codeDHCPSegmentAddress names a listen interface whose device holds no IPv4
// address in any configured subnet, so it answers no client that a relay did
// not forward. internal/core/diagnostic/codes.go declares it.
const codeDHCPSegmentAddress = "doctor-dhcp-segment-address"

// listenInterfacePath is the config path every diagnostic below names.
const listenInterfacePath = "service/dhcp-server/listen-interface"

// dhcpInterfaceByName is the probe checkDHCPInterfaces runs. It is a variable
// so a test can stand in a host without the device; nothing else assigns it.
var dhcpInterfaceByName = net.InterfaceByName

// dhcpDoctorCheck is the registration register.go installs.
//
// Order 140 reproduces the sequence the doctor runner printed before the check
// moved onto the registry: it ran before the runner's registry dispatch, after
// the interface checks (130, internal/component/iface) and before the
// kernel-modules check (150, internal/component/doctor/doctor_checks.go).
var dhcpDoctorCheck = diagnostic.DoctorCheck{
	Name:         "dhcp-listen-interface",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        140,
	Component:    "dhcpserver",
	Dependencies: []string{"network-device"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeDHCPIface, codeDHCPSegmentAddress},
	Check:        checkDHCPInterfaces,
}

// checkDHCPInterfaces reports, for an enabled server, each listen interface
// whose name cannot be a device name, each one whose selector names no single
// device, and each one whose device the host does not hold. An invalid name or
// a missing device is an error: startListeners cannot bind either.
//
// A nil tree is the missing-config phase, which this check does not run in, and
// a context carrying anything else is a runner defect the runner's own type
// assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkDHCPInterfaces(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	dhcp := tree.GetContainerPath(configRootService + "/dhcp-server")
	if dhcp == nil {
		return nil
	}
	if enabled, _ := dhcp.Get("enabled"); enabled != leafTrue {
		return nil
	}

	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer
	subnets := configuredSubnets(dhcp)
	for _, name := range dhcp.GetSlice("listen-interface") {
		if strings.ContainsAny(name, "/\x00") || strings.Contains(name, "..") {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     codeDHCPIface,
				Severity: diagnostic.SeverityError,
				Message:  tb.Reset().Str("DHCP server listen interface has invalid name: ").Str(name).String(),
				Path:     listenInterfacePath,
			})
			continue
		}
		// The name is a logical interface an os-name or mac match selector can
		// bind to another kernel device. The check runs in the CLI process,
		// where the daemon's resolver table is empty, so the device comes from
		// the selector in the config tree, with the severity the iface check
		// gives the same selector: unmatched is a deferred binding, a warning;
		// ambiguous is refused by the apply, an error.
		device, selector := iface.SelectedNetDevice(tree, name)
		if selector != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     codeDHCPIface,
				Severity: selector.Severity,
				Message: tb.Reset().Str("DHCP server listen interface ").Str(name).
					Str(" has no single device to bind: ").Str(selector.Message).String(),
				Path: listenInterfacePath,
				Help: "make the os-name or mac match selector of interface " + name +
					" name exactly one present device, or remove " + name + " from service dhcp-server listen-interface",
			})
			continue
		}
		if _, err := dhcpInterfaceByName(device); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     codeDHCPIface,
				Severity: diagnostic.SeverityError,
				Message:  dhcpNotFound(&tb, name, device),
				Path:     listenInterfacePath,
			})
			continue
		}
		if diag, unserved := checkDHCPSegmentAddress(&tb, name, device, subnets); unserved {
			diags = append(diags, diag)
		}
	}
	return diags
}

// configuredSubnets answers the subnet prefixes of every shared network of
// the dhcp-server tree. A key that does not parse is skipped: the server's
// verifier refuses it at commit, so no committed tree holds one.
func configuredSubnets(dhcp *config.Tree) []netip.Prefix {
	var subnets []netip.Prefix
	for _, network := range dhcp.GetList("shared-network") {
		for key := range network.GetList("subnet") {
			prefix, err := netip.ParsePrefix(key)
			if err != nil {
				continue
			}
			subnets = append(subnets, prefix)
		}
	}
	return subnets
}

// checkDHCPSegmentAddress reports a listen interface whose device holds no
// IPv4 address in any configured subnet, or whose addresses cannot be read.
// The server answers a client that no relay forwarded only from the subnets
// the receiving device is addressed in (resolveSegment), so such an interface
// serves relayed clients alone. It is a warning: a relay-only interface is a
// valid design.
func checkDHCPSegmentAddress(tb *textbuf.Buffer, name, device string, subnets []netip.Prefix) (diagnostic.Diagnostic, bool) {
	// A server with no subnet serves no client whatever addresses its
	// interfaces hold, so there is no segment address to miss.
	if len(subnets) == 0 {
		return diagnostic.Diagnostic{}, false
	}
	help := "add an address in a configured subnet to interface " + name +
		", or remove " + name + " from service dhcp-server listen-interface"
	addrs, err := dhcpDeviceAddrs(device)
	if err != nil {
		return diagnostic.Diagnostic{
			Code:     codeDHCPSegmentAddress,
			Severity: diagnostic.SeverityWarning,
			Message: tb.Reset().Str("DHCP server listen interface ").Str(name).
				Str(": its addresses could not be read, so whether it holds one in a configured subnet is unknown: ").
				Str(err.Error()).String(),
			Path: listenInterfacePath,
			Help: help,
		}, true
	}
	for _, addr := range addrs {
		for _, prefix := range subnets {
			if prefix.Contains(addr) {
				return diagnostic.Diagnostic{}, false
			}
		}
	}
	tb.Reset().Str("DHCP server listen interface ").Str(name).
		Str(" holds no IPv4 address in any configured subnet (addresses:")
	for _, addr := range addrs {
		tb.Str(" ").Str(addr.String())
	}
	if len(addrs) == 0 {
		tb.Str(" none")
	}
	tb.Str("), so it answers only relayed clients")
	return diagnostic.Diagnostic{
		Code:     codeDHCPSegmentAddress,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.String(),
		Path:     listenInterfacePath,
		Help:     help,
	}, true
}

// dhcpNotFound words the report for a listen interface whose kernel device the
// host does not hold, naming the device too when a selector chose another.
func dhcpNotFound(tb *textbuf.Buffer, name, device string) string {
	tb.Reset().Str("DHCP server listen interface not found: ").Str(name)
	if device != name {
		tb.Str(" (kernel device ").Str(device).Str(")")
	}
	return tb.String()
}
