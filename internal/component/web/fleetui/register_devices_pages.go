// Design: docs/architecture/fleet-interface.md -- registration of the Devices tab and of the action of its selection
// Related: handler_devices.go -- the pages registered here
// Related: handler_devices_action.go -- the action of the selection
// Related: handler_action.go -- the action registry it joins

package fleetui

func init() {
	for _, kind := range []ObjectKind{ObjectEstate, ObjectGroup} {
		MustRegisterPage(Page{Kind: kind, Tab: devicesTab, Content: devicesContent, SidePanel: devicesPanel})
	}

	MustRegisterAction(devicesActionName, devicesActionAnswer)
}
