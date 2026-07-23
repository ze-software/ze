// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- IKE engine readiness checks
// Related: register.go -- Registration.DoctorChecks declaration and resolveInterfaceAddr

package engine

import (
	"net"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/ipsec"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// interfaceByName is a seam so the check can be driven without touching the
// host's real interface table. Mirrors internal/component/doctor/checks_listener.go.
var interfaceByName = net.InterfaceByName

// ifNameSize is the kernel's IFNAMSIZ: 16 bytes including the NUL terminator,
// so 15 usable characters.
const ifNameSize = 16

// checkIPsecInterface reports a `vpn ipsec interface` leaf that names an
// interface absent from the system.
//
// This is a readiness check rather than a config-verify rejection because
// interface existence is a property of the HOST, not of the config being
// judged: a config-first deployment legitimately names an interface that the
// same commit is about to create, and the ike plugin's ConfigRoots
// ({"vpn","pki"}, register.go) do not even carry the interfaces section.
// The precedent is checkDHCPInterfaces
// (internal/component/doctor/checks_listener.go), which treats
// service/dhcp-server/listen-interface exactly this way.
//
// The consequence of getting this wrong is quiet: resolveInterfaceAddr
// (register.go) returns "" for an unknown interface, so every peer without an
// explicit local-address silently fails to establish, with only a debug-level
// warning about a missing IPv4 address to explain it.
func checkIPsecInterface(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	cfg, err := ipsec.ParseIPsecConfig(tree)
	if err != nil || cfg == nil || cfg.Interface == "" {
		// A config that does not parse is the config system's error to report,
		// not this check's; an absent interface leaf is simply nothing to check.
		return nil
	}

	var tb textbuf.Buffer

	// Reject names the kernel itself cannot hold, so they are reported as
	// malformed rather than as a confusing "not found". This is a message-quality
	// guard, NOT a security boundary: net.InterfaceByName does an RTM_GETLINK
	// netlink dump and a string compare -- there is no filesystem or resolver
	// behind it to traverse (ai/rules/no-fabrication.md: do not claim a safety
	// property the code does not provide).
	//
	// Mirrors the kernel's dev_valid_name: "." and ".." exactly, anything with
	// '/', NUL or whitespace, and anything at or over IFNAMSIZ (16 with the NUL,
	// so 15 usable). A name like "br..0" is LEGAL and must reach the resolver.
	if invalidInterfaceName(cfg.Interface) {
		return []rpc.DoctorCheckDiagnostic{{
			Code:     "doctor-ipsec-iface",
			Severity: "error",
			Message:  tb.Str("vpn ipsec interface has an invalid name: ").Str(cfg.Interface).String(),
		}}
	}

	if err := cfg.ValidateInterfaceRef(ipsecInterfaceExists); err != nil {
		return []rpc.DoctorCheckDiagnostic{{
			Code:     "doctor-ipsec-iface",
			Severity: "error",
			Message: tb.Str("vpn ipsec interface not found: ").Str(cfg.Interface).
				Str(" (peers without an explicit local-address will not establish)").String(),
		}}
	}
	return nil
}

// invalidInterfaceName reports whether the kernel could never hold this name,
// mirroring dev_valid_name (net/core/dev.c).
func invalidInterfaceName(name string) bool {
	if name == "." || name == ".." {
		return true
	}
	if len(name) >= ifNameSize {
		return true
	}
	return strings.ContainsAny(name, "/ \t\n\v\f\r\x00")
}

// ipsecInterfaceExists reports whether the named interface exists on this host.
func ipsecInterfaceExists(name string) bool {
	_, err := interfaceByName(name)
	return err == nil
}
