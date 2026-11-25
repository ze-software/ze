// VALIDATES: every device record diagnostic names the device, store error and repair.
// PREVENTS: an operator learning history was lost without the command to repair it.

package doctor

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
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

func TestCheckManagedListenerUnboundRepairs(t *testing.T) {
	rows := managedUnboundRows([]server.ManagedListenerUnbound{
		{Addr: "127.0.0.1:1790", Error: "undefined certificate fleet-hub", Fix: "delete bus hub server central certificate"},
		{Addr: "127.0.0.2:1790", Error: "address in use"},
	})
	require.Len(t, rows, 2)
	require.Equal(t, server.CodeDoctorManagedListenerUnbound, rows[0].Code)
	require.Contains(t, rows[0].Message, "undefined certificate fleet-hub")
	require.Equal(t, "delete bus hub server central certificate", rows[0].Fix)
	require.Contains(t, rows[1].Fix, "free 127.0.0.2:1790")
}

func TestCheckManagedServerStartupFailure(t *testing.T) {
	server.RecordManagedStartupFailure(opererr.Wrap(errors.New("disk read failed"), "cannot load the registry", "the index is unavailable", "ze show data check"))
	t.Cleanup(func() { server.RecordManagedStartupFailure(nil) })
	rows := checkManagedListenerUnbound(diagnostic.DoctorCheckContext{})
	require.Len(t, rows, 1)
	require.Equal(t, server.CodeDoctorManagedServerUnavailable, rows[0].Code)
	require.Contains(t, rows[0].Message, "disk read failed")
	require.Equal(t, "ze show data check", rows[0].Fix)
}
