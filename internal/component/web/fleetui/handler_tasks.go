// Design: docs/architecture/fleet-interface.md -- the task tabs, the task list, the task side panel and the task actions
// Related: register_activity_pages.go -- registers the pages and the fragment built here
// Related: page_activity.templ -- tasksSection and taskSide, which draw what is built here
// Related: shell_states.go -- taskRowView, the row of the header task panel, which these rows extend
// Related: handler_alarms.go -- the alarm tabs, which share the table helpers of this file
//
// A Tasks tab lists the tasks that targeted its object: every device of the
// estate, the devices of a group node, or one device. The task list page
// lists every task the hub keeps. Each row is a task and event row that
// expands to the answer of each device, and its eye opens the task in the
// side panel, where a running task can be canceled and a failed one retried.
// Nothing changes anything in the prototype: both actions are confirmed and
// answer a toast saying so.

package fleetui

import (
	"context"
	"maps"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// wordOpenTask labels the link to a task's own page.
const wordOpenTask = "Open the task"

// The tabs the activity pages register under or link to, as their address
// segment names them.
const (
	activityTabTasks  = "tasks"
	activityTabAlarms = "alarms"
	activityTabStates = "states"
	activityTabAudit  = "audit"
)

// The fields the task and alarm filters register besides fieldState.
const (
	activityFieldKind     = "kind"
	activityFieldActor    = "actor"
	activityFieldDevice   = "device"
	activityFieldSeverity = "severity"
)

// tasksSubject names the rows of the task tables in a refusal and a count.
const tasksSubject = "tasks"

// taskActionName is the registered action that answers a confirmed task
// action with the prototype's toast. Its item field is the verb, one space,
// the task.
const taskActionName = "task"

// The verbs of a task action, as its item spells them.
const (
	taskVerbCancel = "cancel"
	taskVerbRetry  = "retry"
)

// taskItemSeparator separates the verb of a task action from the task.
const taskItemSeparator = " "

// The ids of the forms a task's side panel submits its actions with.
const (
	taskFormCancel = "fleet-task-cancel"
	taskFormRetry  = "fleet-task-retry"
)

// changedNothing is the message of every toast that answers an action of the
// prototype.
const changedNothing = "The prototype changed nothing."

// The filter grammar of the task tables. init() fills it, and nothing
// removes from it. taskDeviceNames reads one device name of a task the way a
// text field reads its row: the device field asks it of every device.
var (
	taskFilters     = NewFilterSchema[Task](tasksSubject, taskContains, nil)
	taskDeviceNames = NewFilterSchema[string](tasksSubject, func(name *string, needle string) bool {
		return strings.Contains(strings.ToLower(*name), needle)
	}, nil)
)

// registerTaskFields registers the fields a task filter may name.
func registerTaskFields() {
	taskFilters.MustRegisterField(TextField(fieldState, taskStateKeys[1:], func(task *Task) string {
		return task.State.Key()
	}))
	taskFilters.MustRegisterField(TextField(activityFieldKind, nil, taskKind))
	taskFilters.MustRegisterField(TextField(activityFieldActor, nil, func(task *Task) string {
		return task.Actor
	}))
	taskFilters.MustRegisterField(taskDeviceField())
}

// taskKindWordTable holds the words of the task kinds the hub registers
// today. Kind is open (a registered kind, named by the hub), so a kind the
// table does not hold is shown as the hub names it.
var taskKindWordTable = map[string]string{
	"config-push":           "Config push",
	"command-batch":         "Command batch",
	"diagnostic-run":        "Diagnostic run",
	"divergence-resolution": "Divergence resolution",
	"rollout-batch":         "Rollout batch",
	"software-update":       "Software update",
}

// TaskKindWords answers a task kind as words, for every place a task kind is
// shown: task rows, the side panel, the filter menu and the toasts. A kind
// the hub registered after this table was written is shown by its own name.
func TaskKindWords(kind string) string {
	if words, known := taskKindWordTable[kind]; known {
		return words
	}

	return kind
}

// taskKind answers the kind of a task.
func taskKind(task *Task) string {
	return task.Kind
}

// taskContains reports whether a task holds a bare word of a filter, which
// arrives in lower case: in its identifier, its kind, its actor, its target or
// the name of one of its devices.
func taskContains(task *Task, needle string) bool {
	for _, text := range []string{task.ID, task.Kind, task.Actor, task.Target} {
		if strings.Contains(strings.ToLower(text), needle) {
			return true
		}
	}

	return slices.ContainsFunc(task.Devices, func(name string) bool {
		return strings.Contains(strings.ToLower(name), needle)
	})
}

// taskDeviceField builds the device field of the task filter. A task targets
// several devices, so a term holds for a task when one of its devices has a
// name the term lists, and a negated term holds for a task none of whose
// devices has one.
func taskDeviceField() FilterField[Task] {
	return FilterField[Task]{
		name: activityFieldDevice,
		compile: func(schema *FilterSchema[Task], term, value string) (rowTest[Task], error) {
			// The value is judged as typed against the task grammar, so a
			// refusal names the term and lists the task fields. The reader
			// passed is never called: only the refusal is kept.
			if _, err := schema.compileText(term, value, nil, taskKind); err != nil {
				return nil, err
			}

			list, negated := strings.CutPrefix(value, filterNegation)
			named, err := taskDeviceNames.compileText(term, list, nil, func(name *string) string { return *name })
			if err != nil {
				return nil, err
			}

			return func(task *Task) bool {
				held := slices.ContainsFunc(task.Devices, func(name string) bool { return named(&name) })
				return held != negated
			}, nil
		},
	}
}

// activityDevices answers the devices of the object the request shows: every
// device for the estate and the task list, the members of a group node, the
// device itself. It refuses a group no device is in and a device the estate
// does not hold, with the data layer's own error.
func activityDevices(request *PageRequest) ([]DeviceView, error) {
	address := request.Address
	views := request.Snapshot.Views

	switch address.Kind {
	case ObjectEstate, ObjectTasks:
		return views, nil
	case ObjectGroup:
		return GroupDevices(views, address.Group)
	case ObjectDevice:
		view, err := FindDevice(views, address.Device)
		if err != nil {
			return nil, err
		}

		return []DeviceView{*view}, nil
	case ObjectAdmin:
		panic("BUG: fleetui: an administration page asked for the devices of its object")
	case ObjectUnspecified:
		panic(bugAddressKind)
	default:
		panic(bugAddressKind)
	}
}

// scopedTasks answers the tasks the page lists: every task the hub keeps for
// the task list, and the tasks that targeted the object for a Tasks tab.
func scopedTasks(request *PageRequest) ([]Task, error) {
	tasks := request.Snapshot.Tasks
	if request.Address.Kind == ObjectTasks {
		return tasks, nil
	}

	devices, err := activityDevices(request)
	if err != nil {
		return nil, err
	}

	return ScopeTasks(tasks, devices), nil
}

// tasksContent builds the content of a Tasks tab and of the task list: the
// table stack over the tasks of the object, filtered and paged by the
// address. A filter the grammar refuses is shown under the field, and the
// rows are then the unfiltered ones.
func tasksContent(ctx context.Context, request *PageRequest) (templ.Component, error) {
	tasks, err := scopedTasks(request)
	if err != nil {
		return nil, err
	}

	view := request.Address.View

	kept, refusal := tasks, ""
	filter, err := taskFilters.Parse(view.Filter, nil)
	if err != nil {
		refusal = activityRefusalText(err)
	}
	if err == nil {
		kept = filter.Apply(tasks)
	}

	page := Paginate(len(kept), view.Page, view.Size)
	rows := PageRows(kept, page)

	list := TaskListView{Rows: make([]TaskRowView, 0, len(rows)), Empty: tasksEmpty(view.Filter)}
	for index := range rows {
		row, rowErr := activityTaskRow(ctx, request, &rows[index], activityRowHref(request.Address, rows[index].ID))
		if rowErr != nil {
			return nil, rowErr
		}
		list.Rows = append(list.Rows, row)
	}

	matches := activityMatches(len(kept), len(tasks), tasksSubject)
	kinds := taskKindsInUse(tasks)
	menus := []MenuView{
		activityFilterMenu(request, columnWordState, fieldState, taskStateKeys[1:], taskStateWords[1:]),
		activityFilterMenu(request, wordKind, activityFieldKind, kinds, taskKindWordsOf(kinds)),
	}

	return tasksSection(TasksSectionView{
		Title: tasksTitle,
		Query: activityQuery(request, "fleet-tasks-filter",
			"Filter tasks, as in state:failed kind:config-push device:edge-lon-01", matches, refusal, menus),
		Pager: activityPager(request.Address, page, tasksSubject),
		List:  list,
	}), nil
}

// tasksEmpty is what a task table with no row says: no match for a filter,
// and the good news of a quiet object otherwise.
func tasksEmpty(filter string) GoodNewsView {
	if filter != "" {
		return GoodNewsView{First: "No match", Second: "No task matches this filter"}
	}

	return GoodNewsView{First: goodNewsAllQuiet, Second: "No task targeted this object"}
}

// taskKindsInUse answers the kinds of tasks, each once, in text order.
func taskKindsInUse(tasks []Task) []string {
	kinds := make([]string, 0, len(tasks))
	for index := range tasks {
		kinds = append(kinds, tasks[index].Kind)
	}
	slices.Sort(kinds)

	return slices.Compact(kinds)
}

// taskKindWordsOf answers the words of each kind, in the order of kinds.
func taskKindWordsOf(kinds []string) []string {
	words := make([]string, len(kinds))
	for index, kind := range kinds {
		words[index] = TaskKindWords(kind)
	}

	return words
}

// activityTaskRow answers one task as a task and event row: the row of the
// header task panel, with the time it took once ended, its eye opening it at
// eye, and one result per device that answered, which the row expands to.
func activityTaskRow(ctx context.Context, request *PageRequest, task *Task, eye string) (TaskRowView, error) {
	detail, err := request.Source.Task(ctx, task.ID)
	if err != nil {
		return TaskRowView{}, err
	}

	row := taskRowView(request, task)
	if !task.State.Active() {
		row.When += ", after " + activityDuration(task.Ended.Sub(task.Started))
	}
	row.EyeHref = eye
	row.Results = taskResultViews(request, &detail)

	return row, nil
}

// taskResultViews answers the result of each device of a task, each linking
// to the task opened on that device's Tasks tab.
func taskResultViews(request *PageRequest, detail *TaskDetail) []TaskResultView {
	results := make([]TaskResultView, 0, len(detail.Results))
	for index := range detail.Results {
		result := &detail.Results[index]
		results = append(results, TaskResultView{
			Device:   result.Device,
			Href:     taskOnDevice(request, result.Device, detail.Task.ID),
			Answer:   answerText(result),
			Duration: activityDuration(result.Duration),
			Status:   TaskStatus(result.State),
		})
	}

	return results
}

// taskOnDevice answers the address of the Tasks tab of a device with the task
// opened in its side panel.
func taskOnDevice(request *PageRequest, device, id string) string {
	view := linkView(request)
	view.Row = id

	return Address{Kind: ObjectDevice, Device: device, Tab: activityTabTasks, View: view}.String()
}

// answerText answers what a device answered, with a note when the answer was
// cut to fit.
func answerText(result *TaskResult) string {
	if !result.Truncated {
		return result.Answer
	}

	return textbuf.New().Str(result.Answer).Str(" [the answer was longer than ").
		Int(TaskAnswerBytesMax).Str(" bytes and was cut]").String()
}

// taskSidePanel builds the side panel of the task the address names: its
// fields, the result of each device, and the actions its state allows.
func taskSidePanel(ctx context.Context, request *PageRequest) (templ.Component, error) {
	detail, err := request.Source.Task(ctx, request.Address.View.Row)
	if err != nil {
		return nil, err
	}

	return taskSide(taskSideView(request, &detail)), nil
}

// taskSideView fills the side panel of one task.
func taskSideView(request *PageRequest, detail *TaskDetail) TaskSideView {
	task := &detail.Task

	closed := request.Address
	closed.View.Row = ""

	side := TaskSideView{Panel: PanelView{
		Title:     TaskKindWords(task.Kind) + " " + task.ID,
		CloseHref: closed.String(),
		Icon:      ObjectIconView{Icon: iconTasks, Status: TaskStatus(task.State), Marked: true},
		Cards:     taskCards(request, detail),
	}}

	action := ActionAddress(taskActionName, request.Address)

	if task.State.Active() {
		side.Forms = append(side.Forms, ActionFormView{ID: taskFormCancel, Action: action, Item: taskVerbCancel + taskItemSeparator + task.ID})
		side.Panel.Actions = append(side.Panel.Actions, ActionView{
			Label: "Cancel", Icon: glyphClose, Form: taskFormCancel,
			Confirm: &ConfirmView{
				Action:      "Cancel the task " + task.ID,
				Targets:     ProgressText(task.Finished, len(task.Devices)) + " have a result",
				Detail:      "The devices that have not answered are left as they are. Those already done keep their result.",
				Expect:      task.ID,
				Destructive: true,
			},
		})
	}

	if taskFailed(task.State) {
		failed := failedDevices(detail)
		side.Forms = append(side.Forms, ActionFormView{ID: taskFormRetry, Action: action, Item: taskVerbRetry + taskItemSeparator + task.ID})
		side.Panel.Actions = append(side.Panel.Actions, ActionView{
			Label: "Retry", Icon: glyphRetry, Form: taskFormRetry,
			Confirm: &ConfirmView{
				Action:  "Retry the task " + task.ID,
				Targets: textbuf.New().Str("The failed devices only: ").Join(failed, ", ").String(),
				Detail:  "A new task is started over these devices. The devices that succeeded are not asked again.",
			},
		})
	}

	return side
}

// taskCards answers the cards of a task's side panel: the task, then one per
// device that answered, then the devices still waiting.
func taskCards(request *PageRequest, detail *TaskDetail) []PanelCardView {
	task := &detail.Task

	rows := []ValueView{
		{Label: "Identifier", Text: task.ID, Mono: true, Copy: true},
		{Label: wordKind, Text: TaskKindWords(task.Kind)},
		{Label: columnWordState, Status: TaskStatus(task.State)},
		{Label: rolloutsWordProgress, Text: ProgressText(task.Finished, len(task.Devices))},
		{Label: enumWordFailed, Text: textbuf.StringInt(int64(task.Failed))},
		{Label: auditWordActor, Text: task.Actor},
		{Label: "Targets", Text: task.Target},
		{Label: rolloutsWordStarted, Text: activityTime(task.Started)},
	}
	if !task.State.Active() {
		rows = append(rows,
			ValueView{Label: "Ended", Text: activityTime(task.Ended)},
			ValueView{Label: "Duration", Text: activityDuration(task.Ended.Sub(task.Started))})
	}
	audit := Address{Kind: ObjectAdmin, Tab: activityTabAudit, View: linkView(request)}
	rows = append(rows, ValueView{
		Label: "Audit entries", Text: strings.Join(detail.AuditIDs, ", "), Href: audit.String(), Icon: iconAdmin,
	})

	cards := make([]PanelCardView, 0, len(detail.Results)+2)
	cards = append(cards, PanelCardView{Title: "Task", Rows: rows})

	answered := make(map[string]struct{}, len(detail.Results))
	for index := range detail.Results {
		result := &detail.Results[index]
		answered[result.Device] = struct{}{}
		cards = append(cards, PanelCardView{
			Title: result.Device,
			Rows: []ValueView{
				{Label: auditWordResult, Status: TaskStatus(result.State)},
				{Label: "Answer", Text: answerText(result), Mono: true},
				{Label: "Took", Text: activityDuration(result.Duration)},
			},
			Link: &LinkView{Label: "Open the device", Href: deviceAddress(request, result.Device).String()},
		})
	}

	waiting := make([]string, 0, len(task.Devices))
	for _, name := range task.Devices {
		if _, held := answered[name]; !held {
			waiting = append(waiting, name)
		}
	}
	if len(waiting) > 0 {
		cards = append(cards, PanelCardView{Title: "Waiting for an answer", Rows: []ValueView{
			{Label: devicesWordDevices, Text: strings.Join(waiting, ", ")},
		}})
	}

	return cards
}

// failedDevices answers the devices of a task whose result is a failure,
// which a retry targets.
func failedDevices(detail *TaskDetail) []string {
	failed := make([]string, 0, len(detail.Results))
	for index := range detail.Results {
		if taskFailed(detail.Results[index].State) {
			failed = append(failed, detail.Results[index].Device)
		}
	}

	return failed
}

// taskActionAnswer answers a confirmed task action. The prototype changes
// nothing, so the answer is the toast that says so, or the toast of the
// refusal when the action does not apply.
func taskActionAnswer(ctx context.Context, request *PageRequest, form url.Values) (ToastView, error) {
	return taskActionToast(ctx, request, form.Get(paramItem)), nil
}

// taskActionToast answers the toast of one task action, which item names.
func taskActionToast(ctx context.Context, request *PageRequest, item string) ToastView {
	list := &LinkView{Label: "Open the task list", Href: Address{Kind: ObjectTasks, View: linkView(request)}.String()}

	// An id longer than a toast quotes names no task, and is never handed to
	// the source, which would quote it whole in its refusal.
	verb, id, named := strings.Cut(item, taskItemSeparator)
	if !named || utf8.RuneCountInString(id) > echoMax {
		return refusalToast(taskActionUnknown(item), list)
	}

	detail, err := request.Source.Task(ctx, id)
	if err != nil {
		return refusalToast(err, list)
	}
	task := &detail.Task

	opened := linkView(request)
	opened.Row = task.ID
	link := &LinkView{Label: wordOpenTask, Href: Address{Kind: ObjectTasks, View: opened}.String()}

	switch verb {
	case taskVerbCancel:
		if !task.State.Active() {
			return refusalToast(taskActionRefused(task, "canceled", "only a pending or a running task can be stopped"), link)
		}

		return ToastView{
			Level:   ToastInformational,
			Message: changedNothing,
			Detail: "The cancel of " + TaskKindWords(task.Kind) + " " + task.ID +
				" was confirmed, and no task was stopped. Devices that had finished keep their result.",
			Link: link,
		}
	case taskVerbRetry:
		if !taskFailed(task.State) {
			return refusalToast(taskActionRefused(task, "retried", "only a failed or a partly failed task can be retried"), link)
		}

		failed := failedDevices(&detail)

		return ToastView{
			Level:   ToastInformational,
			Message: changedNothing,
			Detail: textbuf.New().Str("The retry of ").Str(TaskKindWords(task.Kind)).Byte(' ').Str(task.ID).
				Str(" over its failed devices (").Join(failed, ", ").Str(") was confirmed, and no task was started.").String(),
			Link: link,
		}
	}

	return refusalToast(taskActionUnknown(item), link)
}

// taskActionUnknown is the refusal of a task action item that names no verb
// and task.
func taskActionUnknown(item string) error {
	problem := textbuf.New().Str("The task action ").Quoted(echoText(item)).Str(" is not one the fleet interface offers.").String()

	return opererr.Wrap(ErrAddressUnknown, problem,
		"An action names its verb and its task, and this names neither a known verb nor a task, so nothing was done.",
		"use the Cancel or Retry control in the side panel of a task")
}

// taskActionRefused is the refusal of an action the task's state does not
// allow.
func taskActionRefused(task *Task, done, rule string) error {
	problem := textbuf.New().Str("The task ").Str(task.ID).Str(" cannot be ").Str(done).
		Str(", because its state is ").Str(task.State.String()).Str(".").String()

	return opererr.New(problem, "In the fleet interface "+rule+", so nothing was done.",
		"open the task again: its side panel offers the actions its state allows")
}

// refusalToast answers a refusal as an error toast that stays until
// dismissed, linking to the page where the operator can try again.
func refusalToast(err error, link *LinkView) ToastView {
	refusal := refusalOf(err)

	return ToastView{
		Level:   ToastError,
		Message: refusal.Problem(),
		Detail:  refusal.Why() + " Fix: " + refusal.Fix(),
		Link:    link,
	}
}

// activityRefusalText answers a refused filter as the query row writes it under the
// field: what is wrong, why, and the fix.
func activityRefusalText(err error) string {
	refusal := refusalOf(err)

	return refusal.Problem() + " " + refusal.Why() + " Fix: " + refusal.Fix()
}

// refusalsText answers the refusals one table met, each as
// activityRefusalText writes it, in the order they were met: a filter and a
// sort refused together are both shown, so the operator learns why the rows
// are unfiltered as well as why they are unsorted.
func refusalsText(errs []error) string {
	texts := make([]string, 0, len(errs))
	for _, err := range errs {
		texts = append(texts, activityRefusalText(err))
	}

	return textbuf.New().Join(texts, " ").Slice()
}

// activityMatches answers the count of rows a filter keeps: "3 of 15 tasks".
func activityMatches(kept, total int, noun string) string {
	return textbuf.New().Int(int64(kept)).Str(" of ").Int(int64(total)).Byte(' ').Str(noun).String()
}

// activityRowHref answers address with the row id opened in the side panel.
func activityRowHref(address Address, id string) string {
	address.View.Row = id

	return address.String()
}

// activityHidden answers view as the hidden fields of a form of the table, in
// the order of their names, so one view always draws the same form. The
// caller clears the parts the form sets itself.
func activityHidden(view ViewParams) []HiddenFieldView {
	query := view.Query()

	fields := make([]HiddenFieldView, 0, len(query))
	for _, name := range slices.Sorted(maps.Keys(query)) {
		for _, value := range query[name] {
			fields = append(fields, HiddenFieldView{Name: name, Value: value})
		}
	}

	return fields
}

// activityQuery answers the query row of an activity table. The form sets
// the filter and starts again at the first page with no row opened, so those
// parts are not carried along.
func activityQuery(request *PageRequest, id, placeholder, matches, refusal string, menus []MenuView) *QueryRowView {
	carried := request.Address.View
	carried.Filter = ""
	carried.Page = 1
	carried.Row = ""

	return &QueryRowView{
		ID:          id,
		Action:      request.Address.Path(),
		Text:        request.Address.View.Filter,
		Placeholder: placeholder,
		Matches:     matches,
		Refusal:     refusal,
		Hidden:      activityHidden(carried),
		Menus:       menus,
	}
}

// activityFilterMenu answers an assisted filter menu: one entry per value, each the
// page's address with the term written into its filter (SetFilterTerm).
// words holds the word shown for each value, in the same order.
func activityFilterMenu(request *PageRequest, label, field string, values, words []string) MenuView {
	menu := MenuView{Label: label, Icon: glyphFilter, Items: make([]ActionView, 0, len(values))}

	for index, value := range values {
		filter, carried := SetFilterTerm(request.Address.View.Filter, field, value)
		if !carried {
			continue
		}
		target := request.Address
		target.View.Filter = filter
		target.View.Page = 1
		target.View.Row = ""
		menu.Items = append(menu.Items, ActionView{Label: words[index], Href: target.String()})
	}

	return menu
}

// activityPager answers the pagination row of an activity table at page.
func activityPager(address Address, page PageWindow, noun string) *PagerView {
	carried := address.View
	carried.Size = PageSizeDefault
	carried.Page = 1
	carried.Row = ""

	return &PagerView{
		Noun:      noun,
		FirstHref: activityPageHref(address, 1),
		PrevHref:  activityPageHref(address, page.Number-1),
		NextHref:  activityPageHref(address, page.Number+1),
		LastHref:  activityPageHref(address, page.Pages),
		Action:    address.Path(),
		Hidden:    activityHidden(carried),
		Page:      page,
	}
}

// activityPageHref answers address on page number, with no row opened.
func activityPageHref(address Address, number int) string {
	address.View.Page = max(number, 1)
	address.View.Row = ""

	return address.String()
}

// activityTime answers a time as a tooltip and a side panel write it, on the
// hub's clock in UTC.
func activityTime(at time.Time) string {
	return at.UTC().Format(time.DateTime) + " UTC"
}

// activityDuration answers how long something took or has lasted, in its
// largest whole unit: "4 minutes".
func activityDuration(span time.Duration) string {
	if span < time.Second {
		return "under a second"
	}
	if span < time.Minute {
		return activityUnits(int64(span/time.Second), "second")
	}
	if span < time.Hour {
		return activityUnits(int64(span/time.Minute), "minute")
	}
	if span < 24*time.Hour {
		return activityUnits(int64(span/time.Hour), "hour")
	}

	return activityUnits(int64(span/(24*time.Hour)), "day")
}

// activityUnits answers a count and its unit, the unit plural unless the count
// is one.
func activityUnits(count int64, unit string) string {
	text := textbuf.New().Int(count).Byte(' ').Str(unit)
	if count != 1 {
		text.Byte('s')
	}

	return text.String()
}
