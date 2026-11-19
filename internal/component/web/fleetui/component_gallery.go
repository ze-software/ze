// Design: docs/architecture/fleet-interface.md -- fixed view data of the component gallery
// Related: component_gallery.templ -- draws every component from this data

package fleetui

// The gallery draws every fleet component in every state and variant, from
// the fixed data below. It reads no source and no clock, so it renders the
// same bytes every time: it is the states page of the prototype, and the
// coverage of the component tests.
//
// Every address it draws is written by the address writers (Address.String,
// FragmentLink, ActionAddress, consoleURL) and names a page, a fragment or an
// action of the sample estate the prototype serves: its devices, its groups
// and its tasks. A hand-written address drifts from the routes and becomes a
// link the server refuses; TestGalleryAddressesAreAnswered serves each one.

// The values the gallery repeats.
const (
	galleryDevice      = "edge-lon-04"
	galleryNoun        = "devices"
	galleryForm        = "fleet-gallery-bulk"
	galleryVersion     = "1.4.2"
	galleryAddress     = "10.0.4.1"
	galleryWeek        = "Last week"
	galleryEnded       = "Ended 59 minutes ago"
	galleryActor       = "thomas"
	galleryRetry       = "Retry"
	galleryDelete      = "Delete"
	galleryEdit        = "Edit"
	galleryLabelSite   = "site"
	galleryLabelRole   = "role"
	gallerySiteLondon  = "lon"
	galleryRoleEdge    = "edge"
	galleryMemory      = "Memory"
	galleryTotal       = "Total"
	galleryFieldAction = "action"
	galleryCPU         = "CPU"
	galleryStorage     = "Storage"
	galleryState       = "State"
	galleryLabels      = "Labels"
	galleryCPE         = "cpe-0113"
	galleryCore        = "core-lon-01"
	galleryOffline     = "edge-ams-02"
	galleryUsed        = "Used"
	galleryAddressName = "Management address"
	galleryGiB         = "GiB"
	galleryDevices     = "Devices"
	galleryValueDelete = "delete"
	galleryTaskRunning = "T-2050"
	galleryTaskPartly  = "T-2044"
)

// The addresses the gallery repeats, written by the address writers.
var (
	galleryDeviceHref       = galleryDeviceTab(galleryDevice, TabDashboard)
	galleryDevicesHref      = galleryEstateTab(devicesTab, ViewParams{})
	galleryTasksHref        = Address{Kind: ObjectTasks}.String()
	galleryAlarmsHref       = galleryEstateTab(activityTabAlarms, ViewParams{})
	galleryAdminHref        = Address{Kind: ObjectAdmin, Tab: tabAdminLabels}.String()
	galleryAdminDevicesHref = Address{Kind: ObjectAdmin, Tab: tabAdminDevices}.String()
)

// galleryDeviceTab answers the address of the tab of the sample device name.
func galleryDeviceTab(name, tab string) string {
	return Address{Kind: ObjectDevice, Device: name, Tab: tab}.String()
}

// galleryEstateTab answers the address of the estate tab in the view given.
func galleryEstateTab(tab string, view ViewParams) string {
	return Address{Kind: ObjectEstate, Tab: tab, View: view}.String()
}

// galleryDevicesView answers the address of the estate's devices table in
// the view given.
func galleryDevicesView(view ViewParams) string {
	return galleryEstateTab(devicesTab, view)
}

// galleryFiltered answers the address of the estate's devices table filtered
// by the one term field:value.
func galleryFiltered(field, value string) string {
	return galleryDevicesView(ViewParams{Filter: SetFilterTerm("", field, value)})
}

// galleryLondonEdge is the filter of the gallery's saved filter: the devices
// of site lon with role edge.
func galleryLondonEdge() string {
	return SetFilterTerm(SetFilterTerm("", galleryLabelSite, gallerySiteLondon), galleryLabelRole, galleryRoleEdge)
}

// GalleryMarksView holds the icons, the state marks and the small parts.
type GalleryMarksView struct {
	Icons         []string
	Illustrations []string
	States        []DisplayState
	Statuses      []Status
	Objects       []ObjectIconView
	Tags          []TagView
	Copy          CopyView
}

// GalleryControlsView holds every look and state of an action, and the menus.
type GalleryControlsView struct {
	Links   []ActionView
	Buttons []ActionView
	Primary ActionView
	Icon    ActionView
	Menu    MenuView
	Dots    MenuView
}

// GalleryCardsView holds the dashboard cards.
type GalleryCardsView struct {
	Donuts    []DonutCardView
	Figures   FiguresCardView
	Usage     []UsageBarView
	Group     UsageGroupView
	Stacked   StackedBarView
	Charts    []ChartCardView
	QuickInfo QuickInfoView
	Callout   CalloutView
	Alarms    AlarmListView
	NoAlarms  AlarmListView
}

// GalleryTablesView holds a table section in its three states: rows, no row,
// and a refused filter.
type GalleryTablesView struct {
	Full    TableSectionView
	Empty   TableSectionView
	Refused TableSectionView
}

// GalleryPanelsView holds the side panel with a row and without one.
type GalleryPanelsView struct {
	Row   PanelView
	Empty PanelView
}

// GalleryStatesView holds every state a pane shows in place of data.
type GalleryStatesView struct {
	Heroes []StateHeroView
	Stale  StaleBannerView
	Good   GoodNewsView
}

// GalleryOverlaysView holds the toasts, the confirmations and the tasks.
type GalleryOverlaysView struct {
	Toasts      []ToastView
	Routine     ConfirmView
	Destructive ConfirmView
	Openers     []ActionView
	Tasks       TaskPanelView
	NoTasks     TaskListView
}

// GalleryConsoleView holds the terminal, granted and refused, and the diff.
type GalleryConsoleView struct {
	Terminal TerminalView
	Refused  TerminalView
	Entries  []TerminalEntryView
	Diff     DiffView
}

// GalleryView is everything the gallery draws. NewGalleryView sets every
// part.
type GalleryView struct {
	Marks    *GalleryMarksView
	Controls *GalleryControlsView
	Cards    *GalleryCardsView
	Tables   *GalleryTablesView
	Panels   *GalleryPanelsView
	States   *GalleryStatesView
	Overlays *GalleryOverlaysView
	Console  *GalleryConsoleView
}

// NewGalleryView answers the fixed data of the gallery.
func NewGalleryView() GalleryView {
	return GalleryView{
		Marks:    galleryMarks(),
		Controls: galleryControls(),
		Cards:    galleryCards(),
		Tables:   galleryTables(),
		Panels:   galleryPanels(),
		States:   galleryStates(),
		Overlays: galleryOverlays(),
		Console:  galleryConsole(),
	}
}

func galleryTags() []TagView {
	return []TagView{
		{Key: galleryLabelSite, Value: gallerySiteLondon},
		{Key: galleryLabelRole, Value: galleryRoleEdge},
		{Key: "customer", Value: "acme"},
		{Key: "model", Value: "zx-4"},
		{Key: "rack", Value: "r12"},
	}
}

func galleryMarks() *GalleryMarksView {
	statuses := make([]Status, 0, 24)
	for _, state := range ConfigStates() {
		statuses = append(statuses, ConfigStatus(state))
	}

	for _, severity := range AlarmSeverities() {
		statuses = append(statuses, AlarmStatus(severity))
	}

	statuses = append(statuses,
		HealthStatus(HealthStateHealthy), HealthStatus(HealthStateDegraded), HealthStatus(HealthStateDown),
		TaskStatus(TaskStatePending), TaskStatus(TaskStateRunning), TaskStatus(TaskStateSucceeded),
		TaskStatus(TaskStateFailed), TaskStatus(TaskStateCanceled), TaskStatus(TaskStatePartlyFailed),
	)

	return &GalleryMarksView{
		Icons:         IconNames(),
		Illustrations: IllustrationNames(),
		States:        DisplayStates(),
		Statuses:      statuses,
		Objects: []ObjectIconView{
			{Icon: iconEstate},
			{Icon: iconGroup, Status: DisplayStatus(DisplayStateDegraded), Marked: true},
			{Icon: iconDevice, Status: DisplayStatus(DisplayStateHealthy), Marked: true},
			{Icon: iconDevice, Status: DisplayStatus(DisplayStateDown), Marked: true},
			{Icon: iconDevice, Status: DisplayStatus(DisplayStateOffline), Marked: true},
		},
		Tags: []TagView{
			{Key: galleryLabelSite, Value: gallerySiteLondon},
			{Key: galleryLabelRole, Value: galleryRoleEdge, Remove: &ActionView{
				Label: "Remove role=edge", Icon: glyphClose, Form: galleryForm, Name: "remove-label", Value: "role=edge",
			}},
		},
		Copy: CopyView{Text: galleryAddress, Name: galleryAddressName},
	}
}

func galleryRoutineConfirm() *ConfirmView {
	return &ConfirmView{
		Action:  "Run diagnostic on 2 devices",
		Targets: "edge-lon-01, edge-lon-04",
		Detail:  "The diagnostic reads state only. It creates one task with a result per device.",
	}
}

func galleryDestructiveConfirm() *ConfirmView {
	return &ConfirmView{
		Action:      "Restore the hub's version on edge-lon-04",
		Targets:     galleryDevice,
		Detail:      "The hub's configuration is pushed to the device and its local edit is discarded.",
		Expect:      galleryDevice,
		Destructive: true,
	}
}

func galleryControls() *GalleryControlsView {
	disabled := ActionView{
		Label: devicesWordPushConfig, Icon: iconAdmin, Disabled: true,
		Reason: "edge-ams-02 is offline: the hub cannot change a disconnected device",
	}
	destructive := ActionView{
		Label: galleryDelete, Icon: glyphTrash, Form: galleryForm, Name: galleryFieldAction, Value: galleryValueDelete,
		Confirm: galleryDestructiveConfirm(),
	}

	return &GalleryControlsView{
		Links: []ActionView{
			{Label: wordAddDevice, Icon: glyphPlus, Href: galleryAdminDevicesHref},
			{Label: galleryEdit, Icon: glyphEdit, Href: galleryDeviceHref},
			disabled,
		},
		Buttons: []ActionView{
			{Label: "Run diagnostic", Icon: glyphTerminal, Form: galleryForm, Name: galleryFieldAction, Value: "diagnostic", Confirm: galleryRoutineConfirm()},
			{Label: galleryRetry, Icon: glyphRetry, Href: galleryTasksHref},
			disabled,
			destructive,
		},
		Primary: ActionView{Label: "Search", Icon: glyphSearch, Href: galleryDevicesHref},
		Icon:    ActionView{Label: "Open the console", Icon: glyphTerminal, Href: galleryDeviceTab(galleryDevice, tabDeviceConsole)},
		Menu: MenuView{
			Label: "Actions",
			Items: []ActionView{
				{Label: "Open device", Icon: iconDevice, Href: galleryDeviceHref},
				{Label: devicesWordOpenConsole, Icon: glyphTerminal, Href: galleryDeviceTab(galleryDevice, tabDeviceConsole)},
				disabled,
			},
			Destructive: []ActionView{destructive},
		},
		Dots: MenuView{
			Label: "More actions for edge-lon-04",
			Items: []ActionView{{Label: galleryEdit, Icon: glyphEdit, Href: galleryDeviceHref}},
		},
	}
}

func galleryStateSegments() []DonutSegmentView {
	counts := []int{36, 3, 1, 3, 1, 0}
	segments := make([]DonutSegmentView, 0, len(counts))
	for index, state := range DisplayStates() {
		status := DisplayStatus(state)
		segment := DonutSegmentView{
			Word: status.Word, Count: counts[index], Tone: status.Tone,
			Href: galleryFiltered(fieldState, state.Key()),
		}
		if state == DisplayStateDiverged {
			segment.Info = "Connected, and holding a configuration edited on the device itself"
		}

		segments = append(segments, segment)
	}

	return segments
}

func galleryVersionSegments() []DonutSegmentView {
	return []DonutSegmentView{
		{Word: galleryVersion, Count: 31, Color: 1, Href: galleryFiltered(fieldVersion, galleryVersion)},
		{Word: "1.4.1", Count: 12, Color: 2, Href: galleryFiltered(fieldVersion, "1.4.1")},
		{Word: "1.3.9", Count: 1, Color: 3, Href: galleryFiltered(fieldVersion, "1.3.9")},
		{Word: "Older", Count: 0, Color: 6},
	}
}

func galleryUsageBars() []UsageBarView {
	return []UsageBarView{
		{Name: "CPU at 79%", Percent: 79, Known: true, Scale: true},
		{Name: "CPU at 80%", Percent: 80, Known: true, Scale: true},
		{Name: "CPU at 89%", Percent: 89, Known: true, Scale: true},
		{Name: "CPU at 90%", Percent: 90, Known: true, Scale: true},
		{Name: galleryStorage, Percent: 8, Known: true, Tall: true},
		{Name: galleryUsed, Percent: 46, Known: true, Tall: true, Extra: []UsagePartView{{Name: "Cache", Percent: 21}}},
		{Name: "Not reported", Scale: true},
	}
}

func galleryCharts() []ChartCardView {
	dashboard := Address{Kind: ObjectDevice, Device: galleryDevice, Tab: TabDashboard, View: ViewParams{Range: ChartRangeWeek}}

	return []ChartCardView{
		{
			Card: CardView{Title: galleryCPU, Span: 3}, Period: galleryWeek, Unit: "%",
			URL:    FragmentLink(fragmentDashboardChart, dashboard, chartCPU),
			Series: []ChartSeriesView{{Name: galleryCPU, Color: 1, Maximum: true}},
		},
		{
			Card: CardView{Title: "Traffic", Span: 3}, Period: galleryWeek, Unit: "Mbit/s",
			URL:    FragmentLink(fragmentDashboardChart, dashboard, chartTraffic),
			Series: []ChartSeriesView{{Name: "In", Color: 1}, {Name: "Out", Color: 3}},
		},
	}
}

func galleryAlarmRows() []AlarmRowView {
	return []AlarmRowView{
		{
			Severity: AlarmStatus(AlarmSeverityCritical), Device: galleryOffline, DeviceHref: galleryDeviceTab(galleryOffline, TabDashboard),
			Summary: "Component bgp is down", Since: "12 minutes ago", SinceExact: "14:02",
		},
		{
			Severity: AlarmStatus(AlarmSeverityWarning), Device: galleryCore, DeviceHref: galleryDeviceTab(galleryCore, TabDashboard),
			Summary: "Memory at 91% for 20 minutes", Since: "34 minutes ago", SinceExact: "13:40",
		},
		{
			Severity: AlarmStatus(AlarmSeverityInformational), Device: galleryCPE, DeviceHref: galleryDeviceTab(galleryCPE, TabDashboard),
			Summary: "Runs 1.3.9, two versions behind", Since: "3 days ago", SinceExact: "Monday 09:15",
		},
	}
}

func galleryQuickInfo() QuickInfoView {
	return QuickInfoView{
		Card: CardView{Title: "Quick info", Span: 12, TitleHref: galleryDeviceTab(galleryDevice, tabDeviceSystem)},
		Items: []ValueView{
			{Label: devicesWordName, Text: galleryDevice, Mono: true, Copy: true},
			{Label: galleryState, Status: DisplayStatus(DisplayStateHealthy)},
			{Label: galleryLabels, Tags: galleryTags()[:2]},
			{Label: devicesWordModel, Text: "zx-4"},
			{Label: devicesWordSerial, Text: "ZX4-0092-7731", Mono: true, Copy: true},
			{Label: devicesWordVersion, Text: galleryVersion, Mono: true},
			{Label: wordConfigTitle, Status: ConfigStatus(ConfigStateInSync)},
			{Label: devicesWordGroup, Text: "lon / edge", Icon: iconGroup, Href: Address{
				Kind: ObjectGroup, Tab: TabDashboard,
				Group: []GroupLevel{{Key: galleryLabelSite, Value: gallerySiteLondon}, {Key: galleryLabelRole, Value: galleryRoleEdge}},
			}.String()},
			{Label: galleryAddressName, Text: galleryAddress, Mono: true, Copy: true},
			{Label: devicesWordUptime, Text: "41 days"},
			{Label: devicesWordLastSeen, Text: "now"},
			{Label: "Location"},
		},
	}
}

func galleryCards() *GalleryCardsView {
	devices := DonutView{Caption: "Devices by state", Icon: iconDevice, Segments: galleryStateSegments()}
	versions := DonutView{Caption: "Software versions", Icon: iconAdmin, Segments: galleryVersionSegments()}
	none := DonutView{Caption: "Peers", Icon: iconGroup, Segments: []DonutSegmentView{
		{Word: "Established", Tone: ToneSuccess}, {Word: "Not established", Tone: ToneError},
	}}
	single := DonutView{Caption: titleInterfaces, Icon: iconDevice, Segments: []DonutSegmentView{
		{Word: "Up", Count: 12, Tone: ToneSuccess}, {Word: enumWordDown, Tone: ToneError},
	}}

	return &GalleryCardsView{
		Donuts: []DonutCardView{
			{Card: CardView{Title: galleryDevices, Span: 3, TitleHref: galleryDevicesHref}, Donuts: []DonutView{devices}},
			{Card: CardView{Title: "Software", Span: 3, Link: &LinkView{Label: "Top 4", Href: galleryDevicesHref}}, Donuts: []DonutView{versions}},
			{Card: CardView{Title: "Peers and interfaces", Span: 3}, Donuts: []DonutView{none, single}},
		},
		Figures: FiguresCardView{
			Card: CardView{Title: "Estate", Span: 3, Figure: "9 alarms", FigureTone: ToneError},
			Figures: []FigureView{
				{Caption: galleryDevices, Value: "44", Href: galleryDevicesHref},
				{Caption: "Sites", Value: "4"},
				{Caption: "Peers up", Value: "412", Unit: "of 420"},
				{Caption: "History", Value: "2.3", Unit: "TiB"},
			},
		},
		Usage: galleryUsageBars(),
		Group: UsageGroupView{
			Card: CardView{Title: "Resources", Span: 12},
			Columns: []UsageColumnView{
				{
					Title: galleryMemory, Subtitle: "Reported 20 seconds ago",
					Bar: UsageBarView{Name: galleryMemory, Percent: 72, Known: true, Scale: true, Tall: true},
					Foot: []FigureView{
						{Caption: galleryUsed, Value: "5.8", Unit: galleryGiB},
						{Caption: "Available", Value: "2.2", Unit: galleryGiB},
						{Caption: galleryTotal, Value: "8", Unit: galleryGiB},
					},
				},
				{
					Title: galleryStorage, Subtitle: "System volume",
					Bar:  UsageBarView{Name: galleryStorage, Percent: 93, Known: true, Scale: true, Tall: true},
					Foot: []FigureView{{Caption: galleryUsed, Value: "29.8", Unit: galleryGiB}, {Caption: galleryTotal, Value: "32", Unit: galleryGiB}},
				},
			},
		},
		Stacked:   StackedBarView{Name: wordConfigTitle, Segments: galleryStateSegments()},
		Charts:    galleryCharts(),
		QuickInfo: galleryQuickInfo(),
		Callout: CalloutView{
			Title: "edge-lon-04 was edited locally while out of the fleet and is holding its own configuration.",
			Text:  "Nothing has been overwritten. Edited: 3 changes, last at 02:14.",
			Link:  &LinkView{Label: "Review the difference", Href: galleryDeviceTab(galleryDevice, tabDeviceConfig)},
		},
		Alarms:   AlarmListView{Rows: galleryAlarmRows()},
		NoAlarms: AlarmListView{Empty: GoodNewsView{First: goodNewsAllGood, Second: "No alarms open"}},
	}
}

func galleryHeaders() []ColumnHeaderView {
	sorted := func(column string, descending bool) string {
		return galleryDevicesView(ViewParams{Sort: SortSpec{Column: column, Descending: descending}})
	}

	return []ColumnHeaderView{
		{Label: devicesWordDevice, Kind: ColumnKindText, SortHref: sorted(fieldName, true), Sort: SortMarkAscending},
		{Label: galleryState, Kind: ColumnKindState, SortHref: sorted(fieldState, false)},
		{Label: galleryLabels, Kind: ColumnKindTag},
		{Label: devicesWordVersion, Kind: ColumnKindText, SortHref: sorted(fieldVersion, true), Sort: SortMarkDescending},
		{Label: "Address", Kind: ColumnKindAddress},
		{Label: galleryCPU, Kind: ColumnKindNumber, SortHref: sorted(fieldCPU, false), Numeric: true},
		{Label: devicesWordAlarms, Kind: ColumnKindNumber, SortHref: sorted(fieldAlarms, false), Numeric: true},
		{Label: devicesWordLastSeen, Kind: ColumnKindTime, SortHref: sorted(fieldLastSeen, false)},
		{Label: "Config", Kind: ColumnKindState},
	}
}

func galleryRow(name string, state DisplayState, cpu int, known, opened bool) RowView {
	href := galleryDeviceTab(name, TabDashboard)
	seen := TimeCell("now", "connected")
	address := MonoCell(galleryAddress)
	if state == DisplayStateOffline {
		seen = TimeCell("2 hours ago", "today 12:02")
		address = MonoCell("")
	}

	return RowView{
		ID: name, Name: name, Opened: opened,
		EyeHref: galleryDevicesView(ViewParams{Row: name}),
		Cells: []CellView{
			ObjectCell(ObjectIconView{Icon: iconDevice, Status: DisplayStatus(state), Marked: true}, name, href),
			StatusCell(DisplayStatus(state)),
			TagsCell(galleryTags()),
			MonoCell(galleryVersion),
			address,
			UsageCell(galleryCPU, cpu, known),
			NumberCell("2"),
			seen,
			LinkCell("In sync", galleryDeviceTab(name, tabDeviceConfig)),
		},
		Menu: &MenuView{
			Label: "More actions for " + name,
			Items: []ActionView{{Label: "Open device", Icon: iconDevice, Href: href}},
		},
	}
}

func galleryQuery(id, text, matches, refusal string) *QueryRowView {
	return &QueryRowView{
		ID: id, Action: galleryDevicesHref, Text: text,
		Placeholder: "Filter devices, for example state:down site:lon",
		Matches:     matches, Refusal: refusal,
		Hidden: []HiddenFieldView{{Name: "sort", Value: "name"}},
		Menus: []MenuView{{
			Label: galleryState, Icon: glyphFilter,
			Items: []ActionView{
				{Label: enumWordDown, Href: galleryFiltered(fieldState, DisplayStateDown.Key())},
				{Label: enumWordDegraded, Href: galleryFiltered(fieldState, DisplayStateDegraded.Key())},
			},
		}},
		Saved: &MenuView{
			Label: wordFilters,
			Items: []ActionView{{Label: "London edge", Href: galleryDevicesView(ViewParams{Filter: galleryLondonEdge()})}},
		},
		Save: &ActionView{Label: "Save", Icon: glyphSave, Form: galleryForm, Name: galleryFieldAction, Value: "save-filter"},
	}
}

func galleryPager(total, number int) *PagerView {
	london := SetFilterTerm("", galleryLabelSite, gallerySiteLondon)
	first := galleryDevicesView(ViewParams{Filter: london})
	last := galleryDevicesView(ViewParams{Filter: london, Page: 3})

	return &PagerView{
		Page: Paginate(total, number, PageSizeDefault), Noun: galleryNoun, Action: galleryDevicesHref,
		FirstHref: first, PrevHref: first,
		NextHref: last, LastHref: last,
		Hidden: []HiddenFieldView{{Name: paramFilter, Value: london}},
	}
}

func galleryTables() *GalleryTablesView {
	bulk := &BulkView{
		Form: galleryForm, Action: devicesActionAddress(Address{Kind: ObjectEstate, Tab: devicesTab}),
		Filter: "site:lon", Noun: galleryNoun, Total: 124,
		Actions: []ActionView{
			{Label: "Add label", Icon: glyphPlus, Form: galleryForm, Name: galleryFieldAction, Value: "add-label", Bulk: true},
			{Label: galleryEdit, Icon: glyphEdit, Form: galleryForm, Name: galleryFieldAction, Value: glyphEdit, Bulk: true},
			{
				Label: galleryDelete, Icon: glyphTrash, Form: galleryForm, Name: galleryFieldAction, Value: galleryValueDelete, Bulk: true,
				Confirm: &ConfirmView{
					Action: "Delete the selected devices", Targets: "2 devices", Expect: "2", Destructive: true,
					Detail: "The devices leave the hub's configuration at the next commit.",
				},
			},
		},
	}
	headers := galleryHeaders()

	return &GalleryTablesView{
		Full: TableSectionView{
			Title:  galleryDevices,
			Create: &ActionView{Label: wordAddDevice, Icon: glyphPlus, Href: galleryAdminDevicesHref},
			Query:  galleryQuery("fleet-gallery-filter-full", "site:lon", "124 of 144 devices", ""),
			Bulk:   bulk,
			Pager:  galleryPager(124, 2),
			Table: TableView{
				Name: "Devices of the estate", Form: galleryForm, Controls: true, Headers: headers,
				Rows: []RowView{
					galleryRow("edge-lon-01", DisplayStateDegraded, 71, true, false),
					galleryRow(galleryDevice, DisplayStateHealthy, 12, true, true),
					galleryRow("edge-lon-02", DisplayStateDown, 93, true, false),
					galleryRow("edge-lon-03", DisplayStateOffline, 0, false, false),
				},
			},
		},
		Empty: TableSectionView{
			Title: "Devices matching a filter that keeps none",
			CreateMenu: &MenuView{Label: "Add", Icon: glyphPlus, Items: []ActionView{
				{Label: devicesWordDevice, Href: galleryAdminDevicesHref}, {Label: devicesWordLabel, Href: galleryAdminHref},
			}},
			Query: galleryQuery("fleet-gallery-filter-empty", "state:down site:ams", "0 of 144 devices", ""),
			Pager: galleryPager(0, 1),
			Table: TableView{
				Name: "Devices matching the filter", Controls: true, Headers: headers,
				Empty: &StateHeroView{
					Kind: HeroNoData, Sentence: "No devices match this filter",
					Action: &ActionView{Label: wordClearFilter, Icon: glyphClose, Href: galleryDevicesHref},
				},
			},
		},
		Refused: TableSectionView{
			Title: "Devices, with a refused filter",
			Query: galleryQuery("fleet-gallery-filter-refused", "color:red", "Not filtered",
				"The term color:red names no field. Write the term with one of: name, state, version, config, group, model."),
			Table: TableView{
				Name: "Devices, not filtered", Headers: headers[:4],
				Rows: []RowView{{ID: galleryCPE, Name: galleryCPE, Cells: []CellView{
					ObjectCell(ObjectIconView{Icon: iconDevice}, galleryCPE, galleryDeviceTab(galleryCPE, TabDashboard)),
					StatusCell(DisplayStatus(DisplayStateUnknown)),
					TagsCell(nil),
					{},
				}}},
			},
		},
	}
}

func galleryPanels() *GalleryPanelsView {
	return &GalleryPanelsView{
		Row: PanelView{
			Title: galleryDevice, CloseHref: galleryDevicesHref, Pinned: true,
			Icon: ObjectIconView{Icon: iconDevice, Status: DisplayStatus(DisplayStateHealthy), Marked: true},
			Actions: []ActionView{
				{Label: galleryEdit, Icon: glyphEdit, Href: galleryDeviceTab(galleryDevice, tabDeviceConfig)},
				{Label: galleryDelete, Icon: glyphTrash, Form: galleryForm, Name: galleryFieldAction, Value: galleryValueDelete, Confirm: galleryDestructiveConfirm()},
			},
			Menu: &MenuView{Label: "More actions for edge-lon-04", Items: []ActionView{
				{Label: devicesWordOpenConsole, Icon: glyphTerminal, Href: galleryDeviceTab(galleryDevice, tabDeviceConsole)},
			}},
			Cards: []PanelCardView{
				{Title: "Identity", Rows: []ValueView{
					{Label: devicesWordName, Text: galleryDevice, Mono: true, Copy: true},
					{Label: galleryState, Status: DisplayStatus(DisplayStateHealthy)},
					{Label: devicesWordSerial, Text: "ZX4-0092-7731", Mono: true, Copy: true},
					{Label: galleryAddressName, Text: galleryAddress, Mono: true, Copy: true},
					{Label: "Location"},
				}},
				{Title: galleryLabels, Rows: []ValueView{{Label: galleryLabels, Tags: galleryTags()[:3]}}},
				{Title: "Now", Rows: []ValueView{
					{Label: galleryCPU, Usage: &UsageBarView{Name: galleryCPU, Percent: 12, Known: true}},
					{Label: galleryMemory, Usage: &UsageBarView{Name: galleryMemory, Percent: 84, Known: true}},
					{Label: galleryStorage, Usage: &UsageBarView{Name: galleryStorage}},
				}},
				{
					Title: wordOpenAlarms, Link: &LinkView{Label: wordSeeAll, Href: galleryAlarmsHref},
					Rows: []ValueView{{Label: "Memory at 84%", Status: AlarmStatus(AlarmSeverityWarning)}},
				},
			},
		},
		Empty: PanelView{Empty: true},
	}
}

func galleryStates() *GalleryStatesView {
	retry := &ActionView{Label: galleryRetry, Icon: glyphRetry, Href: galleryDeviceTab(galleryDevice, tabDeviceRouting)}

	return &GalleryStatesView{
		Heroes: []StateHeroView{
			{
				Kind: HeroNoData, Sentence: "No devices match this filter",
				Action: &ActionView{Label: wordClearFilter, Icon: glyphClose, Href: galleryDevicesHref},
			},
			{
				Kind: HeroNothingConfigured, Sentence: "No devices are declared on this hub",
				Action: &ActionView{Label: "Declare a device", Icon: glyphPlus, Href: galleryAdminDevicesHref},
			},
			{
				Kind: HeroOffline, Sentence: "edge-lon-04 has been offline since 14:02. The values below were reported then",
				Action: &ActionView{Label: "Open the device's tasks", Icon: iconTasks, Href: galleryDeviceTab(galleryDevice, tabDeviceTasks)},
			},
			{
				Kind: HeroNotSupported, Sentence: "This device runs a version that does not answer this view",
				Detail: "edge-lon-04 runs 1.3.9.",
				Action: &ActionView{Label: "Show the device's version", Icon: glyphInfo, Href: galleryDeviceTab(galleryDevice, tabDeviceSystem)},
			},
			{
				Kind:     HeroRelayRefused,
				Sentence: "relay refused: no profile is granted to hub hub-lon-01 on this device",
				Detail:   "Add this line to the device's configuration to grant the relay:",
				Line:     "fleet { relay { profile read-only; } }", LineName: heroLineConfiguration,
			},
			{
				Kind: HeroUnauthorized, Sentence: "Your profile does not allow this view",
				Detail: "You are signed in with the profile read-only.",
			},
			{
				Kind: HeroFailed, Sentence: "The peers of edge-lon-04 could not be read: the device did not answer within 5 seconds",
				Detail: "The same data at the command line:",
				Line:   "ze show bgp summary", LineName: "the command", Action: retry,
			},
		},
		Stale: StaleBannerView{
			Sentence: "Live updates stopped at 14:02",
			Action:   &ActionView{Label: "Reconnect", Icon: glyphRetry, Href: galleryDevicesHref},
		},
		Good: GoodNewsView{First: goodNewsAllGood, Second: "Nothing needs attention"},
	}
}

func galleryTaskRows() []TaskRowView {
	eye := func(id string) string {
		return Address{Kind: ObjectTasks, View: ViewParams{Row: id}}.String()
	}

	return []TaskRowView{
		{
			Name: "Configuration push to 5 devices", Actor: galleryActor, When: "Started 2 minutes ago",
			Status: TaskStatus(TaskStateRunning), Done: 3, Total: 5, EyeHref: eye(galleryTaskRunning),
			Results: []TaskResultView{
				{Device: "edge-lon-01", Href: galleryDeviceTab("edge-lon-01", TabDashboard), Status: TaskStatus(TaskStateSucceeded), Answer: "committed a3f9", Duration: "4 s"},
				{Device: galleryDevice, Href: galleryDeviceHref, Status: TaskStatus(TaskStateRunning), Answer: "validating"},
				{Device: "edge-lon-02", Href: galleryDeviceTab("edge-lon-02", TabDashboard), Status: TaskStatus(TaskStatePending)},
			},
		},
		{
			Name: "Diagnostic run on site lon", Actor: galleryActor, When: galleryEnded,
			Status: TaskStatus(TaskStatePartlyFailed), Done: 12, Total: 12, EyeHref: eye(galleryTaskPartly),
			Results: []TaskResultView{
				{Device: "edge-lon-03", Href: galleryDeviceTab("edge-lon-03", TabDashboard), Status: TaskStatus(TaskStateFailed), Answer: "device offline", Duration: "30 s"},
			},
		},
		{Name: "Software update of cpe-0113", Actor: "hub", When: galleryEnded, Status: TaskStatus(TaskStateSucceeded), Done: 1, Total: 1},
		{Name: "Rollout batch 2 of edge-refresh", Actor: galleryActor, When: "Ended 3 hours ago", Status: TaskStatus(TaskStateCanceled)},
		{Name: "Divergence resolution on edge-ams-02", Actor: galleryActor, When: "Ended 1 day ago", Status: TaskStatus(TaskStateFailed), Done: 0, Total: 1},
		{Name: "Relayed command batch", Actor: galleryActor, When: "Queued", Status: TaskStatus(TaskStatePending), Done: 0, Total: 4},
	}
}

func galleryOverlays() *GalleryOverlaysView {
	// The task panel's tabs are fragments of the page that draws the panel,
	// which is the estate dashboard, as taskPanelTab writes them.
	estate := Address{Kind: ObjectEstate, Tab: TabDashboard}
	tab := func(key taskTab) string {
		return FragmentLink(fragmentTasks, estate, taskTabKeys[key])
	}

	return &GalleryOverlaysView{
		Toasts: []ToastView{
			{Level: ToastInformational, Message: "Diagnostic run started on 12 devices", Link: &LinkView{Label: wordOpenTask, Href: galleryTasksHref}},
			{Level: ToastSuccess, Message: "Configuration push succeeded", Detail: "5 of 5 devices committed a3f9", Link: &LinkView{Label: wordOpenTask, Href: galleryTasksHref}},
			{Level: ToastWarning, Message: "Rollout edge-refresh paused", Detail: "Batch 2 crossed its failure threshold", Link: &LinkView{Label: "Open the rollout", Href: galleryEstateTab(tabEstateRollouts, ViewParams{})}},
			{Level: ToastError, Message: "Component bgp is down on edge-ams-02", Link: &LinkView{Label: "Open the alarm", Href: galleryAlarmsHref}},
		},
		Routine:     *galleryRoutineConfirm(),
		Destructive: *galleryDestructiveConfirm(),
		Openers: []ActionView{
			{Label: "Open a routine confirmation", Icon: iconCheck, Confirm: galleryRoutineConfirm()},
			{Label: "Open a destructive confirmation", Icon: glyphTrash, Confirm: galleryDestructiveConfirm()},
		},
		Tasks: TaskPanelView{
			Open: true, Preview: true, AllHref: galleryTasksHref,
			Tabs: []TaskTabView{
				{Label: enumWordRunning, Count: 2, Href: tab(taskTabRunning), Active: true},
				{Label: enumWordSucceeded, Count: 18, Href: tab(taskTabSucceeded)},
				{Label: enumWordFailed, Count: 3, Href: tab(taskTabFailed)},
				{Label: "All", Count: 23, Href: tab(taskTabAll)},
			},
			List: TaskListView{Rows: galleryTaskRows()},
		},
		NoTasks: TaskListView{Empty: GoodNewsView{First: "Nothing running", Second: "No task targets this device"}},
	}
}

func galleryConsole() *GalleryConsoleView {
	refusal := "relay refused: no profile is granted to hub hub-lon-01 on this device.\n" +
		"Grant one with: fleet { relay { profile read-only; } }"

	return &GalleryConsoleView{
		Terminal: TerminalView{
			Device: galleryDevice, User: galleryActor, Profile: "read-only",
			URL: consoleURL(galleryDevice), ConfigHref: galleryDeviceTab(galleryDevice, tabDeviceConfig),
		},
		Refused: TerminalView{
			Device: galleryOffline, User: galleryActor, Profile: "none granted",
			URL: consoleURL(galleryOffline), Refusal: refusal,
		},
		Entries: []TerminalEntryView{
			{Command: "show version", Output: "ze 1.4.2\nuptime 41 days"},
			{Command: "show bgp summary", Output: refusal, Refused: true},
		},
		Diff: DiffView{
			LeftTitle: "Hub configuration (a3f9)", RightTitle: "Device configuration (c71e)",
			Rows: []DiffRowView{
				{Kind: DiffSame, LeftNumber: 1, RightNumber: 1, Left: "bgp {", Right: "bgp {"},
				{Kind: DiffChanged, LeftNumber: 2, RightNumber: 2, Left: "  router-id 10.0.4.1;", Right: "  router-id 10.0.4.9;"},
				{Kind: DiffRemoved, LeftNumber: 3, Left: "  peer 10.0.0.1 { }"},
				{Kind: DiffAdded, RightNumber: 3, Right: "  peer 10.0.0.7 { }"},
				{Kind: DiffSame, LeftNumber: 4, RightNumber: 4, Left: "}", Right: "}"},
			},
		},
	}
}
