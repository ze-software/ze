//go:build zetest

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- dataplane backend registry
// Overview: recording_zetest.go -- the backend this registers

package dataplane

// The recording backends register only in a zetest build (recording_zetest.go says
// why), so `ze.test.ike.dataplane=recording` and
// `ze.test.ike.dataplane=recording-esp-form-unavailable` name no backend in a shipped ze.
// Register's only error is a duplicate name (dataplane.go Register), which only a Ze
// defect makes, so a refusal panics, as register_vpp.go's does.
func init() {
	if err := Register("recording-esp-form-unavailable", newRecordingESPFormUnavailableBackend); err != nil {
		panic("BUG: dataplane: recording-esp-form-unavailable registration failed: " + err.Error())
	}
	if err := Register("recording", newRecordingBackend); err != nil {
		panic("BUG: dataplane: recording registration failed: " + err.Error())
	}
}
