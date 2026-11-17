// Design: docs/architecture/behavior/peer-lifecycle.md -- private incoming OPEN validation.
// Related: rfc4271_incoming_listener_test.go -- real listener and useful TCP handshakes.
package reactor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/netip"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpserver "github.com/ze-software/ze/internal/component/bgp/server"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestIncomingOpenValidationIsProvisional checks that rejected and colliding
// incoming OPENs reach the real plugin callback without replacing selected state.
func TestIncomingOpenValidationIsProvisional(t *testing.T) {
	for _, bridged := range []bool{false, true} {
		for _, mismatch := range []bool{false, true} {
			name := "rpc"
			if bridged {
				name = "bridged"
			}
			if mismatch {
				name += "/role-mismatch"
			} else {
				name += "/collision"
			}
			t.Run(name, func(t *testing.T) {
				observed := &incomingOpenObservations{}
				r, peer, original, _ := incomingOpenListenerPeer(t, false, bridged, observed)
				incumbent := incomingListenerDial(t, r)
				delayOpenReadType(t, incumbent, 1)
				incomingListenerEstablish(t, peer, incumbent)
				delayOpenAwait(t, func() bool { return peer.Stats().KeepalivesReceived == 1 })
				incoming := incomingListenerDial(t, r)
				delayOpenReadType(t, incoming, 1)
				open := delayOpenRemote(0x03030303)
				want := []byte{6, 7}
				if mismatch {
					open.OptionalParams = []byte{2, 3, 9, 1, 0}
					want = []byte{2, 11}
				}
				delayOpenWrite(t, incoming, open)
				packet := delayOpenReadType(t, incoming, 3)
				if !reflect.DeepEqual(packet[message.HeaderLen:], want) {
					t.Fatalf("incoming refusal %x, want %x", packet[message.HeaderLen:], want)
				}
				var octet [1]byte
				if n, err := incoming.Read(octet[:]); n != 0 || !errors.Is(err, io.EOF) {
					t.Fatalf("incoming refusal did not close: bytes=%d error=%v", n, err)
				}
				delayOpenAwait(t, func() bool { return !peer.hasPendingConnection() })
				calls := observed.calls()
				if len(calls) != 2 {
					t.Fatalf("validation calls=%d, want incumbent plus one candidate", len(calls))
				}
				if calls[0].Provisional {
					t.Fatal("the incumbent's OPEN was not selected")
				}
				if !calls[1].Provisional {
					t.Fatal("an unselected incoming OPEN was published as selected")
				}
				if peer.currentSession() != original {
					t.Fatal("incoming refusal replaced the incumbent")
				}
				before := peer.Stats().KeepalivesReceived
				delayOpenWrite(t, incumbent, &message.Keepalive{})
				delayOpenAwait(t, func() bool { return peer.Stats().KeepalivesReceived == before+1 })
			})
		}
	}
}

// TestIncomingOpenSelectionPublishesExactPair requires the adopted connection
// to publish its retained OPEN pair once, after provisional validation.
func TestIncomingOpenSelectionPublishesExactPair(t *testing.T) {
	for _, bridged := range []bool{false, true} {
		t.Run(map[bool]string{false: "rpc", true: "bridged"}[bridged], func(t *testing.T) {
			observed := &incomingOpenObservations{}
			r, peer, original, outgoing := incomingOpenListenerPeer(t, true, bridged, observed)
			delayOpenReadType(t, outgoing, 1)
			delayOpenAwait(t, func() bool { return original.State() == fsm.StateOpenSent })
			incoming := incomingListenerDial(t, r)
			delayOpenReadType(t, incoming, 1)
			incomingListenerEstablish(t, peer, incoming)
			delayOpenAwait(t, func() bool {
				peer.mu.RLock()
				defer peer.mu.RUnlock()
				return peer.upPhaseLocked(peer.session) == sessionUpDelivered
			})
			calls := observed.calls()
			if len(calls) != 2 {
				t.Fatalf("selected candidate validation calls=%d, want provisional then selected", len(calls))
			}
			if !calls[0].Provisional {
				t.Fatal("candidate published before selection")
			}
			if calls[1].Provisional {
				t.Fatal("selected candidate never published its OPEN")
			}
			if !reflect.DeepEqual(calls[0].Local, calls[1].Local) {
				t.Fatal("selection rebuilt the local OPEN instead of retaining its proposal")
			}
			if !reflect.DeepEqual(calls[0].Remote, calls[1].Remote) {
				t.Fatal("selection changed the validated remote OPEN")
			}
			if peer.currentSession() == original {
				t.Fatal("incoming winner did not replace the old outgoing connection")
			}
		})
	}
}

// incomingOpenObservations holds the finite callback population of one test's
// two TCP connections. The SDK callback and test goroutine share it under mu.
type incomingOpenObservations struct {
	// onSelected is an optional test gate installed before callback startup.
	onSelected func()
	mu         sync.Mutex
	values     []incomingOpenObservation
}

type incomingOpenObservation struct {
	Provisional bool                    `json:"provisional"`
	Local       rpc.ValidateOpenMessage `json:"local"`
	Remote      rpc.ValidateOpenMessage `json:"remote"`
}

func (o *incomingOpenObservations) calls() []incomingOpenObservation {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]incomingOpenObservation(nil), o.values...)
}

// incomingOpenObserver starts the actual callback dispatcher. Wrapping away
// Bridger forces the same JSON RPC transport a forked plugin receives.
func incomingOpenObserver(t *testing.T, bridged bool, observed *incomingOpenObservations) func(*Peer) {
	t.Helper()
	return func(peer *Peer) {
		snapshot := registry.Snapshot()
		t.Cleanup(func() { registry.Restore(snapshot) })
		const name = "incoming-open-observer"
		err := registry.Register(registry.Registration{
			Name: name, Description: "incoming OPEN publication test",
			RunEngine: func(conn net.Conn) int {
				if !bridged {
					conn = struct{ net.Conn }{conn}
				}
				p := sdk.NewWithConn(name, conn)
				defer p.Close() //nolint:errcheck // Test-owned connection teardown.
				p.OnValidateOpen(func(input *sdk.ValidateOpenInput) *sdk.ValidateOpenOutput {
					wire, err := json.Marshal(input)
					if err != nil {
						return &sdk.ValidateOpenOutput{Reason: err.Error()}
					}
					var call incomingOpenObservation
					if err := json.Unmarshal(wire, &call); err != nil {
						return &sdk.ValidateOpenOutput{Reason: err.Error()}
					}
					observed.mu.Lock()
					observed.values = append(observed.values, call)
					observed.mu.Unlock()
					if !call.Provisional {
						if observed.onSelected != nil {
							observed.onSelected()
						}
					}
					for _, cap := range input.Remote.Capabilities {
						if cap.Code != 9 {
							continue
						}
						if cap.Hex == "00" {
							return &sdk.ValidateOpenOutput{NotifyCode: 2, NotifySubcode: 11, Reason: "test role mismatch"}
						}
					}
					return &sdk.ValidateOpenOutput{Accept: true}
				})
				if err := p.Run(t.Context(), &sdk.Registration{}); err != nil {
					return 1
				}
				return 0
			},
			CLIHandler: func([]string) int { return 0 },
		})
		if err != nil {
			t.Fatal(err)
		}
		r := peer.reactor
		srv := flowForwardPluginServer(t, r, plugin.PluginConfig{Name: name, Internal: true, Encoder: "json"})
		registered := false
		for _, proc := range srv.ProcessManager().AllProcesses() {
			if proc.Name() != name {
				continue
			}
			reg := proc.Registration()
			if reg == nil {
				t.Fatal("OPEN observer did not complete registration")
			}
			if !reg.WantsValidateOpen {
				t.Fatal("OPEN observer did not register its callback")
			}
			registered = true
		}
		if !registered {
			t.Fatal("OPEN observer process is absent after startup")
		}
		r.SetPluginServer(srv)
		r.eventDispatcher = bgpserver.NewEventDispatcher(srv)
		peer.setReconnectDelay(time.Hour, time.Hour)
	}
}

// incomingOpenListenerPeer uses the daemon's borrowed-server mode. A standalone
// reactor deliberately replaces an injected server, so it cannot test callbacks.
func incomingOpenListenerPeer(t *testing.T, outbound, bridged bool, observed *incomingOpenObservations, configure ...func(*Reactor, *Peer)) (*Reactor, *Peer, *Session, net.Conn) {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Connection = ConnectionPassive
	var listener *net.TCPListener
	var dialer *heldDialer
	if outbound {
		settings.Connection = ConnectionBoth
		var lc net.ListenConfig
		bound, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { closeConnQuietlyListener(bound) })
		var ok bool
		listener, ok = bound.(*net.TCPListener)
		if !ok {
			t.Fatal("outgoing control listener is not TCP")
		}
		addr, ok := listener.Addr().(*net.TCPAddr)
		if !ok {
			t.Fatal("outgoing control address is not TCP")
		}
		settings.Port = uint16(addr.Port)
		if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		dialer = newHeldDialer()
	}
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	if err := r.AddPeer(settings); err != nil {
		t.Fatal(err)
	}
	peer := r.Peers()[0]
	for _, apply := range configure {
		apply(r, peer)
	}
	incomingOpenObserver(t, bridged, observed)(peer)
	if dialer != nil {
		peer.SetDialer(dialer)
	}
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		r.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := r.Wait(ctx); err != nil {
			t.Errorf("borrowed incoming reactor did not stop: %v", err)
		}
	})
	if err := r.StartPeers(); err != nil {
		t.Fatal(err)
	}
	var session *Session
	delayOpenAwait(t, func() bool {
		session = peer.currentSession()
		if session == nil {
			return false
		}
		return session.State() != fsm.StateIdle
	})
	var outgoing net.Conn
	if dialer != nil {
		close(dialer.release)
		var err error
		outgoing, err = listener.Accept()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { closeConnQuietly(outgoing) })
	}
	return r, peer, session, outgoing
}
