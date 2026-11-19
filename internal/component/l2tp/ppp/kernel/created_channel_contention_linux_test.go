// Design: docs/architecture/appliance/kernel-profiles.md -- original PPPoX channel capture

//go:build linux

package kernel

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The private-file path can revisit a pre-bind busy result on the same socket
// and file. The diagnostic query and capture each send the original ABI input.
func TestFreshPPPChannelCaptureRetriesOriginalOnBusy(t *testing.T) {
	for _, test := range []struct {
		name     string
		queries  []error
		captures []error
		pauses   int
	}{
		{name: "query", queries: []error{unix.EBUSY, nil}, captures: []error{nil}, pauses: 1},
		{name: "capture", queries: []error{nil}, captures: []error{unix.EBUSY, nil}, pauses: 1},
		{name: "both", queries: []error{unix.EBUSY, unix.EBUSY, nil}, captures: []error{unix.EBUSY, unix.EBUSY, nil}, pauses: 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertFreshPPPChannelCapture(t, test.queries, test.captures, test.pauses, nil)
		})
	}
}

func TestFreshPPPChannelCaptureRefusesTerminalErrors(t *testing.T) {
	for _, cause := range []error{
		unix.EINTR, unix.ENOTTY, unix.EPERM, unix.EXDEV, unix.ENOTCONN,
		fmt.Errorf("indirect refusal: %w", unix.EBUSY),
	} {
		t.Run("query/"+cause.Error(), func(t *testing.T) {
			assertFreshPPPChannelCapture(t, []error{cause}, nil, 0, cause)
		})
		t.Run("capture/"+cause.Error(), func(t *testing.T) {
			assertFreshPPPChannelCapture(t, []error{nil}, []error{cause}, 0, cause)
		})
	}
}

func TestFreshPPPChannelCaptureBoundsBusyAcrossBothOperations(t *testing.T) {
	t.Run("query", func(t *testing.T) {
		assertFreshPPPChannelCapture(t,
			[]error{unix.EBUSY, unix.EBUSY, unix.EBUSY, unix.EBUSY, unix.EBUSY}, nil, 4, unix.EBUSY)
	})
	t.Run("query then capture", func(t *testing.T) {
		assertFreshPPPChannelCapture(t,
			[]error{unix.EBUSY, nil}, []error{unix.EBUSY, unix.EBUSY, unix.EBUSY, unix.EBUSY}, 4, unix.EBUSY)
	})
}

func assertFreshPPPChannelCapture(t *testing.T, queryResults, captureResults []error, wantPauses int, wantErr error) {
	t.Helper()
	const socketFD, fileFD = 73, 9
	queries, captures, pauses := 0, 0, 0
	err := captureFreshPPPChannel(socketFD, fileFD,
		func(fd int, request *pppTransportOwner) error {
			if queries >= len(queryResults) || captures != 0 {
				t.Fatalf("unexpected query after %d queries and %d captures", queries, captures)
			}
			if fd != socketFD || *request != (pppTransportOwner{abi: 1}) {
				t.Fatalf("query used socket %d and input %+v", fd, request)
			}
			result := queryResults[queries]
			queries++
			if result == nil {
				*request = pppTransportOwner{abi: 1, features: 448, channel: 14, unit: -1}
			}
			return result
		},
		func(fd int, request *pppTransportCapture) error {
			if queries != len(queryResults) || captures >= len(captureResults) {
				t.Fatalf("unexpected capture after %d queries and %d captures", queries, captures)
			}
			if fd != socketFD || *request != (pppTransportCapture{abi: 1, fd: fileFD}) {
				t.Fatalf("capture used socket %d and input %+v", fd, request)
			}
			result := captureResults[captures]
			captures++
			return result
		},
		func(delay time.Duration) {
			pauses++
			if delay != 20*time.Millisecond || pauses > wantPauses {
				t.Fatalf("unexpected contention pause %d of %s", pauses, delay)
			}
		})
	if !errors.Is(err, wantErr) {
		t.Fatalf("capture result = %v, want %v", err, wantErr)
	}
	if queries != len(queryResults) || captures != len(captureResults) || pauses != wantPauses {
		t.Fatalf("queries/captures/pauses = %d/%d/%d, want %d/%d/%d",
			queries, captures, pauses, len(queryResults), len(captureResults), wantPauses)
	}
}
