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

// The titles of the tabs more than one object shows.
const (
	tabTitleDashboard = "Dashboard"
	tabTitleDevices   = "Devices"
	tabTitleAlarms    = "Alarms"
)

func init() {
	for _, tab := range []Tab{
		{Name: TabDashboard, Title: tabTitleDashboard},
		{Name: devicesTab, Title: tabTitleDevices},
		{Name: activityTabAlarms, Title: tabTitleAlarms},
		{Name: tabEstateRollouts, Title: rolloutsTitle},
		{Name: activityTabTasks, Title: tasksTitle},
	} {
		MustRegisterTab(ObjectEstate, tab)
	}

	for _, tab := range []Tab{
		{Name: TabDashboard, Title: tabTitleDashboard},
		{Name: devicesTab, Title: tabTitleDevices},
		{Name: activityTabAlarms, Title: tabTitleAlarms},
		{Name: activityTabTasks, Title: tasksTitle},
	} {
		MustRegisterTab(ObjectGroup, tab)
	}

	for _, tab := range []Tab{
		{Name: TabDashboard, Title: tabTitleDashboard},
		{Name: tabDeviceConsole, Title: "Console"},
		{Name: tabDeviceSystem, Title: "System"},
		{Name: tabDeviceInterfaces, Title: titleInterfaces},
		{Name: tabDeviceRouting, Title: "Routing"},
		{Name: tabDeviceConfig, Title: "Config"},
		{Name: tabDeviceLogs, Title: "Logs"},
		{Name: activityTabTasks, Title: tasksTitle},
	} {
		MustRegisterTab(ObjectDevice, tab)
	}

	// The administration pages, States last: it is the prototype's own page.
	for _, tab := range []Tab{
		{Name: tabAdminDevices, Title: tabTitleDevices},
		{Name: tabAdminLabels, Title: devicesWordLabels},
		{Name: tabAdminTreeViews, Title: wordTreeViews},
		{Name: tabAdminFilters, Title: wordFilters},
		{Name: tabAdminTemplates, Title: wordTemplates},
		{Name: tabAdminAccess, Title: "Access"},
		{Name: tabAdminAudit, Title: "Audit"},
		{Name: tabAdminHistory, Title: "History"},
		{Name: tabAdminSettings, Title: "Settings"},
		{Name: activityTabStates, Title: "States"},
	} {
		MustRegisterTab(ObjectAdmin, tab)
	}
}
