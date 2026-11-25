// Design: docs/architecture/fleet-config.md -- what the managed config server tells doctor
// Related: register_external.go -- registers the two runtime checks below
// Related: ../server/managed_doctor.go -- ManagedDoctorSnapshot, the record both checks read

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
	var rows []diagnostic.Diagnostic
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
				Message: "the hub could not remove unrecognised device record " + failure.Key + ": " + failure.Error,
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
	var rows []diagnostic.Diagnostic
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
