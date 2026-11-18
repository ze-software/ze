// Design: docs/contributing/github-integration.md -- registered initialization and one-shot approved delivery.
// Related: notification_activation.go -- private operator authority and credential files.
package github

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

type notificationActivationOperation uint8

const codeNotificationActivation = "github-notification-unavailable"

// notificationActivationRefusal retains the delivery outcome alongside a
// discoverable readiness code. It contains no credential or private contents.
type notificationActivationRefusal struct {
	notificationDeliveryReport
	Diagnostic string `json:"diagnostic"`
}

const (
	notificationActivationUnspecified notificationActivationOperation = iota
	notificationActivationInitialize
	notificationActivationDeliver
)

func notificationActivationActions(read notificationRunReader, transport http.RoundTripper) []leaction.Action {
	return []leaction.Action{
		{Verb: "receipt", Does: "create", Summary: "initialize one approved workflow receipt without delivery",
			Why:        "Read the private operator-provisioned approval and one trusted workflow run. Create an empty durable receipt only if none exists. This explicit action never replaces state or sends a message.",
			Parameters: notificationActivationParameters(), AnswerArgs: func(args leaction.Arguments) (any, int) {
				return answerNotificationActivation(args, notificationActivationInitialize, read, transport)
			}},
		{Verb: "delivery", Does: "run", Summary: "attempt one approved maintainer notification transition",
			Why:        "Read the private operator-provisioned approval and one trusted workflow run. Use its existing locked receipt and separate private credential for at most one message. Missing, corrupt or unresolved state refuses delivery without initialization or retry.",
			Parameters: notificationActivationParameters(), AnswerArgs: func(args leaction.Arguments) (any, int) {
				return answerNotificationActivation(args, notificationActivationDeliver, read, transport)
			}},
	}
}

func notificationActivationParameters() []leaction.Parameter {
	return []leaction.Parameter{
		{Keyword: runKeyword, Kind: leaction.KindUint, Value: runIDValueLabel, Requirement: leaction.Required, Help: "the positive trusted GitHub Actions run ID"},
		{Keyword: "config", Kind: leaction.KindPath, Value: "private-file", Requirement: leaction.Required, Help: "the private operator-provisioned approval file"},
	}
}

func answerNotificationActivation(args leaction.Arguments, operation notificationActivationOperation, read notificationRunReader, transport http.RoundTripper) (any, int) {
	id, err := strconv.ParseUint(args.One(runKeyword), 10, 64)
	if err != nil {
		return notificationActivationError(err, operation, 2)
	}
	if id == 0 {
		return notificationActivationError(errInput, operation, 2)
	}
	activation, err := loadNotificationActivation(args.One("config"))
	if err != nil {
		return notificationActivationError(err, operation, 1)
	}
	var report notificationDeliveryReport
	switch operation {
	case notificationActivationInitialize:
		report, err = activation.initialize(context.Background(), read, id)
	case notificationActivationDeliver:
		report, err = activation.deliver(context.Background(), read, id, transport)
	case notificationActivationUnspecified:
		return notificationActivationError(errors.New("notification activation operation was not selected"), operation, 2)
	default:
		panic("BUG: unknown internal notification activation operation")
	}
	if err != nil {
		notificationActivationError(err, operation, 1)
		if report.State == "" {
			// No delivery result exists when readiness failed before an attempt.
			report.State = notificationWireBlocked
		}
		return notificationActivationRefusal{notificationDeliveryReport: report, Diagnostic: codeNotificationActivation}, 1
	}
	return report, 0
}

func notificationActivationError(err error, operation notificationActivationOperation, code int) (any, int) {
	leaction.ReportError(notificationActivationOperatorError(err, operation))
	report := notificationDeliveryReport{State: notificationWireBlocked,
		Reason: "private approval, credential or delivery readiness is unavailable"}
	return notificationActivationRefusal{notificationDeliveryReport: report, Diagnostic: codeNotificationActivation}, code
}

func notificationActivationOperatorError(err error, operation notificationActivationOperation) *opererr.Error {
	const why = "Keep existing delivery history. Have the operator check private approval, credential and receipt files before another attempt."
	switch operation {
	case notificationActivationInitialize:
		return opererr.Wrap(err, "GitHub notification initialization could not complete ("+codeNotificationActivation+")",
			why, "./le help github receipt")
	case notificationActivationDeliver:
		return opererr.Wrap(err, "GitHub notification delivery could not complete ("+codeNotificationActivation+")",
			why, "./le help github delivery")
	case notificationActivationUnspecified:
		return opererr.Wrap(err, "GitHub notification operation was not selected ("+codeNotificationActivation+")",
			why, "./le help github")
	default:
		panic("BUG: unknown internal notification activation operation")
	}
}
