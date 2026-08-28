// Design: docs/architecture/testing/verify-freshness-scope.md -- verification failure summary command
package verifysummary

import (
	"errors"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/verify"
)

const name = "verify-summary"

// Answer appends one native verification failure summary.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return Actions{Actions: []Action{{
			Action: "append",
			Usage:  "append failures <failures-log> stage <stage> log <stage-log>",
		}}}, 0
	}
	if len(args) != 7 {
		return refuse("usage: le verify-summary append failures <failures-log> stage <stage> log <stage-log>"), 2
	}
	if args[0] != "append" {
		return refuse("unknown action " + args[0]), 2
	}
	if args[1] != "failures" {
		return refuse("expected failures before the failure-index path"), 2
	}
	if args[3] != "stage" {
		return refuse("expected stage before the stage name"), 2
	}
	if args[5] != "log" {
		return refuse("expected log before the stage-log path"), 2
	}
	summary, err := verify.AppendSummary(args[2], args[4], args[6])
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return summary, 0
}

// Action is one closed command form.
type Action struct {
	Action string `json:"action"`
	Usage  string `json:"usage"`
}

// Actions is the command inventory.
type Actions struct {
	Actions []Action `json:"actions"`
}

func refuse(message string) any {
	leaction.ReportError(errors.New("verify-summary: " + message))
	return nil
}
