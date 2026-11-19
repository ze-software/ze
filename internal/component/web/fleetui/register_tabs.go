// Design: docs/architecture/fleet-interface.md -- the tabs the fleet interface registers itself
// Related: tabs.go -- the registry these tabs join
//
// These are the tabs and the administration pages the fleet interface fixes
// for every hub. Tabs list in registration order, so every tab of the
// interface registers here, in the order the tab row and the Administration
// part of the sidebar show them. A page family registers the pages of these
// tabs from its own register file; a page registration does not need its tab
// registered first.

package fleetui

func init() {
	for _, tab := range []Tab{
		{Name: TabDashboard, Title: "Dashboard"},
		{Name: devicesTab, Title: "Devices"},
		{Name: activityTabAlarms, Title: "Alarms"},
		{Name: tabEstateRollouts, Title: "Rollouts"},
		{Name: activityTabTasks, Title: "Tasks"},
	} {
		MustRegisterTab(ObjectEstate, tab)
	}

	for _, tab := range []Tab{
		{Name: TabDashboard, Title: "Dashboard"},
		{Name: devicesTab, Title: "Devices"},
		{Name: activityTabAlarms, Title: "Alarms"},
		{Name: activityTabTasks, Title: "Tasks"},
	} {
		MustRegisterTab(ObjectGroup, tab)
	}

	for _, tab := range []Tab{
		{Name: TabDashboard, Title: "Dashboard"},
		{Name: tabDeviceConsole, Title: "Console"},
		{Name: tabDeviceSystem, Title: "System"},
		{Name: tabDeviceInterfaces, Title: "Interfaces"},
		{Name: tabDeviceRouting, Title: "Routing"},
		{Name: tabDeviceConfig, Title: "Config"},
		{Name: tabDeviceLogs, Title: "Logs"},
		{Name: activityTabTasks, Title: "Tasks"},
	} {
		MustRegisterTab(ObjectDevice, tab)
	}

	// The administration pages, States last: it is the prototype's own page.
	for _, tab := range []Tab{
		{Name: tabAdminDevices, Title: "Devices"},
		{Name: tabAdminLabels, Title: "Labels"},
		{Name: tabAdminTreeViews, Title: "Tree views"},
		{Name: tabAdminFilters, Title: "Saved filters"},
		{Name: tabAdminTemplates, Title: "Templates"},
		{Name: tabAdminAccess, Title: "Access"},
		{Name: tabAdminAudit, Title: "Audit"},
		{Name: tabAdminHistory, Title: "History"},
		{Name: tabAdminSettings, Title: "Settings"},
		{Name: activityTabStates, Title: "States"},
	} {
		MustRegisterTab(ObjectAdmin, tab)
	}
}
