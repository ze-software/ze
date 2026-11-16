//go:build zetest

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- complete-path failure findings
// Related: recording_zetest.go -- the recording dataplane embedded by this fixture

package dataplane

import (
	"errors"
	"time"
)

// These causes identify the fixture's state, never an observed kernel failure.
const (
	recordingESPFormRetainedCause = "the recording-esp-form-retained test backend " +
		"retains an earlier receiver-open failure"
	recordingESPFormCounterCause = "the recording-esp-form-retained test backend " +
		"keeps counter reads unavailable"
)

// newRecordingESPFormRetainedBackend gives the doctor a retained device failure and
// an independent counter-read warning through every configuration apply. Only the
// engine's applied peer set may change whether the device finding applies.
func newRecordingESPFormRetainedBackend() (Dataplane, error) {
	return &recordingESPFormRetained{
		policies: map[recordedPolicyKey]SPParams{},
		receiverFailure: ESPFormCompletePathFailure{
			Err: errors.New(recordingESPFormRetainedCause),
			At:  time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC),
		},
		counterFailure: errors.New(recordingESPFormCounterCause),
	}, nil
}

// recordingESPFormRetained is a zetest-only recording backend with constant failure
// state. Its zero value is not usable; the factory initializes the policy map and
// both failures. Safe for concurrent use: failure fields are immutable, and the
// embedded recordingDataplane owns the synchronization of its policy state.
type recordingESPFormRetained struct {
	recordingDataplane
	receiverFailure ESPFormCompletePathFailure
	counterFailure  error
}

// ESPFormFastPathState retains the receiver's failure even while no SPI is watched.
// The independent read failure must remain visible when the last peer is removed.
func (d *recordingESPFormRetained) ESPFormFastPathState() (ESPFormFastPathState, error) {
	return ESPFormFastPathState{
		Status:              ESPFormFastPathIdle,
		CompletePathFailure: &d.receiverFailure,
		FirewallReading:     ESPFormFirewallUnheld,
	}, d.counterFailure
}

// Only a Ze defect can register this fixed fixture name twice.
func init() {
	if err := Register("recording-esp-form-retained", newRecordingESPFormRetainedBackend); err != nil {
		panic("BUG: dataplane: recording-esp-form-retained registration failed: " + err.Error())
	}
}
