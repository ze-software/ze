// Design: docs/architecture/host/observability.md -- the host trend monitor
// Related: show_host.go -- the inventory sections of show host

package cmd

import (
	"errors"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/host/trend"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
)

// The refusal of show host trend while no trend monitor runs. The fix is the
// one the monitor's own doctor check names: the daemon starts the monitor at
// its system start, so a restart starts it again.
const (
	trendNotRunningProblem = "the host trend monitor is not running"
	trendNotRunningWhy     = "no rule watches the host resources, so a resource that runs out slowly, " +
		"such as kernel memory, is not reported before the box fails. " +
		"The daemon starts the monitor when it starts, so a restart starts it again"
	trendNotRunningFix = "ze signal restart"
)

func registerShowTrend() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-host:show-trend",
			Handler:    handleShowTrend,
		},
	)
}

// trendRuleAnswer is one rule of the show host trend answer. A figure the rule
// does not have is absent from the answer, never zero: its pointer is nil, or
// its text is empty, and the key is omitted. Level, limit and growth count the
// rule's unit.
type trendRuleAnswer struct {
	Name                string   `json:"name"`
	State               string   `json:"state"`
	Unit                string   `json:"unit"`
	ObservedSeconds     int64    `json:"observed-seconds"`
	Level               *float64 `json:"level,omitempty"`
	Limit               *float64 `json:"limit,omitempty"`
	Growth              *float64 `json:"growth,omitempty"`
	GrowthWindowSeconds *int64   `json:"growth-window-seconds,omitempty"`
	ExhaustionSeconds   *int64   `json:"exhaustion-seconds,omitempty"`
	Warning             string   `json:"warning,omitempty"`
	FailureSource       string   `json:"failure-source,omitempty"`
	FailureCause        string   `json:"failure-cause,omitempty"`
}

// handleShowTrend answers show host trend: every rule the running trend
// monitor publishes, in registration order. The monitor runs in the daemon
// process and host-cmd's handlers are builtin RPCs the daemon's dispatcher
// serves, so trend.Rules reads the daemon's own monitor. With no monitor
// running it refuses, and never answers an empty list.
func handleShowTrend(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	statuses, err := trend.Rules()
	if err != nil {
		return plugin.OperatorErrorResponse(trendReadRefusal(err)), nil
	}
	rules := make([]trendRuleAnswer, 0, len(statuses))
	for i := range statuses {
		rules = append(rules, trendRuleOf(&statuses[i]))
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"rules": rules}}, nil
}

// trendReadRefusal is the operator error of a trend read that failed. Rules
// answers no error but ErrNotRunning today; any other error keeps its cause,
// so a later failure is never reported as a monitor that is not running.
func trendReadRefusal(err error) *opererr.Error {
	if errors.Is(err, trend.ErrNotRunning) {
		return opererr.New(trendNotRunningProblem, trendNotRunningWhy, trendNotRunningFix)
	}
	return opererr.Wrap(err, "the host trend state could not be read",
		"show host trend has no rule to list without it", "show doctor")
}

// trendRuleOf builds the answer of one rule from its published state.
func trendRuleOf(status *trend.RuleStatus) trendRuleAnswer {
	answer := trendRuleAnswer{
		Name:            status.Name(),
		State:           status.State().String(),
		Unit:            status.Unit().String(),
		ObservedSeconds: wholeSeconds(status.Observed()),
	}
	if level, limit, read := status.Reading(); read {
		answer.Level, answer.Limit = &level, &limit
	}
	if rise, span, measured := status.Growth(); measured {
		window := wholeSeconds(span)
		answer.Growth, answer.GrowthWindowSeconds = &rise, &window
	}
	if exhaustion, projected := status.Exhaustion(); projected {
		seconds := wholeSeconds(exhaustion)
		answer.ExhaustionSeconds = &seconds
	}
	if message, _, warned := status.Warning(); warned {
		answer.Warning = message
	}
	if source, cause, failed := status.Failure(); failed {
		answer.FailureSource, answer.FailureCause = source, cause
	}
	return answer
}

// wholeSeconds is d in whole seconds, the unit of every duration the answer
// carries.
func wholeSeconds(d time.Duration) int64 {
	return int64(d / time.Second)
}
