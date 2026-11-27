// Design: docs/architecture/fleet-alarms.md -- startup registration of built-in hub alarm kinds and doctor codes
// Related: managed_alarms_kinds.go -- registerBuiltinAlarmKinds
// Related: managed_alarms_store.go -- registerAlarmDoctorCodes

package server

func init() {
	registerBuiltinAlarmKinds()
	registerAlarmDoctorCodes()
}
