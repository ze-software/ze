// Design: docs/architecture/fleet-interface.md -- the administration pages of the Fleet mode
// Related: page_admin.templ -- draws the administration pages and the device join panel
// Related: handler_config.go -- the action form and the helpers the manage pages share
//
// Every change an administration page offers is a configuration change in
// the functional interface: staged in the operator's candidate, shown as
// pending, diffed and committed. The prototype changes nothing, so each one
// opens the confirm dialog and answers the prototype's toast. Saved filters
// are the exception the spec names: per-user state, applied at once.

package fleetui

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The words the administration pages repeat.
const (
	adminAddRow     = "add"
	wordChange      = "Change"
	wordRemove      = "Remove"
	wordRename      = "Rename"
	wordAllowed     = "Allowed"
	wordNotAllowed  = "Not allowed"
	wordAddDevice   = "Add device"
	wordFilters     = "Saved filters"
	wordTreeViews   = "Tree views"
	wordTemplates   = "Templates"
	deviceNameMax   = 64
	deviceNameChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-_"
)

// pendingNote is the note every administration page that changes hub
// configuration carries.
func pendingNote() *CalloutView {
	return &CalloutView{
		Title: "Changes here are configuration changes.",
		Text: "In the functional interface each change is staged in your candidate, shown in the pending change bar, " +
			"diffed and committed: nothing is applied on save. The prototype changes nothing.",
	}
}

// changeAction answers a routine change of an administration page.
func changeAction(key, label, target, detail string) ActionView {
	return manageAction(key, label, target, &ConfirmView{Action: label, Targets: target, Detail: detail})
}

// removeAction answers a removal, confirmed by typing the target's name.
func removeAction(key, label, target, detail string) ActionView {
	action := manageAction(key, label, target, &ConfirmView{
		Action: label, Targets: target, Detail: detail, Expect: target, Destructive: true,
	})
	action.Icon = glyphTrash

	return action
}

// estatePrimary answers the primary action of the estate's object header:
// adding a device, on the administration page that declares devices. A
// group node answers none: its devices are chosen by their labels, so there
// is nothing to add to it.
func estatePrimary(request *PageRequest) (ActionView, bool) {
	if request.Address.Kind != ObjectEstate {
		return ActionView{}, false
	}
	target := Address{Kind: ObjectAdmin, Tab: tabAdminDevices, View: ViewParams{TreeView: request.Address.View.TreeView}}

	return ActionView{Label: wordAddDevice, Icon: "plus", Href: target.String()}, true
}

// estateDevicesHref answers the estate's devices table narrowed by filter.
func estateDevicesHref(request *PageRequest, filter string) string {
	return Address{Kind: ObjectEstate, Tab: devicesTab, View: ViewParams{
		Filter: filter, TreeView: request.Address.View.TreeView,
	}}.String()
}

// adminTable answers an administration page holding one table.
func adminTable(request *PageRequest, note *CalloutView, table TableSectionView) templ.Component {
	return AdminPage(AdminPageView{Note: note, Table: table, Form: manageForm(request)})
}

// connectionStatus answers a device's connection as a status.
func connectionStatus(device *Device) Status {
	if device.Connection.Connected {
		return Status{Tone: ToneSuccess, Word: "Connected"}
	}

	return Status{Tone: ToneMuted, Word: "Offline"}
}

// secretStatus answers whether a device holds its join secret as a status:
// a device without one cannot join, so its absence is a warning.
func secretStatus(device *Device) Status {
	if device.SecretSet {
		return Status{Tone: ToneSuccess, Word: "Set"}
	}

	return Status{Tone: ToneWarning, Word: "Not set"}
}

// defaultFilterCell answers the Default cell of a saved filter: the marked
// word for the default, nothing for the others.
func defaultFilterCell(filter *SavedFilter) CellView {
	if filter.Default {
		return StatusCell(Status{Tone: ToneAccent, Word: "Default"})
	}

	return TextCell("")
}

// adminDevicesContent builds the Devices page: every declared device.
func adminDevicesContent(_ context.Context, request *PageRequest) (templ.Component, error) {
	views := request.Snapshot.Views
	rows := make([]RowView, 0, len(views))
	for index := range views {
		device := &views[index].Device

		tags := make([]TagView, 0, len(device.Labels))
		for _, label := range device.Labels {
			tags = append(tags, TagView{Key: label.Key, Value: label.Value})
		}

		rows = append(rows, RowView{
			ID: device.Name, Name: device.Name,
			EyeHref: rowAddress(request, device.Name),
			Opened:  request.Address.View.Row == device.Name,
			Cells: []CellView{
				LinkCell(device.Name, deviceTabHref(request, device.Name, tabDeviceDashboard)),
				TextCell(device.Group), TagsCell(tags), StatusCell(secretStatus(device)), StatusCell(connectionStatus(device)),
			},
			Menu: &MenuView{
				Label: "Actions for " + device.Name,
				Items: []ActionView{changeAction("edit-device", "Edit", device.Name,
					"The device's group, labels and secret are changed in the hub's configuration.")},
				Destructive: []ActionView{removeAction("remove-device", wordRemove, device.Name,
					"The device leaves the hub's configuration and can no longer connect.")},
			},
		})
	}

	add := ActionView{Label: wordAddDevice, Icon: glyphPlus, Href: rowAddress(request, adminAddRow)}

	return adminTable(request, pendingNote(), TableSectionView{
		Title: "Declared devices", Create: &add,
		Table: TableView{
			Name: "Declared devices",
			Headers: []ColumnHeaderView{
				{Label: devicesWordName, Kind: ColumnKindText}, {Label: devicesWordGroup, Kind: ColumnKindText},
				{Label: devicesWordLabels, Kind: ColumnKindTag}, {Label: "Secret", Kind: ColumnKindState},
				{Label: devicesWordConnection, Kind: ColumnKindState},
			},
			Rows: rows, Controls: true,
			Empty: &StateHeroView{Kind: HeroNothingConfigured, Sentence: "The hub declares no device yet."},
		},
	}), nil
}

// adminDevicesPanel builds the side panel of the Devices page: the add form,
// a declared device's details, or the lines a new device needs to join.
func adminDevicesPanel(_ context.Context, request *PageRequest) (templ.Component, error) {
	row := request.Address.View.Row
	closeHref := rowAddress(request, "")

	view := DeviceJoinView{
		Panel:  PanelView{Title: "Add a device", CloseHref: closeHref},
		Action: request.Address.Path(),
		Hidden: treeViewHidden(request),
	}
	if row == adminAddRow {
		return DeviceJoin(view), nil
	}

	if device, err := FindDevice(request.Snapshot.Views, row); err == nil {
		return PanelContent(PanelView{
			Title: device.Device.Name, CloseHref: closeHref,
			Cards: []PanelCardView{{Title: "Declared", Rows: []ValueView{
				{Label: devicesWordName, Text: device.Device.Name, Mono: true, Copy: true},
				{Label: devicesWordGroup, Text: device.Device.Group},
				{Label: "Secret", Status: secretStatus(&device.Device)},
				{Label: devicesWordConnection, Status: connectionStatus(&device.Device)},
			}}},
		}), nil
	}

	if err := deviceNameValid(row); err != nil {
		return nil, err
	}

	view.Name = row
	view.Panel.Title = "Add " + row
	view.Lines = joinLines(row)
	view.Add = changeAction("add-device", "Add "+row, row,
		"The device is declared in the hub's configuration with a secret of its own, which is never shown again once set.")

	return DeviceJoin(view), nil
}

// deviceNameValid refuses a name the hub could not declare: empty, longer
// than deviceNameMax, or holding a character outside deviceNameChars.
func deviceNameValid(name string) error {
	if name == "" {
		return deviceNameRefused(name, "is empty")
	}
	if len(name) > deviceNameMax {
		return deviceNameRefused(name, "is longer than "+strconv.Itoa(deviceNameMax)+" characters")
	}
	if strings.Trim(name, deviceNameChars) != "" {
		return deviceNameRefused(name, "holds a character other than a letter, a digit, a dot, a dash or an underscore")
	}

	return nil
}

// deviceNameRefused is the refusal of a device name.
func deviceNameRefused(name, reason string) error {
	return opererr.New(
		textbuf.New().Str("The device name ").Quoted(name).Str(" ").Str(reason).Str(".").String(),
		"A device's name is its identity on the hub and in its configuration block, so it must be one word.",
		"type a name of letters, digits, dots, dashes and underscores")
}

// joinLines answers the configuration block a device needs to join the hub
// (docs/guide/fleet-config.md, "Write Each Client's Configuration"). The
// hub's address and port are the operator's to write, and the secret is a
// placeholder: it is never shown once set.
func joinLines(name string) string {
	return textbuf.New().
		Str("plugin {\n    hub {\n        client ").Str(name).Str(" {\n").
		Str("            host HUB-ADDRESS;\n").
		Str("            port HUB-CLIENT-PORT;\n").
		Str("            secret \"the secret set for ").Str(name).Str(" on the hub\";\n").
		Str("        }\n    }\n}\n").String()
}

// adminLabelsContent builds the Labels page: every label key with its values
// and the count of devices carrying each, which links to those devices.
func adminLabelsContent(_ context.Context, request *PageRequest) (templ.Component, error) {
	rows := []RowView{}
	for _, usage := range LabelsInUse(request.Snapshot.Views) {
		for _, value := range usage.Values {
			pair := usage.Key + groupSeparator + value.Value
			rows = append(rows, RowView{
				ID: pair, Name: pair,
				Cells: []CellView{
					MonoCell(usage.Key), MonoCell(value.Value),
					LinkCell(countText(value.Count), estateDevicesHref(request, usage.Key+":"+value.Value)),
				},
				Menu: &MenuView{
					Label: "Actions for " + pair,
					Items: []ActionView{changeAction("rename-value", wordRename+" value", pair,
						"The value is renamed on every device that carries it.")},
					Destructive: []ActionView{removeAction("remove-key", wordRemove+" key", usage.Key,
						"The key and every value of it are removed from every device.")},
				},
			})
		}
	}

	return adminTable(request, pendingNote(), TableSectionView{
		Title: devicesWordLabels,
		Table: TableView{
			Name: devicesWordLabels,
			Headers: []ColumnHeaderView{
				{Label: "Key", Kind: ColumnKindTag}, {Label: "Value", Kind: ColumnKindTag},
				{Label: devicesWordDevices, Kind: ColumnKindNumber, Numeric: true},
			},
			Rows: rows, Controls: true,
			Empty: &StateHeroView{Kind: HeroNothingConfigured, Sentence: "No device carries a label yet."},
		},
	}), nil
}

// adminTreeViewsContent builds the Tree views page: the built-in views and
// the ones the hub declares, with their ordered levels.
func adminTreeViewsContent(_ context.Context, request *PageRequest) (templ.Component, error) {
	rows := []RowView{}
	add := func(view TreeView, declared bool) {
		kind := "Built in"
		menu := &MenuView{Label: "Actions for " + view.Name}
		if declared {
			kind = "Declared"
			menu.Items = []ActionView{
				changeAction("move-up", "Move a level up", view.Name, "The levels of the view are reordered."),
				changeAction("move-down", "Move a level down", view.Name, "The levels of the view are reordered."),
			}
			menu.Destructive = []ActionView{removeAction("remove-view", wordRemove, view.Name,
				"The view leaves the hub's configuration; the tree falls back to the default view.")}
		}
		if !declared {
			menu.Items = []ActionView{{Label: "Reorder", Disabled: true, Reason: "A built-in view cannot be changed."}}
		}
		rows = append(rows, RowView{
			ID: view.Name, Name: view.Name,
			Cells: []CellView{
				MonoCell(view.Name), TextCell(view.Title),
				TextCell(textbuf.New().Join(view.Levels, ", then ").String()), TextCell(kind),
			},
			Menu: menu,
		})
	}
	for _, view := range BuiltinTreeViews() {
		add(view, false)
	}
	for _, view := range request.Snapshot.TreeViews {
		add(view, true)
	}

	create := changeAction("add-view", "Add tree view", "a new tree view", "A named view with its ordered levels joins the hub's configuration.")
	create.Icon = glyphPlus

	return adminTable(request, pendingNote(), TableSectionView{
		Title: wordTreeViews, Create: &create,
		Table: TableView{
			Name: wordTreeViews,
			Headers: []ColumnHeaderView{
				{Label: devicesWordName, Kind: ColumnKindText}, {Label: "Title", Kind: ColumnKindText},
				{Label: "Levels, in order", Kind: ColumnKindText}, {Label: wordKind, Kind: ColumnKindText},
			},
			Rows: rows, Controls: true,
		},
	}), nil
}

// adminFiltersContent builds the Saved filters page of the account.
func adminFiltersContent(ctx context.Context, request *PageRequest) (templ.Component, error) {
	saved, err := request.Source.SavedFilters(ctx, request.Username)
	if err != nil {
		return nil, err
	}

	rows := make([]RowView, 0, len(saved))
	for _, filter := range saved {
		rows = append(rows, RowView{
			ID: filter.Name, Name: filter.Name,
			Cells: []CellView{
				TextCell(filter.Name), LinkCell(filter.Filter, estateDevicesHref(request, filter.Filter)), defaultFilterCell(&filter),
			},
			Menu: &MenuView{
				Label: "Actions for " + filter.Name,
				Items: []ActionView{
					changeAction("rename-filter", wordRename, filter.Name, "The filter keeps its terms under the new name."),
					changeAction("default-filter", "Set as default", filter.Name, "The devices table opens with this filter."),
				},
				Destructive: []ActionView{removeAction("remove-filter", wordRemove, filter.Name, "The saved filter is forgotten.")},
			},
		})
	}

	note := &CalloutView{
		Title: "Saved filters are your own state.",
		Text:  "In the functional interface a change to them applies at once, without the candidate. The prototype changes nothing.",
	}

	return adminTable(request, note, TableSectionView{
		Title: wordFilters,
		Table: TableView{
			Name: wordFilters,
			Headers: []ColumnHeaderView{
				{Label: devicesWordName, Kind: ColumnKindText}, {Label: "Filter", Kind: ColumnKindText}, {Label: "Default", Kind: ColumnKindState},
			},
			Rows: rows, Controls: true,
			Empty: &StateHeroView{Kind: HeroNoData, Sentence: "You have saved no filter yet."},
		},
	}), nil
}

// adminTemplatesContent builds the Templates page.
func adminTemplatesContent(ctx context.Context, request *PageRequest) (templ.Component, error) {
	templates, err := request.Source.Templates(ctx)
	if err != nil {
		return nil, err
	}

	rows := make([]RowView, 0, len(templates))
	for index := range templates {
		template := &templates[index]
		rows = append(rows, RowView{
			ID: template.Name, Name: template.Name,
			EyeHref: rowAddress(request, template.Name),
			Opened:  request.Address.View.Row == template.Name,
			Cells: []CellView{
				MonoCell(template.Name), TextCell(textbuf.New().Join(template.Groups, ", ").String()),
				NumberCell(countText(template.Devices)), whenCell(request, template.Changed), TextCell(template.Actor),
			},
		})
	}

	return adminTable(request, pendingNote(), TableSectionView{
		Title: wordTemplates,
		Table: TableView{
			Name: wordTemplates,
			Headers: []ColumnHeaderView{
				{Label: devicesWordName, Kind: ColumnKindText}, {Label: "Groups using it", Kind: ColumnKindText},
				{Label: "Devices using it", Kind: ColumnKindNumber, Numeric: true},
				{Label: "Last change", Kind: ColumnKindTime}, {Label: auditWordActor, Kind: ColumnKindText},
			},
			Rows: rows, Controls: true,
			Empty: &StateHeroView{Kind: HeroNothingConfigured, Sentence: "The hub holds no template yet."},
		},
	}), nil
}

// adminTemplatesPanel builds a template's side panel: its facts, its groups,
// and the devices it would be previewed for. The source answers neither the
// template's text nor a rendering, and the panel says so rather than show a
// diff it cannot compute.
func adminTemplatesPanel(ctx context.Context, request *PageRequest) (templ.Component, error) {
	templates, err := request.Source.Templates(ctx)
	if err != nil {
		return nil, err
	}

	name := request.Address.View.Row
	var template *Template
	for index := range templates {
		if templates[index].Name == name {
			template = &templates[index]
		}
	}
	if template == nil {
		return nil, opererr.Wrap(ErrObjectUnknown,
			textbuf.New().Str("The hub holds no template ").Quoted(name).Str(".").String(),
			"The address names a template that does not exist, so there is nothing to show.",
			"open the Templates page and choose a template from its table")
	}

	groups := make([]ValueView, 0, len(template.Groups))
	for _, group := range template.Groups {
		groups = append(groups, ValueView{Label: devicesWordGroup, Text: group})
	}

	using, connected := []ValueView{}, 0
	views := request.Snapshot.Views
	for index := range views {
		device := &views[index].Device
		if device.Config.Template != template.Name {
			continue
		}
		if device.Connection.Connected {
			connected++
		}
		using = append(using, ValueView{
			Label: "Preview for", Text: device.Name, Href: deviceTabHref(request, device.Name, tabDeviceConfig),
		})
	}

	change := changeAction("edit-template", wordChange, template.Name, textbuf.New().Int(int64(connected)).
		Str(" connected devices would receive a new configuration; the functional interface offers to push it as a rollout instead of at once.").String())

	return PanelWithDiff(PanelDiffView{
		Panel: PanelView{
			Title: template.Name, CloseHref: rowAddress(request, ""), Actions: []ActionView{change},
			Cards: []PanelCardView{
				{Title: "Template", Rows: []ValueView{
					{Label: devicesWordName, Text: template.Name, Mono: true},
					{Label: "Devices using it", Text: countText(template.Devices)},
					{Label: "Last change", Text: whenText(request, template.Changed)},
					{Label: auditWordActor, Text: template.Actor},
				}},
				{Title: "Groups assigned", Rows: groups},
				{Title: "Preview", Rows: using},
			},
		},
		Note: "The source answers neither the template's text, its variables nor a rendering for a device, " +
			"so no preview or diff can be computed here: each device's configuration is on its Config tab.",
	}), nil
}

// accessStatus answers whether a profile may use a capability.
func accessStatus(allowed bool) CellView {
	if allowed {
		return StatusCell(Status{Tone: ToneSuccess, Word: wordAllowed})
	}

	return StatusCell(Status{Tone: ToneMuted, Word: wordNotAllowed})
}

// adminAccessContent builds the Access page: authorization profiles against
// the fleet capabilities.
func adminAccessContent(ctx context.Context, request *PageRequest) (templ.Component, error) {
	profiles, err := request.Source.AccessProfiles(ctx)
	if err != nil {
		return nil, err
	}

	rows := make([]RowView, 0, len(profiles))
	for index := range profiles {
		profile := &profiles[index]
		rows = append(rows, RowView{
			ID: profile.Name, Name: profile.Name,
			Cells: []CellView{
				MonoCell(profile.Name), accessStatus(profile.FleetMode), accessStatus(profile.Console),
				accessStatus(profile.Diagnostics), accessStatus(profile.ConfigPush), accessStatus(profile.Rollouts),
			},
			Menu: &MenuView{Label: "Actions for " + profile.Name, Items: []ActionView{
				changeAction("change-access", wordChange, profile.Name, "The profile is changed in the authorization configuration."),
			}},
		})
	}

	return adminTable(request, pendingNote(), TableSectionView{
		Title: "Access",
		Table: TableView{
			Name: "Access by authorization profile",
			Headers: []ColumnHeaderView{
				{Label: "Profile", Kind: ColumnKindText}, {Label: "Fleet mode", Kind: ColumnKindState},
				{Label: "Device console", Kind: ColumnKindState}, {Label: "Relayed diagnostics", Kind: ColumnKindState},
				{Label: "Configuration pushes", Kind: ColumnKindState}, {Label: rolloutsTitle, Kind: ColumnKindState},
			},
			Rows: rows, Controls: true,
			Empty: &StateHeroView{Kind: HeroNothingConfigured, Sentence: "The hub declares no authorization profile."},
		},
	}), nil
}

// adminHistoryContent builds the History page: retention, size against the
// cap, the oldest sample, and the devices using most.
func adminHistoryContent(ctx context.Context, request *PageRequest) (templ.Component, error) {
	usage, err := request.Source.HistoryUsage(ctx)
	if err != nil {
		return nil, err
	}

	percent, known := Percent(float64(usage.SizeBytes), float64(usage.CapBytes))
	size := textbuf.New().Str(bytesText(usage.SizeBytes)).Str(" of ").Str(bytesText(usage.CapBytes)).String()

	rows := make([]RowView, 0, len(usage.Largest))
	for _, device := range usage.Largest {
		rows = append(rows, RowView{
			ID: device.Device, Name: device.Device,
			Cells: []CellView{
				LinkCell(device.Device, deviceTabHref(request, device.Device, tabDeviceDashboard)),
				NumberCell(bytesText(device.SizeBytes)),
			},
		})
	}

	retention := changeAction("change-history", wordChange+" retention", "the metric history", "The retention and the cap are changed in the hub's configuration.")

	return AdminPage(AdminPageView{
		Note: pendingNote(),
		Facts: &QuickInfoView{Card: CardView{Title: "Metric history", Span: 12}, Items: []ValueView{
			{Label: "Retention", Text: durationText(usage.Retention)},
			{Label: "Size", Text: size, Usage: &UsageBarView{Name: "History size against its cap", Percent: percent, Known: known, Scale: true}},
			{Label: "Oldest sample", Text: whenText(request, usage.Oldest)},
		}},
		FactsAction: &retention,
		Table: TableSectionView{Title: "Devices using most", Table: TableView{
			Name: "Devices using most",
			Headers: []ColumnHeaderView{
				{Label: devicesWordDevice, Kind: ColumnKindText}, {Label: "Size", Kind: ColumnKindNumber, Numeric: true},
			},
			Rows:  rows,
			Empty: &StateHeroView{Kind: HeroNoData, Sentence: "The history holds no sample yet."},
		}},
		Form: manageForm(request),
	}), nil
}

// adminSettingsContent builds the Settings page.
func adminSettingsContent(ctx context.Context, request *PageRequest) (templ.Component, error) {
	settings, err := request.Source.Settings(ctx)
	if err != nil {
		return nil, err
	}

	setting := func(name, value string) RowView {
		return RowView{
			ID: name, Name: name,
			Cells: []CellView{TextCell(name), MonoCell(value)},
			Menu: &MenuView{Label: "Actions for " + name, Items: []ActionView{
				changeAction("change-setting", wordChange, name, "The setting is changed in the hub's configuration."),
			}},
		}
	}

	threshold := textbuf.New().Str(strconv.FormatFloat(settings.ResourceAlarmPercent, 'f', -1, 64)).
		Str("% for ").Str(durationText(settings.ResourceAlarmDelay)).String()

	return adminTable(request, pendingNote(), TableSectionView{
		Title: "Settings",
		Table: TableView{
			Name: "Fleet settings",
			Headers: []ColumnHeaderView{
				{Label: "Setting", Kind: ColumnKindText}, {Label: "Value", Kind: ColumnKindText},
			},
			Rows: []RowView{
				setting("Offline alarm delay", durationText(settings.OfflineAlarmDelay)),
				setting("Resource alarm threshold", threshold),
				setting("Finished tasks kept", strconv.Itoa(settings.FinishedTasksKept)),
				setting("Maximum live streams", strconv.Itoa(settings.EventStreamsMax)),
			},
			Controls: true,
		},
	}), nil
}

// durationText answers a duration in whole days, hours or minutes when it is
// one, and as Go writes it otherwise.
func durationText(duration time.Duration) string {
	for _, unit := range []struct {
		size time.Duration
		name string
	}{{24 * time.Hour, "day"}, {time.Hour, "hour"}, {time.Minute, "minute"}} {
		if duration < unit.size {
			continue
		}
		if duration%unit.size == 0 {
			return relativeUnits(int64(duration/unit.size), unit.name)
		}
	}

	return duration.String()
}

// relativeUnits writes a count of one unit, plural beyond one.
func relativeUnits(count int64, unit string) string {
	text := textbuf.New().Int(count).Byte(' ').Str(unit)
	if count != 1 {
		text.Byte('s')
	}

	return text.String()
}

// bytesText answers a size in the largest binary unit it fills, with one
// decimal.
func bytesText(size uint64) string {
	units := byteUnits
	scale, unit := uint64(1), 0
	for unit+1 < len(units) && size >= scale*1024 {
		scale *= 1024
		unit++
	}
	tenths := size * 10 / scale

	return textbuf.New().Uint(tenths / 10).Byte('.').Uint(tenths % 10).Byte(' ').Str(units[unit]).String()
}
