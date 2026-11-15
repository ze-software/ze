// Design: docs/contributing/github-integration.md -- preview and explicit approved delivery share the registered action table.
package github

import (
	"context"
	"net/http"
	"strconv"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

type notificationRunReader func(context.Context, uint64) (Report, error)

func notificationActions() []leaction.Action {
	declared := append([]leaction.Action{notificationPreviewAction(readNotificationRun)},
		notificationActivationActions(readNotificationRun, http.DefaultTransport)...)
	return append(declared, notificationPollActions(http.DefaultTransport)...)
}

func notificationPreviewAction(read notificationRunReader) leaction.Action {
	return leaction.Action{Verb: "preview", Summary: "render candidate maintainer messages without delivery",
		Why: "Read one identified trusted main-branch workflow run and render unapproved candidate messages. This action sends nothing, records no approval and changes no delivery state; the common output pipe can retain the preview for review.",
		Parameters: []leaction.Parameter{
			{Keyword: runKeyword, Kind: leaction.KindUint, Value: runIDValueLabel, Requirement: leaction.Required, Help: "the positive GitHub Actions run ID to preview"},
			{Keyword: "destination", Kind: leaction.KindLabel, Value: "label", Requirement: leaction.Required, Help: "the proposed token-free destination label"},
			{Keyword: "channel", Kind: leaction.KindLabel, Value: "channel-id", Requirement: leaction.Optional, Help: "the token-free Discord channel identity to bind"},
			{Keyword: "webhook", Kind: leaction.KindLabel, Value: "webhook-id", Requirement: leaction.Optional, Help: "the token-free Discord webhook identity to bind"},
		}, AnswerArgs: func(args leaction.Arguments) (any, int) { return answerNotificationPreview(args, read) }}
}

func readNotificationRun(ctx context.Context, id uint64) (Report, error) {
	client, err := NewClient(http.DefaultTransport)
	if err != nil {
		return Report{}, err
	}
	return client.reportRun(ctx, id)
}

func answerNotificationPreview(args leaction.Arguments, read notificationRunReader) (any, int) {
	id, err := strconv.ParseUint(args.One(runKeyword), 10, 64)
	if err != nil {
		return notificationPreviewError(err, 2)
	}
	if id == 0 {
		return notificationPreviewError(errInput, 2)
	}
	if args.Has("channel") != args.Has("webhook") {
		return notificationPreviewError(errInput, 2)
	}
	destination, err := newNotificationDestination(args.One("destination"), args.One("channel"), args.One("webhook"))
	if err != nil {
		return notificationPreviewError(err, 2)
	}
	template, err := candidateNotificationTemplate()
	if err != nil {
		return notificationPreviewError(err, 3)
	}
	report, err := read(context.Background(), id)
	if err != nil {
		return notificationPreviewError(err, 1)
	}
	preview, err := buildNotificationPreview(report, template, destination)
	if err != nil {
		return notificationPreviewError(err, 1)
	}
	return preview, 0
}

func notificationPreviewError(err error, code int) (any, int) {
	leaction.ReportError(opererr.Wrap(err, "GitHub notification preview could not be prepared",
		"No message was sent and no approval or delivery state was recorded.", "./le help github preview"))
	return nil, code
}
