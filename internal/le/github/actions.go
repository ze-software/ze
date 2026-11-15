// Design: docs/contributing/github-integration.md -- one selector and shared rendering.

package github

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

const area = "github"
const runKeyword = "run"
const runIDValueLabel = "run-id"

var actions = reportArea(answerReport)

func reportArea(answer func(leaction.Arguments) (any, int)) leaction.Area {
	notifications := notificationActions()
	declared := make([]leaction.Action, 1, 1+len(notifications))
	declared[0] = leaction.Action{
		Verb: "report", Summary: "read identified GitHub workflow and job evidence",
		Why: "Read every bounded API page for one exact revision or workflow run. Return job and step results with their source identities; incomplete data cannot certify a passing revision.",
		Parameters: []leaction.Parameter{
			{Keyword: "revision", Kind: leaction.KindLabel, Value: "full-sha", Help: "the full 40-character commit ID to report", Requirement: leaction.Optional},
			{Keyword: runKeyword, Kind: leaction.KindUint, Value: runIDValueLabel, Help: "one positive GitHub Actions run ID", Requirement: leaction.Optional},
			{Keyword: "machine", Kind: leaction.KindSwitch, Help: "observe local checkout identity and current capacity"},
		},
		AnswerArgs: answer,
	}
	declared = append(declared, notifications...)
	return leaction.New(area, declared...)
}

// Actions describes the registered reporting grammar.
func Actions() leaction.List { return actions.Actions() }

// Subs renders the same grammar for help and completion.
func Subs() string { return actions.Subs() }

// Answer dispatches the registered command and returns structured evidence.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func answerReport(args leaction.Arguments) (any, int) {
	if args.Has("machine") {
		return answerMachine(args)
	}
	client, err := NewClient(http.DefaultTransport)
	if err != nil {
		leaction.ReportError(reportActionError(err, "github report could not initialize its API reader",
			"The selected workflow evidence cannot be read with the current reader configuration.", "./le help github report"))
		return nil, 3
	}
	return answerWith(client, args)
}

func answerWith(client *Client, args leaction.Arguments) (any, int) {
	if args.Has("machine") {
		return answerMachine(args)
	}
	if args.Has("revision") == args.Has(runKeyword) {
		leaction.ReportError(opererr.New("github report needs exactly one selector",
			"A revision report and a run report identify different evidence. Supply revision or run, without combining them.",
			"./le help github report"))
		return nil, 2
	}
	var report Report
	var err error
	if args.Has("revision") {
		report, err = client.reportRevision(context.Background(), args.One("revision"))
	} else {
		id, parseErr := strconv.ParseUint(args.One(runKeyword), 10, 64)
		if parseErr != nil {
			leaction.ReportError(opererr.Wrap(parseErr, "github report run ID is not an unsigned integer",
				"The run cannot be identified from this value.", "./le help github report run"))
			return nil, 2
		}
		if id == 0 {
			leaction.ReportError(opererr.New("github report run ID must be positive",
				"Zero identifies no GitHub Actions run.", "./le help github report run"))
			return nil, 2
		}
		report, err = client.reportRun(context.Background(), id)
	}
	if err != nil {
		leaction.ReportError(reportActionError(err, "github report could not read the selected evidence",
			"The returned observations cannot establish complete evidence for the requested revision or run.", "./le help github report"))
		if report.Diagnostic == codeInput {
			return report, 2
		}
		return report, 1
	}
	return report, 0
}

func answerMachine(args leaction.Arguments) (any, int) {
	if args.Has(runKeyword) {
		leaction.ReportError(opererr.New("machine reporting cannot select a GitHub run",
			"Current local observations cannot certify a previous workflow result.", "./le help github report"))
		return nil, 2
	}
	report, err := observeMachine(args.One("revision"), localMachineSources())
	if err != nil {
		leaction.ReportError(reportActionError(err, "github report machine could not qualify the local checkout",
			"Current source identity and capacity remain unqualified; no synchronization or test was started.", "./le github report machine"))
		if report.Diagnostic == codeInput {
			return report, 2
		}
		return report, 1
	}
	return report, 0
}

// reportActionError retains the producer's operator guidance when it already
// exists. Only an unqualified failure needs the action's problem and remedy.
func reportActionError(err error, problem, why, fix string) *opererr.Error {
	if refusal, found := errors.AsType[*opererr.Error](err); found {
		return refusal
	}
	return opererr.Wrap(err, problem, why, fix)
}
