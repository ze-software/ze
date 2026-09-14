//go:build linux

// Design: docs/architecture/core-design.md -- the readiness check the nft backend owns
// Related: register_linux.go -- the init() that installs the registration below
// Related: backend_linux.go -- the nftables programming that needs the module
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. This backend is the one that programs nf_tables, so
// the module's presence is its question (ai/patterns/registration.md, "Doctor
// Check Registry"): it is owned here now and dropping this backend drops its
// check with it.

package firewallnft

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// codeFirewallNftables names a firewall config on this backend whose kernel
// has not loaded nf_tables. internal/core/diagnostic/codes.go declares it, so
// `ze explain doctor-firewall-nftables` answers.
const codeFirewallNftables = "doctor-firewall-nftables"

// nftModule is the kernel module this backend programs through.
const nftModule = "nf_tables"

// nftLoadedModules is the module-list reader checkFirewallNftables runs. It is
// a variable so a test stands in a kernel without the module; nothing else
// assigns it.
var nftLoadedModules = kernelcap.LoadedModules

// nftDoctorCheck is the registration register_linux.go installs.
//
// Order 160 reproduces the sequence the doctor runner printed before the check
// moved onto the registry: it ran before the runner's registry dispatch, after
// the kernel-modules check (150, internal/component/doctor/doctor_checks.go)
// and before the kernel-nexthop check (170, internal/plugins/fib/kernel).
var nftDoctorCheck = diagnostic.DoctorCheck{
	Name:         "firewall-nftables",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        160,
	Component:    "firewall-nft",
	Dependencies: []string{"kernel"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeFirewallNftables},
	Check:        checkFirewallNftables,
}

// checkFirewallNftables warns when the config carries a firewall block on this
// backend, named or by default, and the kernel has not loaded nf_tables.
//
// A nil tree is the missing-config phase, which this check does not run in, and
// a context carrying anything else is a runner defect the runner's own type
// assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkFirewallNftables(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	firewall := tree.GetContainer("firewall")
	if firewall == nil {
		return nil
	}
	backend, _ := firewall.Get("backend")
	if backend != "" && backend != "nft" {
		return nil
	}
	if nftLoadedModules()[nftModule] {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     codeFirewallNftables,
		Severity: diagnostic.SeverityWarning,
		Message:  "firewall: nf_tables kernel module not loaded",
	}}
}
