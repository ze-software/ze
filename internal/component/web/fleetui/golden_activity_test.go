package fleetui

import (
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/golden"
)

// goldenActivity runs one builder of the activity pages for the page at
// target over source, and answers what it built.
func goldenActivity(t *testing.T, source Source, target string, build PageBuilder) templ.Component {
	t.Helper()

	request, err := NewPageRequest(context.Background(), source, testAddress(t, target), testUser)
	require.NoError(t, err)

	component, err := build(context.Background(), request)
	require.NoError(t, err)

	return component
}

// goldenSpecActivity is the capture plan of the templates the activity
// family owns: page_activity.templ. Every case but the two forms of the
// script control is built by the code a request runs, over the test estate
// of fixture_test.go and the tasks of activity_test.go.
func goldenSpecActivity(t *testing.T) golden.Spec {
	t.Helper()

	source := newActivitySource()

	return golden.Spec{
		"page_activity.templ": {
			{Name: "tasksSection", Variants: []golden.Variant{
				{Name: "estate", Data: goldenActivity(t, source, "/fleet/estate/tasks/", tasksContent)},
				{Name: "filtered", Data: goldenActivity(t, source, "/fleet/tasks/?q=state:failed", tasksContent)},
				{Name: "refused", Data: goldenActivity(t, source, "/fleet/tasks/?q=color:red", tasksContent)},
				{Name: "empty", Data: goldenActivity(t, source, "/fleet/device/"+rsAms01+"/tasks/?q=kind:none", tasksContent)},
			}},
			{Name: "taskSide", Variants: []golden.Variant{
				{Name: "running", Data: goldenActivity(t, source, "/fleet/tasks/?id=t1", taskSidePanel)},
				{Name: "partly-failed", Data: goldenActivity(t, source, "/fleet/tasks/?id=t3", taskSidePanel)},
			}},
			{Name: "actionForm", Variants: []golden.Variant{
				{Data: actionForm(ActionFormView{
					ID: taskFormCancel, Action: "/fleet/action/task/tasks/", Item: "cancel t1",
				})},
			}},
			{Name: "prototypeScript", Variants: []golden.Variant{
				{Name: "stepped", Data: prototypeScript(PrototypeScriptView{
					StepAction: stepPath, HoldAction: holdPath, Output: statesEventID, Steps: true,
				})},
				{Name: "no-script", Data: prototypeScript(PrototypeScriptView{Output: statesEventID})},
			}},
			{Name: "statesPage", Variants: []golden.Variant{
				{Data: goldenActivity(t, steppedSource{source}, "/fleet/admin/states/", statesContent)},
			}},
		},
	}
}
