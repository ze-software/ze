// Related: render.go -- NewRenderer builds the template sets captured here
// Related: fragment.go -- FragmentData, the view model most fragments render

package web

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/test/golden"
)

// webGolden is the golden harness over the web template tree. The spec below,
// the fixture data and webExecuteGolden stay in this package. The walk over the
// FS, the coverage check, the fixture path rule and the byte comparison are
// shared with internal/component/lg through internal/test/golden.
//
// Recapture a deliberate markup change with `make ze-web-golden-update`.
var webGolden = golden.Set{
	FS:      templatesFS,
	Dir:     "templates",
	Spec:    webGoldenSpec,
	SpecVar: "webGoldenSpec",
}

// webGoldenSpec maps each file in the embedded template FS to the templates it
// defines and the data each one renders with. TestWebGoldenOutput compares this
// map against the FS and fails when the two disagree. A new template file, or a
// new {{define}} inside an existing file, fails until it is captured here.
var webGoldenSpec = golden.Spec{
	// --- page templates -------------------------------------------------
	"templates/page/layout.html": {{
		Name: "layout.html",
		Variants: []golden.Variant{
			{Name: "finder", Data: webLayoutData("finder")},
			{Name: "cli", Data: webLayoutData("cli")},
		},
	}},
	"templates/page/workbench.html": {{
		Name: "workbench.html",
		Variants: []golden.Variant{
			{Name: "full", Data: webWorkbenchData(false)},
			{Name: "readonly", Data: webWorkbenchData(true)},
		},
	}},
	"templates/page/login.html": {{
		Name: "login.html",
		Variants: []golden.Variant{
			{Name: "page", Data: LoginData{}},
			{Name: "page-error", Data: LoginData{
				Error: "invalid credentials", ReturnTo: "/show/bgp/", Locale: "fr",
			}},
			{Name: "overlay", Data: LoginData{
				Overlay: true, Error: "session expired", ReturnTo: "/show/", Locale: "en",
			}},
		},
	}},

	// --- config view templates ------------------------------------------
	"templates/container.html": {{
		Name: "container.html",
		Variants: []golden.Variant{
			{Name: "full", Data: webConfigViewData()},
			{Name: "empty", Data: &ConfigViewData{CurrentPath: "bgp"}},
		},
	}},
	"templates/list.html": {{
		Name: "list.html",
		Variants: []golden.Variant{
			{Name: "selected", Data: webConfigListData(true)},
			{Name: "unselected", Data: webConfigListData(false)},
		},
	}},
	"templates/inline_list.html": {{
		Name: "inline_list.html",
		Variants: []golden.Variant{
			{Name: "selected", Data: webConfigListData(true)},
			{Name: "unselected", Data: webConfigListData(false)},
		},
	}},
	"templates/freeform.html": {{
		Name: "freeform.html",
		Variants: []golden.Variant{
			{Name: "entries", Data: &ConfigViewData{
				CurrentPath: "system/banner",
				Entries:     []string{"first line", "second line"},
			}},
			{Name: "empty", Data: &ConfigViewData{CurrentPath: "system/banner"}},
		},
	}},
	// flex.html reads .Value, .Name and .LeafField, which ConfigViewData does
	// not carry, so the live path renders nothing. nodeKindToTemplate
	// (handler_config_leaf.go) answers "flex.html" for config.NodeFlex, and
	// RenderConfigToHTML (render.go) discards the field error and returns empty
	// HTML. The capture uses a map holding the fields the markup reads, so the
	// markup is on record for the port. Recorded in
	// plan/journal/silent-fall-through.md.
	"templates/flex.html": {{
		Name: "flex.html",
		Variants: []golden.Variant{
			{Name: "flag", Data: map[string]any{"Name": "multipath", "Children": nil, "Value": ""}},
			{Name: "value", Data: map[string]any{
				"Name": "hold-time", "Children": nil, "Value": "180",
				"LeafField": webLeafField("number"),
			}},
			{Name: "block", Data: map[string]any{
				"Name": "graceful-restart", "Value": "",
				"LeafFields": []LeafField{webLeafField("text"), webLeafField("checkbox")},
				"Children": []ChildEntry{
					{Name: "timers", Kind: "container", URL: "/show/bgp/timers/", HxPath: "bgp/timers"},
				},
			}},
		},
	}},
	"templates/breadcrumb.html": {{
		Name: "breadcrumb.html",
		Variants: []golden.Variant{
			{Name: "back", Data: map[string]any{
				"BackURL":  "/show/bgp/",
				"Segments": webBreadcrumbs(),
			}},
			{Name: "root", Data: map[string]any{
				"BackURL":  "",
				"Segments": webBreadcrumbs()[:1],
			}},
		},
	}},
	"templates/commit.html": {{
		Name: "commit.html",
		Variants: []golden.Variant{
			{Name: "diff", Data: map[string]any{
				"Error": "", "ConflictPaths": nil,
				"Diff": "+ bgp peer 192.0.2.1",
				"DiffLines": []map[string]any{
					{"IsAdd": true, "IsDel": false, "IsChange": false, "Text": "+ peer 192.0.2.1"},
					{"IsAdd": false, "IsDel": true, "IsChange": false, "Text": "- peer 192.0.2.2"},
					{"IsAdd": false, "IsDel": false, "IsChange": true, "Text": "~ hold-time 90"},
					{"IsAdd": false, "IsDel": false, "IsChange": false, "Text": "  bgp"},
				},
			}},
			{Name: "conflict", Data: map[string]any{
				"Error":         "config changed under you",
				"ConflictPaths": []string{"bgp/peer/192.0.2.1", "bgp/local-as"},
				"Diff":          "",
				"DiffLines":     nil,
			}},
			{Name: "empty", Data: map[string]any{
				"Error": "", "ConflictPaths": nil, "Diff": "", "DiffLines": nil,
			}},
		},
	}},
	"templates/notification.html": {{
		Name: "notification.html",
		Variants: []golden.Variant{
			{Name: "changes", Data: map[string]any{
				"ChangeCount": 3, "Messages": []string{"peer added", "commit pending"},
			}},
			{Name: "one", Data: map[string]any{"ChangeCount": 1, "Messages": nil}},
			{Name: "none", Data: map[string]any{"ChangeCount": 0, "Messages": nil}},
		},
	}},
	"templates/command.html": {{
		Name: "command.html",
		Variants: []golden.Variant{
			{Name: "ok", Data: CommandResultData{
				CommandName: "peer 192.0.2.1 teardown", Output: "session reset", Error: false,
			}},
			{Name: "error", Data: CommandResultData{
				CommandName: "peer 192.0.2.9 teardown", Output: "no such peer", Error: true,
			}},
		},
	}},
	"templates/command_form.html": {{
		Name: "command_form.html",
		Variants: []golden.Variant{
			{Name: "parameters", Data: webCommandForm()},
			{Name: "bare", Data: CommandFormData{
				CommandName: "restart", ActionURL: "/admin/restart",
			}},
		},
	}},
	"templates/leaf_input.html": {{
		Name: "leaf_input",
		Variants: []golden.Variant{
			{Name: "text", Data: webLeafField("text")},
			{Name: "checkbox", Data: webLeafField("checkbox")},
			{Name: "select", Data: webLeafField("select")},
			{Name: "number", Data: webLeafField("number")},
			{Name: "modified", Data: webLeafFieldModified()},
		},
	}},
	// notification_banner.html and terminal.html are in the embedded FS and no
	// renderer parses them: NewRenderer (render.go) names every file it loads
	// and neither appears. Their markup lives on as Go string literals, in
	// notificationBannerSource (sse.go) and in cli_terminal.go. The capture
	// parses each one on its own so the port has their bytes. Recorded in
	// plan/journal/unwired-feature.md.
	"templates/notification_banner.html": {{
		Name: "notification_banner.html",
		Variants: []golden.Variant{{Data: map[string]any{
			"Reason": "Config changed by admin: peer added", "RefreshURL": "/show/",
		}}},
	}},
	"templates/terminal.html": {{
		Name:     "terminal.html",
		Variants: []golden.Variant{{Data: map[string]any{"CLIPrompt": "ze[bgp]# "}}},
	}},

	// --- l2tp page templates --------------------------------------------
	"templates/l2tp/list.html": {{
		Name: "list.html",
		Variants: []golden.Variant{
			{Name: "sessions", Data: webL2TPListData(true)},
			{Name: "empty", Data: webL2TPListData(false)},
		},
	}},
	"templates/l2tp/detail.html": {{
		Name: "detail.html",
		Variants: []golden.Variant{
			{Name: "events", Data: webL2TPDetailData(true)},
			{Name: "no-events", Data: webL2TPDetailData(false)},
		},
	}},

	// --- fragment templates ---------------------------------------------
	"templates/component/add_form_overlay.html": {{
		Name: "add_form_overlay",
		Variants: []golden.Variant{
			{Name: "keyed", Data: webAddFormData(false)},
			{Name: "keyless", Data: webAddFormData(true)},
		},
	}},
	"templates/component/breadcrumb.html": {
		{Name: "breadcrumb_nav", Variants: webBreadcrumbVariants()},
		{Name: "breadcrumb_inner", Variants: webBreadcrumbVariants()},
		{Name: "topbar_actions", Variants: []golden.Variant{
			{Name: "user", Data: webFragmentData()},
			{Name: "insecure", Data: webFragmentDataInsecure()},
		}},
	},
	"templates/component/cli_bar.html": {{
		Name:     "cli_bar",
		Variants: []golden.Variant{{Data: webLayoutData("finder")}},
	}},
	"templates/component/command_form.html": {{
		Name:     "command_form",
		Variants: []golden.Variant{{Data: webFragmentDataCommandForm()}},
	}},
	"templates/component/command_result.html": {{
		Name: "command_result",
		Variants: []golden.Variant{
			{Name: "ok", Data: webFragmentDataCommandResult(false)},
			{Name: "error", Data: webFragmentDataCommandResult(true)},
		},
	}},
	"templates/component/commit_bar.html": {{
		Name: "commit_bar",
		Variants: []golden.Variant{
			{Name: "changes", Data: LayoutData{ChangeCount: 4}},
			{Name: "one", Data: LayoutData{ChangeCount: 1}},
			{Name: "clean", Data: LayoutData{ChangeCount: 0}},
			{Name: "readonly", Data: LayoutData{ChangeCount: 4, ReadOnly: true}},
		},
	}},
	"templates/component/dashboard_events.html": {{
		Name: "dashboard_events",
		Variants: []golden.Variant{
			{Name: "rows", Data: webDashboardEvents(true)},
			{Name: "empty", Data: webDashboardEvents(false)},
		},
	}},
	"templates/component/dashboard_health.html": {{
		Name: "dashboard_health",
		Variants: []golden.Variant{
			{Name: "rows", Data: webDashboardHealth(true)},
			{Name: "empty", Data: webDashboardHealth(false)},
		},
	}},
	"templates/component/dashboard_overview.html": {{
		Name: "dashboard_overview",
		Variants: []golden.Variant{
			{Name: "full", Data: webDashboardData(true)},
			{Name: "empty", Data: webDashboardData(false)},
		},
	}},
	"templates/component/detail.html": {{
		Name: "detail",
		Variants: []golden.Variant{
			{Name: "fields", Data: webFragmentDataFields()},
			{Name: "list-table", Data: webFragmentDataListTable(false)},
			{Name: "command-form", Data: webFragmentDataCommandForm()},
			{Name: "command-result", Data: webFragmentDataCommandResult(false)},
			{Name: "hint", Data: &FragmentData{CurrentPath: "bgp"}},
		},
	}},
	"templates/component/diff_modal.html": {
		{Name: "diff_modal", Variants: []golden.Variant{{Data: nil}}},
		{Name: "diff_modal_open", Variants: []golden.Variant{
			{Name: "diff", Data: commitModalData{
				Diff: "+ bgp peer 192.0.2.1\n- bgp peer 192.0.2.2", ChangeCount: 2,
			}},
			{Name: "clean", Data: commitModalData{}},
		}},
	},
	"templates/component/error_panel.html": {{
		Name:     "error_panel",
		Variants: []golden.Variant{{Data: nil}},
	}},
	"templates/component/finder.html": {{
		Name:     "finder",
		Variants: webFinderVariants(),
	}},
	"templates/component/finder_oob.html": {{
		Name:     "finder_oob",
		Variants: webFinderVariants(),
	}},
	"templates/component/list_table.html": {{
		Name: "list_table",
		Variants: []golden.Variant{
			{Name: "editable", Data: webFragmentDataListTable(false)},
			{Name: "readonly", Data: webFragmentDataListTable(true)},
		},
	}},
	"templates/component/log_live.html": {{
		Name:     "log_live",
		Variants: []golden.Variant{{Data: nil}},
	}},
	"templates/component/log_table.html": {{
		Name: "log_table",
		Variants: []golden.Variant{
			{Name: "rows", Data: webLogTable(true)},
			{Name: "empty", Data: webLogTable(false)},
		},
	}},
	"templates/component/notification_error.html": {{
		Name: "notification_error",
		Variants: []golden.Variant{{Data: struct{ Message string }{
			Message: "commit rejected: hold-time out of range",
		}}},
	}},
	"templates/component/oob_error.html": {
		{Name: "oob_error", Variants: webErrorDataVariants()},
		{Name: "error_item", Variants: webErrorDataVariants()},
	},
	"templates/component/oob_response.html": {
		{Name: "oob_response", Variants: webOOBResponseVariants()},
		{Name: "full_content", Variants: webOOBResponseVariants()},
	},
	"templates/component/oob_save.html": {{
		Name: "oob_save_ok",
		Variants: []golden.Variant{
			{Name: "changes", Data: struct{ ChangeCount int }{ChangeCount: 4}},
			{Name: "one", Data: struct{ ChangeCount int }{ChangeCount: 1}},
			{Name: "clean", Data: struct{ ChangeCount int }{ChangeCount: 0}},
		},
	}},
	"templates/component/path_bar.html": {{
		Name: "path_bar_inner",
		Variants: []golden.Variant{
			{Name: "segments", Data: webFragmentData()},
			{Name: "root", Data: &FragmentData{}},
		},
	}},
	"templates/component/sidebar.html": {
		{Name: "sidebar", Variants: []golden.Variant{
			{Name: "nested", Data: webFragmentData()},
			{Name: "root", Data: &FragmentData{Sidebar: webSidebarSections()}},
		}},
		{Name: "sidebar_section", Variants: []golden.Variant{
			{Name: "list", Data: webSidebarSections()[0]},
			{Name: "container", Data: webSidebarSections()[1]},
		}},
	},
	"templates/component/tool_bgp_decode.html": {{
		Name: "tool_bgp_decode", Variants: webToolPageVariants(),
	}},
	"templates/component/tool_capture.html": {{
		Name: "tool_capture", Variants: webToolPageVariants(),
	}},
	"templates/component/tool_metrics.html": {{
		Name: "tool_metrics", Variants: webToolPageVariants(),
	}},
	"templates/component/tool_ping.html": {{
		Name: "tool_ping", Variants: webToolPageVariants(),
	}},
	"templates/component/tool_overlay.html": {{
		Name: "tool_overlay",
		Variants: []golden.Variant{
			{Name: "result", Data: webToolOverlay(ToolOverlayResult, false)},
			{Name: "overflow", Data: webToolOverlay(ToolOverlayResult, true)},
			{Name: "error", Data: webToolOverlay(ToolOverlayError, false)},
			{Name: "confirm", Data: webToolOverlay(ToolOverlayConfirm, false)},
		},
	}},
	"templates/component/workbench_dashboard.html": {{
		Name: "workbench_dashboard",
		Variants: []golden.Variant{
			{Name: "full", Data: webDashboardData(true)},
			{Name: "empty", Data: webDashboardData(false)},
		},
	}},
	"templates/component/workbench_detail.html": {{
		Name: "workbench_detail",
		Variants: []golden.Variant{
			{Name: "tabs", Data: webWorkbenchDetail(true)},
			{Name: "bare", Data: webWorkbenchDetail(false)},
		},
	}},
	"templates/component/workbench_form.html": {{
		Name: "workbench_form",
		Variants: []golden.Variant{
			{Name: "fields", Data: webWorkbenchForm(true)},
			{Name: "bare", Data: webWorkbenchForm(false)},
		},
	}},
	"templates/component/workbench_nav.html": {{
		Name:     "workbench_nav",
		Variants: []golden.Variant{{Data: webWorkbenchData(false)}},
	}},
	"templates/component/workbench_table.html": {{
		Name: "workbench_table",
		Variants: []golden.Variant{
			{Name: "rows", Data: webWorkbenchTable("rows")},
			{Name: "empty", Data: webWorkbenchTable("empty")},
			{Name: "add-actions", Data: webWorkbenchTable("add-actions")},
		},
	}},
	"templates/component/workbench_topbar.html": {{
		Name: "workbench_topbar",
		Variants: []golden.Variant{
			{Name: "fleet", Data: webWorkbenchData(false)},
			{Name: "single", Data: webWorkbenchData(true)},
		},
	}},

	// --- leaf input templates -------------------------------------------
	"templates/input/bool.html": {{
		Name: "input_bool",
		Variants: []golden.Variant{
			{Name: "true", Data: webFieldMeta("bool", "true")},
			{Name: "false", Data: webFieldMeta("bool", "false")},
			{Name: "unset", Data: webFieldMeta("bool", "")},
		},
	}},
	"templates/input/enum.html": {{
		Name: "input_enum",
		Variants: []golden.Variant{
			{Name: "selected", Data: webFieldMeta("enum", "igp")},
			{Name: "unset", Data: webFieldMetaBare("enum")},
		},
	}},
	"templates/input/number.html": {{
		Name: "input_number",
		Variants: []golden.Variant{
			{Name: "set", Data: webFieldMeta("uint16", "180")},
			{Name: "default", Data: webFieldMeta("uint16", "")},
			{Name: "bare", Data: webFieldMetaBare("uint16")},
		},
	}},
	"templates/input/text.html": {{
		Name: "input_text",
		Variants: []golden.Variant{
			{Name: "set", Data: webFieldMeta("string", "london")},
			{Name: "default", Data: webFieldMeta("string", "")},
			{Name: "bare", Data: webFieldMetaBare("string")},
		},
	}},
	"templates/input/wrapper.html": {
		{Name: "field_wrapper_start", Variants: []golden.Variant{
			{Name: "annotated", Data: webFieldMeta("string", "london")},
			{Name: "bare", Data: webFieldMetaBare("string")},
		}},
		{Name: "field_wrapper_end", Variants: []golden.Variant{
			{Data: webFieldMeta("string", "london")},
		}},
	},
}

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

func webBreadcrumbVariants() []golden.Variant {
	return []golden.Variant{
		{Name: "deep", Data: webFragmentData()},
		{Name: "one", Data: &FragmentData{
			Breadcrumbs: webBreadcrumbs()[:1], Username: "admin", ActiveUI: "finder",
		}},
		{Name: "none", Data: &FragmentData{ActiveUI: "finder"}},
	}
}

func webFinderVariants() []golden.Variant {
	return []golden.Variant{
		{Name: "columns", Data: webFragmentData()},
		{Name: "empty", Data: &FragmentData{CurrentPath: "bgp"}},
	}
}

func webOOBResponseVariants() []golden.Variant {
	monitor := webFragmentDataFields()
	monitor.Monitor = true

	return []golden.Variant{
		{Name: "fields", Data: webFragmentDataFields()},
		{Name: "monitor", Data: monitor},
	}
}

func webErrorDataVariants() []golden.Variant {
	return []golden.Variant{{Data: ErrorData{
		ID:      "42",
		Path:    "bgp/peer/london/hold-time",
		Message: "value 1 below minimum 3",
	}}}
}

func webToolPageVariants() []golden.Variant {
	return []golden.Variant{
		{Name: "form", Data: toolPageData{}},
		{Name: "output", Data: toolPageData{Output: "5 packets transmitted, 5 received"}},
		{Name: "error", Data: toolPageData{Error: "destination unreachable"}},
	}
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

func webAddFormData(keyless bool) map[string]any {
	type formField struct {
		Path        string
		Placeholder string
		Category    string
		Inherited   string
	}

	data := map[string]any{
		"AddURL":     "/config/add/bgp/peer/",
		"ListName":   "Peer",
		"Heading":    "New Peer",
		"KeyName":    "name",
		"KeyLabel":   "name",
		"KeyInputID": "field-name",
		"Keyless":    keyless,
		"DisplayKey": "",
		"Workbench":  keyless,
		"Fields": []formField{
			{Path: "remote/as", Placeholder: "peer AS", Category: "required", Inherited: "64500"},
			{Path: "remote/ip", Placeholder: "peer address", Category: "suggest"},
			{Path: "description", Placeholder: "free text", Category: "unique"},
		},
	}

	if keyless {
		data["KeyName"] = ""
		data["DisplayKey"] = "address"
	}

	return data
}

func webL2TPListData(populated bool) map[string]any {
	type sessionRow struct {
		LocalSID     uint32
		TunnelTID    uint32
		Username     string
		AssignedAddr string
		PeerAddr     string
		State        string
		Interface    string
	}

	data := map[string]any{"TunnelCount": 0, "SessionCount": 0, "Sessions": nil}
	if !populated {
		return data
	}

	data["TunnelCount"] = 1
	data["SessionCount"] = 2
	data["Sessions"] = []sessionRow{
		{LocalSID: 1, TunnelTID: 7, Username: "alice", AssignedAddr: "192.0.2.10", PeerAddr: "198.51.100.1", State: "established", Interface: "ppp0"},
		{LocalSID: 2, TunnelTID: 7, Username: "bob", AssignedAddr: "192.0.2.11", PeerAddr: "198.51.100.1", State: "establishing", Interface: "ppp1"},
	}

	return data
}

func webL2TPDetailData(events bool) map[string]any {
	type sessionEvent struct {
		Timestamp time.Time
		Type      string
		Actor     string
		Reason    string
		RTT       string
	}

	type sessionView struct {
		LocalSID       uint32
		Username       string
		State          string
		AssignedAddr   string
		PppInterface   string
		TunnelLocalTID uint32
		Family         string
	}

	data := map[string]any{
		"Session": sessionView{
			LocalSID: 1, Username: "alice", State: "established",
			AssignedAddr: "192.0.2.10", PppInterface: "ppp0",
			TunnelLocalTID: 7, Family: "ipv4",
		},
		"Login":  "alice",
		"Events": nil,
	}

	if !events {
		data["Login"] = ""
		return data
	}

	// A fixed timestamp keeps the fixture byte-stable: the template formats it
	// with Timestamp.Format, so time.Now would rewrite the fixture every run.
	stamp := time.Date(2026, time.January, 2, 15, 4, 5, 0, time.UTC)
	data["Events"] = []sessionEvent{
		{Timestamp: stamp, Type: "connect", Actor: "lac"},
		{Timestamp: stamp.Add(time.Minute), Type: "echo", RTT: "12ms"},
		{Timestamp: stamp.Add(2 * time.Minute), Type: "disconnect", Reason: "admin request"},
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
	renderer, err := NewRenderer()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	files := webGolden.Files(t)
	webGolden.AssertCoversFS(t, files)

	root := filepath.Join("testdata", "golden")
	if !golden.Updating() {
		if _, statErr := os.Stat(root); statErr != nil {
			t.Fatalf("fixture directory %s is missing; capture it with -update-golden: %v", root, statErr)
		}
	}

	written := make([]string, 0, len(files))

	for _, file := range files {
		for _, unit := range webGolden.Spec[file] {
			content := false

			for _, variant := range unit.Variants {
				name := unit.FixtureName(variant)
				written = append(written, webGolden.FixturePath(root, file, name))

				t.Run(name, func(t *testing.T) {
					var buf bytes.Buffer
					// Execute the parsed template directly rather than through
					// RenderFragment, RenderConfigToHTML, RenderField or
					// RenderL2TPTemplate. Each of those discards the execution
					// error and returns empty HTML. A capture taken through them
					// would record an empty fixture as if the template had
					// rendered.
					if err := webExecuteGolden(renderer, &buf, file, unit.Name, variant.Data); err != nil {
						t.Fatalf("render %q from %s: %v", unit.Name, file, err)
					}

					if strings.TrimSpace(buf.String()) != "" {
						content = true
					}

					golden.Compare(t, webGolden.FixturePath(root, file, name), buf.Bytes())
				})
			}

			if !content && !golden.Updating() {
				t.Errorf("template %q from %s rendered only whitespace in every variant; its fixture data does not reach the markup",
					unit.Name, file)
			}
		}
	}

	// A template deleted from the FS takes its spec entry with it. Its fixture
	// stays on disk, where the next reader counts bytes nobody compares.
	golden.AssertCoversDir(t, root, "webGoldenSpec", written)
}

// TestWebGoldenDetailCarriesItsFields proves the detail capture is not vacuous.
// detail renders its leaf inputs through the fieldFor template function, which
// discards its own error and returns empty HTML (render.go). A wrong FieldMeta
// would therefore leave the field markup out of the fixture and the capture
// would still look healthy.
//
// VALIDATES: the bytes fieldFor produces for a field appear inside the detail
// fixture.
// PREVENTS: a silently empty fieldFor turning the detail capture into an
// assertion about nothing.
func TestWebGoldenDetailCarriesItsFields(t *testing.T) {
	renderer, err := NewRenderer()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	var detail bytes.Buffer
	if err := renderer.fragments.ExecuteTemplate(&detail, "detail", webFragmentDataFields()); err != nil {
		t.Fatalf("render detail: %v", err)
	}

	for _, field := range webFragmentDataFields().Fields {
		// fieldFor tries input_<type> and falls back to input_text when no such
		// template exists, so the expected bytes follow the same rule. Only bool
		// and enum have their own template today. uint16, uint32, int, ip,
		// prefix, duration and string all land on input_text, which reads
		// neither Min nor Max. Recorded in plan/journal/silent-fall-through.md.
		input := "input_" + field.Type
		if renderer.fragments.Lookup(input) == nil {
			input = "input_text"
		}

		var want bytes.Buffer
		for _, part := range []string{"field_wrapper_start", input, "field_wrapper_end"} {
			if err := renderer.fragments.ExecuteTemplate(&want, part, field); err != nil {
				t.Fatalf("render %s for %s: %v", part, field.Type, err)
			}
		}

		if !bytes.Contains(detail.Bytes(), want.Bytes()) {
			t.Errorf("detail output does not carry the rendered %s field; fieldFor swallowed an error", field.Type)
		}
	}
}

// webExecuteGolden renders one template through the parsed set that holds it.
// The set is derived from the file's place in the embedded tree, which is how
// NewRenderer groups them.
func webExecuteGolden(r *Renderer, buf *bytes.Buffer, file, name string, data any) error {
	rel := strings.TrimPrefix(file, "templates/")
	dir := path.Dir(rel)
	base := path.Base(rel)

	switch dir {
	case "page":
		switch base {
		case "layout.html":
			return r.layout.Execute(buf, data)
		case "workbench.html":
			return r.workbench.Execute(buf, data)
		case "login.html":
			return r.login.Execute(buf, data)
		}

	case "component", "input":
		return r.fragments.ExecuteTemplate(buf, name, data)

	case segL2TP:
		t, ok := r.l2tp[base]
		if !ok {
			return fmt.Errorf("no l2tp template named %s", base)
		}

		return t.Execute(buf, data)

	case ".":
		if t, ok := r.config[base]; ok && name == base {
			return t.Execute(buf, data)
		}
		// leaf_input is parsed into every config template and has no entry of
		// its own, so reach it through one of them.
		if t := r.config["container.html"].Lookup(name); t != nil {
			return t.Execute(buf, data)
		}
		// A template file no renderer parses. Parsing it alone puts its bytes
		// on record for the port. The spec map names which files these are.
		t, err := template.New(base).ParseFS(templatesFS, file)
		if err != nil {
			return fmt.Errorf("parse standalone %s: %w", file, err)
		}

		return t.Execute(buf, data)
	}

	return fmt.Errorf("no renderer group holds template %q from %s", name, file)
}
