// Fixture: handlers that refuse with an error answer and a nil Go error, one
// per status, beside the answers the scan does not read.
package answers

import (
	"errors"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
)

var errUnavailable = errors.New("reactor not available")

func answerTyped(_ *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
	if args.Len() == 0 {
		return plugin.OperatorErrorResponse(opererr.New("peer not found", "no session is configured", "ze config edit")), nil
	}
	return &plugin.Response{Status: plugin.StatusDone}, nil
}

func answerPlain(_ *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
	if args.Len() == 0 {
		return &plugin.Response{Status: plugin.StatusError, Error: "dispatcher unavailable"}, nil
	}
	if args.Len() == 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "peer " + args.Get(0) + " refused",
		}, nil
	}
	if args.Len() == 2 {
		return &plugin.Response{Status: plugin.StatusError}, nil
	}
	if args.Len() == 3 {
		return &plugin.Response{Status: plugin.StatusError, Error: errors.New("bad argument").Error()}, nil
	}
	return &plugin.Response{Status: plugin.StatusError, Error: opererr.New("peer refused", "the session is down", "show bgp peer").Error()}, nil
}

func answerPassThrough(_ *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
	if err := work(); err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil
	}
	forwarded := forward()
	if args.Len() == 0 {
		return &plugin.Response{Status: plugin.StatusError, Error: forwarded.Error}, nil
	}
	return &plugin.Response{Status: plugin.StatusError, Error: errUnavailable.Error()}, nil
}

func answerNested(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	helper := func() *plugin.Response {
		return &plugin.Response{Status: plugin.StatusError, Error: "inside a nested literal"}
	}
	return helper(), nil
}

func notRegistered() *plugin.Response {
	return &plugin.Response{Status: plugin.StatusError, Error: "not a handler"}
}

func forward() *plugin.Response {
	return notRegistered()
}

func work() error {
	return errUnavailable
}
