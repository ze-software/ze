package doctor

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestDoctorRefusesUnderivableListenerSet makes the schema fail to load by
// swapping listenerSchema, then reads both the collector and the finding
// checkListeners reports.
//
// VALIDATES: doctor reports doctor-listener-set-unknown, naming the cause and
// the fix, and probes no schema-declared listener when the schema cannot load.
// PREVENTS: doctor answering a guessed listener list (the deleted hardcoded
// fallback) or an empty one, either of which reads as "every listener checked".
func TestDoctorRefusesUnderivableListenerSet(t *testing.T) {
	const cause = "cannot load the YANG schema: module ze-ssh-conf does not resolve"
	useListenerSchema(t, func() (*config.Schema, error) { return nil, errors.New("module ze-ssh-conf does not resolve") })
	probed := recordProbes(t)
	tree := sshEnabledTree()

	listeners, err := collectSchemaListeners(tree)
	require.Error(t, err)
	assert.Equal(t, cause, err.Error())
	assert.Empty(t, listeners, "an underivable listener set must yield no listener, not a guessed list")

	diags := checkListeners(tree)
	var refusal *diagnostic.Diagnostic
	for i := range diags {
		if diags[i].Code == diagnostic.CodeDoctorListenerSetUnknown {
			refusal = &diags[i]
		}
	}
	require.NotNil(t, refusal, "checkListeners must report %s, got %v", diagnostic.CodeDoctorListenerSetUnknown, diags)
	assert.Equal(t, diagnostic.SeverityError, refusal.Severity)
	assert.Contains(t, refusal.Message, cause, "the finding must name the cause")
	assert.True(t, strings.HasSuffix(refusal.Message, "then run: ze doctor"), "the finding must end with the command that confirms the fix: %q", refusal.Message)
	assert.NotContains(t, *probed, "ssh", "no schema-declared listener may be probed when the set is unknown")
	assert.NotNil(t, diagnostic.Lookup(diagnostic.CodeDoctorListenerSetUnknown), "the refusal code must be registered")
}

// TestDoctorSchemaWithNoListenerServiceIsAnAnswer gives doctor a schema that
// loads and declares no ze:listener service, as a build whose linked modules
// carry none does.
//
// VALIDATES: a loaded schema with zero listener services yields no schema
// listener and no error, and checkListeners reports no refusal.
// PREVENTS: a stripped build (or an untagged test run) being told its binary
// is broken when the schema has simply nothing to probe.
func TestDoctorSchemaWithNoListenerServiceIsAnAnswer(t *testing.T) {
	useListenerSchema(t, func() (*config.Schema, error) { return config.NewSchema(), nil })
	probed := recordProbes(t)
	tree := sshEnabledTree()

	listeners, err := collectSchemaListeners(tree)
	require.NoError(t, err, "a schema that loaded is an answer, even with no listener service")
	assert.Empty(t, listeners)

	for _, d := range checkListeners(tree) {
		assert.NotEqual(t, diagnostic.CodeDoctorListenerSetUnknown, d.Code, "a loaded schema must not be refused: %v", d)
	}
	assert.NotContains(t, *probed, "ssh", "a schema without the ssh service yields no ssh probe")
}

// useListenerSchema replaces the schema loader for one test.
func useListenerSchema(t *testing.T, schema func() (*config.Schema, error)) {
	t.Helper()
	old := listenerSchema
	listenerSchema = schema
	t.Cleanup(func() { listenerSchema = old })
}

// recordProbes replaces the bind probe with one that records each service and
// binds nothing.
func recordProbes(t *testing.T) *[]string {
	t.Helper()
	var probed []string
	old := listenerProbe
	listenerProbe = func(l serviceListener) error {
		probed = append(probed, l.service)
		return nil
	}
	t.Cleanup(func() { listenerProbe = old })
	return &probed
}

// sshEnabledTree enables SSH with its default endpoint: a config the schema
// path probes when the ssh service is declared, so an empty probe set is
// evidence and not a config that binds nothing.
func sshEnabledTree() *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer("environment").GetOrCreateContainer("ssh").Set("enabled", "true")
	return tree
}
