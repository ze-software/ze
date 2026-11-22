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
				p.OnExecuteCommand(func(serial, command string, args []string, peer string, _ []sdk.PipeOperation) (string, any, error) {
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

// Goal: explicit peer context and declared pipe operations both reach the real
// registered SDK callback identically over wire and bridge.
// Method: echo peer and pipe operations back from the SDK handler.
func TestExecutePeerPipeReachesActualSDK(t *testing.T) {
	for _, transport := range Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			engine := Start(t, transport, func(conn net.Conn) int {
				p := sdk.NewWithConn("explicit-command-peer-pipe-probe", conn)
				defer p.Close() //nolint:errcheck // Start owns and joins both connection ends.
				p.OnExecuteCommand(func(_ string, _ string, _ []string, peer string, pipe []sdk.PipeOperation) (string, any, error) {
					return rpc.StatusDone, struct {
						Peer string              `json:"peer"`
						Pipe []sdk.PipeOperation `json:"pipe"`
					}{Peer: peer, Pipe: pipe}, nil
				})
				if err := p.Run(ctx, &sdk.Registration{Commands: []sdk.CommandDecl{{Name: "show explicit peer pipe probe"}}}); err != nil {
					return 1
				}
				return 0
			})
			wantPipe := []rpc.PipeOperation{{Name: "received"}, {Name: "match", Args: []string{"192.0.2.0/24"}}}
			wire, err := engine.ExecutePeerPipe(ctx, "show explicit peer pipe probe", nil, "192.0.2.82", wantPipe)
			if err != nil {
				t.Fatal(err)
			}
			var reply struct {
				Peer string              `json:"peer"`
				Pipe []sdk.PipeOperation `json:"pipe"`
			}
			if err := json.Unmarshal(wire, &reply); err != nil {
				t.Fatal(err)
			}
			if reply.Peer != "192.0.2.82" {
				t.Fatalf("actual SDK peer=%q, want %q", reply.Peer, "192.0.2.82")
			}
			if len(reply.Pipe) != 2 || reply.Pipe[0].Name != "received" || reply.Pipe[1].Name != "match" || len(reply.Pipe[1].Args) != 1 || reply.Pipe[1].Args[0] != "192.0.2.0/24" {
				t.Fatalf("actual SDK pipe=%+v, want %+v", reply.Pipe, wantPipe)
			}
		})
	}
}
