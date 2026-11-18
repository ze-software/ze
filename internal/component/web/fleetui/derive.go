// Design: docs/architecture/fleet-interface.md -- the device view the pages work on
// Related: source_devices.go -- the facts a view is derived from
// Related: state.go -- DeviceDisplayState
// Related: filter.go, tree.go, table.go, summary.go -- the functions that work on views
//
// A source answers facts in three lists: devices, open alarms and tasks. A
// page needs them joined: the state of each device, how many alarms it has
// open, whether a task targets it, how long it has been up or away. The join
// is made once per request, here, and every tree, table and card of the
// request then reads the same views. That is what keeps a device in the same
// state everywhere on a page, and a count on a card equal to the rows behind
// it.

package fleetui

import (
	"strings"
	"time"
)

// DeviceView is one device with everything derived from its facts at one
// moment. DeriveDevices builds it.
//
// A DeviceView is larger than a function should take by value: pass a
// pointer.
type DeviceView struct {
	Device Device
	// State is the display state, the same wherever the device is shown.
	State DisplayState
	// AlarmsOpen counts the open alarms about the device.
	AlarmsOpen int
	// TasksActive counts the pending and running tasks that target the
	// device.
	TasksActive int
	// UptimeKnown reports whether the device is connected and has reported
	// when its daemon started. Uptime means nothing while it is false.
	UptimeKnown bool
	Uptime      time.Duration
	// SeenKnown reports whether the hub has ever seen the device. SeenAgo
	// means nothing while it is false.
	SeenKnown bool
	// SeenAgo is how long ago the device was last seen: zero for a connected
	// device.
	SeenAgo time.Duration
}

// DeriveDevices joins the three lists a source answers into one view per
// device, in the order of devices, as of now.
func DeriveDevices(devices []Device, alarms []Alarm, tasks []Task, now time.Time) []DeviceView {
	alarmsOpen := make(map[string]int, len(alarms))
	for index := range alarms {
		alarmsOpen[alarms[index].Device]++
	}

	tasksActive := make(map[string]int)
	for index := range tasks {
		task := &tasks[index]
		if !task.State.Active() {
			continue
		}
		for _, name := range task.Devices {
			tasksActive[name]++
		}
	}

	views := make([]DeviceView, len(devices))
	for index := range devices {
		device := &devices[index]
		view := &views[index]

		view.Device = *device
		view.State = DeviceDisplayState(device)
		view.AlarmsOpen = alarmsOpen[device.Name]
		view.TasksActive = tasksActive[device.Name]
		view.Uptime, view.UptimeKnown = deviceUptime(device, now)
		view.SeenAgo, view.SeenKnown = deviceSeenAgo(device, now)
	}

	return views
}

// deviceUptime answers how long the device's daemon has run, or false when
// the device is offline or never reported when it started.
func deviceUptime(device *Device, now time.Time) (time.Duration, bool) {
	if !device.Connection.Connected {
		return 0, false
	}
	if !device.Inventory.Reported {
		return 0, false
	}

	return now.Sub(device.Inventory.StartedAt), true
}

// deviceSeenAgo answers how long ago the device was last seen, which is zero
// for a connected device, or false when the hub never saw it.
func deviceSeenAgo(device *Device, now time.Time) (time.Duration, bool) {
	if device.Connection.Connected {
		return 0, true
	}
	if !device.Connection.EverConnected {
		return 0, false
	}

	return now.Sub(device.Connection.LastSeen), true
}

// Label answers the value of the device's label key, or false when the device
// does not carry it.
func (view *DeviceView) Label(key string) (string, bool) {
	for _, label := range view.Device.Labels {
		if label.Key == key {
			return label.Value, true
		}
	}

	return "", false
}

// contains reports whether the device's name or any of its label values
// holds needle, which MUST be lower case. It is the test of a bare filter
// word and of the tree filter.
func (view *DeviceView) contains(needle string) bool {
	if strings.Contains(strings.ToLower(view.Device.Name), needle) {
		return true
	}
	for _, label := range view.Device.Labels {
		if strings.Contains(strings.ToLower(label.Value), needle) {
			return true
		}
	}

	return false
}

// FindDevice answers the view of the device named name, or
// DeviceUnknownError.
func FindDevice(views []DeviceView, name string) (*DeviceView, error) {
	for index := range views {
		if views[index].Device.Name == name {
			return &views[index], nil
		}
	}

	return nil, DeviceUnknownError(name)
}

// ScopeAlarms answers the alarms about the devices of views, in their order.
// A group page and a device page show these, never the estate's.
func ScopeAlarms(alarms []Alarm, views []DeviceView) []Alarm {
	names := viewNames(views)

	scoped := make([]Alarm, 0, len(alarms))
	for index := range alarms {
		if _, held := names[alarms[index].Device]; held {
			scoped = append(scoped, alarms[index])
		}
	}

	return scoped
}

// ScopeTasks answers the tasks that target at least one device of views, in
// their order.
func ScopeTasks(tasks []Task, views []DeviceView) []Task {
	names := viewNames(views)

	scoped := make([]Task, 0, len(tasks))
	for index := range tasks {
		if targetsAny(&tasks[index], names) {
			scoped = append(scoped, tasks[index])
		}
	}

	return scoped
}

// viewNames answers the set of the device names of views.
func viewNames(views []DeviceView) map[string]struct{} {
	names := make(map[string]struct{}, len(views))
	for index := range views {
		names[views[index].Device.Name] = struct{}{}
	}

	return names
}

// targetsAny reports whether task targets a device of names.
func targetsAny(task *Task, names map[string]struct{}) bool {
	for _, name := range task.Devices {
		if _, held := names[name]; held {
			return true
		}
	}

	return false
}
