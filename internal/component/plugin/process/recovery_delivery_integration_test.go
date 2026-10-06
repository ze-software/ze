// Design: docs/architecture/plugin/rib-storage-design.md -- source-DOWN recovery and sent ordering.
// Related: recovery_delivery_export_test.go -- test-only FIFO gate, no replacement handler.
package process_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgprib "github.com/ze-software/ze/internal/component/bgp/plugins/rib"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginmgr "github.com/ze-software/ze/internal/component/plugin/manager"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRecoveryQueuedDeliveryTransport proves causal sent ownership with real
// registered RIB handlers, native UpdateRouteWithMeta, and recipient TCP bytes.
// IPC uses the real RIB runner with an unbridged connection: both sent events
// and recovery commands cross JSON RPC. It is not a fake recovery provider.
//
// MUTATION: Remove only recoverNLRIBatch's Process.DrainEventsApplied call. The newer
// owner's held sent event cannot reach the RIB, while lookup can still run. The
// test waits for that real lookup's completion before releasing the event; TCP
// then contains the older, better survivor's attributes after the newer owner.
// This is a semantic failure, not a scheduling timeout. The unheld controls use
// the same producers. The unrelated-prefix case also sends after barrier
// admission, forcing the destination sequence conflict to resolve afresh.
func TestRecoveryQueuedDeliveryTransport(t *testing.T) {
	for _, mode := range []string{"direct", "ipc"} {
		for _, target := range []bool{true, false} {
			name := "unrelated-prefix"
			if target {
				name = "newer-target-owner"
			}
			for _, delayed := range []bool{false, true} {
				control := "control"
				if delayed {
					control = "held-sent"
				}
				t.Run(mode+"/"+name+"/"+control, func(t *testing.T) {
					recoveryDeliveryScenario(t, mode, target, delayed)
				})
			}
		}
	}
}

func recoveryDeliveryScenario(t *testing.T, mode string, target, delayed bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	lookup := make(chan struct{})
	var ipcCommands atomic.Int32
	if mode == "ipc" {
		reg := registry.Lookup("bgp-rib")
		original := reg.RunEngine
		reg.RunEngine = func(conn net.Conn) int {
			return original(&recoveryIPCConn{Conn: conn, lookup: lookup, commands: &ipcCommands})
		}
		t.Cleanup(func() { reg.RunEngine = original })
	}
	r, srv, peers := recoveryDeliveryRouter(t, ctx)
	owner := ribevents.RecoveryProvider()
	if owner == nil {
		t.Fatal("registered RIB did not publish its recovery producer")
	}
	ribProcess := srv.ProcessManager().GetProcess("bgp-rib")
	if mode == "ipc" {
		if ribProcess.HasStructuredHandler() || ribProcess.Bridge().Ready() {
			t.Fatal("IPC fixture unexpectedly activated DirectBridge")
		}
		owner.Close()
	} else {
		if !ribProcess.HasStructuredHandler() {
			t.Fatal("direct fixture has no real structured RIB handler")
		}
		var queried sync.Once
		observed := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryRoute, error) {
			queried.Do(func() { close(lookup) })
			return owner.Lookup(request)
		})
		t.Cleanup(observed.Close)
	}

	// Observe the received generation before ordinary consumer acknowledgements
	// release its cache entry. Cache residency is not the source-DOWN cut.
	var receivedCut atomic.Uint64
	gate := process.GateRecoveryDelivery(t, ctx, ribProcess, func(delivery process.EventDelivery) bool {
		if id, ok := recoveryReceivedFrom(delivery, "127.0.0.2"); ok {
			receivedCut.Store(id)
		}
		return recoverySentFrom(delivery, "127.0.0.1", "127.0.0.4")
	})
	t.Cleanup(func() {
		gate.Release()
		r.Stop()
		srv.Stop()
		join, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := gate.Wait(join); err != nil {
			t.Errorf("delivery proxy did not join: %v", err)
		}
	})

	// TCP is the only route-state producer. Survivor .3 has a shorter path
	// than .4; .2 is advertised last so it owns the original sent identity.
	recoverySend(t, peers[2].conn, recoveryAnnouncement(114, 3, 2))
	recoveryWaitAnnouncement(t, ctx, peers[0], 114, 3)
	recoveryStored(t, ctx, "127.0.0.3", 114)
	recoverySend(t, peers[1].conn, recoveryAnnouncement(114, 2, 1))
	recoveryWaitAnnouncement(t, ctx, peers[0], 114, 2)
	recoveryStored(t, ctx, "127.0.0.2", 114)
	if err := ribProcess.DrainEvents(ctx); err != nil {
		t.Fatal(err)
	}
	recoveryAwait(t, ctx, gate.Barriers, "initial received and sent event drain")
	cut := receivedCut.Load()
	if cut == 0 {
		t.Fatal("received source UPDATE carried no message ID")
	}
	if !delayed {
		gate.Release()
	}
	prefix := byte(115)
	if target {
		prefix = 114
	}
	recoverySend(t, peers[3].conn, recoveryAnnouncement(prefix, 4, 3))
	recoveryWaitAnnouncement(t, ctx, peers[0], prefix, 4)
	recoveryAwait(t, ctx, gate.Held, "real sent event before RIB application")
	if !delayed {
		if err := ribProcess.DrainEvents(ctx); err != nil {
			t.Fatal(err)
		}
		// Discard only the control's explicitly requested barrier observation.
		recoveryAwait(t, ctx, gate.Barriers, "control sent-event drain")
	}
	select {
	case <-lookup:
		t.Fatal("recovery lookup observed before the native source-DOWN command")
	default:
	}

	// Use the production SDK API and sender grant, not recoverNLRIBatch or a
	// manufactured NLRIBatch. This is the native source-DOWN command emitted
	// by RS, with the actual received-message cut captured before newer output.
	bridge := srv.ProcessManager().GetProcess("bgp-rs").Bridge()
	client, unused := net.Pipe()
	api := sdk.NewWithConn("bgp-rs", rpc.NewBridgedConn(client, bridge))
	t.Cleanup(func() {
		client.Close() //nolint:errcheck // Both unused test pipe ends are cleanup only; the shared bridge belongs to RS.
		unused.Close() //nolint:errcheck // No SDK Close: it would close the live RS bridge.
	})
	completed := make(chan error, 1)
	go func() {
		_, _, err := api.UpdateRouteWithMeta(ctx, "127.0.0.1", "update hex nlri ipv4/unicast del 18cb0072",
			map[string]any{"recovery-source": "127.0.0.2", "recovery-cut": strconv.FormatUint(cut, 10)})
		completed <- err
	}()
	joined := false
	defer func() {
		gate.Release()
		cancel()
		if !joined {
			select {
			case <-completed:
			case <-time.After(5 * time.Second):
				t.Error("recovery command did not join after cancellation")
			}
		}
	}()
	if delayed {
		select {
		case <-gate.Barriers:
			if !target {
				// RS's real direct-write rail can emit while RIB delivery is
				// held. This is after snapshot/barrier, not before recovery.
				recoverySend(t, peers[2].conn, recoveryAnnouncement(116, 3, 2))
				recoveryWaitAnnouncement(t, ctx, peers[0], 116, 3)
			}
			gate.Release()
		case <-lookup:
			// The omission mutant reaches the real producer before the held
			// sent event. Leave it held until the final socket rail finishes.
			if err := recoveryResult(t, ctx, completed); err != nil {
				t.Fatal(err)
			}
			joined = true
			gate.Release()
		case <-ctx.Done():
			t.Fatal("neither causal barrier nor real lookup became reachable", ctx.Err())
		}
	}
	if !joined {
		if err := recoveryResult(t, ctx, completed); err != nil {
			t.Fatal(err)
		}
		joined = true
	}
	if err := ribProcess.DrainEvents(ctx); err != nil {
		t.Fatal(err)
	}
	if mode == "ipc" && ipcCommands.Load() == 0 {
		t.Fatal("recovery did not cross the real external JSON command transport")
	}
	if !target {
		recoveryWaitAnnouncement(t, ctx, peers[0], 114, 3)
	}

	// A fresh unrelated native UPDATE is a TCP fence behind completed recovery.
	// Reading it drains every earlier byte, so a forbidden overwrite cannot
	// escape a nonblocking channel assertion or a too-short observation window.
	recoverySend(t, peers[2].conn, recoveryAnnouncement(117, 3, 2))
	for {
		update := recoveryNextUpdate(t, ctx, peers[0])
		if bytes.Equal(update.NLRI, []byte{24, 203, 0, 117}) {
			break
		}
		if target && bytes.Equal(update.NLRI, []byte{24, 203, 0, 114}) {
			t.Errorf("old source DOWN overwrote the newer target owner on TCP: attrs=%x", update.PathAttributes)
		}
		if bytes.Equal(update.WithdrawnRoutes, []byte{24, 203, 0, 114}) {
			t.Error("surviving target was withdrawn on TCP")
		}
	}
}

// recoveryIPCConn deliberately hides rpc.Bridger. Read observes complete RPC
// execute-command frames going into the registered RIB SDK, never generates an
// answer, and leaves every byte unchanged. The startup share-registry also names
// the recovery command: a substring match would mistake that for a live lookup
// and wait for recovery while its required sent-event barrier remained held.
type recoveryIPCConn struct {
	net.Conn
	lookup   chan<- struct{}
	commands *atomic.Int32
	window   []byte
	queried  sync.Once
}

func (conn *recoveryIPCConn) Read(buf []byte) (int, error) {
	n, err := conn.Conn.Read(buf)
	conn.window = append(conn.window, buf[:n]...)
	pending := conn.window
	for len(pending) != 0 {
		advance, frame, frameErr := rpc.ScanAnswerLines(pending, false)
		if frameErr != nil {
			return n, fmt.Errorf("recovery IPC observer framing: %w", frameErr)
		}
		if advance == 0 {
			if len(pending) > rpc.MaxMessageSize {
				return n, fmt.Errorf("recovery IPC observer frame exceeds %d bytes", rpc.MaxMessageSize)
			}
			break
		}
		if len(frame) > rpc.MaxMessageSize {
			return n, fmt.Errorf("recovery IPC observer frame exceeds %d bytes", rpc.MaxMessageSize)
		}
		pending = pending[advance:]
		_, method, params, parseErr := rpc.ParseLine(frame)
		if parseErr != nil {
			return n, fmt.Errorf("recovery IPC observer request: %w", parseErr)
		}
		if method != "ze-plugin-callback:execute-command" {
			continue
		}
		var input rpc.ExecuteCommandInput
		if decodeErr := json.Unmarshal(params, &input); decodeErr != nil {
			return n, fmt.Errorf("recovery IPC observer command: %w", decodeErr)
		}
		if input.Command != "request bgp rib recovery" {
			continue
		}
		conn.commands.Add(1)
		conn.queried.Do(func() { close(conn.lookup) })
	}
	conn.window = conn.window[:copy(conn.window, pending)]
	return n, err
}

// TestRecoveryIPCConnObservesCommands proves the passive observer distinguishes
// startup declarations and payload mentions from complete recovery requests.
// Fragmented and coalesced reads must preserve bytes and count each command once.
func TestRecoveryIPCConnObservesCommands(t *testing.T) {
	request := func(id uint64, method string, input any) []byte {
		t.Helper()
		params, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		return append(rpc.AppendRequest(nil, id, method, params), '\n')
	}
	registryFrame := request(1, "ze-plugin-callback:share-registry", rpc.ShareRegistryInput{
		Commands: []rpc.RegistryCommand{{Name: "request bgp rib recovery", Plugin: "bgp-rib"}},
	})
	unrelatedFrame := request(2, "ze-plugin-callback:execute-command", rpc.ExecuteCommandInput{
		Command: "request bgp rib recovery-status", Args: []string{"request bgp rib recovery"},
	})
	recoveryFrame := request(3, "ze-plugin-callback:execute-command", rpc.ExecuteCommandInput{
		Command: "request bgp rib recovery",
	})
	coalesced := request(4, "ze-plugin-callback:deliver-batch", map[string]any{
		"events": []map[string]string{{"command": "request bgp rib recovery"}},
	})
	coalesced = append(coalesced, request(5, "ze-plugin-callback:execute-command", rpc.ExecuteCommandInput{
		Command: "request bgp rib recovery",
	})...)
	coalesced = append(coalesced, request(6, "ze-plugin-callback:execute-command", rpc.ExecuteCommandInput{
		Command: "request bgp rib recovery",
	})...)
	steps := []struct {
		name  string
		frame []byte
		want  int32
	}{
		{name: "startup registry", frame: registryFrame},
		{name: "response payload", frame: []byte("#7 ok {\"command\":\"request bgp rib recovery\"}\n")},
		{name: "unrelated command", frame: unrelatedFrame},
		{name: "incomplete command", frame: recoveryFrame[:len(recoveryFrame)-1]},
		{name: "command terminator", frame: recoveryFrame[len(recoveryFrame)-1:], want: 1},
		{name: "coalesced callbacks", frame: coalesced, want: 3},
	}
	for _, readSize := range []int{1, 17, 4096} {
		t.Run(strconv.Itoa(readSize), func(t *testing.T) {
			client, engine := net.Pipe()
			written := make(chan error, 1)
			t.Cleanup(func() {
				client.Close() //nolint:errcheck // Closing both pipe ends unblocks all fixture I/O.
				engine.Close() //nolint:errcheck // The writer must join even after an assertion fails.
				select {
				case err := <-written:
					if err != nil && !t.Failed() {
						t.Error(err)
					}
				case <-time.After(5 * time.Second):
					t.Error("recovery IPC observer writer did not join")
				}
			})
			go func() {
				for _, step := range steps {
					if _, err := engine.Write(step.frame); err != nil {
						written <- err
						return
					}
				}
				written <- nil
			}()
			if err := client.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			lookup := make(chan struct{})
			var commands atomic.Int32
			conn := &recoveryIPCConn{Conn: client, lookup: lookup, commands: &commands}
			for _, step := range steps {
				got := make([]byte, len(step.frame))
				for offset := 0; offset < len(got); {
					end := min(offset+readSize, len(got))
					if _, err := io.ReadFull(conn, got[offset:end]); err != nil {
						t.Fatalf("%s: %v", step.name, err)
					}
					offset = end
				}
				if !bytes.Equal(got, step.frame) {
					t.Fatalf("%s: observer changed RPC bytes", step.name)
				}
				if count := commands.Load(); count != step.want {
					t.Fatalf("%s: observed %d recovery commands, want %d", step.name, count, step.want)
				}
				select {
				case <-lookup:
					if step.want == 0 {
						t.Fatalf("%s: signaled recovery before a complete request", step.name)
					}
				default:
					if step.want != 0 {
						t.Fatalf("%s: complete recovery request did not signal lookup", step.name)
					}
				}
			}
		})
	}
}

func recoveryReceivedFrom(delivery process.EventDelivery, source string) (uint64, bool) {
	if event, ok := delivery.Event.(*rpc.StructuredEvent); ok {
		return event.MessageID, event.EventType == rpc.EventKindUpdate &&
			event.Direction == rpc.DirectionReceived && event.PeerAddress == source
	}
	var envelope struct {
		BGP struct {
			Message struct {
				ID        uint64 `json:"id"`
				Direction string `json:"direction"`
			} `json:"message"`
			Peer struct {
				Remote struct {
					Address string `json:"address"`
				} `json:"remote"`
			} `json:"peer"`
		} `json:"bgp"`
	}
	if err := json.Unmarshal([]byte(delivery.Output), &envelope); err != nil {
		return 0, false
	}
	return envelope.BGP.Message.ID, envelope.BGP.Message.Direction == "received" &&
		envelope.BGP.Peer.Remote.Address == source
}

func recoverySentFrom(delivery process.EventDelivery, destination, source string) bool {
	if event, ok := delivery.Event.(*rpc.StructuredEvent); ok {
		if event.Direction != rpc.DirectionSent || event.PeerAddress != destination {
			return false
		}
		msg, ok := event.RawMessage.(*bgptypes.RawMessage)
		return ok && msg.SourcePeerStr == source
	}
	var envelope struct {
		BGP struct {
			Peer struct {
				Remote struct {
					Address string `json:"address"`
				} `json:"remote"`
			} `json:"peer"`
			Meta struct {
				Source string `json:"source-peer"`
			} `json:"route-meta"`
		} `json:"bgp"`
	}
	if err := json.Unmarshal([]byte(delivery.Output), &envelope); err != nil {
		return false
	}
	return envelope.BGP.Peer.Remote.Address == destination && envelope.BGP.Meta.Source == source
}

// recoveryTCPPeer owns a bounded frame channel and one socket reader. Cleanup
// closes its socket before joining stopped; no reader can outlive its fixture.
type recoveryTCPPeer struct {
	conn    net.Conn
	frames  chan []byte
	stopped chan struct{}
}

// recoveryListenerFactory transfers a reserved real socket to the reactor once.
// Reactor listener creation serializes Listen under its mutex. The reactor MUST
// close the adopted socket; fixture cleanup also closes it if startup fails.
type recoveryListenerFactory struct {
	listener net.Listener
}

func (f *recoveryListenerFactory) Listen(_ context.Context, _, address string) (net.Listener, error) {
	if f.listener == nil {
		return nil, fmt.Errorf("recovery listener already transferred")
	}
	if address != f.listener.Addr().String() {
		return nil, fmt.Errorf("recovery listener address %s, want %s", address, f.listener.Addr())
	}
	listener := f.listener
	f.listener = nil
	return listener, nil
}

func recoveryDeliveryRouter(t *testing.T, ctx context.Context) (*reactor.Reactor, *pluginserver.Server, []*recoveryTCPPeer) {
	t.Helper()
	// Reserve the real socket until startup adopts it, avoiding a port-reuse race.
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		listener.Close() //nolint:errcheck // The reactor closes an adopted socket; cleanup also covers failed startup.
	})
	local, err := netip.ParseAddrPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	r := reactor.New(&reactor.Config{Port: int(local.Port())})
	r.SetListenerFactory(&recoveryListenerFactory{listener: listener})
	for index := range 4 {
		address := netip.AddrFrom4([4]byte{127, 0, 0, byte(index + 1)})
		settings := reactor.NewPeerSettings(address, 65000, uint32(65001+index), 0x0a0000fe)
		settings.Connection = reactor.ConnectionPassive
		settings.LocalAddress = local.Addr()
		settings.ReceiveHoldTime = 90 * time.Second
		settings.NextHopMode = reactor.NextHopUnchanged
		settings.RSClient = true
		settings.RSFastPath = true
		settings.Capabilities = []capability.Capability{&capability.ASN4{ASN: 65000}, &capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}}
		for _, binding := range []struct{ name, receive string }{
			{"bgp-rib", "update state refresh"},
			{"bgp-rs", "update-received state open-received refresh"},
			{"bgp-adj-rib-in", "update-received state"},
		} {
			if err := reactor.EnsureProcessBinding(settings, binding.name, binding.receive, "update"); err != nil {
				t.Fatal(err)
			}
		}
		if err := r.AddPeer(settings); err != nil {
			t.Fatal(err)
		}
	}
	adapter, ok := r.ReactorLifecycleAdapter().(plugin.ReactorLifecycle)
	if !ok {
		t.Fatal("reactor lifecycle adapter has wrong type")
	}
	configs := []plugin.PluginConfig{{Name: "bgp-rib", Internal: true, Encoder: "json"}, {Name: "bgp-rs", Internal: true, Encoder: "json"}, {Name: "bgp-adj-rib-in", Internal: true, Encoder: "json"}}
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: configs}, adapter)
	if err != nil {
		t.Fatal(err)
	}
	originalBus := registry.GetEventBus()
	registry.SetEventBus(srv)
	t.Cleanup(func() { registry.SetEventBus(originalBus) })
	mgr := pluginmgr.NewManager()
	if err := mgr.StartAll(ctx, srv, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mgr.StopAll(context.Background()); err != nil {
			t.Error(err)
		}
	})
	srv.SetProcessSpawner(mgr)
	t.Cleanup(srv.Stop)
	if err := srv.StartWithContext(ctx); err != nil {
		t.Fatal(err)
	}
	r.SetPluginServer(srv)
	if err := r.StartWithContext(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		r.Stop()
		join, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := r.Wait(join); err != nil {
			t.Error(err)
		}
	})
	if err := srv.WaitForStartupComplete(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.StartPeers(); err != nil {
		t.Fatal(err)
	}
	recoveryPeersActive(t, ctx, r)
	listenAddrs := r.ListenAddrs()
	if len(listenAddrs) != 1 {
		t.Fatalf("recovery listeners = %v, want one shared listener", listenAddrs)
	}
	peers := make([]*recoveryTCPPeer, 0, 4)
	for index := range 4 {
		dialer := net.Dialer{LocalAddr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, byte(index+1))}}
		conn, err := dialer.DialContext(ctx, "tcp4", listenAddrs[0].String())
		if err != nil {
			t.Fatal(err)
		}
		peer := &recoveryTCPPeer{conn: conn, frames: make(chan []byte, 64), stopped: make(chan struct{})}
		go peer.read(ctx)
		t.Cleanup(func() {
			conn.Close() //nolint:errcheck // Closing both reader and socket is fixture teardown.
			recoveryAwait(t, context.Background(), peer.stopped, "TCP reader shutdown")
		})
		asn := uint16(65001 + index)
		open := &message.Open{Version: 4, MyAS: asn, HoldTime: 90, BGPIdentifier: uint32(0x0a000001 + index),
			OptionalParams: []byte{2, 6, 65, 4, 0, 0, byte(asn >> 8), byte(asn), 2, 6, 1, 4, 0, 1, 0, 1}}
		recoverySend(t, conn, message.PackTo(open, nil))
		recoverySend(t, conn, message.PackTo(message.NewKeepalive(), nil))
		for {
			update := recoveryNextUpdate(t, ctx, peer)
			if len(update.NLRI) == 0 && len(update.WithdrawnRoutes) == 0 && len(update.PathAttributes) == 0 {
				break
			}
		}
		peers = append(peers, peer)
	}
	return r, srv, peers
}

func (peer *recoveryTCPPeer) read(ctx context.Context) {
	defer close(peer.stopped)
	defer close(peer.frames)
	for {
		var header [message.HeaderLen]byte
		if _, err := io.ReadFull(peer.conn, header[:]); err != nil {
			return
		}
		size := int(binary.BigEndian.Uint16(header[16:18]))
		if size < message.HeaderLen {
			return
		}
		frame := make([]byte, size)
		copy(frame, header[:])
		if _, err := io.ReadFull(peer.conn, frame[message.HeaderLen:]); err != nil {
			return
		}
		select {
		case peer.frames <- frame:
		case <-ctx.Done():
			return
		}
	}
}

func recoveryAnnouncement(prefix, source, pathLength byte) []byte {
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 2 + 4*pathLength, 2, pathLength}
	for range pathLength {
		attrs = append(attrs, 0, 0, 0xfd, 0xe8+source)
	}
	attrs = append(attrs, 0x40, 3, 4, 192, 0, 2, source)
	return message.PackTo(&message.Update{PathAttributes: attrs, NLRI: []byte{24, 203, 0, prefix}}, nil)
}

func recoverySend(t *testing.T, conn net.Conn, frame []byte) {
	t.Helper()
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(frame); err != nil {
		t.Fatal(err)
	}
}

func recoveryNextUpdate(t *testing.T, ctx context.Context, peer *recoveryTCPPeer) *message.Update {
	t.Helper()
	for {
		select {
		case frame, open := <-peer.frames:
			if !open {
				t.Fatal("recipient TCP closed before the causal fence")
			}
			if frame[18] == byte(msgtype.TypeNOTIFICATION) {
				t.Fatalf("native peer received NOTIFICATION: %x", frame)
			}
			if frame[18] != byte(msgtype.TypeUPDATE) {
				continue
			}
			update, err := message.UnpackUpdate(frame[message.HeaderLen:])
			if err != nil {
				t.Fatal(err)
			}
			return update
		case <-ctx.Done():
			t.Fatal("recipient TCP update deadline", ctx.Err())
		}
	}
}

func recoveryWaitAnnouncement(t *testing.T, ctx context.Context, peer *recoveryTCPPeer, prefix, source byte) {
	t.Helper()
	for {
		update := recoveryNextUpdate(t, ctx, peer)
		if bytes.Equal(update.WithdrawnRoutes, []byte{24, 203, 0, 114}) {
			t.Fatal("surviving target was withdrawn before replacement")
		}
		if !bytes.Equal(update.NLRI, []byte{24, 203, 0, prefix}) {
			continue
		}
		_, _, nextHop, found := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
		if found && bytes.Equal(nextHop, []byte{192, 0, 2, source}) {
			expected, err := message.UnpackUpdate(recoveryAnnouncement(prefix, source, source-1)[message.HeaderLen:])
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(update.PathAttributes, expected.PathAttributes) {
				t.Fatalf("replacement attributes changed: got=%x want=%x", update.PathAttributes, expected.PathAttributes)
			}
			return
		}
		t.Fatalf("target prefix has wrong owner on TCP: attrs=%x, want source=%d", update.PathAttributes, source)
	}
}

func recoveryStored(t *testing.T, ctx context.Context, source string, prefix byte) {
	t.Helper()
	// Reading the registered dump bridge observes actual Adj-RIB-In. Polling
	// only startup/ingest state is not the causal gate used by this test.
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		found := false
		bgprib.RIBDumpBridge.DumpRIB(registry.RIBDumpVisitor{
			OnPeer: func(address string, _ uint32, _ [4]byte, _ bool) uint16 {
				if address == source {
					return 1
				}
				return 0
			},
			OnRoute: func(peer, afi, safi uint16, bits uint8, nlri, _ []byte) {
				if peer == 1 && afi == 1 && safi == 1 && bits == 24 && bytes.Equal(nlri, []byte{203, 0, prefix}) {
					found = true
				}
			},
		})
		if found {
			return
		}
		select {
		case <-tick.C:
		case <-ctx.Done():
			t.Fatal("real RIB route was not installed", ctx.Err())
		}
	}
}

func recoveryAwait(t *testing.T, ctx context.Context, ready <-chan struct{}, what string) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal(what, ctx.Err())
	case <-deadline.C:
		t.Fatal(what, "deadline")
	}
}

func recoveryResult(t *testing.T, ctx context.Context, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		t.Fatal("native recovery command deadline", ctx.Err())
		return ctx.Err()
	}
}

func recoveryPeersActive(t *testing.T, ctx context.Context, r *reactor.Reactor) {
	t.Helper()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		active := true
		for _, peer := range r.Peers() {
			if peer.SessionState() != fsm.StateActive {
				active = false
			}
		}
		if active {
			return
		}
		select {
		case <-tick.C:
		case <-ctx.Done():
			t.Fatal("native peer workers did not become active", ctx.Err())
		}
	}
}
