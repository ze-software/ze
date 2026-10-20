package fixture

// register_stop_boot_record.go names the driver that stops a daemon while its
// boot record still holds the store.
//
// Related: stop_boot_record_fixture.go -- the driver
// Related: test/plugin/stop-waits-for-boot-record.ci -- one signal, the stop waits
// Related: test/plugin/stop-boot-record-second-signal.ci -- a second signal forces

func init() {
	Register("hub/stop-during-boot-record", stopDuringBootRecord)
}
