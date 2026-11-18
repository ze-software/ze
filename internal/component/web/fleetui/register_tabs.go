// Design: docs/architecture/fleet-interface.md -- the tabs the fleet interface registers itself
// Related: tabs.go -- the registry these tabs join
//
// These are the tabs and the administration pages the fleet interface fixes
// for every hub. A page family that adds another registers it from its own
// register file.

package fleetui

func init() {
	for _, tab := range []Tab{
		{Name: TabDashboard, Title: "Dashboard"},
		{Name: "devices", Title: "Devices"},
		{Name: "alarms", Title: "Alarms"},
		{Name: "rollouts", Title: "Rollouts"},
		{Name: "tasks", Title: "Tasks"},
	} {
		MustRegisterTab(ObjectEstate, tab)
	}

	for _, tab := range []Tab{
		{Name: TabDashboard, Title: "Dashboard"},
		{Name: "devices", Title: "Devices"},
		{Name: "alarms", Title: "Alarms"},
		{Name: "tasks", Title: "Tasks"},
	} {
		MustRegisterTab(ObjectGroup, tab)
	}

	for _, tab := range []Tab{
		{Name: TabDashboard, Title: "Dashboard"},
		{Name: "console", Title: "Console"},
		{Name: "system", Title: "System"},
		{Name: "interfaces", Title: "Interfaces"},
		{Name: "routing", Title: "Routing"},
		{Name: "config", Title: "Config"},
		{Name: "logs", Title: "Logs"},
		{Name: "tasks", Title: "Tasks"},
	} {
		MustRegisterTab(ObjectDevice, tab)
	}

	for _, tab := range []Tab{
		{Name: "labels", Title: "Labels"},
		{Name: "tree-views", Title: "Tree views"},
		{Name: "filters", Title: "Saved filters"},
		{Name: "access", Title: "Access"},
		{Name: "audit", Title: "Audit"},
		{Name: "settings", Title: "Settings"},
	} {
		MustRegisterTab(ObjectAdmin, tab)
	}
}
