package server

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// TestEnsureRollbackFailureRaisesEvent proves a parent an ensure-exists chain
// created and then failed to delete is reported on the bus, naming the parent
// path, with the delete command as its fix. Method: a chain built from a
// command tree (create and delete nodes), a creating parent, a failing
// rollback and a failing leaf.
//
// VALIDATES: AC-30 (spec-doctor-coverage-12-plugin-infra).
// PREVENTS: a half-built interface left behind with only a log line.
func TestEnsureRollbackFailureRaisesEvent(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	create := func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"created": true}}, nil
	}
	remove := func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		return nil, errors.New("netlink: device busy")
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
	require.NotNil(t, found, "no ensure-rollback event on the bus: %v", report.Errors(0))
	require.Equal(t, "interface create dummy name", found.Subject)
	require.Contains(t, found.Message, "netlink: device busy")
	require.Equal(t, "ze interface delete name", found.Fix)
}

// TestDeprecatedCommandRaisesEventOnce proves a deprecated command name is
// reported on the bus once per name per session, with the new name as its
// fix. Method: warnDeprecated twice for one name, once for another.
//
// VALIDATES: AC-31.
// PREVENTS: a script on a name the next release removes, with no doctor row.
func TestDeprecatedCommandRaisesEventOnce(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	r := &CommandRegistry{}

	r.warnDeprecated("bgp peer list", "show bgp peer")
	r.warnDeprecated("BGP peer list", "show bgp peer")
	r.warnDeprecated("rib show", "show rib")

	var raised []report.Issue
	for _, issue := range report.Warnings() {
		if issue.Code == diagnostic.CodeDoctorCommandDeprecatedUsed {
			raised = append(raised, issue)
		}
	}
	require.Len(t, raised, 2, "want one event per old name: %v", raised)
	fixes := map[string]string{}
	for _, issue := range raised {
		fixes[issue.Subject] = issue.Fix
	}
	require.Equal(t, map[string]string{"bgp peer list": "show bgp peer", "rib show": "show rib"}, fixes)
}
