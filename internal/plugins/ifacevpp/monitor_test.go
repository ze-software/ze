package ifacevpp

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"

	ifaceevents "codeberg.org/thomas-mangin/ze/internal/component/iface/events"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// recordingBus captures every Emit call. Satisfies ze.EventBus.
type recordingBus struct {
	mu     sync.Mutex
	events []capturedEvent
}

type capturedEvent struct {
	Namespace string
	Type      string
	Payload   string
}

var _ ze.EventBus = (*recordingBus)(nil)

func (b *recordingBus) Emit(namespace, eventType string, payload any) (int, error) {
	s, ok := payload.(string)
	if !ok {
		data, _ := json.Marshal(payload)
		s = string(data)
	}
	b.mu.Lock()
	b.events = append(b.events, capturedEvent{Namespace: namespace, Type: eventType, Payload: s})
	b.mu.Unlock()
	return 1, nil
}

func (b *recordingBus) Subscribe(_, _ string, _ func(any)) func() {
	return func() {}
}

func (b *recordingBus) len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events)
}

func (b *recordingBus) at(i int) capturedEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.events[i]
}

// monitorChannel mocks api.Channel for the monitor tests. It:
//   - captures WantInterfaceEvents requests
//   - exposes a "push" hook so tests can deliver synthetic SwInterfaceEvents
//     via the notification channel
type monitorChannel struct {
	mu       sync.Mutex
	notif    chan api.Message
	sendErr  error
	want     interfaces.WantInterfaceEvents
	wantOffs []bool // history of enable values observed
	reply    interfaces.WantInterfaceEventsReply
	closed   bool
}

var _ api.Channel = (*monitorChannel)(nil)

func (c *monitorChannel) SendRequest(msg api.Message) api.RequestCtx {
	c.mu.Lock()
	defer c.mu.Unlock()
	if w, ok := msg.(*interfaces.WantInterfaceEvents); ok {
		c.want = *w
		c.wantOffs = append(c.wantOffs, w.EnableDisable != 0)
	}
	return &monitorReqCtx{ch: c}
}

func (c *monitorChannel) SendMultiRequest(_ api.Message) api.MultiRequestCtx {
	// populateNameMap inside ensureChannel issues SwInterfaceDump. Return a
	// multi-reply ctx that reports "last" immediately so dumpAllRaw sees
	// zero interfaces and returns cleanly.
	return &emptyMultiCtx{}
}

type emptyMultiCtx struct{}

func (e *emptyMultiCtx) ReceiveReply(_ api.Message) (bool, error) { return true, nil }

func (c *monitorChannel) SubscribeNotification(notifChan chan api.Message, _ api.Message) (api.SubscriptionCtx, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notif = notifChan
	return &monitorSub{ch: c}, nil
}

func (c *monitorChannel) SetReplyTimeout(time.Duration)          {}
func (c *monitorChannel) CheckCompatiblity(...api.Message) error { return nil }
func (c *monitorChannel) Close()                                 { c.closed = true }

// push delivers a synthetic event to the subscriber. Blocks if the buffered
// channel is full -- tests should keep the payload small.
func (c *monitorChannel) push(ev *interfaces.SwInterfaceEvent) {
	c.mu.Lock()
	ch := c.notif
	c.mu.Unlock()
	ch <- ev
}

type monitorReqCtx struct{ ch *monitorChannel }

func (r *monitorReqCtx) ReceiveReply(msg api.Message) error {
	if r.ch.sendErr != nil {
		return r.ch.sendErr
	}
	if reply, ok := msg.(*interfaces.WantInterfaceEventsReply); ok {
		*reply = r.ch.reply
	}
	return nil
}

type monitorSub struct{ ch *monitorChannel }

func (s *monitorSub) Unsubscribe() error { return nil }

// waitForEvents polls the bus until it has at least n events or the deadline
// fires. Returns whether the threshold was met.
func waitForEvents(b *recordingBus, n int, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if b.len() >= n {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return b.len() >= n
}

func TestStartMonitorRequiresBus(t *testing.T) {
	// VALIDATES: AC-16 -- StartMonitor rejects nil bus
	ch := &monitorChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	if err := b.StartMonitor(nil); err == nil {
		t.Fatal("expected error for nil bus")
	}
}

func TestStartMonitorSendsEnable(t *testing.T) {
	// VALIDATES: AC-16 -- WantInterfaceEvents enable=1 sent on Start
	ch := &monitorChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	bus := &recordingBus{}

	if err := b.StartMonitor(bus); err != nil {
		t.Fatalf("StartMonitor: %v", err)
	}
	defer b.StopMonitor()

	if len(ch.wantOffs) != 1 || !ch.wantOffs[0] {
		t.Errorf("wantOffs history: got %v, want [true]", ch.wantOffs)
	}
}

// TestStartMonitorIdempotent verifies that a second StartMonitor after a
// first success is a no-op (returns nil without re-subscribing). This
// contract is load-bearing for spec-iface-vpp-ready-gate: the
// vppevents.EventConnected handler retries StartMonitor on every event so
// a deferred initial call can succeed, and subsequent events must not
// error after the monitor is already running.
func TestStartMonitorIdempotent(t *testing.T) {
	ch := &monitorChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	bus := &recordingBus{}

	if err := b.StartMonitor(bus); err != nil {
		t.Fatalf("StartMonitor: %v", err)
	}
	defer b.StopMonitor()

	if err := b.StartMonitor(bus); err != nil {
		t.Fatalf("second StartMonitor must be a no-op, got %v", err)
	}
	// wantOffs history should still show a single enable, not two.
	if len(ch.wantOffs) != 1 || !ch.wantOffs[0] {
		t.Errorf("wantOffs history: got %v, want [true]", ch.wantOffs)
	}
}

func TestMonitorEmitsUpEventOnAdminFlag(t *testing.T) {
	// VALIDATES: AC-16 -- ADMIN_UP flag translates to iface "up" event
	ch := &monitorChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe0", 7, "xe0")
	bus := &recordingBus{}

	if err := b.StartMonitor(bus); err != nil {
		t.Fatalf("StartMonitor: %v", err)
	}
	defer b.StopMonitor()

	ch.push(&interfaces.SwInterfaceEvent{
		SwIfIndex: 7,
		Flags:     interface_types.IF_STATUS_API_FLAG_ADMIN_UP,
	})

	if !waitForEvents(bus, 1, time.Second) {
		t.Fatal("no event received")
	}
	ev := bus.at(0)
	if ev.Namespace != ifaceevents.Namespace {
		t.Errorf("Namespace: got %q, want %q", ev.Namespace, ifaceevents.Namespace)
	}
	if ev.Type != ifaceevents.EventUp {
		t.Errorf("Type: got %q, want %q", ev.Type, ifaceevents.EventUp)
	}
	var payload stateEventPayload
	if err := json.Unmarshal([]byte(ev.Payload), &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if payload.Name != "xe0" {
		t.Errorf("payload.Name: got %q, want xe0", payload.Name)
	}
}

func TestMonitorEmitsDownOnAbsentFlag(t *testing.T) {
	// VALIDATES: AC-16 -- no ADMIN_UP flag translates to "down"
	ch := &monitorChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe0", 7, "xe0")
	bus := &recordingBus{}

	if err := b.StartMonitor(bus); err != nil {
		t.Fatalf("StartMonitor: %v", err)
	}
	defer b.StopMonitor()

	ch.push(&interfaces.SwInterfaceEvent{SwIfIndex: 7, Flags: 0})
	if !waitForEvents(bus, 1, time.Second) {
		t.Fatal("no event received")
	}
	if bus.at(0).Type != ifaceevents.EventDown {
		t.Errorf("Type: got %q, want %q", bus.at(0).Type, ifaceevents.EventDown)
	}
}

func TestMonitorDeletedRemovesFromNameMap(t *testing.T) {
	// VALIDATES: SwInterfaceEvent.Deleted=true clears the name-map entry
	ch := &monitorChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe0", 7, "xe0")
	bus := &recordingBus{}

	if err := b.StartMonitor(bus); err != nil {
		t.Fatalf("StartMonitor: %v", err)
	}
	defer b.StopMonitor()

	ch.push(&interfaces.SwInterfaceEvent{SwIfIndex: 7, Deleted: true})
	if !waitForEvents(bus, 1, time.Second) {
		t.Fatal("no event received")
	}
	if _, ok := b.names.LookupIndex("xe0"); ok {
		t.Error("name map should not contain xe0 after delete")
	}
}

func TestStopMonitorSendsDisable(t *testing.T) {
	// VALIDATES: AC-16 -- Stop sends WantInterfaceEvents with enable=0
	ch := &monitorChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	bus := &recordingBus{}

	if err := b.StartMonitor(bus); err != nil {
		t.Fatalf("StartMonitor: %v", err)
	}
	b.StopMonitor()

	if len(ch.wantOffs) < 2 || ch.wantOffs[1] {
		t.Errorf("wantOffs history: got %v, want [true,false]", ch.wantOffs)
	}
}

func TestStopMonitorWithoutStartSafe(t *testing.T) {
	// VALIDATES: StopMonitor is safe to call without Start
	ch := &monitorChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.StopMonitor() // no panic
}

func TestStartMonitorPropagatesSubscribeError(t *testing.T) {
	// VALIDATES: subscribe error returned to caller
	ch := &failSubChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	if err := b.StartMonitor(&recordingBus{}); err == nil {
		t.Fatal("expected error")
	}
}

// failSubChannel is a minimal api.Channel whose SubscribeNotification always
// fails.
type failSubChannel struct{ monitorChannel }

func (c *failSubChannel) SubscribeNotification(_ chan api.Message, _ api.Message) (api.SubscriptionCtx, error) {
	return nil, fmt.Errorf("subscribe failed")
}
