// Design: docs/architecture/fleet-interface.md -- the actions of a devices table's selection
// Related: handler_devices.go -- the table that draws these actions
// Related: register_devices_pages.go -- registers the action
// Related: handler_action.go -- the action route that answers it
//
// A selection of the devices table is acted on through one action. In
// the prototype nothing changes anything: each action answers a toast that
// names the action and the devices it would have acted on.

package fleetui

import (
	"context"
	"net/url"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The fields the bulk form posts. TableActions names the selection fields:
// row once per checked row, and all, set while the selection is every row
// the filter matches.
const (
	devicesFieldAction = "action"
	devicesFieldRow    = "row"
	devicesFieldAll    = "all"
	// devicesAllMatches is the value of devicesFieldAll while the selection
	// is every match.
	devicesAllMatches = "1"
)

// devicesActionName is the registered action of a devices table's
// selection. The path of the table's page follows it in the action's
// address, so the action knows which object's devices the selection and the
// filter are about.
const devicesActionName = "devices"

// devicesActionFilterSave is the action of the Save control of the query
// row. It posts the bulk form, which carries the filter, and needs no
// selected row.
const devicesActionFilterSave = "filter-save"

// devicesPrototypeWord is the message of every action the prototype answers.
const devicesPrototypeWord = "The prototype changed nothing"

// devicesToastNamesMax is how many device names a toast lists before it
// counts the others.
const devicesToastNamesMax = 5

// devicesCountToken stands for the number of selected devices in the text a
// destructive confirmation asks the operator to type. The count is known in
// the browser only, so the confirm dialog's script writes it in.
const devicesCountToken = "{count}"

// devicesDiagnosticDetail is what the confirmation of a diagnostic says.
const devicesDiagnosticDetail = "A read-only command: it creates a task with one result per device, and changes nothing on the devices."

// devicesLabelDetail is what the confirmation of a label edit says.
const devicesLabelDetail = "The label edit is staged in your candidate configuration on the hub: nothing changes until it is committed."

// devicesPlace is where the table draws an action of a selection. The zero
// value is not a place.
type devicesPlace uint8

// The places of an action.
const (
	// devicesPlaceUnspecified is the invalid zero value.
	devicesPlaceUnspecified devicesPlace = iota
	// devicesPlaceBar is the table actions row.
	devicesPlaceBar
	// devicesPlaceMore is the routine part of the More menu.
	devicesPlaceMore
	// devicesPlaceDestructive is the part of the More menu listed last and
	// apart, whose actions are confirmed by typing the count of devices.
	devicesPlaceDestructive
)

// devicesAction is one action of a selection of devices. The table draws it
// and the action route answers it, both from devicesActions.
type devicesAction struct {
	key    string
	label  string
	icon   string
	detail string
	place  devicesPlace
}

// devicesActions are the actions of a selection, in the order the table
// lists them. The three diagnostics are read-only sample tools.
var devicesActions = []devicesAction{
	{key: "label-add", label: "Add label", icon: glyphPlus, detail: devicesLabelDetail, place: devicesPlaceBar},
	{key: "label-remove", label: "Remove label", icon: glyphClose, detail: devicesLabelDetail, place: devicesPlaceBar},
	{
		key: "config-push", label: "Push config", icon: iconAdmin, place: devicesPlaceBar,
		detail: "A rollout of the hub's configuration starts, with the selected devices as its target.",
	},
	{
		key: "diagnostic-version", label: "Run diagnostic: show version", icon: glyphTerminal,
		detail: devicesDiagnosticDetail, place: devicesPlaceMore,
	},
	{
		key: "diagnostic-health", label: "Run diagnostic: show health", icon: glyphTerminal,
		detail: devicesDiagnosticDetail, place: devicesPlaceMore,
	},
	{
		key: "diagnostic-peers", label: "Run diagnostic: show bgp peer list", icon: glyphTerminal,
		detail: devicesDiagnosticDetail, place: devicesPlaceMore,
	},
	{
		key: "estate-remove", label: "Remove from the estate", icon: glyphTrash, place: devicesPlaceDestructive,
		detail: "The hub forgets the devices: each must enrol again to come back.",
	},
}

// view answers the action as the table draws it: a button of the bulk form,
// enabled while a row is selected, that asks for confirmation first.
func (action *devicesAction) view() ActionView {
	confirm := &ConfirmView{Action: action.label, Targets: "The selected devices", Detail: action.detail}
	if action.place == devicesPlaceDestructive {
		confirm.Destructive = true
		confirm.Expect = devicesCountToken
	}

	return ActionView{
		Label: action.label, Icon: action.icon, Form: devicesFormID,
		Name: devicesFieldAction, Value: action.key, Bulk: true, Confirm: confirm,
	}
}

// devicesActionViews answers the actions the table draws at place.
func devicesActionViews(place devicesPlace) []ActionView {
	views := make([]ActionView, 0, len(devicesActions))
	for index := range devicesActions {
		if devicesActions[index].place != place {
			continue
		}
		views = append(views, devicesActions[index].view())
	}

	return views
}

// devicesActionAddress answers the address the bulk form of the table at
// address posts to. The form carries the view's fields itself, so the
// address carries the page and no view.
func devicesActionAddress(address Address) string {
	address.View = ViewParams{}

	return ActionAddress(devicesActionName, address)
}

// devicesActionAnswer is the action of a devices table's selection. An
// address that names a page with no devices table wraps ErrAddressUnknown.
func devicesActionAnswer(_ context.Context, request *PageRequest, form url.Values) (ToastView, error) {
	if err := devicesTableOf(request.Address); err != nil {
		return ToastView{}, err
	}

	return devicesActionToast(request, form)
}

// devicesTableOf refuses an address that is not the Devices tab of the
// estate or of a group node, the two pages with a devices table.
func devicesTableOf(address Address) error {
	if address.Tab != devicesTab {
		return devicesUnknownTab(address.Path())
	}

	switch address.Kind {
	case ObjectEstate, ObjectGroup:
		return nil
	case ObjectDevice, ObjectTasks, ObjectAdmin:
		return devicesUnknownTab(address.Path())
	case ObjectUnspecified:
		panic(bugAddressKind)
	default:
		panic(bugAddressKind)
	}
}

// devicesActionToast answers the toast of one posted action: what the
// prototype would have done, to how many devices, naming them.
func devicesActionToast(request *PageRequest, form url.Values) (ToastView, error) {
	key := form.Get(devicesFieldAction)
	if key == devicesActionFilterSave {
		return devicesFilterSaveToast(form.Get(paramFilter)), nil
	}

	action, err := devicesActionOf(key)
	if err != nil {
		return ToastView{}, err
	}

	scope, err := devicesScope(request)
	if err != nil {
		return ToastView{}, err
	}

	targets, skipped, err := devicesTargets(scope, form)
	if err != nil {
		return ToastView{}, err
	}
	if len(targets) == 0 {
		return ToastView{
			Level:   ToastWarning,
			Message: "No device was selected.",
			Detail:  "Select at least one row, then choose " + action.label + " again.",
		}, nil
	}

	detail := textbuf.New().Str(action.label).Str(", ").Str(devicesCount(len(targets))).Str(": ")
	devicesNames(detail, targets)
	detail.Byte('.')
	if len(skipped) > 0 {
		detail.Str(" Skipped, because no longer in this table: ")
		devicesNames(detail, skipped)
		detail.Byte('.')
	}

	return ToastView{Level: ToastInformational, Message: devicesPrototypeWord, Detail: detail.Slice()}, nil
}

// devicesNames writes at most devicesToastNamesMax names, then the count of
// the others.
func devicesNames(text *textbuf.Buffer, names []string) {
	shown := min(len(names), devicesToastNamesMax)
	text.Join(names[:shown], ", ")
	if len(names) > shown {
		text.Str(", and ").Int(int64(len(names) - shown)).Str(" more")
	}
}

// devicesActionOf answers the action key names.
func devicesActionOf(key string) (*devicesAction, error) {
	for index := range devicesActions {
		if devicesActions[index].key == key {
			return &devicesActions[index], nil
		}
	}

	return nil, opererr.New(
		textbuf.New().Str("The action ").Quoted(key).Str(" is not an action of the devices table.").Slice(),
		"The table posts the actions of its buttons and menus, and this one names none of them, so nothing was done.",
		"choose the action from the table's buttons and menus")
}

// devicesTargets answers the devices a selection names: every device of the
// table the posted filter matches, when the selection is every match, or
// the checked rows. A checked row the table no longer holds, a device
// removed meanwhile, is skipped and answered apart, so the rest of the
// action still goes ahead.
func devicesTargets(scope []DeviceView, form url.Values) ([]string, []string, error) {
	if form.Get(devicesFieldAll) == devicesAllMatches {
		filter, err := DeviceFilters().Parse(form.Get(paramFilter), LabelKeys(scope))
		if err != nil {
			return nil, nil, err
		}

		matched := filter.Apply(scope)
		names := make([]string, 0, len(matched))
		for index := range matched {
			names = append(names, matched[index].Device.Name)
		}

		return names, nil, nil
	}

	rows := form[devicesFieldRow]
	if len(rows) > DevicesMax {
		return nil, nil, devicesSelectionTooLarge(len(rows))
	}

	held := make(map[string]struct{}, len(scope))
	for index := range scope {
		held[scope[index].Device.Name] = struct{}{}
	}

	seen := make(map[string]struct{}, len(rows))
	targets := make([]string, 0, len(rows))
	var skipped []string
	for _, name := range rows {
		if _, twice := seen[name]; twice {
			continue
		}
		seen[name] = struct{}{}

		if _, in := held[name]; in {
			targets = append(targets, name)
			continue
		}
		skipped = append(skipped, name)
	}

	return targets, skipped, nil
}

// devicesSelectionTooLarge is the refusal of a selection of more rows than
// an estate holds.
func devicesSelectionTooLarge(count int) error {
	return opererr.New(
		textbuf.New().Str("The selection holds ").Int(int64(count)).Str(" rows, more than the ").
			Int(int64(DevicesMax)).Str(" devices an estate holds.").Slice(),
		"A selection that large was not made by the table, so nothing was done.",
		"select every match instead of selecting rows")
}

// devicesFilterSaveToast answers the toast of the Save control: saved
// filters are fixed samples in the prototype, so nothing is stored.
func devicesFilterSaveToast(filter string) ToastView {
	if filter == "" {
		return ToastView{Level: ToastWarning, Message: "There is no filter to save.", Detail: "Type a filter first, then save it."}
	}

	detail := textbuf.New().Str("Save filter ").Quoted(filter).
		Str(": the saved filters are fixed samples in the prototype, so it was not stored.").Slice()

	return ToastView{Level: ToastInformational, Message: devicesPrototypeWord, Detail: detail}
}

// devicesUnknownTab is the refusal of an action posted for a page that has
// no devices table.
func devicesUnknownTab(path string) error {
	return opererr.Wrap(ErrAddressUnknown,
		textbuf.New().Str("The page ").Quoted(path).Str(" has no devices table.").Slice(),
		"Only the Devices tab of the estate and of a group node acts on a selection of devices, so nothing was done.",
		"open "+fleetPath+"estate/"+devicesTab+"/")
}
