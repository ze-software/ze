// Related: render.go -- the Renderer whose components are captured here
// Related: fragment.go -- FragmentData, the view model most fragments render

package web

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/test/golden"
)

// webTemplGolden is the golden harness over the web templ tree. templ compiles
// each .templ into Go, so the walk reads the package directory rather than an
// embedded FS. Ext keeps it off the Go files beside them.
//
// ONE SET. The package renders through templ alone: the html/template tree and
// the second Set that covered it went with the last ported template.
//
// Recapture a deliberate markup change with `make ze-web-golden-update`.
var webTemplGolden = golden.Set{
	FS:      os.DirFS("."),
	Dir:     ".",
	Ext:     ".templ",
	Spec:    webTemplGoldenSpec,
	SpecVar: "webTemplGoldenSpec",
}

// webTemplGoldenSpec maps each templ source to the components it declares and
// the data each one renders with.
//
// Fixture names are the html/template paths these components replaced, the
// directory included. A component is a Go function and carries a Go name.
// Moving 30 fixtures in the commit that changes their bytes would hide the
// delta of the port inside a rename.
var webTemplGoldenSpec = golden.Spec{
	// --- page components --------------------------------------------------
	"page_layout.templ": {{
		Name:    "pageLayout",
		Fixture: "page/layout.html",
		Variants: []golden.Variant{
			{Name: "finder", Data: webPageLayout("finder")},
			{Name: "cli", Data: webPageLayout("cli")},
		},
	}},
	"page_workbench.templ": {{
		Name:    "pageWorkbench",
		Fixture: "page/workbench.html",
		Variants: []golden.Variant{
			{Name: "full", Data: webPageWorkbench(false)},
			{Name: "readonly", Data: webPageWorkbench(true)},
		},
	}},
	"page_login.templ": {
		{Name: "pageLogin", Fixture: "page/login.html", Variants: []golden.Variant{
			{Name: "page", Data: pageLogin(LoginData{})},
			{Name: "page-error", Data: pageLogin(LoginData{
				Error: "invalid credentials", ReturnTo: "/show/bgp/", Locale: "fr",
			})},
			{Name: "overlay", Data: pageLogin(LoginData{
				Overlay: true, Error: "session expired", ReturnTo: "/show/", Locale: "en",
			})},
		}},
		{Name: "loginOverlay", Fixture: "page/login_overlay", Variants: []golden.Variant{
			{Data: loginOverlay(LoginData{Overlay: true, Error: "session expired", Locale: "en"})},
		}},
		{Name: "loginFullPage", Fixture: "page/login_full_page", Variants: []golden.Variant{
			{Data: loginFullPage(LoginData{Locale: "fr"})},
		}},
		{Name: "loginForm", Fixture: "page/login_form", Variants: []golden.Variant{
			{Name: "return-to", Data: loginForm(LoginData{ReturnTo: "/show/bgp/"})},
			{Name: "bare", Data: loginForm(LoginData{})},
		}},
	},

	// --- page chrome ------------------------------------------------------
	"component_cli_bar.templ": {{
		Name:     "cliBar",
		Fixture:  "component/cli_bar",
		Variants: []golden.Variant{{Data: cliBar(webLayoutData("finder"))}},
	}},
	"component_commit_bar.templ": {{
		Name:    "commitBar",
		Fixture: "component/commit_bar",
		Variants: []golden.Variant{
			{Name: "changes", Data: commitBar(LayoutData{ChangeCount: 4})},
			{Name: "one", Data: commitBar(LayoutData{ChangeCount: 1})},
			{Name: "clean", Data: commitBar(LayoutData{ChangeCount: 0})},
			{Name: "readonly", Data: commitBar(LayoutData{ChangeCount: 4, ReadOnly: true})},
		},
	}},
	"component_diff_modal.templ": {
		{Name: "diffModal", Fixture: "component/diff_modal", Variants: []golden.Variant{
			{Data: diffModal()},
		}},
		{Name: "diffModalOpen", Fixture: "component/diff_modal_open", Variants: []golden.Variant{
			{Name: "diff", Data: diffModalOpen(commitModalData{
				Diff: "+ bgp peer 192.0.2.1\n- bgp peer 192.0.2.2", ChangeCount: 2,
			})},
			{Name: "clean", Data: diffModalOpen(commitModalData{})},
		}},
	},
	"component_error_panel.templ": {{
		Name:     "errorPanel",
		Fixture:  "component/error_panel",
		Variants: []golden.Variant{{Data: errorPanel()}},
	}},
	"component_workbench_nav.templ": {{
		Name:     "workbenchNav",
		Fixture:  "component/workbench_nav",
		Variants: []golden.Variant{{Data: workbenchNav(webWorkbenchData(false).Sections)}},
	}},

	// --- leaf editors -----------------------------------------------------
	"input_bool.templ": {{
		Name:    "inputBool",
		Fixture: "input/input_bool",
		Variants: []golden.Variant{
			{Name: "true", Data: inputBool(webFieldMeta("bool", "true"))},
			{Name: "false", Data: inputBool(webFieldMeta("bool", "false"))},
			{Name: "unset", Data: inputBool(webFieldMeta("bool", ""))},
		},
	}},
	"input_enum.templ": {{
		Name:    "inputEnum",
		Fixture: "input/input_enum",
		Variants: []golden.Variant{
			{Name: "selected", Data: inputEnum(webFieldMeta("enum", "igp"))},
			{Name: "unset", Data: inputEnum(webFieldMetaBare("enum"))},
		},
	}},
	"input_number.templ": {{
		Name:    "inputNumber",
		Fixture: "input/input_number",
		Variants: []golden.Variant{
			{Name: "set", Data: inputNumber(webFieldMeta("uint16", "180"))},
			{Name: "default", Data: inputNumber(webFieldMeta("uint16", ""))},
			{Name: "bare", Data: inputNumber(webFieldMetaBare("uint16"))},
		},
	}},
	"input_text.templ": {
		{Name: "inputText", Fixture: "input/input_text", Variants: []golden.Variant{
			{Name: "set", Data: inputText(webFieldMeta("string", "london"))},
			{Name: "default", Data: inputText(webFieldMeta("string", ""))},
			{Name: "bare", Data: inputText(webFieldMetaBare("string"))},
		}},
		{Name: "fieldValueTag", Fixture: "input/field_value_tag", Variants: []golden.Variant{
			{Name: "set", Data: fieldValueTag(webFieldMeta("string", "london"))},
			{Name: "default", Data: fieldValueTag(webFieldMeta("string", ""))},
			{Name: "bare", Data: fieldValueTag(webFieldMetaBare("string"))},
		}},
	},
	"input_wrapper.templ": {{
		Name:    "fieldWrapper",
		Fixture: "input/field_wrapper",
		Variants: []golden.Variant{
			{Name: "annotated", Data: webFieldComponent(webFieldMeta("string", "london"))},
			{Name: "bare", Data: webFieldComponent(webFieldMetaBare("string"))},
		},
	}},

	// --- config view components -------------------------------------------
	"config_container.templ": {{
		Name: "configContainer", Fixture: "container.html",
		Variants: []golden.Variant{
			{Name: "full", Data: configContainer(webConfigViewData())},
			{Name: "empty", Data: configContainer(&ConfigViewData{CurrentPath: "bgp"})},
		},
	}},
	"config_list.templ": {{
		Name: "configList", Fixture: "list.html",
		Variants: []golden.Variant{
			{Name: "selected", Data: configList(webConfigListData(true))},
			{Name: "unselected", Data: configList(webConfigListData(false))},
		},
	}},
	"config_inline_list.templ": {{
		Name: "configInlineList", Fixture: "inline_list.html",
		Variants: []golden.Variant{
			{Name: "selected", Data: configInlineList(webConfigListData(true))},
			{Name: "unselected", Data: configInlineList(webConfigListData(false))},
		},
	}},
	"config_freeform.templ": {{
		Name: "configFreeform", Fixture: "freeform.html",
		Variants: []golden.Variant{
			{Name: "entries", Data: configFreeform(webFreeformData())},
			{Name: "empty", Data: configFreeform(&ConfigViewData{CurrentPath: "system/banner"})},
		},
	}},
	// configViewComponent (handler_config_leaf.go) answers nothing for
	// config.NodeFlex, so no live path reaches this. The capture keeps its
	// markup on record. Recorded in plan/journal/silent-fall-through.md.
	"config_flex.templ": {{
		Name: "configFlex", Fixture: "flex.html",
		Variants: []golden.Variant{
			{Name: "flag", Data: configFlex(configFlexData{Name: "multipath"})},
			{Name: "value", Data: configFlex(configFlexData{
				Name: "hold-time", Value: "180", LeafField: webLeafField("number"),
			})},
			{Name: "block", Data: configFlex(configFlexData{
				Name:       "graceful-restart",
				LeafFields: []LeafField{webLeafField("text"), webLeafField("checkbox")},
				Children:   webFlexChildren(),
			})},
		},
	}},
	"config_leaf_input.templ": {{
		Name: "leafInput", Fixture: "leaf_input",
		Variants: []golden.Variant{
			{Name: "text", Data: leafInput(webLeafField("text"))},
			{Name: "checkbox", Data: leafInput(webLeafField("checkbox"))},
			{Name: "select", Data: leafInput(webLeafField("select"))},
			{Name: "number", Data: leafInput(webLeafField("number"))},
			{Name: "modified", Data: leafInput(webLeafFieldModified())},
		},
	}},
	// The four components below sat in the config template map and no node kind
	// names any of them, so nothing renders them. Recorded in
	// plan/journal/unwired-feature.md.
	"config_breadcrumb.templ": {{
		Name: "configBreadcrumb", Fixture: "breadcrumb.html",
		Variants: []golden.Variant{
			{Name: "back", Data: configBreadcrumb(configBreadcrumbData{
				BackURL: "/show/bgp/", Segments: webBreadcrumbs(),
			})},
			{Name: "root", Data: configBreadcrumb(configBreadcrumbData{Segments: webBreadcrumbs()[:1]})},
		},
	}},
	"config_commit.templ": {{
		Name: "configCommit", Fixture: "commit.html",
		Variants: []golden.Variant{
			{Name: "diff", Data: configCommit(webCommitData("diff"))},
			{Name: "conflict", Data: configCommit(webCommitData("conflict"))},
			{Name: "empty", Data: configCommit(webCommitData("empty"))},
		},
	}},
	"config_notification.templ": {{
		Name: "configNotification", Fixture: "notification.html",
		Variants: []golden.Variant{
			{Name: "changes", Data: configNotification(configNotificationData{
				ChangeCount: 3, Messages: []string{"peer added", "commit pending"},
			})},
			{Name: "one", Data: configNotification(configNotificationData{ChangeCount: 1})},
			{Name: "none", Data: configNotification(configNotificationData{})},
		},
	}},
	"config_command.templ": {{
		Name: "configCommand", Fixture: "command.html",
		Variants: []golden.Variant{
			{Name: "ok", Data: configCommand(webCommandResult(false))},
			{Name: "error", Data: configCommand(webCommandResult(true))},
		},
	}},
	"config_command_form.templ": {{
		Name: "configCommandForm", Fixture: "command_form.html",
		Variants: []golden.Variant{
			{Name: "parameters", Data: configCommandForm(webCommandForm())},
			{Name: "bare", Data: configCommandForm(CommandFormData{
				CommandName: "restart", ActionURL: "/admin/restart",
			})},
		},
	}},

	// --- unwired page markup ----------------------------------------------
	"notification_banner.templ": {
		{Name: "notificationBanner", Fixture: "notification_banner_sse", Variants: []golden.Variant{
			{Data: notificationBanner(webBannerData())},
		}},
		{Name: "notificationBannerOOB", Fixture: "notification_banner.html", Variants: []golden.Variant{
			{Data: notificationBannerOOB(webBannerData())},
		}},
	},
	"terminal.templ": {{
		Name: "terminalPage", Fixture: "terminal.html",
		Variants: []golden.Variant{
			{Data: terminalPage(terminalPageData{CLIPrompt: "ze[bgp]# "})},
		},
	}},

	// --- l2tp pages -------------------------------------------------------
	"l2tp_list.templ": {{
		Name: "l2tpList", Fixture: "l2tp/list.html",
		Variants: []golden.Variant{
			{Name: "sessions", Data: l2tpList(webL2TPListView(true))},
			{Name: "empty", Data: l2tpList(webL2TPListView(false))},
		},
	}},
	"l2tp_detail.templ": {{
		Name: "l2tpDetail", Fixture: "l2tp/detail.html",
		Variants: []golden.Variant{
			{Name: "events", Data: l2tpDetail(webL2TPDetailView(true))},
			{Name: "no-events", Data: l2tpDetail(webL2TPDetailView(false))},
		},
	}},

	// --- fragments --------------------------------------------------------
	"component_add_form_overlay.templ": {{
		Name: "addFormOverlay", Fixture: "component/add_form_overlay",
		Variants: []golden.Variant{
			{Name: "keyed", Data: addFormOverlay(webAddFormView(false))},
			{Name: "keyless", Data: addFormOverlay(webAddFormView(true))},
		},
	}},
	"component_breadcrumb.templ": {
		{Name: "breadcrumbNav", Fixture: "component/breadcrumb_nav", Variants: webChromeVariants(breadcrumbNav)},
		{Name: "topbarActions", Fixture: "component/topbar_actions", Variants: []golden.Variant{
			{Name: "user", Data: topbarActions(webFragmentData().chrome())},
			{Name: "insecure", Data: topbarActions(webFragmentDataInsecure().chrome())},
		}},
		{Name: "breadcrumbInner", Fixture: "component/breadcrumb_inner", Variants: webChromeVariants(breadcrumbInner)},
	},
	"component_command_form.templ": {{
		Name: "commandForm", Fixture: "component/command_form",
		Variants: []golden.Variant{{Data: commandForm(webFragmentDataCommandForm())}},
	}},
	"component_command_result.templ": {{
		Name: "commandResult", Fixture: "component/command_result",
		Variants: []golden.Variant{
			{Name: "ok", Data: commandResult(webFragmentDataCommandResult(false))},
			{Name: "error", Data: commandResult(webFragmentDataCommandResult(true))},
		},
	}},
	"component_dashboard_events.templ": {{
		Name: "dashboardEvents", Fixture: "component/dashboard_events",
		Variants: []golden.Variant{
			{Name: "rows", Data: dashboardEvents(webDashboardEvents(true))},
			{Name: "empty", Data: dashboardEvents(webDashboardEvents(false))},
		},
	}},
	"component_dashboard_health.templ": {{
		Name: "dashboardHealth", Fixture: "component/dashboard_health",
		Variants: []golden.Variant{
			{Name: "rows", Data: dashboardHealth(webDashboardHealth(true))},
			{Name: "empty", Data: dashboardHealth(webDashboardHealth(false))},
		},
	}},
	"component_dashboard_overview.templ": {{
		Name: "dashboardOverview", Fixture: "component/dashboard_overview",
		Variants: []golden.Variant{
			{Name: "full", Data: dashboardOverview(webDashboardData(true))},
			{Name: "empty", Data: dashboardOverview(webDashboardData(false))},
		},
	}},
	"component_detail.templ": {{
		Name: "detail", Fixture: "component/detail",
		Variants: []golden.Variant{
			{Name: "fields", Data: detail(webFragmentDataFields())},
			{Name: "list-table", Data: detail(webFragmentDataListTable(false))},
			{Name: "command-form", Data: detail(webFragmentDataCommandForm())},
			{Name: "command-result", Data: detail(webFragmentDataCommandResult(false))},
			{Name: "hint", Data: detail(&FragmentData{CurrentPath: "bgp"})},
		},
	}},
	"component_finder.templ": {
		{Name: "finder", Fixture: "component/finder", Variants: []golden.Variant{
			{Name: "columns", Data: finder(webFragmentData())},
			{Name: "empty", Data: finder(&FragmentData{CurrentPath: "bgp"})},
		}},
		{Name: "finderOOB", Fixture: "component/finder_oob", Variants: []golden.Variant{
			{Name: "columns", Data: finderOOB(webFragmentData())},
			{Name: "empty", Data: finderOOB(&FragmentData{CurrentPath: "bgp"})},
		}},
		{Name: "finderItem", Fixture: "component/finder_item", Variants: []golden.Variant{
			{Name: "entry", Data: finderItem(webFinderColumns()[0].NamedItems[0])},
			{Name: "list", Data: finderItem(webFinderColumns()[1].NamedItems[0])},
		}},
	},
	"component_list_table.templ": {
		{Name: "listTable", Fixture: "component/list_table", Variants: []golden.Variant{
			{Name: "editable", Data: listTable(webFragmentDataListTable(false))},
			{Name: "readonly", Data: listTable(webFragmentDataListTable(true))},
		}},
		{Name: "pendingMarker", Fixture: "component/pending_marker", Variants: []golden.Variant{
			{Name: "pending", Data: pendingMarker(true)},
			{Name: "clean", Data: pendingMarker(false)},
		}},
	},
	"component_log_live.templ": {{
		Name: "logLive", Fixture: "component/log_live",
		Variants: []golden.Variant{{Data: logLive()}},
	}},
	"component_log_table.templ": {{
		Name: "logTable", Fixture: "component/log_table",
		Variants: []golden.Variant{
			{Name: "rows", Data: logTable(webLogTable(true))},
			{Name: "empty", Data: logTable(webLogTable(false))},
		},
	}},
	"component_notification_error.templ": {{
		Name: "notificationError", Fixture: "component/notification_error",
		Variants: []golden.Variant{
			{Data: notificationError(notificationErrorData{Message: webNotificationErrorText})},
		},
	}},
	"component_oob_error.templ": {
		{Name: "oobError", Fixture: "component/oob_error", Variants: []golden.Variant{
			{Data: oobError(webErrorData())},
		}},
		{Name: "errorItem", Fixture: "component/error_item", Variants: []golden.Variant{
			{Data: errorItem(webErrorData())},
		}},
	},
	"component_oob_response.templ": {
		{Name: "oobResponse", Fixture: "component/oob_response", Variants: []golden.Variant{
			{Name: "fields", Data: oobResponse(webFragmentDataFields())},
			{Name: "monitor", Data: oobResponse(webFragmentDataMonitor())},
		}},
		{Name: "fullContent", Fixture: "component/full_content", Variants: []golden.Variant{
			{Name: "fields", Data: fullContent(webFragmentDataFields())},
			{Name: "monitor", Data: fullContent(webFragmentDataMonitor())},
		}},
		{Name: "mainSplit", Fixture: "component/main_split", Variants: []golden.Variant{
			{Name: "fields", Data: mainSplit(webFragmentDataFields())},
			{Name: "monitor", Data: mainSplit(webFragmentDataMonitor())},
		}},
	},
	"component_oob_save.templ": {{
		Name: "oobSaveOK", Fixture: "component/oob_save_ok",
		Variants: []golden.Variant{
			{Name: "changes", Data: oobSaveOK(saveOKData{ChangeCount: 4})},
			{Name: "one", Data: oobSaveOK(saveOKData{ChangeCount: 1})},
			{Name: "clean", Data: oobSaveOK(saveOKData{})},
		},
	}},
	"component_path_bar.templ": {{
		Name: "pathBarInner", Fixture: "component/path_bar_inner",
		Variants: []golden.Variant{
			{Name: "segments", Data: pathBarInner(webFragmentData())},
			{Name: "root", Data: pathBarInner(&FragmentData{})},
		},
	}},
	"component_sidebar.templ": {
		{Name: "sidebar", Fixture: "component/sidebar", Variants: []golden.Variant{
			{Name: "nested", Data: sidebar(webFragmentData())},
			{Name: "root", Data: sidebar(webSidebarRoot())},
		}},
		{Name: "sidebarSection", Fixture: "component/sidebar_section", Variants: []golden.Variant{
			{Name: "list", Data: sidebarSection(webSidebarSections()[0])},
			{Name: "container", Data: sidebarSection(webSidebarSections()[1])},
		}},
	},
	"component_tool_bgp_decode.templ": {{
		Name: "toolBGPDecode", Fixture: "component/tool_bgp_decode",
		Variants: webToolVariants(toolBGPDecode),
	}},
	"component_tool_capture.templ": {{
		Name: "toolCapture", Fixture: "component/tool_capture",
		Variants: webToolVariants(toolCapture),
	}},
	"component_tool_metrics.templ": {{
		Name: "toolMetrics", Fixture: "component/tool_metrics",
		Variants: webToolVariants(toolMetrics),
	}},
	"component_tool_ping.templ": {
		{Name: "toolPing", Fixture: "component/tool_ping", Variants: webToolVariants(toolPing)},
		{Name: "toolResult", Fixture: "component/tool_result", Variants: webToolVariants(toolResult)},
	},
	"component_tool_overlay.templ": {{
		Name: "toolOverlay", Fixture: "component/tool_overlay",
		Variants: []golden.Variant{
			{Name: "result", Data: toolOverlay(webToolOverlay(ToolOverlayResult, false))},
			{Name: "overflow", Data: toolOverlay(webToolOverlay(ToolOverlayResult, true))},
			{Name: "error", Data: toolOverlay(webToolOverlay(ToolOverlayError, false))},
			{Name: "confirm", Data: toolOverlay(webToolOverlay(ToolOverlayConfirm, false))},
		},
	}},
	"component_workbench_dashboard.templ": {{
		Name: "workbenchDashboard", Fixture: "component/workbench_dashboard",
		Variants: []golden.Variant{
			{Name: "full", Data: workbenchDashboard(webDashboardData(true))},
			{Name: "empty", Data: workbenchDashboard(webDashboardData(false))},
		},
	}},
	"component_workbench_detail.templ": {{
		Name: "workbenchDetail", Fixture: "component/workbench_detail",
		Variants: []golden.Variant{
			{Name: "tabs", Data: workbenchDetail(webWorkbenchDetail(true))},
			{Name: "bare", Data: workbenchDetail(webWorkbenchDetail(false))},
		},
	}},
	"component_workbench_form.templ": {{
		Name: "workbenchForm", Fixture: "component/workbench_form",
		Variants: []golden.Variant{
			{Name: "fields", Data: workbenchForm(webWorkbenchForm(true))},
			{Name: "bare", Data: workbenchForm(webWorkbenchForm(false))},
		},
	}},
	"component_workbench_table.templ": {{
		Name: "workbenchTable", Fixture: "component/workbench_table",
		Variants: []golden.Variant{
			{Name: "rows", Data: workbenchTable(webWorkbenchTable("rows"))},
			{Name: "empty", Data: workbenchTable(webWorkbenchTable("empty"))},
			{Name: "add-actions", Data: workbenchTable(webWorkbenchTable("add-actions"))},
		},
	}},
	"component_workbench_topbar.templ": {{
		Name: "workbenchTopbar", Fixture: "component/workbench_topbar",
		Variants: []golden.Variant{
			{Name: "fleet", Data: workbenchTopbar(webWorkbenchData(false).LayoutData)},
			{Name: "single", Data: workbenchTopbar(webWorkbenchData(true).LayoutData)},
		},
	}},
}

// webChromeVariants covers the three breadcrumb depths for a component that
// takes the page chrome.
func webChromeVariants(build func(chromeData) templ.Component) []golden.Variant {
	return []golden.Variant{
		{Name: "deep", Data: build(webFragmentData().chrome())},
		{Name: "one", Data: build(webBreadcrumbOne().chrome())},
		{Name: "none", Data: build(webBreadcrumbNone().chrome())},
	}
}

// webToolVariants covers the three states of a tool page: the bare form, an
// answer, and an error.
func webToolVariants(build func(toolPageData) templ.Component) []golden.Variant {
	return []golden.Variant{
		{Name: "form", Data: build(toolPageData{})},
		{Name: "output", Data: build(webToolOutput())},
		{Name: "error", Data: build(webToolError())},
	}
}

// webPageLayout reproduces what RenderLayout composes: the page chrome around a
// breadcrumb the port has not reached yet.
func webPageLayout(activeUI string) templ.Component {
	return pageLayout(webLayoutData(activeUI))
}

// webPageWorkbench reproduces what RenderWorkbench composes.
func webPageWorkbench(single bool) templ.Component {
	return pageWorkbench(webWorkbenchData(single))
}

// webFieldComponent is the wrapper around the editor a field type resolves to,
// which is what RenderField and the detail fragment both render.
func webFieldComponent(f FieldMeta) templ.Component { return fieldComponent(f) }

// --- fixture data -------------------------------------------------------

func webBreadcrumbs() []BreadcrumbSegment {
	return []BreadcrumbSegment{
		{Name: "ze", URL: "/show/", Active: false},
		{Name: "bgp", URL: "/show/bgp/", Active: false},
		{Name: "peer", URL: "/show/bgp/peer/", Active: true},
	}
}

func webPortalServices() []PortalService {
	return []PortalService{
		{Key: "grafana", Title: "Grafana", Path: "/portal/grafana", Icon: "/assets/grafana.svg"},
		{Key: "lg", Title: "Looking Glass", Path: "/portal/lg"},
	}
}

func webPathBarSegments() []PathBarSegment {
	return []PathBarSegment{
		{Name: "bgp", URL: "/show/bgp/", HxPath: "bgp"},
		{Name: "peer", URL: "/show/bgp/peer/", HxPath: "bgp/peer"},
	}
}

func webLayoutData(activeUI string) LayoutData {
	return LayoutData{
		Title:            "Ze: /bgp/peer",
		Content:          template.HTML("<p>content</p>"),         //nolint:gosec // fixed test fixture
		NotificationHTML: template.HTML("<span>2 changes</span>"), //nolint:gosec // fixed test fixture
		Breadcrumbs:      webBreadcrumbs(),
		CLIPrompt:        "ze[bgp peer]# ",
		CLIContextPath:   "bgp/peer",
		CLIPathBar:       template.HTML("<a href=\"/show/bgp/\">bgp</a>"), //nolint:gosec // fixed test fixture
		HasSession:       true,
		Username:         "admin",
		Services:         webPortalServices(),
		ActiveUI:         activeUI,
		RouterIdentity:   "edge1",
		ChangeCount:      2,
	}
}

func webWorkbenchSections() []WorkbenchSection {
	return []WorkbenchSection{
		{
			Key: "routing", Label: "Routing", URL: "/show/bgp/",
			Selected: true, Expanded: true,
			Children: []WorkbenchSubPage{
				{Key: "bgp", Label: "BGP", URL: "/show/bgp/", Selected: true},
				{Key: "ospf", Label: "OSPF", URL: "/show/ospf/"},
			},
		},
		{
			Key: "system", Label: "System", URL: "/show/system/",
			Children: []WorkbenchSubPage{
				{Key: "users", Label: "Users", URL: "/show/system/user/"},
			},
		},
	}
}

func webWorkbenchData(single bool) workbenchData {
	layout := webLayoutData("workbench")
	if single {
		layout.RouterIdentity = "ze"
		layout.ReadOnly = true
	} else {
		layout.FleetPeers = []FleetPeer{
			{Name: "edge1", URL: "https://edge1.example/", Active: true},
			{Name: "edge2", URL: "https://edge2.example/"},
		}
	}

	return workbenchData{LayoutData: layout, Sections: webWorkbenchSections()}
}

func webLeafField(inputType string) LeafField {
	f := LeafField{
		Name:         "hold-time",
		Value:        "180",
		Default:      "90",
		InputType:    inputType,
		Placeholder:  "seconds",
		Description:  "BGP hold timer in seconds",
		IsConfigured: true,
	}

	switch inputType {
	case "number":
		f.Min = "3"
		f.Max = "65535"
	case "select":
		f.Options = []string{"90", "180", "240"}
	case "text":
		f.Pattern = "[0-9]+"
	}

	return f
}

func webLeafFieldModified() LeafField {
	f := webLeafField("text")
	f.Modified = true
	f.OldValue = "90"
	f.IsConfigured = false

	return f
}

func webChildEntries() []ChildEntry {
	return []ChildEntry{
		{Name: "peer", Kind: "list", URL: "/show/bgp/peer/", HxPath: "bgp/peer"},
		{Name: "timers", Kind: "container", URL: "/show/bgp/timers/", HxPath: "bgp/timers"},
	}
}

func webConfigViewData() *ConfigViewData {
	return &ConfigViewData{
		Path:        []string{"bgp"},
		CurrentPath: "bgp",
		Breadcrumbs: webBreadcrumbs(),
		Children:    webChildEntries(),
		LeafFields:  []LeafField{webLeafField("text"), webLeafField("checkbox")},
	}
}

func webConfigListData(selected bool) *ConfigViewData {
	data := &ConfigViewData{
		Path:        []string{"bgp", "peer"},
		CurrentPath: "bgp/peer",
		Breadcrumbs: webBreadcrumbs(),
		Keys:        []string{"london", "paris"},
		BasePath:    "/show/bgp/peer/",
	}

	if selected {
		data.SelectedKey = "london"
		data.DetailPath = "bgp/peer/london"
		data.SelectedDetail = &ConfigViewData{
			CurrentPath: "bgp/peer/london",
			LeafFields:  []LeafField{webLeafField("text")},
			Children:    webChildEntries(),
		}
	}

	return data
}

func webCommandForm() CommandFormData {
	return CommandFormData{
		CommandName: "peer teardown",
		Description: "Reset a BGP session",
		ActionURL:   "/admin/bgp/peer/london/teardown",
		Parameters: []CommandParameter{
			{Name: "peer", Value: "london", Placeholder: "peer name"},
			{Name: "reason", Placeholder: "free text"},
		},
	}
}

func webFieldMeta(fieldType, value string) FieldMeta {
	f := FieldMeta{
		Leaf:        "hold-time",
		Path:        "bgp/peer/london",
		Type:        fieldType,
		Value:       value,
		Default:     "90",
		Description: "BGP hold timer in seconds",
		Decoration:  "Example Transit",
	}

	switch fieldType {
	case "enum":
		f.Options = "igp,egp,incomplete"
	case "uint16":
		f.Min = "3"
		f.Max = "65535"
	case "string":
		f.Pattern = "[a-z]+"
	}

	return f
}

// webFieldMetaBare drops every optional annotation, so the else branch of each
// conditional in the input templates renders.
func webFieldMetaBare(fieldType string) FieldMeta {
	return FieldMeta{Leaf: "router-id", Path: "bgp", Type: fieldType}
}

func webSidebarSections() []SidebarSection {
	return []SidebarSection{
		{
			Name: "peer", Description: "BGP neighbors", URL: "/show/bgp/peer/",
			HxPath: "bgp/peer", IsList: true, AddURL: "/config/add/bgp/peer/",
			Selected: "london",
			Entries: []SidebarEntry{
				{Key: "london", URL: "/show/bgp/peer/london/", HxPath: "bgp/peer/london", Selected: true},
				{Key: "paris", URL: "/show/bgp/peer/paris/", HxPath: "bgp/peer/paris"},
			},
		},
		{Name: "timers", URL: "/show/bgp/timers/", HxPath: "bgp/timers"},
	}
}

func webFinderColumns() []FinderColumn {
	return []FinderColumn{
		{
			NamedItems: []ColumnItem{
				{Name: "london", URL: "/show/bgp/peer/london/", HxPath: "bgp/peer/london", Selected: true, HasChildren: true},
				{Name: "+ new", IsList: true, AddURL: "/config/add/bgp/peer/"},
			},
			UnnamedItems: []ColumnItem{
				{Name: "timers", URL: "/show/bgp/timers/", HxPath: "bgp/timers"},
			},
		},
		{
			NamedItems: []ColumnItem{
				{Name: "peer", URL: "/show/bgp/peer/", HxPath: "bgp/peer", IsList: true, Count: 2},
			},
		},
	}
}

func webListTableView() *ListTableView {
	return &ListTableView{
		Name:    "peer",
		AddURL:  "/config/add/bgp/peer/",
		FormURL: "/config/add-form/bgp/peer/",
		SetURL:  "/config/set/",
		Columns: []ListTableColumn{
			{Name: "name", Key: true},
			{Name: "remote/ip"},
			{Name: "remote/as"},
		},
		TableTools: []RelatedToolButton{
			{ToolID: "bgp-summary", Label: "Summary", ContextPath: "bgp/peer", Class: "inspect"},
		},
		Rows: []ListTableRow{
			{
				KeyValue: "london", URL: "/show/bgp/peer/london/", HxPath: "bgp/peer/london",
				HasPendingChanges: true,
				Cells: []ListTableCell{
					{Value: "192.0.2.1", Leaf: "ip", Path: "bgp/peer/london/remote", Placeholder: "peer address"},
					{Value: "64500", Leaf: "as", Path: "bgp/peer/london/remote", Placeholder: "peer AS"},
				},
				RowTools: []RelatedToolButton{
					{ToolID: "peer-teardown", Label: "Teardown", ContextPath: "bgp/peer/london", Class: "danger", Confirm: "Reset the session?"},
				},
			},
			{
				KeyValue: "paris", URL: "/show/bgp/peer/paris/", HxPath: "bgp/peer/paris",
				Cells: []ListTableCell{
					{Value: "", Leaf: "ip", Path: "bgp/peer/paris/remote", Placeholder: "peer address"},
					{Value: "64501", Leaf: "as", Path: "bgp/peer/paris/remote", Placeholder: "peer AS"},
				},
			},
		},
	}
}

func webFragmentData() *FragmentData {
	return &FragmentData{
		Path:            []string{"bgp", "peer"},
		CurrentPath:     "bgp/peer",
		Children:        webChildEntries(),
		Sidebar:         webSidebarSections(),
		Columns:         webFinderColumns(),
		ParentURL:       "/show/bgp/",
		ParentHxPath:    "bgp",
		Breadcrumbs:     webBreadcrumbs(),
		HasSession:      true,
		Username:        "admin",
		Services:        webPortalServices(),
		ContextHeading:  []ContextEntry{{ListName: "peer", Key: "london"}},
		CLIPrompt:       "ze[bgp peer]# ",
		CLIContextPath:  "bgp/peer",
		CLIPathSegments: webPathBarSegments(),
		ActiveUI:        "finder",
	}
}

func webFragmentDataInsecure() *FragmentData {
	data := webFragmentData()
	data.Insecure = true
	data.Username = ""
	data.Services = nil
	data.ActiveUI = "workbench"

	return data
}

func webFragmentDataFields() *FragmentData {
	data := webFragmentData()
	data.ActiveUI = "workbench"
	// The type strings are the ones valueTypeToFieldType and buildFieldMeta
	// (fragment.go) produce, so the capture exercises the dispatch fieldFor
	// really performs, fallback included.
	data.Fields = []FieldMeta{
		webFieldMeta("string", "london"),
		webFieldMeta("uint16", "180"),
		webFieldMeta("bool", "true"),
		webFieldMeta("enum", "igp"),
		webFieldMeta("ip", "192.0.2.1"),
	}

	return data
}

func webFragmentDataListTable(readOnly bool) *FragmentData {
	data := webFragmentData()
	data.ActiveUI = "workbench"
	data.ListTable = webListTableView()
	data.ReadOnly = readOnly

	return data
}

func webFragmentDataCommandForm() *FragmentData {
	data := webFragmentData()
	form := webCommandForm()
	data.CommandForm = &form

	return data
}

func webFragmentDataCommandResult(failed bool) *FragmentData {
	data := webFragmentData()
	result := CommandResultData{
		CommandName: "peer 192.0.2.1 teardown",
		Output:      "session reset",
		Error:       failed,
	}

	if failed {
		result.Output = "no such peer"
	}

	data.CommandResult = &result

	return data
}

func webToolOverlay(state ToolOverlayState, overflow bool) toolOverlayData {
	data := toolOverlayData{
		ID:            "tool-overlay-1",
		State:         state,
		Title:         "Peer summary",
		Command:       "show bgp summary",
		ToolID:        "bgp-summary",
		ContextPath:   "bgp/peer/london",
		OutputInline:  template.HTML("peer 192.0.2.1 established"), //nolint:gosec // fixed test fixture
		ErrorMessage:  "dispatch failed: no such command",
		ConfirmPrompt: "Reset the session with 192.0.2.1?",
	}

	if overflow {
		data.OutputOverflow = template.HTML("... 4096 more lines") //nolint:gosec // fixed test fixture
		data.HasOverflow = true
		data.Truncated = true
	}

	return data
}

func webWorkbenchColumns() []WorkbenchTableColumn {
	return []WorkbenchTableColumn{
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "state", Label: "State"},
		{Key: "uptime", Label: "Uptime"},
	}
}

func webWorkbenchTable(shape string) WorkbenchTableData {
	data := WorkbenchTableData{
		Title:        "BGP Peers",
		AddURL:       "/config/add/bgp/peer/",
		AddLabel:     "Add Peer",
		Columns:      webWorkbenchColumns(),
		EmptyMessage: "No BGP peers configured",
		EmptyHint:    "Add a peer to start a session",
	}

	switch shape {
	case "rows":
		data.Rows = []WorkbenchTableRow{
			{
				Key: "london", URL: "/show/bgp/peer/london/",
				Flags: "UP", FlagClass: "green", Pending: true,
				Cells: []string{"london", "established", "3h 12m"},
				Actions: []WorkbenchRowAction{
					{Label: "Detail", URL: "/show/bgp/peer/london/"},
					{Label: "Teardown", HxPost: "/admin/bgp/peer/london/teardown", Class: "danger", Confirm: "Reset the session?"},
				},
			},
			{
				Key: "paris", URL: "/show/bgp/peer/paris/",
				Flags: "DOWN", FlagClass: "red",
				Cells: []string{"paris", "idle", "-"},
			},
		}
	case "add-actions":
		data.AddActions = []WorkbenchTableAddAction{
			{Label: "Add IPv4 peer", URL: "/config/add/bgp/peer/"},
			{Label: "Add IPv6 peer", URL: "/config/add/bgp/peer6/"},
		}
	}

	return data
}

func webWorkbenchForm(full bool) WorkbenchFormData {
	data := WorkbenchFormData{
		Title:   "System settings",
		SaveURL: "/config/form/system/",
	}

	if !full {
		data.Fields = []WorkbenchFormField{
			{Name: "hostname", Label: "Hostname", Type: "text", Value: "edge1"},
		}

		return data
	}

	data.DiscardURL = "/config/discard"
	data.Fields = []WorkbenchFormField{
		{Name: "hostname", Path: "system/host", Label: "Hostname", Type: "text", Value: "edge1", Required: true, Description: "System host name"},
		{Name: "enabled", Path: "system/enabled", Label: "Enabled", Type: "toggle", Value: "true"},
		{Name: "disabled-toggle", Label: "Locked", Type: "toggle", Value: "false", Disabled: true},
		{Name: "log-level", Path: "system/log", Label: "Log level", Type: "dropdown", Value: "info", Options: []string{"debug", "info", "warning"}},
		{Name: "port", Label: "Port", Type: "number", Value: "179", Required: true},
		{Name: "secret", Label: "Secret", Type: "password", Value: "hunter2"},
		{Name: "router-id", Label: "Router ID", Type: "ip", Value: "192.0.2.1"},
		{Name: "servers", Label: "Servers", Type: "list", Items: []string{"192.0.2.53", "192.0.2.54"}},
	}

	return data
}

func webWorkbenchDetail(full bool) WorkbenchDetailData {
	if !full {
		return WorkbenchDetailData{Title: "Peer london"}
	}

	return WorkbenchDetailData{
		Title:    "Peer london",
		CloseURL: "/show/bgp/peer/",
		Tabs: []WorkbenchDetailTab{
			{Key: "state", Label: "State", Content: template.HTML("<p>established</p>"), Active: true}, //nolint:gosec // fixed test fixture
			{Key: "routes", Label: "Routes", Content: template.HTML("<p>1200 routes</p>")},             //nolint:gosec // fixed test fixture
		},
		Tools: []WorkbenchDetailTool{
			{Label: "Refresh", HxPost: "/admin/bgp/peer/london/refresh", Class: "refresh"},
			{Label: "Teardown", HxPost: "/admin/bgp/peer/london/teardown", Class: "danger", Confirm: "Reset the session?"},
		},
	}
}

func webDashboardData(populated bool) dashboardData {
	data := dashboardData{
		System: DashboardSystemPanel{
			Hostname: "edge1", Uptime: "3d 4h", Version: "1.2.3",
			BuildDate: "2026-01-01", CPUCount: 8,
			MemTotal: "16 GiB", MemAlloc: "412 MiB",
		},
		BGP:        DashboardBGPPanel{Empty: true, HintURL: "/show/bgp/peer/"},
		Interfaces: DashboardIfacePanel{Empty: true, HintURL: "/show/interface/"},
	}

	if !populated {
		data.System = DashboardSystemPanel{Uptime: "1m", Version: "1.2.3", CPUCount: 1, MemAlloc: "20 MiB"}
		return data
	}

	data.BGP = DashboardBGPPanel{Established: 3, Active: 1, Idle: 1, Down: 2, Total: 7}
	data.Interfaces = DashboardIfacePanel{Up: 4, Down: 1, AdminDown: 2, Total: 7}
	data.Warnings = []DashboardEvent{{Time: "12:00:01", Component: "bgp", Message: "peer flap"}}
	data.Errors = []DashboardEvent{{Time: "12:00:02", Component: "iface", Message: "link down"}}

	return data
}

func webDashboardHealth(populated bool) dashboardHealthData {
	data := dashboardHealthData{
		Title:        "Component Health",
		Columns:      webWorkbenchColumns(),
		EmptyMessage: "No components report health",
	}

	if populated {
		data.Rows = []WorkbenchTableRow{
			{Key: "bgp", FlagClass: "green", Cells: []string{"UP", "bgp", "7 peers"}},
			{Key: "iface", FlagClass: "red", Cells: []string{"DOWN", "iface", "1 link down"}},
		}
	}

	return data
}

func webDashboardEvents(populated bool) dashboardEventsData {
	data := dashboardEventsData{
		Title:        "Recent Events",
		Columns:      webWorkbenchColumns(),
		Namespaces:   []string{"bgp", "iface"},
		SelectedNS:   "bgp",
		EmptyMessage: "No events recorded",
		EmptyHint:    "Events appear once a component reports one",
	}

	if populated {
		data.Rows = []WorkbenchTableRow{
			{Key: "1", Cells: []string{"12:00:01", "bgp", "peer flap"}},
			{Key: "2", Cells: []string{"12:00:02", "iface", "link down"}},
		}
	}

	return data
}

func webLogTable(populated bool) logTableData {
	data := logTableData{
		Title:        "Warnings",
		Columns:      webWorkbenchColumns(),
		EmptyMessage: "No warnings recorded",
		EmptyHint:    "Warnings appear once a component reports one",
	}

	if populated {
		data.Rows = []WorkbenchTableRow{
			{Key: "1", FlagClass: "yellow", Cells: []string{"12:00:01", "bgp", "peer flap"}},
			{Key: "2", Cells: []string{"12:00:02", "iface", "link down"}},
		}
	}

	return data
}

// --- the harness --------------------------------------------------------

// TestWebGoldenOutput captures the rendered bytes of every web template and
// compares them against the committed fixtures.
//
// VALIDATES: the web template set renders byte for byte what it rendered when
// the fixtures were captured.
// PREVENTS: a rendering-engine change that keeps every substring assertion
// green and still moves the bytes an operator receives. render_test.go asserts
// with strings.Contains, which cannot see a byte-level change that keeps the
// asserted substring.
func TestWebGoldenOutput(t *testing.T) {
	root := filepath.Join("testdata", "golden")
	if !golden.Updating() {
		if _, statErr := os.Stat(root); statErr != nil {
			t.Fatalf("fixture directory %s is missing; capture it with -update-golden: %v", root, statErr)
		}
	}

	// A templ component is a Go function, so the spec holds the component
	// itself and rendering it needs no renderer at all.
	written := webCaptureSet(t, webTemplGolden, root,
		func(file, name string, data any) ([]byte, error) {
			component, ok := data.(templ.Component)
			if !ok {
				return nil, fmt.Errorf("variant of %q in %s carries %T, not a templ.Component", name, file, data)
			}

			var buf bytes.Buffer
			err := component.Render(context.Background(), &buf)

			return buf.Bytes(), err
		})

	// A component deleted from the tree takes its spec entry with it. Its
	// fixture stays on disk, where the next reader counts bytes nobody
	// compares.
	golden.AssertCoversDir(t, root, "webTemplGoldenSpec", written)
}

// webCaptureSet renders every unit of one set and returns the fixture paths it
// wrote, so the caller can judge the whole tree once.
func webCaptureSet(t *testing.T, set golden.Set, root string,
	render func(file, name string, data any) ([]byte, error),
) []string {
	t.Helper()

	files := set.Files(t)
	set.AssertCoversFS(t, files)

	written := make([]string, 0, len(files))

	for _, file := range files {
		for _, unit := range set.Spec[file] {
			content := false

			for _, variant := range unit.Variants {
				name := unit.FixtureName(variant)
				written = append(written, set.FixturePath(root, file, name))

				t.Run(name, func(t *testing.T) {
					got, renderErr := render(file, unit.Name, variant.Data)
					if renderErr != nil {
						t.Fatalf("render %q from %s: %v", unit.Name, file, renderErr)
					}

					if strings.TrimSpace(string(got)) != "" {
						content = true
					}

					golden.Compare(t, set.FixturePath(root, file, name), got)
				})
			}

			if !content && !golden.Updating() {
				t.Errorf("template %q from %s rendered only whitespace in every variant; its fixture data does not reach the markup",
					unit.Name, file)
			}
		}
	}

	return written
}

// TestWebGoldenDetailCarriesItsFields proves the detail capture is not vacuous.
// detail renders its leaf editors through the input registry, and the registry
// answers a type it does not know with the text editor. A wrong FieldMeta would
// therefore leave the wrong editor in the fixture while the capture still
// looked healthy.
//
// VALIDATES: the bytes the input registry produces for a field appear inside
// the detail fixture.
// PREVENTS: the detail capture becoming an assertion about nothing.
func TestWebGoldenDetailCarriesItsFields(t *testing.T) {
	var rendered bytes.Buffer
	if err := detail(webFragmentDataFields()).Render(context.Background(), &rendered); err != nil {
		t.Fatalf("render detail: %v", err)
	}

	for _, field := range webFragmentDataFields().Fields {
		// fieldComponent resolves the editor through the same registry the
		// detail component calls, fallback included. Only bool, enum and number
		// have an editor of their own. uint16, uint32, int, ip, prefix,
		// duration and string all land on the text editor, which reads neither
		// Min nor Max. Recorded in plan/journal/silent-fall-through.md.
		var want bytes.Buffer
		if renderErr := fieldComponent(field).Render(context.Background(), &want); renderErr != nil {
			t.Fatalf("render field for %s: %v", field.Type, renderErr)
		}

		if !bytes.Contains(rendered.Bytes(), want.Bytes()) {
			t.Errorf("detail output does not carry the rendered %s field", field.Type)
		}
	}
}
