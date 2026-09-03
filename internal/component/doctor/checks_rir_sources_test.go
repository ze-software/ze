package doctor

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

// VALIDATES: the doctor reports a configured delegation source that the fetch
// rule refuses, and stays silent on one it accepts.
// PREVENTS: a mirror an operator can no longer commit but a running config
// still carries, whose refusal would otherwise wait until the day they run
// `update resolve rir` and need it to work.
func TestDoctorReportsADelegationSourceTheRefreshWillRefuse(t *testing.T) {
	diags := checkRIRDelegationSources(rirSourceTree(t, "http://mirror.example.com/delegated-ripencc-extended-latest"))
	require.Len(t, diags, 1, "one refused source, one diagnostic")
	assert.Equal(t, diagnosticRIRSourceRefused, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "ripencc", "the message names the registry")

	assert.Empty(t, checkRIRDelegationSources(rirSourceTree(t, "https://mirror.example.com/delegated-ripencc-extended-latest")),
		"an HTTPS mirror is read, so it is not reported")
	assert.Empty(t, checkRIRDelegationSources(rirSourceTree(t, "http://127.0.0.1:8080/delegated-ripencc-extended-latest")),
		"a mirror on the router itself is read over plain HTTP, so it is not reported")
}

// VALIDATES: a config naming no delegation source produces no diagnostic, and
// neither does an absent tree.
// PREVENTS: a warning on every daemon that never configured a mirror, which is
// the shape that teaches operators to ignore doctor output.
func TestDoctorSaysNothingWhenNoDelegationSourceIsConfigured(t *testing.T) {
	tree, err := config.ParseTreeForValidation("system {\n\thost router1;\n}\n")
	require.NoError(t, err, "parse the config")

	assert.Empty(t, checkRIRDelegationSources(tree))
	assert.Empty(t, checkRIRDelegationSources(nil))
}

// VALIDATES: the code this check emits is registered, so `ze explain` answers
// for it.
// PREVENTS: a diagnostic an operator cannot look up.
func TestDoctorRIRSourceCodeIsRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()

	meta := diagnostic.Lookup(diagnosticRIRSourceRefused)
	require.NotNil(t, meta, "the code reaches ze explain")
	assert.NotEmpty(t, meta.Title)
	assert.NotEmpty(t, meta.Description)
}
