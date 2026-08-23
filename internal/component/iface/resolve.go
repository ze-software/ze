// Design: docs/architecture/iface/logical-name-resolution.md -- shared logical-name resolver
// Overview: iface.go -- Binding / LinkEvent value types
// Related: dispatch.go -- GetInterface backend dispatch this builds on
// Related: package internal/core/iface/events -- monitor event namespace and constants

package iface

import (
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"

	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/pkg/ze"
)

// resolver maps a logical interface name to a kernel device and serves value
// snapshots (Resolve), a scope-tagged address list (Addresses), and per-name
// link events (Subscribe). It is the single owner of logical-name -> device
// resolution for external consumers; the iface component's own mutation ops
// keep using netlink.Link directly (they need the full link object).
//
// Translation: a logical name maps to its OS device via the os-name selector
// (config os-name leaf, defaulting to the name itself). The forward map
// (logical -> os) drives Resolve/Addresses; the reverse map (os -> logical)
// lets a kernel-name monitor event reach the logical name(s) bound to it.
//
// Caching: Resolve results are cached by logical name. The cached ifindex is a
// hint -- any create/up/down monitor event for the device drops the entry, so
// the next Resolve re-reads the kernel and a deleted device never returns a
// stale ifindex (RTM_DELLINK arrives as a down event, monitor_linux.go).
type resolver struct {
	mu        sync.Mutex
	cache     map[string]Binding
	subs      map[string]map[int]chan LinkEvent
	nextSubID int
	osNames   map[string]string   // logical -> os (only entries that override)
	logicalOf map[string][]string // os -> logical names bound to it
	// permMACs maps a logical name to the normalized MAC its mac/match selector
	// binds to. Empty when no mac/match selector is configured, which is the
	// common case. A reverse map (MAC -> logical names) sat beside it until
	// 2026-08-22, to answer "which names select THIS device" from a device's
	// MAC. Nothing asks that any more: reading a device's MAC inside a link
	// event handler meant a backend call on the monitor's read loop, and
	// logicalsForLocked reaches the same names without it.
	permMACs map[string]string
	// boundOS records the kernel device each logical name last resolved to. It
	// outlives a cache invalidation so a down/delete event for a device whose
	// MAC can no longer be read still reaches the mac/match binding it backed.
	boundOS map[string]string
	bound   bool
	unsub   []func()
}

var globalResolver = &resolver{
	cache: make(map[string]Binding),
	subs:  make(map[string]map[int]chan LinkEvent),
}

// Resolve returns a value Binding for the logical interface name. The name is
// translated to its OS device via the os-name selector, then resolved against
// the active backend. Returns an error if no backend is loaded or the device
// is absent. The result is cached until a monitor link event invalidates it.
func Resolve(name string) (Binding, error) { return globalResolver.resolve(name) }

// Addresses returns the IP addresses of the logical interface, each tagged with
// Family and LinkLocal so a consumer can split v4 / v6-link-local / v6-global
// without re-parsing. The logical name is translated via the os-name selector
// first. Returns an error if no backend is loaded or the device is absent.
func Addresses(name string) ([]AddrInfo, error) { return globalResolver.addresses(name) }

// Subscribe registers for link events on the logical name and returns a
// buffered channel plus a cancel func. The channel receives appeared/up/down
// events even for an interface that does not exist yet (it fires when the
// device appears).
//
// A subscriber that falls behind loses the OLDEST events buffered for it, never
// the newest, so the state the interface ENDED in always arrives (sendLatest).
// A consumer MAY therefore hold state across events; it MUST NOT assume it saw
// every transition. Each loss is counted in
// ze_iface_resolver_events_dropped_total.
//
// The caller MUST call cancel. It removes the subscription and closes the
// channel; it leaks no goroutine because event fan-out is synchronous in the
// monitor handler.
func Subscribe(name string) (<-chan LinkEvent, func()) { return globalResolver.subscribe(name) }

func (r *resolver) resolve(name string) (Binding, error) {
	r.mu.Lock()
	if b, ok := r.cache[name]; ok {
		r.mu.Unlock()
		return b, nil
	}
	r.mu.Unlock()

	info, err := r.osDeviceFor(name)
	if err != nil {
		return Binding{}, err
	}
	b := bindingFromInfo(info.OsName, info)

	r.mu.Lock()
	r.cache[name] = b
	r.mu.Unlock()
	return b, nil
}

func (r *resolver) addresses(name string) ([]AddrInfo, error) {
	info, err := r.osDeviceFor(name)
	if err != nil {
		return nil, err
	}
	return classifyAddresses(info.Addresses), nil
}

// osDeviceFor resolves a logical name to its kernel device InterfaceInfo. The
// mac/match selector wins when set: the resolver scans every interface and
// binds to the one carrying the configured hardware MAC (matchByMAC). Without a
// MAC selector the os-name selector (or the logical name itself) is looked up
// directly. On success it records the resolved kernel device so a later down
// event for it can reach this logical name. The backend call runs outside the
// lock; only the small map reads/writes are locked.
func (r *resolver) osDeviceFor(name string) (*InterfaceInfo, error) {
	r.mu.Lock()
	wantMAC := r.permMACs[name]
	osn := r.osNames[name]
	r.mu.Unlock()

	if wantMAC != "" {
		info, err := r.matchByMAC(name, wantMAC)
		if err != nil {
			return nil, err
		}
		r.recordBinding(name, info.OsName)
		return info, nil
	}

	osName := osn
	if osName == "" {
		osName = name
	}
	info, err := GetInterface(osName)
	if err != nil {
		return nil, err
	}
	r.recordBinding(name, info.OsName)
	return info, nil
}

// matchByMAC returns the interface whose hardware MAC equals want. It matches
// the permanent (factory) MAC when the device reports one -- so the binding
// survives an operational MAC override on a real NIC -- and falls back to the
// current MAC for the virtual kinds that report no permanent address
// (deviceMatchMAC, shared with the config-apply path through devicesWithMAC).
//
// Several matching devices is refused rather than resolved. Nothing
// distinguishes the candidates, so picking one -- by lowest ifindex or by any
// other rule -- is a guess about which physical port a caller's addresses,
// routes and admin state reach.
func (r *resolver) matchByMAC(name, want string) (*InterfaceInfo, error) {
	target := normalizeMAC(want)
	infos, err := ListInterfaces()
	if err != nil {
		return nil, err
	}
	matched := devicesWithMAC(infos, want)
	switch len(matched) {
	case 0:
		return nil, fmt.Errorf("iface: no device with MAC %s for logical interface %q", target, name)
	case 1:
		info := infos[matched[0]]
		return &info, nil
	default:
		names := make([]string, 0, len(matched))
		for _, idx := range matched {
			names = append(names, infos[idx].Name)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("iface: MAC %s for logical interface %q is carried by %d devices (%s); a hardware MAC selects at most one device",
			target, name, len(names), strings.Join(names, ", "))
	}
}

// deviceMatchMAC returns the MAC the resolver matches a device on: its permanent
// (factory) address when present, else its current address.
func deviceMatchMAC(info *InterfaceInfo) string {
	if info.PermanentMAC != "" {
		return info.PermanentMAC
	}
	return info.MAC
}

// normalizeMAC canonicalizes a MAC string for comparison (lower-case,
// colon-separated) via net.ParseMAC. An empty string stays empty; an
// unparseable value falls back to a trimmed lower-case copy so a garbage
// selector never collides with a real address by coincidence.
func normalizeMAC(s string) string {
	if s == "" {
		return ""
	}
	if hw, err := net.ParseMAC(s); err == nil {
		return hw.String()
	}
	return strings.ToLower(strings.TrimSpace(s))
}

// recordBinding remembers the kernel device a logical name currently resolves
// to. The reverse lookup (boundOS) lets a down/delete event for that device
// reach the logical name even after GetInterface can no longer report its MAC
// (the device is gone) -- the only way a deferred-then-bound mac/match selector
// learns it lost its device.
func (r *resolver) recordBinding(name, osDevice string) {
	if osDevice == "" {
		return
	}
	r.mu.Lock()
	if r.boundOS == nil {
		r.boundOS = make(map[string]string)
	}
	r.boundOS[name] = osDevice
	r.mu.Unlock()
}

// bindingFromInfo builds a value Binding from an InterfaceInfo resolved for the
// given OS device name. osName is the kernel device the info was read from.
func bindingFromInfo(osName string, info *InterfaceInfo) Binding {
	return Binding{
		Ifindex: info.Index,
		OsName:  osName,
		OperMAC: info.MAC,
		PermMAC: info.PermanentMAC,
		MTU:     info.MTU,
		State:   info.State,
	}
}

// classifyAddresses returns a copy of addrs with LinkLocal set per the IPv6
// fe80::/10 rule. IPv4 is never link-local-classified. Pure and backend-free
// so the scope split is host-testable.
func classifyAddresses(addrs []AddrInfo) []AddrInfo {
	if len(addrs) == 0 {
		return nil
	}
	out := make([]AddrInfo, len(addrs))
	for i := range addrs {
		out[i] = addrs[i]
		if ip, err := netip.ParseAddr(addrs[i].Address); err == nil {
			out[i].LinkLocal = ip.Is6() && ip.IsLinkLocalUnicast()
		}
	}
	return out
}

func (r *resolver) subscribe(name string) (<-chan LinkEvent, func()) {
	ch := make(chan LinkEvent, 8)
	r.mu.Lock()
	if r.subs[name] == nil {
		r.subs[name] = make(map[int]chan LinkEvent)
	}
	id := r.nextSubID
	r.nextSubID++
	r.subs[name][id] = ch
	r.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			r.mu.Lock()
			if m, ok := r.subs[name]; ok {
				if c, ok := m[id]; ok {
					delete(m, id)
					close(c)
				}
				if len(m) == 0 {
					delete(r.subs, name)
				}
			}
			r.mu.Unlock()
		})
	}
	return ch, cancel
}

// onLinkEvent invalidates the cache for every logical name bound to kernelName
// and fans the event out to each one's subscribers.
//
// It does NO I/O. It runs on the netlink monitor's read loop, because
// EmitEngineEvent (internal/component/plugin/server/engine_event.go) calls a
// subscriber synchronously on the emitter's goroutine, and pkg/ze/eventbus.go
// states that such a handler "MUST NOT block on I/O". A backend call here stops
// the loop, and the kernel-side subscription queue overflows behind it, one
// layer further from anything that can report it.
//
// Sends are non-blocking and performed under the lock (cancel holds the same
// lock when it closes a channel, so there is no send-on-closed race). A full
// channel loses its OLDEST event rather than this one, and COUNTS the loss, so
// a subscriber that falls behind is late and never stranded. sendLatest says
// why that direction is the load-bearing one.
func (r *resolver) onLinkEvent(kernelName string, kind LinkEventKind, index int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, logical := range r.logicalsForLocked(kernelName, kind != LinkDown) {
		delete(r.cache, logical)
		ev := LinkEvent{Name: logical, Kind: kind, Index: index}
		for _, c := range r.subs[logical] {
			if sendLatest(c, ev) {
				countResolverEventDropped(logical)
				loggerPtr.Load().Warn("iface resolver: subscriber channel full, discarded the oldest event",
					"interface", logical, "kind", string(kind))
			}
		}
	}
}

// sendLatest delivers ev to c, discarding the OLDEST buffered event when c is
// full so the state an interface ENDED in always reaches the subscriber. It
// reports whether an event was discarded, which the caller counts and logs.
//
// The direction is the whole point. A bare non-blocking send discards the
// NEWEST, and that is unrecoverable for a consumer that accumulates rather than
// recomputes. Sender.onLinkEvent (internal/plugins/iface/ra/sender_linux.go)
// records a down in state.linkDown, and its timer branch returns without
// rearming while that flag is set, so the next up event is the only thing that
// can restart router advertisements. Lose that one event and the interface
// stops advertising for the life of the process. Three consumers do re-attempt
// on a timer (30s for the IS-IS, OSPF and OSPFv3 rescans, 5s for the LDP
// discovery retry) and VRRP recomputes readiness on every wake-up, but a
// guarantee that holds for every subscriber is what a per-consumer audit cannot
// keep: the audit that said this drop was safe counted five subscribers when
// the tree held six.
//
// No channel operation here blocks, because the caller runs on the netlink
// monitor's read loop. The receive is non-blocking because a consumer can empty
// c between the send above it and it. The second send is non-blocking so a
// second sender, added later, cannot stall that loop either; today the caller
// holds r.mu and there is none, so it always succeeds.
//
// The caller MUST hold r.mu: cancel takes the same lock when it closes c, so
// there is no send-on-closed race.
func sendLatest(c chan LinkEvent, ev LinkEvent) bool {
	select {
	case c <- ev:
		return false
	default: // c is full; make room below rather than discarding ev
	}
	select {
	case <-c:
	default: // a consumer emptied c since the send above, so there is room
	}
	select {
	case c <- ev:
	default: // unreachable under r.mu; the caller still counts and logs the loss
	}
	return true
}

// hasSelector reports whether the logical name carries a hardware selector --
// an os-name alias or a mac/match binding. It is what separates "this name IS
// its kernel device" from "this name was bound to other hardware", which is the
// difference between a failed resolution that may fall back to the name and one
// that must not (resolveOS in dispatch.go).
func (r *resolver) hasSelector(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.permMACs[name]; ok {
		return true
	}
	_, ok := r.osNames[name]
	return ok
}

// logicalsForLocked returns every logical name a monitor event for kernelName
// must reach: kernelName itself (the default identity mapping), names whose
// os-name selector points at it, names whose mac/match binding last resolved to
// it (so a down event invalidates them even though the device's MAC is no
// longer readable), and -- when the device is appearing or coming up -- every
// mac/match name that does not currently know its device.
//
// That last set is how a freshly appeared device reaches a binding it was never
// bound to. It is deliberately a SUPERSET of the names whose selector matches
// this device's MAC, because knowing which ones match means reading the MAC,
// and the only way to read it here is a backend call on the monitor's read
// loop, which pkg/ze/eventbus.go forbids a subscriber to make. Every consumer
// answers a link event by re-resolving, so the extra names cost one Resolve
// each and reach the same answer.
//
// The set is empty in the steady state and stays small: a mac/match name leaves
// it as soon as it resolves, because a successful Resolve caches its binding,
// and it re-enters only when an event for its own device invalidates that
// cache. A name with no mac/match selector is never in it.
//
// Caller holds r.mu.
func (r *resolver) logicalsForLocked(kernelName string, appearing bool) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(ln string) {
		if _, ok := seen[ln]; ok {
			return
		}
		seen[ln] = struct{}{}
		out = append(out, ln)
	}
	add(kernelName)
	for _, ln := range r.logicalOf[kernelName] {
		add(ln)
	}
	for ln, dev := range r.boundOS {
		if dev == kernelName {
			add(ln)
		}
	}
	if appearing {
		for ln := range r.permMACs {
			if _, known := r.cache[ln]; !known {
				add(ln)
			}
		}
	}
	return out
}

// setMapping publishes the logical<->os-name and logical<->match-MAC selector
// maps after a config apply and drops the per-name cache and learned bindings
// (a config change can move any binding). The match-MAC values are normalized
// once here so the hot lookups compare canonical forms.
func (r *resolver) setMapping(logicalToOS, logicalToMAC map[string]string) {
	rev := make(map[string][]string, len(logicalToOS))
	for ln, osn := range logicalToOS {
		rev[osn] = append(rev[osn], ln)
	}
	macs := make(map[string]string, len(logicalToMAC))
	for ln, mac := range logicalToMAC {
		n := normalizeMAC(mac)
		if n == "" {
			continue
		}
		macs[ln] = n
	}
	r.mu.Lock()
	r.osNames = logicalToOS
	r.logicalOf = rev
	r.permMACs = macs
	r.cache = make(map[string]Binding)
	r.boundOS = make(map[string]string)
	r.mu.Unlock()
}

// bindEvents subscribes the resolver to the iface monitor's link events so it
// can invalidate its cache and drive Subscribe consumers. Called once by the
// iface component when the event bus is available; idempotent.
func (r *resolver) bindEvents(eb ze.EventBus) {
	if eb == nil {
		return
	}
	r.mu.Lock()
	if r.bound {
		r.mu.Unlock()
		return
	}
	r.bound = true
	r.mu.Unlock()

	sub := func(eventType string, kind LinkEventKind) func() {
		return eb.Subscribe(ifaceevents.Namespace, eventType, func(p any) {
			if name, idx, ok := decodeLinkEvent(p); ok {
				r.onLinkEvent(name, kind, idx)
			}
		})
	}
	u1 := sub(ifaceevents.EventCreated, LinkAppeared)
	u2 := sub(ifaceevents.EventUp, LinkUp)
	u3 := sub(ifaceevents.EventDown, LinkDown)

	r.mu.Lock()
	r.unsub = append(r.unsub, u1, u2, u3)
	r.mu.Unlock()
}

// decodeLinkEvent extracts the interface name and index from a monitor event
// payload. The monitor emits the payload as a JSON string (monitor_linux.go
// emit); both the link and state payload shapes carry name + index.
func decodeLinkEvent(p any) (string, int, bool) {
	s, ok := p.(string)
	if !ok {
		return "", 0, false
	}
	var sp struct {
		Name  string `json:"name"`
		Index int    `json:"index"`
	}
	if err := json.Unmarshal([]byte(s), &sp); err != nil || sp.Name == "" {
		return "", 0, false
	}
	return sp.Name, sp.Index, true
}

// setResolverConfig publishes the iface config's os-name mapping to the shared
// resolver. Called from the iface component's config-apply path.
func setResolverConfig(cfg *ifaceConfig) {
	globalResolver.setMapping(cfg.osNameMap(), cfg.permMACMap())
}

// bindResolverEvents wires the shared resolver to the event bus. Called from
// the iface component's config-apply path once a bus is available.
func bindResolverEvents(eb ze.EventBus) {
	globalResolver.bindEvents(eb)
}
