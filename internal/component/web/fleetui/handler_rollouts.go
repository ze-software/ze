// Design: docs/architecture/fleet-interface.md -- the Rollouts tab of the estate, a rollout's page and the new rollout form
// Related: page_rollouts.templ -- draws the rollout page, its batch strip and the new rollout form
// Related: handler_config.go -- the action form and the helpers the manage pages share
//
// The registry has no page below a tab, so a rollout's page is the Rollouts
// tab with the rollout's name in its id parameter, drawn across the whole
// content region, and the new rollout form is the same tab with the sub
// parameter newRolloutSub, so no rollout name is taken by the form. The table, a rollout and the form are each reached by an
// address that reproduces them.

package fleetui

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// newRolloutSub is the sub parameter of the Rollouts tab that shows the new
// rollout form. Its filter parameter is the target of the new rollout.
const newRolloutSub = "new"

// The batch sizes the new rollout form offers, as a percentage of the
// connected devices of the target, and the one it starts with.
var rolloutBatchPercents = []int{10, 25, 50, 100}

const rolloutBatchPercentDefault = 25

// rolloutFilters is the filter grammar of the rollouts table. Its fields are
// registered from the family's register file.
var rolloutFilters = NewFilterSchema[Rollout]("rollouts", rolloutBare, nil)

// rolloutBare reports whether a rollout's name or target holds a bare word.
func rolloutBare(row *Rollout, needle string) bool {
	if strings.Contains(strings.ToLower(row.Name), needle) {
		return true
	}

	return strings.Contains(strings.ToLower(row.Target), needle)
}

// registerRolloutFields registers the filter fields of the rollouts table:
// state, actor and target.
func registerRolloutFields() {
	states := []RolloutState{
		RolloutStatePending, RolloutStateRolling, RolloutStatePaused,
		RolloutStateComplete, RolloutStateFailed, RolloutStateAborted,
	}
	keys := make([]string, 0, len(states))
	for _, state := range states {
		keys = append(keys, state.Key())
	}

	rolloutFilters.MustRegisterField(TextField("state", keys, func(row *Rollout) string { return row.State.Key() }))
	rolloutFilters.MustRegisterField(TextField("actor", nil, func(row *Rollout) string { return row.Actor }))
	rolloutFilters.MustRegisterField(TextField("target", nil, func(row *Rollout) string { return row.Target }))
}

// rolloutsAddress answers the address of the Rollouts tab with row opened
// and filter as its filter.
func rolloutsAddress(request *PageRequest, row, filter string) Address {
	return Address{Kind: ObjectEstate, Tab: tabEstateRollouts, View: ViewParams{
		Row: row, Filter: filter, TreeView: request.Address.View.TreeView,
	}}
}

// rolloutFormAddress answers the address of the new rollout form, targeting
// the devices filter matches.
func rolloutFormAddress(request *PageRequest, filter string) Address {
	return Address{Kind: ObjectEstate, Tab: tabEstateRollouts, View: ViewParams{
		Sub: newRolloutSub, Filter: filter, TreeView: request.Address.View.TreeView,
	}}
}

// treeViewHidden answers the hidden field that keeps the tree view across a
// form, or none for the default view.
func treeViewHidden(request *PageRequest) []HiddenFieldView {
	if !request.Address.View.treeViewChosen() {
		return nil
	}

	return []HiddenFieldView{{Name: paramTreeView, Value: request.Address.View.TreeView}}
}

// refusalText answers a refused filter as the query row writes it under the
// field: what is wrong, then the fix.
func refusalText(err error) string {
	refusal := refusalOf(err)

	return textbuf.New().Str(refusal.Problem()).Str(" Fix: ").Str(refusal.Fix()).Str(".").String()
}

// rolloutsContent builds the Rollouts tab: the table, one rollout's page, or
// the new rollout form, as the sub and the id parameters say.
func rolloutsContent(ctx context.Context, request *PageRequest) (templ.Component, error) {
	if request.Address.View.Sub == newRolloutSub {
		return rolloutForm(ctx, request)
	}
	row := request.Address.View.Row
	if row == "" {
		return rolloutsTable(ctx, request)
	}

	return rolloutPage(ctx, request, row)
}

// rolloutsTable builds the table of every rollout the hub keeps, filtered.
func rolloutsTable(ctx context.Context, request *PageRequest) (templ.Component, error) {
	rollouts, err := request.Source.Rollouts(ctx)
	if err != nil {
		return nil, err
	}

	text := request.Address.View.Filter
	query := &QueryRowView{
		ID: "fleet-rollouts-query", Action: request.Address.Path(), Text: text,
		Placeholder: "Filter rollouts: state:paused actor:alice target:site=lon",
		Hidden:      treeViewHidden(request),
	}

	matched := []Rollout{}
	filter, refused := rolloutFilters.Parse(text, nil)
	if refused != nil {
		query.Refusal = refusalText(refused)
	} else {
		matched = filter.Apply(rollouts)
	}
	query.Matches = textbuf.New().Int(int64(len(matched))).Str(" of ").Int(int64(len(rollouts))).Str(" rollouts").String()

	rows := make([]RowView, 0, len(matched))
	for index := range matched {
		rollout := &matched[index]
		rows = append(rows, RowView{
			ID: rollout.Name, Name: rollout.Name,
			Cells: []CellView{
				LinkCell(rollout.Name, rolloutsAddress(request, rollout.Name, "").String()),
				TextCell(rollout.Target),
				StatusCell(rolloutStatus(rollout.State)),
				TextCell(rolloutProgress(rollout)),
				TextCell(rollout.Actor),
				whenCell(request, rollout.Started),
			},
		})
	}

	create := ActionView{Label: "New rollout", Icon: glyphPlus, Href: rolloutFormAddress(request, "").String()}

	return TableSection(TableSectionView{
		Title: "Rollouts", Create: &create, Query: query,
		Table: TableView{
			Name: "Rollouts",
			Headers: []ColumnHeaderView{
				{Label: "Name", Kind: "kind-text"}, {Label: "Target", Kind: "kind-text"},
				{Label: "State", Kind: "kind-state"}, {Label: "Progress", Kind: "kind-number"},
				{Label: "Actor", Kind: "kind-text"}, {Label: "Started", Kind: "kind-time"},
			},
			Rows:  rows,
			Empty: &StateHeroView{Kind: HeroNoData, Sentence: "No rollout matches the filter."},
		},
	}), nil
}

// rolloutProgress answers a rollout's progress as batches and as devices.
func rolloutProgress(rollout *Rollout) string {
	return textbuf.New().Int(int64(rollout.BatchesDone)).Str(" of ").Int(int64(rollout.BatchesTotal)).
		Str(" batches, ").Int(int64(rollout.DevicesDone)).Str(" of ").Int(int64(rollout.DevicesTotal)).
		Str(" devices").String()
}

// rolloutPage builds the page of one rollout: its header and controls, the
// pause banner, the batch strip and the devices table.
func rolloutPage(ctx context.Context, request *PageRequest, name string) (templ.Component, error) {
	detail, err := request.Source.Rollout(ctx, name)
	if err != nil {
		return nil, err
	}
	rollout := &detail.Rollout

	view := RolloutPageView{
		Back:   LinkView{Label: "All rollouts", Href: rolloutsAddress(request, "", "").String()},
		Name:   rollout.Name,
		Status: rolloutStatus(rollout.State),
		Facts: KeyValueListView{Rows: []ValueView{
			{Label: "Target", Text: rollout.Target},
			{Label: "Progress", Text: rolloutProgress(rollout)},
			{Label: "Actor", Text: rollout.Actor},
			{Label: "Started", Text: whenText(request, rollout.Started)},
		}},
		Form: manageForm(request),
	}
	view.Controls, view.Destructive = rolloutControls(rollout)

	if rollout.State == RolloutStatePaused {
		pause := &RolloutPauseView{Banner: CalloutView{
			Title: rollout.Name + " paused: " + rollout.PauseReason,
			Text:  "It waits for an operator. The devices that caused the pause:",
		}}
		for _, device := range detail.PauseDevices {
			pause.Causes = append(pause.Causes, LinkView{Label: device, Href: deviceTabHref(request, device, tabDeviceDashboard)})
		}
		view.Pause = pause
	}

	for _, batch := range detail.Batches {
		view.Batches = append(view.Batches, BatchSegmentView{
			Label: "Batch " + strconv.Itoa(batch.Number), Status: rolloutStatus(batch.State),
		})
	}

	view.Devices = rolloutDevices(request, &detail)

	return RolloutPage(view), nil
}

// rolloutControls answers the controls valid for the rollout's state: the
// routine ones, and apart from them the destructive ones, each confirmed by
// typing the rollout's name.
func rolloutControls(rollout *Rollout) ([]ActionView, []ActionView) {
	name := rollout.Name
	routineAction := func(key, label, detail string) ActionView {
		return manageAction(key, label, name, &ConfirmView{Action: label, Targets: name, Detail: detail})
	}
	destructiveAction := func(key, label, detail string) ActionView {
		return manageAction(key, label, name, &ConfirmView{
			Action: label, Targets: name, Detail: detail, Expect: name, Destructive: true,
		})
	}
	abort := destructiveAction("abort", "Abort", "The batches not started are dropped; the devices already changed keep their configuration.")
	force := destructiveAction("force-complete", "Force complete", "The rollout is marked complete without waiting for the batches still running.")

	switch rollout.State {
	case RolloutStatePending:
		return nil, []ActionView{abort}
	case RolloutStateRolling:
		return []ActionView{routineAction("pause", "Pause", "The rollout stops after the batch in progress.")}, []ActionView{abort, force}
	case RolloutStatePaused:
		return []ActionView{routineAction("resume", "Resume", "The rollout starts its next batch.")}, []ActionView{abort, force}
	case RolloutStateComplete, RolloutStateFailed, RolloutStateAborted:
		return nil, nil
	case RolloutStateUnspecified:
		return nil, nil
	}

	return nil, nil
}

// rolloutDevices answers the devices table of a rollout.
func rolloutDevices(request *PageRequest, detail *RolloutDetail) TableSectionView {
	rows := make([]RowView, 0, len(detail.Devices))
	for index := range detail.Devices {
		device := &detail.Devices[index]

		when := TextCell("")
		if device.Answered {
			when = whenCell(request, device.At)
		}

		rows = append(rows, RowView{
			ID: device.Device, Name: device.Device,
			Cells: []CellView{
				LinkCell(device.Device, deviceTabHref(request, device.Device, tabDeviceConfig)),
				NumberCell(strconv.Itoa(device.Batch)),
				TextCell(device.Result),
				TextCell(device.Answer),
				when,
			},
		})
	}

	return TableSectionView{Title: "Devices", Table: TableView{
		Name: "Devices of " + detail.Rollout.Name,
		Headers: []ColumnHeaderView{
			{Label: "Device", Kind: "kind-text"}, {Label: "Batch", Kind: "kind-number", Numeric: true},
			{Label: "Result", Kind: "kind-state"}, {Label: "Device's answer", Kind: "kind-text"},
			{Label: "Time", Kind: "kind-time"},
		},
		Rows:  rows,
		Empty: &StateHeroView{Kind: HeroNoData, Sentence: "The rollout targets no device."},
	}}
}

// rolloutForm builds the new rollout form over the target the filter
// parameter names: the choices of target, the counts it resolves to, what
// to push, the batch size, the failure threshold, the health gate and the
// preview of the batches.
func rolloutForm(ctx context.Context, request *PageRequest) (templ.Component, error) {
	saved, err := request.Source.SavedFilters(ctx, request.Username)
	if err != nil {
		return nil, err
	}

	choices, err := rolloutTargets(request, saved)
	if err != nil {
		return nil, err
	}

	target := request.Address.View.Filter
	view := RolloutFormView{
		Back:     LinkView{Label: "All rollouts", Href: rolloutsAddress(request, "", "").String()},
		Target:   targetWords(target),
		Percent:  rolloutBatchPercentDefault,
		Percents: rolloutBatchPercents,
		Choices:  choices,
		Query: QueryRowView{
			ID: "fleet-rollout-target", Action: request.Address.Path(), Text: target,
			Placeholder: "Target devices: site:lon role:edge",
			Hidden:      append(treeViewHidden(request), HiddenFieldView{Name: paramSub, Value: newRolloutSub}),
		},
		PreviewURL: FragmentLink(fragmentRolloutPreview, request.Address, ""),
		Form:       manageForm(request),
	}
	preview, err := rolloutPreview(request.Snapshot.Views, target, rolloutBatchPercentDefault)
	if err != nil {
		view.Query.Refusal = refusalText(err)
		view.Start = ActionView{Label: "Start", Disabled: true, Reason: "The target filter is refused, so it names no device."}
		return RolloutForm(view), nil
	}
	view.Preview = preview
	view.Start = rolloutStart(target, &preview)

	return RolloutForm(view), nil
}

// targetWords answers the target as the form names it.
func targetWords(filter string) string {
	if filter == "" {
		return "Every device of the estate"
	}

	return "The devices that match " + filter
}

// rolloutTargets answers the targets the form offers: the whole estate, the
// operator's saved filters and the group nodes of the first level of the
// tree view in hand.
func rolloutTargets(request *PageRequest, saved []SavedFilter) ([]LinkView, error) {
	choices := []LinkView{{Label: "Every device", Href: rolloutFormAddress(request, "").String()}}
	for _, filter := range saved {
		choices = append(choices, LinkView{
			Label: "Saved filter: " + filter.Name,
			Href:  rolloutFormAddress(request, filter.Filter).String(),
		})
	}

	tree, err := BuildTree(request.Snapshot.Estate.Name, request.treeView, request.Snapshot.Views)
	if err != nil {
		return nil, err
	}
	for index := range tree.Root.Children {
		node := &tree.Root.Children[index]
		if node.Level.Missing || strings.ContainsAny(node.Level.Value, " \t") {
			continue
		}
		choices = append(choices, LinkView{
			Label: "Group: " + node.Title,
			Href:  rolloutFormAddress(request, SetFilterTerm("", node.Level.Key, node.Level.Value)).String(),
		})
	}

	return choices, nil
}

// rolloutStart answers the Start action, confirmed with the counts of
// devices and batches, and disabled for a target with no connected device.
func rolloutStart(target string, preview *RolloutPreviewView) ActionView {
	if preview.Connected == 0 {
		return ActionView{Label: "Start", Disabled: true, Reason: "The target holds no connected device, so there is nothing to push."}
	}

	targets := textbuf.New().Int(int64(preview.Connected)).Str(" connected devices in ").
		Int(int64(len(preview.Batches))).Str(" batches").String()
	detail := textbuf.New().Int(int64(preview.Offline)).
		Str(" offline devices are skipped. One task is created per batch.").String()

	start := manageAction("start-rollout", "Start", targetWords(target), &ConfirmView{
		Action: "Start the rollout", Targets: targets, Detail: detail,
	})
	start.Icon = glyphPlus

	return start
}

// rolloutPreview resolves a target filter over the estate's devices and
// splits its connected devices into batches of percent of them: the count it
// changes, the count it skips as offline, and the devices of each batch.
func rolloutPreview(views []DeviceView, target string, percent int) (RolloutPreviewView, error) {
	filter, err := DeviceFilters().Parse(target, LabelKeys(views))
	if err != nil {
		return RolloutPreviewView{}, err
	}

	connected := []string{}
	offline := 0
	matched := filter.Apply(views)
	for index := range matched {
		if matched[index].Device.Connection.Connected {
			connected = append(connected, matched[index].Device.Name)
			continue
		}
		offline++
	}

	preview := RolloutPreviewView{Connected: len(connected), Offline: offline, Percent: percent}
	if len(connected) == 0 {
		return preview, nil
	}

	size := max((len(connected)*percent+99)/100, 1)
	for first := 0; first < len(connected); first += size {
		last := min(first+size, len(connected))
		preview.Batches = append(preview.Batches, RolloutBatchPreview{
			Number:  len(preview.Batches) + 1,
			Devices: textbuf.New().Join(connected[first:last], ", ").String(),
		})
	}

	return preview, nil
}

// rolloutPreviewFragment answers the preview of the new rollout form for the
// batch size its item names, as a percentage.
func rolloutPreviewFragment(_ context.Context, request *PageRequest, item string) (templ.Component, error) {
	percent := rolloutBatchPercentDefault
	if item != "" {
		chosen, err := strconv.Atoi(item)
		if err != nil {
			return nil, batchPercentRefused(item)
		}
		if !slices.Contains(rolloutBatchPercents, chosen) {
			return nil, batchPercentRefused(item)
		}
		percent = chosen
	}

	preview, err := rolloutPreview(request.Snapshot.Views, request.Address.View.Filter, percent)
	if err != nil {
		return nil, err
	}

	return RolloutPreview(preview), nil
}

// batchPercentRefused is the refusal for a batch size the form does not
// offer.
func batchPercentRefused(item string) error {
	return opererr.New(
		textbuf.New().Str("The batch size ").Quoted(item).Str(" is not one the form offers.").String(),
		"A batch is a share of the connected devices of the target, chosen from the form's list.",
		"choose 10, 25, 50 or 100 percent in the form")
}
