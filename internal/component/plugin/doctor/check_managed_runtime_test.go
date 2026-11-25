// VALIDATES: fleet-1 AC-47 -- hub-side managed doctor checks format
// certificate-refused addresses (with known or absent devices, certificate
// identity, and forgotten-count row) and non-certificate nameless failures
// under doctor-managed-client-rejected.
// PREVENTS: silent certificate rotation failures or misclassified handshake errors.

package doctor

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// VALIDATES: AC-47 -- managed-certificate-refused reports the address, count,
// timestamps, alert, presented certificate, devices last seen from that address
// (or wording when none is recorded), fix, and forgotten-count row.
func TestCheckManagedCertificateRefused(t *testing.T) {
	t1 := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 11, 25, 10, 5, 0, 0, time.UTC)

	record := server.ManagedDoctorRecord{
		CertificateName: "fleet-hub",
		CertRefused: []server.ManagedUnidentifiedFailure{
			{
				Addr: "192.0.2.10", ListenerAddr: "10.0.0.1:1791",
				FirstTime: t1, LastTime: t2, Count: 3, LastError: "remote error: tls: bad certificate",
			},
			{
				Addr: "192.0.2.11", ListenerAddr: "10.0.0.1:1791",
				FirstTime: t1, LastTime: t1, Count: 1, LastError: "remote error: tls: unknown certificate authority",
			},
		},
		DevicesByAddress: map[string][]string{
			"192.0.2.10": {"edge-01", "edge-02"},
		},
		CertRefusedForgotten: 4,
	}

	rows := managedCertRefusedRows(record)
	require.Len(t, rows, 3)

	require.Equal(t, server.CodeDoctorManagedCertificateRefused, rows[0].Code)
	require.Equal(t, diagnostic.SeverityWarning, rows[0].Severity)
	require.Contains(t, rows[0].Message, "192.0.2.10")
	require.Contains(t, rows[0].Message, "certificate fleet-hub")
	require.Contains(t, rows[0].Message, "3 time(s)")
	require.Contains(t, rows[0].Message, t1.Format(time.RFC3339))
	require.Contains(t, rows[0].Message, t2.Format(time.RFC3339))
	require.Contains(t, rows[0].Message, "bad certificate")
	require.Contains(t, rows[0].Message, "edge-01, edge-02")
	require.Contains(t, rows[0].Fix, "ze doctor")
	require.Contains(t, rows[0].Fix, "show fleet devices")

	require.Contains(t, rows[1].Message, "192.0.2.11")
	require.Contains(t, rows[1].Message, "no device is recorded")

	require.Equal(t, server.CodeDoctorManagedCertificateRefused, rows[2].Code)
	require.Contains(t, rows[2].Message, "4 additional")
	require.Contains(t, rows[2].Fix, "show fleet devices")

	// When serving the daemon's issued leaf (CertificateName == ""), the row names it.
	issuedRows := managedCertRefusedRows(server.ManagedDoctorRecord{
		CertificateIssued: true,
		CertRefused: []server.ManagedUnidentifiedFailure{{
			Addr: "192.0.2.12", ListenerAddr: "10.0.0.1:1791",
			FirstTime: t1, LastTime: t1, Count: 1, LastError: "remote error: tls: bad certificate",
		}},
	})
	require.Len(t, issuedRows, 1)
	require.Contains(t, issuedRows[0].Message, "the leaf ze issues")

	// Registration check: managed-certificate-refused is registered in runtime phase.
	var registered bool
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name == managedCertRefusedCheckName {
			registered = true
			require.True(t, slices.Contains(check.Codes, server.CodeDoctorManagedCertificateRefused))
		}
	}
	require.True(t, registered)
	require.NotNil(t, diagnostic.Lookup(server.CodeDoctorManagedCertificateRefused))
}

// VALIDATES: AC-47 -- connections ending without a certificate-rejection TLS
// alert (non-certificate fatal alerts, TLS mismatch, closed TCP) emit
// doctor-managed-client-rejected rows and zero doctor-managed-certificate-refused rows.
func TestCheckManagedClientRejectedUnidentified(t *testing.T) {
	t1 := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	record := server.ManagedDoctorRecord{
		Unidentified: []server.ManagedUnidentifiedFailure{
			{
				Addr: "192.0.2.20", ListenerAddr: "10.0.0.1:1791",
				FirstTime: t1, LastTime: t1, Count: 2, LastError: "read auth request: remote error: tls: handshake failure",
			},
			{
				Addr: "192.0.2.21", ListenerAddr: "10.0.0.1:1791",
				FirstTime: t1, LastTime: t1, Count: 1, LastError: "read auth request: EOF",
			},
		},
		UnidentifiedForgotten: 2,
	}

	certRows := managedCertRefusedRows(record)
	require.Empty(t, certRows)

	rejRows := managedClientRejectedRows(record)
	require.Len(t, rejRows, 3)
	require.Equal(t, server.CodeDoctorManagedClientRejected, rejRows[0].Code)
	require.Equal(t, diagnostic.SeverityWarning, rejRows[0].Severity)
	require.Contains(t, rejRows[0].Message, "192.0.2.20")
	require.Contains(t, rejRows[0].Message, "handshake failure")
	require.Contains(t, rejRows[1].Message, "192.0.2.21")
	require.Contains(t, rejRows[1].Message, "EOF")
	require.Contains(t, rejRows[2].Message, "2 additional")
}
