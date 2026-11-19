package fleetui

import (
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/test/golden"
)

// goldenDashboardContent answers the content of the dashboard at target over
// the dashboard test source.
func goldenDashboardContent(t *testing.T, target string) templ.Component {
	t.Helper()

	content, err := dashboardContent(context.Background(), dashboardRequest(t, newDashboardSource(), target))
	require.NoError(t, err)

	return content
}

// goldenDashboardCard answers one card built by its builder over the
// dashboard at target.
func goldenDashboardCard(t *testing.T, target string, build dashboardCardBuilder, frame CardView) templ.Component {
	t.Helper()

	request := dashboardRequest(t, newDashboardSource(), target)
	card, err := build(context.Background(), request, dashboardScopeFor(t, request), frame)
	require.NoError(t, err)

	return card
}

// goldenSpecDashboard is the capture plan of page_dashboard.templ. Every
// case is built by the code a request runs, over the test estate of
// fixture_test.go and the dashboard test source, so a fixture is what a
// dashboard of that estate renders.
func goldenSpecDashboard(t *testing.T) golden.Spec {
	t.Helper()

	estate := dashboardRequest(t, newDashboardSource(), pathEstate)
	estateScope := dashboardScopeFor(t, estate)
	group := dashboardRequest(t, newDashboardSource(), pathDashboardGroup)
	quiet := dashboardRequest(t, newDashboardSource(), "/fleet/group/site=lon/role=edge/dashboard/")
	device := dashboardRequest(t, newDashboardSource(), pathDevice)
	offline := dashboardRequest(t, newDashboardSource(), pathDashboardOffline)

	lines, err := NeedsAttention(estateScope.views, newDashboardSource().rollouts)
	require.NoError(t, err)

	busiest, err := busiestCardView(estate, estateScope, CardView{Title: titleBusiest, Span: 4})
	require.NoError(t, err)

	rollouts, held := dashboardCards.lookup(ObjectEstate, cardRollouts)
	require.True(t, held)
	slot := dashboardSlotView(estate, cardRollouts)
	failure := opererr.New("The rollouts could not be read.", "The rollout store did not answer.", "ze show fleet rollouts")

	note := DashboardNoteView{Card: CardView{Title: "Peers", Span: 2}, Text: dashboardNoSample}

	return golden.Spec{
		"page_dashboard.templ": {
			{Name: "Dashboard", Variants: []golden.Variant{
				{Name: "estate", Data: goldenDashboardContent(t, pathEstate)},
				{Name: "group", Data: goldenDashboardContent(t, pathDashboardGroup)},
				{Name: "device", Data: goldenDashboardContent(t, pathDevice)},
				{Name: "offline", Data: goldenDashboardContent(t, pathDashboardOffline)},
			}},
			{Name: "dashboardChoices", Variants: []golden.Variant{
				{Name: "week", Data: dashboardChoices(dashboardView(dashboardRequest(t, newDashboardSource(), pathEstate+"?range=week")).Ranges)},
			}},
			{Name: "dashboardSlot", Variants: []golden.Variant{
				{Data: dashboardSlot(dashboardSlotView(device, cardPeers), dashboardNote(note))},
			}},
			{Name: "dashboardFailedCard", Variants: []golden.Variant{
				{Data: dashboardFailedCard(dashboardFailedView(estate, &rollouts, slot, failure))},
			}},
			{Name: "dashboardOffline", Variants: []golden.Variant{
				{Data: dashboardOfflineSlot(offline, dashboardScopeFor(t, offline))},
			}},
			{Name: "dashboardAlarms", Variants: []golden.Variant{
				{Name: "open", Data: dashboardAlarms(alarmsCardView(estate, estateScope, CardView{Title: titleAlarms, Span: 8}))},
				{Name: "none", Data: dashboardAlarms(alarmsCardView(quiet, dashboardScopeFor(t, quiet), CardView{Title: titleAlarms, Span: 8}))},
			}},
			{Name: "dashboardAttention", Variants: []golden.Variant{
				{Name: "lines", Data: dashboardAttention(attentionCardView(estateScope, lines, CardView{Title: titleAttention, Span: 4}))},
				{Name: "none", Data: dashboardAttention(attentionCardView(estateScope, nil, CardView{Title: titleAttention, Span: 4}))},
			}},
			{Name: "dashboardRollouts", Variants: []golden.Variant{
				{Name: "rows", Data: goldenDashboardCard(t, pathEstate, dashboardRolloutsCard, CardView{Title: "Rollouts", Span: 4})},
				{Name: "none", Data: dashboardRollouts(DashboardRolloutsView{Card: CardView{Title: "Rollouts", Span: 4}, Empty: "No rollout has run yet."})},
			}},
			{Name: "dashboardChanges", Variants: []golden.Variant{
				{Name: "rows", Data: goldenDashboardCard(t, pathEstate, dashboardChangesCard, CardView{Title: "Recent changes", Span: 4})},
				{Name: "none", Data: dashboardChanges(DashboardChangesView{Card: CardView{Title: "Recent changes", Span: 4}, Empty: "Nothing has changed yet."})},
			}},
			{Name: "dashboardBusiest", Variants: []golden.Variant{
				{Name: "cpu", Data: dashboardBusiest(busiest)},
				{Name: "traffic", Data: goldenDashboardCard(t, pathEstate+"?sort=traffic-in", dashboardBusiestCard, CardView{Title: titleBusiest, Span: 4})},
			}},
			{Name: "dashboardGroup", Variants: []golden.Variant{
				{Data: dashboardGroup(groupCardView(group, dashboardScopeFor(t, group), CardView{Title: "Group", Span: 3}))},
			}},
			{Name: "dashboardConfig", Variants: []golden.Variant{
				{Name: "in-sync", Data: goldenDashboardCard(t, pathDevice, dashboardDeviceConfigCard, CardView{Title: titleConfiguration, Span: 3})},
				{Name: "diverged", Data: goldenDashboardCard(t, "/fleet/device/"+rsAms01+"/dashboard/", dashboardDeviceConfigCard, CardView{Title: titleConfiguration, Span: 3})},
			}},
			{Name: "dashboardNote", Variants: []golden.Variant{
				{Data: dashboardNote(note)},
			}},
		},
	}
}
