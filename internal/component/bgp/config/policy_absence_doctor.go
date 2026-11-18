// Design: docs/architecture/doctor-and-health-checks.md -- saved BGP policy absence.
// Related: register.go -- the same resolved peer walk as config validate.
package bgpconfig

import "github.com/ze-software/ze/internal/core/diagnostic"

// This config-phase check reads the supplied saved tree, never a live session.
// It MUST clone that shared tree because the peer resolver prunes inactive nodes.
func doctorCheckBGPPolicyAbsences(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree := doctorTree(ctx)
	if tree == nil || tree.GetContainer("bgp") == nil {
		return nil
	}
	absences, err := policyAbsencesFromTree(tree.Clone())
	if err != nil {
		return nil
	} // The existing peer-config check reports refusal.
	var findings []diagnostic.Diagnostic
	for _, absence := range absences {
		findings = append(findings, diagnostic.Diagnostic{Code: absence.Code, Severity: diagnostic.SeverityWarning,
			Message: absence.Owner + ": " + absence.Message, Fix: absence.Fix})
	}
	return findings
}
