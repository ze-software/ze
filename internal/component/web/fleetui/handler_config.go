// Design: docs/architecture/fleet-interface.md -- the device Config tab, its divergence resolution, and what the manage pages share
// Related: page_config.templ -- draws the tab, its sections and the side panel with a diff
// Related: handler_rollouts.go, handler_audit.go, handler_admin.go -- the other pages of the manage family
// Related: register_manage_pages.go -- registers the pages, tabs and fragments of this family
//
// The Config tab reads the configuration the hub holds for one device, its
// history, and, for a diverged device, both sides of the divergence. Nothing
// here changes anything: every action opens the confirm dialog, and its
// request answers a toast saying the prototype changed nothing.

package fleetui

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The fragments the manage family registers. Each section of a page that can
// fail on its own is one of them, so it can be fetched again alone.
const (
	fragmentConfigState    = "config-state"
	fragmentConfigText     = "config-text"
	fragmentConfigHistory  = "config-history"
	fragmentDivergence     = "config-divergence"
	fragmentRolloutPreview = "rollout-preview"
)

// The tabs and administration pages of the manage family, as their address
// segments name them.
const (
	tabDeviceConfig     = "config"
	tabEstateRollouts   = "rollouts"
	tabAdminAudit       = "audit"
	tabAdminDevices     = "devices"
	tabAdminLabels      = "labels"
	tabAdminTreeViews   = "tree-views"
	tabAdminFilters     = "filters"
	tabAdminTemplates   = "templates"
	tabAdminAccess      = "access"
	tabAdminHistory     = "history"
	tabAdminSettings    = "settings"
	tabDeviceDashboard  = TabDashboard
	manageActionFormID  = "fleet-manage-actions"
	manageActionSep     = ":"
	exactTimeLayout     = "2006-01-02 15:04:05 MST"
	clockLayout         = "15:04"
	prototypeNothing    = "The prototype changed nothing."
	divergenceSub       = "divergence"
	wordNever           = "never"
	wordConfigTitle     = "Configuration"
	wordHistory         = "History"
	wordStateLine       = "State"
	wordReadOnlyOffline = "The device is offline: the hub cannot change its configuration until it connects again."
)

// manageActions names every action of the manage family by the key its
// request carries, with the words its toast says it would have done. The
// toast is the whole of what a prototype action does.
var manageActions = map[string]string{
	"edit-config":     "Edit the configuration",
	"rollback":        "Roll back to this version",
	"keep-device":     "Keep the device's changes",
	"restore-hub":     "Restore the hub's version",
	"pause":           "Pause the rollout",
	"resume":          "Resume the rollout",
	"abort":           "Abort the rollout",
	"force-complete":  "Force the rollout complete",
	"start-rollout":   "Start the rollout",
	"add-device":      "Add the device",
	"edit-device":     "Edit the device",
	"remove-device":   "Remove the device",
	"rename-value":    "Rename the label value",
	"remove-key":      "Remove the label key",
	"add-view":        "Add a tree view",
	"move-up":         "Move the level up",
	"move-down":       "Move the level down",
	"remove-view":     "Remove the tree view",
	"rename-filter":   "Rename the saved filter",
	"remove-filter":   "Remove the saved filter",
	"default-filter":  "Make the saved filter the default",
	"edit-template":   "Change the template",
	"change-access":   "Change the access of the profile",
	"change-setting":  "Change the setting",
	"change-history":  "Change the history retention",
	"preview-default": "Preview the template",
}

// manageAction answers an action of the manage family that submits the
// family's action form with key and target, behind confirm.
func manageAction(key, label, target string, confirm *ConfirmView) ActionView {
	return ActionView{
		Label: label, Form: manageActionFormID, Name: paramItem,
		Value: key + manageActionSep + target, Confirm: confirm,
	}
}

// manageActionName is the registered action every manage page's controls
// post to.
const manageActionName = "manage"

// manageForm answers the hidden form every manage action submits: it posts
// to the manage action of the page in hand, whose answer is a toast.
func manageForm(request *PageRequest) ManageFormView {
	return ManageFormView{ID: manageActionFormID, URL: ActionAddress(manageActionName, request.Address)}
}

// manageActionAnswer answers the toast of one prototype action. The item
// field is the action's key and its target. An action nobody declared
// answers an error toast that names it, since the form's answer is swapped
// nowhere else.
func manageActionAnswer(_ context.Context, _ *PageRequest, form url.Values) (ToastView, error) {
	key, target, _ := strings.Cut(form.Get(paramItem), manageActionSep)

	word, known := manageActions[key]
	if !known {
		return ToastView{
			Level:   ToastError,
			Message: textbuf.New().Str("The fleet interface has no action named ").Quoted(key).Str(".").String(),
			Detail:  "Nothing was done. Reload the page to get its current actions.",
		}, nil
	}

	detail := textbuf.New().Str(word)
	if target != "" {
		detail.Str(" on ").Str(target)
	}
	detail.Str(" was not done: in the functional interface it goes through the candidate, the diff and the commit.")

	return ToastView{Level: ToastInformational, Message: prototypeNothing, Detail: detail.String()}, nil
}

// rowAddress answers the address of the page in hand with row opened.
func rowAddress(request *PageRequest, row string) string {
	address := request.Address
	address.View.Row = row

	return address.String()
}

// subAddress answers the address of the page in hand with its sub parameter
// set to sub.
func subAddress(request *PageRequest, sub string) string {
	address := request.Address
	address.View.Sub = sub

	return address.String()
}

// deviceTabHref answers the address of one tab of a device.
func deviceTabHref(request *PageRequest, device, tab string) string {
	return Address{Kind: ObjectDevice, Device: device, Tab: tab, View: linkView(request)}.String()
}

// whenCell answers the cell of a time, relative to the hub's clock.
func whenCell(request *PageRequest, at time.Time) CellView {
	return TimeCell(RelativeTime(request.Snapshot.Now.Sub(at)), at.Format(exactTimeLayout))
}

// whenText answers a time as a page writes it beside a word: relative, then
// exact.
func whenText(request *PageRequest, at time.Time) string {
	return textbuf.New().Str(RelativeTime(request.Snapshot.Now.Sub(at))).Str(" (").
		Str(at.Format(exactTimeLayout)).Byte(')').String()
}

// sectionOf runs one section builder and answers the section, drawn as the
// failure state when the builder fails, so one section's failure leaves the
// others standing.
func sectionOf(ctx context.Context, request *PageRequest, title, fragment string, build FragmentBuilder) ManageSectionView {
	body, err := build(ctx, request, "")
	if err != nil {
		body = failureHero(request, err)
	}

	return ManageSectionView{Title: title, Name: fragment, Body: body}
}

// configDevice answers the device the Config tab of request belongs to.
func configDevice(request *PageRequest) (*DeviceView, error) {
	return FindDevice(request.Snapshot.Views, request.Address.Device)
}

// configContent builds the Config tab of a device: the offline banner, the
// divergence when the device is diverged, the state line, the configuration
// text and the history.
func configContent(ctx context.Context, request *PageRequest) (templ.Component, error) {
	device, err := configDevice(request)
	if err != nil {
		return nil, err
	}

	view := ConfigTabView{Form: manageForm(request)}
	if !device.Device.Connection.Connected {
		banner := offlineBanner(request, device)
		view.Offline = &banner
	}

	if device.Device.Config.State == ConfigStateDiverged {
		view.Sections = append(view.Sections,
			sectionOf(ctx, request, "Divergence", fragmentDivergence, divergenceFragment))
	}

	view.Sections = append(view.Sections,
		sectionOf(ctx, request, wordStateLine, fragmentConfigState, configStateFragment),
		sectionOf(ctx, request, wordConfigTitle, fragmentConfigText, configTextFragment),
		sectionOf(ctx, request, wordHistory, fragmentConfigHistory, configHistoryFragment),
	)

	return ConfigTab(view), nil
}

// offlineBanner answers the banner of a device the hub cannot reach.
func offlineBanner(request *PageRequest, device *DeviceView) CalloutView {
	seen := "The hub has never seen it."
	if device.SeenKnown {
		seen = textbuf.New().Str("It was last seen ").Str(RelativeTime(device.SeenAgo)).Str(", at ").
			Str(request.Snapshot.Now.Add(-device.SeenAgo).Format(exactTimeLayout)).Str(".").String()
	}

	return CalloutView{
		Title: device.Device.Name + " is offline, so its configuration is read-only.",
		Text:  "The hub cannot change a disconnected device's configuration. " + seen + " The history below stays readable.",
	}
}

// configStateFragment answers the state line and the source of a device's
// configuration.
func configStateFragment(_ context.Context, request *PageRequest, _ string) (templ.Component, error) {
	device, err := configDevice(request)
	if err != nil {
		return nil, err
	}

	return KeyValueList(KeyValueListView{Rows: configStateRows(request, device)}), nil
}

// configStateRows answers the rows of the state line.
func configStateRows(request *PageRequest, device *DeviceView) []ValueView {
	facts := device.Device.Config

	fetch := wordNever
	if facts.Fetched {
		fetch = textbuf.New().Str(whenText(request, facts.LastFetch)).Str(": ").Str(facts.LastFetchResult).String()
	}

	rows := []ValueView{
		{Label: wordConfigTitle, Status: ConfigStatus(facts.State)},
		{Label: "Device runs", Text: versionText(facts.DeviceVersion), Mono: true, Copy: facts.DeviceVersion != ""},
		{Label: "Hub holds", Text: versionText(facts.HubVersion), Mono: true, Copy: facts.HubVersion != ""},
		{Label: "Last fetch", Text: fetch},
	}

	if facts.Template == "" {
		return append(rows, ValueView{Label: "Source", Text: "Its own configuration"})
	}

	template := Address{Kind: ObjectAdmin, Tab: tabAdminTemplates, View: ViewParams{Row: facts.Template}}
	rows = append(rows, ValueView{Label: "Source", Text: "Template " + facts.Template, Href: template.String()})
	if device.Device.Group != "" {
		rows = append(rows, ValueView{Label: "Group", Text: device.Device.Group})
	}

	return rows
}

// versionText answers a version as the state line writes it.
func versionText(version string) string {
	if version == "" {
		return "none"
	}

	return version
}

// configTextFragment answers the configuration the hub holds for the device,
// read-only, with the Edit action.
func configTextFragment(ctx context.Context, request *PageRequest, _ string) (templ.Component, error) {
	device, err := configDevice(request)
	if err != nil {
		return nil, err
	}

	config, err := request.Source.DeviceConfig(ctx, device.Device.Name)
	if err != nil {
		return nil, err
	}

	edit := manageAction("edit-config", "Edit", device.Device.Name, nil)
	edit.Icon = glyphEdit
	edit.Confirm = &ConfirmView{
		Action:  "Edit the configuration",
		Targets: device.Device.Name,
		Detail:  "The prototype has no editor: in the functional interface the configuration opens in the YANG editor, with the candidate and the commit bar.",
	}
	if !device.Device.Connection.Connected {
		edit.Disabled = true
		edit.Reason = wordReadOnlyOffline
	}

	return ConfigText(ConfigTextView{Version: config.Version, Text: config.Text, Edit: edit}), nil
}

// configHistoryFragment answers the versions the hub has held for the
// device, newest first.
func configHistoryFragment(ctx context.Context, request *PageRequest, _ string) (templ.Component, error) {
	device, err := configDevice(request)
	if err != nil {
		return nil, err
	}

	versions, err := request.Source.ConfigVersions(ctx, device.Device.Name)
	if err != nil {
		return nil, err
	}

	rows := make([]RowView, 0, len(versions))
	for index := range versions {
		version := &versions[index]
		rollback := manageAction("rollback", "Roll back to this version", device.Device.Name+"@"+version.Version,
			&ConfirmView{
				Action:  "Roll back to this version",
				Targets: device.Device.Name + " version " + version.Version,
				Detail:  "The version is staged as a candidate; nothing reaches the device before the commit.",
			})
		if !device.Device.Connection.Connected {
			rollback.Disabled = true
			rollback.Reason = wordReadOnlyOffline
		}

		rows = append(rows, RowView{
			ID: version.Version, Name: version.Version,
			EyeHref: rowAddress(request, version.Version),
			Opened:  request.Address.View.Row == version.Version,
			Cells: []CellView{
				whenCell(request, version.At), MonoCell(version.Version), TextCell(version.Actor),
				TextCell(version.Summary), StatusCell(ConfigVersionResultStatus(version.Result)),
			},
			Menu: &MenuView{Label: "Actions for version " + version.Version, Items: []ActionView{rollback}},
		})
	}

	return TableSection(TableSectionView{Table: TableView{
		Name: "Configuration versions of " + device.Device.Name,
		Headers: []ColumnHeaderView{
			{Label: "Time", Kind: "kind-time"}, {Label: "Version", Kind: "kind-text"},
			{Label: "Actor", Kind: "kind-text"}, {Label: "Summary", Kind: "kind-text"},
			{Label: "Result on the device", Kind: "kind-state"},
		},
		Rows:     rows,
		Controls: true,
		Empty:    &StateHeroView{Kind: HeroNoData, Sentence: "The hub holds no earlier version of this configuration."},
	}}), nil
}

// divergenceOpened reports whether the address shows the difference of a
// diverged device: the "Review the difference" link sets the sub parameter
// to divergenceSub, and that is the mark that the difference was seen.
func divergenceOpened(request *PageRequest) bool {
	return request.Address.View.Sub == divergenceSub
}

// divergenceFragment answers the divergence banner, the difference when it
// was opened, and the two resolutions.
func divergenceFragment(ctx context.Context, request *PageRequest, _ string) (templ.Component, error) {
	device, err := configDevice(request)
	if err != nil {
		return nil, err
	}

	divergence, err := request.Source.Divergence(ctx, device.Device.Name)
	if err != nil {
		return nil, err
	}

	return DivergenceSection(divergenceView(request, &divergence)), nil
}

// divergenceView answers the view of a divergence: the banner of the spec's
// wireframe, the diff with the hub on the left, and the resolutions. The
// destructive one is offered only once the difference has been opened.
func divergenceView(request *PageRequest, divergence *Divergence) DivergenceView {
	name := divergence.Device

	banner := CalloutView{
		Title: name + " was edited locally while out of the fleet and is holding its own configuration. Nothing has been overwritten.",
		Text: textbuf.New().Str("Edited: ").Int(int64(divergence.Changes)).Str(changesWord(divergence.Changes)).
			Str(", last at ").Str(divergence.EditedAt.Format(clockLayout)).Str(".").String(),
		Link: &LinkView{Label: "Review the difference", Href: subAddress(request, divergenceSub)},
	}

	keep := manageAction("keep-device", "Keep the device's changes", name, &ConfirmView{
		Action:  "Keep the device's changes",
		Targets: name,
		Detail: textbuf.New().Str("The hub's version ").Str(divergence.HubVersion).
			Str(" is replaced by the device's configuration ").Str(divergence.DeviceVersion).Str(".").String(),
	})

	restore := manageAction("restore-hub", "Restore the hub's version", name, &ConfirmView{
		Action:  "Restore the hub's version",
		Targets: name,
		Detail: textbuf.New().Str("The hub's version ").Str(divergence.HubVersion).
			Str(" is pushed to the device and its local edit is discarded.").String(),
		Expect:      name,
		Destructive: true,
	})

	opened := divergenceOpened(request)
	if !opened {
		restore.Disabled = true
		restore.Reason = "Review the difference first: this discards the device's local edit, so it is offered once the difference has been opened."
	}

	view := DivergenceView{Banner: banner, Opened: opened, Keep: keep, Restore: restore}
	if opened {
		view.Diff = DiffView{
			LeftTitle:  "Hub configuration (" + divergence.HubVersion + ")",
			RightTitle: "Device configuration (" + divergence.DeviceVersion + ")",
			Rows:       DiffLines(divergence.HubText, divergence.DeviceText),
		}
	}

	return view
}

// changesWord answers the noun after a count of changes.
func changesWord(count int) string {
	if count == 1 {
		return " change"
	}

	return " changes"
}

// configPanel builds the side panel of the Config tab: the divergence facts
// for the divergence row, and a version with its diff against the version
// before it for any other row.
func configPanel(ctx context.Context, request *PageRequest) (templ.Component, error) {
	device, err := configDevice(request)
	if err != nil {
		return nil, err
	}
	name := device.Device.Name
	closeHref := rowAddress(request, "")

	if divergenceOpened(request) {
		divergence, divergenceErr := request.Source.Divergence(ctx, name)
		if divergenceErr != nil {
			return nil, divergenceErr
		}

		return PanelWithDiff(PanelDiffView{Panel: PanelView{
			Title: "Divergence of " + name, CloseHref: closeHref,
			Cards: []PanelCardView{{Title: "Both sides", Rows: []ValueView{
				{Label: "Hub holds", Text: divergence.HubVersion, Mono: true},
				{Label: "Device holds", Text: divergence.DeviceVersion, Mono: true},
				{Label: "Local edits", Text: countText(divergence.Changes)},
				{Label: "Last edit", Text: whenText(request, divergence.EditedAt)},
			}}},
		}}), nil
	}

	versions, err := request.Source.ConfigVersions(ctx, name)
	if err != nil {
		return nil, err
	}

	diff, version, err := versionDiff(ctx, request, name, versions, request.Address.View.Row)
	if err != nil {
		return nil, err
	}

	rows := []ValueView{
		{Label: "Version", Text: version.Version, Mono: true, Copy: true},
		{Label: "Time", Text: whenText(request, version.At)},
		{Label: "Actor", Text: version.Actor},
		{Label: "Summary", Text: version.Summary},
		{Label: "Result on the device", Status: ConfigVersionResultStatus(version.Result)},
	}
	if version.Answer != "" {
		rows = append(rows, ValueView{Label: "Device answer", Text: version.Answer})
	}

	return PanelWithDiff(PanelDiffView{
		Panel: PanelView{
			Title: "Version " + version.Version, CloseHref: closeHref,
			Cards: []PanelCardView{{Title: "Version", Rows: rows}},
		},
		Diff: diff,
	}), nil
}

// versionDiff answers the diff of one version of a device's configuration
// against the version before it, and the version. The oldest version has no
// earlier one, and its diff is nil.
func versionDiff(ctx context.Context, request *PageRequest, device string, versions []ConfigVersion, wanted string) (*DiffView, ConfigVersion, error) {
	for index := range versions {
		if versions[index].Version != wanted {
			continue
		}

		version := versions[index]
		if index+1 >= len(versions) {
			return nil, version, nil
		}
		previous := versions[index+1]

		text, err := request.Source.ConfigText(ctx, device, version.Version)
		if err != nil {
			return nil, version, err
		}
		before, err := request.Source.ConfigText(ctx, device, previous.Version)
		if err != nil {
			return nil, version, err
		}

		return &DiffView{
			LeftTitle: "Version " + previous.Version, RightTitle: "Version " + version.Version,
			Rows: DiffLines(before, text),
		}, version, nil
	}

	return nil, ConfigVersion{}, versionUnknown(device, wanted)
}

// versionUnknown is the refusal for a version the hub does not hold.
func versionUnknown(device, version string) error {
	return opererr.Wrap(ErrObjectUnknown,
		textbuf.New().Str("The hub holds no version ").Quoted(version).Str(" of ").Str(device).Str("'s configuration.").String(),
		"The address names a version that is not in the device's history, so there is no diff to show.",
		"open the device's Config tab and choose a version from its history")
}

// diffCellsMax bounds the table the longest common subsequence of a diff is
// computed in: the product of the line counts of the parts that differ. Past
// it the parts that differ are shown as removed, then added.
const diffCellsMax = 1 << 20

// DiffLines answers the rows of a two-column diff of left against right, line
// by line. A run of removed lines followed by a run of added ones is paired
// into changed lines, so the two columns line up.
func DiffLines(left, right string) []DiffRowView {
	before, after := splitLines(left), splitLines(right)

	head := 0
	for head < len(before) && head < len(after) && before[head] == after[head] {
		head++
	}
	tail := 0
	for tail < len(before)-head && tail < len(after)-head &&
		before[len(before)-1-tail] == after[len(after)-1-tail] {
		tail++
	}

	builder := diffBuilder{}
	for index := range head {
		builder.same(before[index], index+1, index+1)
	}

	middleBefore := before[head : len(before)-tail]
	middleAfter := after[head : len(after)-tail]
	builder.middle(middleBefore, middleAfter, head)

	for index := range tail {
		at := len(before) - tail + index
		builder.same(before[at], at+1, len(after)-tail+index+1)
	}
	builder.flush()

	return builder.rows
}

// splitLines answers the lines of a text, without the empty line after a
// final newline.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}

	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// diffBuilder collects the rows of a diff, holding back removed and added
// lines until a common line or the end pairs them.
type diffBuilder struct {
	rows    []DiffRowView
	removed []DiffRowView
	added   []DiffRowView
}

// same adds a line both sides hold.
func (builder *diffBuilder) same(line string, leftNumber, rightNumber int) {
	builder.flush()
	builder.rows = append(builder.rows, DiffRowView{
		Left: line, Right: line, LeftNumber: leftNumber, RightNumber: rightNumber, Kind: DiffSame,
	})
}

// flush pairs the held removed and added lines into changed lines, then adds
// the rest as they are.
func (builder *diffBuilder) flush() {
	paired := min(len(builder.removed), len(builder.added))
	for index := range paired {
		builder.rows = append(builder.rows, DiffRowView{
			Left: builder.removed[index].Left, LeftNumber: builder.removed[index].LeftNumber,
			Right: builder.added[index].Right, RightNumber: builder.added[index].RightNumber,
			Kind: DiffChanged,
		})
	}
	builder.rows = append(builder.rows, builder.removed[paired:]...)
	builder.rows = append(builder.rows, builder.added[paired:]...)
	builder.removed = builder.removed[:0]
	builder.added = builder.added[:0]
}

// middle adds the lines between the common head and tail, from the longest
// common subsequence while its table fits diffCellsMax. offset is the count
// of lines before them on each side.
func (builder *diffBuilder) middle(before, after []string, offset int) {
	if len(before)*len(after) > diffCellsMax {
		for index, line := range before {
			builder.removed = append(builder.removed, DiffRowView{Left: line, LeftNumber: offset + index + 1, Kind: DiffRemoved})
		}
		for index, line := range after {
			builder.added = append(builder.added, DiffRowView{Right: line, RightNumber: offset + index + 1, Kind: DiffAdded})
		}
		return
	}

	// common[i][j] is the length of the longest common subsequence of
	// before[i:] and after[j:], in one slice of (len(before)+1) rows.
	width := len(after) + 1
	common := make([]int32, (len(before)+1)*width)
	for i := len(before) - 1; i >= 0; i-- {
		for j := len(after) - 1; j >= 0; j-- {
			if before[i] == after[j] {
				common[i*width+j] = common[(i+1)*width+j+1] + 1
				continue
			}
			common[i*width+j] = max(common[(i+1)*width+j], common[i*width+j+1])
		}
	}

	i, j := 0, 0
	for i < len(before) && j < len(after) {
		if before[i] == after[j] {
			builder.same(before[i], offset+i+1, offset+j+1)
			i++
			j++
			continue
		}
		if common[(i+1)*width+j] >= common[i*width+j+1] {
			builder.removed = append(builder.removed, DiffRowView{Left: before[i], LeftNumber: offset + i + 1, Kind: DiffRemoved})
			i++
			continue
		}
		builder.added = append(builder.added, DiffRowView{Right: after[j], RightNumber: offset + j + 1, Kind: DiffAdded})
		j++
	}
	for ; i < len(before); i++ {
		builder.removed = append(builder.removed, DiffRowView{Left: before[i], LeftNumber: offset + i + 1, Kind: DiffRemoved})
	}
	for ; j < len(after); j++ {
		builder.added = append(builder.added, DiffRowView{Right: after[j], RightNumber: offset + j + 1, Kind: DiffAdded})
	}
}
