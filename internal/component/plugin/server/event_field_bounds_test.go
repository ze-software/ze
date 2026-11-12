package server

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

// TestEnsureRollbackLongErrorStillRaises proves a delete handler whose error
// is longer than the bus's message bound still raises the ensure-rollback
// event, with the message cut and the fix whole.
//
// VALIDATES: ensureRollbackFailed passes its message through
// report.FitMessage.
// PREVENTS: the AC-30 event lost, the bus refusing the oversized message with
// only a debug line, so the left-behind resource shows only as a Warn line.
// METHOD: the chain of TestEnsureRollbackFailureRaisesEvent, with a delete
// handler that fails with a 2000-byte error. Red: the message raised unfitted.
func TestEnsureRollbackLongErrorStillRaises(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	create := func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"created": true}}, nil
	}
	remove := func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		return nil, errors.New("netlink: " + strings.Repeat("device busy, ", 160))
	}
	leaf := func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		return nil, errors.New("unit refused")
	}
	tree := &command.Node{Children: map[string]*command.Node{
		"interface": {Name: "interface", Children: map[string]*command.Node{
			"create": {Name: "create", Children: map[string]*command.Node{
				"dummy": {Name: "dummy", Children: map[string]*command.Node{
					"name": {Name: "name", WireMethod: "test:create-dummy", EnsureExists: "test:delete", Children: map[string]*command.Node{
						"unit": {Name: "unit", WireMethod: "test:unit-add"},
					}},
				}},
			}},
			"delete": {Name: "delete", Children: map[string]*command.Node{
				"name": {Name: "name", WireMethod: "test:delete"},
			}},
		}},
	}}
	chain := buildEnsureChain(tree, "interface create dummy name unit", map[string]Handler{
		"test:create-dummy": create, "test:delete": remove,
	})
	require.Len(t, chain, 1)

	_, err := wrapWithEnsureChain(leaf, chain)(nil, commandtest.Args())
	require.Error(t, err)

	var found *report.Issue
	for _, issue := range report.Errors(0) {
		if issue.Code == diagnostic.CodeDoctorCommandEnsureRollbackFailed {
			found = &issue
		}
	}
	require.NotNil(t, found, "a long handler error lost the ensure-rollback event: %v", report.Errors(0))
	require.LessOrEqual(t, len(found.Message), report.MessageMax)
	require.Contains(t, found.Message, "netlink: device busy")
	require.Equal(t, "ze interface delete name", found.Fix)
}

// TestReloadRefusedLongFixStillRaises proves a refusal whose own fix is longer
// than the bus's fix bound still raises the reload-refused event, with a fix
// that runs.
//
// VALIDATES: reloadRefusalFix gives way to fixShowErrors for a fix the bus
// cannot carry (busFix), and the message still carries the whole fix.
// PREVENTS: the bus refusing the oversized fix with only a debug line, which
// loses the whole event, or a cut fix printing a command that does not run.
// METHOD: raiseReloadRefused with an operator error whose fix is past
// report.FixMax. Red: the refusal's fix raised as it is (no event).
func TestReloadRefusedLongFixStillRaises(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	longFix := "ze plugin install " + strings.Repeat("dependency-", 30)
	require.Greater(t, len(longFix), report.FixMax)
	refusal := opererr.New("plugin x needs plugin y", "this build does not carry plugin y", longFix)

	raiseReloadRefused(reloadRefusalMessage("ze could not resolve the plugins the configuration needs", []string{"x"}, refusal), refusal)

	event := serverEventOf(diagnostic.CodeDoctorConfigReloadRefused)
	require.NotNil(t, event, "a refusal with a long fix lost the reload-refused event: %v", report.Errors(0))
	require.Equal(t, fixShowErrors, event.Fix, "the event's fix is not a whole command")
	require.Contains(t, event.Message, longFix, "the message no longer carries the refusal's whole fix")
}

// TestEventFieldFitHelpers proves fitSubject cuts a long name at a rune
// boundary and marks it, and busFix keeps a fix that fits and replaces one
// that does not; each answer is one the bus accepts.
//
// VALIDATES: fitSubject, busFix, against the bus's own bounds.
// PREVENTS: a long subject or fix dropping the event that carries it.
// METHOD: a subject of multi-byte runes past report.SubjectMax, and fixes on
// either side of report.FixMax, each raised to show the bus takes it.
func TestEventFieldFitHelpers(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	short := "show bgp peer"
	require.Equal(t, short, fitSubject(short))
	long := strings.Repeat("é", report.SubjectMax)
	subject := fitSubject(long)
	require.LessOrEqual(t, len(subject), report.SubjectMax)
	require.True(t, utf8.ValidString(subject), "the cut subject split a rune")
	require.True(t, strings.HasSuffix(subject, subjectCutMark), "the cut subject is not marked")

	fits := strings.Repeat("f", report.FixMax)
	require.Equal(t, fits, busFix(fits, fixShowErrors))
	require.Equal(t, fixShowErrors, busFix(fits+"f", fixShowErrors))
	require.Equal(t, fixShowWarnings, busFix(fits+"f", fixShowWarnings), "a warning's fallback is not the warnings list")

	report.RaiseErrorFix(serverEventSource, diagnostic.CodeDoctorConfigReloadRefused, subject, "probe", busFix(fits, fixShowErrors))
	event := serverEventOf(diagnostic.CodeDoctorConfigReloadRefused)
	require.NotNil(t, event, "the bus refused a fitted subject or fix")
	require.Equal(t, subject, event.Subject)
}

// TestLongFixFallsBackToItsOwnList proves a command event whose fix is past
// report.FixMax still tells the operator what to run: the ensure-rollback
// error carries the whole delete command in its message and lists itself
// under the errors, and the deprecated-name warning points at the warnings
// list, the one it is in.
//
// PREVENTS: a warning whose fix points at the errors list, and a delete
// command that survives only in the log while the event's fix names the list
// the operator is already reading.
// METHOD: ensureRollbackFailed with a rollback path whose selector is past
// the fix bound, and deprecatedUsed with a replacement name past it.
func TestLongFixFallsBackToItsOwnList(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	selector := strings.Repeat("x", report.FixMax)
	step := &EnsureStep{
		Path:         []string{"interface", "create", "dummy", "name"},
		RollbackPath: []string{"interface", "delete", "name", selector},
	}
	ensureRollbackFailed(nil, step, errors.New("netlink: device busy"))
	var rollback *report.Issue
	for _, issue := range report.Errors(0) {
		if issue.Code == diagnostic.CodeDoctorCommandEnsureRollbackFailed {
			rollback = &issue
		}
	}
	require.NotNil(t, rollback, "the ensure-rollback event was lost")
	require.Equal(t, fixShowErrors, rollback.Fix)
	// Red: the carried command in ensureRollbackFailed's message.
	require.Contains(t, rollback.Message, "`ze interface delete name "+selector+"`", "the message does not carry the whole delete command")
	require.Contains(t, rollback.Message, "netlink: device busy")

	newName := "show " + strings.Repeat("y", report.FixMax)
	deprecatedUsed("show old", newName)
	var deprecated *report.Issue
	for _, issue := range report.Warnings() {
		if issue.Code == diagnostic.CodeDoctorCommandDeprecatedUsed {
			deprecated = &issue
		}
	}
	require.NotNil(t, deprecated, "the deprecated-name warning was lost")
	// Red: deprecatedUsed's fallback back at fixShowErrors.
	require.Equal(t, fixShowWarnings, deprecated.Fix)
	require.Contains(t, deprecated.Message, newName)
}
