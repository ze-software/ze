// Design: docs/features/ai-first.md -- the readiness check this component owns
// Related: validate_semantic.go -- ValidateSemantics, the validator the check runs
//
// The check lived in internal/component/doctor as checkSemanticValidation and
// the runner reached it by writing its name out. It returns ValidateSemantics
// verbatim, so it belongs to the package that owns that validator
// (ai/patterns/registration.md, "Doctor Check Registry"), and it declares the
// config-* codes the validator really emits: a doctor- alias for them would be
// a second code for one fact.

package config

import (
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// doctorComponent is the name this package registers its doctor checks under.
const doctorComponent = "config"

// semanticsDoctorCheck is the registration init() below installs.
//
// Order 100 reproduces the sequence the doctor runner printed before the check
// moved onto the registry: it was the first post-config check, ahead of the
// BGP peer check at 110 (internal/component/bgp/config/doctor_checks.go).
func semanticsDoctorCheck() diagnostic.DoctorCheck {
	return diagnostic.DoctorCheck{
		Name:         "config-semantics",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        100,
		Component:    doctorComponent,
		Dependencies: []string{"config-tree"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes: []string{
			diagnostic.CodeConfigMCPInvalid,
			diagnostic.CodeConfigGNMIInvalid,
			diagnostic.CodeConfigPluginVerify,
			diagnostic.CodeConfigHubInvalid,
		},
		Check: checkSemantics,
	}
}

// checkSemantics runs ValidateSemantics over the loaded config tree.
//
// A nil tree is the missing-config phase, which this check does not run in, and
// a context carrying anything else is a runner defect the runner's own type
// assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkSemantics(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*Tree)
	if !ok || tree == nil {
		return nil
	}
	return ValidateSemantics(tree)
}

func init() {
	// A refusal is a programmer error in the declaration above, and none of
	// its causes can be reached from a config or a peer, so it stops the
	// process rather than leaving `ze doctor` quietly short of a check.
	if err := diagnostic.RegisterDoctorCheck(semanticsDoctorCheck()); err != nil {
		var tb textbuf.Buffer
		tb.Str("config: doctor check registration failed: ").Err(err).Byte('\n')
		tb.StdErr() //nolint:errcheck // the process is exiting on the next line
		os.Exit(1)
	}
}
