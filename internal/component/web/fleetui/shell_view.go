// Design: docs/architecture/fleet-interface.md -- the view data of the fleet shell and how a request fills it
// Related: shell.templ -- the markup these views render
// Related: handler_shell.go -- the handler that builds a ShellView for a page
// Related: handler_tree.go -- the tree pane of the sidebar
// Related: component_view.go -- ActionView, MenuView and ObjectIconView, which these views hold
//
// Every view here holds only what the markup writes: words, addresses, counts
// and small enums. The typed-data guard refuses a field of a type declared in
// another package, so a time or a duration is turned into its words before it
// reaches a view.

package fleetui

// The names of the icons a confirmation and a chart tool draw.
const (
	iconCheck = "check"
	iconChart = "chart"
)

// The names of the icons the shell draws for each kind of object.
const (
	iconEstate = "estate"
	iconGroup  = "group"
	iconDevice = "device"
	iconTasks  = "tasks"
	iconAdmin  = "config"
	iconAlert  = "alert"
)

// The words the shell writes that more than one place needs.
const (
	productName     = "Ze Fleet"
	adminTitle      = "Administration"
	tasksTitle      = "Tasks"
	notFoundTitle   = "Not found"
	titleSeparator  = " - "
	actionsMenuWord = "Actions"
)

// The states of the side panel region, as its data attribute names them for
// the script: the page has no panel, the panel shows no row, it shows one.
const (
	panelStateNone  = "none"
	panelStateEmpty = "empty"
	panelStateOpen  = "open"
)

// panelState answers the state of the side panel region.
func panelState(view SidePanelView) string {
	if !view.Present {
		return panelStateNone
	}
	if view.Open {
		return panelStateOpen
	}

	return panelStateEmpty
}

// tabClass answers the classes of one tab of the tab row.
func tabClass(tab TabView) string {
	if tab.Active {
		return "fleet-tab fleet-tab--active"
	}

	return "fleet-tab"
}

// ShellView is everything the shell renders for one page, apart from the two
// components the page itself supplies: its content and its side panel.
type ShellView struct {
	Header  HeaderView
	Sidebar SidebarView
	Main    MainView
	Panel   SidePanelView
	Tasks   TaskPanelView
}

// HeaderView is the header region.
type HeaderView struct {
	EstateName string
	// Sample reports that the registered source answers an invented estate,
	// which the header says with a tag beside the estate's name.
	Sample bool
	// Script holds the controls of a scripted source, drawn beside the
	// sample tag. It is nil over a source that is not a Stepper, whose step
	// and hold routes are not mounted, so the header draws no control that
	// would post to nothing.
	Script   *HeaderScriptView
	Username string
	// SearchURL is the fragment the search field asks as the operator
	// types, without its item parameter.
	SearchURL string
	Tasks     HeaderTasksView
}

// HeaderTasksView is the Tasks button of the header: the count of tasks that
// are not over, and whether a task failed.
type HeaderTasksView struct {
	Running int
	Failed  bool
	// Label names the button for assistive technology, count included.
	Label string
}

// SidebarView is the sidebar region: its two tabs and what each shows.
type SidebarView struct {
	// AdminActive reports that the Administration tab is the open one,
	// which it is on an administration page.
	AdminActive bool
	Tree        TreePaneView
	Admin       []SidebarLinkView
}

// SidebarLinkView is one administration page in the sidebar.
type SidebarLinkView struct {
	Title  string
	Href   string
	Active bool
}

// TreePaneView is the Estate tab of the sidebar: the tree view selector, the
// filter box and the tree.
type TreePaneView struct {
	Options []TreeViewOptionView
	Rows    TreeRowsView
}

// TreeViewOptionView is one entry of the tree view selector. Href is the
// address of the page in hand under that view.
type TreeViewOptionView struct {
	Title    string
	Href     string
	Selected bool
}

// TreeRowsView is the tree itself: what the tree fragment answers.
type TreeRowsView struct {
	// ViewName is the name of the tree view drawn. The script keeps the
	// expansion state of the tree under it.
	ViewName string
	// Rows holds the estate row, with everything drawn under it.
	Rows []TreeRowView
	// Filter is the filter text the rows were narrowed by, as typed.
	Filter string
	// Empty reports that the filter keeps no device.
	Empty bool
}

// TreeRowView is one row of the tree: the estate, a group node or a device.
type TreeRowView struct {
	// Key identifies the row in the browser's expansion state and in the
	// address of the fragment that loads its children.
	Key   string
	Title string
	Href  string
	Icon  ObjectIconView
	// Busy reports that a task targets the row, so a spinner replaces the
	// mark of its state.
	Busy bool
	// Count is "connected/total" for the estate and a group node. It is
	// empty for a device.
	Count string
	// Attention reports that a device below the row is not healthy, so the
	// count takes the warning color.
	Attention bool
	// Alarms is the number of open alarms of a device row.
	Alarms     int
	Selected   bool
	Expandable bool
	Expanded   bool
	// LazyURL is the fragment that answers the row's children. It is set
	// for a row whose children were left out, and empty otherwise.
	LazyURL  string
	Menu     MenuView
	Children []TreeRowView
}

// MainView is the main pane apart from the tab's content: the object header,
// the breadcrumb and the tab row.
type MainView struct {
	// Title is the document title: the object's name, then the product.
	Title string
	// EventsURL is the address of the live event stream. It is empty for
	// an address that names no page or no object, which no stream follows.
	EventsURL string
	// PagePath is the path of the page in hand, without its query, and
	// PageAddress the same address with its view as a query. The script
	// reads both from the content region rather than from the browser's
	// location, so a page plays wherever it is shown, a recording's frame
	// included.
	PagePath    string
	PageAddress string
	Object      ObjectHeaderView
	Crumbs      []CrumbView
	Tabs        []TabView
	// TabsLabel names the tab row for assistive technology.
	TabsLabel string
	// ContentLive reports that a live event refreshes the content region.
	ContentLive bool
}

// ObjectHeaderView is the header of the object the page shows.
type ObjectHeaderView struct {
	Icon ObjectIconView
	Name string
	// Stated reports that the object has a display state, which a device
	// has and the estate, a group node and a page list do not.
	Stated bool
	State  DisplayState
	// Busy reports that a task targets the object.
	Busy bool
	// HasPrimary reports that Primary holds the object's primary action.
	HasPrimary bool
	Primary    ActionView
	// Menu holds the Actions menu. It has no item when the page registered
	// no action.
	Menu MenuView
}

// CrumbView is one step of the breadcrumb.
type CrumbView struct {
	Title string
	Href  string
	// Current marks the last step, which is the page in hand and no link.
	Current bool
}

// TabView is one tab of the tab row.
type TabView struct {
	Title  string
	Href   string
	Active bool
}

// SidePanelView is the side panel region.
type SidePanelView struct {
	// Present reports that the page has a side panel at all.
	Present bool
	// Open reports that the address names a row, whose details the panel
	// shows.
	Open bool
	// CloseHref is the page's address without its row.
	CloseHref string
	// Row is the row the panel shows while Open.
	Row string
	// PageAddress is the address of the page the region was drawn for, with
	// its query. A side panel swapped in alone carries it to the content
	// region, so the page in hand names the row it shows.
	PageAddress string
}

// SearchResultsView is what the header search answers for typed text.
type SearchResultsView struct {
	Text    string
	Results []SearchResultView
}

// SearchResultView is one device or one page the search found.
type SearchResultView struct {
	Title string
	// Detail says what the result is: a device with its state, or a page.
	Detail string
	Href   string
	Icon   string
}

// linkView answers the view parameters a link to another page keeps: the
// tree view, which stays as the operator moves around. The filter, the sort,
// the page and the row belong to the table of the page they were set on.
func linkView(request *PageRequest) ViewParams {
	return ViewParams{TreeView: request.Address.View.TreeView}
}

// estateAddress answers the address of the estate dashboard under the tree
// view of the request.
func estateAddress(request *PageRequest) Address {
	return Address{Kind: ObjectEstate, Tab: TabDashboard, View: linkView(request)}
}

// groupAddress answers the address of the dashboard of the group node at
// path.
func groupAddress(request *PageRequest, path []GroupLevel) Address {
	return Address{Kind: ObjectGroup, Group: path, Tab: TabDashboard, View: linkView(request)}
}

// deviceAddress answers the address of the dashboard of the device named
// name.
func deviceAddress(request *PageRequest, name string) Address {
	return Address{Kind: ObjectDevice, Device: name, Tab: TabDashboard, View: linkView(request)}
}

// groupTitle answers what a group node shows for one level of its path: the
// value its devices share, or "(no <key>)" for the devices that lack the key.
// It is the wording BuildTree gives the node.
func groupTitle(level GroupLevel) string {
	if level.Missing {
		return "(no " + level.Key + ")"
	}

	treeLevel := levelOf(level.Key)

	return treeLevel.title(level.Value)
}

// devicePath answers the path of the node a device sits under in the tree
// view of the request: one level per level of the view.
func devicePath(request *PageRequest, view *DeviceView) []GroupLevel {
	path := make([]GroupLevel, 0, len(request.treeView.Levels))
	for _, key := range request.treeView.Levels {
		treeLevel := levelOf(key)
		value, present := treeLevel.Value(view)
		path = append(path, GroupLevel{Key: key, Value: value, Missing: !present})
	}

	return path
}

// SampleSource is implemented by a source whose estate is invented, so the
// pages show sample data rather than a real hub's. The shell discovers it by
// type assertion, as it discovers Stepper: a source that does not implement
// it is a real one, and the header carries no tag.
type SampleSource interface {
	// SampleData marks the source as answering sample data. It does nothing.
	SampleData()
}

// HeaderScriptView is the prototype control group of the header: Step and
// Hold, posting to the prototype-only routes, so the estate can be changed one
// event at a time from any page while it is reviewed.
type HeaderScriptView struct {
	StepAction string
	HoldAction string
}

// headerView fills the header region.
func headerView(request *PageRequest) HeaderView {
	_, sample := request.Source.(SampleSource)

	return HeaderView{
		EstateName: request.Snapshot.Estate.Name,
		Sample:     sample,
		Script:     headerScriptView(request.Source),
		Username:   request.Username,
		SearchURL:  FragmentLink(fragmentSearch, estateAddress(request), ""),
		Tasks:      headerTasksView(request),
	}
}

// headerScriptView answers the script controls of the header, or nil over a
// source that is not a Stepper: the step and hold routes mount only over one
// (stepperRegistered, NewRoutes), as the states page's controls do.
func headerScriptView(source Source) *HeaderScriptView {
	if _, steps := source.(Stepper); !steps {
		return nil
	}

	return &HeaderScriptView{StepAction: stepPath, HoldAction: holdPath}
}

// headerTasksView counts the tasks that are not over, which is the figure the
// Tasks button shows, and looks for a task that failed, which is its dot.
func headerTasksView(request *PageRequest) HeaderTasksView {
	view := HeaderTasksView{}

	tasks := request.Snapshot.Tasks
	for index := range tasks {
		state := tasks[index].State
		if state.Active() {
			view.Running++
		}
		if taskFailed(state) {
			view.Failed = true
		}
	}

	label := "Tasks, " + countText(view.Running) + " running"
	if view.Failed {
		label += ", a task failed"
	}
	view.Label = label

	return view
}

// taskFailed reports whether a task in this state failed on at least one
// device.
func taskFailed(state TaskState) bool {
	// The other states are a task that is not over, one that succeeded
	// everywhere, and one an operator stopped.
	//exhaustive:ignore
	switch state {
	case TaskStateFailed, TaskStatePartlyFailed:
		return true
	}

	return false
}

// sidebarView fills the sidebar region. selects is false for an address that
// names no page, which marks no row of the tree.
func sidebarView(request *PageRequest, selects bool) (SidebarView, error) {
	tree, err := treePaneView(request, selects)
	if err != nil {
		return SidebarView{}, err
	}

	address := request.Address
	onAdmin := selects && address.Kind == ObjectAdmin

	pages := Tabs(ObjectAdmin)
	links := make([]SidebarLinkView, 0, len(pages))
	for _, page := range pages {
		target := Address{Kind: ObjectAdmin, Tab: page.Name, View: linkView(request)}
		links = append(links, SidebarLinkView{
			Title:  page.Title,
			Href:   target.String(),
			Active: onAdmin && page.Name == address.Tab,
		})
	}

	return SidebarView{AdminActive: onAdmin, Tree: tree, Admin: links}, nil
}

// tabViews answers the tab row of the object the request shows: one tab per
// registered tab of its kind, each a link that keeps the object and the tree
// view. The task list has no tab.
func tabViews(request *PageRequest) []TabView {
	address := request.Address

	tabs := Tabs(address.Kind)
	views := make([]TabView, 0, len(tabs))
	for _, tab := range tabs {
		target := Address{
			Kind: address.Kind, Group: address.Group, Device: address.Device,
			Tab: tab.Name, View: linkView(request),
		}
		views = append(views, TabView{Title: tab.Title, Href: target.String(), Active: tab.Name == address.Tab})
	}

	return views
}

// tabTitle answers the title of the tab the address names, or its segment
// when no tab of that name is registered for the kind.
func tabTitle(address Address) string {
	for _, tab := range Tabs(address.Kind) {
		if tab.Name == address.Tab {
			return tab.Title
		}
	}

	return address.Tab
}

// estateCrumb answers the first step of every breadcrumb: the estate, a link,
// because the estate page itself shows no breadcrumb.
func estateCrumb(request *PageRequest) CrumbView {
	return CrumbView{Title: request.Snapshot.Estate.Name, Href: estateAddress(request).String()}
}

// groupCrumbs answers one step per level of path, each the group node of the
// path so far. last marks the final step as the page in hand.
func groupCrumbs(request *PageRequest, path []GroupLevel, last bool) []CrumbView {
	crumbs := make([]CrumbView, 0, len(path))
	for index, level := range path {
		crumbs = append(crumbs, CrumbView{
			Title:   groupTitle(level),
			Href:    groupAddress(request, path[:index+1]).String(),
			Current: last && index == len(path)-1,
		})
	}

	return crumbs
}

// objectMenu answers the Actions menu of the object at address: the actions
// the page of its dashboard registered. A kind whose dashboard has no page,
// or whose page registered no action, has a menu with no item, which the
// markup leaves out.
func objectMenu(request *PageRequest, address Address, name string) MenuView {
	menu := MenuView{Label: actionsMenuWord + " for " + name}

	page, built := request.pages.lookup(address.Kind, TabDashboard)
	if !built {
		return menu
	}
	if page.Actions == nil {
		return menu
	}

	target := *request
	target.Address = address
	menu.Items = page.Actions(&target)

	return menu
}

// objectPrimary answers the primary action the page in hand registered for
// its object, or false.
func objectPrimary(request *PageRequest) (ActionView, bool) {
	page, built := request.pages.lookup(request.Address.Kind, request.Address.Tab)
	if !built {
		return ActionView{}, false
	}
	if page.Primary == nil {
		return ActionView{}, false
	}

	return page.Primary(request)
}
