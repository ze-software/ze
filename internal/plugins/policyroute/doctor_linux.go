//go:build linux

// Design: docs/architecture/policyroute/policy-routing.md -- the readiness check this plugin owns
// Related: register_linux.go -- the init() that installs the registration below
// Related: rules_linux.go -- newRuleManager, which opens the same route netlink handle
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. Every ip rule this plugin installs goes through a
// NETLINK_ROUTE handle, so a handle that cannot be opened is its question
// (ai/patterns/registration.md, "Doctor Check Registry"): it is owned here now
// and dropping this plugin drops its check with it. Linux-only because the
// handle is.

package policyroute

import (
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codePolicyRouteNetlink names a route netlink handle this process cannot
// open. internal/core/diagnostic/codes.go declares it, so `ze explain
// doctor-policyroute-netlink` answers.
const codePolicyRouteNetlink = "doctor-policyroute-netlink"

// doctorNetlinkFailEnv forces the probe to fail, so a functional test reaches
// the diagnostic on a host whose netlink is healthy. It keeps the doctor
// spelling because the doctor tier's fixtures set it.
const doctorNetlinkFailEnv = "ze.test.doctor.netlink-fail"

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorNetlinkFailEnv,
	Type:        "bool",
	Description: "Force route netlink doctor probe failure (test infrastructure)",
	Private:     true,
})

// routeNetlinkHandle is what the probe opens and the check closes.
type routeNetlinkHandle interface {
	Close()
}

// openRouteNetlink is the probe checkPolicyRouteNetlink runs. It is a variable
// so a test stands in a kernel that refuses the handle; nothing else assigns
// it.
var openRouteNetlink = func() (routeNetlinkHandle, error) {
	return netlink.NewHandle(unix.NETLINK_ROUTE)
}

// policyRouteDoctorCheck is the registration register_linux.go installs.
//
// Order 2080 reproduces the sequence the doctor runner printed before the check
// moved onto the registry: after the conntrack check (2070,
// internal/component/config/system) and before the clock-skew check (2100,
// internal/component/doctor/doctor_checks.go).
var policyRouteDoctorCheck = diagnostic.DoctorCheck{
	Name:         "policyroute-netlink",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2080,
	Component:    "policy-routes",
	Dependencies: []string{"netlink"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codePolicyRouteNetlink},
	Check:        checkPolicyRouteNetlink,
}

// checkPolicyRouteNetlink warns when the config carries a policy route and a
// route netlink handle cannot be opened.
//
// A nil tree is the missing-config phase, which this check does not run in, and
// a context carrying anything else is a runner defect the runner's own type
// assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkPolicyRouteNetlink(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	policy := tree.GetContainer(configRoot)
	if policy == nil || len(policy.GetList("route")) == 0 {
		return nil
	}
	if env.IsEnabled(doctorNetlinkFailEnv) {
		return []diagnostic.Diagnostic{{
			Code:     codePolicyRouteNetlink,
			Severity: diagnostic.SeverityWarning,
			Message:  "policy route: route netlink unavailable: forced failure",
		}}
	}
	h, err := openRouteNetlink()
	if err != nil {
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     codePolicyRouteNetlink,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("policy route: route netlink unavailable: ").Err(err).String(),
		}}
	}
	if h != nil {
		h.Close()
	}
	return nil
}
