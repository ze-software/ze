// Design: docs/guide/monitoring.md -- original native cleanup warning recovery
//go:build linux

package l2tp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// TestKernelCleanupWarningClearsAfterExactDeleteRetry observes real worker
// cleanup under its syscall seam. Only the same original session/tunnel's
// confirmed delete may clear the warning that its failed delete produced.
func TestKernelCleanupWarningClearsAfterExactDeleteRetry(t *testing.T) {
	for _, resource := range []string{"session", "tunnel"} {
		t.Run(resource, func(t *testing.T) {
			report.ResetForTest()
			t.Cleanup(report.ResetForTest)
			worker, _, event := workerCleanupBaseline(t)
			birth, err := subscriberid.ParseL2TPBirth("l2tp-00000000000000000000000000000001")
			require.NoError(t, err)
			event.session.birth = birth
			failure := errors.New("original Linux delete remains unconfirmed")
			fail := true
			if resource == "session" {
				worker.ops.sessionDelete = func(uint16, uint16) error {
					if fail {
						return failure
					}
					return nil
				}
			} else {
				worker.ops.tunnelDelete = func(uint16) error {
					if fail {
						return failure
					}
					return nil
				}
			}
			t.Cleanup(func() { fail = false; worker.teardownAll() })
			worker.setupSession(event)
			worker.teardownAll()
			warnings := report.Warnings()
			require.Len(t, warnings, 1, "the original cleanup failure must reach runtime doctor")
			warning := warnings[0]
			require.Equal(t, ppp.CleanupReportSource, warning.Source)
			require.Equal(t, diagnostic.CodeDoctorPPPCleanup, warning.Code)
			require.Contains(t, warning.Message, "unconfirmed")
			require.Equal(t, "ze show warnings", warning.Fix)
			require.Equal(t, failure.Error(), warning.Detail["error"])
			require.Equal(t, event.localTID, warning.Detail["tunnel-id"])
			require.Equal(t, birth.String(), warning.Detail["birth"])
			if resource == "session" {
				require.Equal(t, event.localSID, warning.Detail["session-id"])
			}
			fail = false
			worker.teardownAll()
			require.Empty(t, worker.sessions)
			require.Empty(t, worker.tunnels)
			require.Empty(t, report.Warnings(), "only confirmed cleanup of this original may clear its delete warning")
		})
	}
}

// TestKernelCleanupWarningPreservesAnotherOriginalCloseUncertainty lets a new
// sibling receive descriptor numbers consumed by an earlier failed Close.
// Successful sibling cleanup cannot certify that old physical Close outcome.
func TestKernelCleanupWarningPreservesAnotherOriginalCloseUncertainty(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	worker, _, original := workerCleanupBaseline(t)
	originalBirth, err := subscriberid.ParseL2TPBirth("l2tp-00000000000000000000000000000001")
	require.NoError(t, err)
	original.session.birth = originalBirth
	failure := errors.New("original descriptor close remains unconfirmed")
	closeFails := true
	closes := map[int]int{}
	worker.ops.closeFD = func(fd int) error {
		closes[fd]++
		if closeFails {
			if fd == 11 {
				return failure
			}
		}
		return nil
	}
	worker.setupSession(original)
	worker.teardownAll()
	require.Len(t, report.Warnings(), 1)
	closeFails = false
	sibling := setupEvent(original.localTID, original.localSID+1)
	siblingBirth, err := subscriberid.ParseL2TPBirth("l2tp-00000000000000000000000000000002")
	require.NoError(t, err)
	sibling.session.birth = siblingBirth
	deleteFails := true
	worker.ops.sessionDelete = func(_ uint16, sid uint16) error {
		if sid == sibling.localSID {
			if deleteFails {
				return errors.New("sibling delete remains unconfirmed")
			}
		}
		return nil
	}
	worker.setupSession(sibling)
	worker.teardownAll()
	require.Len(t, report.Warnings(), 2)
	deleteFails = false
	worker.teardownAll()
	warnings := report.Warnings()
	require.Len(t, warnings, 1)
	require.Equal(t, originalBirth.String(), warnings[0].Detail["birth"])
	require.Equal(t, failure.Error(), warnings[0].Detail["error"])
	require.NotNil(t, worker.sessions[sessionKey{original.localTID, original.localSID}])
	require.Nil(t, worker.sessions[sessionKey{sibling.localTID, sibling.localSID}])
	require.Equal(t, 2, closes[11], "the sibling owns the reused number; the original raw Close is never retried")
	// Both admissions' descriptors are syscall recorder tokens. The retained old
	// row is the unknown outcome under test, not an unclosed OS resource.
}
