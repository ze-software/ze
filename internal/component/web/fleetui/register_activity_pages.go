// Design: docs/architecture/fleet-interface.md -- the pages of the activity family: tasks, alarms, the states page
// Related: handler_tasks.go -- the task tabs, the task list, the task side panel and the task actions
// Related: handler_alarms.go -- the alarm tabs and the alarm side panel
// Related: handler_prototype.go -- the states page and its toast demonstrations
//
// The activity family registers its pages, its fragments, its filter fields,
// its sort columns and the device tab each alarm kind is examined on here. A
// later feature that adds an alarm kind or a task field registers it from its
// own register file.

package fleetui

func init() {
	registerTaskFields()
	registerAlarmFields()
	registerAlarmColumns()
	registerAlarmTabs()

	for _, kind := range []ObjectKind{ObjectEstate, ObjectGroup, ObjectDevice} {
		MustRegisterPage(Page{Kind: kind, Tab: activityTabTasks, Content: tasksContent, SidePanel: taskSidePanel})
	}
	MustRegisterPage(Page{Kind: ObjectTasks, Content: tasksContent, SidePanel: taskSidePanel})

	for _, kind := range []ObjectKind{ObjectEstate, ObjectGroup} {
		MustRegisterPage(Page{Kind: kind, Tab: activityTabAlarms, Content: alarmsContent, SidePanel: alarmSidePanel})
	}

	MustRegisterPage(Page{Kind: ObjectAdmin, Tab: activityTabStates, Content: statesContent})

	MustRegisterAction(taskActionName, taskActionAnswer)
	MustRegisterFragment(fragmentToastDemo, toastDemoFragment)
}

// registerAlarmTabs registers the device tab on which the condition of each
// alarm kind the hub derives can be examined.
func registerAlarmTabs() {
	mustRegisterAlarmTab("device-offline", TabDashboard)
	mustRegisterAlarmTab("component-down", tabDeviceSystem)
	mustRegisterAlarmTab("component-degraded", tabDeviceSystem)
	mustRegisterAlarmTab("config-diverged", activityTabConfig)
	mustRegisterAlarmTab("config-rejected", activityTabConfig)
	mustRegisterAlarmTab("resource-high", TabDashboard)
	mustRegisterAlarmTab("software-behind", tabDeviceSystem)
	mustRegisterAlarmTab("relay-not-granted", tabDeviceSystem)
	mustRegisterAlarmTab("history-full", TabDashboard)
}
