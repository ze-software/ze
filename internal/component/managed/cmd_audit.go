// Design: docs/architecture/fleet-config.md -- centralized hub-side fleet audit log query command and pipes
// Design: docs/architecture/api/commands.md -- show fleet audit command contract
// Related: register.go -- init invokes registerAuditCommands
// Related: cmd_history.go -- shared pipeOpDevice, pipeOpUntil, and requireSinglePipeArg helpers
// Related: cmd_alarms.go -- shared alarmDeviceExample constant

package managed

import (
	"errors"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	fleetAuditPath = "show fleet audit"
	wireFleetAudit = "ze-managed:show-fleet-audit"

	pipeOpType  = "type"
	pipeOpSince = "since"

	auditDeviceExample = alarmDeviceExample
	auditTypeExample   = pluginserver.FleetAuditConfigFetch
	auditTimeExample   = "2026-11-28T00:00:00Z"
)

// fleetAuditRow is one structured audit entry returned by `show fleet audit`.
type fleetAuditRow struct {
	Seq             uint64            `json:"seq"`
	EventTime       string            `json:"event-time"`
	ReceiptTime     string            `json:"receipt-time"`
	Type            string            `json:"type"`
	Device          string            `json:"device,omitempty"`
	Actor           string            `json:"actor"`
	Surface         string            `json:"surface"`
	Outcome         string            `json:"outcome"`
	Version         string            `json:"version,omitempty"`
	PreviousVersion string            `json:"previous-version,omitempty"`
	DiagnosticCode  string            `json:"diagnostic-code,omitempty"`
	Summary         string            `json:"summary,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

func registerAuditCommands() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: wireFleetAudit, Handler: handleFleetAudit},
	)

	command.RegisterShape([]string{fleetAuditPath}, command.ShapeTab)
	command.RegisterColumns([]string{fleetAuditPath}, command.ColumnOrder{
		"seq", "event-time", "receipt-time", pipeOpType, pipeOpDevice, "actor", "surface",
		"outcome", "version", "previous-version", "diagnostic-code", "summary", "metadata",
	})
	command.RegisterPipeFilters([]string{fleetAuditPath},
		command.PipeFilter{Name: pipeOpDevice, Description: "Select fleet audit entries for one managed device", Argument: command.ArgText},
		command.PipeFilter{Name: pipeOpType, Description: "Select fleet audit entries by dot-separated event type", Argument: command.ArgText},
		command.PipeFilter{Name: pipeOpSince, Description: "Select fleet audit entries at or after an RFC 3339 timestamp", Argument: command.ArgText},
		command.PipeFilter{Name: pipeOpUntil, Description: "Select fleet audit entries at or before an RFC 3339 timestamp", Argument: command.ArgText},
	)
}

func handleFleetAudit(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	query, refusal := parseAuditPipeQuery(validated.PipeOperations())
	if refusal != nil {
		return refusal, nil
	}

	scope := pluginserver.ResolveFleetDeviceScope(ctx)
	entries, err := pluginserver.ManagedAuditQuery(scope, query)
	if err != nil {
		return auditQueryRefusal(err), nil
	}

	rows := make(plugin.Slice[fleetAuditRow], 0, len(entries))
	for i := range entries {
		rows = append(rows, auditRowFromEntry(&entries[i]))
	}
	return plugin.NewResponse(plugin.StatusDone, rows), nil
}

func auditRowFromEntry(e *pluginserver.FleetAuditEntry) fleetAuditRow {
	return fleetAuditRow{
		Seq:             e.Seq,
		EventTime:       e.EventTime.UTC().Format(time.RFC3339Nano),
		ReceiptTime:     e.ReceiptTime.UTC().Format(time.RFC3339Nano),
		Type:            e.Type,
		Device:          e.Device,
		Actor:           e.Actor,
		Surface:         e.Surface,
		Outcome:         e.Outcome,
		Version:         e.Version,
		PreviousVersion: e.PreviousVersion,
		DiagnosticCode:  e.DiagnosticCode,
		Summary:         e.Summary,
		Metadata:        e.Metadata,
	}
}

func parseAuditPipeQuery(ops []command.PipeOperation) (pluginserver.FleetAuditQuery, *plugin.Response) {
	var query pluginserver.FleetAuditQuery
	for _, op := range ops {
		switch op.Name {
		case pipeOpDevice:
			raw, refusal := requireSinglePipeArg(fleetAuditPath, pipeOpDevice, op.Args, auditDeviceExample)
			if refusal != nil {
				return pluginserver.FleetAuditQuery{}, refusal
			}
			if query.Device != "" {
				if query.Device != raw {
					return pluginserver.FleetAuditQuery{}, conflictingAuditFilterRefusal(pipeOpDevice, query.Device, raw)
				}
			}
			query.Device = raw
		case pipeOpType:
			raw, refusal := requireSinglePipeArg(fleetAuditPath, pipeOpType, op.Args, auditTypeExample)
			if refusal != nil {
				return pluginserver.FleetAuditQuery{}, refusal
			}
			if !pluginserver.ValidFleetAuditEventType(raw) {
				return pluginserver.FleetAuditQuery{}, plugin.OperatorErrorResponse(opererr.New(
					"invalid fleet audit event type "+raw,
					"event type must be a dot-separated kebab-case identifier such as "+auditTypeExample,
					fleetAuditPath+" | "+pipeOpType+" "+auditTypeExample,
				))
			}
			if query.Type != "" {
				if query.Type != raw {
					return pluginserver.FleetAuditQuery{}, conflictingAuditFilterRefusal(pipeOpType, query.Type, raw)
				}
			}
			query.Type = raw
		case pipeOpSince:
			if !query.Since.IsZero() {
				return pluginserver.FleetAuditQuery{}, duplicateAuditTimeRefusal(pipeOpSince, "lower")
			}
			raw, refusal := requireSinglePipeArg(fleetAuditPath, pipeOpSince, op.Args, auditTimeExample)
			if refusal != nil {
				return pluginserver.FleetAuditQuery{}, refusal
			}
			ts, parseErr := parseAuditInstant(pipeOpSince, raw)
			if parseErr != nil {
				return pluginserver.FleetAuditQuery{}, plugin.OperatorErrorResponse(parseErr)
			}
			query.Since = ts
		case pipeOpUntil:
			if !query.Until.IsZero() {
				return pluginserver.FleetAuditQuery{}, duplicateAuditTimeRefusal(pipeOpUntil, "upper")
			}
			raw, refusal := requireSinglePipeArg(fleetAuditPath, pipeOpUntil, op.Args, auditTimeExample)
			if refusal != nil {
				return pluginserver.FleetAuditQuery{}, refusal
			}
			ts, parseErr := parseAuditInstant(pipeOpUntil, raw)
			if parseErr != nil {
				return pluginserver.FleetAuditQuery{}, plugin.OperatorErrorResponse(parseErr)
			}
			query.Until = ts
		default:
			return pluginserver.FleetAuditQuery{}, plugin.OperatorErrorResponse(opererr.New(
				"unknown fleet audit pipe operation "+op.Name,
				"show fleet audit supports device, type, since, and until filters",
				fleetAuditPath+" | "+pipeOpDevice+" "+auditDeviceExample,
			))
		}
	}
	if !query.Since.IsZero() {
		if !query.Until.IsZero() {
			if query.Until.Before(query.Since) {
				return pluginserver.FleetAuditQuery{}, plugin.OperatorErrorResponse(opererr.New(
					"reversed fleet audit time range",
					"the upper time bound (until) must be at or after the lower time bound (since)",
					fleetAuditPath+" | "+pipeOpSince+" "+auditTimeExample,
				))
			}
		}
	}
	return query, nil
}

func parseAuditInstant(opName, raw string) (time.Time, *opererr.Error) {
	ts, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, opererr.Wrap(err,
			"invalid "+opName+" timestamp "+raw,
			opName+" requires an RFC 3339 timestamp such as "+auditTimeExample,
			fleetAuditPath+" | "+opName+" "+auditTimeExample,
		)
	}
	return ts.UTC(), nil
}

func conflictingAuditFilterRefusal(filterName, first, second string) *plugin.Response {
	var what, why, fix textbuf.Buffer
	what.Str("conflicting fleet audit ").Str(filterName).Str(" filters ").Str(first).Str(" and ").Str(second)
	why.Str(fleetAuditPath).Str(" accepts one ").Str(filterName).Str(" filter")
	fix.Str(fleetAuditPath).Str(" | ").Str(filterName).Byte(' ').Str(second)
	return plugin.OperatorErrorResponse(opererr.New(what.String(), why.String(), fix.String()))
}

func duplicateAuditTimeRefusal(opName, bound string) *plugin.Response {
	var what, why, fix textbuf.Buffer
	what.Str("cannot specify ").Str(opName).Str(" more than once")
	why.Str("a fleet audit query accepts one ").Str(bound).Str(" time bound")
	fix.Str(fleetAuditPath).Str(" | ").Str(opName).Byte(' ').Str(auditTimeExample)
	return plugin.OperatorErrorResponse(opererr.New(what.String(), why.String(), fix.String()))
}

func auditQueryRefusal(cause error) *plugin.Response {
	if opErr, ok := errors.AsType[*opererr.Error](cause); ok {
		return plugin.OperatorErrorResponse(opErr)
	}
	if errors.Is(cause, pluginserver.ErrManagedNotServing) {
		return plugin.OperatorErrorResponse(opererr.Wrap(cause,
			fleetAuditPath+": unable to read fleet audit log",
			"the hub is not serving managed devices",
			exampleShowDoctor,
		))
	}
	return plugin.OperatorErrorResponse(opererr.Wrap(cause,
		fleetAuditPath+": unable to read fleet audit log",
		"the hub could not answer the fleet audit query",
		exampleShowDoctor,
	))
}
