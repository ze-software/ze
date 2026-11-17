// Design: docs/architecture/api/process-protocol.md -- portable facts across both callback transports.
package sdk

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestOriginIntakeTransportsPreservePortableEncodingFacts(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			p, engine := newTestPair(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			bridge := rpc.NewDirectBridge()
			connection := ipc.NewMuxPluginConn(engine.mux)
			if typed {
				p.bridge = bridge
				connection.SetBridge(bridge)
			}
			captured := make(chan rpc.OriginIntakeInput, 1)
			p.OnOriginIntake(func(_ context.Context, in rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
				captured <- in
				return originIntakeTestReply(in), nil
			})
			stop, err := p.startOriginIntakeWorker(ctx)
			require.NoError(t, err)
			defer func() { require.NoError(t, stop()) }()
			if typed {
				bridge.SetReady()
			}
			for _, facts := range []struct{ known, asn4 bool }{{false, false}, {true, false}, {true, true}} {
				input := originIntakeTestInput()
				data, err := json.Marshal(input)
				require.NoError(t, err)
				var outer map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(data, &outer))
				var route map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(outer["route"], &route))
				route["as-path-known"], err = json.Marshal(facts.known)
				require.NoError(t, err)
				route["as-path-asn4"], err = json.Marshal(facts.asn4)
				require.NoError(t, err)
				outer["route"], err = json.Marshal(route)
				require.NoError(t, err)
				data, err = json.Marshal(outer)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(data, &input))
				output, err := connection.SendOriginIntake(ctx, input)
				require.NoError(t, err)
				require.NotNil(t, output)
				require.True(t, output.Retained())
				require.Zero(t, output.Announced)
				var seen rpc.OriginIntakeInput
				select {
				case seen = <-captured:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				require.Equal(t, input.Route.SourceContext, seen.Route.SourceContext)
				require.Equal(t, input.Route.Attributes, seen.Route.Attributes)
				data, err = json.Marshal(seen)
				require.NoError(t, err)
				var remote struct {
					Route map[string]json.RawMessage `json:"route"`
				}
				require.NoError(t, json.Unmarshal(data, &remote))
				require.Contains(t, remote.Route, "as-path-known")
				require.Contains(t, remote.Route, "as-path-asn4")
				var known, four bool
				require.NoError(t, json.Unmarshal(remote.Route["as-path-known"], &known))
				require.NoError(t, json.Unmarshal(remote.Route["as-path-asn4"], &four))
				require.Equal(t, facts.known, known)
				require.Equal(t, facts.asn4, four)
			}
		})
	}
}
