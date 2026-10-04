package gr

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rib"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// realGRRIB runs the registered RIB engine and consumes GR's production command
// boundary. Assertions read the RIB's stored routes, not copies of those commands.
type realGRRIB struct {
	t   *testing.T
	ctx context.Context
	mux *rpc.MuxConn
}

func newGRWithRealRIB(t *testing.T) (*grPlugin, *realGRRIB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	engine, plugin := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engine, engine))
	r := &realGRRIB{t: t, ctx: ctx, mux: mux}
	finished := make(chan struct{})
	go func() { defer close(finished); registry.Lookup("bgp-rib").RunEngine(plugin) }()
	t.Cleanup(func() {
		cancel()
		if err := mux.Close(); err != nil {
			t.Errorf("close RIB mux: %v", err)
		}
		if err := plugin.Close(); err != nil {
			t.Errorf("close RIB plugin pipe: %v", err)
		}
		<-finished
	})
	next := func() *rpc.Request {
		select {
		case request := <-mux.Requests():
			require.NotNil(t, request)
			return request
		case <-ctx.Done():
			t.Fatal("RIB engine startup timed out")
			return nil
		}
	}
	request := next()
	require.NoError(t, mux.SendOK(ctx, request.ID))
	_, err := mux.CallRPC(ctx, "ze-plugin-callback:configure", &rpc.ConfigureInput{})
	require.NoError(t, err)
	request = next()
	require.NoError(t, mux.SendOK(ctx, request.ID))
	_, err = mux.CallRPC(ctx, "ze-plugin-callback:share-registry", &rpc.ShareRegistryInput{})
	require.NoError(t, err)
	request = next()
	require.NoError(t, mux.SendOK(ctx, request.ID))
	go func() {
		for request := range mux.Requests() {
			// No transport destination is installed in this received-store test.
			// Runtime notifications can be acknowledged; route state is read below.
			_ = mux.SendOK(ctx, request.ID)
		}
	}()
	gp, _ := newRecordedGRPlugin()
	gp.dispatchHook = func(command string, args ...string) {
		r.command(command, args...)
	}
	t.Cleanup(func() {
		gp.state.removePeer(testPeer)
		r.command("request bgp rib release-routes", testPeer)
	})
	// Received UPDATEs belong to an established session. RIB tears down on
	// the UP->DOWN edge, not an initial/repeated DOWN notification.
	r.up(testPeer)
	return gp, r
}

func (r *realGRRIB) command(command string, args ...string) json.RawMessage {
	r.t.Helper()
	answer, err := r.mux.CallAnswer(r.ctx, "ze-plugin-callback:execute-command", &rpc.ExecuteCommandInput{Command: command, Peer: "*", Args: args})
	require.NoError(r.t, err)
	data, err := rpc.CollapseAnswer(answer)
	require.NoError(r.t, err)
	require.NoError(r.t, answer.Err())
	require.Equal(r.t, rpc.VerdictDone, answer.Verdict(), answer.Message())
	return data
}

func (r *realGRRIB) event(event map[string]any) {
	r.t.Helper()
	encoded, err := json.Marshal(event)
	require.NoError(r.t, err)
	_, err = r.mux.CallRPC(r.ctx, "ze-plugin-callback:deliver-event", &rpc.DeliverEventInput{Event: string(encoded)})
	require.NoError(r.t, err)
}

func (r *realGRRIB) received(wire, attributes string) {
	r.t.Helper()
	raw, err := hex.DecodeString(wire)
	require.NoError(r.t, err)
	items, err := nlrisplit.Split(family.IPv4Unicast, raw, false)
	require.NoError(r.t, err)
	prefixes := make([]string, 0, len(items))
	for _, item := range items {
		prefix, ok := nlri.WirePrefixToKey(item, family.IPv4Unicast)
		require.True(r.t, ok)
		prefixes = append(prefixes, prefix.String())
	}
	// A format=full UPDATE carries both decoded family operations and raw
	// bytes. The consumer intentionally admits no route without an operation.
	r.event(map[string]any{
		"type": "update",
		"peer": map[string]any{"remote": map[string]any{"address": testPeer}},
		"raw":  map[string]any{"attributes": attributes, "nlri": map[string]string{"ipv4/unicast": wire}},
		"nlri": map[string]any{"ipv4/unicast": []map[string]any{{"action": "add", "next-hop": "192.0.2.1", "nlri": prefixes}}},
	})
	for _, prefix := range prefixes {
		require.Len(r.t, r.routes(prefix), 1, "received UPDATE must enter actual storage before lifecycle assertions")
	}
}

func (r *realGRRIB) up(peer string) {
	r.event(map[string]any{"type": "state", "state": "up", "peer": map[string]any{"remote": map[string]any{"address": peer}}})
}

func (r *realGRRIB) down() {
	r.event(map[string]any{"type": "state", "state": "down", "peer": map[string]any{"remote": map[string]any{"address": testPeer}}})
}

func (r *realGRRIB) routes(prefix string) []map[string]any {
	r.t.Helper()
	var data struct {
		Routes []map[string]any `json:"routes"`
	}
	require.NoError(r.t, json.Unmarshal(r.command("show bgp rib", "received", "prefix", prefix), &data))
	return data.Routes
}

func (r *realGRRIB) requireStale(prefix string, level int) {
	r.t.Helper()
	routes := r.routes(prefix)
	require.Len(r.t, routes, 1, prefix)
	require.Equal(r.t, float64(level), routes[0]["stale-level"], prefix)
}

const grReceivedAttrs = "40010100400200400304c0000201"

// Optional attributes are omitted when absent; when present, received rows
// expose community.value with RFC 4271 flags alongside it. Parse the values
// semantically rather than pinning their display notation.
func (r *realGRRIB) communities(prefix string) []uint32 {
	r.t.Helper()
	routes := r.routes(prefix)
	require.Len(r.t, routes, 1, prefix)
	raw, present := routes[0]["community"]
	if !present {
		return nil
	}
	wrapped, ok := raw.(map[string]any)
	require.True(r.t, ok, "stored community must have an attribute envelope")
	values, ok := wrapped["value"].([]any)
	require.True(r.t, ok, "stored community.value must be an array")
	communities := make([]uint32, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		require.True(r.t, ok, "stored community values must be strings")
		community, err := attribute.ParseCommunity(text)
		require.NoError(r.t, err)
		communities = append(communities, community)
	}
	return communities
}
