// Design: docs/contributing/testing.md -- command answers on actual SDK transports.
// Related: pluginengine.go -- answerWire selects the registered command's answer shape.
package pluginengine

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Goal: ordinary command answers reach the SDK's record reader, while a
// destination-bound command keeps the exact joined receipt its reader validates.
// Method: execute the actual SDK callback after five-stage startup and dispatch
// back through the fake engine on both transports, preserving data and failures.
// MUTATION: answer an ordinary dispatch with SendResult instead of record lines.
func TestDispatchAnswersUseActualSDKReaders(t *testing.T) {
	for _, transport := range Transports() {
		for _, mode := range []string{"ordinary", "args", "warning", "error", "bound", "physical"} {
			t.Run(transport.String()+"/"+mode, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				type observed struct {
					Status   string          `json:"status"`
					Data     json.RawMessage `json:"data"`
					Warnings []string        `json:"warnings"`
					Error    string          `json:"error"`
				}
				engine := Start(t, transport, func(conn net.Conn) int {
					p := sdk.NewWithConn("dispatch-answer-probe", conn)
					defer p.Close() //nolint:errcheck // the fixture closes and joins the connection
					p.OnExecuteCommand(func(_, _ string, _ []string, _ string) (string, any, error) {
						var result sdk.DispatchResult
						var err error
						switch mode {
						case "bound":
							result.Status, result.Data, err = p.DispatchDestinationCommand(ctx, rpc.DispatchCommandInput{
								Command: "request fixture bound", DestinationOwner: 701, DestinationSession: 141})
						case "physical":
							result.Status, result.Data, err = p.DispatchPeerCommand(ctx, rpc.DispatchCommandInput{
								Command: "request fixture physical", PeerOwner: 701, ConnectionID: 41})
						case "args":
							result, err = p.DispatchCommandArgsResult(ctx, "request fixture answer", []string{"one JSON argument"}, "")
						default:
							result, err = p.DispatchCommandResult(ctx, "request fixture answer")
						}
						out := observed{Status: result.Status, Data: result.Data, Warnings: result.Warnings}
						if err != nil {
							out.Error = err.Error()
						}
						return rpc.StatusDone, out, nil
					})
					if err := p.Run(ctx, &sdk.Registration{}); err != nil {
						return 1
					}
					return 0
				})
				engine.SetRPCHandler(func(_ context.Context, method string, params json.RawMessage) (any, error) {
					out := &rpc.DispatchCommandOutput{Status: rpc.VerdictDone, Data: json.RawMessage(`{"useful":"survivor"}`)}
					if mode == "args" {
						if method != rpc.MethodDispatchCommandArgs {
							return nil, errors.New("args probe reached the wrong engine operation")
						}
						var input rpc.DispatchCommandArgsInput
						if err := json.Unmarshal(params, &input); err != nil {
							return nil, err
						}
						if len(input.Args) != 1 || input.Args[0] != "one JSON argument" {
							return nil, errors.New("pre-tokenized argument changed")
						}
					} else {
						if method != rpc.MethodDispatchCommand {
							return nil, errors.New("command probe reached the wrong engine operation")
						}
						var input rpc.DispatchCommandInput
						if err := json.Unmarshal(params, &input); err != nil {
							return nil, err
						}
						if mode == "bound" {
							out.DestinationOwner, out.DestinationSession, out.DeadlineMS = input.DestinationOwner, input.DestinationSession, input.DeadlineMS
						}
						if mode == "physical" {
							out.PeerOwner, out.ConnectionID, out.DeadlineMS = input.PeerOwner, input.ConnectionID, input.DeadlineMS
						}
					}
					if mode == "error" {
						return nil, errors.New("fixture command refused")
					}
					if mode == "warning" {
						out.Status = rpc.VerdictWarning
						out.Warnings = []string{"useful output retained"}
					}
					return out, nil
				})
				body, err := engine.Execute(ctx, "probe", nil)
				require.NoError(t, err)
				var got observed
				require.NoError(t, json.Unmarshal(body, &got))
				if mode == "error" {
					require.Contains(t, got.Error, "fixture command refused")
					return
				}
				require.Empty(t, got.Error)
				require.JSONEq(t, `{"useful":"survivor"}`, string(got.Data))
				if mode == "warning" {
					require.Equal(t, rpc.VerdictWarning, got.Status)
					require.Equal(t, []string{"useful output retained"}, got.Warnings)
				} else {
					require.Equal(t, rpc.VerdictDone, got.Status)
				}
			})
		}
	}
}
