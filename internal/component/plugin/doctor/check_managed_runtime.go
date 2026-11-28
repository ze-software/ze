// Design: docs/architecture/fleet-config.md -- what the managed config server tells doctor
// Related: register_external.go -- registers the managed runtime checks below
// Related: ../server/managed_doctor.go -- ManagedRuntimeHealth, the record the checks read

package doctor

import (
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// managedUnboundCheckName reports a managed listener address that did not bind.
const managedUnboundCheckName = "managed-listener-unbound"

// managedRejectedCheckName reports a managed client the hub last refused.
const managedRejectedCheckName = "managed-client-rejected"

// managedCertRefusedCheckName reports remote addresses that refused the hub's
// managed TLS certificate.
const managedCertRefusedCheckName = "managed-certificate-refused"

// managedHandshakeSaturatedCheckName reports when the managed listener's
// unauthenticated handshake cap was reached.
const managedHandshakeSaturatedCheckName = "managed-handshake-saturated"

// managedHistoryCheckpointCheckName reports a failed hub metric history
// checkpoint or deletion flush.
const managedHistoryCheckpointCheckName = "managed-history-checkpoint"

// managedHistoryCapacityCheckName reports hub metric history capacity pressure
// that shortened effective retention under a byte cap.
const managedHistoryCapacityCheckName = "managed-history-capacity"

// managedHistoryCorruptCheckName reports persisted fleet metric history shards
// or latest records that failed validation on startup load.
const managedHistoryCorruptCheckName = "managed-history-corrupt"

// managedAlarmWriteCheckName reports failed hub fleet alarm shard persistence.
const managedAlarmWriteCheckName = "managed-alarm-write"

// managedAlarmCorruptCheckName reports persisted fleet alarm shards that failed
// validation on startup load.
const managedAlarmCorruptCheckName = "managed-alarm-corrupt"

// managedChainDriftCheckName reports managed clients whose SHA-1 configuration
// hash chain failed to converge after diff resync and full replay.
const managedChainDriftCheckName = "managed-chain-drift"

// managedChainUnreadableCheckName reports hub-side managed client chain state
// keys that could not be read, decoded, or written.
const managedChainUnreadableCheckName = "managed-chain-unreadable"

// managedAuditWriteCheckName reports failed hub fleet audit log persistence.
const managedAuditWriteCheckName = "managed-audit-write"

// managedAuditDroppedCheckName reports dropped hub fleet audit events or
// device-reported offline receipt overflow gaps.
const managedAuditDroppedCheckName = "managed-audit-dropped"

// managedAuditCorruptCheckName reports unreadable persisted hub fleet audit
// manifest or page keys skipped during startup recovery.
const managedAuditCorruptCheckName = "managed-audit-corrupt"

// checkManagedListenerUnbound answers one error per managed listen address the
// daemon could not bind, naming the address and the kernel's error. A
// collision with the plugin acceptor is not among them: the local
// hub-managed-listener check reports it from the config.
func checkManagedListenerUnbound(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	record, _ := server.ManagedRuntimeHealth()
	if failure := record.StartupFailure; failure != nil {
		message := failure.Problem() + ": " + failure.Why()
		if cause := failure.Unwrap(); cause != nil {
			message += ": " + cause.Error()
		}
		return []diagnostic.Diagnostic{{
			Code: server.CodeDoctorManagedServerUnavailable, Severity: diagnostic.SeverityError,
			Message: message,
			Fix:     failure.Fix(),
		}}
	}
	return managedUnboundRows(record.Unbound)
}

func managedUnboundRows(failures []server.ManagedListenerUnbound) []diagnostic.Diagnostic {
	rows := make([]diagnostic.Diagnostic, 0, len(failures))
	for _, unbound := range failures {
		var message textbuf.Buffer
		message.Str("managed listener ").Str(unbound.Addr).Str(" did not bind: ").Str(unbound.Error).
			Str(", so managed clients of that server block cannot connect")
		fix := unbound.Fix
		if fix == "" {
			fix = "free " + unbound.Addr + " or give the server block another ip and port, then restart ze"
		}
		rows = append(rows, diagnostic.Diagnostic{
			Code:     server.CodeDoctorManagedListenerUnbound,
			Severity: diagnostic.SeverityError,
			Message:  message.String(),
			Fix:      fix,
		})
	}
	return rows
}

// checkManagedClientRejected answers one warning per managed client whose last
// exchange with the hub was a refusal: a second session under a name already
// connected, a config the client refused to apply, a token that is not the
// client's secret, a name the hub has no client entry for, or a connection
// that ended before naming a client without a certificate-rejection TLS alert.
func checkManagedClientRejected(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	record, _ := server.ManagedRuntimeHealth()
	return managedClientRejectedRows(record)
}

func managedClientRejectedRows(record server.ManagedDoctorRecord) []diagnostic.Diagnostic {
	var rows []diagnostic.Diagnostic
	for _, rejection := range record.Rejected {
		rows = append(rows, managedRejectedRow(rejection))
	}
	for _, entry := range record.Unidentified {
		rows = append(rows, managedUnidentifiedRow(entry))
	}
	if record.UnidentifiedForgotten > 0 {
		rows = append(rows, diagnostic.Diagnostic{
			Code:     server.CodeDoctorManagedClientRejected,
			Severity: diagnostic.SeverityWarning,
			Message: strconv.Itoa(record.UnidentifiedForgotten) +
				" additional remote address(es) failed before naming a managed client and were forgotten because the table keeps at most 16",
			Fix: "run ze show fleet devices to see every device that is not connected, and inspect the hub warnings with ze show warnings",
		})
	}
	return rows
}

func managedUnidentifiedRow(entry server.ManagedUnidentifiedFailure) diagnostic.Diagnostic {
	var message textbuf.Buffer
	message.Str("a connection from ").Str(entry.Addr).Str(" to managed listener ").Str(entry.ListenerAddr).
		Str(" ended before naming a client (").Str(strconv.Itoa(entry.Count)).Str(" time(s), first ").
		Str(entry.FirstTime.UTC().Format(time.RFC3339)).Str(", last ").Str(entry.LastTime.UTC().Format(time.RFC3339)).
		Str("): ").Str(entry.LastError).
		Str(", so no managed session opened from that address")
	return diagnostic.Diagnostic{
		Code:     server.CodeDoctorManagedClientRejected,
		Severity: diagnostic.SeverityWarning,
		Message:  message.String(),
		Fix:      "check the remote client at " + entry.Addr + " with ze doctor or confirm it speaks TLS 1.3 to the managed listener",
	}
}

// checkManagedCertificateRefused reports remote addresses whose TLS handshake
// ended with a certificate-rejection alert.
func checkManagedCertificateRefused(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	record, _ := server.ManagedRuntimeHealth()
	return managedCertRefusedRows(record)
}

func managedCertRefusedRows(record server.ManagedDoctorRecord) []diagnostic.Diagnostic {
	var rows []diagnostic.Diagnostic
	certDesc := "the leaf ze issues"
	if record.CertificateName != "" {
		certDesc = "certificate " + record.CertificateName
	}
	for _, entry := range record.CertRefused {
		devices := record.DevicesByAddress[entry.Addr]
		devicesDesc := "no device is recorded from this address"
		if len(devices) > 0 {
			devicesDesc = "devices last seen from this address: " + strings.Join(devices, ", ")
		}
		var message textbuf.Buffer
		message.Str("remote address ").Str(entry.Addr).Str(" refused the managed listener's certificate (").
			Str(certDesc).Str(") ").Str(strconv.Itoa(entry.Count)).Str(" time(s) (first ").
			Str(entry.FirstTime.UTC().Format(time.RFC3339)).Str(", last ").Str(entry.LastTime.UTC().Format(time.RFC3339)).
			Str("): ").Str(entry.LastError).Str("; ").Str(devicesDesc).
			Str(", so those devices hold no hub session")
		rows = append(rows, diagnostic.Diagnostic{
			Code:     server.CodeDoctorManagedCertificateRefused,
			Severity: diagnostic.SeverityWarning,
			Message:  message.String(),
			Fix: "run ze doctor on the device at " + entry.Addr +
				" for the verification cause, or have the hub present a certificate its devices verify (bus hub server <block> certificate); " +
				"show fleet devices lists every device that is not connected",
		})
	}
	if record.CertRefusedForgotten > 0 {
		rows = append(rows, diagnostic.Diagnostic{
			Code:     server.CodeDoctorManagedCertificateRefused,
			Severity: diagnostic.SeverityWarning,
			Message: strconv.Itoa(record.CertRefusedForgotten) +
				" additional remote address(es) refused the hub's managed certificate and were forgotten because the table keeps at most 16",
			Fix: "run ze show fleet devices to list every device that is not connected, and check bus hub server <block> certificate",
		})
	}
	return rows
}

// managedRejectedRow is the row for one client's last refusal.
func managedRejectedRow(rejection server.ManagedClientRejection) diagnostic.Diagnostic {
	var message, fix textbuf.Buffer
	message.Str("managed client ").Quoted(rejection.Client)
	switch rejection.Cause {
	case server.ManagedRejectNameConnected:
		message.Str(" was refused a second session while one is connected under that name: ").Str(rejection.Detail)
		fix.Str("stop the second ze connecting as ").Str(rejection.Client).
			Str(", or give it its own client entry in the hub server block")
	case server.ManagedRejectConfigRejected:
		message.Str(" refused to apply the config the hub serves it: ").Str(rejection.Detail)
		fix.Str("correct the hub's client-").Str(rejection.Client).
			Str(".conf until ze config validate accepts it on the client, then write it again")
	case server.ManagedRejectUnknownName:
		message.Str(" is not a client the hub serves, so its connection was refused: ").Str(rejection.Detail)
		fix.Str("add client ").Str(rejection.Client).
			Str(" { secret ...; } to the hub server block, or correct the name the managed client connects as")
	case server.ManagedRejectBadToken:
		message.Str(" sent a token that is not its client secret, so its connection was refused: ").
			Str(rejection.Detail)
		fix.Str("set the managed client's secret to the one in client ").Str(rejection.Client).
			Str(" { secret ...; } of the hub server block")
	case server.ManagedRejectOversize:
		message.Str(" sent a wire line past the managed read maximum, so the hub closed its connection: ").
			Str(rejection.Detail)
		fix.Str("check the managed client ").Str(rejection.Client).
			Str(" with ze doctor and ensure its configuration stays within 1 MiB")
	case server.ManagedRejectUnspecified:
		message.Str(" was refused for an unrecorded cause: ").Str(rejection.Detail)
		fix.Str("ze show warnings")
	}
	return diagnostic.Diagnostic{
		Code:     server.CodeDoctorManagedClientRejected,
		Severity: diagnostic.SeverityWarning,
		Message:  message.String(),
		Fix:      fix.String(),
	}
}

// checkManagedHandshakeSaturated reports when all unauthenticated managed
// handshake slots were occupied at once.
func checkManagedHandshakeSaturated(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	record, _ := server.ManagedRuntimeHealth()
	return managedHandshakeSaturatedRows(record.HandshakeSaturation)
}

func managedHandshakeSaturatedRows(sat *server.ManagedHandshakeSaturation) []diagnostic.Diagnostic {
	if sat == nil {
		return nil
	}
	holders := "none recorded"
	if len(sat.Holders) > 0 {
		holders = strings.Join(sat.Holders, ", ")
	}
	var message textbuf.Buffer
	message.Str("the managed listener's 32-slot unauthenticated handshake cap was reached ").
		Str(strconv.Itoa(sat.Count)).Str(" time(s) (last at ").
		Str(sat.Time.UTC().Format(time.RFC3339)).Str(", remote addresses holding slots: ").
		Str(holders).Str("), so new managed connections had to wait before authenticating")
	return []diagnostic.Diagnostic{{
		Code:     server.CodeDoctorManagedHandshakeSaturated,
		Severity: diagnostic.SeverityWarning,
		Message:  message.String(),
		Fix:      "check firewall rules and the listed addresses for stalled or abusive connections, and run ze show fleet devices",
	}}
}

// checkManagedDeviceRecordUnwritable explains which history a restart could lose.
func checkManagedDeviceRecordUnwritable(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	record, _ := server.ManagedRuntimeHealth()
	return managedRecordUnwritableRows(record.Unwritable)
}

func managedRecordUnwritableRows(failures []server.ManagedDeviceRecordFailure) []diagnostic.Diagnostic {
	var rows []diagnostic.Diagnostic
	for _, failure := range failures {
		if failure.UnknownDevice {
			rows = append(rows, diagnostic.Diagnostic{
				Code: server.CodeDoctorManagedDeviceRecordUnwritable, Severity: diagnostic.SeverityError,
				Message: "the hub could not remove unrecognized device record " + failure.Key + ": " + failure.Error,
				Fix:     "run ze show data check, correct the store error, then delete the key with ze data rm " + failure.Key,
			})
			continue
		}
		rows = append(rows, diagnostic.Diagnostic{
			Code:     server.CodeDoctorManagedDeviceRecordUnwritable,
			Severity: diagnostic.SeverityError,
			Message:  "managed device " + failure.Name + " has unsaved history at " + failure.Key + ": " + failure.Error,
			Fix:      "run ze show data check, correct the store error, then reconnect " + failure.Name + " to write its history",
		})
	}
	return rows
}

// checkManagedDeviceRecordUnreadable reports records retained for correction.
func checkManagedDeviceRecordUnreadable(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	record, _ := server.ManagedRuntimeHealth()
	return managedRecordUnreadableRows(record.Unreadable)
}

func managedRecordUnreadableRows(failures []server.ManagedDeviceRecordFailure) []diagnostic.Diagnostic {
	rows := make([]diagnostic.Diagnostic, 0, len(failures))
	for _, failure := range failures {
		rows = append(rows, diagnostic.Diagnostic{
			Code:     server.CodeDoctorManagedDeviceRecordUnreadable,
			Severity: diagnostic.SeverityError,
			Message:  "managed device " + failure.Name + " has unreadable history at " + failure.Key + ": " + failure.Error,
			Fix:      "reconnect the device to replace its record, or forget its history with ze data rm " + failure.Key,
		})
	}
	return rows
}

// checkManagedHistoryCheckpoint reports a failed hub metric history checkpoint
// or deletion flush together with the age since the last successful checkpoint.
func checkManagedHistoryCheckpoint(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	record, _ := server.ManagedRuntimeHealth()
	return managedHistoryCheckpointRows(record.History)
}

func managedHistoryCheckpointRows(health *server.ManagedHistoryHealth) []diagnostic.Diagnostic {
	if health == nil {
		return nil
	}
	if !health.CheckpointFailed {
		return nil
	}
	var message textbuf.Buffer
	message.Str("managed metric history checkpoint failed")
	if !health.LastCheckpointFailAt.IsZero() {
		message.Str(" at ").Str(health.LastCheckpointFailAt.UTC().Format(time.RFC3339))
	}
	if health.LastCheckpointOKAt.IsZero() {
		message.Str(" (no checkpoint has succeeded yet)")
	} else {
		message.Str(" (last successful checkpoint ").Int(health.LastCheckpointAgeSeconds).
			Str("s ago at ").Str(health.LastCheckpointOKAt.UTC().Format(time.RFC3339)).Str(")")
	}
	if health.LastCheckpointError != "" {
		message.Str(": ").Str(health.LastCheckpointError)
	}
	message.Str("; dirty samples stay in memory for retry while committed shards remain readable")
	return []diagnostic.Diagnostic{{
		Code:     server.CodeDoctorManagedHistoryCheckpointFailed,
		Severity: diagnostic.SeverityError,
		Message:  message.String(),
		Fix:      "run ze show data check and ze show fleet sample storage, then repair the store before the next checkpoint",
	}}
}

// checkManagedHistoryCapacity reports when hub metric history shortened
// effective retention oldest-first under its per-device or hub-wide byte cap.
func checkManagedHistoryCapacity(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	record, _ := server.ManagedRuntimeHealth()
	return managedHistoryCapacityRows(record.History)
}

func managedHistoryCapacityRows(health *server.ManagedHistoryHealth) []diagnostic.Diagnostic {
	if health == nil {
		return nil
	}
	if !health.CapacityPressure {
		return nil
	}
	var message textbuf.Buffer
	message.Str("managed metric history reached its storage cap (used ").Int(health.UsedBytes).
		Str(" of ").Int(health.HubBudgetBytes).Str(" hub bytes, per-device cap ").Int(health.DeviceBudgetBytes).
		Str(" bytes): effective retention shortened to ").Int(health.EffectiveRetentionSeconds).
		Str("s (configured ").Int(int64(health.ConfiguredRetentionDays)).Str(" day(s)); oldest buckets were evicted while recent samples and latest observations remain kept")
	return []diagnostic.Diagnostic{{
		Code:     server.CodeDoctorManagedHistoryCapacityPressure,
		Severity: diagnostic.SeverityWarning,
		Message:  message.String(),
		Fix:      "raise bus hub history device-budget-bytes or hub-budget-bytes, or lower retention-days, and inspect with ze show fleet sample storage",
	}}
}

// checkManagedHistoryCorrupt reports persisted fleet metric history shards or
// latest observations that could not be decoded on startup load.
func checkManagedHistoryCorrupt(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	record, _ := server.ManagedRuntimeHealth()
	return managedHistoryCorruptRows(record.History)
}

func managedHistoryCorruptRows(health *server.ManagedHistoryHealth) []diagnostic.Diagnostic {
	if health == nil {
		return nil
	}
	var rows []diagnostic.Diagnostic
	for _, failure := range health.CorruptRecords {
		rows = append(rows, diagnostic.Diagnostic{
			Code:     server.CodeDoctorManagedHistoryCorrupt,
			Severity: diagnostic.SeverityWarning,
			Message:  "managed device " + failure.Name + " has unreadable metric history at " + failure.Key + ": " + failure.Error + "; healthy shards remain readable",
			Fix:      "inspect with ze show fleet sample storage; the next valid sample report replaces the damaged record",
		})
	}
	return rows
}

// checkManagedAlarmWrite reports a failed hub fleet alarm shard persistence
// attempt while dirty shards remain queued for retry.
func checkManagedAlarmWrite(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	record, _ := server.ManagedRuntimeHealth()
	return managedAlarmWriteRows(record.Alarms)
}

func managedAlarmWriteRows(health *server.ManagedAlarmHealth) []diagnostic.Diagnostic {
	if health == nil {
		return nil
	}
	if !health.WriteFailed {
		return nil
	}
	var message textbuf.Buffer
	message.Str("managed fleet alarm persistence failed")
	if !health.LastWriteFailAt.IsZero() {
		message.Str(" at ").Str(health.LastWriteFailAt.UTC().Format(time.RFC3339))
	}
	if health.LastWriteOKAt.IsZero() {
		message.Str(" (no alarm shard write has succeeded yet)")
	} else {
		message.Str(" (last successful write at ").Str(health.LastWriteOKAt.UTC().Format(time.RFC3339)).Str(")")
	}
	if health.LastWriteError != "" {
		message.Str(": ").Str(health.LastWriteError)
	}
	message.Str("; live alarms remain derived in memory and dirty shards stay queued for retry while durability is degraded")
	return []diagnostic.Diagnostic{{
		Code:     server.CodeDoctorManagedAlarmWriteFailed,
		Severity: diagnostic.SeverityError,
		Message:  message.String(),
		Fix:      "run ze show data check and ze show fleet alarm, then repair the daemon state store before the next alarm flush",
	}}
}

// checkManagedAlarmCorrupt reports persisted fleet alarm shards that failed
// size, version, or schema validation on startup load and were skipped.
func checkManagedAlarmCorrupt(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	record, _ := server.ManagedRuntimeHealth()
	return managedAlarmCorruptRows(record.Alarms)
}

func managedAlarmCorruptRows(health *server.ManagedAlarmHealth) []diagnostic.Diagnostic {
	if health == nil {
		return nil
	}
	if health.CorruptShards <= 0 {
		return nil
	}
	var message textbuf.Buffer
	message.Str("skipped ").Int(int64(health.CorruptShards)).
		Str(" unreadable managed fleet alarm shard(s) on startup load")
	if health.LastCorruptError != "" {
		message.Str(": ").Str(health.LastCorruptError)
	}
	message.Str("; healthy alarm shards remain readable and live alarms continue to be derived")
	return []diagnostic.Diagnostic{{
		Code:     server.CodeDoctorManagedAlarmCorrupt,
		Severity: diagnostic.SeverityWarning,
		Message:  message.String(),
		Fix:      "inspect ze show fleet alarm and ze show warnings; the next alarm state transition replaces the damaged shard",
	}}
}

// checkManagedChainDrift reports managed clients whose configuration hash chain
// failed to converge after diff resync and full replay.
func checkManagedChainDrift(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	record, _ := server.ManagedRuntimeHealth()
	return managedChainDriftRows(record.ChainDrifted)
}

func managedChainDriftRows(entries []server.ManagedChainDoctorEntry) []diagnostic.Diagnostic {
	if len(entries) == 0 {
		return nil
	}
	rows := make([]diagnostic.Diagnostic, 0, len(entries))
	for _, entry := range entries {
		var message textbuf.Buffer
		message.Str("managed client ").Quoted(entry.Client).Str(" configuration hash chain did not converge")
		if !entry.FailedAt.IsZero() {
			message.Str(" at ").Str(entry.FailedAt.UTC().Format(time.RFC3339))
		}
		if entry.ClientChain != "" || entry.ExpectedChain != "" {
			message.Str(" (client-chain=").Str(entry.ClientChain).Str(" expected-chain=").Str(entry.ExpectedChain).Byte(')')
		}
		if entry.Reason != "" {
			message.Str(": ").Str(entry.Reason)
		}
		message.Str("; the device keeps running its previous configuration until resync succeeds")
		var fix textbuf.Buffer
		fix.Str("run ze show fleet status on ").Str(entry.Client).
			Str(" and ze show fleet alarm on the hub, resolve the reported cause, then let the client resync or reconnect")
		rows = append(rows, diagnostic.Diagnostic{
			Code:     server.CodeDoctorManagedChainDrift,
			Severity: diagnostic.SeverityWarning,
			Message:  message.String(),
			Fix:      fix.String(),
		})
	}
	return rows
}

// checkManagedChainUnreadable reports hub-side managed client chain state keys
// that failed read, strict JSON decode, or write.
func checkManagedChainUnreadable(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	record, _ := server.ManagedRuntimeHealth()
	return managedChainUnreadableRows(record.ChainUnreadable)
}

func managedChainUnreadableRows(entries []server.ManagedChainUnreadableEntry) []diagnostic.Diagnostic {
	if len(entries) == 0 {
		return nil
	}
	rows := make([]diagnostic.Diagnostic, 0, len(entries))
	for _, entry := range entries {
		var message, fix textbuf.Buffer
		key := server.ManagedClientChainKey(entry.Client)
		message.Str("managed client ").Quoted(entry.Client).Str(" has unreadable hash chain state at ").
			Str(key).Str(": ").Str(entry.Err).
			Str("; the hub does not treat the key as absent and waits for a confirmed full replay to overwrite it")
		fix.Str("run ze show data check to inspect the state store, then let ").Str(entry.Client).
			Str(" complete a full configuration replay to rewrite ").Str(key)
		rows = append(rows, diagnostic.Diagnostic{
			Code:     server.CodeDoctorManagedChainUnreadable,
			Severity: diagnostic.SeverityWarning,
			Message:  message.String(),
			Fix:      fix.String(),
		})
	}
	return rows
}

// checkManagedAuditWrite reports a failed hub fleet audit log write while
// previously published audit history remains readable.
func checkManagedAuditWrite(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	record, _ := server.ManagedRuntimeHealth()
	return managedAuditWriteRows(record.Audit)
}

func managedAuditWriteRows(health *server.ManagedAuditHealth) []diagnostic.Diagnostic {
	if health == nil {
		return nil
	}
	if !health.WriteFailed {
		return nil
	}
	var message textbuf.Buffer
	message.Str("managed fleet audit log write failed (").Uint(health.WriteFailCount).Str(" failure(s))")
	if !health.LastWriteFailAt.IsZero() {
		message.Str(" at ").Str(health.LastWriteFailAt.UTC().Format(time.RFC3339))
	}
	if health.LastWriteOKAt.IsZero() {
		message.Str(" (no audit write has succeeded yet)")
	} else {
		message.Str(" (last successful write at ").Str(health.LastWriteOKAt.UTC().Format(time.RFC3339)).Str(")")
	}
	if health.LastWriteError != "" {
		message.Str(": ").Str(health.LastWriteError)
	}
	message.Str("; configuration delivery continues while previously published audit rows remain readable")
	return []diagnostic.Diagnostic{{
		Code:     server.CodeDoctorManagedAuditWriteFailed,
		Severity: diagnostic.SeverityError,
		Message:  message.String(),
		Fix:      "run ze show data check and repair the daemon state store before the next fleet audit batch writes",
	}}
}

// checkManagedAuditDropped reports hub fleet audit queue drops and
// device-reported offline audit receipt overflow gaps.
func checkManagedAuditDropped(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	record, _ := server.ManagedRuntimeHealth()
	return managedAuditDroppedRows(record.Audit)
}

func managedAuditDroppedRows(health *server.ManagedAuditHealth) []diagnostic.Diagnostic {
	if health == nil {
		return nil
	}
	var rows []diagnostic.Diagnostic
	if health.QueueDropped > 0 {
		var message textbuf.Buffer
		message.Str("managed fleet audit queue dropped ").Uint(health.QueueDropped).Str(" event(s)")
		if !health.LastDropAt.IsZero() {
			message.Str(" (last drop at ").Str(health.LastDropAt.UTC().Format(time.RFC3339)).Byte(')')
		}
		message.Str("; configuration delivery continued without blocking")
		rows = append(rows, diagnostic.Diagnostic{
			Code:     server.CodeDoctorManagedAuditDropped,
			Severity: diagnostic.SeverityWarning,
			Message:  message.String(),
			Fix:      "inspect hub storage latency with ze show data check and review fleet event volume",
		})
	}
	if health.ReceiptGaps > 0 {
		var message textbuf.Buffer
		message.Str("managed fleet audit observed ").Uint(health.ReceiptGaps).Str(" device receipt outbox overflow gap(s)")
		if health.LastReceiptGapDevice != "" {
			message.Str(" (last device ").Quoted(health.LastReceiptGapDevice)
			if !health.LastReceiptGapAt.IsZero() {
				message.Str(" at ").Str(health.LastReceiptGapAt.UTC().Format(time.RFC3339))
			}
			message.Byte(')')
		} else if !health.LastReceiptGapAt.IsZero() {
			message.Str(" (last at ").Str(health.LastReceiptGapAt.UTC().Format(time.RFC3339)).Byte(')')
		}
		message.Str("; offline mode-change receipts evicted on the device cannot be reconstructed")
		rows = append(rows, diagnostic.Diagnostic{
			Code:     server.CodeDoctorManagedAuditDropped,
			Severity: diagnostic.SeverityWarning,
			Message:  message.String(),
			Fix:      "inspect the affected device's local audit log and connectivity with ze show fleet devices",
		})
	}
	return rows
}

// checkManagedAuditCorrupt reports persisted fleet audit manifest or page keys
// that failed read, size, or JSON validation on startup load.
func checkManagedAuditCorrupt(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	record, _ := server.ManagedRuntimeHealth()
	return managedAuditCorruptRows(record.Audit)
}

func managedAuditCorruptRows(health *server.ManagedAuditHealth) []diagnostic.Diagnostic {
	if health == nil {
		return nil
	}
	if !health.Unreadable {
		return nil
	}
	var message textbuf.Buffer
	message.Str("skipped ").Uint(health.UnreadableCount).Str(" unreadable managed fleet audit key(s) on startup load")
	if !health.LastUnreadableAt.IsZero() {
		message.Str(" at ").Str(health.LastUnreadableAt.UTC().Format(time.RFC3339))
	}
	if health.LastUnreadableError != "" {
		message.Str(": ").Str(health.LastUnreadableError)
	}
	message.Str("; healthy fleet audit pages remain readable")
	return []diagnostic.Diagnostic{{
		Code:     server.CodeDoctorManagedAuditCorrupt,
		Severity: diagnostic.SeverityWarning,
		Message:  message.String(),
		Fix:      "run ze show data check to inspect damaged keys under meta/fleet/audit/",
	}}
}
