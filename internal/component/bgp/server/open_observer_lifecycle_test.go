// Design: docs/architecture/api/process-protocol.md -- OPEN observer removal lifetime.
package server

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

type openObserverSpawner struct {
	ctx context.Context
	pm  *process.ProcessManager
}

func (s *openObserverSpawner) SpawnMore(configs []plugin.PluginConfig) error {
	if s.pm == nil {
		s.pm = process.NewProcessManager(configs)
		return s.pm.StartWithContext(s.ctx)
	}
	return s.pm.StartMore(configs)
}

func (s *openObserverSpawner) GetProcessManager() any { return s.pm }

// TestOpenObserversReceiveRemovalWithoutFilterBindings drives both producers
// through real SDK startup and event delivery. Validators see OPENs regardless
// of attachments; only subscribed validators see an unattached peer's removal,
// with its generation, once. Ordinary state and non-validator subscribers gain
// nothing, and an existing state binding must not duplicate the removal.
func TestOpenObserversReceiveRemovalWithoutFilterBindings(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	var delivered [4]atomic.Int64
	var opened [4]atomic.Int64
	var generation atomic.Uint64
	var plugins []plugin.PluginConfig
	for index, name := range []string{"open-observer", "non-validator", "unsubscribed-validator", "other-peer-validator"} {
		err := registry.Register(registry.Registration{
			Name: name, Description: "OPEN observation lifecycle test",
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(name, conn)
				defer p.Close() //nolint:errcheck // Test connection teardown.
				if index != 1 {
					p.OnValidateOpen(func(*sdk.ValidateOpenInput) *sdk.ValidateOpenOutput {
						opened[index].Add(1)
						return &sdk.ValidateOpenOutput{Accept: true}
					})
				}
				if index != 2 {
					var peers []string
					if index == 3 {
						peers = []string{"192.0.2.99"}
					}
					p.SetStartupSubscriptionsIn(bgpevents.Namespace, []string{"state"}, peers, "full")
				}
				p.OnEvent(func(payload string) error {
					var event struct {
						BGP struct {
							Generation uint64 `json:"session-generation,string"`
						} `json:"bgp"`
					}
					if err := json.Unmarshal([]byte(payload), &event); err != nil {
						return err
					}
					generation.Store(event.BGP.Generation)
					delivered[index].Add(1)
					return nil
				})
				if err := p.Run(ctx, &sdk.Registration{}); err != nil {
					return 1
				}
				return 0
			},
			CLIHandler: func([]string) int { return 0 },
		})
		if err != nil {
			t.Fatal(err)
		}
		plugins = append(plugins, plugin.PluginConfig{Name: name, Internal: true, Run: "ze." + name, Encoder: plugin.EncodingJSON})
	}
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: plugins,
		ConfigNeedHolds: config.ConfigNeedHolds(map[string]any{})}, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv.SetProcessSpawner(&openObserverSpawner{ctx: ctx})
	if err := srv.StartWithContext(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Stop)
	if err := srv.WaitForStartupComplete(ctx); err != nil {
		t.Fatal(err)
	}
	peer := &plugin.PeerInfo{Name: "roleless", Address: netip.MustParseAddr("192.0.2.1"),
		PeerAS: rpc.RemoteASOf(65001), SessionGeneration: 9}
	if err := askOpenValidators(ctx, srv.ProcessManager().AllProcesses(), &rpc.ValidateOpenInput{
		Peer: peer.Name, Generation: 8}, map[string]bool{}); err != nil {
		t.Fatal(err)
	}
	onPeerStateChange(srv, peer, rpc.SessionStateUp, "")
	onPeerStateChange(srv, peer, rpc.SessionStateDown, rpc.ReasonTCPFailure)
	onPeerStateChange(srv, peer, rpc.SessionStateDown, rpc.ReasonPeerRemoved)
	if got := delivered[0].Load(); got != 1 || generation.Load() != 9 {
		t.Fatalf("unattached validator received %d events, generation %d; want one removal at 9", got, generation.Load())
	}
	for index := 1; index < len(delivered); index++ {
		if got := delivered[index].Load(); got != 0 {
			t.Errorf("process %s gained %d unauthorized state events", plugins[index].Name, got)
		}
	}
	for _, index := range []int{0, 2, 3} {
		if opened[index].Load() != 1 {
			t.Errorf("validator %s did not observe the OPEN", plugins[index].Name)
		}
	}
	srv.UpdateDeliveryGraph(bgpevents.Namespace, []pluginserver.DeliveryPeer{{Addr: peer.AddrStr(), Name: peer.Name,
		Bindings: []plugin.PeerProcessBinding{{PluginName: plugins[0].Name,
			Receive: map[string]events.Direction{bgpevents.EventState: events.DirBoth}}}}})
	onPeerStateChange(srv, peer, rpc.SessionStateDown, rpc.ReasonPeerRemoved)
	if got := delivered[0].Load(); got != 2 {
		t.Fatalf("bound validator received %d total events, want exactly two removals", got)
	}
}
