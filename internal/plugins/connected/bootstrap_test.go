// Design: docs/guide/redistribution.md -- connected startup inventory.

package connected

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	connectedevents "github.com/ze-software/ze/internal/plugins/connected/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// bootstrapHost is a warm address owner and the engine end of a real SDK
// connection. Its inventory exists before runConnectedPlugin subscribes. It
// replays only in response to the consumer's bootstrap request; no new address
// event is needed to make the initial routes visible.
type bootstrapHost struct {
	ctx         context.Context
	mux         *rpc.MuxConn
	bridge      *rpc.DirectBridge
	seen        chan string
	rib         *locrib.RIB
	mu          sync.Mutex
	ready       rpc.ReadyInput
	requests    int
	batches     []redistevents.RouteChangeBatch
	addresses   map[string]json.RawMessage
	stopped     chan struct{}
	beforeEmit  func(*redistevents.RouteChangeBatch) error
	reportIssue func(rpc.ReportIssueInput) error
}

func (h *bootstrapHost) serve() {
	for {
		select {
		case <-h.ctx.Done():
			return
		case req, ok := <-h.mux.Requests():
			if !ok {
				return
			}
			out, err := h.answer(req.Method, req.Params)
			if err != nil {
				_ = h.mux.SendError(h.ctx, req.ID, err.Error())
			} else {
				_ = h.mux.SendResult(h.ctx, req.ID, out)
			}
			h.seen <- req.Method
		}
	}
}

func (h *bootstrapHost) answer(method string, body json.RawMessage) (any, error) {
	switch method {
	case rpc.MethodDeclareRegistration:
		return rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}, nil
	case "ze-plugin-engine:ready":
		h.mu.Lock()
		defer h.mu.Unlock()
		return nil, json.Unmarshal(body, &h.ready)
	case rpc.MethodEmitEvent:
		var in rpc.EmitEventInput
		if err := json.Unmarshal(body, &in); err != nil {
			return nil, err
		}
		return h.emit(in)
	case rpc.MethodReportIssue:
		var in rpc.ReportIssueInput
		if err := json.Unmarshal(body, &in); err != nil {
			return nil, err
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.reportIssue != nil {
			return nil, h.reportIssue(in)
		}
		return nil, nil
	case "ze-plugin-engine:route-install":
		var in rpc.RouteInstallInput
		if err := json.Unmarshal(body, &in); err != nil {
			return nil, err
		}
		for _, route := range in.Routes {
			if route.Protocol != connectedevents.Namespace {
				return nil, fmt.Errorf("unexpected protocol %q", route.Protocol)
			}
			prefix, err := netip.ParsePrefix(route.Prefix)
			if err != nil {
				return nil, err
			}
			if route.NextHop != "" {
				return nil, fmt.Errorf("connected route has gateway %q", route.NextHop)
			}
			h.rib.InsertForward(familyOf(prefix), prefix, locrib.Path{Source: connectedevents.ProtocolID, Instance: route.Instance}, nil)
		}
		return rpc.RouteInstallOutput{Installed: uint32(len(in.Routes))}, nil
	case "ze-plugin-engine:route-remove":
		var in rpc.RouteRemoveInput
		if err := json.Unmarshal(body, &in); err != nil {
			return nil, err
		}
		for _, route := range in.Routes {
			prefix, err := netip.ParsePrefix(route.Prefix)
			if err != nil {
				return nil, err
			}
			h.rib.Remove(familyOf(prefix), prefix, connectedevents.ProtocolID, route.Instance)
		}
		return rpc.RouteRemoveOutput{Removed: uint32(len(in.Routes))}, nil
	default:
		return nil, nil
	}
}

func (h *bootstrapHost) emit(in rpc.EmitEventInput) (rpc.EmitEventOutput, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if in.Namespace == ifaceevents.Namespace && in.EventType == ifaceevents.EventAddressReplayRequest {
		h.requests++
		// Engine subscribers do not contribute to Delivered. Zero does not
		// distinguish an empty active owner from an absent interface provider.
		return rpc.EmitEventOutput{}, nil
	}
	if in.Namespace != connectedevents.Namespace || in.EventType != redistevents.EventType {
		return rpc.EmitEventOutput{}, fmt.Errorf("unexpected event %s/%s", in.Namespace, in.EventType)
	}
	var batch redistevents.RouteChangeBatch
	if err := json.Unmarshal([]byte(in.Event), &batch); err != nil {
		return rpc.EmitEventOutput{}, err
	}
	if h.beforeEmit != nil {
		if err := h.beforeEmit(&batch); err != nil {
			return rpc.EmitEventOutput{}, err
		}
	}
	h.batches = append(h.batches, batch)
	return rpc.EmitEventOutput{Delivered: 1}, nil
}

func (h *bootstrapHost) waitFor(t *testing.T, method string) {
	t.Helper()
	for {
		select {
		case got := <-h.seen:
			if got == method {
				return
			}
		case <-h.ctx.Done():
			t.Fatalf("waiting for %s: %v", method, h.ctx.Err())
		}
	}
}

func (h *bootstrapHost) callback(t *testing.T, method string, in any) {
	t.Helper()
	if err := h.call(method, in); err != nil {
		t.Fatalf("%s: %v", method, err)
	}
}

func (h *bootstrapHost) call(method string, in any) error {
	var err error
	if h.bridge != nil && method != "ze-plugin-callback:configure" && method != "ze-plugin-callback:share-registry" {
		body, marshalErr := json.Marshal(in)
		if marshalErr != nil {
			return marshalErr
		}
		_, err = h.bridge.SendCallback(h.ctx, method, body)
	} else {
		_, err = h.mux.CallRPC(h.ctx, method, in)
	}
	return err
}

func (h *bootstrapHost) deliver(t *testing.T, namespace, event string, payload json.RawMessage) {
	t.Helper()
	if err := h.deliverEvent(namespace, event, payload); err != nil {
		t.Fatal(err)
	}
}

func (h *bootstrapHost) deliverEvent(namespace, event string, payload json.RawMessage) error {
	body, err := json.Marshal(rpc.EventEnvelope{Namespace: namespace, Event: event, Payload: payload})
	if err != nil {
		return err
	}
	if h.bridge != nil {
		return h.bridge.DeliverEvents([]string{string(body)})
	}
	return h.call("ze-plugin-callback:deliver-event", rpc.DeliverEventInput{Event: string(body)})
}

func (h *bootstrapHost) replay(t *testing.T) {
	t.Helper()
	for _, payload := range h.addresses {
		h.deliver(t, ifaceevents.Namespace, ifaceevents.EventAddrAdded, payload)
	}
}

func (h *bootstrapHost) add(address string, bits int) {
	h.addresses[address] = makePayload(address, bits)
}

func (h *bootstrapHost) remove(t *testing.T, address string) {
	t.Helper()
	payload := h.addresses[address]
	delete(h.addresses, address)
	h.deliver(t, ifaceevents.Namespace, ifaceevents.EventAddrRemoved, payload)
}

func startBootstrapConsumer(t *testing.T, mode string, addresses map[string]json.RawMessage) *bootstrapHost {
	t.Helper()
	declareConnectedDistance(t, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	pluginEnd, engineEnd := net.Pipe()
	h := &bootstrapHost{ctx: ctx, mux: rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)), seen: make(chan string, 64), rib: locrib.NewRIB(), addresses: addresses, stopped: make(chan struct{})}
	var conn net.Conn = pluginEnd
	if mode == "direct" {
		h.bridge = rpc.NewDirectBridge()
		h.bridge.SetDispatchRPC(func(_ context.Context, method string, body json.RawMessage) (json.RawMessage, error) {
			out, err := h.answer(method, body)
			if err != nil {
				return nil, err
			}
			return json.Marshal(out)
		})
		h.bridge.SetEmitEvent(func(namespace, event, direction, peer, payload string) (int, error) {
			out, err := h.emit(rpc.EmitEventInput{Namespace: namespace, EventType: event, Direction: direction, PeerAddress: peer, Event: payload})
			return out.Delivered, err
		})
		conn = rpc.NewBridgedConn(pluginEnd, h.bridge)
	}
	go h.serve()
	done := make(chan int, 1)
	go func() {
		done <- runConnectedPlugin(conn)
		close(h.stopped)
	}()
	t.Cleanup(func() {
		cancel()
		if h.bridge != nil {
			h.bridge.CloseCallbacks()
		}
		_ = h.mux.Close()
		_ = pluginEnd.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("connected did not stop after its transport closed")
		}
	})
	h.waitFor(t, rpc.MethodDeclareRegistration)
	h.callback(t, "ze-plugin-callback:configure", rpc.ConfigureInput{})
	h.waitFor(t, "ze-plugin-engine:declare-capabilities")
	h.callback(t, "ze-plugin-callback:share-registry", rpc.ShareRegistryInput{})
	h.waitFor(t, "ze-plugin-engine:ready")
	// This callback completes only after OnStarted has returned and the SDK
	// event loop runs. It is a state barrier, with no startup sleep or poll.
	h.callback(t, "ze-plugin-callback:post-startup", struct{}{})
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.requests != 1 {
		t.Fatalf("connected sent %d address replay requests after ready, want 1 for the warm owner", h.requests)
	}
	if h.ready.Subscribe == nil {
		t.Fatal("connected registered no atomic startup subscriptions")
	}
	if h.ready.Subscribe.Namespace != ifaceevents.Namespace || !h.ready.Subscribe.Envelope {
		t.Fatalf("interface startup subscription = %+v, want interface envelope", h.ready.Subscribe)
	}
	for _, event := range []string{ifaceevents.EventAddrAdded, ifaceevents.EventAddrRemoved} {
		if !slices.Contains(h.ready.Subscribe.Events, event) {
			t.Fatalf("startup subscription omits %s", event)
		}
	}
	replay := false
	for _, sub := range h.ready.Subscriptions {
		if sub.Namespace == redistevents.ReplayNamespace && slices.Contains(sub.Events, redistevents.ReplayRequestEventType) {
			replay = true
		}
	}
	if !replay {
		t.Fatal("redistribution replay was not subscribed atomically with ready")
	}
	return h
}

// Each real runner starts in its own test process so locrib.Default sees the
// actual subprocess token before its sync.Once. This proves route-install RPC
// publication instead of accidentally reading the test process's singleton.
func bootstrapModes(t *testing.T, scenario func(*testing.T, string)) {
	t.Helper()
	if mode := os.Getenv("CONNECTED_BOOTSTRAP_CHILD"); mode != "" {
		scenario(t, mode)
		return
	}
	for _, mode := range []string{"direct", "socket"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			args := []string{"-test.run=^" + t.Name()[:len(t.Name())-len(mode)-1] + "$"}
			// testing's teardown merges counter files from this directory,
			// including child runs of the same instrumented test executable.
			if coverdir := flag.Lookup("test.gocoverdir"); coverdir != nil {
				if directory := coverdir.Value.String(); directory != "" {
					args = append(args, "-test.gocoverdir="+directory)
				}
			}
			cmd := exec.CommandContext(ctx, os.Args[0], args...)
			cmd.Env = append(os.Environ(), "CONNECTED_BOOTSTRAP_CHILD="+mode, "ZE_PLUGIN_HUB_TOKEN=connected-bootstrap-test")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s bootstrap child: %v\n%s", mode, err, out)
			}
		})
	}
}

func assertBootstrapPath(t *testing.T, h *bootstrapHost, text string, present bool) {
	t.Helper()
	prefix := netip.MustParsePrefix(text)
	path, found := h.rib.Best(familyOf(prefix), prefix)
	if found != present {
		t.Fatalf("Loc-RIB %s present=%v, want %v", prefix, found, present)
	}
	if found && (path.Source != connectedevents.ProtocolID || path.NextHop.IsValid()) {
		t.Fatalf("Loc-RIB %s path=%+v, want connected with no gateway", prefix, path)
	}
}

// MUTATION: omitting the OnStarted address request or startup event handler
// removes the initial positive routes, without any kernel event to rescue them.
func TestConnectedBootstrapWarmOwner(t *testing.T) {
	bootstrapModes(t, func(t *testing.T, mode string) {
		owner := &bootstrapHost{addresses: make(map[string]json.RawMessage)}
		owner.add("192.0.2.1", 24)
		owner.add("192.0.2.2", 24)
		owner.add("2001:db8:5::1", 64)
		owner.add("127.0.0.1", 8)
		owner.add("::1", 128)
		owner.add("fe80::1", 64)
		owner.add("198.51.100.1", 24)
		delete(owner.addresses, "198.51.100.1")
		h := startBootstrapConsumer(t, mode, owner.addresses)
		h.replay(t)
		for _, prefix := range []string{"192.0.2.0/24", "2001:db8:5::/64", "127.0.0.0/8", "::1/128", "fe80::/64"} {
			assertBootstrapPath(t, h, prefix, true)
		}
		assertBootstrapPath(t, h, "198.51.100.0/24", false)
		h.replay(t)
		h.mu.Lock()
		if len(h.batches) != 2 {
			t.Fatalf("warm and repeated inventory emitted %d redistribution batches, want exactly IPv4 and IPv6", len(h.batches))
		}
		prefixes := make(map[netip.Prefix]bool)
		for _, batch := range h.batches {
			if batch.Protocol != connectedevents.ProtocolID || batch.ReplayID != 0 || len(batch.Entries) != 1 || batch.Entries[0].Action != redistevents.ActionAdd {
				t.Fatalf("initial redistribution batch=%+v", batch)
			}
			prefixes[batch.Entries[0].Prefix] = true
		}
		h.mu.Unlock()
		for _, text := range []string{"192.0.2.0/24", "2001:db8:5::/64"} {
			if !prefixes[netip.MustParsePrefix(text)] {
				t.Fatalf("redistribution omitted %s", text)
			}
		}
		h.remove(t, "192.0.2.1")
		assertBootstrapPath(t, h, "192.0.2.0/24", true)
		h.remove(t, "192.0.2.2")
		assertBootstrapPath(t, h, "192.0.2.0/24", false)
		h.replay(t)
		assertBootstrapPath(t, h, "192.0.2.0/24", false)
		request, err := json.Marshal(redistevents.ReplayRequest{ReplayID: 73})
		if err != nil {
			t.Fatal(err)
		}
		h.deliver(t, redistevents.ReplayNamespace, redistevents.ReplayRequestEventType, request)
		h.mu.Lock()
		defer h.mu.Unlock()
		if len(h.batches) != 4 {
			t.Fatalf("redistribution has %d batches after final removal and targeted replay, want 4", len(h.batches))
		}
		removed, replayed := h.batches[2], h.batches[3]
		if removed.Entries[0].Action != redistevents.ActionRemove || removed.Entries[0].Prefix != netip.MustParsePrefix("192.0.2.0/24") {
			t.Fatalf("final removal batch=%+v", removed)
		}
		if replayed.ReplayID != 73 || len(replayed.Entries) != 1 || replayed.Entries[0].Prefix != netip.MustParsePrefix("2001:db8:5::/64") || replayed.Entries[0].Action != redistevents.ActionAdd {
			t.Fatalf("targeted replay=%+v, want only live IPv6 prefix and ReplayID 73", replayed)
		}
	})
}

func TestConnectedBootstrapWithoutProviderDoesNotWait(t *testing.T) {
	bootstrapModes(t, func(t *testing.T, mode string) {
		h := startBootstrapConsumer(t, mode, nil)
		assertBootstrapPath(t, h, "192.0.2.0/24", false)
		// No owner answers the request. The SDK still processes a later event;
		// startup did not wait forever or invent a route from an empty inventory.
		h.deliver(t, ifaceevents.Namespace, ifaceevents.EventAddrAdded, makePayload("192.0.2.1", 24))
		assertBootstrapPath(t, h, "192.0.2.0/24", true)
		h.mu.Lock()
		defer h.mu.Unlock()
		if len(h.batches) != 1 {
			t.Fatalf("later address emitted %d route batches, want 1", len(h.batches))
		}
	})
}

// MUTATION: omitting connected's own local withdrawal on stop leaves its
// shared Loc-RIB path behind, even though a restarted observer cannot replay it.
func TestConnectedBootstrapInternalStopWithdrawsOnlyOwnedPrefixes(t *testing.T) {
	bootstrapModes(t, func(t *testing.T, mode string) {
		// The child isolates the singleton while the empty token selects the
		// real internal Loc-RIB branch, before its first initialization.
		t.Setenv("ZE_PLUGIN_HUB_TOKEN", "")
		env.ResetCache()
		h := startBootstrapConsumer(t, mode, make(map[string]json.RawMessage))
		h.rib = locrib.Default()
		if h.rib == nil {
			t.Fatal("internal connected test has no shared Loc-RIB")
		}
		h.add("192.0.2.1", 24)
		h.add("2001:db8:5::1", 64)
		h.replay(t)
		assertBootstrapPath(t, h, "192.0.2.0/24", true)
		assertBootstrapPath(t, h, "2001:db8:5::/64", true)
		survivor := redistevents.RegisterProtocol("connected-stop-survivor")
		shared := netip.MustParsePrefix("192.0.2.0/24")
		other := netip.MustParsePrefix("203.0.113.0/24")
		for _, prefix := range []netip.Prefix{shared, other} {
			h.rib.InsertForward(familyOf(prefix), prefix, locrib.Path{Source: survivor, Instance: 47}, nil)
		}
		h.callback(t, "ze-plugin-callback:bye", rpc.ByeInput{Reason: "config root removed"})
		select {
		case <-h.stopped:
		case <-h.ctx.Done():
			t.Fatal("connected did not finish its clean stop")
		}
		assertBootstrapPath(t, h, "2001:db8:5::/64", false)
		for _, prefix := range []netip.Prefix{shared, other} {
			path, found := h.rib.Best(familyOf(prefix), prefix)
			if !found || path.Source != survivor || path.Instance != 47 {
				t.Fatalf("after connected stop, %s path=%+v found=%v; want unrelated source instance 47", prefix, path, found)
			}
		}
	})
}

// MUTATION: omitting the live OnBye redistribution withdrawal leaves a
// destination holding the stopped producer's routes after its runner exits.
func TestConnectedBootstrapByeWithdrawsRedistributedPrefixes(t *testing.T) {
	bootstrapModes(t, func(t *testing.T, mode string) {
		h := startBootstrapConsumer(t, mode, make(map[string]json.RawMessage))
		h.add("192.0.2.1", 24)
		h.add("192.0.2.2", 24)
		h.add("2001:db8:5::1", 64)
		h.replay(t)
		type routeKey struct {
			source redistevents.ProtocolID
			prefix netip.Prefix
		}
		shared := netip.MustParsePrefix("192.0.2.0/24")
		ipv6 := netip.MustParsePrefix("2001:db8:5::/64")
		survivor := redistevents.RegisterProtocol("connected-bye-survivor")
		held := map[routeKey]bool{{source: survivor, prefix: shared}: true}
		h.mu.Lock()
		for _, batch := range h.batches {
			if batch.Protocol != connectedevents.ProtocolID || batch.ReplayID != 0 {
				t.Fatalf("initial redistribution source=%v replay=%d", batch.Protocol, batch.ReplayID)
			}
			for _, entry := range batch.Entries {
				if entry.Action != redistevents.ActionAdd {
					t.Fatalf("initial redistribution entry=%+v, want add", entry)
				}
				held[routeKey{source: batch.Protocol, prefix: entry.Prefix}] = true
			}
		}
		initial := len(h.batches)
		h.mu.Unlock()
		for _, prefix := range []netip.Prefix{shared, ipv6} {
			if !held[routeKey{source: connectedevents.ProtocolID, prefix: prefix}] {
				t.Fatalf("destination never received initial connected add for %s", prefix)
			}
		}
		h.callback(t, "ze-plugin-callback:bye", rpc.ByeInput{Reason: "removed"})
		select {
		case <-h.stopped:
		case <-h.ctx.Done():
			t.Fatal("connected did not finish its clean stop")
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		removed := make(map[netip.Prefix]int)
		for _, batch := range h.batches[initial:] {
			if batch.Protocol != connectedevents.ProtocolID || batch.ReplayID != 0 {
				t.Fatalf("stop redistribution source=%v replay=%d", batch.Protocol, batch.ReplayID)
			}
			for _, entry := range batch.Entries {
				if entry.Action != redistevents.ActionRemove {
					t.Fatalf("stop redistribution entry=%+v, want remove", entry)
				}
				removed[entry.Prefix]++
				delete(held, routeKey{source: batch.Protocol, prefix: entry.Prefix})
			}
		}
		for _, prefix := range []netip.Prefix{shared, ipv6} {
			if removed[prefix] != 1 {
				t.Fatalf("clean bye emitted %d connected removes for %s, want exactly one before runner exit", removed[prefix], prefix)
			}
			if held[routeKey{source: connectedevents.ProtocolID, prefix: prefix}] {
				t.Fatalf("destination still holds stopped connected prefix %s", prefix)
			}
		}
		if len(removed) != 2 {
			t.Fatalf("clean bye removed %d prefixes, want exactly the two owned prefixes", len(removed))
		}
		if !held[routeKey{source: survivor, prefix: shared}] || len(held) != 1 {
			t.Fatalf("clean bye changed unrelated source at %s: %+v", shared, held)
		}
	})
}

func (h *bootstrapHost) waitResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-h.ctx.Done():
		t.Fatal("connected lifecycle operation did not finish")
		return h.ctx.Err()
	}
}

// The publication barriers put Bye behind an accepted Add, then hold Bye's
// withdrawal while another address change and replay enter the real SDK.
func TestConnectedBootstrapByeDrainsIngress(t *testing.T) {
	bootstrapModes(t, func(t *testing.T, mode string) {
		h := startBootstrapConsumer(t, mode, nil)
		addEntered, removeEntered := make(chan struct{}), make(chan struct{})
		releaseAdd, releaseRemove := make(chan struct{}), make(chan struct{})
		var addOnce, removeOnce sync.Once
		t.Cleanup(func() {
			addOnce.Do(func() { close(releaseAdd) })
			removeOnce.Do(func() { close(releaseRemove) })
		})
		h.mu.Lock()
		addPaused, removePaused := false, false
		h.beforeEmit = func(batch *redistevents.RouteChangeBatch) error {
			if batch.Entries[0].Action == redistevents.ActionAdd && !addPaused {
				addPaused = true
				close(addEntered)
				select {
				case <-releaseAdd:
				case <-h.ctx.Done():
					return h.ctx.Err()
				}
			}
			if batch.Entries[0].Action == redistevents.ActionRemove && !removePaused {
				removePaused = true
				close(removeEntered)
				select {
				case <-releaseRemove:
				case <-h.ctx.Done():
					return h.ctx.Err()
				}
			}
			return nil
		}
		h.mu.Unlock()
		accepted := make(chan error, 1)
		go func() {
			accepted <- h.deliverEvent(ifaceevents.Namespace, ifaceevents.EventAddrAdded, makePayload("192.0.2.1", 24))
		}()
		select {
		case <-addEntered:
		case <-h.ctx.Done():
			t.Fatal("accepted address did not reach publication")
		}
		bye := make(chan error, 1)
		go func() { bye <- h.call("ze-plugin-callback:bye", rpc.ByeInput{Reason: "removed"}) }()
		addOnce.Do(func() { close(releaseAdd) })
		select {
		case <-removeEntered:
		case <-h.ctx.Done():
			t.Fatal("Bye did not drain the accepted Add and reach withdrawal")
		}
		request, err := json.Marshal(redistevents.ReplayRequest{ReplayID: 73})
		if err != nil {
			t.Fatal(err)
		}
		lateAdd, lateRemove, replay := make(chan error, 1), make(chan error, 1), make(chan error, 1)
		go func() {
			lateAdd <- h.deliverEvent(ifaceevents.Namespace, ifaceevents.EventAddrAdded, makePayload("203.0.113.1", 24))
		}()
		go func() {
			lateRemove <- h.deliverEvent(ifaceevents.Namespace, ifaceevents.EventAddrRemoved, makePayload("192.0.2.1", 24))
		}()
		go func() {
			replay <- h.deliverEvent(redistevents.ReplayNamespace, redistevents.ReplayRequestEventType, request)
		}()
		removeOnce.Do(func() { close(releaseRemove) })
		if err := h.waitResult(t, accepted); err != nil {
			t.Fatalf("accepted address failed: %v", err)
		}
		if err := h.waitResult(t, bye); err != nil {
			t.Fatalf("Bye failed: %v", err)
		}
		// Socket callbacks queued behind a successful Bye can be rejected as
		// the connection closes; direct admitted handlers return after retiring.
		for _, result := range []<-chan error{lateAdd, lateRemove, replay} {
			_ = h.waitResult(t, result)
		}
		select {
		case <-h.stopped:
		case <-h.ctx.Done():
			t.Fatal("connected did not stop after draining ingress")
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if len(h.batches) != 2 {
			t.Fatalf("concurrent stop published %d batches, want accepted Add then final Remove: %+v", len(h.batches), h.batches)
		}
		for index, action := range []redistevents.RouteAction{redistevents.ActionAdd, redistevents.ActionRemove} {
			batch := h.batches[index]
			if batch.Protocol != connectedevents.ProtocolID || batch.ReplayID != 0 || len(batch.Entries) != 1 || batch.Entries[0].Action != action || batch.Entries[0].Prefix != netip.MustParsePrefix("192.0.2.0/24") {
				t.Fatalf("concurrent stop batch %d=%+v, want %s for accepted prefix", index, batch, action)
			}
		}
	})
}

func TestConnectedBootstrapByeRefusalAllowsRetry(t *testing.T) {
	bootstrapModes(t, func(t *testing.T, mode string) {
		h := startBootstrapConsumer(t, mode, make(map[string]json.RawMessage))
		h.add("192.0.2.1", 24)
		h.add("2001:db8:5::1", 64)
		h.replay(t)
		h.mu.Lock()
		removes := 0
		h.beforeEmit = func(batch *redistevents.RouteChangeBatch) error {
			if batch.Entries[0].Action == redistevents.ActionRemove {
				removes++
				if removes == 2 {
					return fmt.Errorf("test engine refused withdrawal")
				}
			}
			return nil
		}
		initial := len(h.batches)
		h.mu.Unlock()
		if err := h.call("ze-plugin-callback:bye", rpc.ByeInput{Reason: "removed"}); err == nil {
			t.Fatal("Bye acknowledged a failed withdrawal")
		}
		select {
		case <-h.stopped:
			t.Fatal("failed Bye stopped the observer instead of allowing retry")
		default:
		}
		h.mu.Lock()
		restored := make(map[netip.Prefix]bool)
		for _, batch := range h.batches[initial:] {
			for _, entry := range batch.Entries {
				if entry.Action == redistevents.ActionAdd {
					restored[entry.Prefix] = true
				}
			}
		}
		h.beforeEmit = nil
		h.mu.Unlock()
		for _, text := range []string{"192.0.2.0/24", "2001:db8:5::/64"} {
			if !restored[netip.MustParsePrefix(text)] {
				t.Fatalf("refused Bye did not restore %s", text)
			}
		}
		h.deliver(t, ifaceevents.Namespace, ifaceevents.EventAddrAdded, makePayload("203.0.113.1", 24))
		assertBootstrapPath(t, h, "203.0.113.0/24", true)
		h.mu.Lock()
		initial = len(h.batches)
		h.mu.Unlock()
		h.callback(t, "ze-plugin-callback:bye", rpc.ByeInput{Reason: "removed"})
		h.mu.Lock()
		defer h.mu.Unlock()
		withdrawn := make(map[netip.Prefix]bool)
		for _, batch := range h.batches[initial:] {
			for _, entry := range batch.Entries {
				if entry.Action != redistevents.ActionRemove {
					t.Fatalf("retry emitted %+v, want withdrawal", entry)
				}
				withdrawn[entry.Prefix] = true
			}
		}
		if len(withdrawn) != 3 {
			t.Fatalf("retry withdrew %d prefixes, want original two and the later live address", len(withdrawn))
		}
	})
}

func TestConnectedBootstrapByeRestorationFailureIsReported(t *testing.T) {
	bootstrapModes(t, func(t *testing.T, mode string) {
		h := startBootstrapConsumer(t, mode, make(map[string]json.RawMessage))
		h.add("192.0.2.1", 24)
		h.replay(t)
		h.mu.Lock()
		h.beforeEmit = func(batch *redistevents.RouteChangeBatch) error {
			if batch.Entries[0].Action == redistevents.ActionRemove {
				return fmt.Errorf("test engine refused withdrawal")
			}
			return fmt.Errorf("test engine refused restoration")
		}
		h.mu.Unlock()
		err := h.call("ze-plugin-callback:bye", rpc.ByeInput{Reason: "removed"})
		if err == nil {
			t.Fatal("Bye acknowledged failed withdrawal and restoration")
		}
		for _, message := range []string{"test engine refused withdrawal", "test engine refused restoration"} {
			if !strings.Contains(err.Error(), message) {
				t.Fatalf("Bye error %q omits %q", err, message)
			}
		}
		h.mu.Lock()
		h.beforeEmit = nil
		h.mu.Unlock()
		h.deliver(t, ifaceevents.Namespace, ifaceevents.EventAddrAdded, makePayload("203.0.113.1", 24))
		assertBootstrapPath(t, h, "203.0.113.0/24", true)
		h.callback(t, "ze-plugin-callback:bye", rpc.ByeInput{Reason: "removed"})
	})
}
