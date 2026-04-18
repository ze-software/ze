// Design: docs/research/vpp-deployment-reference.md -- VPP interface event delivery
// Overview: ifacevpp.go -- VPP Backend implementation
// Related: query.go -- SwInterfaceDump used for initial name-map seed

package ifacevpp

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"sync"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"

	ifaceevents "codeberg.org/thomas-mangin/ze/internal/component/iface/events"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// monitor drives the VPP interface event loop. It subscribes to
// sw_interface_event notifications via WantInterfaceEvents + the GoVPP
// channel's SubscribeNotification hook, then translates each event into
// the same (namespace, event-type, JSON) shape ifacenetlink emits.
//
// start MUST be called exactly once. stop MUST be called after a successful
// start to release the subscription and drain the goroutine. stop is safe to
// call multiple times.
type monitor struct {
	b        *vppBackendImpl
	eventBus ze.EventBus
	sub      api.SubscriptionCtx
	notif    chan api.Message
	done     chan struct{}
	stopOnce sync.Once
}

// linkEventPayload matches the shape ifacenetlink emits on (interface,
// created/up/down). Keeping both backends on the same JSON keeps downstream
// subscribers (bgp/reactor, logging, web UI) backend-agnostic.
type linkEventPayload struct {
	Name  string `json:"name"`
	Unit  int    `json:"unit"`
	Type  string `json:"type"`
	Index int    `json:"index"`
	MTU   int    `json:"mtu"`
}

// stateEventPayload matches ifacenetlink's (interface, up/down) shape for
// non-creation transitions.
type stateEventPayload struct {
	Name  string `json:"name"`
	Unit  int    `json:"unit"`
	Index int    `json:"index"`
}

// StartMonitor begins delivering VPP interface events to the EventBus.
// Registration uses WantInterfaceEvents with PID=0 (VPP fills in the
// caller's PID server-side); the goroutine runs until StopMonitor closes
// the subscription.
func (b *vppBackendImpl) StartMonitor(eb ze.EventBus) error {
	if eb == nil {
		return fmt.Errorf("ifacevpp: StartMonitor requires non-nil event bus")
	}
	if err := b.ensureChannel(); err != nil {
		return err
	}
	// Idempotent: a second StartMonitor after a first success is a no-op.
	// This is important because the iface register-time code retries
	// StartMonitor on vppevents.EventConnected when the initial attempt
	// fired too early (backend not ready); the retry must not error out
	// after the backend has reconnected and monitor was already installed.
	b.monMu.Lock()
	if b.mon != nil {
		b.monMu.Unlock()
		return nil
	}
	b.monMu.Unlock()

	m := &monitor{
		b:        b,
		eventBus: eb,
		notif:    make(chan api.Message, 64),
		done:     make(chan struct{}),
	}
	sub, err := b.ch.SubscribeNotification(m.notif, &interfaces.SwInterfaceEvent{})
	if err != nil {
		return fmt.Errorf("ifacevpp: SubscribeNotification: %w", err)
	}
	m.sub = sub

	req := &interfaces.WantInterfaceEvents{EnableDisable: 1}
	reply := &interfaces.WantInterfaceEventsReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		_ = sub.Unsubscribe() //nolint:errcheck // cleanup path
		return fmt.Errorf("ifacevpp: WantInterfaceEvents enable: %w", err)
	}
	if reply.Retval != 0 {
		_ = sub.Unsubscribe() //nolint:errcheck // cleanup path
		return fmt.Errorf("ifacevpp: WantInterfaceEvents retval=%d", reply.Retval)
	}

	b.monMu.Lock()
	b.mon = m
	b.monMu.Unlock()

	go m.run()
	return nil
}

// StopMonitor halts the monitor goroutine and unsubscribes. Safe to call
// when no monitor is running.
func (b *vppBackendImpl) StopMonitor() {
	b.monMu.Lock()
	m := b.mon
	b.mon = nil
	b.monMu.Unlock()
	if m == nil {
		return
	}
	m.stop()
}

// stop disables event delivery on the VPP side, unsubscribes from the
// GoVPP channel, and waits for the dispatch goroutine to drain.
func (m *monitor) stop() {
	m.stopOnce.Do(func() {
		// Tell VPP to stop emitting. Ignore errors: the channel may
		// already be closed by the time Stop is called.
		req := &interfaces.WantInterfaceEvents{EnableDisable: 0}
		reply := &interfaces.WantInterfaceEventsReply{}
		_ = m.b.ch.SendRequest(req).ReceiveReply(reply) //nolint:errcheck // best-effort on stop

		if m.sub != nil {
			_ = m.sub.Unsubscribe() //nolint:errcheck // best-effort on stop
		}
		close(m.notif)
	})
	<-m.done
}

// run dispatches incoming SwInterfaceEvent notifications. It exits when the
// notif channel is closed (by stop) or when the GoVPP channel drops it.
func (m *monitor) run() {
	defer close(m.done)
	for msg := range m.notif {
		ev, ok := msg.(*interfaces.SwInterfaceEvent)
		if !ok {
			continue
		}
		m.safeHandleEvent(ev)
	}
}

// safeHandleEvent shields the event loop from a panicking consumer so one
// bad event cannot kill the monitor.
func (m *monitor) safeHandleEvent(ev *interfaces.SwInterfaceEvent) {
	defer func() {
		if r := recover(); r != nil {
			loggerPtr.Load().Error("ifacevpp monitor: panic in event handler",
				"panic", r, "stack", string(debug.Stack()))
		}
	}()
	m.handleEvent(ev)
}

// handleEvent translates a VPP sw_interface_event into the ze iface namespace.
// VPP emits one event for every interface state transition; Deleted=true
// distinguishes interface removal from an admin/link-state change.
func (m *monitor) handleEvent(ev *interfaces.SwInterfaceEvent) {
	idx := uint32(ev.SwIfIndex)
	name, hasName := m.b.names.LookupName(idx)
	if !hasName {
		name = fmt.Sprintf("sw_if_index_%d", idx)
	}
	if ev.Deleted {
		m.b.names.Remove(name)
		m.emit(ifaceevents.EventDown, linkEventPayload{
			Name: name, Index: int(idx),
		})
		return
	}
	if ev.Flags&interface_types.IF_STATUS_API_FLAG_ADMIN_UP != 0 {
		m.emit(ifaceevents.EventUp, stateEventPayload{
			Name: name, Index: int(idx),
		})
		return
	}
	m.emit(ifaceevents.EventDown, stateEventPayload{
		Name: name, Index: int(idx),
	})
}

// emit marshals payload and publishes it on the EventBus under the iface
// namespace. Errors are logged at debug: a single marshal failure must not
// silence the stream.
func (m *monitor) emit(eventType string, payload any) {
	if m.eventBus == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		loggerPtr.Load().Debug("ifacevpp monitor: marshal failed",
			"event", eventType, "err", err)
		return
	}
	if _, err := m.eventBus.Emit(ifaceevents.Namespace, eventType, string(data)); err != nil {
		loggerPtr.Load().Debug("ifacevpp monitor: emit failed",
			"event", eventType, "err", err)
	}
}
