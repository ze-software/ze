// Design: docs/contributing/testing.md -- actual SDK command peer context.
package pluginengine

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Goal: explicit peer context reaches the real registered SDK callback
// identically over wire and bridge, while Execute preserves empty peer behavior.
// Method: echo callback arguments, including IPv6, escaped bytes and the empty
// control. This fixture does not authenticate a peer or prove native authority.
func TestExecutePeerReachesActualSDKAndPreservesEmptyControl(t *testing.T) {
	for _, transport := range Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			engine := Start(t, transport, func(conn net.Conn) int {
				p := sdk.NewWithConn("explicit-command-peer-probe", conn)
				defer p.Close() //nolint:errcheck // Start owns and joins both connection ends.
				p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
					if serial != "1" {
						return rpc.StatusError, nil, errors.New("command serial changed")
					}
					if command != "show explicit peer probe" {
						return rpc.StatusError, nil, errors.New("command route changed")
					}
					if len(args) != 1 {
						return rpc.StatusError, nil, errors.New("command argument count changed")
					}
					return rpc.StatusDone, struct {
						Peer     string `json:"peer"`
						Argument string `json:"argument"`
					}{Peer: peer, Argument: args[0]}, nil
				})
				if err := p.Run(ctx, &sdk.Registration{Commands: []sdk.CommandDecl{{Name: "show explicit peer probe"}}}); err != nil {
					return 1
				}
				return 0
			})
			argument := "quote\" backslash\\ newline\n"
			for _, peer := range []string{"192.0.2.81", "2001:db8::81", ""} {
				var wire json.RawMessage
				var err error
				if peer == "" {
					wire, err = engine.Execute(ctx, "show explicit peer probe", []string{argument})
				} else {
					wire, err = engine.ExecutePeer(ctx, "show explicit peer probe", []string{argument}, peer)
				}
				if err != nil {
					t.Fatal(err)
				}
				var reply struct {
					Peer     string `json:"peer"`
					Argument string `json:"argument"`
				}
				if err := json.Unmarshal(wire, &reply); err != nil {
					t.Fatal(err)
				}
				if reply.Peer != peer {
					t.Fatalf("actual SDK peer=%q, want %q", reply.Peer, peer)
				}
				if reply.Argument != argument {
					t.Fatal("actual SDK argument bytes changed")
				}
			}
		})
	}
}
