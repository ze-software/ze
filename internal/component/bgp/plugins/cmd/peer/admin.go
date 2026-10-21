// Design: docs/architecture/api/commands.md — BGP peer lifecycle and introspection handlers
// Related: peer.go — registration and the teardown handler beside these
// RFC: rfc/short/rfc9003.md -- Section 2, Shutdown Communication

package peer

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// handlePeerDisable handles "request peer <sel> disable [message]": the peer is
// held administratively down until "request peer <sel> enable". Every token
// after the verb is the message, joined by one space, as teardown joins its own.
func handlePeerDisable(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	if _, errResp, err := pluginserver.RequireReactor(ctx); err != nil {
		return errResp, err
	}
	addr, errResp, err := pluginserver.ResolveSinglePeer(ctx, "disable")
	if err != nil {
		return errResp, err
	}

	shutdownMsg := textbuf.Join(validated.Tokens(), " ")
	if err := ctx.Reactor().DisablePeer(addr, shutdownMsg); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  fmt.Sprintf("disable failed: %v", err),
		}, fmt.Errorf("disable peer %s: %w", addr, err)
	}

	resp := map[string]any{fieldPeer: addr.String(), "admin-state": "down"}
	if shutdownMsg != "" {
		// RFC 9003 Section 2: the communication is carried in the Cease
		// "Administrative Shutdown". The answer echoes what fits on the wire.
		wireData := message.BuildShutdownData(shutdownMsg)
		if wireData[0] > 0 {
			resp["shutdown-message"] = string(wireData[1:])
		}
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(resp)}, nil
}

// handlePeerEnable handles "request peer <sel> enable": an administrative hold
// is released and the peer connects straight away. It takes no argument.
func handlePeerEnable(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	if _, errResp, err := pluginserver.RequireReactor(ctx); err != nil {
		return errResp, err
	}
	if extra := validated.Tokens(); len(extra) > 0 {
		var tb textbuf.Buffer
		reason := tb.Str("enable takes no argument, got '").Str(textbuf.Join(extra, " ")).
			Str("': run 'request peer <selector> enable'").String()
		return &plugin.Response{Status: plugin.StatusError, Error: reason}, errEnableTakesNoArgument
	}
	addr, errResp, err := pluginserver.ResolveSinglePeer(ctx, "enable")
	if err != nil {
		return errResp, err
	}
	if err := ctx.Reactor().EnablePeer(addr); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  fmt.Sprintf("enable failed: %v", err),
		}, fmt.Errorf("enable peer %s: %w", addr, err)
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{fieldPeer: addr.String(), "admin-state": "up"},
	}, nil
}
