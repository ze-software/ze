// Design: docs/contributing/testing.md -- strict final command verdicts on both transports.
package pluginengine

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Only a final Done may return a useful command document. A warning with useful
// data must remain a refusal, just like an explicit error, on both actual SDK
// transports and with either an explicit or empty peer context.
// MUTATION: omit the wire ExecutePeer final Verdict check; warning data then
// returns without an error, while the useful Done control still passes.
func TestExecutePeerRequiresDoneAcrossActualSDKTransports(t *testing.T) {
	for _, transport := range Transports() {
		for _, status := range []string{rpc.StatusDone, rpc.StatusWarning, rpc.StatusError} {
			for _, peer := range []string{"", "192.0.2.188"} {
				name := "empty-peer"
				if peer != "" {
					name = "explicit-peer"
				}
				t.Run(transport.String()+"/"+status+"/"+name, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
					defer cancel()
					type payload struct {
						Peer  string `json:"peer"`
						Value string `json:"value"`
					}
					type observation struct {
						serial, command, peer string
						args                  []string
					}
					called := make(chan observation, 1)
					const command = "show final verdict probe"
					engine := Start(t, transport, func(conn net.Conn) int {
						p := sdk.NewWithConn("strict-command-verdict-probe", conn)
						defer p.Close() //nolint:errcheck // Start owns and joins both connection ends.
						p.OnExecuteCommand(func(serial, actual string, args []string, callbackPeer string, _ []sdk.PipeOperation) (string, any, error) {
							called <- observation{serial: serial, command: actual, args: args, peer: callbackPeer}
							return status, payload{Peer: callbackPeer, Value: "useful-result"}, nil
						})
						if err := p.Run(ctx, &sdk.Registration{Commands: []sdk.CommandDecl{{Name: command}}}); err != nil {
							return 1
						}
						return 0
					})
					var body json.RawMessage
					var err error
					if peer == "" {
						body, err = engine.Execute(ctx, command, []string{"original argument"})
					} else {
						body, err = engine.ExecutePeer(ctx, command, []string{"original argument"}, peer)
					}
					select {
					case got := <-called:
						require.Equal(t, "1", got.serial)
						require.Equal(t, command, got.command)
						require.Equal(t, []string{"original argument"}, got.args)
						require.Equal(t, peer, got.peer)
					default:
						t.Fatal("the actual SDK callback did not run")
					}
					if status != rpc.StatusDone {
						require.Error(t, err, "a non-Done verdict cannot return useful output as success")
						require.Empty(t, body, "refused command output must not escape as a successful document")
						return
					}
					require.NoError(t, err, "the useful Done control must remain available")
					var got payload
					require.NoError(t, json.Unmarshal(body, &got))
					require.Equal(t, payload{Peer: peer, Value: "useful-result"}, got)
				})
			}
		}
	}
}
