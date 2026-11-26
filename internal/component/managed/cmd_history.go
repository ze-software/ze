// Design: docs/architecture/fleet-config.md -- hub-side fleet metric history query commands and pipes
// Design: docs/architecture/api/commands.md -- show fleet sample latest, series, and storage
// Related: register.go -- init invokes registerHistoryCommands
// Related: cmd_devices.go -- shared splitFleetLabel helper

package managed

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
)

const (
	fleetSampleLatestPath  = "show fleet sample latest"
	fleetSampleSeriesPath  = "show fleet sample series"
	fleetSampleStoragePath = "show fleet sample storage"

	wireFleetSampleLatest  = "ze-managed:show-fleet-sample-latest"
	wireFleetSampleSeries  = "ze-managed:show-fleet-sample-series"
	wireFleetSampleStorage = "ze-managed:show-fleet-sample-storage"

	pipeOpDevice = "device"
	pipeOpLabel  = "label"
	pipeOpFigure = "figure"
	pipeOpReduce = "reduce"
	pipeOpRank   = "rank"
	pipeOpFrom   = "from"
	pipeOpUntil  = "until"
	pipeOpPoints = "points"

	sampleFigureExample = "cpu-used"
)

// fleetHistoryStorageDoc is the structured document answered by
// `show fleet sample storage`.
type fleetHistoryStorageDoc struct {
	plugin.DataMarker
	pluginserver.HistoryStorageStatus
}

type latestPipeStep struct {
	op  string
	arg string
}

type latestPipePlan struct {
	device           string
	labels           map[string]string
	impossibleDevice bool
	hasReduce        bool
	hasRank          bool
	singleFigure     string
	multipleFigures  bool
	steps            []latestPipeStep
}

type seriesPipePlan struct {
	query      pluginserver.HistorySeriesQuery
	impossible bool
}

func registerHistoryCommands() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: wireFleetSampleLatest, Handler: handleFleetSampleLatest},
		pluginserver.RPCRegistration{WireMethod: wireFleetSampleSeries, Handler: handleFleetSampleSeries},
		pluginserver.RPCRegistration{WireMethod: wireFleetSampleStorage, Handler: handleFleetSampleStorage},
	)

	command.RegisterShape([]string{fleetSampleLatestPath}, command.ShapeTab)
	command.RegisterShape([]string{fleetSampleSeriesPath}, command.ShapeTab)
	command.RegisterShape([]string{fleetSampleStoragePath}, command.ShapeDoc)

	command.RegisterColumns([]string{fleetSampleLatestPath},
		command.ColumnOrder{
			pipeOpDevice, colStatus, connectionUp, "declared", "has-report",
			colReceivedAt, "age-seconds", "device-time", colLabels, "figures", "unsupported",
		},
		command.ColumnOrder{
			"figure", "unit", "aggregation", "value", "ratio", "contributors", "missing",
		},
	)
	command.RegisterColumns([]string{fleetSampleSeriesPath}, command.ColumnOrder{
		pipeOpDevice, "figure", "unit", "resolution", "start", "end", "value", "sum", "max",
		"ratio", "count", "observed-minutes", "total-minutes", "coverage", "gap",
		"contributors", "missing", colLabels,
	})
	command.RegisterColumns([]string{fleetSampleStoragePath},
		command.ColumnOrder{
			"interval-seconds", "configured-retention-days", "effective-retention-seconds",
			"used-bytes", "hub-budget-bytes", "device-budget-bytes", "capacity-pressure",
			"oldest-sample", "device-count", "last-checkpoint-at", "last-checkpoint-age-seconds",
			"checkpoint-failed", "last-checkpoint-error", "corrupt-shards",
			"rejected-reports", "rate-limited-reports", "devices",
		},
		command.ColumnOrder{
			pipeOpDevice, "used-bytes", "latest-bytes", "shard-bytes",
			"oldest-sample", "newest-sample", "capacity-pressure", "effective-retention-seconds",
		},
	)

	command.RegisterPipeFilters([]string{fleetSampleLatestPath},
		command.PipeFilter{Name: pipeOpDevice, Description: "Select one managed device by exact registry name", Argument: command.ArgText},
		command.PipeFilter{Name: pipeOpLabel, Description: "Select managed devices with this key=value label", Argument: command.ArgText},
		command.PipeFilter{Name: pipeOpFigure, Description: "Select one registered sample figure", Argument: command.ArgText},
		command.PipeFilter{Name: pipeOpReduce, Description: "Fold selected observations across matching devices", Argument: command.ArgNone},
		command.PipeFilter{Name: pipeOpRank, Description: "Sort measured devices descending by one figure", Argument: command.ArgText},
	)
	command.RegisterPipeFilters([]string{fleetSampleSeriesPath},
		command.PipeFilter{Name: pipeOpDevice, Description: "Select one managed device by exact registry name", Argument: command.ArgText},
		command.PipeFilter{Name: pipeOpLabel, Description: "Select managed devices with this key=value label", Argument: command.ArgText},
		command.PipeFilter{Name: pipeOpFigure, Description: "Select one registered sample figure", Argument: command.ArgText},
		command.PipeFilter{Name: pipeOpFrom, Description: "Set series start time as an RFC 3339 UTC instant", Argument: command.ArgText},
		command.PipeFilter{Name: pipeOpUntil, Description: "Set series end time as an RFC 3339 UTC instant", Argument: command.ArgText},
		command.PipeFilter{Name: pipeOpReduce, Description: "Fold matching devices per figure and UTC slot", Argument: command.ArgNone},
		command.PipeFilter{Name: pipeOpPoints, Description: "Limit each series to at most N points (1 to 2112)", Argument: command.ArgCount},
	)
	command.RegisterPipeFilters([]string{fleetSampleStoragePath})
}

func handleFleetSampleLatest(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	plan, refusal := parseLatestPipePlan(validated.PipeOperations())
	if refusal != nil {
		return refusal, nil
	}

	scope := pluginserver.ResolveFleetDeviceScope(ctx)
	filter := pluginserver.HistoryLatestFilter{
		Device: plan.device,
		Labels: plan.labels,
	}
	if !plan.hasReduce && !plan.hasRank && !plan.multipleFigures && plan.singleFigure != "" {
		filter.Figure = plan.singleFigure
	}

	rows, err := pluginserver.ManagedHistoryLatest(scope, filter)
	if err != nil {
		return historyQueryRefusal(fleetSampleLatestPath, err), nil
	}
	if plan.impossibleDevice {
		rows = nil
	}

	var (
		selectedFigure string
		figureEmpty    bool
		reduced        bool
		reducedRows    []pluginserver.HistoryLatestReducedRow
	)
	for _, step := range plan.steps {
		switch step.op {
		case pipeOpFigure:
			if reduced {
				reducedRows = filterReducedLatestByFigure(reducedRows, step.arg)
				continue
			}
			if selectedFigure != "" && selectedFigure != step.arg {
				figureEmpty = true
				continue
			}
			selectedFigure = step.arg
		case pipeOpRank:
			if figureEmpty {
				rows = nil
				continue
			}
			if selectedFigure != "" && selectedFigure != step.arg {
				rows = nil
				continue
			}
			rows, err = pluginserver.RankLatestRows(rows, step.arg)
			if err != nil {
				return historyQueryRefusal(fleetSampleLatestPath, err), nil
			}
		case pipeOpReduce:
			reduced = true
			if figureEmpty {
				reducedRows = []pluginserver.HistoryLatestReducedRow{}
				continue
			}
			reducedRows, err = pluginserver.ReduceLatestRows(rows, selectedFigure)
			if err != nil {
				return historyQueryRefusal(fleetSampleLatestPath, err), nil
			}
		}
	}

	if reduced {
		if reducedRows == nil {
			reducedRows = []pluginserver.HistoryLatestReducedRow{}
		}
		return plugin.NewResponse(plugin.StatusDone, plugin.Slice[pluginserver.HistoryLatestReducedRow](reducedRows)), nil
	}

	if figureEmpty {
		for i := range rows {
			rows[i].Figures = nil
		}
	} else if selectedFigure != "" && filter.Figure == "" {
		narrowLatestRowsToFigure(rows, selectedFigure)
	}
	if rows == nil {
		rows = []pluginserver.HistoryLatestRow{}
	}
	return plugin.NewResponse(plugin.StatusDone, plugin.Slice[pluginserver.HistoryLatestRow](rows)), nil
}

func handleFleetSampleSeries(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	plan, refusal := parseSeriesPipePlan(validated.PipeOperations())
	if refusal != nil {
		return refusal, nil
	}

	scope := pluginserver.ResolveFleetDeviceScope(ctx)
	if plan.impossible {
		if _, err := pluginserver.ManagedHistory(); err != nil {
			return historyQueryRefusal(fleetSampleSeriesPath, err), nil
		}
		return plugin.NewResponse(plugin.StatusDone, plugin.Slice[pluginserver.HistorySeriesRow]{}), nil
	}

	rows, err := pluginserver.ManagedHistorySeries(scope, plan.query)
	if err != nil {
		return historyQueryRefusal(fleetSampleSeriesPath, err), nil
	}
	if rows == nil {
		rows = []pluginserver.HistorySeriesRow{}
	}
	return plugin.NewResponse(plugin.StatusDone, plugin.Slice[pluginserver.HistorySeriesRow](rows)), nil
}

func handleFleetSampleStorage(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	ops := validated.PipeOperations()
	if len(ops) > 0 {
		return plugin.OperatorErrorResponse(opererr.New(
			"unknown fleet sample storage pipe operation "+ops[0].Name,
			"storage status accepts only generic output and filtering pipes",
			fleetSampleStoragePath+" | json",
		)), nil
	}

	scope := pluginserver.ResolveFleetDeviceScope(ctx)
	status, err := pluginserver.ManagedHistoryStorage(scope)
	if err != nil {
		return historyQueryRefusal(fleetSampleStoragePath, err), nil
	}
	if status.Devices == nil {
		status.Devices = []pluginserver.HistoryStorageDeviceUsage{}
	}
	return plugin.NewResponse(plugin.StatusDone, fleetHistoryStorageDoc{HistoryStorageStatus: status}), nil
}

func parseLatestPipePlan(ops []command.PipeOperation) (latestPipePlan, *plugin.Response) {
	var plan latestPipePlan
	for _, op := range ops {
		switch op.Name {
		case pipeOpDevice:
			if plan.hasReduce {
				return latestPipePlan{}, postReduceRefusal(fleetSampleLatestPath, pipeOpDevice, "edge-01")
			}
			name, refusal := requireSinglePipeArg(fleetSampleLatestPath, pipeOpDevice, op.Args, "edge-01")
			if refusal != nil {
				return latestPipePlan{}, refusal
			}
			if plan.device != "" && plan.device != name {
				plan.impossibleDevice = true
				continue
			}
			plan.device = name
		case pipeOpLabel:
			if plan.hasReduce {
				return latestPipePlan{}, postReduceRefusal(fleetSampleLatestPath, pipeOpLabel, "region=west")
			}
			key, value, refusal := parseLabelPipeArg(fleetSampleLatestPath, op.Args)
			if refusal != nil {
				return latestPipePlan{}, refusal
			}
			if plan.labels == nil {
				plan.labels = make(map[string]string, 2)
			}
			if existing, exists := plan.labels[key]; exists && existing != value {
				plan.impossibleDevice = true
				continue
			}
			plan.labels[key] = value
		case pipeOpFigure:
			fig, refusal := parseFigurePipeArg(fleetSampleLatestPath, pipeOpFigure, op.Args)
			if refusal != nil {
				return latestPipePlan{}, refusal
			}
			if plan.singleFigure == "" {
				plan.singleFigure = fig
			} else if plan.singleFigure != fig {
				plan.multipleFigures = true
			}
			plan.steps = append(plan.steps, latestPipeStep{op: pipeOpFigure, arg: fig})
		case pipeOpRank:
			if plan.hasReduce {
				return latestPipePlan{}, plugin.OperatorErrorResponse(opererr.New(
					"cannot rank after reduce",
					"reduced rows carry aggregated figure values with no device identity to rank",
					fleetSampleLatestPath+" | rank "+sampleFigureExample,
				))
			}
			fig, refusal := parseFigurePipeArg(fleetSampleLatestPath, pipeOpRank, op.Args)
			if refusal != nil {
				return latestPipePlan{}, refusal
			}
			plan.hasRank = true
			plan.steps = append(plan.steps, latestPipeStep{op: pipeOpRank, arg: fig})
		case pipeOpReduce:
			if len(op.Args) > 0 {
				return latestPipePlan{}, plugin.OperatorErrorResponse(opererr.New(
					"pipe filter reduce does not accept an argument",
					"reduce folds the selected device observations using each figure's registered aggregation rule",
					fleetSampleLatestPath+" | reduce",
				))
			}
			if plan.hasReduce {
				return latestPipePlan{}, plugin.OperatorErrorResponse(opererr.New(
					"cannot apply reduce more than once",
					"the observations are already folded into reduced figure rows",
					fleetSampleLatestPath+" | reduce",
				))
			}
			plan.hasReduce = true
			plan.steps = append(plan.steps, latestPipeStep{op: pipeOpReduce})
		default:
			return latestPipePlan{}, plugin.OperatorErrorResponse(opererr.New(
				"unknown fleet sample latest pipe operation "+op.Name,
				"latest supports device, label, figure, reduce, and rank filters",
				fleetSampleLatestPath+" | figure "+sampleFigureExample,
			))
		}
	}
	return plan, nil
}

func parseSeriesPipePlan(ops []command.PipeOperation) (seriesPipePlan, *plugin.Response) {
	var plan seriesPipePlan
	var hasPoints bool
	for _, op := range ops {
		switch op.Name {
		case pipeOpDevice:
			if plan.query.Reduce {
				return seriesPipePlan{}, postReduceRefusal(fleetSampleSeriesPath, pipeOpDevice, "edge-01")
			}
			name, refusal := requireSinglePipeArg(fleetSampleSeriesPath, pipeOpDevice, op.Args, "edge-01")
			if refusal != nil {
				return seriesPipePlan{}, refusal
			}
			if plan.query.Device != "" && plan.query.Device != name {
				plan.impossible = true
				continue
			}
			plan.query.Device = name
		case pipeOpLabel:
			if plan.query.Reduce {
				return seriesPipePlan{}, postReduceRefusal(fleetSampleSeriesPath, pipeOpLabel, "region=west")
			}
			key, value, refusal := parseLabelPipeArg(fleetSampleSeriesPath, op.Args)
			if refusal != nil {
				return seriesPipePlan{}, refusal
			}
			if plan.query.Labels == nil {
				plan.query.Labels = make(map[string]string, 2)
			}
			if existing, exists := plan.query.Labels[key]; exists && existing != value {
				plan.impossible = true
				continue
			}
			plan.query.Labels[key] = value
		case pipeOpFigure:
			fig, refusal := parseFigurePipeArg(fleetSampleSeriesPath, pipeOpFigure, op.Args)
			if refusal != nil {
				return seriesPipePlan{}, refusal
			}
			if plan.query.Figure != "" && plan.query.Figure != fig {
				plan.impossible = true
				continue
			}
			plan.query.Figure = fig
		case pipeOpFrom:
			if hasPoints {
				return seriesPipePlan{}, plugin.OperatorErrorResponse(opererr.New(
					"cannot set from after points",
					"the time range must be bounded before rebucketing into points",
					fleetSampleSeriesPath+" | from 2026-11-24T00:00:00Z | points 300",
				))
			}
			if !plan.query.From.IsZero() {
				return seriesPipePlan{}, plugin.OperatorErrorResponse(opererr.New(
					"cannot specify from more than once",
					"a series query accepts one start bound",
					fleetSampleSeriesPath+" | from 2026-11-24T00:00:00Z",
				))
			}
			raw, refusal := requireSinglePipeArg(fleetSampleSeriesPath, pipeOpFrom, op.Args, "2026-11-24T00:00:00Z")
			if refusal != nil {
				return seriesPipePlan{}, refusal
			}
			ts, parseErr := parseSeriesInstant(pipeOpFrom, raw)
			if parseErr != nil {
				return seriesPipePlan{}, plugin.OperatorErrorResponse(parseErr)
			}
			plan.query.From = ts
		case pipeOpUntil:
			if hasPoints {
				return seriesPipePlan{}, plugin.OperatorErrorResponse(opererr.New(
					"cannot set until after points",
					"the time range must be bounded before rebucketing into points",
					fleetSampleSeriesPath+" | until 2026-11-25T00:00:00Z | points 300",
				))
			}
			if !plan.query.Until.IsZero() {
				return seriesPipePlan{}, plugin.OperatorErrorResponse(opererr.New(
					"cannot specify until more than once",
					"a series query accepts one end bound",
					fleetSampleSeriesPath+" | until 2026-11-25T00:00:00Z",
				))
			}
			raw, refusal := requireSinglePipeArg(fleetSampleSeriesPath, pipeOpUntil, op.Args, "2026-11-25T00:00:00Z")
			if refusal != nil {
				return seriesPipePlan{}, refusal
			}
			ts, parseErr := parseSeriesInstant(pipeOpUntil, raw)
			if parseErr != nil {
				return seriesPipePlan{}, plugin.OperatorErrorResponse(parseErr)
			}
			plan.query.Until = ts
		case pipeOpReduce:
			if len(op.Args) > 0 {
				return seriesPipePlan{}, plugin.OperatorErrorResponse(opererr.New(
					"pipe filter reduce does not accept an argument",
					"reduce folds matching devices per figure and aligned UTC slot",
					fleetSampleSeriesPath+" | reduce",
				))
			}
			if hasPoints {
				return seriesPipePlan{}, plugin.OperatorErrorResponse(opererr.New(
					"cannot apply reduce after points",
					"rebucketing individual devices before reduce loses minute-aligned set peaks",
					fleetSampleSeriesPath+" | reduce | points 300",
				))
			}
			if plan.query.Reduce {
				return seriesPipePlan{}, plugin.OperatorErrorResponse(opererr.New(
					"cannot apply reduce more than once",
					"the series is already reduced across matching devices",
					fleetSampleSeriesPath+" | reduce",
				))
			}
			plan.query.Reduce = true
		case pipeOpPoints:
			if hasPoints {
				return seriesPipePlan{}, plugin.OperatorErrorResponse(opererr.New(
					"cannot specify points more than once",
					"a series query accepts one point budget",
					fleetSampleSeriesPath+" | points 300",
				))
			}
			raw, refusal := requireSinglePipeArg(fleetSampleSeriesPath, pipeOpPoints, op.Args, "300")
			if refusal != nil {
				return seriesPipePlan{}, refusal
			}
			n, convErr := strconv.Atoi(raw)
			if convErr != nil {
				return seriesPipePlan{}, invalidPointsRefusal(raw)
			}
			if n < 1 {
				return seriesPipePlan{}, invalidPointsRefusal(raw)
			}
			if n > pluginserver.HistorySeriesMaxPoints {
				return seriesPipePlan{}, invalidPointsRefusal(raw)
			}
			hasPoints = true
			plan.query.Points = n
		default:
			return seriesPipePlan{}, plugin.OperatorErrorResponse(opererr.New(
				"unknown fleet sample series pipe operation "+op.Name,
				"series supports device, label, figure, from, until, reduce, and points filters",
				fleetSampleSeriesPath+" | figure "+sampleFigureExample,
			))
		}
	}
	if !plan.query.From.IsZero() && !plan.query.Until.IsZero() && !plan.query.From.Before(plan.query.Until) {
		return seriesPipePlan{}, plugin.OperatorErrorResponse(opererr.New(
			"empty or reversed time range",
			"the series start time (from) must be strictly before the end time (until)",
			fleetSampleSeriesPath+" | from 2026-11-24T00:00:00Z | until 2026-11-25T00:00:00Z",
		))
	}
	return plan, nil
}

func requireSinglePipeArg(cmdPath, opName string, args []string, example string) (string, *plugin.Response) {
	if len(args) != 1 {
		return "", plugin.OperatorErrorResponse(opererr.New(
			"pipe filter "+opName+" requires one argument",
			"provide a single value for "+opName,
			cmdPath+" | "+opName+" "+example,
		))
	}
	val := strings.TrimSpace(args[0])
	if val == "" {
		return "", plugin.OperatorErrorResponse(opererr.New(
			"pipe filter "+opName+" requires a non-empty argument",
			"provide a value for "+opName,
			cmdPath+" | "+opName+" "+example,
		))
	}
	return val, nil
}

func parseLabelPipeArg(cmdPath string, args []string) (key, value string, refusal *plugin.Response) {
	pair := strings.Join(args, " ")
	key, value, valid := splitFleetLabel(pair)
	if !valid {
		return "", "", plugin.OperatorErrorResponse(opererr.New(
			"the fleet label filter requires a key=value pair",
			"each managed device label has a non-empty key and value separated by =",
			cmdPath+" | label region=west",
		))
	}
	return key, value, nil
}

func parseFigurePipeArg(cmdPath, opName string, args []string) (string, *plugin.Response) {
	fig, refusal := requireSinglePipeArg(cmdPath, opName, args, sampleFigureExample)
	if refusal != nil {
		return "", refusal
	}
	if _, ok := fleet.LookupSampleFigure(fig); !ok {
		return "", plugin.OperatorErrorResponse(opererr.New(
			"unknown fleet sample figure "+fig,
			"the figure name is not in the registered fleet sample vocabulary",
			cmdPath+" | "+opName+" "+sampleFigureExample,
		))
	}
	return fig, nil
}

func parseSeriesInstant(opName, raw string) (time.Time, *opererr.Error) {
	ts, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, opererr.Wrap(err,
			"invalid "+opName+" timestamp "+raw,
			opName+" requires an RFC 3339 UTC timestamp",
			fleetSampleSeriesPath+" | "+opName+" 2026-11-24T00:00:00Z",
		)
	}
	_, offset := ts.Zone()
	if offset != 0 {
		return time.Time{}, opererr.New(
			opName+" timestamp "+raw+" must be in UTC",
			"series time bounds use UTC timestamps ending in Z or +00:00",
			fleetSampleSeriesPath+" | "+opName+" "+ts.UTC().Format(time.RFC3339),
		)
	}
	return ts.UTC(), nil
}

func postReduceRefusal(cmdPath, opName, example string) *plugin.Response {
	return plugin.OperatorErrorResponse(opererr.New(
		"cannot filter by "+opName+" after reduce",
		"cannot filter by device or label after reduce because reduced rows carry no device identity",
		cmdPath+" | "+opName+" "+example+" | reduce",
	))
}

func invalidPointsRefusal(raw string) *plugin.Response {
	var why textbuf.Buffer
	why.Str("points must be an integer between 1 and ").Int(pluginserver.HistorySeriesMaxPoints)
	return plugin.OperatorErrorResponse(opererr.New(
		"invalid points count "+raw,
		why.String(),
		fleetSampleSeriesPath+" | points 300",
	))
}

func historyQueryRefusal(path string, cause error) *plugin.Response {
	if opErr, ok := errors.AsType[*opererr.Error](cause); ok {
		return plugin.OperatorErrorResponse(opErr)
	}
	if errors.Is(cause, pluginserver.ErrManagedNotServing) {
		return plugin.OperatorErrorResponse(opererr.Wrap(cause,
			path+": unable to read fleet metric history",
			"the hub is not serving managed devices",
			"ze show doctor",
		))
	}
	return plugin.OperatorErrorResponse(opererr.Wrap(cause,
		path+": unable to read fleet metric history",
		"the hub could not answer the metric history query",
		"ze show doctor",
	))
}

func filterReducedLatestByFigure(rows []pluginserver.HistoryLatestReducedRow, figure string) []pluginserver.HistoryLatestReducedRow {
	matched := make([]pluginserver.HistoryLatestReducedRow, 0, 1)
	for i := range rows {
		if rows[i].Figure == figure {
			matched = append(matched, rows[i])
		}
	}
	return matched
}

func narrowLatestRowsToFigure(rows []pluginserver.HistoryLatestRow, figure string) {
	for i := range rows {
		if !rows[i].HasReport {
			rows[i].Figures = nil
			continue
		}
		if val, ok := rows[i].Figures[figure]; ok {
			rows[i].Figures = map[string]float64{figure: val}
			continue
		}
		rows[i].Figures = nil
	}
}
