//go:build zetest

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- dataplane backend registry
// Overview: recording_zetest.go -- the backend this registers

package dataplane

import (
	"fmt"
	"os"
)

// The recording backend registers only in a zetest build (recording_zetest.go says
// why), so `ze.test.ike.dataplane=recording` names no backend in a shipped ze.
func init() {
	if err := Register("recording", newRecordingBackend); err != nil {
		fmt.Fprintf(os.Stderr, "dataplane: recording registration failed: %v\n", err)
		os.Exit(1)
	}
}
