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

// VALIDATES: fleet-1 AC-27 -- managed-handshake-saturated reports the time,
// count, remote addresses holding slots, and fix, and is registered in the
// runtime phase.
func TestCheckManagedHandshakeSaturated(t *testing.T) {
	require.Empty(t, managedHandshakeSaturatedRows(nil))

	when := time.Date(2026, 11, 25, 11, 30, 0, 0, time.UTC)
	rows := managedHandshakeSaturatedRows(&server.ManagedHandshakeSaturation{
		Time:    when,
		Count:   5,
		Holders: []string{"192.0.2.1", "192.0.2.2"},
	})
	require.Len(t, rows, 1)
	require.Equal(t, server.CodeDoctorManagedHandshakeSaturated, rows[0].Code)
	require.Equal(t, diagnostic.SeverityWarning, rows[0].Severity)
	require.Contains(t, rows[0].Message, when.Format(time.RFC3339))
	require.Contains(t, rows[0].Message, "5 time(s)")
	require.Contains(t, rows[0].Message, "192.0.2.1, 192.0.2.2")
	require.Contains(t, rows[0].Fix, "show fleet devices")

	var registered bool
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name == managedHandshakeSaturatedCheckName {
			registered = true
			require.True(t, slices.Contains(check.Codes, server.CodeDoctorManagedHandshakeSaturated))
		}
	}
	require.True(t, registered)
	require.NotNil(t, diagnostic.Lookup(server.CodeDoctorManagedHandshakeSaturated))
}

// VALIDATES: fleet-ui-5 -- managed-alarm-write reports failed alarm shard
// persistence with timestamps, last error, and fix, and is registered in the
// runtime phase.
// PREVENTS: silent loss of durable fleet alarm history on store write failure.
func TestCheckManagedAlarmWrite(t *testing.T) {
	require.Empty(t, managedAlarmWriteRows(nil))
	require.Empty(t, managedAlarmWriteRows(&server.ManagedAlarmHealth{}))

	failAt := time.Date(2026, 11, 27, 6, 10, 0, 0, time.UTC)
	okAt := time.Date(2026, 11, 27, 6, 5, 0, 0, time.UTC)

	firstFailRows := managedAlarmWriteRows(&server.ManagedAlarmHealth{
		Degraded:        true,
		WriteFailed:     true,
		LastWriteFailAt: failAt,
		LastWriteError:  "store read-only",
	})
	require.Len(t, firstFailRows, 1)
	require.Equal(t, server.CodeDoctorManagedAlarmWriteFailed, firstFailRows[0].Code)
	require.Equal(t, diagnostic.SeverityError, firstFailRows[0].Severity)
	require.Contains(t, firstFailRows[0].Message, failAt.Format(time.RFC3339))
	require.Contains(t, firstFailRows[0].Message, "no alarm shard write has succeeded yet")
	require.Contains(t, firstFailRows[0].Message, "store read-only")
	require.Contains(t, firstFailRows[0].Fix, "ze show data check")
	require.Contains(t, firstFailRows[0].Fix, "ze show fleet alarm")

	laterFailRows := managedAlarmWriteRows(&server.ManagedAlarmHealth{
		Degraded:        true,
		WriteFailed:     true,
		LastWriteOKAt:   okAt,
		LastWriteFailAt: failAt,
		LastWriteError:  "no space left on device",
	})
	require.Len(t, laterFailRows, 1)
	require.Contains(t, laterFailRows[0].Message, okAt.Format(time.RFC3339))
	require.Contains(t, laterFailRows[0].Message, "no space left on device")

	var registered bool
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name == managedAlarmWriteCheckName {
			registered = true
			require.True(t, slices.Contains(check.Codes, server.CodeDoctorManagedAlarmWriteFailed))
		}
	}
	require.True(t, registered)
	require.NotNil(t, diagnostic.Lookup(server.CodeDoctorManagedAlarmWriteFailed))
}

// VALIDATES: fleet-ui-5 -- managed-alarm-corrupt reports skipped corrupt alarm
// shards with count, error detail, and fix, and is registered in the runtime
// phase.
// PREVENTS: corrupt alarm shards being silently dropped on startup load.
func TestCheckManagedAlarmCorrupt(t *testing.T) {
	require.Empty(t, managedAlarmCorruptRows(nil))
	require.Empty(t, managedAlarmCorruptRows(&server.ManagedAlarmHealth{}))

	rows := managedAlarmCorruptRows(&server.ManagedAlarmHealth{
		Degraded:         true,
		CorruptShards:    2,
		LastCorruptError: "unsupported alarm device shard version 99",
	})
	require.Len(t, rows, 1)
	require.Equal(t, server.CodeDoctorManagedAlarmCorrupt, rows[0].Code)
	require.Equal(t, diagnostic.SeverityWarning, rows[0].Severity)
	require.Contains(t, rows[0].Message, "skipped 2 unreadable managed fleet alarm shard(s)")
	require.Contains(t, rows[0].Message, "unsupported alarm device shard version 99")
	require.Contains(t, rows[0].Fix, "ze show fleet alarm")
	require.Contains(t, rows[0].Fix, "ze show warnings")

	var registered bool
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name == managedAlarmCorruptCheckName {
			registered = true
			require.True(t, slices.Contains(check.Codes, server.CodeDoctorManagedAlarmCorrupt))
		}
	}
	require.True(t, registered)
	require.NotNil(t, diagnostic.Lookup(server.CodeDoctorManagedAlarmCorrupt))
}

// VALIDATES: fleet-10 AC-12 -- managed-chain-drift reports drifted clients with
// UTC timestamp, chain hashes, bounded reason, and fix, and is registered in the
// runtime phase.
// PREVENTS: persistent resync failures being invisible to hub ze doctor.
func TestCheckManagedChainDrift(t *testing.T) {
	require.Empty(t, managedChainDriftRows(nil))

	failAt := time.Date(2026, 11, 27, 9, 15, 0, 0, time.UTC)
	rows := managedChainDriftRows([]server.ManagedChainDoctorEntry{{
		Client:        "edge-01",
		Reason:        "diff: base missing; full: confirmed commit window open",
		FailedAt:      failAt,
		ClientChain:   "0123456789abcdef0123456789abcdef01234567",
		ExpectedChain: "fedcba9876543210fedcba9876543210fedcba98",
	}})
	require.Len(t, rows, 1)
	require.Equal(t, server.CodeDoctorManagedChainDrift, rows[0].Code)
	require.Equal(t, diagnostic.SeverityWarning, rows[0].Severity)
	require.Contains(t, rows[0].Message, "edge-01")
	require.Contains(t, rows[0].Message, failAt.Format(time.RFC3339))
	require.Contains(t, rows[0].Message, "0123456789abcdef0123456789abcdef01234567")
	require.Contains(t, rows[0].Message, "fedcba9876543210fedcba9876543210fedcba98")
	require.Contains(t, rows[0].Message, "confirmed commit window open")
	require.Contains(t, rows[0].Fix, "ze show fleet status")
	require.Contains(t, rows[0].Fix, "ze show fleet alarm")

	var registered bool
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name == managedChainDriftCheckName {
			registered = true
			require.True(t, slices.Contains(check.Codes, server.CodeDoctorManagedChainDrift))
		}
	}
	require.True(t, registered)
	require.NotNil(t, diagnostic.Lookup(server.CodeDoctorManagedChainDrift))
}

// VALIDATES: fleet-10 AC-12 -- managed-chain-unreadable reports unreadable hub
// chain keys naming the key, error, and ze show data check fix, and is
// registered in the runtime phase.
// PREVENTS: corrupt hub chain keys being silently treated as absent.
func TestCheckManagedChainUnreadable(t *testing.T) {
	require.Empty(t, managedChainUnreadableRows(nil))

	rows := managedChainUnreadableRows([]server.ManagedChainUnreadableEntry{{
		Client: "edge-01",
		Err:    "decode managed chain key meta/managed/client/edge-01/chain: unexpected EOF",
	}})
	require.Len(t, rows, 1)
	require.Equal(t, server.CodeDoctorManagedChainUnreadable, rows[0].Code)
	require.Equal(t, diagnostic.SeverityWarning, rows[0].Severity)
	require.Contains(t, rows[0].Message, "edge-01")
	require.Contains(t, rows[0].Message, "meta/managed/client/edge-01/chain")
	require.Contains(t, rows[0].Message, "unexpected EOF")
	require.Contains(t, rows[0].Fix, "ze show data check")

	var registered bool
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name == managedChainUnreadableCheckName {
			registered = true
			require.True(t, slices.Contains(check.Codes, server.CodeDoctorManagedChainUnreadable))
		}
	}
	require.True(t, registered)
	require.NotNil(t, diagnostic.Lookup(server.CodeDoctorManagedChainUnreadable))
}

// VALIDATES: fleet-3 AC-13 -- managed-audit-write reports failed hub fleet
// audit persistence with count, timestamps, error detail, and store-check fix,
// and is registered in the runtime phase.
// PREVENTS: silent hub audit write failures hidden from ze doctor.
func TestCheckManagedAuditWrite(t *testing.T) {
	require.Empty(t, managedAuditWriteRows(nil))
	require.Empty(t, managedAuditWriteRows(&server.ManagedAuditHealth{WriteFailed: false}))

	failAt := time.Date(2026, 11, 28, 1, 20, 0, 0, time.UTC)
	okAt := time.Date(2026, 11, 28, 1, 10, 0, 0, time.UTC)
	rows := managedAuditWriteRows(&server.ManagedAuditHealth{
		WriteFailed:     true,
		WriteFailCount:  4,
		LastWriteFailAt: failAt,
		LastWriteOKAt:   okAt,
		LastWriteError:  "no space left on device",
	})
	require.Len(t, rows, 1)
	require.Equal(t, server.CodeDoctorManagedAuditWriteFailed, rows[0].Code)
	require.Equal(t, diagnostic.SeverityError, rows[0].Severity)
	require.Contains(t, rows[0].Message, "4 failure(s)")
	require.Contains(t, rows[0].Message, failAt.Format(time.RFC3339))
	require.Contains(t, rows[0].Message, okAt.Format(time.RFC3339))
	require.Contains(t, rows[0].Message, "no space left on device")
	require.Contains(t, rows[0].Fix, "ze show data check")

	var registered bool
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name == managedAuditWriteCheckName {
			registered = true
			require.True(t, slices.Contains(check.Codes, server.CodeDoctorManagedAuditWriteFailed))
		}
	}
	require.True(t, registered)
	require.NotNil(t, diagnostic.Lookup(server.CodeDoctorManagedAuditWriteFailed))
}

// VALIDATES: fleet-3 AC-14, AC-17 -- managed-audit-dropped reports both hub
// admission queue drops and device receipt outbox overflow gaps with counts,
// device identity, timestamps, and fixes, and is registered in runtime phase.
// PREVENTS: silent audit event loss under overload or device receipt overflow.
func TestCheckManagedAuditDropped(t *testing.T) {
	require.Empty(t, managedAuditDroppedRows(nil))
	require.Empty(t, managedAuditDroppedRows(&server.ManagedAuditHealth{}))

	dropAt := time.Date(2026, 11, 28, 2, 0, 0, 0, time.UTC)
	gapAt := time.Date(2026, 11, 28, 2, 5, 0, 0, time.UTC)
	rows := managedAuditDroppedRows(&server.ManagedAuditHealth{
		QueueDropped:         7,
		LastDropAt:           dropAt,
		ReceiptGaps:          2,
		LastReceiptGapAt:     gapAt,
		LastReceiptGapDevice: "edge-01",
	})
	require.Len(t, rows, 2)
	require.Equal(t, server.CodeDoctorManagedAuditDropped, rows[0].Code)
	require.Equal(t, diagnostic.SeverityWarning, rows[0].Severity)
	require.Contains(t, rows[0].Message, "7 event(s)")
	require.Contains(t, rows[0].Message, dropAt.Format(time.RFC3339))
	require.Contains(t, rows[0].Fix, "ze show data check")

	require.Equal(t, server.CodeDoctorManagedAuditDropped, rows[1].Code)
	require.Equal(t, diagnostic.SeverityWarning, rows[1].Severity)
	require.Contains(t, rows[1].Message, "2 device receipt outbox overflow gap(s)")
	require.Contains(t, rows[1].Message, "edge-01")
	require.Contains(t, rows[1].Message, gapAt.Format(time.RFC3339))
	require.Contains(t, rows[1].Fix, "ze show fleet devices")

	var registered bool
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name == managedAuditDroppedCheckName {
			registered = true
			require.True(t, slices.Contains(check.Codes, server.CodeDoctorManagedAuditDropped))
		}
	}
	require.True(t, registered)
	require.NotNil(t, diagnostic.Lookup(server.CodeDoctorManagedAuditDropped))
}

// VALIDATES: fleet-3 AC-17 -- managed-audit-corrupt reports unreadable
// persisted fleet audit keys skipped at startup load with count, timestamp,
// error, and fix, and is registered in the runtime phase.
// PREVENTS: corrupt audit manifest or page keys being silently ignored.
func TestCheckManagedAuditCorrupt(t *testing.T) {
	require.Empty(t, managedAuditCorruptRows(nil))
	require.Empty(t, managedAuditCorruptRows(&server.ManagedAuditHealth{Unreadable: false}))

	at := time.Date(2026, 11, 28, 3, 0, 0, 0, time.UTC)
	rows := managedAuditCorruptRows(&server.ManagedAuditHealth{
		Unreadable:          true,
		UnreadableCount:     2,
		LastUnreadableAt:    at,
		LastUnreadableError: "decode fleet audit page: invalid character",
	})
	require.Len(t, rows, 1)
	require.Equal(t, server.CodeDoctorManagedAuditCorrupt, rows[0].Code)
	require.Equal(t, diagnostic.SeverityWarning, rows[0].Severity)
	require.Contains(t, rows[0].Message, "2 unreadable")
	require.Contains(t, rows[0].Message, at.Format(time.RFC3339))
	require.Contains(t, rows[0].Message, "decode fleet audit page: invalid character")
	require.Contains(t, rows[0].Fix, "ze show data check")

	var registered bool
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name == managedAuditCorruptCheckName {
			registered = true
			require.True(t, slices.Contains(check.Codes, server.CodeDoctorManagedAuditCorrupt))
		}
	}
	require.True(t, registered)
	require.NotNil(t, diagnostic.Lookup(server.CodeDoctorManagedAuditCorrupt))
}
