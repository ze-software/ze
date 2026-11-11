// Design: docs/architecture/fleet-config.md -- what the managed config server tells doctor
// Related: register_external.go -- registers the two runtime checks below
// Related: ../server/managed_doctor.go -- ManagedDoctorSnapshot, the record both checks read

package doctor

import (
	"errors"

	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// managedUnboundCheckName reports a managed listener address that did not bind.
const managedUnboundCheckName = "managed-listener-unbound"

// managedRejectedCheckName reports a managed client the hub last refused.
const managedRejectedCheckName = "managed-client-rejected"

// checkManagedListenerUnbound answers one error per managed listen address the
// daemon could not bind, naming the address and the kernel's error. A
// collision with the plugin acceptor is not among them: the local
// hub-managed-listener check reports it from the config.
func checkManagedListenerUnbound(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	record, err := server.ManagedDoctorSnapshot()
	if errors.Is(err, server.ErrManagedNotServing) {
		// No managed server runs here: no client entry in any server block.
		// No listener was asked for, so none is unbound.
		return nil
	}
	var rows []diagnostic.Diagnostic
	for _, unbound := range record.Unbound {
		var message, fix textbuf.Buffer
		message.Str("managed listener ").Str(unbound.Addr).Str(" did not bind: ").Str(unbound.Error).
			Str(", so managed clients of that server block cannot connect")
		fix.Str("free ").Str(unbound.Addr).Str(" or give the server block another ip and port, then restart ze")
		rows = append(rows, diagnostic.Diagnostic{
			Code:     diagnostic.CodeDoctorManagedListenerUnbound,
			Severity: diagnostic.SeverityError,
			Message:  message.String(),
			Fix:      fix.String(),
		})
	}
	return rows
}

// checkManagedClientRejected answers one warning per managed client whose last
// exchange with the hub was a refusal: a second session under a name already
// connected, or a config the client refused to apply.
func checkManagedClientRejected(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	record, err := server.ManagedDoctorSnapshot()
	if errors.Is(err, server.ErrManagedNotServing) {
		// No managed server runs here, so it refused no client.
		return nil
	}
	var rows []diagnostic.Diagnostic
	for _, rejection := range record.Rejected {
		rows = append(rows, managedRejectedRow(rejection))
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
	case server.ManagedRejectUnspecified:
		message.Str(" was refused for an unrecorded cause: ").Str(rejection.Detail)
		fix.Str("ze show warnings")
	}
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorManagedClientRejected,
		Severity: diagnostic.SeverityWarning,
		Message:  message.String(),
		Fix:      fix.String(),
	}
}
