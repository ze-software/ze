// Design: docs/architecture/fleet-interface.md -- what the devices table and the tree register
// Related: filter.go -- the filter grammar these fields join
// Related: table.go -- the column set these columns join
// Related: tree.go -- the tree levels these levels join
//
// The devices table registers its filter fields, its filter flags and its
// sortable columns here, and the tree registers the levels that read a field
// of the device. A later feature that adds a field, a column or a level
// registers it from its own register file.

package fleetui

import (
	"cmp"
	"strings"
)

// devicesSubject names the rows of the devices table in a refusal.
const devicesSubject = "devices"

// The names of the device fields, as a filter term, a sort parameter and a
// tree view spell them.
const (
	fieldName     = "name"
	fieldState    = "state"
	fieldVersion  = "version"
	fieldConfig   = "config"
	fieldGroup    = "group"
	fieldCPU      = "cpu"
	fieldMemory   = "memory"
	fieldUptime   = "uptime"
	fieldLastSeen = "last-seen"
	fieldModel    = "model"
	fieldAlarms   = "alarms"
)

// The names of the device flags a has: term takes.
const (
	flagAlarm = "alarm"
	flagTask  = "task"
)

// The filter grammar and the sortable columns of the devices table. init()
// fills both, and nothing removes from either.
var (
	deviceFilters = NewFilterSchema[DeviceView](devicesSubject, (*DeviceView).contains, (*DeviceView).Label)
	deviceColumns = NewColumnSet[DeviceView](devicesSubject)
)

// DeviceFilters answers the filter grammar of the devices table.
func DeviceFilters() *FilterSchema[DeviceView] {
	return deviceFilters
}

// DeviceColumns answers the sortable columns of the devices table. The first
// is the device's name, the table's default sort.
func DeviceColumns() *ColumnSet[DeviceView] {
	return deviceColumns
}

func init() {
	registerDeviceFields()
	registerDeviceColumns()
	registerDeviceLevels()
}

// registerDeviceFields registers the fields and the flags a devices filter
// may name.
func registerDeviceFields() {
	deviceFilters.MustRegisterField(TextField(fieldName, nil, deviceName))
	deviceFilters.MustRegisterField(TextField(fieldState, displayStateKeys[1:], deviceStateKey))
	deviceFilters.MustRegisterField(TextField(fieldVersion, nil, deviceVersion))
	deviceFilters.MustRegisterField(TextField(fieldConfig, configStateKeys[1:], func(view *DeviceView) string {
		return view.Device.Config.State.Key()
	}))
	deviceFilters.MustRegisterField(TextField(fieldGroup, nil, func(view *DeviceView) string {
		return view.Device.Group
	}))
	deviceFilters.MustRegisterField(NumberField(fieldCPU, deviceCPU))
	deviceFilters.MustRegisterField(NumberField(fieldMemory, deviceMemory))
	deviceFilters.MustRegisterField(DurationField(fieldUptime, func(view *DeviceView) (float64, bool) {
		return view.Uptime.Seconds(), view.UptimeKnown
	}))
	deviceFilters.MustRegisterField(DurationField(fieldLastSeen, func(view *DeviceView) (float64, bool) {
		return view.SeenAgo.Seconds(), view.SeenKnown
	}))
	deviceFilters.MustRegisterField(TextField(fieldModel, nil, func(view *DeviceView) string {
		return view.Device.Inventory.Model
	}))

	deviceFilters.MustRegisterFlag(flagAlarm, func(view *DeviceView) bool { return view.AlarmsOpen > 0 })
	deviceFilters.MustRegisterFlag(flagTask, func(view *DeviceView) bool { return view.TasksActive > 0 })
}

// registerDeviceColumns registers the columns the devices table sorts by.
// The name comes first, so it is the default sort.
func registerDeviceColumns() {
	deviceColumns.MustRegister(Column[DeviceView]{Key: fieldName, Compare: func(a, b *DeviceView) int {
		return strings.Compare(deviceName(a), deviceName(b))
	}})
	deviceColumns.MustRegister(Column[DeviceView]{Key: fieldState, Compare: func(a, b *DeviceView) int {
		return cmp.Compare(a.State.Severity(), b.State.Severity())
	}})
	deviceColumns.MustRegister(Column[DeviceView]{Key: fieldVersion, Compare: func(a, b *DeviceView) int {
		return compareVersions(deviceVersion(a), deviceVersion(b))
	}})
	deviceColumns.MustRegister(Column[DeviceView]{Key: fieldConfig, Compare: func(a, b *DeviceView) int {
		return cmp.Compare(a.Device.Config.State, b.Device.Config.State)
	}})
	deviceColumns.MustRegister(Column[DeviceView]{Key: fieldUptime, Compare: func(a, b *DeviceView) int {
		return compareReported(a.Uptime, a.UptimeKnown, b.Uptime, b.UptimeKnown)
	}})
	deviceColumns.MustRegister(Column[DeviceView]{Key: fieldLastSeen, Compare: func(a, b *DeviceView) int {
		return compareReported(a.SeenAgo, a.SeenKnown, b.SeenAgo, b.SeenKnown)
	}})
	deviceColumns.MustRegister(Column[DeviceView]{Key: fieldCPU, Compare: func(a, b *DeviceView) int {
		return compareSampled(a, b, deviceCPU)
	}})
	deviceColumns.MustRegister(Column[DeviceView]{Key: fieldMemory, Compare: func(a, b *DeviceView) int {
		return compareSampled(a, b, deviceMemory)
	}})
	deviceColumns.MustRegister(Column[DeviceView]{Key: fieldAlarms, Compare: func(a, b *DeviceView) int {
		return cmp.Compare(a.AlarmsOpen, b.AlarmsOpen)
	}})
}

// registerDeviceLevels registers the tree levels that read a field of the
// device: its software version, its display state and its configuration
// group.
func registerDeviceLevels() {
	MustRegisterTreeLevel(TreeLevel{
		Key: fieldVersion,
		Value: func(view *DeviceView) (string, bool) {
			version := deviceVersion(view)
			return version, version != ""
		},
		// The newest version comes first.
		Compare: func(a, b string) int { return compareVersions(b, a) },
	})
	MustRegisterTreeLevel(TreeLevel{
		Key:   fieldState,
		Value: func(view *DeviceView) (string, bool) { return deviceStateKey(view), true },
		// The worst state comes first.
		Compare: func(a, b string) int { return cmp.Compare(stateKeySeverity(b), stateKeySeverity(a)) },
		Title:   stateKeyWord,
	})
	MustRegisterTreeLevel(TreeLevel{
		Key: fieldGroup,
		Value: func(view *DeviceView) (string, bool) {
			return view.Device.Group, view.Device.Group != ""
		},
	})
}

// deviceName answers the device's name.
func deviceName(view *DeviceView) string {
	return view.Device.Name
}

// deviceStateKey answers the key of the device's display state.
func deviceStateKey(view *DeviceView) string {
	return view.State.Key()
}

// deviceVersion answers the software version the device reported, which is
// empty until it reports its inventory.
func deviceVersion(view *DeviceView) string {
	return view.Device.Inventory.SoftwareVersion
}

// deviceCPU answers the processor use of the device's last sample, or false
// when it has reported none.
func deviceCPU(view *DeviceView) (float64, bool) {
	return view.Device.Latest.CPUPercent, view.Device.Latest.Reported
}

// deviceMemory answers the memory use of the device's last sample, or false
// when it has reported none.
func deviceMemory(view *DeviceView) (float64, bool) {
	return view.Device.Latest.MemoryPercent, view.Device.Latest.Reported
}

// stateKeySeverity answers the severity of the display state a key names.
// The state level only ever holds keys of display states, and a key that
// names none ranks lowest.
func stateKeySeverity(key string) int {
	state, _ := ParseDisplayState(key)
	return state.Severity()
}

// stateKeyWord answers the word of the display state a key names.
func stateKeyWord(key string) string {
	state, _ := ParseDisplayState(key)
	return state.String()
}

// compareReported orders two values that may be unknown. An unknown value
// sorts before every known one, and two unknown values are equal.
func compareReported[N cmp.Ordered](a N, aKnown bool, b N, bKnown bool) int {
	switch {
	case aKnown && bKnown:
		return cmp.Compare(a, b)
	case aKnown:
		return 1
	case bKnown:
		return -1
	}

	return 0
}

// compareSampled orders two devices by one figure of their last sample.
func compareSampled(a, b *DeviceView, figure func(view *DeviceView) (float64, bool)) int {
	valueA, knownA := figure(a)
	valueB, knownB := figure(b)

	return compareReported(valueA, knownA, valueB, knownB)
}
