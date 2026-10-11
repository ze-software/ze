package server

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	pluginrpc "github.com/ze-software/ze/pkg/plugin/rpc"
)

// payloadResponder answers delivery RPCs and records "<name>:<payload>" for
// each, so a test can read what each process was told and in which order.
func payloadResponder(ctx context.Context, conn *plugipc.PluginConn, rec *barrierReactor, name string) {
	for {
		req, err := conn.ReadRequest(ctx)
		if err != nil {
			return
		}
		rec.record("deliver:" + name + ":" + string(req.Params))
		if err := conn.SendResult(ctx, req.ID, nil); err != nil {
			return
		}
	}
}

// subscribeNegotiated registers proc for negotiated events.
func subscribeNegotiated(srv *pluginserver.Server, proc *process.Process) {
	srv.Subscriptions().Add(proc, &pluginserver.Subscription{
		Namespace: events.LookupNamespaceID("bgp"),
		EventType: events.LookupEventTypeID(bgpevents.EventNegotiated),
		Direction: events.DirUnspecified,
	})
}

// deliveriesTo returns the recorded payloads delivered to name, in order.
func deliveriesTo(steps []string, name string) []string {
	var out []string
	for _, step := range steps {
		if payload, ok := strings.CutPrefix(step, "deliver:"+name+":"); ok {
			out = append(out, payload)
		}
	}
	return out
}

// TestOnPeerBoundTargetsNamedProcesses proves the targeted up of the ordered
// bind delivery: the named process, absent from the graph, is told the peer is
// up with reason attached, then the negotiated event when granted it; a
// process the graph feeds this peer is told nothing (AC-8, "no other process
// receives either").
func TestOnPeerBoundTargetsNamedProcesses(t *testing.T) {
	srv, rec := newBarrierTestServer(t)

	bound, boundConn := newTestProcWithConn(t, "monitor")
	subscribeState(srv, bound)
	subscribeNegotiated(srv, bound)
	go payloadResponder(t.Context(), boundConn, rec, "monitor")

	bystander, bystanderConn := newTestProcWithConn(t, "bystander")
	subscribeState(srv, bystander)
	subscribeNegotiated(srv, bystander)
	go payloadResponder(t.Context(), bystanderConn, rec, "bystander")

	attachForDelivery(t, srv, testPeerInfo(), "bystander")

	onPeerBound(srv, format.NewJSONEncoder("6.0.0"), testPeerInfo(), format.DecodedNegotiated{}, []string{"monitor"}, []string{"monitor"})

	steps := rec.recorded()
	got := deliveriesTo(steps, "monitor")
	require.Len(t, got, 2, "an up, then the negotiated event")
	assert.Contains(t, got[0], "up")
	assert.Contains(t, got[0], pluginrpc.ReasonAttached, "the up carries reason attached")
	assert.Contains(t, got[1], "negotiated", "the negotiated event follows the up")
	assert.Empty(t, deliveriesTo(steps, "bystander"), "a process the bind did not name is told nothing")

	// Not granted the negotiated event: the up alone.
	rec.mu.Lock()
	rec.steps = nil
	rec.mu.Unlock()
	onPeerBound(srv, format.NewJSONEncoder("6.0.0"), testPeerInfo(), format.DecodedNegotiated{}, []string{"monitor"}, nil)
	require.Len(t, deliveriesTo(rec.recorded(), "monitor"), 1)
}

// TestPeerBoundCarriesNoReplayOrBarrier proves AC-16 at the plugin server: a
// targeted up to a process that declares the peer-up barrier arms no barrier,
// and the claimed roles on it are judged over every process the peer feeds,
// the graph's and the bound ones together.
func TestPeerBoundCarriesNoReplayOrBarrier(t *testing.T) {
	barrierName, _ := registerBarrierTestPlugins(t)
	srv, rec := newBarrierTestServer(t)

	proc, conn := newTestProcWithConn(t, barrierName)
	subscribeState(srv, proc)
	go payloadResponder(t.Context(), conn, rec, barrierName)

	onPeerBound(srv, format.NewJSONEncoder("6.0.0"), testPeerInfo(), nil, []string{barrierName}, nil)

	steps := rec.recorded()
	require.Len(t, deliveriesTo(steps, barrierName), 1, "the up is delivered")
	for _, step := range steps {
		assert.False(t, strings.HasPrefix(step, "arm:"), "a targeted up arms no peer-up barrier: %s", step)
		assert.False(t, strings.HasPrefix(step, "ack:"), "and acknowledges none: %s", step)
	}

	graphFed := process.NewProcess(plugin.PluginConfig{Name: "graph-fed"})
	boundProc := process.NewProcess(plugin.PluginConfig{Name: "bound"})
	fed := peerBoundFedSet([]*process.Process{graphFed}, []*process.Process{boundProc, graphFed})
	assert.Equal(t, []*process.Process{graphFed, boundProc}, fed,
		"the fed set is the graph's processes and the bound ones, each once")
}
