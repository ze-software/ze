// VALIDATES: spec-fixit-plugin-event-subscription.
//   - Gap A: a startup subscription that names a non-default namespace registers
//     in that namespace and delivers (AC-1); an unknown namespace warns and skips
//     rather than registering a silently-dead NamespaceUnknown subscription (AC-6).
//   - Gap B: a subscriber that opted into the envelope receives {namespace,event,
//     payload} so two events sharing a payload shape are still discriminable
//     (AC-2); without the opt-in the delivered bytes are the bare payload,
//     byte-identical to pre-change (AC-3/AC-7).
//   - Gap C: a "*" startup subscription expands at registration time into one
//     subscription per registered event type of the namespace and actually
//     delivers, where today it is a dead no-op (AC-8).
//
// All assertions drive the real production entry points (registerSubscriptions
// + EmitEngineEvent + the deliver-batch wire), never a hand-built Subscription,
// so they prove wiring, not just data shape.
package server

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// saShape mirrors the ike SAEvent property that makes Gap B sharp: sa-up and
// sa-down share one payload type, so the bare payload cannot discriminate them.
type saShape struct {
	PeerName string `json:"peer-name"`
}

func newSubscriptionServer() *Server {
	return &Server{
		subscriptions:     newSubscriptionManager(),
		engineSubscribers: newEngineEventSubscribers(),
	}
}

// newDeliveryProbe wires a process to a net.Pipe and reads deliver-batch RPCs on
// the plugin side, pushing the decoded event strings onto the returned channel.
// This is the real engine->plugin delivery path (SendDeliverBatch), so the
// captured strings are the exact bytes an external plugin would receive.
func newDeliveryProbe(t *testing.T, name string) (*process.Process, <-chan []string) {
	t.Helper()
	engineSide, pluginSide := net.Pipe()
	t.Cleanup(func() { _ = engineSide.Close() })
	t.Cleanup(func() { _ = pluginSide.Close() })

	proc := process.NewProcess(plugin.PluginConfig{Name: name})
	proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))
	proc.SetRunning(true)
	proc.StartDelivery(context.Background())

	batches := make(chan []string, 8)
	go func() {
		pluginConn := rpc.NewConn(pluginSide, pluginSide)
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			req, err := pluginConn.ReadRequest(ctx)
			cancel()
			if err != nil {
				return
			}
			if req.Method != "ze-plugin-callback:deliver-batch" {
				continue
			}
			raws, perr := rpc.ParseBatchEvents(req.Params)
			if perr != nil {
				return
			}
			out := make([]string, 0, len(raws))
			for _, r := range raws {
				var s string
				if json.Unmarshal(r, &s) == nil {
					out = append(out, s)
				}
			}
			wctx, wcancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = pluginConn.SendResult(wctx, req.ID, nil)
			wcancel()
			batches <- out
		}
	}()
	return proc, batches
}

func waitBatch(t *testing.T, batches <-chan []string) []string {
	t.Helper()
	select {
	case b := <-batches:
		return b
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for event delivery")
		return nil
	}
}

// AC-1 (Gap A): a startup subscription that names a non-default namespace lands
// in that namespace and a matching emit is delivered.
func TestStartupSubscriptionHonorsNamespace(t *testing.T) {
	events.Register[*saShape]("test-ns-honors", "sa-up")

	s := newSubscriptionServer()
	proc, batches := newDeliveryProbe(t, "ns-probe")

	// A control process subscribing to the SAME event name but WITHOUT a
	// namespace (default) must NOT receive the test-namespace emit: proves the
	// namespace, not just the event type, is honored.
	control, controlBatches := newDeliveryProbe(t, "control-probe")
	s.registerSubscriptions(control, &rpc.SubscribeEventsInput{Events: []string{"sa-up"}})

	s.registerSubscriptions(proc, &rpc.SubscribeEventsInput{
		Namespace: "test-ns-honors",
		Events:    []string{"sa-up"},
	})

	delivered, err := s.EmitEngineEvent("test-ns-honors", "sa-up", &saShape{PeerName: "peer-1"})
	if err != nil {
		t.Fatalf("EmitEngineEvent: %v", err)
	}
	if delivered != 1 {
		t.Fatalf("delivered = %d, want 1 (only the namespaced subscription matches)", delivered)
	}

	batch := waitBatch(t, batches)
	if len(batch) != 1 {
		t.Fatalf("batch len = %d, want 1", len(batch))
	}
	if !strings.Contains(batch[0], `"peer-name":"peer-1"`) {
		t.Fatalf("delivered payload = %q, want the bare payload", batch[0])
	}

	select {
	case b := <-controlBatches:
		t.Fatalf("control (default namespace) received %v, want nothing", b)
	case <-time.After(100 * time.Millisecond):
	}
}

// AC-2 (Gap B): with the envelope opt-in, two events that share a payload type
// are discriminable by the envelope's (namespace, event) fields.
func TestDeliveredEventIsDiscriminable(t *testing.T) {
	events.Register[*saShape]("test-ns-disc", "sa-up")
	events.Register[*saShape]("test-ns-disc", "sa-down")

	s := newSubscriptionServer()
	proc, batches := newDeliveryProbe(t, "disc-probe")

	s.registerSubscriptions(proc, &rpc.SubscribeEventsInput{
		Namespace: "test-ns-disc",
		Events:    []string{"sa-up", "sa-down"},
		Envelope:  true,
	})

	// Identical payloads for both events -- only the envelope can tell them apart.
	payload := &saShape{PeerName: "peer-shared"}

	if _, err := s.EmitEngineEvent("test-ns-disc", "sa-up", payload); err != nil {
		t.Fatalf("emit sa-up: %v", err)
	}
	up := waitBatch(t, batches)
	envUp, err := rpc.ParseEventEnvelope(up[0])
	if err != nil {
		t.Fatalf("parse sa-up envelope from %q: %v", up[0], err)
	}
	if envUp.Namespace != "test-ns-disc" || envUp.Event != "sa-up" {
		t.Fatalf("sa-up envelope = %+v, want namespace=test-ns-disc event=sa-up", envUp)
	}
	if !strings.Contains(string(envUp.Payload), `"peer-name":"peer-shared"`) {
		t.Fatalf("sa-up envelope payload = %q", envUp.Payload)
	}

	if _, err := s.EmitEngineEvent("test-ns-disc", "sa-down", payload); err != nil {
		t.Fatalf("emit sa-down: %v", err)
	}
	down := waitBatch(t, batches)
	envDown, err := rpc.ParseEventEnvelope(down[0])
	if err != nil {
		t.Fatalf("parse sa-down envelope from %q: %v", down[0], err)
	}
	if envDown.Event != "sa-down" {
		t.Fatalf("sa-down envelope event = %q, want sa-down", envDown.Event)
	}

	// The two payloads are byte-identical; only the envelope discriminates.
	if string(envUp.Payload) != string(envDown.Payload) {
		t.Fatalf("payloads differ (%q vs %q): test no longer proves envelope-only discrimination",
			envUp.Payload, envDown.Payload)
	}
}

// AC-3/AC-7: without the envelope opt-in the delivered bytes are exactly the
// bare payload -- no envelope keys leak in. A count-only assertion would pass an
// accidental always-on envelope, so assert the BYTES.
func TestBgpStartupSubscriptionUnchanged(t *testing.T) {
	events.Register[*saShape]("test-ns-bare", "sa-up")

	s := newSubscriptionServer()
	proc, batches := newDeliveryProbe(t, "bare-probe")

	s.registerSubscriptions(proc, &rpc.SubscribeEventsInput{
		Namespace: "test-ns-bare",
		Events:    []string{"sa-up"},
		// No Envelope: legacy default.
	})

	payload := &saShape{PeerName: "peer-x"}
	if _, err := s.EmitEngineEvent("test-ns-bare", "sa-up", payload); err != nil {
		t.Fatalf("emit: %v", err)
	}

	want, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}

	batch := waitBatch(t, batches)
	if len(batch) != 1 {
		t.Fatalf("batch len = %d, want 1", len(batch))
	}
	if batch[0] != string(want) {
		t.Fatalf("delivered bytes = %q, want bare payload %q (envelope must be OFF by default)", batch[0], want)
	}
	if strings.Contains(batch[0], `"namespace"`) || strings.Contains(batch[0], `"payload"`) {
		t.Fatalf("delivered bytes contain envelope keys: %q", batch[0])
	}
}

// AC-3 fallback: an empty namespace resolves to the process-global default
// exactly as before Gap A (here the default is unregistered, so "" -> ok=true,
// preserving the legacy warn-and-continue rather than skipping).
func TestEmptyNamespaceUsesDefault(t *testing.T) {
	s := newSubscriptionServer()
	proc := process.NewProcess(plugin.PluginConfig{Name: "default-probe"})

	nsID, ns, ok := s.resolveSubscriptionNamespace(proc, "")
	if !ok {
		t.Fatal("empty namespace must resolve (ok=true), preserving legacy behavior")
	}
	if ns != plugin.DefaultEventNamespace() {
		t.Fatalf("resolved namespace = %q, want default %q", ns, plugin.DefaultEventNamespace())
	}
	if nsID != events.LookupNamespaceID(plugin.DefaultEventNamespace()) {
		t.Fatalf("resolved nsID = %d, want default's ID", nsID)
	}
}

// AC-6 (Gap A): an unknown namespace is skipped, not registered as a
// silently-dead NamespaceUnknown subscription.
func TestStartupSubscriptionUnknownNamespaceWarnsAndSkips(t *testing.T) {
	s := newSubscriptionServer()
	proc := process.NewProcess(plugin.PluginConfig{Name: "unknown-ns-probe"})

	s.registerSubscriptions(proc, &rpc.SubscribeEventsInput{
		Namespace: "no-such-namespace-xyz",
		Events:    []string{"sa-up", "*"},
	})

	if n := s.subscriptions.Count(proc); n != 0 {
		t.Fatalf("subscription count = %d, want 0 (unknown namespace must skip, not register NamespaceUnknown)", n)
	}
}

// A rejected subscribe block (unknown namespace) must have NO side effects: it
// must not reconfigure the process's per-process delivery state (format/
// encoding/envelope) for subscriptions already registered.
func TestUnknownNamespaceBlockHasNoSideEffects(t *testing.T) {
	events.Register[*saShape]("test-ns-noside", "sa-up")

	s := newSubscriptionServer()
	proc := process.NewProcess(plugin.PluginConfig{Name: "noside-probe"})

	// A valid bare (no-envelope) subscription first.
	s.registerSubscriptions(proc, &rpc.SubscribeEventsInput{
		Namespace: "test-ns-noside",
		Events:    []string{"sa-up"},
	})
	if proc.Envelope() {
		t.Fatal("precondition: envelope should be off")
	}

	// A later block naming an unknown namespace, opting into the envelope, must
	// be rejected wholesale -- including its envelope opt-in.
	s.registerSubscriptions(proc, &rpc.SubscribeEventsInput{
		Namespace: "no-such-namespace-xyz",
		Events:    []string{"sa-up"},
		Envelope:  true,
	})
	if proc.Envelope() {
		t.Fatal("rejected block flipped the process to enveloped delivery (side effect on reject)")
	}
	if n := s.subscriptions.Count(proc); n != 1 {
		t.Fatalf("subscription count = %d, want 1 (rejected block must not add subs)", n)
	}
}

// A nil (signal) payload delivered under the envelope must produce a well-formed
// envelope with payload:null -- buildEventEnvelope must not choke on the "null"
// passthrough from payloadToJSON.
func TestEnvelopeWithNilSignalPayload(t *testing.T) {
	events.RegisterSignal("test-ns-signal", "tick")

	s := newSubscriptionServer()
	proc, batches := newDeliveryProbe(t, "signal-probe")

	s.registerSubscriptions(proc, &rpc.SubscribeEventsInput{
		Namespace: "test-ns-signal",
		Events:    []string{"tick"},
		Envelope:  true,
	})

	if _, err := s.EmitEngineEvent("test-ns-signal", "tick", nil); err != nil {
		t.Fatalf("emit signal: %v", err)
	}
	batch := waitBatch(t, batches)
	env, err := rpc.ParseEventEnvelope(batch[0])
	if err != nil {
		t.Fatalf("parse envelope %q: %v", batch[0], err)
	}
	if env.Namespace != "test-ns-signal" || env.Event != "tick" {
		t.Fatalf("envelope = %+v, want namespace=test-ns-signal event=tick", env)
	}
	if string(env.Payload) != "null" {
		t.Fatalf("signal payload = %q, want null", env.Payload)
	}
}

// AC-8 (Gap C): a "*" startup subscription expands into one subscription per
// registered event type of the namespace and delivers, where today it is dead.
func TestStartupWildcardExpandsToNamespaceEvents(t *testing.T) {
	events.Register[*saShape]("test-ns-wild", "sa-up")
	events.Register[*saShape]("test-ns-wild", "sa-down")

	s := newSubscriptionServer()
	proc, batches := newDeliveryProbe(t, "wild-probe")

	s.registerSubscriptions(proc, &rpc.SubscribeEventsInput{
		Namespace: "test-ns-wild",
		Events:    []string{"*"},
	})

	want := len(events.AllEventTypes()["test-ns-wild"])
	if want < 2 {
		t.Fatalf("expected at least sa-up and sa-down registered, got %d", want)
	}
	if n := s.subscriptions.Count(proc); n != want {
		t.Fatalf("wildcard expanded to %d subscriptions, want %d (one per event type)", n, want)
	}

	delivered, err := s.EmitEngineEvent("test-ns-wild", "sa-up", &saShape{PeerName: "peer-w"})
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if delivered != 1 {
		t.Fatalf("delivered = %d, want 1 (wildcard subscription must match, not be a dead no-op)", delivered)
	}
	batch := waitBatch(t, batches)
	if len(batch) != 1 || !strings.Contains(batch[0], `"peer-name":"peer-w"`) {
		t.Fatalf("delivered batch = %v, want the sa-up payload", batch)
	}
}
