package fleetui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

// activityTask is one task of the activity tests with what each of its
// devices answered.
type activityTask struct {
	task    Task
	results []TaskResult
}

// activityTasks answers the tasks of the activity tests, newest first, one in
// each state a task can be in:
//
//	t1  running        edge-lon-01 (succeeded), edge-lon-02 (waiting)
//	t2  succeeded      core-lon-01
//	t3  partly failed  core-lon-01 (succeeded), rs-ams-01 (failed)
//	t4  failed         cpe-0113 (failed, answer cut)
//	t5  pending        edge-lon-02
//	t6  canceled       core-ams-01
func activityTasks() []activityTask {
	minutes := func(count int) time.Time { return testNow.Add(-time.Duration(count) * time.Minute) }
	done := func(device string, state TaskState, answer string) TaskResult {
		return TaskResult{Device: device, State: state, Answer: answer, Duration: 4 * time.Second}
	}

	return []activityTask{
		{task: Task{
			ID: "t1", Kind: "config-push", Actor: "thomas", Target: "site=lon role=edge", State: TaskStateRunning,
			Devices: []string{edgeLon01, edgeLon02}, Finished: 1, Started: minutes(5),
		}, results: []TaskResult{done(edgeLon01, TaskStateSucceeded, "configuration applied")}},
		{task: Task{
			ID: "t2", Kind: "diagnostic-run", Actor: "hub", Target: coreLon01, State: TaskStateSucceeded,
			Devices: []string{coreLon01}, Finished: 1, Started: minutes(60), Ended: minutes(59),
		}, results: []TaskResult{done(coreLon01, TaskStateSucceeded, "all checks passed")}},
		{task: Task{
			ID: "t3", Kind: "config-push", Actor: "thomas", Target: "selection of 2 devices", State: TaskStatePartlyFailed,
			Devices: []string{coreLon01, rsAms01}, Finished: 2, Failed: 1, Started: minutes(130), Ended: minutes(120),
		}, results: []TaskResult{
			done(coreLon01, TaskStateSucceeded, "configuration applied"),
			done(rsAms01, TaskStateFailed, "refused: the configuration was edited on the device"),
		}},
		{task: Task{
			ID: "t4", Kind: "software-update", Actor: "thomas", Target: cpe0113, State: TaskStateFailed,
			Devices: []string{cpe0113}, Finished: 1, Failed: 1, Started: minutes(300), Ended: minutes(290),
		}, results: []TaskResult{{
			Device: cpe0113, State: TaskStateFailed, Answer: "disk full", Truncated: true, Duration: time.Minute,
		}}},
		{task: Task{
			ID: "t5", Kind: "command-batch", Actor: "thomas", Target: edgeLon02, State: TaskStatePending,
			Devices: []string{edgeLon02}, Started: minutes(1),
		}},
		{task: Task{
			ID: "t6", Kind: "diagnostic-run", Actor: "thomas", Target: coreAms01, State: TaskStateCanceled,
			Devices: []string{coreAms01}, Started: minutes(600), Ended: minutes(590),
		}},
	}
}

// activitySource is the test source of the activity pages. Over the shell's
// test source it holds the tasks of activityTasks and answers the two
// questions only these pages ask: one task with its per-device results, and
// a device's alarm history.
type activitySource struct {
	*testSource
	details []activityTask
}

// newActivitySource answers a source over the test estate with the tasks of
// activityTasks.
func newActivitySource() *activitySource {
	details := activityTasks()

	source := &activitySource{testSource: newTestSource(), details: details}
	source.tasks = make([]Task, 0, len(details))
	for index := range details {
		source.tasks = append(source.tasks, details[index].task)
	}

	return source
}

func (source *activitySource) Task(_ context.Context, id string) (TaskDetail, error) {
	for index := range source.details {
		held := &source.details[index]
		if held.task.ID == id {
			return TaskDetail{Task: held.task, Results: held.results, AuditIDs: []string{"audit-" + id}}, nil
		}
	}

	return TaskDetail{}, opererr.Wrap(ErrObjectUnknown, "No task is named "+id+".",
		"The test estate keeps no such task.", "ask for a task of activityTasks")
}

// AlarmHistory answers three alarms of every device: one open, one that
// cleared three days ago, and one that cleared ten days ago, which is older
// than the week the side panel shows.
func (source *activitySource) AlarmHistory(context.Context, string) ([]AlarmEvent, error) {
	days := func(count int) time.Time { return testNow.Add(-time.Duration(count) * 24 * time.Hour) }

	return []AlarmEvent{
		{Kind: "component-down", Severity: AlarmSeverityCritical, Summary: "bgp is down", Opened: testNow.Add(-2 * time.Hour)},
		{
			Kind: "resource-high", Severity: AlarmSeverityWarning, Summary: "memory above 90%",
			Opened: days(4), Cleared: true, ClearedAt: days(3),
		},
		{
			Kind: "config-rejected", Severity: AlarmSeverityWarning, Summary: "an old rejection",
			Opened: days(11), Cleared: true, ClearedAt: days(10),
		},
	}, nil
}

// steppedSource is an activity source whose events follow a script, as the
// sample source's do.
type steppedSource struct {
	*activitySource
}

func (steppedSource) Step() (Event, error) {
	return Event{Kind: EventDeviceChanged, Device: edgeLon01}, nil
}

func (steppedSource) Hold() {}

// activityFragment asks for one fragment of the activity pages, registered
// as this build registers them, over source, and answers its markup.
func activityFragment(t *testing.T, source Source, target string) string {
	t.Helper()

	handler := newFleetHandler(source)
	answer := getFleet(t, http.HandlerFunc(handler.serveFragment), target)
	require.Equal(t, http.StatusOK, answer.Code, "the fragment %s must answer", target)

	return answer.Body.String()
}

// postAction posts form to the action at target and answers the body.
func postAction(t *testing.T, source Source, target string, form url.Values) string {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	recorder := httptest.NewRecorder()
	newActionHandler(source).ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, "the action %s must answer", target)

	return recorder.Body.String()
}

// withFilter answers path asked with the filter q.
func withFilter(path, filter string) string {
	if filter == "" {
		return path
	}

	return path + "?" + url.Values{paramFilter: {filter}}.Encode()
}

// taskRows counts the task and event rows of markup.
func taskRows(markup string) int {
	return strings.Count(markup, `<li class="fleet-task-row">`)
}

// TestTasksTabListsEveryTaskWithItsState asks for the estate's Tasks tab.
//
// VALIDATES: the estate's Tasks tab lists one row per task the hub keeps,
// each with the word of its state.
// PREVENTS: a task in a state the table forgets, and a tab that lists the
// header panel's running tasks only.
func TestTasksTabListsEveryTaskWithItsState(t *testing.T) {
	body := activityFragment(t, newActivitySource(), "/fleet/fragment/content/estate/tasks/")

	assert.Equal(t, len(activityTasks()), taskRows(body))

	// The assisted filter menu above the list names every state too, so the
	// words are looked for in the list alone.
	require.Contains(t, body, `class="fleet-task-list"`)
	list := body[strings.Index(body, `class="fleet-task-list"`):]
	for _, word := range []string{"Running", "Succeeded", "Partly failed", "Failed", "Pending", "Canceled"} {
		assert.Contains(t, list, `<span class="fleet-state-word">`+word+"</span>", "the state word %s", word)
	}
	assert.Contains(t, body, "6 of 6 tasks")
}

// TestTaskPanelCountsEqualTheRows compares each tab of the header task panel
// with the task list filtered to the same states.
//
// VALIDATES: the count on each of the four tabs (Running, Succeeded, Failed,
// All) equals the rows the task list shows for those states.
// PREVENTS: a panel count and a table that disagree about the same tasks.
func TestTaskPanelCountsEqualTheRows(t *testing.T) {
	source := newActivitySource()

	request, err := NewPageRequest(context.Background(), source, testAddress(t, pathEstate), testUser)
	require.NoError(t, err)
	panel, err := taskPanelTab(context.Background(), request, taskTabAll, true)
	require.NoError(t, err)

	filters := map[string]string{
		"Running":   "state:pending,running",
		"Succeeded": "state:succeeded",
		"Failed":    "state:failed,partly-failed",
		"All":       "",
	}
	require.Len(t, panel.Tabs, len(filters))

	for _, tab := range panel.Tabs {
		filter, known := filters[tab.Label]
		require.True(t, known, "the panel tab %s", tab.Label)

		body := activityFragment(t, source, withFilter("/fleet/fragment/content/tasks/", filter))
		assert.Equal(t, tab.Count, taskRows(body), "the %s tab counts the rows of %q", tab.Label, filter)
	}
}

// TestDeviceTasksTabHoldsItsTasksOnly asks for the Tasks tab of one device.
//
// VALIDATES: a device's Tasks tab lists the tasks that targeted it and no
// other.
// PREVENTS: a device tab that shows the estate's tasks.
func TestDeviceTasksTabHoldsItsTasksOnly(t *testing.T) {
	body := activityFragment(t, newActivitySource(), "/fleet/fragment/content/device/"+edgeLon02+"/tasks/")

	assert.Equal(t, 2, taskRows(body))
	assert.Contains(t, body, `href="/fleet/device/`+edgeLon02+`/tasks/?id=t1"`)
	assert.Contains(t, body, `href="/fleet/device/`+edgeLon02+`/tasks/?id=t5"`)
	for _, other := range []string{"t2", "t3", "t4", "t6"} {
		assert.NotContains(t, body, "?id="+other+`"`, "the task %s did not target %s", other, edgeLon02)
	}
}

// TestTaskSidePanelShowsEachDeviceResult opens a partly failed task in the
// side panel of the task list.
//
// VALIDATES: the side panel of a task shows its state and, for each device,
// its result and its answer; a partly failed task offers Retry over its
// failed device only, behind the confirm dialog.
// PREVENTS: a side panel that shows the task without what each device said.
func TestTaskSidePanelShowsEachDeviceResult(t *testing.T) {
	body := activityFragment(t, newActivitySource(), "/fleet/fragment/panel/tasks/?id=t3")

	assert.Contains(t, body, `<span class="fleet-state-word">Partly failed</span>`)
	assert.Contains(t, body, ">"+coreLon01+"</h3>")
	assert.Contains(t, body, ">"+rsAms01+"</h3>")
	assert.Contains(t, body, "configuration applied")
	assert.Contains(t, body, "refused: the configuration was edited on the device")
	assert.Contains(t, body, `data-confirm-action="Retry the task t3"`)
	assert.Contains(t, body, `data-confirm-targets="The failed devices only: `+rsAms01+`"`)
	assert.Contains(t, body, `value="retry t3"`, "the form the Retry control submits")
	assert.NotContains(t, body, `value="cancel t3"`, "a task that ended cannot be canceled")
}

// TestTaskActionsAnswerThePrototypeToast confirms Cancel on a running task
// and Retry on a partly failed one.
//
// VALIDATES: each confirmed action answers the toast saying the prototype
// changed nothing, naming the task, and linking to it.
// PREVENTS: an action of the prototype that answers nothing, or that claims
// it did something.
func TestTaskActionsAnswerThePrototypeToast(t *testing.T) {
	for _, item := range []string{"cancel t1", "retry t3"} {
		verb, id, _ := strings.Cut(item, " ")
		body := postAction(t, newActivitySource(), "/fleet/action/task/tasks/", url.Values{paramItem: {item}})

		assert.Contains(t, body, "fleet-toast--info", item)
		assert.Contains(t, body, changedNothing, item)
		assert.Contains(t, body, "The "+verb+" of ", item)
		assert.Contains(t, body, " "+id+" ", item)
		assert.Contains(t, body, `href="/fleet/tasks/?id=`+id+`"`, item)
	}
}

// TestTaskActionRefusedByState asks to cancel a task that already ended.
//
// VALIDATES: an action the task's state does not allow answers an error
// toast that says why and how to go on.
// PREVENTS: a prototype toast for an action the real hub would refuse.
func TestTaskActionRefusedByState(t *testing.T) {
	body := postAction(t, newActivitySource(), "/fleet/action/task/tasks/", url.Values{paramItem: {"cancel t2"}})

	assert.Contains(t, body, "fleet-toast--error")
	assert.Contains(t, body, "The task t2 cannot be canceled, because its state is Succeeded.")
	assert.NotContains(t, body, changedNothing)
}

// alarmOrder answers the alarm identifiers in the order the eyes of markup
// open them. A NavLink carries its address twice, as href and as hx-get, so
// only the href is read: one identifier per row.
func alarmOrder(markup string) []string {
	matches := regexp.MustCompile(`href="/fleet/estate/alarms/\?id=(a\d+)`).FindAllStringSubmatch(markup, -1)

	order := make([]string, 0, len(matches))
	for _, match := range matches {
		order = append(order, match[1])
	}

	return order
}

// TestAlarmsTabOrdersBySeverityThenAge asks for the estate's Alarms tab.
//
// VALIDATES: the alarms are listed the most severe first and, within a
// severity, the oldest first; the count is the source's open alarms.
// PREVENTS: a table in the source's order, and a count that drops the
// alarms of a page or of the hub.
func TestAlarmsTabOrdersBySeverityThenAge(t *testing.T) {
	body := activityFragment(t, newActivitySource(), "/fleet/fragment/content/estate/alarms/")

	assert.Equal(t, []string{"a3", "a1", "a2"}, alarmOrder(body))
	assert.Contains(t, body, "3 of 3 alarms")
	assert.Contains(t, body, "1 - 3 of 3 alarms")
}

// TestAlarmsDeviceFilterNarrows filters the estate's Alarms tab to one
// device, which is how a device's alarms are listed.
//
// VALIDATES: device:<name> keeps the alarms of that device and no other.
// PREVENTS: a filter field that is registered and matches nothing.
func TestAlarmsDeviceFilterNarrows(t *testing.T) {
	body := activityFragment(t, newActivitySource(),
		withFilter("/fleet/fragment/content/estate/alarms/", "device:"+coreLon01))

	assert.Equal(t, []string{"a1", "a2"}, alarmOrder(body))
	assert.Contains(t, body, "2 of 3 alarms")
}

// TestAlarmSidePanelShowsTheWeek opens an alarm in the side panel.
//
// VALIDATES: the side panel shows the alarm's history on its device over the
// last week, leaves out what cleared before it, and links to the device tab
// registered for the alarm's kind.
// PREVENTS: a history of the whole retention period under a title that says
// a week, and a link to a tab where the condition is not shown.
func TestAlarmSidePanelShowsTheWeek(t *testing.T) {
	body := activityFragment(t, newActivitySource(), "/fleet/fragment/panel/estate/alarms/?id=a1")

	assert.Contains(t, body, "History on "+coreLon01+" over the last week")
	assert.Contains(t, body, "memory above 90%, cleared after 1 day")
	assert.Contains(t, body, "bgp is down, still open")
	assert.NotContains(t, body, "an old rejection")
	assert.Contains(t, body, `href="/fleet/device/`+coreLon01+`/system/"`)
}

// galleryTitles answers the title of every group the gallery draws.
func galleryTitles(t *testing.T) []string {
	t.Helper()

	gallery := componentMarkup(t, ComponentGallery(NewGalleryView()))
	matches := regexp.MustCompile(`<h2 class="fleet-gallery-title">([^<]+)</h2>`).FindAllStringSubmatch(gallery, -1)
	require.NotEmpty(t, matches, "the gallery draws its groups under titles")

	titles := make([]string, 0, len(matches))
	for _, match := range matches {
		titles = append(titles, match[1])
	}

	return titles
}

// TestStatesPageRendersEveryGallerySection asks for the states page over a
// source that follows a script.
//
// VALIDATES: the states page draws every group of the gallery under its
// title, the two script controls posting to the prototype routes, and one
// toast control per level.
// PREVENTS: a states page that leaves out a component's states, or whose
// controls post nowhere.
func TestStatesPageRendersEveryGallerySection(t *testing.T) {
	body := activityFragment(t, steppedSource{newActivitySource()}, "/fleet/fragment/content/admin/states/")

	for _, title := range galleryTitles(t) {
		assert.Contains(t, body, `<h2 class="fleet-gallery-title">`+title+`</h2>`)
	}
	assert.Contains(t, body, `hx-post="`+stepPath+`"`)
	assert.Contains(t, body, `hx-post="`+holdPath+`"`)
	assert.Contains(t, body, "Advance the sample script by one event")
	assert.Contains(t, body, "Hold the sample script")
	for _, level := range toastDemoLevels {
		assert.Contains(t, body, "/fleet/fragment/"+fragmentToastDemo+"/admin/states/?item="+level.Key())
	}
}

// TestStatesPageWithoutScript asks for the states page over a source that
// follows no script.
//
// VALIDATES: the script controls are replaced by a sentence saying why.
// PREVENTS: controls that post to routes a source without a script never
// mounts.
func TestStatesPageWithoutScript(t *testing.T) {
	body := activityFragment(t, newActivitySource(), "/fleet/fragment/content/admin/states/")

	assert.NotContains(t, body, `hx-post="`+stepPath+`"`)
	assert.Contains(t, body, "This data source follows no script")
}

// TestToastDemoAnswersEachLevel asks the toast demonstration for each level,
// then for one that does not exist.
//
// VALIDATES: each level answers its toast, with the lifetime of its level;
// an unknown level answers an error toast naming the levels.
// PREVENTS: a demonstration that shows one level for all four.
func TestToastDemoAnswersEachLevel(t *testing.T) {
	for _, level := range toastDemoLevels {
		body := activityFragment(t, newActivitySource(),
			"/fleet/fragment/"+fragmentToastDemo+"/admin/states/?item="+level.Key())

		assert.Contains(t, body, "fleet-toast--"+level.Key())
		assert.Contains(t, body, `data-toast-lifetime="`+textLifetime(level)+`"`)
	}

	body := activityFragment(t, newActivitySource(), "/fleet/fragment/"+fragmentToastDemo+"/admin/states/?item=loud")
	assert.Contains(t, body, "fleet-toast--error")
	assert.Contains(t, body, "ask for one of: info, success, warning, error")
}

// textLifetime answers the lifetime of a level as the toast's attribute
// writes it.
func textLifetime(level ToastLevel) string {
	return map[ToastLevel]string{ToastInformational: "6", ToastSuccess: "6", ToastWarning: "0", ToastError: "0"}[level]
}
