package fleetui

import (
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/golden"
)

// goldenPage resolves the page at target over the test estate, with pages
// registered for it alone, and answers the shell's view and the outcome that
// holds the page's own components.
func goldenPage(t *testing.T, target string, pages ...Page) (ShellView, *pageOutcome) {
	t.Helper()

	request := testRequest(t, target)
	for _, page := range pages {
		registerTestPage(t, request.pages, page)
	}

	outcome := resolvePage(context.Background(), request)

	view, err := shellView(context.Background(), request, outcome)
	require.NoError(t, err)

	return view, outcome
}

// goldenBranch answers the rows under the tree row key names.
func goldenBranch(t *testing.T, key string) templ.Component {
	t.Helper()

	branch, err := treeBranchFragment(context.Background(), testRequest(t, pathEstate), key)
	require.NoError(t, err)

	return branch
}

// goldenTree answers the tree of the estate page narrowed by filter.
func goldenTree(t *testing.T, filter string) TreeRowsView {
	t.Helper()

	rows, err := treeRowsView(testRequest(t, pathEstate), filter, true)
	require.NoError(t, err)

	return rows
}

// goldenSpecShell is the capture plan of the templates the shell owns:
// shell.templ, tree.templ and search.templ. Every case is built by the code
// a request runs, over the test estate of fixture_test.go, so a fixture is
// what a page of that estate renders.
func goldenSpecShell(t *testing.T) golden.Spec {
	t.Helper()

	estate, estateOutcome := goldenPage(t, pathEstate)
	device, deviceOutcome := goldenPage(t, pathDevice)
	group, groupOutcome := goldenPage(t, pathGroup+"?view=site-role")
	missing, missingOutcome := goldenPage(t, "/fleet/device/no-such-device/dashboard/")
	admin, adminOutcome := goldenPage(t, "/fleet/admin/labels/")
	tasks, _ := goldenPage(t, "/fleet/tasks/")

	table := Page{
		Kind: ObjectEstate, Tab: tabTestPage,
		Content:   sentencePage("The devices table goes here."),
		SidePanel: sentencePage("The details of the row go here."),
	}
	rows, rowsOutcome := goldenPage(t, pathEstateTab, table)
	opened, openedOutcome := goldenPage(t, pathEstateTab+"?id="+coreLon01, table)

	static := table
	static.Static = true
	terminal, terminalOutcome := goldenPage(t, pathEstateTab, static)

	busy := ObjectHeaderView{
		Icon:       ObjectIconView{Icon: iconDevice, Status: DisplayStatus(DisplayStateDegraded), Marked: true},
		Name:       cpe0113,
		Stated:     true,
		State:      DisplayStateDegraded,
		Busy:       true,
		HasPrimary: true,
		Primary:    ActionView{Label: "Run diagnostic", Icon: "plus", Href: "/fleet/device/" + cpe0113 + "/tasks/"},
		Menu: MenuView{
			Label:       actionsMenuWord + " for " + cpe0113,
			Items:       []ActionView{{Label: "Push config", Icon: "config", Href: "/fleet/device/" + cpe0113 + "/config/"}},
			Destructive: []ActionView{{Label: "Remove", Icon: "trash", Href: "/fleet/device/" + cpe0113 + "/config/"}},
		},
	}

	busyRow := TreeRowView{
		Key:      treeKeyDevice + cpe0113,
		Title:    cpe0113,
		Href:     "/fleet/device/" + cpe0113 + "/dashboard/",
		Icon:     ObjectIconView{Icon: iconDevice, Status: DisplayStatus(DisplayStateDegraded), Marked: true},
		Busy:     true,
		Alarms:   2,
		Selected: true,
		Menu:     busy.Menu,
	}

	search := testRequest(t, pathEstate)

	return golden.Spec{
		"shell.templ": {
			{Name: "Shell", Variants: []golden.Variant{
				{Name: "estate", Data: Shell(estate, estateOutcome.content, estateOutcome.side)},
				{Name: "device", Data: Shell(device, deviceOutcome.content, deviceOutcome.side)},
				{Name: "admin", Data: Shell(admin, adminOutcome.content, adminOutcome.side)},
				{Name: "row-open", Data: Shell(opened, openedOutcome.content, openedOutcome.side)},
				{Name: "not-found", Data: Shell(missing, missingOutcome.content, missingOutcome.side)},
			}},
			{Name: "pageUpdate", Variants: []golden.Variant{
				{Name: "group", Data: pageUpdate(group, groupOutcome.content, groupOutcome.side)},
				{Name: "row-open", Data: pageUpdate(opened, openedOutcome.content, openedOutcome.side)},
			}},
			{Name: "shellHeader", Variants: []golden.Variant{
				{Data: shellHeader(estate.Header)},
			}},
			{Name: "headerTasks", Variants: []golden.Variant{
				{Name: "quiet", Data: headerTasks(HeaderTasksView{Label: "Tasks, 0 running"})},
				{Name: "running", Data: headerTasks(estate.Header.Tasks)},
				{Name: "failed", Data: headerTasks(HeaderTasksView{Running: 2, Failed: true, Label: "Tasks, 2 running, a task failed"})},
			}},
			{Name: "shellSidebar", Variants: []golden.Variant{
				{Name: "estate", Data: shellSidebar(estate.Sidebar)},
				{Name: "admin", Data: shellSidebar(admin.Sidebar)},
			}},
			{Name: "adminLinks", Variants: []golden.Variant{
				{Data: adminLinks(admin.Sidebar)},
			}},
			{Name: "shellMain", Variants: []golden.Variant{
				{Data: shellMain(device.Main, deviceOutcome.content)},
			}},
			// The fixture is renamed: staleBanner.html and the component
			// StaleBanner.html are one file on a case-insensitive disk.
			{Name: "staleBanner", Fixture: "shellStaleBanner", Variants: []golden.Variant{
				{Data: staleBanner()},
			}},
			{Name: "objectRegion", Variants: []golden.Variant{
				{Name: "device", Data: objectRegion(device.Main)},
				{Name: "group", Data: objectRegion(group.Main)},
				{Name: "tasks", Data: objectRegion(tasks.Main)},
			}},
			{Name: "objectHeader", Variants: []golden.Variant{
				{Name: "estate", Data: objectHeader(estate.Main.Object)},
				{Name: "device", Data: objectHeader(device.Main.Object)},
				{Name: "busy-with-actions", Data: objectHeader(busy)},
			}},
			{Name: "breadcrumb", Variants: []golden.Variant{
				{Name: "device", Data: breadcrumb(device.Main.Crumbs)},
				{Name: "admin", Data: breadcrumb(admin.Main.Crumbs)},
			}},
			{Name: "tabRow", Variants: []golden.Variant{
				{Data: tabRow(device.Main)},
			}},
			{Name: "contentRegion", Variants: []golden.Variant{
				{Name: "live", Data: contentRegion(rows.Main, rowsOutcome.content)},
				{Name: "static", Data: contentRegion(terminal.Main, terminalOutcome.content)},
				{Name: "not-found", Data: contentRegion(missing.Main, missingOutcome.content)},
			}},
			{Name: "sidePanel", Variants: []golden.Variant{
				{Name: "none", Data: sidePanel(estate.Panel, estateOutcome.side, false)},
				{Name: "empty", Data: sidePanel(rows.Panel, rowsOutcome.side, false)},
				{Name: "open", Data: sidePanel(opened.Panel, openedOutcome.side, false)},
				{Name: "open-out-of-band", Data: sidePanel(opened.Panel, openedOutcome.side, true)},
			}},
		},
		"tree.templ": {
			{Name: "treePane", Variants: []golden.Variant{
				{Name: "estate", Data: treePane(estate.Sidebar.Tree)},
				{Name: "device", Data: treePane(device.Sidebar.Tree)},
			}},
			{Name: "treeViewOptions", Variants: []golden.Variant{
				{Data: treeViewOptions(device.Sidebar.Tree)},
			}},
			{Name: "treeRows", Variants: []golden.Variant{
				{Name: "whole", Data: treeRows(goldenTree(t, ""))},
				{Name: "filtered", Data: treeRows(goldenTree(t, "ams"))},
				{Name: "no-match", Data: treeRows(goldenTree(t, "no-such-text"))},
			}},
			{Name: "treeBranch", Variants: []golden.Variant{
				{Data: goldenBranch(t, "group/site=lon/role=edge/dashboard/")},
			}},
			{Name: "treeRow", Variants: []golden.Variant{
				{Name: "estate", Data: treeRow(goldenTree(t, "").Rows[0])},
				{Name: "busy-device", Data: treeRow(busyRow)},
			}},
			{Name: "treeSkeleton", Variants: []golden.Variant{
				{Data: treeSkeleton()},
			}},
		},
		"search.templ": {
			{Name: "searchResults", Variants: []golden.Variant{
				{Name: "found", Data: searchResults(searchView(search, "lon"))},
				{Name: "pages", Data: searchResults(searchView(search, "s"))},
				{Name: "none", Data: searchResults(searchView(search, "no-such-text"))},
			}},
		},
	}
}
