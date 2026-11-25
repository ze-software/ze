// VALIDATES: every device record diagnostic names the device, store error and repair.
// PREVENTS: an operator learning history was lost without the command to repair it.

package doctor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func TestCheckManagedDeviceRecordUnwritable(t *testing.T) {
	failure := server.ManagedDeviceRecordFailure{Name: "edge-01", Key: "meta/fleet/device/edge-01/record", Error: "disk full"}
	rows := managedRecordUnwritableRows([]server.ManagedDeviceRecordFailure{failure})
	require.Len(t, rows, 1)
	require.Equal(t, server.CodeDoctorManagedDeviceRecordUnwritable, rows[0].Code)
	require.Equal(t, diagnostic.SeverityError, rows[0].Severity)
	require.Contains(t, rows[0].Message, failure.Key)
	require.Contains(t, rows[0].Message, "disk full")
	require.Contains(t, rows[0].Fix, "ze show data check")
	require.Contains(t, rows[0].Fix, "edge-01")
}

func TestCheckManagedDeviceRecordUnreadable(t *testing.T) {
	failure := server.ManagedDeviceRecordFailure{Name: "edge-02", Key: "meta/fleet/device/edge-02/record", Error: "unknown field"}
	rows := managedRecordUnreadableRows([]server.ManagedDeviceRecordFailure{failure})
	require.Len(t, rows, 1)
	require.Equal(t, server.CodeDoctorManagedDeviceRecordUnreadable, rows[0].Code)
	require.Equal(t, diagnostic.SeverityError, rows[0].Severity)
	require.Contains(t, rows[0].Message, failure.Key)
	require.Contains(t, rows[0].Message, failure.Error)
	require.Contains(t, rows[0].Fix, "ze data rm "+failure.Key)
}

func TestCheckManagedDeviceRecordUnwritableUnknownKey(t *testing.T) {
	failure := server.ManagedDeviceRecordFailure{Name: "invented", Key: "meta/fleet/device/invented/record", Error: "read-only store", UnknownDevice: true}
	rows := managedRecordUnwritableRows([]server.ManagedDeviceRecordFailure{failure})
	require.Len(t, rows, 1)
	require.Equal(t, server.CodeDoctorManagedDeviceRecordUnwritable, rows[0].Code)
	require.Contains(t, rows[0].Message, failure.Key)
	require.Contains(t, rows[0].Message, failure.Error)
	require.Contains(t, rows[0].Fix, "ze data rm "+failure.Key)
}
