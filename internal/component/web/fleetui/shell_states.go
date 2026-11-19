// Design: docs/architecture/fleet-interface.md -- the states the shell shows in place of a page, and the task panel it fills
// Related: component_hero.templ -- StateHero, which draws a state
// Related: component_task.templ -- TaskPanel, which draws the task panel
// Related: component_panel.templ -- PanelContent, which draws the side panel
// Related: source_errors.go -- FailureOf, which names the state of a failed answer
//
// A pane never renders blank. When a page cannot show its data, the shell
// shows the state that matches the reason, built here from the operator
// error the data layer answered: its problem is the sentence, its reason the
// second sentence, and its fix the line the operator can copy.

package fleetui

import (
	"context"
	"errors"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The names of the icons of the actions a state offers.
const (
	iconRetry = "retry"
	iconOpen  = "chevron-right"
)

// The first lines of the good news a state shows when nothing is wrong.
const (
	goodNewsAllGood  = "All good"
	goodNewsAllQuiet = "All quiet"
)

// refusalOf answers err as an operator error. An error that is none yet is
// wrapped, so every state has a sentence, a reason and a fix to show.
func refusalOf(err error) *opererr.Error {
	if refusal, isOperatorError := errors.AsType[*opererr.Error](err); isOperatorError {
		return refusal
	}

	return opererr.Wrap(err, "This view could not be read.",
		"The fleet data source failed without saying why, so there is nothing to show.",
		"ze show doctor")
}

// retryAction is the action of a state the operator can simply try again:
// the page in hand, asked for once more.
func retryAction(request *PageRequest) *ActionView {
	return &ActionView{Label: "Retry", Icon: iconRetry, Href: request.Address.String()}
}

// estateAction is the action of a state whose address leads nowhere: the
// estate, which always exists.
func estateAction(request *PageRequest) *ActionView {
	return &ActionView{Label: "Open the estate", Icon: iconOpen, Href: estateAddress(request).String()}
}

// deviceAction is the action of a state that sends the operator to another
// tab of the device in hand. A page that shows no single device offers
// none: the state then names the device in its sentence.
func deviceAction(request *PageRequest, tab, label, icon string) *ActionView {
	if request.Address.Kind != ObjectDevice {
		return nil
	}

	target := Address{Kind: ObjectDevice, Device: request.Address.Device, Tab: tab, View: linkView(request)}

	return &ActionView{Label: label, Icon: icon, Href: target.String()}
}

// failureHeroView answers the state a pane shows for err, which MUST be
// non-nil: the kind FailureOf names, in the operator error's own words.
func failureHeroView(request *PageRequest, err error) StateHeroView {
	refusal := refusalOf(err)

	view := StateHeroView{Sentence: refusal.Problem(), Detail: refusal.Why()}

	switch FailureOf(err) {
	case FailureNotFound:
		view.Kind = HeroNoData
		view.Action = estateAction(request)
	case FailureOffline:
		view.Kind = HeroOffline
		view.Action = deviceAction(request, tabDeviceTasks, "Open the device's tasks", iconTasks)
	case FailureRelayRefused:
		// The fix of a refused relay is the configuration line that grants
		// it, which the operator copies to the device.
		view.Kind = HeroRelayRefused
		view.Line = refusal.Fix()
		view.LineName = heroLineConfiguration
	case FailureUnsupported:
		view.Kind = HeroNotSupported
		view.Action = deviceAction(request, tabDeviceSystem, "Show the device's version", iconDevice)
	case FailureUnauthorized:
		view.Kind = HeroUnauthorized
	case FailureDeadline, FailureFailed:
		view.Kind = HeroFailed
		view.Line = refusal.Fix()
		view.LineName = "the command"
		view.Action = retryAction(request)
	case FailureUnspecified:
		panic("BUG: fleetui: FailureOf answered no failure")
	default:
		panic("BUG: fleetui: FailureOf answered a failure outside its set")
	}

	return view
}

// failureHero answers the state a pane shows for err, which MUST be non-nil.
func failureHero(request *PageRequest, err error) templ.Component {
	return StateHero(failureHeroView(request, err))
}

// notBuiltHeroView answers the state of a tab no page family registered a
// page for. The shell is complete without the page, and says which one is
// missing.
func notBuiltHeroView(request *PageRequest) StateHeroView {
	address := request.Address

	sentence := textbuf.New().Str("The ")
	if address.Tab != "" {
		sentence.Str(tabTitle(address)).Str(" page of ")
	}
	sentence.Str(address.Kind.String()).Str(" is not built yet.")

	return StateHeroView{
		Kind:     HeroNoData,
		Sentence: sentence.Slice(),
		Detail:   "The address is right and the shell around it is complete. The page that fills this tab has not been added.",
		Action:   estateAction(request),
	}
}

// notBuiltHero answers the state of a tab with no registered page.
func notBuiltHero(request *PageRequest) templ.Component {
	return StateHero(notBuiltHeroView(request))
}

// panelEmpty answers the side panel with no row chosen: the sentence that
// tells the operator how to fill it.
func panelEmpty() templ.Component {
	return PanelContent(PanelView{Empty: true})
}

// taskTab is one tab of the task panel. The zero value is not a tab.
type taskTab uint8

// The tabs of the task panel, in the order it shows them.
const (
	taskTabUnspecified taskTab = iota
	taskTabRunning
	taskTabSucceeded
	taskTabFailed
	taskTabAll
)

// bugTaskTab is the panic of a task panel asked for a tab outside its set.
// parseTaskTab answers only a named tab, so only a defect reaches it.
const bugTaskTab = "BUG: fleetui: the task panel was asked for a tab it does not have"

// heroLineConfiguration names the line a relay refused state shows: the
// configuration line that grants the relay.
const heroLineConfiguration = "the configuration line"

// The tabs of a task panel, as its address names them, that share their text
// with a task state.
const (
	taskTabKeyRunning   = "running"
	taskTabKeySucceeded = "succeeded"
)

// The tabs of a device a state links to.
const (
	tabDeviceTasks  = "tasks"
	tabDeviceSystem = "system"
)

// taskPanelRowsMax is the most tasks the task panel lists under one tab. The
// task list page shows the rest.
const taskPanelRowsMax = 20

var (
	taskTabKeys   = []string{enumKeyUnspecified, taskTabKeyRunning, taskTabKeySucceeded, "failed", "all"}
	taskTabLabels = []string{enumWordUnspecified, enumWordRunning, enumWordSucceeded, enumWordFailed, "All"}
	taskTabEmpty  = []GoodNewsView{
		{},
		{First: goodNewsAllQuiet, Second: "No task is running"},
		{First: "Nothing yet", Second: "No task has succeeded"},
		{First: goodNewsAllGood, Second: "No task failed"},
		{First: goodNewsAllQuiet, Second: "The hub keeps no task"},
	}
	taskTabs = []taskTab{taskTabRunning, taskTabSucceeded, taskTabFailed, taskTabAll}
)

// holds reports whether the tab lists a task in this state.
func (tab taskTab) holds(state TaskState) bool {
	switch tab {
	case taskTabRunning:
		return state.Active()
	case taskTabSucceeded:
		return state == TaskStateSucceeded
	case taskTabFailed:
		return taskFailed(state)
	case taskTabAll:
		return true
	case taskTabUnspecified:
		panic(bugTaskTab)
	default:
		panic(bugTaskTab)
	}
}

// taskTabUnknown is the refusal for a task panel tab that does not exist.
func taskTabUnknown(key string) error {
	problem := textbuf.New().Str("The task panel has no tab named ").Quoted(key).Str(".").Slice()

	return opererr.Wrap(ErrAddressUnknown, problem,
		"The panel lists tasks by state, and this names no state it lists, so it has nothing to show.",
		textbuf.New().Str("choose one of: ").Join(taskTabKeys[taskTabRunning:], ", ").Slice())
}

// parseTaskTab reads the tab a task panel fragment names in its item.
func parseTaskTab(key string) (taskTab, error) {
	tab, known := enumParse[taskTab](taskTabKeys, key)
	if !known {
		return taskTabUnspecified, taskTabUnknown(key)
	}

	return tab, nil
}

// taskRowView answers one task as the panel lists it.
func taskRowView(request *PageRequest, task *Task) TaskRowView {
	now := request.Snapshot.Now
	when := "Started " + RelativeTime(now.Sub(task.Started))
	if !task.State.Active() {
		when = "Ended " + RelativeTime(now.Sub(task.Ended))
	}

	return TaskRowView{
		Name:   TaskKindWords(task.Kind),
		Target: task.Target,
		Actor:  task.Actor,
		When:   when,
		Status: TaskStatus(task.State),
		Done:   task.Finished,
		Total:  len(task.Devices),
	}
}

// taskPanelTab answers the task panel with one tab open. open says whether
// the panel is drawn shown. Each row is the task list's row, so it expands
// to its per-device results and its eye opens the task; a task the source
// cannot answer fails the panel.
func taskPanelTab(ctx context.Context, request *PageRequest, active taskTab, open bool) (TaskPanelView, error) {
	tasks := request.Snapshot.Tasks
	page := estateAddress(request)

	view := TaskPanelView{
		Tabs:    make([]TaskTabView, 0, len(taskTabs)),
		List:    TaskListView{Empty: taskTabEmpty[active]},
		AllHref: Address{Kind: ObjectTasks, View: linkView(request)}.String(),
		Open:    open,
	}

	for _, tab := range taskTabs {
		count := 0
		for index := range tasks {
			if tab.holds(tasks[index].State) {
				count++
			}
		}

		view.Tabs = append(view.Tabs, TaskTabView{
			Label:  taskTabLabels[tab],
			Href:   FragmentLink(fragmentTasks, page, taskTabKeys[tab]),
			Count:  count,
			Active: tab == active,
		})
	}

	for index := range tasks {
		if len(view.List.Rows) == taskPanelRowsMax {
			break
		}
		if !active.holds(tasks[index].State) {
			continue
		}

		eye := linkView(request)
		eye.Row = tasks[index].ID
		row, err := activityTaskRow(ctx, request, &tasks[index], Address{Kind: ObjectTasks, View: eye}.String())
		if err != nil {
			return TaskPanelView{}, err
		}
		view.List.Rows = append(view.List.Rows, row)
	}

	return view, nil
}

// taskPanelView answers the task panel as a page first draws it: closed, on
// the tab of the tasks that are not over.
func taskPanelView(ctx context.Context, request *PageRequest) (TaskPanelView, error) {
	return taskPanelTab(ctx, request, taskTabRunning, false)
}

// taskPanelItem answers the task panel a fragment asks for. An empty item is
// the panel as a page first draws it, which is what a closed panel is
// refreshed with. A named tab is the panel shown, on that tab.
func taskPanelItem(ctx context.Context, request *PageRequest, item string) (TaskPanelView, error) {
	if item == "" {
		return taskPanelView(ctx, request)
	}

	tab, err := parseTaskTab(item)
	if err != nil {
		return TaskPanelView{}, err
	}

	return taskPanelTab(ctx, request, tab, true)
}
