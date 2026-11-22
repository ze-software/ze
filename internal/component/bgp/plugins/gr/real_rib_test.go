// Design: docs/guide/graceful-restart.md -- registered received-route lifecycle.
// Related: internal/test/pluginengine/pluginengine.go -- actual SDK startup and source leases.
package gr

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rib"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// realGRRIB runs the registered RIB engine on its actual SDK transport. Each
// received physical source has a fixture-issued receipt, which the RIB MUST
// acquire and retain through the real SDK before physical cleanup can use it.
// Tests MUST let t.Cleanup stop GR timers, release RIB routes and join the SDK.
type realGRRIB struct {
	t       *testing.T
	ctx     context.Context
	engine  *pluginengine.Engine
	source  grSource
	session uint64
	receipt uint64
	message uint64
}

func newGRWithRealRIB(t *testing.T) (*grPlugin, *realGRRIB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	registered := registry.Lookup("bgp-rib")
	require.NotNil(t, registered)
	engine := pluginengine.Start(t, pluginengine.TransportWire, registered.RunEngine)
	source := grFixtureSource(t, testPeer, 701, 41)
	r := &realGRRIB{t: t, ctx: ctx, engine: engine, source: source, session: 141}
	r.receipt = engine.IssueSource(source.peer(), source.owner)
	gp, _ := newRecordedGRPlugin()
	gp.recordSentLLGR(source, nil)
	// The RIB's own answer reaches the plugin, which reads the retain-routes
	// one. A test that sets dispatchHook afterwards takes the commands instead.
	gp.dispatchHook = nil
	gp.answerHook = func(command string, args ...string) json.RawMessage { return r.command(command, args...) }
	t.Cleanup(func() {
		gp.state.removePeer(source.grPeer)
		// This explicit operator cleanup is deliberately peer-wide within the
		// isolated fixture. GR itself always uses original physical commands.
		r.command("request bgp rib release-routes", testPeer)
	})
	// Receive-only attachment records the original live source, without
	// manufacturing an output recipient or an initial replay obligation.
	r.up(testPeer)
	return gp, r
}

func (r *realGRRIB) command(command string, args ...string) json.RawMessage {
	r.t.Helper()
	data, err := r.engine.ExecutePeer(r.ctx, command, args, "*")
	require.NoError(r.t, err)
	return data
}

func (r *realGRRIB) event(event map[string]any) {
	r.t.Helper()
	encoded, err := json.Marshal(event)
	require.NoError(r.t, err)
	require.NoError(r.t, r.engine.Deliver(r.ctx, string(encoded)))
}

func (r *realGRRIB) received(wire, attributes string) {
	r.t.Helper()
	r.receivedFamily(family.IPv4Unicast, wire, attributes, "192.0.2.1")
}

// receivedIPv6 supplies a received path, rather than the unrelated manual
// forwarding slot whose unrecorded source cannot be retained by GR.
func (r *realGRRIB) receivedIPv6(attributes string) {
	r.t.Helper()
	r.receivedFamily(family.IPv6Unicast, "2020010db8", attributes, "::1")
}

func (r *realGRRIB) receivedFamily(selected family.Family, wire, attributes, nextHop string) {
	r.t.Helper()
	raw, err := hex.DecodeString(wire)
	require.NoError(r.t, err)
	items, err := nlrisplit.Split(selected, raw, false)
	require.NoError(r.t, err)
	prefixes := make([]string, 0, len(items))
	for _, item := range items {
		prefix, ok := nlri.WirePrefixToKey(item, selected)
		require.True(r.t, ok)
		prefixes = append(prefixes, prefix.String())
	}
	r.message++
	r.event(map[string]any{
		"type": "update", "peer-owner": r.source.owner,
		"connection-id": strconv.FormatUint(r.source.connectionID, 10),
		"msg-id":        r.message, "source-receipt": r.receipt, "source-policy-epoch": uint64(5),
		"peer": map[string]any{"remote": map[string]any{"address": r.source.peer(), "as": 65001}, "local": map[string]any{"as": 65000}},
		"raw":  map[string]any{"attributes": attributes, "nlri": map[string]string{selected.String(): wire}},
		"nlri": map[string]any{selected.String(): []map[string]any{{"action": "add", "next-hop": nextHop, "nlri": prefixes}}},
	})
	for _, prefix := range prefixes {
		require.Len(r.t, r.routes(prefix), 1, "received UPDATE must enter actual storage before lifecycle assertions")
	}
}

func (r *realGRRIB) state(state, reason string) {
	r.event(map[string]any{
		"type": "state", "state": state, "reason": reason,
		"peer-owner": r.source.owner, "connection-id": strconv.FormatUint(r.source.connectionID, 10),
		"destination-session": strconv.FormatUint(r.session, 10),
		"peer":                map[string]any{"remote": map[string]any{"address": r.source.peer(), "as": 65001}, "local": map[string]any{"as": 65000}},
	})
}

func (r *realGRRIB) up(peer string) {
	r.t.Helper()
	require.Equal(r.t, r.source.peer(), peer, "UP must name the fixture's explicit source")
	r.state("up", rpc.ReasonAttached)
}

func (r *realGRRIB) down() { r.state("down", rpc.ReasonTCPFailure) }

// reconnect installs exactly the explicitly supplied successor actor. The
// caller MUST give its newly captured physical source and independent UP token;
// this fixture MUST issue a fresh receipt before delivering any successor input.
func (r *realGRRIB) reconnect(source grSource, session uint64) {
	r.t.Helper()
	require.Equal(r.t, r.source.owner, source.owner)
	require.NotEqual(r.t, r.source.connectionID, source.connectionID)
	require.Positive(r.t, session)
	require.NotEqual(r.t, r.session, session)
	r.source, r.session = source, session
	r.receipt = r.engine.IssueSource(source.peer(), source.owner)
	r.up(source.peer())
}

func (r *realGRRIB) routes(prefix string) []map[string]any {
	r.t.Helper()
	var data struct {
		Routes []map[string]any `json:"routes"`
	}
	raw, err := r.engine.ExecutePeerPipe(r.ctx, "show bgp rib", nil, "*", []rpc.PipeOperation{
		{Name: "received"},
		{Name: "prefix", Args: []string{prefix}},
	})
	require.NoError(r.t, err)
	require.NoError(r.t, json.Unmarshal(raw, &data))
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
