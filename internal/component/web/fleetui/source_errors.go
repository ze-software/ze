// Design: docs/architecture/fleet-interface.md -- the failures a source reports
// Overview: source.go -- the data source seam
//
// A source that cannot answer returns an operator error. Where the reason is
// one a page shows a state for, the operator error wraps one of the errors
// below, and FailureOf reads it back, so a page picks its state without
// matching text.

package fleetui

import (
	"errors"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The reasons a source cannot answer that a page shows a state for. A source
// wraps one with opererr.Wrap, so errors.Is finds it under the operator error.
var (
	// ErrObjectUnknown means the device, task, rollout or version asked for
	// does not exist.
	ErrObjectUnknown = errors.New("no such object")
	// ErrDeviceOffline means the device holds no managed connection, so it
	// cannot be asked.
	ErrDeviceOffline = errors.New("the device is offline")
	// ErrRelayRefused means the device refused the relayed command. The
	// operator error carries the device's own words.
	ErrRelayRefused = errors.New("the device refused the relayed command")
	// ErrViewUnsupported means the device's software does not answer the
	// command behind the view.
	ErrViewUnsupported = errors.New("the device does not answer this view")
	// ErrDeadline means the device did not answer within the deadline.
	ErrDeadline = errors.New("the device did not answer in time")
	// ErrUnauthorized means the operator's profile does not allow the view.
	ErrUnauthorized = errors.New("the profile does not allow this view")
	// ErrStreamsFull means the hub already serves its maximum of live event
	// subscriptions. The page then works without live updates.
	ErrStreamsFull = errors.New("the maximum of live event streams is open")
	// ErrFeedClosed means the event feed was closed, which a source does when
	// it stops.
	ErrFeedClosed = errors.New("the event feed is closed")
)

// Failure is the state a page shows for an answer a source could not give.
// The zero value is not a failure.
type Failure uint8

const (
	// FailureUnspecified is the invalid zero value.
	FailureUnspecified Failure = iota
	// FailureFailed is every failure with no state of its own: the page says
	// what failed and offers a retry.
	FailureFailed
	// FailureNotFound is the not-found state.
	FailureNotFound
	// FailureOffline is the device-offline state.
	FailureOffline
	// FailureRelayRefused is the relay-refused state.
	FailureRelayRefused
	// FailureUnsupported is the not-supported-by-the-device state.
	FailureUnsupported
	// FailureDeadline is the failed state of a device that did not answer in
	// time.
	FailureDeadline
	// FailureUnauthorized is the unauthorized state.
	FailureUnauthorized
)

// failureWordOffline is the word of the offline failure.
const failureWordOffline = "offline"

var failureWords = []string{
	enumKeyUnspecified, "request failed", "not found", failureWordOffline, "relay refused",
	"not supported", "no answer in time", "unauthorized",
}

// String answers the failure in words, for a log line or a test.
func (failure Failure) String() string {
	return enumText(failureWords, failure)
}

// failureCauses pairs each error of this file that has a state with it.
var failureCauses = []struct {
	cause   error
	failure Failure
}{
	{ErrObjectUnknown, FailureNotFound},
	{ErrDeviceOffline, FailureOffline},
	{ErrRelayRefused, FailureRelayRefused},
	{ErrViewUnsupported, FailureUnsupported},
	{ErrDeadline, FailureDeadline},
	{ErrUnauthorized, FailureUnauthorized},
}

// FailureOf answers the state a page shows for err, which MUST be non-nil.
// An error that wraps none of the errors of this file is FailureFailed.
func FailureOf(err error) Failure {
	for _, pair := range failureCauses {
		if errors.Is(err, pair.cause) {
			return pair.failure
		}
	}

	return FailureFailed
}

// DeviceUnknownError is the refusal for a device name the estate does not
// hold. A source and this package use the one wording.
func DeviceUnknownError(name string) error {
	problem := textbuf.New().Str("The estate holds no device named ").Quoted(name).Str(".").Slice()

	return opererr.Wrap(ErrObjectUnknown, problem,
		"The address or the action names a device this hub does not manage, so there is nothing to show.",
		"open /fleet/estate/devices/")
}
