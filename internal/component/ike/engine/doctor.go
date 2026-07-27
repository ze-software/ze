// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- IKE engine readiness checks
// Related: register.go -- Registration.DoctorChecks declaration and resolveInterfaceAddr

package engine

import (
	"net"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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
	if err != nil {
		// Say it. The previous comment here claimed this was "the config
		// system's error to report, not this check's" -- and NO layer reports
		// it. ike's ipsec validation hangs off the SDK OnConfigVerify
		// (register.go), which VerifyPluginConfig deliberately does not run for
		// a live plugin (internal/component/config/plugin_verify.go), and this
		// Registration declares no InProcessConfigVerifier. The measured result
		// was that `vpn { ipsec { interface ze-missing0; esp-group ESP-1 { } } }`
		// reported "ready": true and exit 0 from `ze doctor`, with both the
		// unparseable esp-group AND the missing interface silently gone.
		//
		// A guard that can neither evaluate nor speak does not exist
		// (ai/rules/fail-closed-guards.md). Where the validation ultimately
		// belongs is a separate, larger question; being unable to answer it is
		// not a license to stay quiet here.
		// Error, not warning. ParseIPsecConfig returns an error from exactly one
		// class of cause -- a malformed esp-group or ike-group (ipsec/config.go)
		// -- which is a real config defect, not a benign partial config. A
		// warning would leave `ze doctor` reporting "ready": true over a vpn
		// section that cannot be used, which is the reading this fix exists to
		// remove.
		var tb textbuf.Buffer
		return []rpc.DoctorCheckDiagnostic{{
			Code:     "doctor-ipsec-iface",
			Severity: "error",
			Message:  tb.Str("vpn ipsec config could not be parsed, so its interface reference was not checked: ").Err(err).String(),
		}}
	}
	if cfg == nil {
		return nil
	}
	if cfg.Interface == "" {
		// An ABSENT interface leaf is nothing to check; an EMPTY one is not the
		// same thing. `interface ""` passes `ze config validate` and resolves to
		// "" at runtime, which is the silent no-establish failure this check
		// exists to catch -- and the kernel's own dev_valid_name rejects the
		// empty name explicitly. Distinguishing them is the whole point: a zero
		// value must not read as "nothing was asked for".
		ipsecRoot := tree.GetContainerPath("vpn/ipsec")
		if ipsecRoot == nil {
			return nil
		}
		if _, present := ipsecRoot.Get("interface"); !present {
			return nil
		}
		var tb textbuf.Buffer
		return []rpc.DoctorCheckDiagnostic{{
			Code:     "doctor-ipsec-iface",
			Severity: "error",
			Message:  tb.Str("vpn ipsec interface is set to the empty name; IPsec SAs will not bind to any interface").String(),
		}}
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
