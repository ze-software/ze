// Design: docs/architecture/resolve.md -- the RIR delegation source readiness check tests
// Detail: doctor_rir.go -- checkRIRDelegationSources and its registration
//
// These cases arrived from internal/component/doctor with the check they
// drive, case for case.

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// rirSourceTree parses a config naming one delegation source.
func rirSourceTree(t *testing.T, url string) *config.Tree {
	t.Helper()

	tree, err := config.ParseTreeForValidation("system {\n\trir {\n\t\tdelegation-source ripencc { url \"" + url + "\"; }\n\t}\n}\n")
	require.NoError(t, err, "parse the config")
	return tree
}

func rirSourceDiags(t *testing.T, tree *config.Tree) []diagnostic.Diagnostic {
	t.Helper()
	return checkRIRDelegationSources(diagnostic.DoctorCheckContext{Tree: tree})
}

// VALIDATES: the doctor reports a configured delegation source that the fetch
// rule refuses, and stays silent on one it accepts.
// PREVENTS: a mirror an operator can no longer commit but a running config
// still carries, whose refusal would otherwise wait until the day they run
// `update resolve rir` and need it to work.
func TestDoctorReportsADelegationSourceTheRefreshWillRefuse(t *testing.T) {
	diags := rirSourceDiags(t, rirSourceTree(t, "http://mirror.example.com/delegated-ripencc-extended-latest"))
	require.Len(t, diags, 1, "one refused source, one diagnostic")
	assert.Equal(t, codeRIRSourceRefused, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "ripencc", "the message names the registry")

	assert.Empty(t, rirSourceDiags(t, rirSourceTree(t, "https://mirror.example.com/delegated-ripencc-extended-latest")),
		"an HTTPS mirror is read, so it is not reported")
	assert.Empty(t, rirSourceDiags(t, rirSourceTree(t, "http://127.0.0.1:8080/delegated-ripencc-extended-latest")),
		"a mirror on the router itself is read over plain HTTP, so it is not reported")
}

// VALIDATES: a config naming no delegation source produces no diagnostic, and
// neither does an absent tree.
// PREVENTS: a warning on every daemon that never configured a mirror, which is
// the shape that teaches operators to ignore doctor output.
func TestDoctorSaysNothingWhenNoDelegationSourceIsConfigured(t *testing.T) {
	tree, err := config.ParseTreeForValidation("system {\n\thost router1;\n}\n")
	require.NoError(t, err, "parse the config")

	assert.Empty(t, rirSourceDiags(t, tree))
	assert.Empty(t, checkRIRDelegationSources(diagnostic.DoctorCheckContext{}), "nil tree")
}

// VALIDATES: the init() in register_rir.go installed the check, the registry
// accepted it at the phase and order the declaration states, and the code it
// emits is registered so `ze explain` answers for it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green, and a diagnostic an operator cannot look up.
func TestRIRSourcesDoctorCheckRegistered(t *testing.T) {
	want := rirSourcesDoctorCheck
	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(want.Phase)
	for i := range checks {
		if checks[i].Name == want.Name {
			found = &checks[i]
			break
		}
	}
	require.NotNil(t, found, "doctor check %q is not registered for phase %q", want.Name, want.Phase)
	assert.Equal(t, want.Order, found.Order)
	assert.Equal(t, want.Component, found.Component)

	diagnostic.RegisterBuiltinCodes()
	meta := diagnostic.Lookup(codeRIRSourceRefused)
	require.NotNil(t, meta, "the code reaches ze explain")
	assert.NotEmpty(t, meta.Title)
	assert.NotEmpty(t, meta.Description)
}
