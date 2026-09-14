// Design: docs/architecture/resolve.md -- the RIR delegation table and its sources
// Overview: register_rir.go -- the init() that installs the registration below
// Related: rir.go -- handleRIRRefresh, the refresh that refuses the source this check reports
// Related: internal/component/config/validators.go -- ValidateFetchURL, the rule this reports
//
// RIR delegation source check: report a configured mirror `update resolve rir`
// will refuse to read.
//
// The leaf's own ze:validate refuses such a URL at commit time, so a running
// config carrying one arrived another way: a file written by an older binary,
// a restore from a backup taken before the rule existed, or an edit made
// outside the editor. The refusal then waits until an operator runs the
// refresh, which is the moment they least want to discover it.
//
// The check reads the config and judges the URL. It fetches nothing: a doctor
// run must not reach five registries, and an unreachable mirror is a fact
// about the network at that second rather than about the configuration.
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. The refresh that applies the rule lives here, so the
// check does too (ai/patterns/registration.md, "Doctor Check Registry"), and
// dropping this package drops its check with it.

package cmd

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codeRIRSourceRefused marks a configured delegation source that the fetch
// rule refuses. It is declared in internal/core/diagnostic/codes.go, so
// `ze explain doctor-rir-source-refused` answers.
const codeRIRSourceRefused = "doctor-rir-source-refused"

// rirSourcesDoctorCheck describes the check. register_rir.go registers it.
//
// Order 2320 reproduces the sequence the doctor runner printed before the
// check moved onto the registry: it was the last check the runner called, so
// it sorts last in the 2000 band that internal/component/doctor/doctor_checks.go
// reserves for the checks that ran after the runner's own phase dispatch.
var rirSourcesDoctorCheck = diagnostic.DoctorCheck{
	Name:         "rir-delegation-sources",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2320,
	Component:    "resolve",
	Dependencies: []string{"config-tree"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeRIRSourceRefused},
	Check:        checkRIRDelegationSources,
}

// checkRIRDelegationSources judges every configured delegation source against
// the rule the refresh applies.
//
// A nil tree is the missing-config phase, which this check does not run in, and
// a context carrying anything else is a runner defect the runner's own type
// assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkRIRDelegationSources(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}

	sources := system.ExtractSystemConfig(tree).RIRDelegationSources
	if len(sources) == 0 {
		return nil
	}

	var diags []diagnostic.Diagnostic
	for _, registry := range sortedRegistries(sources) {
		err := config.ValidateFetchURL(sources[registry])
		if err == nil {
			continue
		}
		var tb textbuf.Buffer
		diags = append(diags, diagnostic.Diagnostic{
			Code:     codeRIRSourceRefused,
			Severity: diagnostic.SeverityWarning,
			Message: tb.Str("the ").Str(registry).Str(" delegation source will not be read: ").
				Err(err).Str("; `update resolve rir` refuses it and the table stays as it is").String(),
		})
	}
	return diags
}

// sortedRegistries answers the registry tokens of a source map in a stable
// order, so two doctor runs over one config report the same list.
func sortedRegistries(sources map[string]string) []string {
	registries := make([]string, 0, len(sources))
	for registry := range sources {
		registries = append(registries, registry)
	}
	slices.Sort(registries)
	return registries
}
