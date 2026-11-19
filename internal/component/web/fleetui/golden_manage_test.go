package fleetui

import (
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/golden"
)

// goldenManageBuild runs a page or fragment builder of the manage family on
// the request of target over the manage source, and answers its component.
func goldenManageBuild(t *testing.T, target string, build FragmentBuilder, item string) templ.Component {
	t.Helper()

	component, err := build(context.Background(), manageRequest(t, target), item)
	require.NoError(t, err)
	require.NotNil(t, component)

	return component
}

// goldenManagePage wraps a page builder as a fragment builder.
func goldenManagePage(build PageBuilder) FragmentBuilder {
	return func(ctx context.Context, request *PageRequest, _ string) (templ.Component, error) {
		return build(ctx, request)
	}
}

// goldenSpecManage is the capture plan of the templates of the manage
// family: page_config.templ, page_rollouts.templ and page_admin.templ. Every
// case is built by the code a request runs, over the test estate and the
// manage source of manage_test.go.
func goldenSpecManage(t *testing.T) golden.Spec {
	t.Helper()

	inSync := "/fleet/device/" + edgeLon01 + "/config/"
	offline := "/fleet/device/" + coreAms01 + "/config/"
	diverged := "/fleet/device/" + rsAms01 + "/config/"
	rollouts := "/fleet/estate/rollouts/"
	request := manageRequest(t, inSync)

	preview, err := rolloutPreview(request.Snapshot.Views, "site:ams", rolloutBatchPercentDefault)
	require.NoError(t, err)

	return golden.Spec{
		"page_config.templ": {
			{Name: "manageSection", Variants: []golden.Variant{
				{Data: manageSection(ManageSectionView{
					Title: wordStateLine, Name: fragmentConfigState,
					Body: goldenManageBuild(t, inSync, configStateFragment, ""),
				})},
			}},
			{Name: "ManageForm", Variants: []golden.Variant{{Data: ManageForm(manageForm(request))}}},
			{Name: "ConfigTab", Variants: []golden.Variant{
				{Name: "in-sync", Data: goldenManageBuild(t, inSync, goldenManagePage(configContent), "")},
				{Name: "offline", Data: goldenManageBuild(t, offline, goldenManagePage(configContent), "")},
				{Name: "diverged", Data: goldenManageBuild(t, diverged, goldenManagePage(configContent), "")},
			}},
			{Name: "ConfigText", Variants: []golden.Variant{
				{Data: goldenManageBuild(t, inSync, configTextFragment, "")},
			}},
			{Name: "DivergenceSection", Variants: []golden.Variant{
				{Name: "closed", Data: goldenManageBuild(t, diverged, divergenceFragment, "")},
				{Name: "opened", Data: goldenManageBuild(t, diverged+"?sub="+divergenceSub, divergenceFragment, "")},
			}},
			{Name: "PanelWithDiff", Variants: []golden.Variant{
				{Name: "version", Data: goldenManageBuild(t, inSync+"?id="+manageVersionNew, goldenManagePage(configPanel), "")},
				{Name: "template", Data: goldenManageBuild(t, "/fleet/admin/templates/?id=edge-base", goldenManagePage(adminTemplatesPanel), "")},
			}},
		},
		"page_rollouts.templ": {
			{Name: "RolloutPage", Variants: []golden.Variant{
				{Name: "paused", Data: goldenManageBuild(t, rollouts+"?id="+manageRolloutPaused, goldenManagePage(rolloutsContent), "")},
				{Name: "complete", Data: goldenManageBuild(t, rollouts+"?id="+manageRolloutDone, goldenManagePage(rolloutsContent), "")},
			}},
			{Name: "batchStrip", Variants: []golden.Variant{
				{Data: batchStrip(BatchStripView{Segments: []BatchSegmentView{
					{Label: "Batch 1", Status: rolloutStatus(RolloutStateComplete)},
					{Label: "Batch 2", Status: rolloutStatus(RolloutStatePaused)},
				}})},
			}},
			{Name: "RolloutForm", Variants: []golden.Variant{
				{Data: goldenManageBuild(t, rollouts+"?sub="+newRolloutSub+"&q=site%3Aams", goldenManagePage(rolloutsContent), "")},
			}},
			{Name: "RolloutPreview", Variants: []golden.Variant{{Data: RolloutPreview(preview)}}},
		},
		"page_admin.templ": {
			{Name: "AdminPage", Variants: []golden.Variant{
				{Name: "labels", Data: goldenManageBuild(t, "/fleet/admin/labels/", goldenManagePage(adminLabelsContent), "")},
				{Name: "audit", Data: goldenManageBuild(t, "/fleet/admin/audit/?q=actor%3Abob", goldenManagePage(auditContent), "")},
				{Name: "history", Data: goldenManageBuild(t, "/fleet/admin/history/", goldenManagePage(adminHistoryContent), "")},
			}},
			{Name: "DeviceJoin", Variants: []golden.Variant{
				{Name: "form", Data: goldenManageBuild(t, "/fleet/admin/devices/?id="+adminAddRow, goldenManagePage(adminDevicesPanel), "")},
				{Name: "lines", Data: goldenManageBuild(t, "/fleet/admin/devices/?id=edge-new-01", goldenManagePage(adminDevicesPanel), "")},
			}},
		},
	}
}
