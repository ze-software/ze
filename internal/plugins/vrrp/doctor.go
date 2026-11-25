// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- doctor codes + config-sanity check
//
// The codes live here, not in internal/core/diagnostic's builtin list, so that
// deleting this plugin removes its diagnostics with it
// (ai/rules/repo-maintenance.md, ai/rules/plugins.md). The
// transport owns doctor-vrrp-raw-socket; the iface kernel backend owns
// doctor-iface-macvlan. This file owns only what the CONFIG can be wrong about.
//
// The check re-runs the plugin's own verifier rather than re-implementing the
// rules: one rule set means `ze doctor` and a commit can never disagree.
package vrrp

import (
	"encoding/json"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

const codeVRRPConfigInvalid = "doctor-vrrp-config-invalid"

// vrrpDiagnosticCodes is the explanation metadata for the codes this plugin
// owns. Registered from register.go (init-time registration is confined to
// that file, ai/patterns/registration.md).
var vrrpDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeVRRPConfigInvalid,
		Title:       "VRRP configuration is not usable",
		Description: "A VRRP group fails a cross-leaf rule that per-leaf YANG constraints cannot express: an advertise interval the configured version cannot encode on the wire, accept-mode combined with version 2, an IPv6 group whose first virtual-address is not link-local, one virtual address claimed by two groups on the same unit, or track configured on the group that owns its virtual address, whose priority is fixed at 255. The affected group does not run; correct the reported group.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-vrrp-config-invalid"},
	},
}

// checkVRRPConfigSanity is the doctor check function.
func checkVRRPConfigSanity(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	ifaceTree := tree.GetContainer(configRoot)
	if ifaceTree == nil {
		return nil
	}
	data, err := json.Marshal(map[string]any{configRoot: ifaceTree.ToMap()})
	if err != nil {
		return nil
	}
	return diagnoseSections([]configSection{{Root: configRoot, Data: string(data)}})
}

// diagnoseSections is the check's rule path: extract, then validate, reporting
// the first failure with the code whose remediation actually fits.
func diagnoseSections(sections []configSection) []diagnostic.Diagnostic {
	specs, err := extractGroupSpecs(sections)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     codeVRRPConfigInvalid,
			Severity: diagnostic.SeverityError,
			Message:  err.Error(),
		}}
	}
	// No groups: the plugin auto-loads with the shared `interface` root, so
	// staying silent here is what keeps `ze doctor` quiet on the vast majority
	// of deployments that never configure VRRP.
	if len(specs) == 0 {
		return nil
	}

	verr := validateGroups(specs)
	if verr == nil {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     codeVRRPConfigInvalid,
		Severity: diagnostic.SeverityError,
		Message:  verr.Error(),
		Fix:      "ze config validate",
	}}
}
