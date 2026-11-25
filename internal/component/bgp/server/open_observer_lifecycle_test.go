// Design: docs/architecture/api/process-protocol.md -- OPEN observer removal lifetime.
package server

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

type openObserverSpawner struct {
	admissionMu sync.Mutex
	sealed      bool
	ctx         context.Context
	pm          *process.ProcessManager
}

func (s *openObserverSpawner) SpawnMore(configs []plugin.PluginConfig) error {
	s.admissionMu.Lock()
	if s.sealed {
		s.admissionMu.Unlock()
		return process.ErrDaemonStopping
	}
	first := s.pm == nil
	if first {
		s.pm = process.NewProcessManager(configs)
	}
	pm := s.pm
	s.admissionMu.Unlock()
	if first {
		return pm.StartWithContext(s.ctx)
	}
	return pm.StartMore(configs)
}

func (s *openObserverSpawner) GetProcessManager() any {
	s.admissionMu.Lock()
	defer s.admissionMu.Unlock()
	return s.pm
}

func (s *openObserverSpawner) SealProcessAdmission() func() {
	s.admissionMu.Lock()
	defer s.admissionMu.Unlock()
	s.sealed = true
	return func() {}
}

// TestOpenObserversReceiveRemovalWithoutFilterBindings drives both producers
// through real SDK startup and event delivery. Validators see OPENs regardless
// of attachments; under AC-39 / S32 a WantsValidateOpen endpoint synthesizes a
// registered bgp:state reception by default unless narrowed by its declared
// from-scope or a bus receive allow-list. An unsubscribed non-validator gains
// nothing, and an existing state binding must not duplicate the removal.
func TestOpenObserversReceiveRemovalWithoutFilterBindings(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	var delivered [5]atomic.Int64
	var opened [5]atomic.Int64
	var generation atomic.Uint64
	var plugins []plugin.PluginConfig
	for index, name := range []string{"open-observer", "non-validator", "implicit-state-validator", "other-peer-validator", "unsubscribed-non-validator"} {
		var receives []rpc.MessageReceiveDecl
		if index == 0 || index == 1 || index == 3 {
			decl := rpc.MessageReceiveDecl{Message: bgpevents.Namespace + ":" + bgpevents.EventState}
			if index == 3 {
				decl.From = []string{"/bgp/ip/192.0.2.99"}
			}
			receives = []rpc.MessageReceiveDecl{decl}
		}
		err := registry.Register(registry.Registration{
			Name: name, Description: "OPEN observation lifecycle test",
			Receives: receives,
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(name, conn)
				defer p.Close() //nolint:errcheck // Test connection teardown.
				if index == 0 || index == 2 || index == 3 {
					p.OnValidateOpen(func(*sdk.ValidateOpenInput) *sdk.ValidateOpenOutput {
						opened[index].Add(1)
						return &sdk.ValidateOpenOutput{Accept: true}
					})
				}
				if index != 4 {
					p.SetEventFormat("full")
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
				if err := p.Run(ctx, &sdk.Registration{Receives: receives}); err != nil {
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
	srv.UpdateDeliveryGraph(bgpevents.Namespace, []pluginserver.DeliveryPeer{{
		Addr:           peer.AddrStr(),
		Name:           peer.Name,
		Path:           "/bgp/peer/" + peer.Name,
		SecondaryPaths: []string{"/bgp/ip/" + peer.AddrStr()},
	}})
	if err := askOpenValidators(ctx, srv.ProcessManager().AllProcesses(), &rpc.ValidateOpenInput{
		Peer: peer.Name, Generation: 8}, map[string]bool{}); err != nil {
		t.Fatal(err)
	}
	onPeerStateChange(srv, peer, rpc.SessionStateDown, rpc.ReasonPeerRemoved)
	if got := delivered[0].Load(); got != 1 || generation.Load() != 9 {
		t.Fatalf("validator received %d events, generation %d; want one removal at 9", got, generation.Load())
	}
	if got := delivered[1].Load(); got != 1 {
		t.Fatalf("non-validator declaring bgp:state received %d events; want 1", got)
	}
	if got := delivered[2].Load(); got != 1 {
		t.Fatalf("WantsValidateOpen validator without explicit Receives received %d events; want 1 (AC-39 synthesized state)", got)
	}
	for _, index := range []int{3, 4} {
		if got := delivered[index].Load(); got != 0 {
			t.Errorf("process %s gained %d unauthorized state events", plugins[index].Name, got)
		}
	}
	for _, index := range []int{0, 2, 3} {
		if opened[index].Load() != 1 {
			t.Errorf("validator %s did not observe the OPEN", plugins[index].Name)
		}
	}
	// Narrowing open-observer and implicit-state-validator via receive > allow none stops subsequent removal delivery (AC-39).
	srv.SetBusConfig(plugin.BusConfig{
		Endpoints: map[string]plugin.BusBlock{
			plugins[0].Name: {Receive: plugin.BusDirectionBlock{AllowPresent: true, AllowNone: true}},
			plugins[2].Name: {Receive: plugin.BusDirectionBlock{AllowPresent: true, AllowNone: true}},
		},
	})
	srv.UpdateDeliveryGraph(bgpevents.Namespace, []pluginserver.DeliveryPeer{{
		Addr:           peer.AddrStr(),
		Name:           peer.Name,
		Path:           "/bgp/peer/" + peer.Name,
		SecondaryPaths: []string{"/bgp/ip/" + peer.AddrStr()},
	}})
	onPeerStateChange(srv, peer, rpc.SessionStateDown, rpc.ReasonPeerRemoved)
	for _, index := range []int{0, 2} {
		if got := delivered[index].Load(); got != 1 {
			t.Fatalf("narrowed validator %s received %d total events, want 1", plugins[index].Name, got)
		}
	}
}
