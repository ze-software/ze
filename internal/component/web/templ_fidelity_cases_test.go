package web

import (
	"time"
)

// SCAFFOLDING, deleted with templ_fidelity_test.go when phase 3b closes.
//
// One case per unit and per branch of it. The html/template side reads the data
// the phase-1 capture already authored, so the two engines see the same values.

//nolint:funlen // one entry per ported unit; splitting it hides the census
func fidelityCases() []fidelityCase {
	cases := []fidelityCase{
		// --- config view templates ------------------------------------
		{
			name: "container-full", file: "templates/container.html", tmpl: "container.html",
			data: webConfigViewData(), got: configContainer(webConfigViewData()),
		},
		{
			name: "container-empty", file: "templates/container.html", tmpl: "container.html",
			data: &ConfigViewData{CurrentPath: "bgp"}, got: configContainer(&ConfigViewData{CurrentPath: "bgp"}),
		},
		{
			name: "list-selected", file: "templates/list.html", tmpl: "list.html",
			data: webConfigListData(true), got: configList(webConfigListData(true)),
		},
		{
			name: "list-unselected", file: "templates/list.html", tmpl: "list.html",
			data: webConfigListData(false), got: configList(webConfigListData(false)),
		},
		{
			name: "inline-list-selected", file: "templates/inline_list.html", tmpl: "inline_list.html",
			data: webConfigListData(true), got: configInlineList(webConfigListData(true)),
		},
		{
			name: "inline-list-unselected", file: "templates/inline_list.html", tmpl: "inline_list.html",
			data: webConfigListData(false), got: configInlineList(webConfigListData(false)),
		},
		{
			name: "freeform-entries", file: "templates/freeform.html", tmpl: "freeform.html",
			data: webFreeformData(), got: configFreeform(webFreeformData()),
		},
		{
			name: "freeform-empty", file: "templates/freeform.html", tmpl: "freeform.html",
			data: &ConfigViewData{CurrentPath: "system/banner"},
			got:  configFreeform(&ConfigViewData{CurrentPath: "system/banner"}),
		},
		{
			name: "flex-flag", file: "templates/flex.html", tmpl: "flex.html",
			data: map[string]any{"Name": "multipath", "Children": nil, "Value": ""},
			got:  configFlex(configFlexData{Name: "multipath"}),
		},
		{
			name: "flex-value", file: "templates/flex.html", tmpl: "flex.html",
			data: map[string]any{
				"Name": "hold-time", "Children": nil, "Value": "180",
				"LeafField": webLeafField("number"),
			},
			got: configFlex(configFlexData{
				Name: "hold-time", Value: "180", LeafField: webLeafField("number"),
			}),
		},
		{
			name: "flex-block", file: "templates/flex.html", tmpl: "flex.html",
			data: map[string]any{
				"Name": "graceful-restart", "Value": "",
				"LeafFields": []LeafField{webLeafField("text"), webLeafField("checkbox")},
				"Children":   webFlexChildren(),
			},
			got: configFlex(configFlexData{
				Name:       "graceful-restart",
				LeafFields: []LeafField{webLeafField("text"), webLeafField("checkbox")},
				Children:   webFlexChildren(),
			}),
		},
		{
			name: "config-breadcrumb-back", file: "templates/breadcrumb.html", tmpl: "breadcrumb.html",
			data: map[string]any{"BackURL": "/show/bgp/", "Segments": webBreadcrumbs()},
			got: configBreadcrumb(configBreadcrumbData{
				BackURL: "/show/bgp/", Segments: webBreadcrumbs(),
			}),
		},
		{
			name: "config-breadcrumb-root", file: "templates/breadcrumb.html", tmpl: "breadcrumb.html",
			data: map[string]any{"BackURL": "", "Segments": webBreadcrumbs()[:1]},
			got:  configBreadcrumb(configBreadcrumbData{Segments: webBreadcrumbs()[:1]}),
		},
		{
			name: "commit-diff", file: "templates/commit.html", tmpl: "commit.html",
			data: webCommitMap(webCommitData("diff")), got: configCommit(webCommitData("diff")),
		},
		{
			name: "commit-conflict", file: "templates/commit.html", tmpl: "commit.html",
			data: webCommitMap(webCommitData("conflict")), got: configCommit(webCommitData("conflict")),
		},
		{
			name: "commit-empty", file: "templates/commit.html", tmpl: "commit.html",
			data: webCommitMap(webCommitData("empty")), got: configCommit(webCommitData("empty")),
		},
		{
			name: "notification-changes", file: "templates/notification.html", tmpl: "notification.html",
			data: map[string]any{"ChangeCount": 3, "Messages": []string{"peer added", "commit pending"}},
			got: configNotification(configNotificationData{
				ChangeCount: 3, Messages: []string{"peer added", "commit pending"},
			}),
		},
		{
			name: "notification-one", file: "templates/notification.html", tmpl: "notification.html",
			data: map[string]any{"ChangeCount": 1, "Messages": nil},
			got:  configNotification(configNotificationData{ChangeCount: 1}),
		},
		{
			name: "config-command-ok", file: "templates/command.html", tmpl: "command.html",
			data: webCommandResult(false), got: configCommand(webCommandResult(false)),
		},
		{
			name: "config-command-error", file: "templates/command.html", tmpl: "command.html",
			data: webCommandResult(true), got: configCommand(webCommandResult(true)),
		},
		{
			name: "config-command-form", file: "templates/command_form.html", tmpl: "command_form.html",
			data: webCommandForm(), got: configCommandForm(webCommandForm()),
		},
		{
			name: "config-command-form-bare", file: "templates/command_form.html", tmpl: "command_form.html",
			data: CommandFormData{CommandName: "restart", ActionURL: "/admin/restart"},
			got:  configCommandForm(CommandFormData{CommandName: "restart", ActionURL: "/admin/restart"}),
		},
		{
			name: "notification-banner-oob", file: "templates/notification_banner.html", tmpl: "notification_banner.html",
			data: webBannerData(), got: notificationBannerOOB(webBannerData()),
		},
		{
			name: "terminal", file: "templates/terminal.html", tmpl: "terminal.html",
			data: map[string]any{"CLIPrompt": "ze[bgp]# "},
			got:  terminalPage(terminalPageData{CLIPrompt: "ze[bgp]# "}),
		},

		// --- l2tp pages -----------------------------------------------
		{
			name: "l2tp-list-sessions", file: "templates/l2tp/list.html", tmpl: "list.html",
			data: webL2TPListMap(true), got: l2tpList(webL2TPListView(true)),
		},
		{
			name: "l2tp-list-empty", file: "templates/l2tp/list.html", tmpl: "list.html",
			data: webL2TPListMap(false), got: l2tpList(webL2TPListView(false)),
		},
		{
			name: "l2tp-detail-events", file: "templates/l2tp/detail.html", tmpl: "detail.html",
			data: webL2TPDetailMap(true), got: l2tpDetail(webL2TPDetailView(true)),
		},
		{
			name: "l2tp-detail-no-events", file: "templates/l2tp/detail.html", tmpl: "detail.html",
			data: webL2TPDetailMap(false), got: l2tpDetail(webL2TPDetailView(false)),
		},
	}

	return append(cases, fidelityFragmentCases()...)
}

//nolint:funlen // one entry per ported fragment; splitting it hides the census
func fidelityFragmentCases() []fidelityCase {
	const dir = "templates/component/"

	return []fidelityCase{
		{
			name: "add-form-keyed", file: dir + "add_form_overlay.html", tmpl: "add_form_overlay",
			data: webAddFormMap(false), got: addFormOverlay(webAddFormView(false)),
		},
		{
			name: "add-form-keyless", file: dir + "add_form_overlay.html", tmpl: "add_form_overlay",
			data: webAddFormMap(true), got: addFormOverlay(webAddFormView(true)),
		},
		{
			name: "breadcrumb-nav-deep", file: dir + "breadcrumb.html", tmpl: "breadcrumb_nav",
			data: webFragmentData(), got: breadcrumbNav(webFragmentData().chrome()),
		},
		{
			name: "breadcrumb-nav-one", file: dir + "breadcrumb.html", tmpl: "breadcrumb_nav",
			data: webBreadcrumbOne(), got: breadcrumbNav(webBreadcrumbOne().chrome()),
		},
		{
			name: "breadcrumb-nav-none", file: dir + "breadcrumb.html", tmpl: "breadcrumb_nav",
			data: webBreadcrumbNone(), got: breadcrumbNav(webBreadcrumbNone().chrome()),
		},
		{
			name: "breadcrumb-inner-deep", file: dir + "breadcrumb.html", tmpl: "breadcrumb_inner",
			data: webFragmentData(), got: breadcrumbInner(webFragmentData().chrome()),
		},
		{
			name: "topbar-actions-user", file: dir + "breadcrumb.html", tmpl: "topbar_actions",
			data: webFragmentData(), got: topbarActions(webFragmentData().chrome()),
		},
		{
			name: "topbar-actions-insecure", file: dir + "breadcrumb.html", tmpl: "topbar_actions",
			data: webFragmentDataInsecure(), got: topbarActions(webFragmentDataInsecure().chrome()),
		},
		{
			name: "command-form", file: dir + "command_form.html", tmpl: "command_form",
			data: webFragmentDataCommandForm(), got: commandForm(webFragmentDataCommandForm()),
		},
		{
			name: "command-result-ok", file: dir + "command_result.html", tmpl: "command_result",
			data: webFragmentDataCommandResult(false), got: commandResult(webFragmentDataCommandResult(false)),
		},
		{
			name: "command-result-error", file: dir + "command_result.html", tmpl: "command_result",
			data: webFragmentDataCommandResult(true), got: commandResult(webFragmentDataCommandResult(true)),
		},
		{
			name: "dashboard-events-rows", file: dir + "dashboard_events.html", tmpl: "dashboard_events",
			data: webDashboardEvents(true), got: dashboardEvents(webDashboardEvents(true)),
		},
		{
			name: "dashboard-events-empty", file: dir + "dashboard_events.html", tmpl: "dashboard_events",
			data: webDashboardEvents(false), got: dashboardEvents(webDashboardEvents(false)),
		},
		{
			name: "dashboard-health-rows", file: dir + "dashboard_health.html", tmpl: "dashboard_health",
			data: webDashboardHealth(true), got: dashboardHealth(webDashboardHealth(true)),
		},
		{
			name: "dashboard-health-empty", file: dir + "dashboard_health.html", tmpl: "dashboard_health",
			data: webDashboardHealth(false), got: dashboardHealth(webDashboardHealth(false)),
		},
		{
			name: "dashboard-overview-full", file: dir + "dashboard_overview.html", tmpl: "dashboard_overview",
			data: webDashboardData(true), got: dashboardOverview(webDashboardData(true)),
		},
		{
			name: "dashboard-overview-empty", file: dir + "dashboard_overview.html", tmpl: "dashboard_overview",
			data: webDashboardData(false), got: dashboardOverview(webDashboardData(false)),
		},
		{
			name: "detail-fields", file: dir + "detail.html", tmpl: "detail",
			data: webFragmentDataFields(), got: detail(webFragmentDataFields()),
		},
		{
			name: "detail-list-table", file: dir + "detail.html", tmpl: "detail",
			data: webFragmentDataListTable(false), got: detail(webFragmentDataListTable(false)),
		},
		{
			name: "detail-command-form", file: dir + "detail.html", tmpl: "detail",
			data: webFragmentDataCommandForm(), got: detail(webFragmentDataCommandForm()),
		},
		{
			name: "detail-command-result", file: dir + "detail.html", tmpl: "detail",
			data: webFragmentDataCommandResult(false), got: detail(webFragmentDataCommandResult(false)),
		},
		{
			name: "detail-hint", file: dir + "detail.html", tmpl: "detail",
			data: &FragmentData{CurrentPath: "bgp"}, got: detail(&FragmentData{CurrentPath: "bgp"}),
		},
		{
			name: "finder-columns", file: dir + "finder.html", tmpl: "finder",
			data: webFragmentData(), got: finder(webFragmentData()),
		},
		{
			name: "finder-empty", file: dir + "finder.html", tmpl: "finder",
			data: &FragmentData{CurrentPath: "bgp"}, got: finder(&FragmentData{CurrentPath: "bgp"}),
		},
		{
			name: "finder-oob-columns", file: dir + "finder_oob.html", tmpl: "finder_oob",
			data: webFragmentData(), got: finderOOB(webFragmentData()),
		},
		{
			name: "finder-oob-empty", file: dir + "finder_oob.html", tmpl: "finder_oob",
			data: &FragmentData{CurrentPath: "bgp"}, got: finderOOB(&FragmentData{CurrentPath: "bgp"}),
		},
		{
			name: "list-table-editable", file: dir + "list_table.html", tmpl: "list_table",
			data: webFragmentDataListTable(false), got: listTable(webFragmentDataListTable(false)),
		},
		{
			name: "list-table-readonly", file: dir + "list_table.html", tmpl: "list_table",
			data: webFragmentDataListTable(true), got: listTable(webFragmentDataListTable(true)),
		},
		{
			name: "log-live", file: dir + "log_live.html", tmpl: "log_live",
			data: nil, got: logLive(),
		},
		{
			name: "log-table-rows", file: dir + "log_table.html", tmpl: "log_table",
			data: webLogTable(true), got: logTable(webLogTable(true)),
		},
		{
			name: "log-table-empty", file: dir + "log_table.html", tmpl: "log_table",
			data: webLogTable(false), got: logTable(webLogTable(false)),
		},
		{
			name: "notification-error", file: dir + "notification_error.html", tmpl: "notification_error",
			data: struct{ Message string }{Message: webNotificationErrorText},
			got:  notificationError(notificationErrorData{Message: webNotificationErrorText}),
		},
		{
			name: "oob-error", file: dir + "oob_error.html", tmpl: "oob_error",
			data: webErrorData(), got: oobError(webErrorData()),
		},
		{
			name: "error-item", file: dir + "oob_error.html", tmpl: "error_item",
			data: webErrorData(), got: errorItem(webErrorData()),
		},
		{
			name: "oob-response-fields", file: dir + "oob_response.html", tmpl: "oob_response",
			data: webFragmentDataFields(), got: oobResponse(webFragmentDataFields()),
		},
		{
			name: "oob-response-monitor", file: dir + "oob_response.html", tmpl: "oob_response",
			data: webFragmentDataMonitor(), got: oobResponse(webFragmentDataMonitor()),
		},
		{
			name: "full-content-fields", file: dir + "oob_response.html", tmpl: "full_content",
			data: webFragmentDataFields(), got: fullContent(webFragmentDataFields()),
		},
		{
			name: "full-content-monitor", file: dir + "oob_response.html", tmpl: "full_content",
			data: webFragmentDataMonitor(), got: fullContent(webFragmentDataMonitor()),
		},
		{
			name: "oob-save-changes", file: dir + "oob_save.html", tmpl: "oob_save_ok",
			data: struct{ ChangeCount int }{ChangeCount: 4}, got: oobSaveOK(saveOKData{ChangeCount: 4}),
		},
		{
			name: "oob-save-one", file: dir + "oob_save.html", tmpl: "oob_save_ok",
			data: struct{ ChangeCount int }{ChangeCount: 1}, got: oobSaveOK(saveOKData{ChangeCount: 1}),
		},
		{
			name: "oob-save-clean", file: dir + "oob_save.html", tmpl: "oob_save_ok",
			data: struct{ ChangeCount int }{ChangeCount: 0}, got: oobSaveOK(saveOKData{}),
		},
		{
			name: "path-bar-segments", file: dir + "path_bar.html", tmpl: "path_bar_inner",
			data: webFragmentData(), got: pathBarInner(webFragmentData()),
		},
		{
			name: "path-bar-root", file: dir + "path_bar.html", tmpl: "path_bar_inner",
			data: &FragmentData{}, got: pathBarInner(&FragmentData{}),
		},
		{
			name: "sidebar-nested", file: dir + "sidebar.html", tmpl: "sidebar",
			data: webFragmentData(), got: sidebar(webFragmentData()),
		},
		{
			name: "sidebar-root", file: dir + "sidebar.html", tmpl: "sidebar",
			data: webSidebarRoot(), got: sidebar(webSidebarRoot()),
		},
		{
			name: "sidebar-section-list", file: dir + "sidebar.html", tmpl: "sidebar_section",
			data: webSidebarSections()[0], got: sidebarSection(webSidebarSections()[0]),
		},
		{
			name: "sidebar-section-container", file: dir + "sidebar.html", tmpl: "sidebar_section",
			data: webSidebarSections()[1], got: sidebarSection(webSidebarSections()[1]),
		},
		{
			name: "workbench-dashboard-full", file: dir + "workbench_dashboard.html", tmpl: "workbench_dashboard",
			data: webDashboardData(true), got: workbenchDashboard(webDashboardData(true)),
		},
		{
			name: "workbench-dashboard-empty", file: dir + "workbench_dashboard.html", tmpl: "workbench_dashboard",
			data: webDashboardData(false), got: workbenchDashboard(webDashboardData(false)),
		},
		{
			name: "workbench-detail-tabs", file: dir + "workbench_detail.html", tmpl: "workbench_detail",
			data: webWorkbenchDetail(true), got: workbenchDetail(webWorkbenchDetail(true)),
		},
		{
			name: "workbench-detail-bare", file: dir + "workbench_detail.html", tmpl: "workbench_detail",
			data: webWorkbenchDetail(false), got: workbenchDetail(webWorkbenchDetail(false)),
		},
		{
			name: "workbench-form-fields", file: dir + "workbench_form.html", tmpl: "workbench_form",
			data: webWorkbenchForm(true), got: workbenchForm(webWorkbenchForm(true)),
		},
		{
			name: "workbench-form-bare", file: dir + "workbench_form.html", tmpl: "workbench_form",
			data: webWorkbenchForm(false), got: workbenchForm(webWorkbenchForm(false)),
		},
		{
			name: "workbench-table-rows", file: dir + "workbench_table.html", tmpl: "workbench_table",
			data: webWorkbenchTable("rows"), got: workbenchTable(webWorkbenchTable("rows")),
		},
		{
			name: "workbench-table-empty", file: dir + "workbench_table.html", tmpl: "workbench_table",
			data: webWorkbenchTable("empty"), got: workbenchTable(webWorkbenchTable("empty")),
		},
		{
			name: "workbench-table-add-actions", file: dir + "workbench_table.html", tmpl: "workbench_table",
			data: webWorkbenchTable("add-actions"), got: workbenchTable(webWorkbenchTable("add-actions")),
		},
		{
			name: "workbench-topbar-fleet", file: dir + "workbench_topbar.html", tmpl: "workbench_topbar",
			data: webWorkbenchData(false), got: workbenchTopbar(webWorkbenchData(false).LayoutData),
		},
		{
			name: "workbench-topbar-single", file: dir + "workbench_topbar.html", tmpl: "workbench_topbar",
			data: webWorkbenchData(true), got: workbenchTopbar(webWorkbenchData(true).LayoutData),
		},
		{
			name: "tool-bgp-decode-form", file: dir + "tool_bgp_decode.html", tmpl: "tool_bgp_decode",
			data: toolPageData{}, got: toolBGPDecode(toolPageData{}),
		},
		{
			name: "tool-bgp-decode-output", file: dir + "tool_bgp_decode.html", tmpl: "tool_bgp_decode",
			data: webToolOutput(), got: toolBGPDecode(webToolOutput()),
		},
		{
			name: "tool-capture-output", file: dir + "tool_capture.html", tmpl: "tool_capture",
			data: webToolOutput(), got: toolCapture(webToolOutput()),
		},
		{
			name: "tool-capture-error", file: dir + "tool_capture.html", tmpl: "tool_capture",
			data: webToolError(), got: toolCapture(webToolError()),
		},
		{
			name: "tool-metrics-output", file: dir + "tool_metrics.html", tmpl: "tool_metrics",
			data: webToolOutput(), got: toolMetrics(webToolOutput()),
		},
		{
			name: "tool-ping-output", file: dir + "tool_ping.html", tmpl: "tool_ping",
			data: webToolOutput(), got: toolPing(webToolOutput()),
		},
		{
			name: "tool-ping-error", file: dir + "tool_ping.html", tmpl: "tool_ping",
			data: webToolError(), got: toolPing(webToolError()),
		},
		{
			name: "tool-overlay-result", file: dir + "tool_overlay.html", tmpl: "tool_overlay",
			data: webToolOverlay(ToolOverlayResult, false), got: toolOverlay(webToolOverlay(ToolOverlayResult, false)),
		},
		{
			name: "tool-overlay-overflow", file: dir + "tool_overlay.html", tmpl: "tool_overlay",
			data: webToolOverlay(ToolOverlayResult, true), got: toolOverlay(webToolOverlay(ToolOverlayResult, true)),
		},
		{
			name: "tool-overlay-error", file: dir + "tool_overlay.html", tmpl: "tool_overlay",
			data: webToolOverlay(ToolOverlayError, false), got: toolOverlay(webToolOverlay(ToolOverlayError, false)),
		},
		{
			name: "tool-overlay-confirm", file: dir + "tool_overlay.html", tmpl: "tool_overlay",
			data: webToolOverlay(ToolOverlayConfirm, false), got: toolOverlay(webToolOverlay(ToolOverlayConfirm, false)),
		},
	}
}

// --- fixture data the two engines share ---------------------------------

const webNotificationErrorText = "commit rejected: hold-time out of range"

func webFreeformData() *ConfigViewData {
	return &ConfigViewData{
		CurrentPath: "system/banner",
		Entries:     []string{"first line", "second line"},
	}
}

func webFlexChildren() []ChildEntry {
	return []ChildEntry{
		{Name: "timers", Kind: "container", URL: "/show/bgp/timers/", HxPath: "bgp/timers"},
	}
}

func webCommandResult(failed bool) CommandResultData {
	if failed {
		return CommandResultData{
			CommandName: "peer 192.0.2.9 teardown", Output: "no such peer", Error: true,
		}
	}

	return CommandResultData{
		CommandName: "peer 192.0.2.1 teardown", Output: "session reset",
	}
}

func webBannerData() notificationBannerData {
	return notificationBannerData{
		Reason: "Config changed by admin: peer added", RefreshURL: "/show/",
	}
}

func webErrorData() ErrorData {
	return ErrorData{
		ID:      "42",
		Path:    "bgp/peer/london/hold-time",
		Message: "value 1 below minimum 3",
	}
}

func webToolOutput() toolPageData {
	return toolPageData{Output: "5 packets transmitted, 5 received"}
}

func webToolError() toolPageData {
	return toolPageData{Error: "destination unreachable"}
}

func webBreadcrumbOne() *FragmentData {
	return &FragmentData{
		Breadcrumbs: webBreadcrumbs()[:1], Username: "admin", ActiveUI: "finder",
	}
}

func webBreadcrumbNone() *FragmentData {
	return &FragmentData{ActiveUI: "finder"}
}

func webSidebarRoot() *FragmentData {
	return &FragmentData{Sidebar: webSidebarSections()}
}

func webFragmentDataMonitor() *FragmentData {
	data := webFragmentDataFields()
	data.Monitor = true

	return data
}

func webCommitData(shape string) configCommitData {
	switch shape {
	case "diff":
		return configCommitData{
			Diff: "+ bgp peer 192.0.2.1",
			DiffLines: []configDiffLine{
				{Text: "+ peer 192.0.2.1", IsAdd: true},
				{Text: "- peer 192.0.2.2", IsDel: true},
				{Text: "~ hold-time 90", IsChange: true},
				{Text: "  bgp"},
			},
		}
	case "conflict":
		return configCommitData{
			Error:         "config changed under you",
			ConflictPaths: []string{"bgp/peer/192.0.2.1", "bgp/local-as"},
		}
	}

	return configCommitData{}
}

// webCommitMap is the same commit data in the shape html/template reads.
func webCommitMap(v configCommitData) map[string]any {
	lines := make([]map[string]any, 0, len(v.DiffLines))
	for _, l := range v.DiffLines {
		lines = append(lines, map[string]any{
			"IsAdd": l.IsAdd, "IsDel": l.IsDel, "IsChange": l.IsChange, "Text": l.Text,
		})
	}

	data := map[string]any{
		"Error": v.Error, "ConflictPaths": v.ConflictPaths, "Diff": v.Diff,
		"DiffLines": nil,
	}
	if len(lines) > 0 {
		data["DiffLines"] = lines
	}

	return data
}

func webAddFormView(keyless bool) addFormData {
	data := addFormData{
		AddURL:     "/config/add/bgp/peer/",
		ListName:   "Peer",
		Heading:    "New Peer",
		KeyName:    "name",
		KeyLabel:   "name",
		KeyInputID: "field-name",
		Keyless:    keyless,
		Workbench:  keyless,
		Fields: []addFormField{
			{Path: "remote/as", Placeholder: "peer AS", Category: "required", Inherited: "64500"},
			{Path: "remote/ip", Placeholder: "peer address", Category: "suggest"},
			{Path: "description", Placeholder: "free text", Category: "unique"},
		},
	}

	if keyless {
		data.KeyName = ""
		data.DisplayKey = "address"
	}

	return data
}

// webAddFormMap is the same overlay data in the shape html/template reads.
func webAddFormMap(keyless bool) map[string]any {
	v := webAddFormView(keyless)

	return map[string]any{
		"AddURL": v.AddURL, "ListName": v.ListName, "Heading": v.Heading,
		"KeyName": v.KeyName, "KeyLabel": v.KeyLabel, "KeyInputID": v.KeyInputID,
		"Keyless": v.Keyless, "DisplayKey": v.DisplayKey, "Workbench": v.Workbench,
		"Fields": v.Fields,
	}
}

func webL2TPListView(populated bool) l2tpListView {
	if !populated {
		return l2tpListView{}
	}

	return l2tpListView{
		TunnelCount:  1,
		SessionCount: 2,
		Sessions: []l2tpSessionRow{
			{
				LocalSID: 1, TunnelTID: 7, Username: "alice", AssignedAddr: "192.0.2.10",
				PeerAddr: "198.51.100.1", State: "established", Interface: "ppp0",
			},
			{
				LocalSID: 2, TunnelTID: 7, Username: "bob", AssignedAddr: "192.0.2.11",
				PeerAddr: "198.51.100.1", State: "establishing", Interface: "ppp1",
			},
		},
	}
}

// webL2TPListMap is the same list data in the shape html/template reads.
func webL2TPListMap(populated bool) map[string]any {
	v := webL2TPListView(populated)

	data := map[string]any{
		"TunnelCount": v.TunnelCount, "SessionCount": v.SessionCount, "Sessions": nil,
	}
	if len(v.Sessions) > 0 {
		data["Sessions"] = v.Sessions
	}

	return data
}

func webL2TPDetailView(events bool) l2tpDetailView {
	view := l2tpDetailView{
		Session: l2tpSessionView{
			LocalSID: 1, Username: "alice", State: "established",
			AssignedAddr: "192.0.2.10", PppInterface: "ppp0",
			TunnelLocalTID: 7, Family: "ipv4",
		},
		Login: "alice",
	}

	if !events {
		view.Login = ""

		return view
	}

	// A fixed timestamp keeps the comparison stable: the page formats it, so
	// time.Now would move the bytes on every run.
	stamp := time.Date(2026, time.January, 2, 15, 4, 5, 0, time.UTC)
	view.Events = []l2tpEventRow{
		{Timestamp: stamp, Type: "connect", Actor: "lac"},
		{Timestamp: stamp.Add(time.Minute), Type: "echo", RTT: "12ms"},
		{Timestamp: stamp.Add(2 * time.Minute), Type: "disconnect", Reason: "admin request"},
	}

	return view
}

// webL2TPDetailMap is the same detail data in the shape html/template reads.
func webL2TPDetailMap(events bool) map[string]any {
	v := webL2TPDetailView(events)

	data := map[string]any{
		"Session": v.Session, "Login": v.Login, "Events": nil,
	}
	if len(v.Events) > 0 {
		data["Events"] = v.Events
	}

	return data
}
