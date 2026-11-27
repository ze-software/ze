// Design: docs/architecture/fleet-alarms.md -- hub-derived fleet alarm and alarm history commands
// Design: docs/architecture/api/commands.md -- show fleet alarm and show fleet alarm history device
// Related: register.go -- init invokes registerAlarmCommands
// Related: cmd_devices.go -- shared unknownFleetDeviceRefusal and completeFleetDevices helpers
// Related: cmd_history.go -- shared pipeOpDevice and requireSinglePipeArg helpers

package managed

import (
	"errors"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	fleetAlarmPath        = "show fleet alarm"
	fleetAlarmHistoryPath = "show fleet alarm history device"

	wireFleetAlarm        = "ze-managed:show-fleet-alarm"
	wireFleetAlarmHistory = "ze-managed:show-fleet-alarm-history"

	pipeOpSeverity = "severity"
	pipeOpKind     = "kind"
	pipeOpHidden   = "hidden"
	pipeOpCursor   = "cursor"
	pipeOpLimit    = "limit"

	alarmSeverityExample = "critical"
	alarmDeviceExample   = "edge-01"
	alarmLimitExample    = "100"
	alarmHiddenTrue      = "true"
	alarmHiddenFalse     = "false"

	colAlarmTotal          = "total"
	colAlarmHasMore        = "has-more"
	colAlarmSnapshotID     = "snapshot-id"
	colAlarmDegraded       = "degraded"
	colAlarmDegradedReason = "degraded-reason"
	colAlarmID             = "id"
	colAlarmScope          = "scope"
	colAlarmSubject        = "subject"
	colAlarmSummary        = "summary"
	colAlarmDetail         = "detail"
	colAlarmHiddenBehind   = "hidden-behind"
)

// fleetAlarmPageDoc is the structured response envelope answered by
// `show fleet alarm`. Its single row set (`alarms`) is always a non-nil slice
// so generic pipe operators act on rows while preserving page metadata.
type fleetAlarmPageDoc struct {
	plugin.DataMarker
	pluginserver.AlarmListPage
}

// fleetAlarmHistoryPageDoc is the structured response envelope answered by
// `show fleet alarm history device <name>`. Its single row set (`events`) is
// always a non-nil slice so generic pipe operators act on rows while preserving
// page metadata.
type fleetAlarmHistoryPageDoc struct {
	plugin.DataMarker
	pluginserver.AlarmHistoryPage
}

func registerAlarmCommands() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: wireFleetAlarm, Handler: handleFleetAlarm},
		pluginserver.RPCRegistration{WireMethod: wireFleetAlarmHistory, Handler: handleFleetAlarmHistory, Complete: completeFleetDevices},
	)

	command.RegisterShape([]string{fleetAlarmPath}, command.ShapeTab)
	command.RegisterShape([]string{fleetAlarmHistoryPath}, command.ShapeTab)

	command.RegisterColumns([]string{fleetAlarmPath},
		command.ColumnOrder{
			colAlarmTotal, "counts", colAlarmHasMore, pipeOpCursor, colAlarmSnapshotID,
			colAlarmDegraded, colAlarmDegradedReason, "alarms",
		},
		command.ColumnOrder{
			colAlarmID, pipeOpKind, pipeOpSeverity, colAlarmScope, pipeOpDevice,
			colAlarmSubject, colAlarmSummary, colAlarmDetail, "since", colAlarmHiddenBehind,
		},
	)
	command.RegisterColumns([]string{fleetAlarmHistoryPath},
		command.ColumnOrder{
			pipeOpDevice, colAlarmTotal, colAlarmHasMore, pipeOpCursor, colAlarmSnapshotID,
			"retained-since", "truncated", colAlarmDegraded, colAlarmDegradedReason, "events",
		},
		command.ColumnOrder{
			colAlarmID, pipeOpKind, pipeOpSeverity, colAlarmScope, pipeOpDevice,
			colAlarmSubject, colAlarmSummary, colAlarmDetail, "opened", "cleared", "cleared-at",
			pipeOpHidden, colAlarmHiddenBehind, "hidden-unknown",
		},
	)

	command.RegisterPipeFilters([]string{fleetAlarmPath},
		command.PipeFilter{Name: pipeOpSeverity, Description: "Filter open alarms by severity: critical, warning, or informational", Argument: command.ArgText},
		command.PipeFilter{Name: pipeOpKind, Description: "Filter open alarms by registered alarm kind", Argument: command.ArgText},
		command.PipeFilter{Name: pipeOpDevice, Description: "Filter open alarms by managed device name", Argument: command.ArgText},
		command.PipeFilter{Name: pipeOpHidden, Description: "Filter open alarms by hidden state: true or false", Argument: command.ArgText},
		command.PipeFilter{Name: pipeOpCursor, Description: "Continue an open-alarm listing from a prior page cursor", Argument: command.ArgText},
		command.PipeFilter{Name: pipeOpLimit, Description: "Limit open alarms returned per page (1 to 1024)", Argument: command.ArgCount},
	)
	command.RegisterPipeFilters([]string{fleetAlarmHistoryPath},
		command.PipeFilter{Name: pipeOpCursor, Description: "Continue alarm history from a prior page cursor", Argument: command.ArgText},
		command.PipeFilter{Name: pipeOpLimit, Description: "Limit history events returned per page (1 to 512)", Argument: command.ArgCount},
	)
}

func handleFleetAlarm(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	query, refusal := parseAlarmListPipeQuery(validated.PipeOperations())
	if refusal != nil {
		return refusal, nil
	}

	scope := pluginserver.ResolveFleetDeviceScope(ctx)
	page, err := pluginserver.ManagedAlarmList(scope, query)
	if err != nil {
		return alarmQueryRefusal(fleetAlarmPath, err), nil
	}
	if page.Alarms == nil {
		page.Alarms = []pluginserver.AlarmOccurrence{}
	}
	return plugin.NewResponse(plugin.StatusDone, fleetAlarmPageDoc{AlarmListPage: page}), nil
}

func handleFleetAlarmHistory(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	name, ok := validated.Value(fleetDeviceNameLeaf)
	if !ok {
		return plugin.OperatorErrorResponse(opererr.New(
			fleetAlarmHistoryPath+": a device name is required",
			"the command reads the retained alarm history of one managed device",
			fleetDevicesPath,
		)), nil
	}

	query, refusal := parseAlarmHistoryPipeQuery(name, validated.PipeOperations())
	if refusal != nil {
		return refusal, nil
	}

	scope := pluginserver.ResolveFleetDeviceScope(ctx)
	if !scope.AllowsDevice(name) {
		return unknownFleetDeviceRefusal(fleetAlarmHistoryPath, name), nil
	}

	queryPath := fleetAlarmHistoryPath + " " + name
	_, devErr := pluginserver.ManagedDeviceByName(name)
	if errors.Is(devErr, pluginserver.ErrManagedNotServing) {
		return alarmQueryRefusal(queryPath, devErr), nil
	}
	unknownDevice := errors.Is(devErr, pluginserver.ErrManagedDeviceUnknown)

	page, err := pluginserver.ManagedAlarmHistory(scope, query)
	if err != nil {
		return alarmQueryRefusal(queryPath, err), nil
	}
	if unknownDevice && page.Total == 0 {
		return unknownFleetDeviceRefusal(fleetAlarmHistoryPath, name), nil
	}
	if page.Events == nil {
		page.Events = []pluginserver.AlarmHistoryEvent{}
	}
	return plugin.NewResponse(plugin.StatusDone, fleetAlarmHistoryPageDoc{AlarmHistoryPage: page}), nil
}

func parseAlarmListPipeQuery(ops []command.PipeOperation) (pluginserver.AlarmListQuery, *plugin.Response) {
	var (
		query    pluginserver.AlarmListQuery
		hasLimit bool
	)
	for _, op := range ops {
		switch op.Name {
		case pipeOpSeverity:
			raw, refusal := requireSinglePipeArg(fleetAlarmPath, pipeOpSeverity, op.Args, alarmSeverityExample)
			if refusal != nil {
				return pluginserver.AlarmListQuery{}, refusal
			}
			if _, err := pluginserver.ParseAlarmSeverity(raw); err != nil {
				return pluginserver.AlarmListQuery{}, plugin.OperatorErrorResponse(opererr.New(
					"unknown alarm severity "+raw,
					"alarm severity must be critical, warning, or informational",
					fleetAlarmPath+" | "+pipeOpSeverity+" "+alarmSeverityExample,
				))
			}
			if query.Severity != "" && query.Severity != raw {
				return pluginserver.AlarmListQuery{}, conflictingAlarmFilterRefusal(
					pipeOpSeverity, query.Severity, raw,
				)
			}
			query.Severity = raw
		case pipeOpKind:
			raw, refusal := requireSinglePipeArg(fleetAlarmPath, pipeOpKind, op.Args, pluginserver.AlarmKindDeviceOffline)
			if refusal != nil {
				return pluginserver.AlarmListQuery{}, refusal
			}
			if _, ok := pluginserver.LookupAlarmKind(raw); !ok {
				return pluginserver.AlarmListQuery{}, plugin.OperatorErrorResponse(opererr.New(
					"unknown alarm kind "+raw,
					"the alarm kind is not registered on this hub",
					fleetAlarmPath+" | "+pipeOpKind+" "+pluginserver.AlarmKindDeviceOffline,
				))
			}
			if query.Kind != "" && query.Kind != raw {
				return pluginserver.AlarmListQuery{}, conflictingAlarmFilterRefusal(
					pipeOpKind, query.Kind, raw,
				)
			}
			query.Kind = raw
		case pipeOpDevice:
			raw, refusal := requireSinglePipeArg(fleetAlarmPath, pipeOpDevice, op.Args, alarmDeviceExample)
			if refusal != nil {
				return pluginserver.AlarmListQuery{}, refusal
			}
			if query.Device != "" && query.Device != raw {
				return pluginserver.AlarmListQuery{}, conflictingAlarmFilterRefusal(
					pipeOpDevice, query.Device, raw,
				)
			}
			query.Device = raw
		case pipeOpHidden:
			raw, refusal := requireSinglePipeArg(fleetAlarmPath, pipeOpHidden, op.Args, alarmHiddenTrue)
			if refusal != nil {
				return pluginserver.AlarmListQuery{}, refusal
			}
			var parsed bool
			switch raw {
			case alarmHiddenTrue:
				parsed = true
			case alarmHiddenFalse:
				parsed = false
			default:
				return pluginserver.AlarmListQuery{}, plugin.OperatorErrorResponse(opererr.New(
					"invalid hidden filter value "+raw,
					"hidden accepts only true (alarms hidden behind a cause) or false (shown alarms)",
					fleetAlarmPath+" | "+pipeOpHidden+" "+alarmHiddenTrue,
				))
			}
			if query.Hidden != nil && *query.Hidden != parsed {
				first := alarmHiddenFalse
				if *query.Hidden {
					first = alarmHiddenTrue
				}
				return pluginserver.AlarmListQuery{}, conflictingAlarmFilterRefusal(
					pipeOpHidden, first, raw,
				)
			}
			query.Hidden = &parsed
		case pipeOpCursor:
			if query.Cursor != "" {
				return pluginserver.AlarmListQuery{}, duplicateAlarmCursorRefusal(fleetAlarmPath)
			}
			raw, refusal := parseAlarmCursorPipeArg(fleetAlarmPath, op.Args)
			if refusal != nil {
				return pluginserver.AlarmListQuery{}, refusal
			}
			query.Cursor = raw
		case pipeOpLimit:
			if hasLimit {
				return pluginserver.AlarmListQuery{}, duplicateAlarmLimitRefusal(fleetAlarmPath)
			}
			limit, refusal := parseAlarmLimitPipeArg(fleetAlarmPath, op.Args, pluginserver.AlarmPageMax)
			if refusal != nil {
				return pluginserver.AlarmListQuery{}, refusal
			}
			hasLimit = true
			query.Limit = limit
		default:
			return pluginserver.AlarmListQuery{}, plugin.OperatorErrorResponse(opererr.New(
				"unknown fleet alarm pipe operation "+op.Name,
				"show fleet alarm supports severity, kind, device, hidden, cursor, and limit filters",
				fleetAlarmPath+" | "+pipeOpSeverity+" "+alarmSeverityExample,
			))
		}
	}
	return query, nil
}

func parseAlarmHistoryPipeQuery(device string, ops []command.PipeOperation) (pluginserver.AlarmHistoryQuery, *plugin.Response) {
	query := pluginserver.AlarmHistoryQuery{Device: device}
	cmdPath := fleetAlarmHistoryPath + " " + device
	var hasLimit bool
	for _, op := range ops {
		switch op.Name {
		case pipeOpCursor:
			if query.Cursor != "" {
				return pluginserver.AlarmHistoryQuery{}, duplicateAlarmCursorRefusal(cmdPath)
			}
			raw, refusal := parseAlarmCursorPipeArg(cmdPath, op.Args)
			if refusal != nil {
				return pluginserver.AlarmHistoryQuery{}, refusal
			}
			query.Cursor = raw
		case pipeOpLimit:
			if hasLimit {
				return pluginserver.AlarmHistoryQuery{}, duplicateAlarmLimitRefusal(cmdPath)
			}
			limit, refusal := parseAlarmLimitPipeArg(cmdPath, op.Args, pluginserver.AlarmHistoryPageMax)
			if refusal != nil {
				return pluginserver.AlarmHistoryQuery{}, refusal
			}
			hasLimit = true
			query.Limit = limit
		default:
			return pluginserver.AlarmHistoryQuery{}, plugin.OperatorErrorResponse(opererr.New(
				"unknown fleet alarm history pipe operation "+op.Name,
				"show fleet alarm history device supports cursor and limit filters",
				cmdPath+" | "+pipeOpLimit+" "+alarmLimitExample,
			))
		}
	}
	return query, nil
}

func parseAlarmCursorPipeArg(cmdPath string, args []string) (string, *plugin.Response) {
	if len(args) != 1 {
		return "", plugin.OperatorErrorResponse(opererr.New(
			"pipe filter cursor requires one argument",
			"provide the continuation cursor returned by a previous page",
			cmdPath,
		))
	}
	val := strings.TrimSpace(args[0])
	if val == "" {
		return "", plugin.OperatorErrorResponse(opererr.New(
			"pipe filter cursor requires a non-empty argument",
			"provide the continuation cursor returned by a previous page",
			cmdPath,
		))
	}
	return val, nil
}

func parseAlarmLimitPipeArg(cmdPath string, args []string, maxLimit int) (int, *plugin.Response) {
	raw, refusal := requireSinglePipeArg(cmdPath, pipeOpLimit, args, alarmLimitExample)
	if refusal != nil {
		return 0, refusal
	}
	n, convErr := strconv.Atoi(raw)
	if convErr != nil {
		return 0, invalidAlarmLimitRefusal(cmdPath, raw, maxLimit)
	}
	if n < 1 {
		return 0, invalidAlarmLimitRefusal(cmdPath, raw, maxLimit)
	}
	if n > maxLimit {
		return 0, invalidAlarmLimitRefusal(cmdPath, raw, maxLimit)
	}
	return n, nil
}

func conflictingAlarmFilterRefusal(filterName, first, second string) *plugin.Response {
	var what, why, fix textbuf.Buffer
	what.Str("conflicting alarm ").Str(filterName).Str(" filters ").Str(first).Str(" and ").Str(second)
	why.Str(fleetAlarmPath).Str(" accepts one ").Str(filterName).Str(" filter")
	fix.Str(fleetAlarmPath).Str(" | ").Str(filterName).Byte(' ').Str(second)
	return plugin.OperatorErrorResponse(opererr.New(what.String(), why.String(), fix.String()))
}

func duplicateAlarmCursorRefusal(cmdPath string) *plugin.Response {
	return plugin.OperatorErrorResponse(opererr.New(
		"cannot specify cursor more than once",
		"an alarm query continues from a single page cursor",
		cmdPath,
	))
}

func duplicateAlarmLimitRefusal(cmdPath string) *plugin.Response {
	return plugin.OperatorErrorResponse(opererr.New(
		"cannot specify limit more than once",
		"an alarm query accepts one page limit",
		cmdPath+" | "+pipeOpLimit+" "+alarmLimitExample,
	))
}

func invalidAlarmLimitRefusal(cmdPath, raw string, maxLimit int) *plugin.Response {
	var why textbuf.Buffer
	why.Str("limit must be an integer between 1 and ").Int(int64(maxLimit))
	return plugin.OperatorErrorResponse(opererr.New(
		"invalid alarm page limit "+raw,
		why.String(),
		cmdPath+" | "+pipeOpLimit+" "+alarmLimitExample,
	))
}

func alarmQueryRefusal(path string, cause error) *plugin.Response {
	if opErr, ok := errors.AsType[*opererr.Error](cause); ok {
		return plugin.OperatorErrorResponse(opErr)
	}
	if errors.Is(cause, pluginserver.ErrManagedNotServing) {
		return plugin.OperatorErrorResponse(opererr.Wrap(cause,
			path+": unable to read fleet alarms",
			"the hub is not serving managed devices",
			exampleShowDoctor,
		))
	}
	return plugin.OperatorErrorResponse(opererr.Wrap(cause,
		path+": unable to read fleet alarms",
		"the hub could not answer the fleet alarm query",
		exampleShowDoctor,
	))
}
