// Design: docs/architecture/l2tp/followup-l2tp-call.md -- AC-4 request l2tp outgoing-call
// RFC: rfc/short/rfc2661.md -- RFC 2661 Section 10.4 (LNS-side outgoing call)
// Related: ../outgoing_call.go -- Subsystem.PlaceOutgoingCall drives the dial

package cmd

import (
	"errors"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errOutgoingCallMissingRemote = errors.New("l2tp: missing remote argument (request l2tp outgoing-call remote <name> called <number>)")
	errOutgoingCallMissingCalled = errors.New("l2tp: missing called argument (request l2tp outgoing-call remote <name> called <number>)")
)

// handleOutgoingCall implements `request l2tp outgoing-call remote <name>
// called <number>`. The two typed selectors arrive via the CommandContext
// (the YANG nests a container per selector, so both reach ctx.Selectors).
// The handler blocks in Subsystem.PlaceOutgoingCall until the call
// establishes or fails, then reports the outcome -- including the failure
// cause and RFC 2661 Result Code when the call did not come up, so an
// operator sees why (tunnel auth reject, tie-breaker loss, peer CDN, or
// timeout) rather than a bare error.
func handleOutgoingCall(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	remote := ctx.Selector("remote")
	called := ctx.Selector("called")
	if remote == "" {
		return errResponse(errOutgoingCallMissingRemote), nil
	}
	if called == "" {
		return errResponse(errOutgoingCallMissingCalled), nil
	}

	svc := l2tp.LookupService()
	if svc == nil {
		return errResponse(errSubsystemUnavailable), nil
	}

	result, err := svc.PlaceOutgoingCall(remote, called)
	if err != nil {
		// The call did not come up. Surface the cause and, when the peer or
		// ze supplied one, the RFC 2661 Result Code, so tie-breaker loss and
		// authentication rejections are visible instead of a silent drop.
		payload := map[string]any{
			keyAction:     "outgoing-call",
			"remote":      remote,
			"called":      called,
			"established": false,
			"error":       err.Error(),
		}
		if result.ResultCode != 0 {
			payload["result-code"] = int(result.ResultCode)
		}
		var tb textbuf.Buffer
		tb.Str("l2tp outgoing-call to ").Quoted(remote).Str(" failed: ").Err(err)
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.String(),
			Data:   plugin.Map(payload),
		}, nil
	}

	return jsonResponse("l2tp outgoing-call", map[string]any{
		keyAction:     "outgoing-call",
		"remote":      remote,
		"called":      called,
		"established": true,
		"local-sid":   int(result.LocalSID),
		"remote-sid":  int(result.RemoteSID),
	})
}
