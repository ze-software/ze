// Design: docs/architecture/doctor-and-health-checks.md -- scoped transport readiness
// Related: transport_scope.go -- physical interface aliases require a readable table

package reactor

import (
	"net"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

const (
	codeTransportScopeUnknown = "doctor-bgp-transport-scope-unknown"
	transportScopeSubject     = "interface-zones"
	transportScopeFix         = "ze show interface brief"
)

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        codeTransportScopeUnknown,
		Title:       "BGP TCP interface zones could not be resolved",
		Description: "Ze could not read the host interface names and indices. BGP refuses to equate differently spelled TCP interface zones until that read succeeds, so a numeric zone and its interface name may not match. Inspect the host interface access failure and the available interfaces with ze show interface brief.",
		Examples:    []string{transportScopeFix, "ze show diagnostic " + codeTransportScopeUnknown},
	}); err != nil {
		panic("BUG: BGP transport scope diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeTransportScopeUnknown}); err != nil {
		panic("BUG: BGP transport scope event registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name: "bgp-transport-scopes", Phase: diagnostic.DoctorPhasePostConfig,
		Order: 730, Component: reportSourceBGP,
		Dependencies: []string{"network", "config-tree"}, Platforms: []string{diagnostic.DoctorPlatformAny},
		Codes: []string{codeTransportScopeUnknown}, Check: doctorTransportScopes,
	}); err != nil {
		panic("BUG: BGP transport scope doctor registration failed: " + err.Error())
	}
}

func doctorTransportScopes(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	if ctx.Tree == nil {
		return nil
	}
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok {
		panic("BUG: BGP transport scope doctor context holds no config tree")
	}
	if tree.GetContainer("bgp") == nil {
		return nil
	}
	if _, err := net.Interfaces(); err != nil {
		return []diagnostic.Diagnostic{transportScopeDiagnostic(err)}
	}
	return nil
}

func transportScopeDiagnostic(cause error) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Code: codeTransportScopeUnknown, Severity: diagnostic.SeverityWarning,
		Message: "BGP cannot read the host interface names and indices, so differently spelled TCP interface zones cannot be matched: " + cause.Error(),
		Fix:     transportScopeFix,
	}
}
