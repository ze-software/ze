// Design: docs/guide/command-reference.md -- NTP state belongs to its running process.
// Related: register.go -- runtime command handlers and date enrichment.

package ntp

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	commandNTPStatus = "show system ntp"
	commandNTPPeers  = "show system ntp peers"
)

// commandDecls is the one declaration read by the catalog and the runtime.
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{Name: commandNTPStatus, ShortHelp: "Show the NTP client's current synchronization state", Shape: "doc"},
		{Name: commandNTPPeers, ShortHelp: "Show the NTP client's current server measurements", Shape: "doc"},
	}
}

// executeNTPCommand reads only this process's worker state. The daemon reaches
// it through the same command callback for internal and forked runners.
func executeNTPCommand(_ string, name string, args []string, _ string) (string, any, error) {
	if len(args) != 0 {
		return plugin.StatusError, nil, opererr.New(name+" takes no arguments",
			"The NTP runtime commands read one state document and accept no arguments.",
			"ze show system ntp")
	}
	var handler pluginserver.Handler
	switch name {
	case commandNTPStatus:
		handler = handleShowSystemNTP
	case commandNTPPeers:
		handler = handleShowSystemNTPPeers
	default:
		return plugin.StatusError, nil, opererr.Wrap(fmt.Errorf("command %q is not served by the NTP worker", name),
			"unknown NTP command",
			"This worker serves show system ntp and show system ntp peers.",
			"ze show system ntp")
	}
	validated, err := command.ValidateArgs(args, nil, nil)
	if err != nil {
		return plugin.StatusError, nil, err
	}
	response, err := handler(nil, validated)
	if err != nil {
		return plugin.StatusError, nil, err
	}
	if response.Status == plugin.StatusError {
		return plugin.StatusError, nil, opererr.Wrap(errors.New(response.Error),
			"NTP runtime command could not read its worker state",
			"An unreadable worker state cannot be presented as an inactive client or an empty peer set.",
			"ze show system ntp")
	}
	return response.Status, response.Data, nil
}

func forwardShowSystemNTP(ctx *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
	return forwardNTPCommand(ctx, args, commandNTPStatus)
}

func forwardShowSystemNTPPeers(ctx *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
	return forwardNTPCommand(ctx, args, commandNTPPeers)
}

// forwardNTPCommand permits an inactive answer only from the accepted config.
// An active client is read through its runtime command, never daemon-local state.
func forwardNTPCommand(ctx *pluginserver.CommandContext, args command.ValidatedArgs, name string) (*plugin.Response, error) {
	cfg, refusal := commandNTPConfig(ctx)
	if refusal != nil {
		return plugin.OperatorErrorResponse(refusal), nil
	}
	if !cfg.Enabled {
		if name == commandNTPPeers {
			return &plugin.Response{Status: plugin.StatusDone,
				Data: plugin.Map{"peers": []map[string]any{}, "count": 0}}, nil
		}
		return &plugin.Response{Status: plugin.StatusDone,
			Data: plugin.Map{fieldEnabled: false}}, nil
	}
	if ctx.Dispatcher() == nil {
		return plugin.OperatorErrorResponse(ntpStateUnavailable()), nil
	}
	response, err := ctx.Dispatcher().ForwardToPlugin(ctx, name, args.Tokens(), ctx.PeerSelector())
	if err != nil {
		return plugin.OperatorErrorResponse(ntpCommandReadRefusal(name, err)), nil
	}
	if response == nil {
		return plugin.OperatorErrorResponse(ntpCommandReadRefusal(name,
			errors.New("the NTP worker returned no response"))), nil
	}
	if response.Status != plugin.StatusDone {
		return plugin.OperatorErrorResponse(ntpCommandReadRefusal(name,
			fmt.Errorf("worker status %q: %s", response.Status, response.Error))), nil
	}
	raw, ok := response.Data.(plugin.RawJSON)
	if !ok {
		return plugin.OperatorErrorResponse(ntpCommandReadRefusal(name,
			fmt.Errorf("worker data has type %T, expected a JSON document", response.Data))), nil
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return plugin.OperatorErrorResponse(ntpCommandReadRefusal(name, err)), nil
	}
	if data == nil {
		return plugin.OperatorErrorResponse(ntpCommandReadRefusal(name,
			errors.New("the NTP worker returned a null document"))), nil
	}
	if name == commandNTPStatus {
		enabled, ok := data[fieldEnabled].(bool)
		if !ok {
			return plugin.OperatorErrorResponse(ntpCommandReadRefusal(name,
				errors.New("the NTP worker returned no boolean enabled field"))), nil
		}
		if !enabled {
			return plugin.OperatorErrorResponse(ntpCommandReadRefusal(name,
				errors.New("the accepted configuration enables NTP but the worker is disabled"))), nil
		}
		if _, err := ntpReplyMetadata(data); err != nil {
			return plugin.OperatorErrorResponse(ntpCommandReadRefusal(name, err)), nil
		}
	} else {
		peers, ok := data["peers"].([]any)
		if !ok {
			return plugin.OperatorErrorResponse(ntpCommandReadRefusal(name,
				errors.New("the NTP worker returned no peers array"))), nil
		}
		count, ok := data["count"].(float64)
		if !ok {
			return plugin.OperatorErrorResponse(ntpCommandReadRefusal(name,
				errors.New("the NTP worker returned no numeric peer count"))), nil
		}
		if count != float64(len(peers)) {
			return plugin.OperatorErrorResponse(ntpCommandReadRefusal(name,
				errors.New("the NTP worker peer count disagrees with its peers array"))), nil
		}
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
}

// commandNTPConfig reads the daemon's accepted configuration. The coordinator
// holds it even when no BGP reactor is loaded. Missing authority is not disabled.
func commandNTPConfig(ctx *pluginserver.CommandContext) (ntpConfig, *opererr.Error) {
	if ctx == nil {
		return ntpConfig{}, ntpConfigurationUnavailable()
	}
	reactor := ctx.Reactor()
	if reactor == nil {
		return ntpConfig{}, ntpConfigurationUnavailable()
	}
	tree := reactor.GetConfigTree()
	if tree == nil {
		return ntpConfig{}, ntpConfigurationUnavailable()
	}
	encoded, err := json.Marshal(config.ActiveConfigMap(tree))
	if err != nil {
		return ntpConfig{}, ntpReadRefusal(err)
	}
	cfg, err := parseNTPConfig(string(encoded))
	if err != nil {
		return ntpConfig{}, ntpReadRefusal(err)
	}
	return cfg, nil
}

func ntpConfigurationUnavailable() *opererr.Error {
	return opererr.New("NTP state cannot be classified: the accepted daemon configuration is unavailable",
		"An unreadable configuration does not establish that the NTP client is disabled. Retry after the daemon is ready.",
		"ze show system ntp")
}

func ntpStateUnavailable() *opererr.Error {
	return opererr.New("NTP state is unavailable: the running client has not published a state snapshot",
		"No snapshot is an unknown state, not a disabled client or an empty peer set. Inspect the client before retrying.",
		"ze show system ntp")
}
