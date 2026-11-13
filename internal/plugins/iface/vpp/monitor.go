// Design: docs/research/vpp-deployment-reference.md -- VPP interface event delivery
// Overview: ifacevpp.go -- VPP Backend implementation
// Related: query.go -- SwInterfaceDump used for initial name-map seed

package ifacevpp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"

	"github.com/ze-software/ze/internal/component/iface"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/ze"
)

var errIfacevppStartmonitorRequiresNonNilEvent = errors.New("ifacevpp: StartMonitor requires non-nil event bus")

// monitor drives the VPP interface event loop. It subscribes to
// sw_interface_event notifications via WantInterfaceEvents + the GoVPP
// channel's SubscribeNotification hook, then translates each event into
// the same (namespace, event-type, JSON) shape ifacenetlink emits.
//
// start MUST be called exactly once. stop MUST be called after a successful
// start to release the subscription and drain the goroutine. stop is safe to
// call multiple times.
type monitor struct {
	b                 *vppBackendImpl
	eventBus          ze.EventBus
	sub               api.SubscriptionCtx
	notif             chan api.Message
	stopped           chan struct{}
	done              chan struct{}
	stopOnce          sync.Once
	replay            chan struct{} // Depth one: simultaneous requests coalesce.
	unsubscribeReplay func()
	// publicationMu orders replay, address mutations and interface deletion.
	// addresses holds only addresses on current VPP interfaces, seeded from
	// the API and reclaimed on removal or deletion. Zero monitor has no cache.
	publicationMu sync.Mutex
	addresses     map[addressKey]addrEventPayload
}

type addressKey struct {
	index  interface_types.InterfaceIndex
	prefix netip.Prefix
}

// linkEventPayload matches the shape ifacenetlink emits on (interface,
// created/up/down). Keeping both backends on the same JSON keeps downstream
// subscribers (bgp/reactor, logging, web UI) backend-agnostic.
//
// MTU is omitted rather than reported as 0: no VPP create reply and no
// sw_interface_event carries the MTU, and a 0 would read as a real value.
type linkEventPayload struct {
	Name  string `json:"name"`
	Unit  int    `json:"unit"`
	Type  string `json:"type"`
	Index int    `json:"index"`
	MTU   int    `json:"mtu,omitempty"`
}

// addrEventPayload matches the shape ifacenetlink emits on (interface,
// addr-added/addr-removed), for the same reason as linkEventPayload.
type addrEventPayload struct {
	Name         string `json:"name"`
	Unit         int    `json:"unit"`
	Index        int    `json:"index"`
	Address      string `json:"address"`
	PrefixLength int    `json:"prefix-length"`
	Family       string `json:"family"`
	Origin       string `json:"origin,omitempty"`
}

// emitAddress announces an address change the backend has just made.
//
// VPP sends no notification for an interface address, so no monitor event can
// report one. The binary API call is synchronous: a zero retval means the
// address is on (or off) the interface when the call returns. So the backend
// announces it itself, after that reply. Without it, the config transaction
// waits for (interface, addr-added) until its settlement timeout and rolls the
// whole reload back, and every other consumer of the event (the BGP listener
// start, connected routes) never learns of the address.
//
// Nothing is emitted before StartMonitor has given the backend an event bus.
func (b *vppBackendImpl) emitAddress(eventType, ifaceName string, idx interface_types.InterfaceIndex, prefix netip.Prefix) {
	b.monMu.Lock()
	m := b.mon
	b.monMu.Unlock()
	if m == nil {
		return
	}
	parent, unit := splitUnit(ifaceName)
	family := "ipv6"
	if prefix.Addr().Is4() {
		family = "ipv4"
	}
	payload := addrEventPayload{
		Name:         parent,
		Unit:         unit,
		Index:        int(idx),
		Address:      prefix.Addr().String(),
		PrefixLength: prefix.Bits(),
		Family:       family,
		Origin:       "static",
	}
	m.publicationMu.Lock()
	defer m.publicationMu.Unlock()
	select {
	case <-m.stopped:
		return
	default:
	}
	key := addressKey{index: idx, prefix: prefix}
	if eventType == ifaceevents.EventAddrAdded {
		m.addresses[key] = payload
	} else {
		delete(m.addresses, key)
	}
	m.emit(eventType, payload)
}

// emitCreated announces an interface the backend has just created.
//
// VPP sends no sw_interface_event when an interface is created, only when its
// state changes or it is deleted, so no monitor event can report a create. The
// create call is synchronous, so the backend announces the interface itself,
// once every step of the create has succeeded. Without it, the config
// transaction waits for (interface, created) on an add-interface operation
// until its settlement timeout and rolls the whole reload back, and the
// interface resolver never learns that the link appeared.
//
// The name is the full ze name ("xe0.100" for a VLAN unit, unit 0), which is
// how ifacenetlink names a created link. kind is the ze interface type.
//
// Nothing is emitted before StartMonitor has given the backend an event bus.
func (b *vppBackendImpl) emitCreated(name, kind string, idx interface_types.InterfaceIndex) {
	b.monMu.Lock()
	m := b.mon
	b.monMu.Unlock()
	if m == nil {
		return
	}
	m.emit(ifaceevents.EventCreated, linkEventPayload{
		Name:  name,
		Type:  kind,
		Index: int(idx),
	})
}

// splitUnit splits "<parent>.<unit>" the way ifacenetlink names a VLAN unit
// in its address events. A name with no numeric suffix is unit 0.
func splitUnit(name string) (parent string, unit int) {
	dot := strings.LastIndexByte(name, '.')
	if dot <= 0 {
		return name, 0
	}
	n, err := strconv.Atoi(name[dot+1:])
	if err != nil || n < 0 {
		return name, 0
	}
	return name[:dot], n
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
		return errIfacevppStartmonitorRequiresNonNilEvent
	}
	if err := b.ensureChannel(); err != nil {
		return err
	}
	b.addressMu.Lock()
	defer b.addressMu.Unlock()
	// Idempotent: a second StartMonitor after a first success is a no-op.
	// This is important because the iface register-time code retries
	// StartMonitor on vppevents.EventConnected when the initial attempt
	// fired too early (backend not ready); the retry must not error out
	// after the backend has reconnected and monitor was already installed.
	b.monMu.Lock()
	defer b.monMu.Unlock()
	if b.mon != nil {
		return nil
	}

	m := &monitor{
		b:         b,
		eventBus:  eb,
		notif:     make(chan api.Message, 64),
		stopped:   make(chan struct{}),
		done:      make(chan struct{}),
		replay:    make(chan struct{}, 1),
		addresses: make(map[addressKey]addrEventPayload),
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

	if err := m.seedAddresses(); err != nil {
		disable := &interfaces.WantInterfaceEvents{EnableDisable: 0}
		_ = b.ch.SendRequest(disable).ReceiveReply(&interfaces.WantInterfaceEventsReply{}) //nolint:errcheck // failed startup releases its notification request
		_ = sub.Unsubscribe()                                                              //nolint:errcheck // failed startup releases its subscription
		return opererr.Wrap(err, "The VPP interface monitor could not read its initial addresses",
			"Connected routes need a complete interface address inventory", "ze show errors")
	}
	m.unsubscribeReplay = ifaceevents.AddressReplayRequest.Subscribe(eb, m.requestReplay)
	b.mon = m

	go m.run()
	return nil
}

// StopMonitor halts the monitor goroutine and unsubscribes. Safe to call
// when no monitor is running.
func (b *vppBackendImpl) StopMonitor() {
	b.addressMu.Lock()
	defer b.addressMu.Unlock()
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
// GoVPP channel, and waits for the dispatch goroutine to drain. It MUST close
// only stopped: GoVPP owns closing notif during Unsubscribe. run MUST join
// even when an unsuccessful unsubscribe leaves notif open.
func (m *monitor) stop() {
	m.stopOnce.Do(func() {
		m.publicationMu.Lock()
		close(m.stopped)
		m.publicationMu.Unlock()
		m.unsubscribeReplay()
		// Tell VPP to stop emitting. Ignore errors: the channel may
		// already be closed by the time Stop is called.
		req := &interfaces.WantInterfaceEvents{EnableDisable: 0}
		reply := &interfaces.WantInterfaceEventsReply{}
		_ = m.b.ch.SendRequest(req).ReceiveReply(reply) //nolint:errcheck // best-effort on stop

		if m.sub != nil {
			_ = m.sub.Unsubscribe() //nolint:errcheck // best-effort on stop
		}
	})
	<-m.done
}

// run dispatches incoming SwInterfaceEvent notifications. It MUST drain the
// queued notifications and join when stop closes stopped, even if GoVPP did
// not close notif. A closed producer channel also ends dispatch.
func (m *monitor) run() {
	defer close(m.done)
	defer m.unsubscribeReplay()
	for {
		// Prefer the stop signal even if a failed unsubscribe left a
		// producer continuously ready to send.
		select {
		case <-m.stopped:
			m.drainQueued()
			return
		default:
		}
		select {
		case <-m.stopped:
			m.drainQueued()
			return
		case <-m.replay:
			m.replayAddresses()
		case msg, ok := <-m.notif:
			if !ok {
				return
			}
			if ev, ok := msg.(*interfaces.SwInterfaceEvent); ok {
				m.safeHandleEvent(ev)
			}
		}
	}
}

// drainQueued preserves events queued before successful unsubscribe. Its
// snapshot is bounded by notif's capacity (64), so a producer still sending
// after an unsuccessful unsubscribe cannot keep shutdown alive indefinitely.
func (m *monitor) drainQueued() {
	for queued := len(m.notif); queued > 0; queued-- {
		msg, ok := <-m.notif
		if !ok {
			return
		}
		if ev, ok := msg.(*interfaces.SwInterfaceEvent); ok {
			m.safeHandleEvent(ev)
		}
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
	name, hasName := m.b.names.lookupName(idx)
	if !hasName {
		var bIdx textbuf.Buffer
		name = bIdx.Reset().Str("sw_if_index_").Int(int64(idx)).String()
	}
	if ev.Deleted {
		m.publicationMu.Lock()
		defer m.publicationMu.Unlock()
		for key, payload := range m.addresses {
			if uint32(key.index) == idx {
				delete(m.addresses, key)
				m.emit(ifaceevents.EventAddrRemoved, payload)
			}
		}
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

// seedAddresses MUST run while addressMu excludes complete Add/Remove calls.
// No event is emitted until the monitor is installed and a consumer asks.
func (m *monitor) seedAddresses() error {
	details, err := m.b.dumpAllRaw()
	if err != nil {
		return err
	}
	for i := range details {
		var info iface.InterfaceInfo
		if err := m.b.fillAddresses(&info, details[i].SwIfIndex); err != nil {
			return err
		}
		name, found := m.b.names.lookupName(uint32(details[i].SwIfIndex))
		if !found {
			name = trimCString(details[i].InterfaceName)
		}
		parent, unit := splitUnit(name)
		for _, address := range info.Addresses {
			ip, err := netip.ParseAddr(address.Address)
			if err != nil {
				return err
			}
			ip = ip.Unmap()
			if ip.IsUnspecified() {
				continue
			}
			if ip.IsMulticast() {
				continue
			}
			prefix := netip.PrefixFrom(ip, address.PrefixLength)
			if !prefix.IsValid() {
				return fmt.Errorf("ifacevpp: invalid address prefix %q/%d in interface dump", address.Address, address.PrefixLength)
			}
			m.addresses[addressKey{index: details[i].SwIfIndex, prefix: prefix}] = addrEventPayload{
				Name: parent, Unit: unit, Index: int(details[i].SwIfIndex),
				Address: ip.String(), PrefixLength: prefix.Bits(), Family: address.Family,
			}
		}
	}
	return nil
}

func (m *monitor) requestReplay() {
	select {
	case <-m.stopped:
		return
	default:
	}
	select {
	case m.replay <- struct{}{}:
	default: // One pending replay covers all requests received before it.
	}
}

func (m *monitor) replayAddresses() {
	m.publicationMu.Lock()
	defer m.publicationMu.Unlock()
	select {
	case <-m.stopped:
		return
	default:
	}
	for _, payload := range m.addresses {
		m.emit(ifaceevents.EventAddrAdded, payload)
	}
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
