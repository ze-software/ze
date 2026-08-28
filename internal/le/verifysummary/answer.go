// Design: docs/architecture/testing/verify-freshness-scope.md -- verification failure summary command
package verifysummary

import (
	"errors"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/verifyengine"
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
		refuse("usage: le verify-summary append failures <failures-log> stage <stage> log <stage-log>")
		return nil, 2
	}
	if args[0] != "append" {
		refuse("unknown action " + args[0])
		return nil, 2
	}
	if args[1] != "failures" {
		refuse("expected failures before the failure-index path")
		return nil, 2
	}
	if args[3] != "stage" {
		refuse("expected stage before the stage name")
		return nil, 2
	}
	if args[5] != "log" {
		refuse("expected log before the stage-log path")
		return nil, 2
	}
	summary, err := verifyengine.AppendSummary(args[2], args[4], args[6])
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

func refuse(message string) {
	leaction.ReportError(errors.New("verify-summary: " + message))
}
