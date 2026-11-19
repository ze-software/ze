//go:build ze_bgp

// Design: docs/architecture/api/birdwatcher-compat.md -- Section 9, real summary contract
// Related: server_test.go -- mockDispatch, whose summary drives the HTTP goldens

package lg

import (
	"encoding/json"
	"net/netip"
	"reflect"
	"testing"
	"time"

	// Registration loads the producer, which is found through the public RPC
	// registry below; no test-only production adapter exports its handler.
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/peer"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestMockSummaryMatchesRegisteredProducer compares the fixture's identity,
// counters and duration types with bytes emitted by the actual registered BGP
// overview. The RIB-owned additions have their own contract test, because this
// server has no running RIB plugin and the real producer must omit its counts.
func TestMockSummaryMatchesRegisteredProducer(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, &summaryContractReactor{})
	if err != nil {
		t.Fatalf("new command server: %v", err)
	}
	var overview pluginserver.Handler
	for _, registered := range pluginserver.AllBuiltinRPCs() {
		if registered.WireMethod == "ze-bgp:overview" {
			overview = registered.Handler
			break
		}
	}
	if overview == nil {
		t.Fatal("real BGP overview producer is not registered")
	}
	response, err := overview(&pluginserver.CommandContext{Server: server}, commandtest.Args())
	if err != nil {
		t.Fatalf("real overview: %v", err)
	}
	if response.Status != plugin.StatusDone {
		t.Fatalf("real overview status = %v, error %s", response.Status, response.Error)
	}
	encoded, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatalf("encode real overview: %v", err)
	}
	var produced map[string]any
	if err := json.Unmarshal(encoded, &produced); err != nil {
		t.Fatalf("decode real overview: %v", err)
	}
	fixture := mockSummaryAnswer(t)
	producedPeers := summaryPeers(produced)
	fixturePeers := summaryPeers(fixture)
	if len(producedPeers) != 1 {
		t.Fatalf("real overview peers = %d, want 1", len(producedPeers))
	}
	if len(fixturePeers) != 1 {
		t.Fatalf("fixture peers = %d, want 1", len(fixturePeers))
	}
	peer, ok := fixturePeers[0].(map[string]any)
	if !ok {
		t.Fatalf("fixture peer = %T, want object", fixturePeers[0])
	}
	// These are the only additions made by mergeRibRouteCounts when a RIB
	// exists. Keep every other field, including unrecognized extras, so the
	// comparison catches both missing producer fields and invented ones.
	for _, key := range []string{"routes-received", "routes-accepted", "routes-sent"} {
		delete(peer, key)
	}
	if !reflect.DeepEqual(fixture, produced) {
		t.Errorf("mock summary differs from registered producer\nmock: %#v\nreal: %#v", fixture, produced)
	}
}

// summaryContractReactor supplies the two snapshots the overview consumes.
// All other lifecycle methods are deliberately unimplemented: this test never
// starts a reactor, listener or plugin, and an unexpected call must fail.
type summaryContractReactor struct {
	plugin.ReactorLifecycle
}

func (*summaryContractReactor) Peers() []plugin.PeerInfo {
	return []plugin.PeerInfo{{
		Name: "peer1", Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS: rpc.RemoteASOf(65001), PeerType: "external",
		State: plugin.PeerStateEstablished, Uptime: time.Hour,
		LastStateChange: time.Date(2026, time.January, 15, 10, 0, 0, 0, time.UTC),
	}}
}

func (*summaryContractReactor) Stats() plugin.ReactorStats {
	return plugin.ReactorStats{RouterID: 0x01020304, LocalAS: 65000, Uptime: time.Hour, PeerCount: 1}
}
