// Design: docs/architecture/resolve.md — the RIR delegation table and its sources
// Overview: doctor.go — runChecks calls checkRIRDelegationSources
// Related: internal/component/config/validators.go — ValidateFetchURL, the rule this reports
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

package doctor

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// diagnosticRIRSourceRefused marks a configured delegation source that the
// fetch rule refuses. It is declared in internal/core/diagnostic/codes.go, so
// `ze explain doctor-rir-source-refused` answers.
const diagnosticRIRSourceRefused = "doctor-rir-source-refused"

// checkRIRDelegationSources judges every configured delegation source against
// the rule the refresh applies. This is the entry point runChecks calls.
func checkRIRDelegationSources(tree *config.Tree) []diagnostic.Diagnostic {
	if tree == nil {
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
			Code:     diagnosticRIRSourceRefused,
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
