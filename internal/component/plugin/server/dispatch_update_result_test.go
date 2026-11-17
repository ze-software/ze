// Design: docs/architecture/api/process-protocol.md -- full update-route results.
package server

import (
	"context"
	"errors"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// These are result transport fixtures, not engine-issued origin authority.
func updateResultReceipt() rpc.RouteAdmission {
	return rpc.RouteAdmission{Family: "ipv4/unicast", KeyNLRI: []byte{24, 10, 0, 0},
		Destination: "192.0.2.7", PeerOwner: 17, Generation: 3,
		Outcome: rpc.RouteAdmissionRetained, Disposition: rpc.RouteAdmissionSuppressedSelection}
}

func updateResultDisposition(disposition rpc.RouteAdmissionDisposition) rpc.RouteAdmission {
	result := updateResultReceipt()
	result.Disposition = disposition
	return result
}

func updateResultRefused() rpc.RouteAdmission {
	result := updateResultReceipt()
	result.Outcome = rpc.RouteAdmissionRefused
	result.Generation = 0
	result.Disposition = rpc.RouteAdmissionDispositionUnspecified
	result.Reason = "origin admission denied"
	return result
}

// The same registered operation returns native and real framed SDK receipts,
// including retained zero-output admission, partial failure, and direct counts.
func TestUpdateRouteAdmissionResultTransportParity(t *testing.T) {
	for _, typed := range []bool{false, true} {
		name := "socket"
		if typed {
			name = "typed"
		}
		t.Run(name, func(t *testing.T) {
			for _, test := range []struct {
				name     string
				response *plugin.Response
				cause    error
				want     rpc.UpdateRouteOutput
				failure  string
			}{
				{name: "retained", response: &plugin.Response{Status: plugin.StatusDone,
					Data: &plugin.RouteResult{Admissions: []rpc.RouteAdmission{updateResultReceipt()}}},
					want: rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{updateResultReceipt()}}},
				{name: "export-suppressed", response: &plugin.Response{Status: plugin.StatusDone,
					Data: &plugin.RouteResult{Admissions: []rpc.RouteAdmission{updateResultDisposition(rpc.RouteAdmissionSuppressedExport)}}},
					want: rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{updateResultDisposition(rpc.RouteAdmissionSuppressedExport)}}},
				{name: "selected", response: &plugin.Response{Status: plugin.StatusDone,
					Data: &plugin.RouteResult{Announced: 1, Admissions: []rpc.RouteAdmission{updateResultDisposition(rpc.RouteAdmissionSelected)}}},
					want: rpc.UpdateRouteOutput{Announced: 1, Admissions: []rpc.RouteAdmission{updateResultDisposition(rpc.RouteAdmissionSelected)}}},
				{name: "refused", response: &plugin.Response{Status: plugin.StatusDone,
					Data: &plugin.RouteResult{Admissions: []rpc.RouteAdmission{updateResultRefused()}}},
					want: rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{updateResultRefused()}}, failure: "origin admission denied"},
				{name: "direct-no-rib", response: &plugin.Response{Status: plugin.StatusDone,
					Data: &plugin.RouteResult{Announced: 1}}, want: rpc.UpdateRouteOutput{Announced: 1}},
				{name: "partial-failure", response: &plugin.Response{Status: plugin.StatusError,
					Data:  &plugin.RouteResult{Announced: 2, Withdrawn: 1, Warnings: []string{"withdrawal withheld"}},
					Error: "admission refused"}, cause: errors.New("admission refused"),
					want: rpc.UpdateRouteOutput{Announced: 2, Withdrawn: 1,
						Warnings: []string{"withdrawal withheld"}, Failure: "admission refused"}, failure: "admission refused"},
				{name: "warning", response: &plugin.Response{Status: plugin.StatusWarning,
					Data: plugin.Map{"message": "no negotiated BGP peer"}},
					want: rpc.UpdateRouteOutput{Warnings: []string{"no negotiated BGP peer"}}},
			} {
				t.Run(test.name, func(t *testing.T) {
					d := NewDispatcher()
					if err := d.Register("send bgp", func(ctx *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
						if _, named := ctx.Sender.Process(); !named {
							t.Error("route sender was lost")
						}
						return test.response, test.cause
					}, "send bgp"); err != nil {
						t.Fatal(err)
					}
					s := &Server{subscriptions: newSubscriptionManager(), dispatcher: d}
					s.ctx, s.cancel = context.WithCancel(t.Context())
					defer s.cancel()
					proc := process.NewProcess(plugin.PluginConfig{Name: "origin-result"})
					client, engine := net.Pipe()
					var p *sdk.Plugin
					if typed {
						bridge := rpc.NewDirectBridge()
						for i := range engineOps {
							if engineOps[i].typedWire != nil {
								engineOps[i].typedWire(s, proc, bridge)
							}
						}
						bridge.SetReady()
						p = sdk.NewWithConn("origin-result", rpc.NewBridgedConn(client, bridge))
					} else {
						p = sdk.NewWithConn("origin-result", client)
						conn := plugipc.NewPluginConn(engine, engine)
						done := make(chan struct{})
						go func() {
							defer close(done)
							for {
								request, err := conn.ReadRequest(s.ctx)
								if err != nil {
									return
								}
								s.dispatchPluginRPC(proc, conn, request)
							}
						}()
						t.Cleanup(func() { <-done })
					}
					t.Cleanup(func() {
						if err := p.Close(); err != nil {
							t.Error(err)
						}
						if err := engine.Close(); err != nil {
							t.Error(err)
						}
					})
					got, err := p.UpdateRouteSelResult(t.Context(), selector.ParseDefault("192.0.2.7"), "route")
					if test.failure == "" {
						if err != nil {
							t.Fatal(err)
						}
					} else if err == nil || !strings.Contains(err.Error(), test.failure) {
						t.Fatalf("failure = %v", err)
					}
					if !reflect.DeepEqual(got, &test.want) {
						t.Fatalf("result = %+v, want %+v", got, test.want)
					}
					announced, withdrawn, oldErr := p.UpdateRouteSel(t.Context(), selector.ParseDefault("192.0.2.7"), "route")
					if announced != test.want.Announced || withdrawn != test.want.Withdrawn {
						t.Fatalf("legacy counts = %d/%d", announced, withdrawn)
					}
					if test.failure == "" {
						if oldErr != nil {
							t.Fatal(oldErr)
						}
					} else if oldErr == nil {
						t.Fatal("legacy wrapper hid failure")
					}
				})
			}
		})
	}
}
