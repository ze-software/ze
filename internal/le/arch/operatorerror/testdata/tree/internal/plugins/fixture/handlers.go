// Fixture: handler bodies, one per return-site status.
package fixture

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/rpc"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
)

var handlers []pluginserver.Handler

func handlePlain(_ *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
	if args.Len() == 0 {
		return nil, errors.New("peer not found")
	}
	if args.Len() == 1 {
		return nil, fmt.Errorf("peer %s refused", "192.0.2.1")
	}
	// The zero operator error: a literal names no problem, why or fix.
	return nil, &opererr.Error{}
}

func handleTyped(_ *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
	if args.Len() == 0 {
		return nil, opererr.New("peer not found", "no session is configured", "ze config edit")
	}
	return nil, opererr.Wrap(errors.New("eof"), "peer refused", "the session is down", "show bgp peer")
}

func handlePassThrough(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	if err := work(); err != nil {
		return nil, err
	}
	return nil, nil
}

func forward(name string) pluginserver.Handler {
	return func(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		return nil, errors.New("forward refused: " + name)
	}
}

func verifyFixture(_ []rpc.ConfigSection) error {
	return fmt.Errorf("fixture section invalid")
}

func rpcFixture(_ json.RawMessage) (any, error) {
	return nil, opererr.New("rpc refused", "the request carried no name", "ze help fixture")
}

func notRegistered() error {
	return errors.New("not a handler")
}

func work() error {
	helper := func() error { return errors.New("inside a nested literal") }
	return helper()
}
